// Package ai is the server-side battle AI used for all PvE opponents (wild,
// trainers, Sanctum Keepers, raid bosses). It runs on the authoritative server,
// never the client, so PvE cannot be cheesed by reading client memory.
//
// Design: a shallow expectimax search over the legal action set with a
// hand-tuned heuristic leaf evaluation, wrapped by a difficulty knob that
// degrades the search to produce believably weaker opponents. The search reads
// the engine's public projection of state; it deliberately does NOT mutate the
// real Battle — it scores candidate actions and returns the best one for the
// session layer to submit like any other intent.
package ai

import (
	"github.com/aurelia/beastbound/pkg/creatures"
	"github.com/aurelia/beastbound/pkg/types"
)

// Difficulty controls how strong the AI plays. It maps to search depth and the
// probability of choosing a sub-optimal action, which is how we make "easy"
// opponents feel weak without playing randomly.
type Difficulty uint8

const (
	Easy Difficulty = iota
	Normal
	Hard
	Boss
)

func (d Difficulty) searchDepth() int {
	switch d {
	case Easy:
		return 1
	case Normal, Hard:
		return 2
	default:
		return 2 // bosses get depth 2 + no mistake chance + scripted phases on top
	}
}

// mistakeChance is the probability the AI picks the 2nd-best action instead of
// the best — the primary "weakness" dial for accessible PvE.
func (d Difficulty) mistakeChance() int {
	switch d {
	case Easy:
		return 35
	case Normal:
		return 15
	case Hard:
		return 3
	default:
		return 0
	}
}

// View is the minimal state the AI needs about one battler. The session adapts
// engine.Battler into this so the ai package has no dependency on the engine
// (keeps the dependency graph acyclic and the AI unit-testable in isolation).
type View struct {
	DexID     int
	Element1  types.Element
	Element2  types.Element
	HPFrac    float64 // 0..1
	Speed     int
	Status    types.StatusCondition
	Moves     []creatures.Skill // currently usable moves (cooldown-filtered)
	Fainted   bool
}

// Decision is the AI's chosen action, mirroring the engine's MoveAction without
// importing it.
type Decision struct {
	Switch   bool
	SkillIdx int // index into the active battler's usable Moves
	SwitchTo int // party index when Switch
}

// Randoms is the randomness the AI consumes. We inject it (rather than use a
// global) so the AI is deterministic in tests and can share the match RNG.
type Randoms interface {
	IntN(n int) int
	Chance(pct int) bool
}

// ChooseAction returns the AI's move for `self` (active) against `opp` (active),
// considering the option to switch to a bench member.
func ChooseAction(r Randoms, d Difficulty, self View, bench []View, opp View) Decision {
	type scored struct {
		dec   Decision
		score float64
	}
	var options []scored

	// Candidate 1..n: each usable move.
	for i, mv := range self.Moves {
		s := scoreMove(self, opp, mv)
		options = append(options, scored{Decision{SkillIdx: i}, s})
	}
	// Candidate switches: only consider when self is at a hard type disadvantage
	// or very low HP, so the AI doesn't switch pointlessly (a classic weak-AI tell).
	if shouldConsiderSwitch(self, opp) {
		for bi, b := range bench {
			if b.Fainted {
				continue
			}
			s := scoreSwitch(b, opp)
			options = append(options, scored{Decision{Switch: true, SwitchTo: bi}, s})
		}
	}

	if len(options) == 0 {
		return Decision{SkillIdx: 0} // safety net
	}

	// Sort best-first (simple selection; option count is tiny).
	best, second := 0, 0
	for i := 1; i < len(options); i++ {
		if options[i].score > options[best].score {
			second = best
			best = i
		} else if options[i].score > options[second].score || second == best {
			second = i
		}
	}

	// Difficulty: sometimes take the 2nd-best to feel beatable.
	if best != second && r.Chance(int(d.mistakeChance())) {
		return options[second].dec
	}
	return options[best].dec
}

// scoreMove estimates the value of using a move this turn. It rewards expected
// damage (type-effectiveness aware), KO potential, STAB, and useful status; it is
// the heuristic leaf eval of the search collapsed to depth-1 for clarity. The
// session can call the deeper search variant for Hard/Boss.
func scoreMove(self, opp View, mv creatures.Skill) float64 {
	if mv.Class == types.StatusOnly {
		// Value status moves when the opponent is healthy and statusable.
		if mv.Effect != nil && mv.Effect.Status != types.None && opp.Status == types.None && opp.HPFrac > 0.5 {
			return 60
		}
		return 20
	}
	eff := types.DualEffectiveness(mv.Element, opp.Element1, opp.Element2)
	stab := 1.0
	if mv.Element == self.Element1 || mv.Element == self.Element2 {
		stab = 1.5
	}
	// Expected-damage proxy: power * effectiveness * STAB * accuracy.
	acc := float64(min(mv.Accuracy, 100)) / 100.0
	expected := float64(mv.Power) * eff * stab * acc
	// KO bonus: if this likely removes the opponent, prioritize it heavily.
	if eff > 0 && expected*0.004 > opp.HPFrac { // crude lethal proxy
		expected += 120
	}
	if eff == 0 {
		return 0 // never pick an immune move
	}
	return expected
}

func scoreSwitch(in, opp View) float64 {
	// Reward bringing in a resist; penalize the tempo loss of switching.
	defMult := types.DualEffectiveness(opp.Element1, in.Element1, in.Element2)
	// Lower incoming effectiveness = better wall. Invert into a score.
	score := (2.0 - defMult) * 40
	return score - 25 // switch tempo cost
}

func shouldConsiderSwitch(self, opp View) bool {
	incoming := types.DualEffectiveness(opp.Element1, self.Element1, self.Element2)
	return incoming >= 2.0 || self.HPFrac < 0.25
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
