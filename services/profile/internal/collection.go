package profile

import (
	"context"
	"errors"
	"sort"

	"github.com/aurelia/beastbound/pkg/creatures"
)

// OwnedCreature is a persisted creature instance plus its owner. It wraps the
// domain creatures.Instance so the profile service can enforce ownership without
// the engine/domain layer knowing about accounts.
type OwnedCreature struct {
	Instance creatures.Instance
	OwnerID  string
}

// Repo is the profile persistence boundary (Postgres in prod, memory in tests).
// It exposes only the operations the use cases need — not raw SQL — so ownership
// and dupe invariants can be enforced in one place.
type Repo interface {
	AddCreature(ctx context.Context, oc OwnedCreature) error
	GetCreature(ctx context.Context, id string) (OwnedCreature, error)
	ListCreatures(ctx context.Context, ownerID string, cursor string, limit int) ([]OwnedCreature, string, error)
	UpdateCreature(ctx context.Context, oc OwnedCreature) error
	// SetTeam and GetTeam persist the ordered active party (creature ids).
	SetTeam(ctx context.Context, ownerID string, creatureIDs []string) error
	GetTeam(ctx context.Context, ownerID string) ([]string, error)
	// TransferOwnership moves a single creature (used outside trades, e.g. rewards).
	TransferOwnership(ctx context.Context, creatureID, fromID, toID string) error
	// TransferBatch applies MANY ownership changes ATOMICALLY — all succeed or none
	// do. This is what a trade commit runs on: swapping N creatures between two
	// accounts must never partially apply (that would dupe or vanish a creature).
	// Postgres runs it in one transaction; the memory impl under one lock.
	TransferBatch(ctx context.Context, transfers []OwnershipTransfer) error
}

// OwnershipTransfer describes one creature moving from one account to another.
type OwnershipTransfer struct {
	CreatureID string
	FromID     string
	ToID       string
}

var (
	ErrNotOwner    = errors.New("profile: creature not owned by caller")
	ErrTeamTooBig  = errors.New("profile: team exceeds 6 members")
	ErrTeamDup     = errors.New("profile: duplicate creature in team")
	ErrEmptyTeam   = errors.New("profile: team must have at least one member")
)

// Collection is the use-case layer for a player's creatures. It depends on the
// Repo interface and the immutable Dex (for evolution rules) — never on a DB
// directly.
type Collection struct {
	repo Repo
	dex  *creatures.Dex
}

func NewCollection(repo Repo, dex *creatures.Dex) *Collection {
	return &Collection{repo: repo, dex: dex}
}

// List returns a page of the caller's creatures with an opaque forward cursor.
func (c *Collection) List(ctx context.Context, ownerID, cursor string, limit int) ([]OwnedCreature, string, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	return c.repo.ListCreatures(ctx, ownerID, cursor, limit)
}

// SetTeam validates and persists the active party: 1..6 members, no duplicates,
// and every creature must belong to the caller. Validation is server-side because
// the team is competitively meaningful (clients cannot be trusted to enforce it).
func (c *Collection) SetTeam(ctx context.Context, ownerID string, creatureIDs []string) error {
	if len(creatureIDs) == 0 {
		return ErrEmptyTeam
	}
	if len(creatureIDs) > 6 {
		return ErrTeamTooBig
	}
	seen := map[string]bool{}
	for _, id := range creatureIDs {
		if seen[id] {
			return ErrTeamDup
		}
		seen[id] = true
		oc, err := c.repo.GetCreature(ctx, id)
		if err != nil {
			return err
		}
		if oc.OwnerID != ownerID {
			return ErrNotOwner
		}
	}
	return c.repo.SetTeam(ctx, ownerID, creatureIDs)
}

// sortByCreatedForCursor is a helper the memory repo uses for stable pagination.
func sortByID(list []OwnedCreature) {
	sort.Slice(list, func(i, j int) bool { return list[i].Instance.InstanceID < list[j].Instance.InstanceID })
}
