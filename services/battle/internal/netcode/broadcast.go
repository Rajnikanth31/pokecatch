package netcode

import "github.com/aurelia/beastbound/services/battle/internal/engine"

// broadcastTurn streams the resolved turn's events plus a fresh digest to both
// players and the spectator feed. The digest lets clients self-check for desync:
// they replay events locally and compare; a mismatch triggers a resync request.
func (s *Session) broadcastTurn(events []engine.Event) {
	for side := 0; side < 2; side++ {
		msg := ServerMessage{
			Type:      "turn",
			SessionID: s.ID,
			Turn:      s.battle.Turn,
			Events:    events,
			Digest:    s.battle.Digest(),
			State:     s.snapshot(side),
		}
		_ = s.tport.Send(s.players[side], msg)
	}
	// Spectators see side 0's perspective (or a neutral one); never private data.
	s.tport.Spectate(ServerMessage{
		Type: "turn", SessionID: s.ID, Turn: s.battle.Turn,
		Events: events, Digest: s.battle.Digest(), State: s.snapshot(0),
	})
}

func (s *Session) broadcastState(kind string) {
	for side := 0; side < 2; side++ {
		_ = s.tport.Send(s.players[side], ServerMessage{
			Type: kind, SessionID: s.ID, Turn: s.battle.Turn,
			State: s.snapshot(side), Digest: s.battle.Digest(),
		})
	}
}

func (s *Session) broadcastEnd() {
	for side := 0; side < 2; side++ {
		_ = s.tport.Send(s.players[side], ServerMessage{
			Type: "end", SessionID: s.ID, Winner: s.battle.Winner, Digest: s.battle.Digest(),
		})
	}
	s.tport.Spectate(ServerMessage{Type: "end", SessionID: s.ID, Winner: s.battle.Winner})
}

// snapshot builds the redacted view for a given side. The opponent's bench is
// shown only by count/identity that has already been revealed in battle; here we
// expose dex/name/level (public) and HP/status (public once on field) but never
// IVs/EVs/exact stats.
func (s *Session) snapshot(side int) *StateSnapshot {
	view := func(b *engine.Battler) BattlerView {
		return BattlerView{
			DexID: b.Species.DexID, Name: b.Species.Name, Level: b.Inst.Level,
			HPCur: b.CurHP, HPMax: b.MaxHP, Status: b.Status.String(), Fainted: b.Fainted(),
		}
	}
	bench := func(sd *engine.Side) []BattlerView {
		out := make([]BattlerView, 0, len(sd.Party))
		for i, b := range sd.Party {
			if i == sd.Active {
				continue
			}
			out = append(out, view(b))
		}
		return out
	}
	a, b := s.battle.Sides[0], s.battle.Sides[1]
	return &StateSnapshot{
		ActiveA: view(a.Party[a.Active]), ActiveB: view(b.Party[b.Active]),
		BenchA: bench(a), BenchB: bench(b), YourSide: side,
	}
}
