package profile

// Postgres implementation of Repo, backed by pgx. It maps the collection domain
// onto creature_instances + teams (db/migrations/0001_init.sql). The genes blob
// (IVs/EVs/nature/ability) is stored as JSONB since it is always read together.
//
// The two methods that carry real risk — SetTeam and TransferBatch — run inside
// transactions. TransferBatch is the trade-commit path and is the single most
// important piece of anti-dupe machinery in the whole backend.
//
// Dependency: github.com/jackc/pgx/v5.

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/aurelia/beastbound/pkg/creatures"
	"github.com/aurelia/beastbound/pkg/types"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PGRepo satisfies Repo against PostgreSQL.
type PGRepo struct {
	pool *pgxpool.Pool
}

func NewPGRepo(pool *pgxpool.Pool) *PGRepo { return &PGRepo{pool: pool} }

// genes is the JSONB shape for a creature's mutable identity.
type genes struct {
	IVs     types.Stats            `json:"ivs"`
	EVs     types.Stats            `json:"evs"`
	Nature  creatures.Nature       `json:"nature"`
	Ability creatures.PassiveAbility `json:"ability"`
}

func (r *PGRepo) AddCreature(ctx context.Context, oc OwnedCreature) error {
	g := genes{IVs: oc.Instance.IVs, EVs: oc.Instance.EVs, Nature: oc.Instance.Nature, Ability: oc.Instance.Ability}
	blob, err := json.Marshal(g)
	if err != nil {
		return err
	}
	skills := oc.Instance.SkillIDs[:]
	_, err = r.pool.Exec(ctx,
		`INSERT INTO creature_instances
		   (id, owner_id, dex_id, nickname, level, xp, genes, skill_ids, acquired_via)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		oc.Instance.InstanceID, oc.OwnerID, oc.Instance.DexID, nullify(oc.Instance.Nickname),
		oc.Instance.Level, oc.Instance.XP, blob, skills, "wild")
	return err
}

func (r *PGRepo) GetCreature(ctx context.Context, id string) (OwnedCreature, error) {
	return scanCreature(r.pool.QueryRow(ctx, selectCreature+` WHERE id = $1`, id))
}

const selectCreature = `SELECT id, owner_id, dex_id, COALESCE(nickname,''), level, xp, genes, skill_ids FROM creature_instances`

// ListCreatures uses keyset pagination (WHERE id > cursor ORDER BY id) — stable
// under concurrent inserts and O(page), unlike OFFSET paging.
func (r *PGRepo) ListCreatures(ctx context.Context, ownerID, cursor string, limit int) ([]OwnedCreature, string, error) {
	rows, err := r.pool.Query(ctx,
		selectCreature+` WHERE owner_id = $1 AND id > $2 ORDER BY id ASC LIMIT $3`,
		ownerID, cursor, limit)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()
	var out []OwnedCreature
	for rows.Next() {
		oc, err := scanCreatureRow(rows)
		if err != nil {
			return nil, "", err
		}
		out = append(out, oc)
	}
	next := ""
	if len(out) > 0 {
		next = out[len(out)-1].Instance.InstanceID
	}
	return out, next, rows.Err()
}

func (r *PGRepo) UpdateCreature(ctx context.Context, oc OwnedCreature) error {
	g := genes{IVs: oc.Instance.IVs, EVs: oc.Instance.EVs, Nature: oc.Instance.Nature, Ability: oc.Instance.Ability}
	blob, _ := json.Marshal(g)
	ct, err := r.pool.Exec(ctx,
		`UPDATE creature_instances
		    SET dex_id=$2, nickname=$3, level=$4, xp=$5, genes=$6, skill_ids=$7
		  WHERE id=$1`,
		oc.Instance.InstanceID, oc.Instance.DexID, nullify(oc.Instance.Nickname),
		oc.Instance.Level, oc.Instance.XP, blob, oc.Instance.SkillIDs[:])
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return errCreatureNotFound
	}
	return nil
}

// SetTeam replaces the active party atomically (delete-then-insert in one tx) so
// a failure never leaves a half-written team.
func (r *PGRepo) SetTeam(ctx context.Context, ownerID string, ids []string) error {
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `DELETE FROM teams WHERE account_id = $1`, ownerID); err != nil {
			return err
		}
		for slot, id := range ids {
			if _, err := tx.Exec(ctx,
				`INSERT INTO teams (account_id, slot, creature_id) VALUES ($1,$2,$3)`,
				ownerID, slot, id); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *PGRepo) GetTeam(ctx context.Context, ownerID string) ([]string, error) {
	rows, err := r.pool.Query(ctx, `SELECT creature_id FROM teams WHERE account_id=$1 ORDER BY slot`, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// TransferOwnership moves one creature, asserting the current owner (rows-affected
// guard). The WHERE owner_id = fromID clause is the concurrency-safe part: if two
// requests race, only one updates a row.
func (r *PGRepo) TransferOwnership(ctx context.Context, creatureID, fromID, toID string) error {
	ct, err := r.pool.Exec(ctx,
		`UPDATE creature_instances SET owner_id=$3 WHERE id=$1 AND owner_id=$2`,
		creatureID, fromID, toID)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotOwner
	}
	return nil
}

// TransferBatch is the trade-commit path: every ownership change in ONE
// transaction with a rows-affected guard on each. If any creature is no longer
// owned by its expected owner (someone moved it, a double-commit, a dupe attempt),
// the whole transaction rolls back and nothing changes.
func (r *PGRepo) TransferBatch(ctx context.Context, transfers []OwnershipTransfer) error {
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		for _, t := range transfers {
			ct, err := tx.Exec(ctx,
				`UPDATE creature_instances SET owner_id=$3 WHERE id=$1 AND owner_id=$2`,
				t.CreatureID, t.FromID, t.ToID)
			if err != nil {
				return err
			}
			if ct.RowsAffected() != 1 {
				return ErrNotOwner // aborts the tx -> full rollback (anti-dupe)
			}
		}
		return nil
	})
}

// --- scanning helpers ------------------------------------------------------

type scannable interface {
	Scan(dest ...any) error
}

func scanCreature(row pgx.Row) (OwnedCreature, error) {
	oc, err := scanCreatureRow(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return OwnedCreature{}, errCreatureNotFound
	}
	return oc, err
}

func scanCreatureRow(row scannable) (OwnedCreature, error) {
	var oc OwnedCreature
	var blob []byte
	var skills []string
	if err := row.Scan(
		&oc.Instance.InstanceID, &oc.OwnerID, &oc.Instance.DexID, &oc.Instance.Nickname,
		&oc.Instance.Level, &oc.Instance.XP, &blob, &skills,
	); err != nil {
		return OwnedCreature{}, err
	}
	var g genes
	if len(blob) > 0 {
		if err := json.Unmarshal(blob, &g); err != nil {
			return OwnedCreature{}, err
		}
	}
	oc.Instance.IVs, oc.Instance.EVs = g.IVs, g.EVs
	oc.Instance.Nature, oc.Instance.Ability = g.Nature, g.Ability
	copy(oc.Instance.SkillIDs[:], skills)
	return oc, nil
}

func nullify(s string) any {
	if s == "" {
		return nil
	}
	return s
}
