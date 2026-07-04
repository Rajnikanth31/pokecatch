package engine

import (
	"github.com/aurelia/beastbound/pkg/creatures"
	"github.com/aurelia/beastbound/pkg/types"
)

// Battler is a creature's mutable state inside a single battle. It is created
// from a (Species, Instance) pair at battle start and discarded at the end —
// nothing here is persisted directly; only the resulting XP/HP deltas are.
type Battler struct {
	Species *creatures.Species
	Inst    *creatures.Instance

	MaxHP     int
	CurHP     int
	Stats     types.Stats // computed, level-scaled base battle stats
	Status    types.StatusCondition
	StatusCtr int // remaining sleep turns / toxic counter

	// stages are the -6..+6 boost levels applied on top of base stats.
	stages stageBlock
	// cooldowns[i] is turns remaining before SkillIDs[i] is usable again.
	cooldowns [4]int8
	// flags reset every turn (flinch) or on switch.
	flinched bool
}

type stageBlock struct {
	atk, def, spa, spd, spe, acc, eva int8
}

// NewBattler builds a battle-ready instance from static + dynamic data.
func NewBattler(sp *creatures.Species, inst *creatures.Instance) *Battler {
	stats := creatures.ComputeStats(sp, inst)
	cur := inst.CurrentHP
	if cur <= 0 || cur > stats.HP {
		cur = stats.HP // fresh battler enters at full unless mid-run HP supplied
	}
	return &Battler{
		Species: sp,
		Inst:    inst,
		MaxHP:   stats.HP,
		CurHP:   cur,
		Stats:   stats,
		Status:  inst.Status,
	}
}

// Fainted reports whether this battler is out of the fight.
func (b *Battler) Fainted() bool { return b.CurHP <= 0 }

// stageMultiplier converts a -6..+6 stage to the canonical multiplier. Attack-
// style stats use 2/(2-s) for negatives; accuracy/evasion use a 3/3 curve.
func stageMultiplier(stage int8) float64 {
	s := int(stage)
	if s >= 0 {
		return float64(2+s) / 2.0
	}
	return 2.0 / float64(2-s)
}

func accStageMultiplier(stage int8) float64 {
	s := int(stage)
	if s >= 0 {
		return float64(3+s) / 3.0
	}
	return 3.0 / float64(3-s)
}

// effAttack / effDefense return the stage-adjusted offensive/defensive stat for a
// given damage class. Burn halves physical attack, matching genre convention and
// giving the status real weight against physical attackers.
func (b *Battler) effAttack(class types.DamageClass) float64 {
	if class == types.Special {
		return float64(b.Stats.SpAttack) * stageMultiplier(b.stages.spa)
	}
	atk := float64(b.Stats.Attack) * stageMultiplier(b.stages.atk)
	if b.Status == types.Burn {
		atk *= 0.5
	}
	return atk
}

func (b *Battler) effDefense(class types.DamageClass) float64 {
	if class == types.Special {
		return float64(b.Stats.SpDefense) * stageMultiplier(b.stages.spd)
	}
	return float64(b.Stats.Defense) * stageMultiplier(b.stages.def)
}

// effSpeed applies the speed stage and the paralysis 50% speed cut.
func (b *Battler) effSpeed() float64 {
	spe := float64(b.Stats.Speed) * stageMultiplier(b.stages.spe)
	if b.Status == types.Paralysis {
		spe *= 0.5
	}
	return spe
}

// applyStage clamps a stage delta into [-6,6] and returns whether it changed.
func clampStage(cur, delta int8) int8 {
	v := cur + delta
	if v > 6 {
		v = 6
	}
	if v < -6 {
		v = -6
	}
	return v
}

// hasAbility reports whether the battler's chosen ability matches.
func (b *Battler) hasAbility(a creatures.PassiveAbility) bool {
	return b.Inst.Ability == a
}

// hasElement reports whether either of the battler's types matches.
func (b *Battler) hasElement(e types.Element) bool {
	return b.Species.Element1 == e || b.Species.Element2 == e
}
