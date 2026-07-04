package netcode

import "github.com/aurelia/beastbound/services/battle/internal/engine"

// ServerMessage is the envelope pushed to clients over the WebSocket. A single
// tagged type keeps the wire protocol versionable and easy to mirror on the
// Godot client (which has a matching GDScript struct).
type ServerMessage struct {
	Type      string         `json:"type"` // match_start|turn|state|end|error
	SessionID string         `json:"session_id"`
	Turn      int            `json:"turn,omitempty"`
	Events    []engine.Event `json:"events,omitempty"`
	State     *StateSnapshot `json:"state,omitempty"`
	Digest    uint64         `json:"digest,omitempty"`
	Winner    int            `json:"winner,omitempty"`
	Error     string         `json:"error,omitempty"`
}

// StateSnapshot is a full authoritative view sent on connect/reconnect so a
// client can rebuild its UI without replaying the whole match. Deltas (Events)
// are used during normal play to save bandwidth; the snapshot is the recovery
// path for reconnection and desync correction.
type StateSnapshot struct {
	ActiveA  BattlerView   `json:"active_a"`
	ActiveB  BattlerView   `json:"active_b"`
	BenchA   []BattlerView `json:"bench_a"`
	BenchB   []BattlerView `json:"bench_b"`
	YourSide int           `json:"your_side"`
}

// BattlerView is the redacted projection a client is allowed to see. Opponent
// exact stats/IVs are intentionally omitted (only current/max HP %, status and
// visible stage changes) to prevent information-based cheating.
type BattlerView struct {
	DexID   int    `json:"dex_id"`
	Name    string `json:"name"`
	Level   int    `json:"level"`
	HPCur   int    `json:"hp_cur"`
	HPMax   int    `json:"hp_max"`
	Status  string `json:"status"`
	Fainted bool   `json:"fainted"`
}

// ClientMessage is what a client may send. The server validates and ignores
// anything beyond an action intent.
type ClientMessage struct {
	Type     string `json:"type"` // action | resync | heartbeat
	Seq      uint64 `json:"seq"`
	Kind     string `json:"kind"`        // move | switch
	Slot     int    `json:"slot"`        // for move
	SwitchTo int    `json:"switch_to"`   // for switch
}

// toIntent converts a wire message to an engine action.
func (m ClientMessage) toIntent() Intent {
	a := engine.MoveAction{}
	switch m.Kind {
	case "switch":
		a.Kind = engine.ActSwitch
		a.SwitchTo = m.SwitchTo
	default:
		a.Kind = engine.ActMove
		a.SkillSlot = m.Slot
	}
	return Intent{Action: a, Seq: m.Seq}
}
