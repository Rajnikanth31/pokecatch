// Package netcode wraps the pure battle engine in a server-authoritative,
// real-time session. The engine itself knows nothing about networking, clocks,
// or players timing out — that separation is deliberate (SRP): the engine stays
// a pure, testable function while everything time- and trust-sensitive lives
// here.
//
// Trust model: the client may ONLY submit an intent (move slot or switch). The
// server owns the Battle, the RNG seed and the authoritative clock. Each turn
// the server resolves both intents, advances state, computes a state Digest and
// streams the resulting events. A client that desyncs is corrected from the
// server's snapshot; a client that submits an illegal action has it dropped and
// replaced with a default (struggle / no-op), never trusted.
package netcode

import (
	"context"
	"sync"
	"time"

	"github.com/aurelia/beastbound/services/battle/internal/engine"
)

// Phase is the session lifecycle state.
type Phase uint8

const (
	PhaseWaiting   Phase = iota // both clients connecting
	PhaseSelecting              // collecting this turn's intents
	PhaseResolving              // engine applying the turn
	PhaseEnded
)

// Intent is the validated, queued action for one side this turn.
type Intent struct {
	Action engine.MoveAction
	Seq    uint64 // client sequence number for idempotency
}

// Transport is the minimal interface the session needs to talk to a client. A
// WebSocket implementation lives in ws.go; tests use an in-memory fake. Keeping
// this an interface is what lets us unit-test the whole session without sockets.
type Transport interface {
	Send(playerID string, msg ServerMessage) error
	Spectate(msg ServerMessage) // fan-out to spectators (may be no-op)
}

// Session coordinates one PvP match. One goroutine (run loop) owns the engine;
// all external input arrives via the inbox channel, so the engine is never
// touched concurrently — no locks around battle state, only around the inbox.
type Session struct {
	ID       string
	battle   *engine.Battle
	players  [2]string
	tport    Transport
	turnTime time.Duration

	mu      sync.Mutex
	phase   Phase
	pending [2]*Intent
	inbox   chan inboundCmd
	done    chan struct{}
	onTurn  func(actions [2]engine.MoveAction)
	onEnd   func(winner int, digest uint64)
}

type inboundCmd struct {
	playerID string
	intent   Intent
}

// Config carries the wiring for a new session.
type Config struct {
	ID         string
	Seed       uint64
	PlayerA    string
	PlayerB    string
	SideA      *engine.Side
	SideB      *engine.Side
	Transport  Transport
	TurnBudget time.Duration // wall-clock allowed per turn before auto-default
	// OnTurn/OnEnd are optional observers. The battle-service wires these to the
	// persistence.Recorder so every ranked match is captured for replay/anti-cheat
	// re-simulation — without the session package depending on persistence
	// (dependency inversion: the session emits events, the caller decides to store).
	OnTurn func(actions [2]engine.MoveAction)
	OnEnd  func(winner int, digest uint64)
}

// NewSession builds (but does not start) a match session.
func NewSession(cfg Config) *Session {
	if cfg.TurnBudget == 0 {
		cfg.TurnBudget = 45 * time.Second
	}
	return &Session{
		ID:       cfg.ID,
		battle:   engine.NewBattle(cfg.Seed, cfg.SideA, cfg.SideB),
		players:  [2]string{cfg.PlayerA, cfg.PlayerB},
		tport:    cfg.Transport,
		turnTime: cfg.TurnBudget,
		phase:    PhaseSelecting,
		inbox:    make(chan inboundCmd, 16),
		done:     make(chan struct{}),
		onTurn:   cfg.OnTurn,
		onEnd:    cfg.OnEnd,
	}
}

// Submit is the only entry point for client actions. It is safe to call from any
// goroutine (e.g. a WebSocket reader). Invalid or late submissions are dropped by
// the run loop, not here, so all game logic stays single-threaded.
func (s *Session) Submit(playerID string, intent Intent) {
	select {
	case s.inbox <- inboundCmd{playerID, intent}:
	case <-s.done:
	}
}

// sideOf maps a player id to its side index, or -1.
func (s *Session) sideOf(playerID string) int {
	switch playerID {
	case s.players[0]:
		return 0
	case s.players[1]:
		return 1
	default:
		return -1
	}
}

// Run drives the match until it ends or ctx is cancelled. It implements the
// authoritative turn loop: collect both intents (or time out), resolve, broadcast.
func (s *Session) Run(ctx context.Context) {
	defer close(s.done)
	s.broadcastState("match_start")

	for {
		if s.battle.Over {
			s.phase = PhaseEnded
			if s.onEnd != nil {
				s.onEnd(s.battle.Winner, s.battle.Digest())
			}
			s.broadcastEnd()
			return
		}
		timer := time.NewTimer(s.turnTime)
		s.phase = PhaseSelecting
		s.pending = [2]*Intent{}

		for !s.bothReady() {
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
				// Turn budget exhausted: any missing intent defaults to "use
				// slot 0". This keeps ranked matches moving and is the hook for
				// AFK detection (see anti-cheat doc).
				s.fillDefaults()
			case cmd := <-s.inbox:
				s.acceptIntent(cmd)
			}
		}
		timer.Stop()

		s.phase = PhaseResolving
		actions := [2]engine.MoveAction{s.pending[0].Action, s.pending[1].Action}
		events, err := s.battle.ResolveTurn(actions)
		if err != nil {
			return
		}
		if s.onTurn != nil {
			s.onTurn(actions) // record the resolved intents for replay/anti-cheat
		}
		s.broadcastTurn(events)
	}
}

func (s *Session) acceptIntent(cmd inboundCmd) {
	side := s.sideOf(cmd.playerID)
	if side < 0 {
		return // not a participant
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pending[side] != nil {
		return // already submitted this turn; ignore duplicates (idempotent)
	}
	if !s.validate(side, cmd.intent.Action) {
		// Illegal action: substitute the safe default rather than trust client.
		cmd.intent.Action = engine.MoveAction{Kind: engine.ActMove, SkillSlot: 0}
	}
	i := cmd.intent
	s.pending[side] = &i
}

// validate enforces server-side legality of an action. The engine also defends
// against bad input, but rejecting early gives the cheater no signal.
func (s *Session) validate(side int, a engine.MoveAction) bool {
	switch a.Kind {
	case engine.ActMove:
		return a.SkillSlot >= 0 && a.SkillSlot < 4
	case engine.ActSwitch:
		return a.SwitchTo >= 0 // deeper party/faint checks happen in engine.doSwitch
	default:
		return false
	}
}

func (s *Session) fillDefaults() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := 0; i < 2; i++ {
		if s.pending[i] == nil {
			s.pending[i] = &Intent{Action: engine.MoveAction{Kind: engine.ActMove, SkillSlot: 0}}
		}
	}
}

func (s *Session) bothReady() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.pending[0] != nil && s.pending[1] != nil
}
