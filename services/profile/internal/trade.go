package profile

import (
	"context"
	"errors"
	"sync"
)

// Trading is THE dupe-attack surface of a collection game, so it is modeled as an
// explicit two-phase-commit state machine with escrow:
//
//	open ──stage──▶ open ──both lock──▶ locked ──commit──▶ committed
//	  └────────────── cancel ──────────────┘ (any time before commit)
//
// Invariants enforced here:
//   - You can only stage creatures you own.
//   - Staging (or un-staging) any item resets BOTH lock flags — you cannot change
//     the offer out from under a partner who already agreed (the classic switch
//     scam / TOCTOU dupe).
//   - The swap is applied exactly once, atomically, in Commit. In production
//     Commit runs inside a single Postgres transaction over TransferOwnership so a
//     crash mid-swap cannot dupe or vanish a creature.
type TradeState string

const (
	TradeOpen      TradeState = "open"      // staging + locking happen here
	TradeCommitted TradeState = "committed" // terminal: ownership swapped
	TradeCancelled TradeState = "cancelled" // terminal: voided, no changes
)

type Trade struct {
	ID          string
	InitiatorID string
	PartnerID   string
	State       TradeState
	// Offers maps accountID -> creature ids staged by that account.
	Offers      map[string][]string
	locked      map[string]bool
}

var (
	ErrTradeNotFound   = errors.New("profile: trade not found")
	ErrNotParticipant  = errors.New("profile: not a participant in this trade")
	ErrTradeClosed     = errors.New("profile: trade is not open")
	ErrNotLocked       = errors.New("profile: both parties must lock before commit")
)

// TradeManager coordinates trades. It holds transient trade state in memory
// (mirrored to the `trades`/`trade_items` tables); the authoritative swap goes
// through the Repo transactionally.
type TradeManager struct {
	mu     sync.Mutex
	repo   Repo
	trades map[string]*Trade
	idFn   func() string
}

func NewTradeManager(repo Repo, idFn func() string) *TradeManager {
	return &TradeManager{repo: repo, trades: map[string]*Trade{}, idFn: idFn}
}

// Open starts a trade between two players.
func (m *TradeManager) Open(initiatorID, partnerID string) *Trade {
	m.mu.Lock()
	defer m.mu.Unlock()
	t := &Trade{
		ID: m.idFn(), InitiatorID: initiatorID, PartnerID: partnerID, State: TradeOpen,
		Offers: map[string][]string{initiatorID: {}, partnerID: {}},
		locked: map[string]bool{initiatorID: false, partnerID: false},
	}
	m.trades[t.ID] = t
	return t
}

// Stage sets an account's offered creatures. Verifies ownership and resets locks.
func (m *TradeManager) Stage(ctx context.Context, tradeID, accountID string, creatureIDs []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, err := m.getOpen(tradeID, accountID)
	if err != nil {
		return err
	}
	// Ownership check: you can only offer what you own.
	for _, id := range creatureIDs {
		oc, err := m.repo.GetCreature(ctx, id)
		if err != nil {
			return err
		}
		if oc.OwnerID != accountID {
			return ErrNotOwner
		}
	}
	t.Offers[accountID] = append([]string{}, creatureIDs...)
	// Any change to the offer invalidates prior agreement from BOTH sides.
	t.locked[t.InitiatorID] = false
	t.locked[t.PartnerID] = false
	return nil
}

// Lock records that an account agrees to the CURRENT offer. The trade stays in
// the `open` state so either party can still change or withdraw their offer up
// until commit — doing so clears BOTH locks (see Stage). This is what prevents
// the switch-scam: a lock only counts against the exact offer it was placed on.
func (m *TradeManager) Lock(tradeID, accountID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, err := m.getOpen(tradeID, accountID)
	if err != nil {
		return err
	}
	t.locked[accountID] = true
	return nil
}

// Commit atomically swaps ownership once BOTH parties have locked the current
// offer. This is the only method that mutates ownership, and it does so through
// the Repo's transactional TransferOwnership so the whole swap succeeds or none
// of it does.
func (m *TradeManager) Commit(ctx context.Context, tradeID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.trades[tradeID]
	if !ok {
		return ErrTradeNotFound
	}
	if t.State != TradeOpen {
		return ErrTradeClosed // already committed or cancelled
	}
	if !(t.locked[t.InitiatorID] && t.locked[t.PartnerID]) {
		return ErrNotLocked
	}
	// Build the full set of transfers and apply them ATOMICALLY. If any single
	// transfer would fail (e.g. a creature was moved out from under the trade),
	// TransferBatch rolls back ALL of them — the swap is all-or-nothing, which is
	// what makes trading dupe-proof.
	transfers := make([]OwnershipTransfer, 0, len(t.Offers[t.InitiatorID])+len(t.Offers[t.PartnerID]))
	for _, id := range t.Offers[t.InitiatorID] {
		transfers = append(transfers, OwnershipTransfer{CreatureID: id, FromID: t.InitiatorID, ToID: t.PartnerID})
	}
	for _, id := range t.Offers[t.PartnerID] {
		transfers = append(transfers, OwnershipTransfer{CreatureID: id, FromID: t.PartnerID, ToID: t.InitiatorID})
	}
	if err := m.repo.TransferBatch(ctx, transfers); err != nil {
		return err // nothing applied; trade stays open so it can be retried/cancelled
	}
	t.State = TradeCommitted
	return nil
}

// Cancel voids a trade before commit. No ownership changes.
func (m *TradeManager) Cancel(tradeID, accountID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.trades[tradeID]
	if !ok {
		return ErrTradeNotFound
	}
	if accountID != t.InitiatorID && accountID != t.PartnerID {
		return ErrNotParticipant
	}
	if t.State == TradeCommitted {
		return ErrTradeClosed
	}
	t.State = TradeCancelled
	return nil
}

// Get returns a snapshot copy of a trade for read APIs.
func (m *TradeManager) Get(tradeID string) (Trade, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.trades[tradeID]
	if !ok {
		return Trade{}, ErrTradeNotFound
	}
	return *t, nil
}

// getOpen fetches a trade the account participates in and asserts it is still open
// (caller must hold m.mu).
func (m *TradeManager) getOpen(tradeID, accountID string) (*Trade, error) {
	t, ok := m.trades[tradeID]
	if !ok {
		return nil, ErrTradeNotFound
	}
	if accountID != t.InitiatorID && accountID != t.PartnerID {
		return nil, ErrNotParticipant
	}
	if t.State != TradeOpen {
		return nil, ErrTradeClosed
	}
	return t, nil
}
