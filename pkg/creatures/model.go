// Package creatures defines the static creature/species data model and the
// runtime Battler that wraps a captured individual. Static species data is
// loaded from the JSON seed produced by tools/creaturegen; runtime instances
// (the player's actual caught creatures) carry IVs/EVs/level/nature on top.
package creatures

import "github.com/aurelia/beastbound/pkg/types"

// Skill is a usable move. Power 0 + StatusOnly class means a pure utility move.
// Accuracy is 0..100; >100 means it cannot miss (used by a few signature moves).
type Skill struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Element     types.Element     `json:"element"`
	Class       types.DamageClass `json:"class"`
	Power       int               `json:"power"`
	Accuracy    int               `json:"accuracy"`
	PP          int               `json:"pp"`        // uses before rest
	Priority    int8              `json:"priority"`  // higher acts first regardless of speed
	CritStage   int8              `json:"crit_stage"`// added to base crit tier
	Cooldown    int8              `json:"cooldown"`  // turns before reuse (0 = none)
	// Effect describes a rider applied on hit; nil for plain damage moves.
	Effect *SkillEffect `json:"effect,omitempty"`
}

// SkillEffect encodes status/stat riders in a data-driven way so designers add
// moves without touching engine code.
type SkillEffect struct {
	Status      types.StatusCondition `json:"status,omitempty"`
	StatusChance int                  `json:"status_chance,omitempty"` // 0..100
	// StatChanges maps a stat name ("attack","speed",...) to stage delta (-6..+6).
	StatChanges  map[string]int8      `json:"stat_changes,omitempty"`
	TargetSelf   bool                 `json:"target_self,omitempty"`
	// HealFrac heals the user by this fraction of damage dealt (drain moves) or
	// of max HP when no damage (recovery moves).
	HealFrac     float64              `json:"heal_frac,omitempty"`
	// RecoilFrac inflicts this fraction of damage dealt back on the user.
	RecoilFrac   float64              `json:"recoil_frac,omitempty"`
}

// PassiveAbility is an always-on trait checked by engine hooks. Implemented as
// an enum the engine switches on, keeping hot-path logic allocation-free.
type PassiveAbility uint8

const (
	NoAbility       PassiveAbility = iota
	Blaze                          // +50% Fire dmg under 1/3 HP
	Torrent                        // +50% Water dmg under 1/3 HP
	Overgrow                       // +50% Grass dmg under 1/3 HP
	Static                         // 30% paralyze on contact
	Levitate                       // immune to Earth
	Regenerator                    // heal 1/16 max HP each turn
	IronHide                       // -25% physical damage taken
	Insulate                       // immune to Electric, +Speed when hit by it
	Pressure                       // foe skills cost +1 PP
	Adaptable                      // STAB is 2.0x instead of 1.5x
)

// Species is immutable design data shared by every individual of a kind.
type Species struct {
	DexID       int              `json:"dex_id"`
	Name        string           `json:"name"`
	Element1    types.Element    `json:"element1"`
	Element2    types.Element    `json:"element2"` // == Element1 if mono-typed
	Base        types.Stats      `json:"base"`
	Rarity      types.Rarity     `json:"rarity"`
	Abilities   []PassiveAbility `json:"abilities"`   // possible abilities; instance picks one
	Learnset    []LearnEntry     `json:"learnset"`
	EvolvesToID int              `json:"evolves_to_id"` // 0 = final form
	EvolveLevel int              `json:"evolve_level"`
	CatchRate   int              `json:"catch_rate"`    // 1..255, higher = easier
	XPYield     int              `json:"xp_yield"`
	Spawns      []string         `json:"spawns"`        // biome/region ids
	Lore        string           `json:"lore,omitempty"`
}

// LearnEntry binds a skill id to the level it unlocks at.
type LearnEntry struct {
	Level   int    `json:"level"`
	SkillID string `json:"skill_id"`
}

// Nature applies a +10%/-10% multiplier to one stat pair, adding build diversity
// without new art. Stored as two stat keys; neutral natures repeat the same key.
type Nature struct {
	Name string `json:"name"`
	Up   string `json:"up"`
	Down string `json:"down"`
}

// Instance is a concrete caught creature owned by a player. This is what gets
// persisted per-player and what the battle engine instantiates a Battler from.
type Instance struct {
	InstanceID string              `json:"instance_id"`
	DexID      int                 `json:"dex_id"`
	Nickname   string              `json:"nickname,omitempty"`
	Level      int                 `json:"level"`
	XP         int                 `json:"xp"`
	IVs        types.Stats         `json:"ivs"` // 0..31 per stat, rolled at capture
	EVs        types.Stats         `json:"evs"` // 0..252 per stat, 510 total cap
	Nature     Nature              `json:"nature"`
	Ability    PassiveAbility      `json:"ability"`
	SkillIDs   [4]string           `json:"skill_ids"`
	Status     types.StatusCondition `json:"status"`
	CurrentHP  int                 `json:"current_hp"`
}
