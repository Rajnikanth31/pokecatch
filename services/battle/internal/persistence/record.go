// Package persistence records finished matches and re-simulates them for
// anti-cheat. The key insight (see docs/07-security.md): because the engine is a
// pure function of (seed, teams, per-turn intents), a match is fully captured by
// those inputs — a few kilobytes — and can be replayed to reproduce the exact
// outcome and state digest. Ranked results are only trusted if the stored final
// digest matches a fresh re-simulation.
package persistence

import (
	"github.com/aurelia/beastbound/pkg/creatures"
	"github.com/aurelia/beastbound/services/battle/internal/engine"
)

// TurnRecord is both players' intents for one turn, in side order.
type TurnRecord struct {
	Actions [2]engine.MoveAction `json:"actions"`
}

// MatchRecord is the complete, replayable capture of a match. It is what gets
// written to the matches/match_actions tables and to replay blobs in object
// storage. Teams are stored as full instances so re-simulation is independent of
// any mutable player collection state at replay time.
type MatchRecord struct {
	MatchID     string                  `json:"match_id"`
	Mode        string                  `json:"mode"`
	Seed        uint64                  `json:"seed"`
	TeamA       []creatures.Instance    `json:"team_a"`
	TeamB       []creatures.Instance    `json:"team_b"`
	Turns       []TurnRecord            `json:"turns"`
	Winner      int                     `json:"winner"`       // authoritative result claimed at play time
	FinalDigest uint64                  `json:"final_digest"` // engine digest at conclusion
}

// Recorder accumulates a MatchRecord as a live session progresses. The session
// calls Begin once and RecordTurn each resolved turn; Finish stamps the result.
type Recorder struct {
	rec MatchRecord
}

func NewRecorder(matchID, mode string, seed uint64, teamA, teamB []creatures.Instance) *Recorder {
	return &Recorder{rec: MatchRecord{
		MatchID: matchID, Mode: mode, Seed: seed, TeamA: teamA, TeamB: teamB,
	}}
}

// RecordTurn appends the intents that were resolved this turn.
func (r *Recorder) RecordTurn(actions [2]engine.MoveAction) {
	r.rec.Turns = append(r.rec.Turns, TurnRecord{Actions: actions})
}

// Finish stamps the authoritative result + digest and returns the record.
func (r *Recorder) Finish(winner int, digest uint64) MatchRecord {
	r.rec.Winner = winner
	r.rec.FinalDigest = digest
	return r.rec
}

// Replay re-simulates a recorded match from scratch using the immutable Dex and
// returns the reproduced winner, final digest, and turns played. It builds FRESH
// battlers from the stored instances so it shares no state with the original
// session — a true independent recomputation.
func Replay(rec MatchRecord, dex *creatures.Dex) (winner int, digest uint64, turns int, err error) {
	sideA, err := buildSide("A", rec.TeamA, dex)
	if err != nil {
		return 0, 0, 0, err
	}
	sideB, err := buildSide("B", rec.TeamB, dex)
	if err != nil {
		return 0, 0, 0, err
	}
	bt := engine.NewBattle(rec.Seed, sideA, sideB)
	for _, tr := range rec.Turns {
		if bt.Over {
			break
		}
		if _, err := bt.ResolveTurn(tr.Actions); err != nil {
			return 0, 0, 0, err
		}
	}
	return bt.Winner, bt.Digest(), bt.Turn, nil
}

// Verify is the anti-cheat check: re-simulate and confirm the reproduced digest
// and winner match what was recorded at play time. A mismatch means the recorded
// result was tampered with (or the engine version drifted) and must not be
// trusted for ranked rewards.
func Verify(rec MatchRecord, dex *creatures.Dex) (bool, error) {
	w, d, _, err := Replay(rec, dex)
	if err != nil {
		return false, err
	}
	return w == rec.Winner && d == rec.FinalDigest, nil
}

func buildSide(id string, team []creatures.Instance, dex *creatures.Dex) (*engine.Side, error) {
	party := make([]*engine.Battler, 0, len(team))
	for i := range team {
		inst := team[i] // copy; Replay must not mutate the caller's slice
		sp, ok := dex.Species[inst.DexID]
		if !ok {
			return nil, errUnknownSpecies(inst.DexID)
		}
		party = append(party, engine.NewBattler(sp, &inst))
	}
	return &engine.Side{PlayerID: id, Party: party}, nil
}
