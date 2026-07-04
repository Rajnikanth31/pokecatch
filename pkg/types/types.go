// Package types defines the shared domain vocabulary for Aurelia: Beastbound.
// Elemental types and the resistance matrix live here because both the battle
// engine and the creature generator depend on them. Keeping this package free of
// any I/O or service dependency lets it compile into the client (via gomobile or
// a transpiled mirror) and the authoritative server unchanged.
package types

// Element is one of the 12 elemental affinities. Values are stable and used as
// indices into the effectiveness matrix, so DO NOT reorder them without also
// migrating the matrix and any persisted data.
type Element uint8

const (
	Neutral Element = iota
	Fire
	Water
	Grass
	Electric
	Earth
	Air
	Ice
	Toxin
	Mind
	Spectre
	Metal
	elementCount // sentinel; keep last
)

// ElementCount is the number of real elements (excludes the sentinel).
const ElementCount = int(elementCount)

var elementNames = [elementCount]string{
	"Neutral", "Fire", "Water", "Grass", "Electric", "Earth",
	"Air", "Ice", "Toxin", "Mind", "Spectre", "Metal",
}

func (e Element) String() string {
	if int(e) >= ElementCount {
		return "Unknown"
	}
	return elementNames[e]
}

// effectiveness[attacker][defender] is the STAB-independent multiplier applied
// to type matchups. 2.0 = super effective, 0.5 = resisted, 0.0 = immune, 1.0 =
// neutral. The matrix is intentionally hand-tuned (not auto-generated) so the
// metagame has deliberate rock-paper-scissors structure rather than uniform noise.
//
// Rows = attacking element, Columns = defending element, ordered as the consts
// above (Neutral=0 ... Metal=11).
var effectiveness = [ElementCount][ElementCount]float64{
	//             Neu  Fir  Wat  Gra  Ele  Ear  Air  Ice  Tox  Min  Spe  Met
	/*Neutral*/ {1.0, 1.0, 1.0, 1.0, 1.0, 1.0, 1.0, 1.0, 1.0, 1.0, 0.5, 0.5},
	/*Fire   */ {1.0, 0.5, 0.5, 2.0, 1.0, 1.0, 1.0, 2.0, 1.0, 1.0, 1.0, 2.0},
	/*Water  */ {1.0, 2.0, 0.5, 0.5, 1.0, 2.0, 1.0, 1.0, 1.0, 1.0, 1.0, 1.0},
	/*Grass  */ {1.0, 0.5, 2.0, 0.5, 1.0, 2.0, 0.5, 1.0, 0.5, 1.0, 1.0, 0.5},
	/*Electric*/{1.0, 1.0, 2.0, 0.5, 0.5, 0.0, 2.0, 1.0, 1.0, 1.0, 1.0, 1.0},
	/*Earth  */ {1.0, 2.0, 0.5, 0.5, 2.0, 1.0, 0.0, 1.0, 2.0, 1.0, 1.0, 2.0},
	/*Air    */ {1.0, 1.0, 1.0, 2.0, 0.5, 1.0, 1.0, 1.0, 1.0, 1.0, 1.0, 0.5},
	/*Ice    */ {1.0, 0.5, 0.5, 2.0, 1.0, 2.0, 2.0, 0.5, 1.0, 1.0, 1.0, 0.5},
	/*Toxin  */ {1.0, 1.0, 1.0, 2.0, 1.0, 0.5, 1.0, 1.0, 0.5, 1.0, 0.5, 0.0},
	/*Mind   */ {1.0, 1.0, 1.0, 1.0, 1.0, 1.0, 1.0, 1.0, 2.0, 0.5, 0.0, 1.0},
	/*Spectre*/ {0.0, 1.0, 1.0, 1.0, 1.0, 1.0, 1.0, 1.0, 1.0, 2.0, 2.0, 1.0},
	/*Metal  */ {1.0, 0.5, 0.5, 1.0, 0.5, 1.0, 1.0, 2.0, 1.0, 1.0, 1.0, 0.5},
}

// Effectiveness returns the matchup multiplier of an attacking element against a
// single defending element.
func Effectiveness(attacker, defender Element) float64 {
	if int(attacker) >= ElementCount || int(defender) >= ElementCount {
		return 1.0
	}
	return effectiveness[attacker][defender]
}

// DualEffectiveness multiplies matchups against a (possibly mono-typed) defender.
// A mono-typed creature passes Neutral as its second element with no penalty,
// because Neutral defends at 1.0 against every offensive element.
func DualEffectiveness(attacker, def1, def2 Element) float64 {
	m := Effectiveness(attacker, def1)
	if def2 != def1 {
		m *= Effectiveness(attacker, def2)
	}
	return m
}

// Stats is the canonical six-stat block shared by base stats, IVs, EVs and the
// computed in-battle stats. Using one struct keeps stat math generic.
type Stats struct {
	HP        int `json:"hp"`
	Attack    int `json:"attack"`
	Defense   int `json:"defense"`
	SpAttack  int `json:"sp_attack"`
	SpDefense int `json:"sp_defense"`
	Speed     int `json:"speed"`
}

// BaseStatTotal (BST) is the standard balance lever. Generator clamps BST per
// rarity tier so no single creature dominates its bracket.
func (s Stats) BaseStatTotal() int {
	return s.HP + s.Attack + s.Defense + s.SpAttack + s.SpDefense + s.Speed
}

// Rarity drives spawn weighting, BST budget and reward economy. Higher rarity
// never means strictly stronger (anti-pay-to-win): legendaries trade raw BST for
// catch difficulty and team-slot opportunity cost.
type Rarity uint8

const (
	Common Rarity = iota
	Uncommon
	Rare
	Epic
	Legendary
	Mythic
)

func (r Rarity) String() string {
	switch r {
	case Common:
		return "Common"
	case Uncommon:
		return "Uncommon"
	case Rare:
		return "Rare"
	case Epic:
		return "Epic"
	case Legendary:
		return "Legendary"
	case Mythic:
		return "Mythic"
	default:
		return "Unknown"
	}
}

// DamageClass selects which offensive/defensive stat pair a skill keys off.
type DamageClass uint8

const (
	Physical DamageClass = iota // Attack vs Defense
	Special                     // SpAttack vs SpDefense
	StatusOnly                  // deals no direct damage
)

// StatusCondition is a persistent (non-volatile) ailment. At most one is active
// per creature at a time, matching genre convention and simplifying state.
type StatusCondition uint8

const (
	None StatusCondition = iota
	Burn
	Poison
	Freeze
	Sleep
	Paralysis
	Toxic // escalating poison
)

func (s StatusCondition) String() string {
	switch s {
	case Burn:
		return "Burn"
	case Poison:
		return "Poison"
	case Freeze:
		return "Freeze"
	case Sleep:
		return "Sleep"
	case Paralysis:
		return "Paralysis"
	case Toxic:
		return "Toxic"
	default:
		return "None"
	}
}
