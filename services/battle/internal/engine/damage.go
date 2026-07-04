package engine

import (
	"github.com/aurelia/beastbound/pkg/creatures"
	"github.com/aurelia/beastbound/pkg/types"
)

// DamageResult is the fully-resolved outcome of one offensive skill hit. It is
// returned (not just applied) so the netcode layer can emit a precise event for
// the client to animate, and so tests can assert on intermediate factors.
type DamageResult struct {
	Damage        int
	Crit          bool
	Effectiveness float64 // combined type multiplier (0, .25, .5, 1, 2, 4)
	STAB          float64
	Missed        bool
}

// critMultiplier is the bonus applied on a critical hit. Crits also ignore the
// defender's positive defense stages and the attacker's negative attack stages,
// handled in computeDamage.
const critMultiplier = 1.5

// critChanceByStage maps an effective crit tier to a percentage. Tier is the
// move's CritStage plus any ability/item bonuses, clamped to the table length.
var critChanceByStage = []int{4, 12, 50, 100}

// ComputeDamage resolves one hit of skill from attacker against defender using
// the supplied RNG. It performs the accuracy check, type/STAB/crit factors and
// the spread random multiplier, but does NOT mutate either battler — the caller
// applies Damage and any rider effects so the turn loop stays in one place.
//
// Pipeline (each step intentionally separated for testability):
//
//	1. Accuracy gate: acc = move.Accuracy * accStage(attacker) / evaStage(defender)
//	2. Base   = floor( floor(2*L/5 + 2) * Power * A/D / 50 ) + 2
//	3. * STAB (1.5, or 2.0 with Adaptable)
//	4. * type effectiveness (dual-type product)
//	5. * crit (1.5, ignoring unfavourable stages)
//	6. * random spread in [0.85, 1.00]
//	7. * ability modifiers (Blaze/Torrent/Overgrow/IronHide)
func ComputeDamage(r *RNG, attacker, defender *Battler, skill *creatures.Skill) DamageResult {
	res := DamageResult{Effectiveness: 1, STAB: 1}

	if skill.Class == types.StatusOnly || skill.Power <= 0 {
		return res // non-damaging move; rider handled elsewhere
	}

	// 1. Accuracy --------------------------------------------------------
	if skill.Accuracy <= 100 { // >100 = bypass accuracy entirely
		accMult := accStageMultiplier(attacker.stages.acc) / accStageMultiplier(defender.stages.eva)
		effAcc := float64(skill.Accuracy) * accMult
		if !r.Chance(int(effAcc)) {
			res.Missed = true
			return res
		}
	}

	// 5 (decide crit early so we can null unfavourable stages) -----------
	critTier := int(skill.CritStage)
	if critTier < 0 {
		critTier = 0
	}
	if critTier >= len(critChanceByStage) {
		critTier = len(critChanceByStage) - 1
	}
	res.Crit = r.Chance(critChanceByStage[critTier])

	// 2. Base damage -----------------------------------------------------
	atk := attacker.effAttack(skill.Class)
	def := defender.effDefense(skill.Class)
	if res.Crit {
		// Crits ignore the defender's positive stages and attacker's negatives
		// by recomputing from raw stats when those stages are unfavourable.
		atk = critAttack(attacker, skill.Class)
		def = critDefense(defender, skill.Class)
	}
	level := float64(attacker.Inst.Level)
	base := (float64(int(2*level/5)+2) * float64(skill.Power) * atk / def) / 50.0
	dmg := float64(int(base) + 2)

	// 3. STAB ------------------------------------------------------------
	if attacker.hasElement(skill.Element) {
		res.STAB = 1.5
		if attacker.hasAbility(creatures.Adaptable) {
			res.STAB = 2.0
		}
		dmg *= res.STAB
	}

	// 4. Type effectiveness ---------------------------------------------
	res.Effectiveness = types.DualEffectiveness(skill.Element, defender.Species.Element1, defender.Species.Element2)
	if res.Effectiveness == 0 {
		res.Damage = 0
		return res // immune; no chip, no crit display
	}
	dmg *= res.Effectiveness

	// 5. Crit multiplier -------------------------------------------------
	if res.Crit {
		dmg *= critMultiplier
	}

	// 7. Ability modifiers (offensive) ----------------------------------
	dmg *= offensiveAbilityMod(attacker, skill)
	// 7b. Ability modifiers (defensive)
	dmg *= defensiveAbilityMod(defender, skill)

	// 6. Random spread (last, like the source games) --------------------
	spread := 0.85 + r.Float()*0.15 // [0.85, 1.00)
	dmg *= spread

	res.Damage = roundHalfUp(dmg)
	if res.Damage < 1 {
		res.Damage = 1 // any connecting damaging move deals at least 1
	}
	return res
}

func critAttack(b *Battler, class types.DamageClass) float64 {
	if class == types.Special {
		s := b.stages.spa
		if s < 0 {
			s = 0
		}
		return float64(b.Stats.SpAttack) * stageMultiplier(s)
	}
	s := b.stages.atk
	if s < 0 {
		s = 0
	}
	atk := float64(b.Stats.Attack) * stageMultiplier(s)
	if b.Status == types.Burn {
		atk *= 0.5
	}
	return atk
}

func critDefense(b *Battler, class types.DamageClass) float64 {
	if class == types.Special {
		s := b.stages.spd
		if s > 0 {
			s = 0
		}
		return float64(b.Stats.SpDefense) * stageMultiplier(s)
	}
	s := b.stages.def
	if s > 0 {
		s = 0
	}
	return float64(b.Stats.Defense) * stageMultiplier(s)
}

// offensiveAbilityMod implements the under-1/3-HP elemental boosts.
func offensiveAbilityMod(b *Battler, skill *creatures.Skill) float64 {
	low := b.CurHP*3 <= b.MaxHP
	switch {
	case b.hasAbility(creatures.Blaze) && skill.Element == types.Fire && low:
		return 1.5
	case b.hasAbility(creatures.Torrent) && skill.Element == types.Water && low:
		return 1.5
	case b.hasAbility(creatures.Overgrow) && skill.Element == types.Grass && low:
		return 1.5
	}
	return 1.0
}

// defensiveAbilityMod implements IronHide (physical) and elemental immunities are
// handled in DualEffectiveness/type checks at the turn layer for Levitate/Insulate.
func defensiveAbilityMod(b *Battler, skill *creatures.Skill) float64 {
	if b.hasAbility(creatures.IronHide) && skill.Class == types.Physical {
		return 0.75
	}
	return 1.0
}
