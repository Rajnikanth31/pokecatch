package auth

// Postgres implementation of Store, backed by pgx. It maps the auth domain onto
// the accounts + auth_tokens tables (db/migrations/0001_init.sql). The service
// layer is unchanged — it depends on the Store interface, so switching from
// MemStore to PGStore is a one-line wiring change in main (dependency inversion).
//
// Dependency: github.com/jackc/pgx/v5 (add via `go mod tidy`).

import (
	"context"
	"errors"
	"time"

	"github.com/aurelia/beastbound/pkg/db"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PGStore satisfies Store against PostgreSQL.
type PGStore struct {
	pool *pgxpool.Pool
}

// NewPGStore wraps a pgx pool. The caller owns the pool's lifecycle.
func NewPGStore(pool *pgxpool.Pool) *PGStore { return &PGStore{pool: pool} }

func (s *PGStore) CreateAccount(ctx context.Context, a Account) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO accounts (id, email, password_hash, display_name)
		 VALUES ($1, $2, $3, $4)`,
		a.ID, a.Email, a.PasswordHash, a.DisplayName)
	if db.IsUniqueViolation(err) {
		return ErrEmailTaken
	}
	return err
}

func (s *PGStore) AccountByEmail(ctx context.Context, email string) (Account, error) {
	return s.scanAccount(ctx,
		`SELECT id, email, display_name, password_hash FROM accounts WHERE email = $1`, email)
}

func (s *PGStore) AccountByID(ctx context.Context, id string) (Account, error) {
	return s.scanAccount(ctx,
		`SELECT id, email, display_name, password_hash FROM accounts WHERE id = $1`, id)
}

func (s *PGStore) scanAccount(ctx context.Context, sql string, arg any) (Account, error) {
	var a Account
	// TokenVer is not persisted in the base schema; default to 1. A production
	// migration would add a token_ver column to support generation invalidation.
	a.TokenVer = 1
	err := s.pool.QueryRow(ctx, sql, arg).Scan(&a.ID, &a.Email, &a.DisplayName, &a.PasswordHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return Account{}, errNotFound
	}
	return a, err
}

func (s *PGStore) SaveRefresh(ctx context.Context, rt RefreshToken) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO auth_tokens (id, account_id, token_hash, device_id, expires_at, revoked)
		 VALUES ($1, $2, $3, $4, $5, false)`,
		rt.ID, rt.AccountID, rt.Hash, rt.DeviceID, rt.ExpiresAt)
	return err
}

func (s *PGStore) RefreshByHash(ctx context.Context, hash string) (RefreshToken, error) {
	var rt RefreshToken
	var device *string
	var expires time.Time
	err := s.pool.QueryRow(ctx,
		`SELECT id, account_id, token_hash, device_id, expires_at, revoked
		 FROM auth_tokens WHERE token_hash = $1`, hash).
		Scan(&rt.ID, &rt.AccountID, &rt.Hash, &device, &expires, &rt.Revoked)
	if errors.Is(err, pgx.ErrNoRows) {
		return RefreshToken{}, errNotFound
	}
	if device != nil {
		rt.DeviceID = *device
	}
	rt.ExpiresAt = expires
	return rt, err
}

func (s *PGStore) RevokeRefresh(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `UPDATE auth_tokens SET revoked = true WHERE id = $1`, id)
	return err
}
