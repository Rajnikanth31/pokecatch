package auth

import (
	"context"
	"errors"
	"sync"
)

// MemStore is an in-memory Store for local dev and unit tests. It is safe for
// concurrent use. The production Postgres implementation (store_postgres.go)
// satisfies the same interface against the accounts/auth_tokens tables.
type MemStore struct {
	mu        sync.RWMutex
	byID      map[string]Account
	byEmail   map[string]string // email -> id
	refresh   map[string]RefreshToken // id -> token
	refByHash map[string]string       // hash -> id
}

var errNotFound = errors.New("auth: not found")

func NewMemStore() *MemStore {
	return &MemStore{
		byID: map[string]Account{}, byEmail: map[string]string{},
		refresh: map[string]RefreshToken{}, refByHash: map[string]string{},
	}
}

func (m *MemStore) CreateAccount(_ context.Context, a Account) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.byEmail[a.Email]; ok {
		return ErrEmailTaken
	}
	m.byID[a.ID] = a
	m.byEmail[a.Email] = a.ID
	return nil
}

func (m *MemStore) AccountByEmail(_ context.Context, email string) (Account, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	id, ok := m.byEmail[email]
	if !ok {
		return Account{}, errNotFound
	}
	return m.byID[id], nil
}

func (m *MemStore) AccountByID(_ context.Context, id string) (Account, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	a, ok := m.byID[id]
	if !ok {
		return Account{}, errNotFound
	}
	return a, nil
}

func (m *MemStore) SaveRefresh(_ context.Context, rt RefreshToken) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.refresh[rt.ID] = rt
	m.refByHash[rt.Hash] = rt.ID
	return nil
}

func (m *MemStore) RefreshByHash(_ context.Context, hash string) (RefreshToken, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	id, ok := m.refByHash[hash]
	if !ok {
		return RefreshToken{}, errNotFound
	}
	return m.refresh[id], nil
}

func (m *MemStore) RevokeRefresh(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	rt, ok := m.refresh[id]
	if !ok {
		return errNotFound
	}
	rt.Revoked = true
	m.refresh[id] = rt
	return nil
}
