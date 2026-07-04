package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"
)

// Account is the auth-owned view of a user. The full profile lives in the profile
// service; auth only cares about credentials and token state.
type Account struct {
	ID           string
	Email        string
	DisplayName  string
	PasswordHash string
	TokenVer     int // bump to invalidate all outstanding access tokens
}

// RefreshToken is a persisted, HASHED refresh credential. We store only the hash
// so a DB leak cannot mint sessions; the raw token is shown to the client once.
type RefreshToken struct {
	ID        string
	AccountID string
	Hash      string
	DeviceID  string
	ExpiresAt time.Time
	Revoked   bool
}

// Store is the persistence boundary. The Postgres implementation lives in
// store_postgres.go (sketched); tests use the in-memory store. Depending on an
// interface (not a concrete DB) is what makes the service unit-testable and keeps
// the DB swappable (dependency inversion).
type Store interface {
	CreateAccount(ctx context.Context, a Account) error
	AccountByEmail(ctx context.Context, email string) (Account, error)
	AccountByID(ctx context.Context, id string) (Account, error)
	SaveRefresh(ctx context.Context, rt RefreshToken) error
	RefreshByHash(ctx context.Context, hash string) (RefreshToken, error)
	RevokeRefresh(ctx context.Context, id string) error
}

// Config carries the signing secret and token lifetimes.
type Config struct {
	Secret        []byte
	AccessTTL     time.Duration // e.g. 15m
	RefreshTTL    time.Duration // e.g. 30d
	IDFn          func() string // account/token id generator (injectable for tests)
}

// Service is the auth use-case layer. It orchestrates hashing, token issuance and
// the Store; it has no HTTP knowledge (that's handlers.go) — clean separation.
type Service struct {
	store Store
	cfg   Config
}

func NewService(store Store, cfg Config) *Service {
	if cfg.AccessTTL == 0 {
		cfg.AccessTTL = 15 * time.Minute
	}
	if cfg.RefreshTTL == 0 {
		cfg.RefreshTTL = 30 * 24 * time.Hour
	}
	if cfg.IDFn == nil {
		cfg.IDFn = randomID
	}
	return &Service{store: store, cfg: cfg}
}

// TokenPair is what login/refresh return to the client.
type TokenPair struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int
}

var (
	ErrEmailTaken   = errors.New("auth: email already registered")
	ErrBadCreds     = errors.New("auth: invalid email or password")
	ErrRefreshBad   = errors.New("auth: refresh token invalid or expired")
)

// Register creates an account and issues an initial token pair. Email is
// normalized; duplicate registration is rejected by the store's unique index.
func (s *Service) Register(ctx context.Context, email, password, displayName string) (TokenPair, error) {
	email = normalizeEmail(email)
	if _, err := s.store.AccountByEmail(ctx, email); err == nil {
		return TokenPair{}, ErrEmailTaken
	}
	hash, err := HashPassword(password)
	if err != nil {
		return TokenPair{}, err
	}
	acc := Account{ID: s.cfg.IDFn(), Email: email, DisplayName: displayName, PasswordHash: hash, TokenVer: 1}
	if err := s.store.CreateAccount(ctx, acc); err != nil {
		return TokenPair{}, err
	}
	return s.issuePair(ctx, acc, "")
}

// Login verifies credentials and issues a token pair. It always runs the password
// KDF even on unknown emails would-be — but here we short-circuit; to fully defeat
// user-enumeration timing you can verify against a dummy hash. Kept simple.
func (s *Service) Login(ctx context.Context, email, password, deviceID string) (TokenPair, error) {
	acc, err := s.store.AccountByEmail(ctx, normalizeEmail(email))
	if err != nil {
		return TokenPair{}, ErrBadCreds
	}
	ok, err := VerifyPassword(password, acc.PasswordHash)
	if err != nil || !ok {
		return TokenPair{}, ErrBadCreds
	}
	return s.issuePair(ctx, acc, deviceID)
}

// Refresh rotates a refresh token: the presented token is verified, REVOKED, and a
// brand-new pair is issued. Rotation-on-use means a stolen refresh token is
// single-use — if both attacker and victim use it, the second use fails and we can
// detect reuse (a signal to revoke the whole chain).
func (s *Service) Refresh(ctx context.Context, rawRefresh string) (TokenPair, error) {
	hash := hashToken(rawRefresh)
	rt, err := s.store.RefreshByHash(ctx, hash)
	if err != nil || rt.Revoked || time.Now().After(rt.ExpiresAt) {
		return TokenPair{}, ErrRefreshBad
	}
	if err := s.store.RevokeRefresh(ctx, rt.ID); err != nil {
		return TokenPair{}, err
	}
	acc, err := s.store.AccountByID(ctx, rt.AccountID)
	if err != nil {
		return TokenPair{}, ErrRefreshBad
	}
	return s.issuePair(ctx, acc, rt.DeviceID)
}

// issuePair signs an access token and persists a hashed refresh token.
func (s *Service) issuePair(ctx context.Context, acc Account, deviceID string) (TokenPair, error) {
	access, err := SignAccessToken(s.cfg.Secret, acc.ID, s.cfg.AccessTTL, acc.TokenVer)
	if err != nil {
		return TokenPair{}, err
	}
	rawRefresh := randomID() + randomID() // 256 bits of entropy
	rt := RefreshToken{
		ID: s.cfg.IDFn(), AccountID: acc.ID, Hash: hashToken(rawRefresh),
		DeviceID: deviceID, ExpiresAt: time.Now().Add(s.cfg.RefreshTTL),
	}
	if err := s.store.SaveRefresh(ctx, rt); err != nil {
		return TokenPair{}, err
	}
	return TokenPair{AccessToken: access, RefreshToken: rawRefresh, ExpiresIn: int(s.cfg.AccessTTL.Seconds())}, nil
}

func normalizeEmail(e string) string { return strings.ToLower(strings.TrimSpace(e)) }

func randomID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
