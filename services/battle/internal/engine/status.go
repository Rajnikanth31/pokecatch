package engine

import (
	"github.com/aurelia/beastbound/pkg/creatures"
	"github.com/aurelia/beastbound/pkg/types"
)

// applyStatus attempts to set a non-volatile status, respecting type immunities
// and the "one status at a time" rule. Returns true if it took hold.
func (b *Battler) applyStatus(s types.StatusCondition) bool {
	if b.Status != types.None || b.Fainted() {
		return false
	}
	// Type-based immunities keep the metagame coherent.
	switch s {
	case types.Burn:
		if b.hasElement(types.Fire) {
			return false
		}
	case types.Freeze:
		if b.hasElement(types.Ice) {
			return false
		}
	case types.Poison, types.Toxic:
		if b.hasElement(types.Toxin) || b.hasElement(types.Metal) {
			return false
		}
	case types.Paralysis:
		if b.hasElement(types.Electric) {
			return false
		}
	}
	b.Status = s
	if s == types.Sleep {
		b.StatusCtr = 1 + sleepRoll // 1..3 turns, set by caller via SetSleep
	}
	if s == types.Toxic {
		b.StatusCtr = 1
	}
	return true
}

// sleepRoll is replaced per-application; kept as a package var only for the
// zero-arg fallback. Real sleep duration is rolled with the match RNG below.
var sleepRoll int

// SetSleep applies sleep with an RNG-rolled 1..3 turn duration.
func (b *Battler) SetSleep(r *RNG) bool {
	if b.Status != types.None {
		return false
	}
	b.Status = types.Sleep
	b.StatusCtr = 1 + r.IntN(3) // 1..3
	return true
}

// PreMoveGate resolves status conditions that can prevent acting this turn.
// Returns (canAct, autoMessage). It mutates status counters (sleep ticking,
// thaw chance) so it must be called exactly once per battler per turn.
func (b *Battler) PreMoveGate(r *RNG) (bool, string) {
	switch b.Status {
	case types.Sleep:
		b.StatusCtr--
		if b.StatusCtr <= 0 {
			b.Status = types.None
			return true, "woke up"
		}
		return false, "is fast asleep"
	case types.Freeze:
		if r.Chance(20) { // 20% thaw each turn
			b.Status = types.None
			return true, "thawed out"
		}
		return false, "is frozen solid"
	case types.Paralysis:
		if r.Chance(25) { // 25% full-paralysis
			return false, "is paralyzed and can't move"
		}
	}
	if b.flinched {
		b.flinched = false
		return false, "flinched"
	}
	return true, ""
}

// EndOfTurnTick applies residual status damage and Regenerator. Returns the HP
// delta (negative = damage, positive = heal) for event reporting.
func (b *Battler) EndOfTurnTick() int {
	if b.Fainted() {
		return 0
	}
	delta := 0
	switch b.Status {
	case types.Burn, types.Poison:
		dmg := b.MaxHP / 8
		if dmg < 1 {
			dmg = 1
		}
		delta -= dmg
	case types.Toxic:
		dmg := b.MaxHP * b.StatusCtr / 16
		if dmg < 1 {
			dmg = 1
		}
		delta -= dmg
		b.StatusCtr++ // escalates each turn
	}
	if b.hasAbility(creatures.Regenerator) {
		heal := b.MaxHP / 16
		if heal < 1 {
			heal = 1
		}
		delta += heal
	}
	b.CurHP += delta
	if b.CurHP > b.MaxHP {
		b.CurHP = b.MaxHP
	}
	if b.CurHP < 0 {
		b.CurHP = 0
	}
	return delta
}
