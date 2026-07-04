package creatures

import "github.com/aurelia/beastbound/pkg/types"

// natureMult returns the multiplier this nature applies to the named stat.
func natureMult(n Nature, stat string) float64 {
	switch {
	case n.Up == n.Down:
		return 1.0 // neutral nature
	case stat == n.Up:
		return 1.1
	case stat == n.Down:
		return 0.9
	default:
		return 1.0
	}
}

// ComputeStats derives the in-battle stat block from base/IV/EV/level/nature.
// Formula mirrors the genre standard so balance intuition transfers, with the
// EV term divided by 4 and HP using a +level+10 constant.
//
//	HP    = floor((2*Base + IV + floor(EV/4)) * Level / 100) + Level + 10
//	Other = floor( (floor((2*Base + IV + floor(EV/4)) * Level / 100) + 5) * nature )
func ComputeStats(sp *Species, inst *Instance) types.Stats {
	lvl := inst.Level
	calc := func(base, iv, ev int, isHP bool, statKey string) int {
		core := (2*base + iv + ev/4) * lvl / 100
		if isHP {
			return core + lvl + 10
		}
		return int(float64(core+5) * natureMult(inst.Nature, statKey))
	}
	return types.Stats{
		HP:        calc(sp.Base.HP, inst.IVs.HP, inst.EVs.HP, true, "hp"),
		Attack:    calc(sp.Base.Attack, inst.IVs.Attack, inst.EVs.Attack, false, "attack"),
		Defense:   calc(sp.Base.Defense, inst.IVs.Defense, inst.EVs.Defense, false, "defense"),
		SpAttack:  calc(sp.Base.SpAttack, inst.IVs.SpAttack, inst.EVs.SpAttack, false, "sp_attack"),
		SpDefense: calc(sp.Base.SpDefense, inst.IVs.SpDefense, inst.EVs.SpDefense, false, "sp_defense"),
		Speed:     calc(sp.Base.Speed, inst.IVs.Speed, inst.EVs.Speed, false, "speed"),
	}
}

// UniformStats returns a Stats block with every field set to v. Handy for
// building test/demo instances with max-roll IVs (v=31).
func UniformStats(v int) types.Stats {
	return types.Stats{HP: v, Attack: v, Defense: v, SpAttack: v, SpDefense: v, Speed: v}
}

// XPForLevel uses the medium-fast cubic curve: level^3 XP to reach a level.
func XPForLevel(level int) int { return level * level * level }

// LevelForXP inverts the curve (capped at 100).
func LevelForXP(xp int) int {
	lvl := 1
	for lvl < 100 && XPForLevel(lvl+1) <= xp {
		lvl++
	}
	return lvl
}
