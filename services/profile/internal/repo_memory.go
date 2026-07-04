package profile

import (
	"context"
	"errors"
	"sync"
)

// MemRepo is an in-memory Repo for dev/tests. The Postgres implementation
// satisfies the same interface against creature_instances/teams and runs
// TransferOwnership inside a transaction. Keeping the interface identical means
// the use-case tests here exercise the exact same logic that runs in production.
type MemRepo struct {
	mu        sync.RWMutex
	creatures map[string]OwnedCreature
	teams     map[string][]string
}

var errCreatureNotFound = errors.New("profile: creature not found")

func NewMemRepo() *MemRepo {
	return &MemRepo{creatures: map[string]OwnedCreature{}, teams: map[string][]string{}}
}

func (r *MemRepo) AddCreature(_ context.Context, oc OwnedCreature) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.creatures[oc.Instance.InstanceID] = oc
	return nil
}

func (r *MemRepo) GetCreature(_ context.Context, id string) (OwnedCreature, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	oc, ok := r.creatures[id]
	if !ok {
		return OwnedCreature{}, errCreatureNotFound
	}
	return oc, nil
}

func (r *MemRepo) UpdateCreature(_ context.Context, oc OwnedCreature) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.creatures[oc.Instance.InstanceID]; !ok {
		return errCreatureNotFound
	}
	r.creatures[oc.Instance.InstanceID] = oc
	return nil
}

// ListCreatures returns a stable, cursor-paginated slice ordered by instance id.
// The cursor is simply the last id returned; the next page starts after it. This
// keyset pagination is O(page) and stable under inserts (unlike offset paging).
func (r *MemRepo) ListCreatures(_ context.Context, ownerID, cursor string, limit int) ([]OwnedCreature, string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var owned []OwnedCreature
	for _, oc := range r.creatures {
		if oc.OwnerID == ownerID && oc.Instance.InstanceID > cursor {
			owned = append(owned, oc)
		}
	}
	sortByID(owned)
	next := ""
	if len(owned) > limit {
		owned = owned[:limit]
	}
	if len(owned) > 0 {
		next = owned[len(owned)-1].Instance.InstanceID
	}
	return owned, next, nil
}

func (r *MemRepo) SetTeam(_ context.Context, ownerID string, ids []string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.teams[ownerID] = append([]string{}, ids...)
	return nil
}

func (r *MemRepo) GetTeam(_ context.Context, ownerID string) ([]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return append([]string{}, r.teams[ownerID]...), nil
}

// TransferOwnership moves a creature between accounts, asserting the current owner
// matches (a guard against concurrent/duplicate commits). In Postgres this is an
// UPDATE ... WHERE owner_id = fromID with a rows-affected check inside the trade
// transaction.
func (r *MemRepo) TransferOwnership(_ context.Context, creatureID, fromID, toID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	oc, ok := r.creatures[creatureID]
	if !ok {
		return errCreatureNotFound
	}
	if oc.OwnerID != fromID {
		return ErrNotOwner // someone already moved it — abort (anti-dupe)
	}
	oc.OwnerID = toID
	r.creatures[creatureID] = oc
	return nil
}

// TransferBatch applies all transfers atomically: it validates every current
// owner FIRST, and only mutates if all checks pass, so a failure leaves the
// collection untouched (mirrors the Postgres single-transaction behaviour).
func (r *MemRepo) TransferBatch(_ context.Context, transfers []OwnershipTransfer) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	// Phase 1: validate every transfer against current state (no mutation).
	for _, t := range transfers {
		oc, ok := r.creatures[t.CreatureID]
		if !ok {
			return errCreatureNotFound
		}
		if oc.OwnerID != t.FromID {
			return ErrNotOwner // someone moved it — abort the whole batch (anti-dupe)
		}
	}
	// Phase 2: apply (cannot fail now).
	for _, t := range transfers {
		oc := r.creatures[t.CreatureID]
		oc.OwnerID = t.ToID
		r.creatures[t.CreatureID] = oc
	}
	return nil
}
