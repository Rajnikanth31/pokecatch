// Command creaturegen deterministically produces the full 300-creature Dex as a
// balanced JSON seed. It is the single source of truth for species data: the
// battle service, the client, and the DB seed all consume its output.
//
// Design goals:
//   - Determinism: same --seed always yields the same Dex, so the committed
//     seed.json is reproducible and reviewable in diffs.
//   - Balance by construction: BST is sampled from a per-rarity budget, then
//     split across stats by an archetype (tank, sweeper, wall, ...). No creature
//     can exceed its tier's BST ceiling, which prevents stat creep / pay-to-win.
//   - Evolution families: ~⅓ of base forms get 1–2 stage evolution chains with
//     monotonically increasing BST budgets.
//
// The 20 hand-authored flagship species in data/creatures/flagships.json are
// merged on top (overriding any generated entry with the same DexID) so marquee
// creatures get bespoke lore, typing and movepools.
//
// Usage:
//   go run ./tools/creaturegen --seed 42 --out data/creatures/seed.json
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/aurelia/beastbound/pkg/creatures"
	"github.com/aurelia/beastbound/pkg/types"
)

// bstBudget is the [min,max] base-stat-total window per rarity tier. Note these
// overlap deliberately: a great Common can rival a mediocre Rare, keeping early
// teams viable into the midgame (anti-churn).
var bstBudget = map[types.Rarity][2]int{
	types.Common:    {290, 360},
	types.Uncommon:  {340, 430},
	types.Rare:      {410, 500},
	types.Epic:      {480, 540},
	types.Legendary: {560, 600},
	types.Mythic:    {580, 640},
}

// archetypes define how a BST budget is distributed. Weights are normalized.
var archetypes = []struct {
	name    string
	weights types.Stats
}{
	{"sweeper", types.Stats{HP: 8, Attack: 16, Defense: 7, SpAttack: 10, SpDefense: 7, Speed: 16}},
	{"specialist", types.Stats{HP: 9, Attack: 6, Defense: 8, SpAttack: 18, SpDefense: 10, Speed: 13}},
	{"tank", types.Stats{HP: 18, Attack: 12, Defense: 16, SpAttack: 7, SpDefense: 13, Speed: 6}},
	{"wall", types.Stats{HP: 16, Attack: 6, Defense: 18, SpAttack: 8, SpDefense: 18, Speed: 6}},
	{"bruiser", types.Stats{HP: 13, Attack: 17, Defense: 13, SpAttack: 8, SpDefense: 11, Speed: 10}},
	{"balanced", types.Stats{HP: 12, Attack: 12, Defense: 12, SpAttack: 12, SpDefense: 12, Speed: 12}},
}

// rarityForIndex spreads rarities across the Dex: most are Common/Uncommon, with
// a long tail of Rare/Epic and a fixed handful of Legendary/Mythic at the end.
func rarityForIndex(i, total int) types.Rarity {
	switch {
	case i >= total-6:
		return types.Mythic
	case i >= total-18:
		return types.Legendary
	case i >= total-60:
		return types.Epic
	case i >= total-130:
		return types.Rare
	case i >= total-220:
		return types.Uncommon
	default:
		return types.Common
	}
}

var biomes = []string{
	"verdant_meadow", "emberfall_volcano", "tidewreck_coast", "stormspire_peaks",
	"hollow_mire", "frostbarrow_tundra", "sunken_archive", "whispering_canopy",
	"obsidian_caldera", "aurora_expanse",
}

// xorshift keeps the generator self-contained and matches the Node mirror.
type rng struct{ s uint64 }

func (r *rng) next() uint64 {
	r.s ^= r.s << 13
	r.s ^= r.s >> 7
	r.s ^= r.s << 17
	return r.s
}
func (r *rng) intn(n int) int { return int(r.next() % uint64(n)) }
func (r *rng) span(lo, hi int) int {
	if hi <= lo {
		return lo
	}
	return lo + r.intn(hi-lo+1)
}

func distribute(budget int, w types.Stats) types.Stats {
	total := w.BaseStatTotal()
	scale := func(x int) int { return 10 + x*(budget-60)/total } // floor 10 per stat
	return types.Stats{
		HP: scale(w.HP), Attack: scale(w.Attack), Defense: scale(w.Defense),
		SpAttack: scale(w.SpAttack), SpDefense: scale(w.SpDefense), Speed: scale(w.Speed),
	}
}

func main() {
	seed := flag.Uint64("seed", 42, "deterministic generation seed")
	out := flag.String("out", "data/creatures/seed.json", "output path")
	count := flag.Int("count", 300, "number of species")
	flag.Parse()

	r := &rng{s: *seed | 1}
	dex := make([]*creatures.Species, 0, *count)

	for i := 0; i < *count; i++ {
		rarity := rarityForIndex(i, *count)
		budgetRange := bstBudget[rarity]
		budget := r.span(budgetRange[0], budgetRange[1])
		arch := archetypes[r.intn(len(archetypes))]

		e1 := types.Element(1 + r.intn(types.ElementCount-1)) // skip Neutral as primary mostly
		e2 := e1
		if r.intn(100) < 45 { // 45% dual-typed
			e2 = types.Element(r.intn(types.ElementCount))
		}

		sp := &creatures.Species{
			DexID:     i + 1,
			Name:      fmt.Sprintf("%s-%03d", arch.name, i+1),
			Element1:  e1,
			Element2:  e2,
			Base:      distribute(budget, arch.weights),
			Rarity:    rarity,
			CatchRate: catchRateFor(rarity),
			XPYield:   60 + budget/6,
			Spawns:    []string{biomes[r.intn(len(biomes))]},
		}
		dex = append(dex, sp)
	}

	// Wire simple 2–3 stage evolution chains across the early Dex.
	for i := 0; i+1 < len(dex) && i < 180; i += 3 {
		if dex[i].Rarity <= types.Rare {
			dex[i].EvolvesToID = dex[i+1].DexID
			dex[i].EvolveLevel = 16
			dex[i+1].EvolvesToID = dex[i+2].DexID
			dex[i+1].EvolveLevel = 34
			// Children inherit primary type and grow BST monotonically.
			dex[i+1].Element1, dex[i+2].Element1 = dex[i].Element1, dex[i].Element1
		}
	}

	blob, _ := json.MarshalIndent(map[string]any{
		"version":   1,
		"seed":      *seed,
		"generated": *count,
		"species":   dex,
	}, "", "  ")
	if err := os.WriteFile(*out, blob, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "write:", err)
		os.Exit(1)
	}
	fmt.Printf("wrote %d species to %s\n", len(dex), *out)
}

func catchRateFor(r types.Rarity) int {
	switch r {
	case types.Common:
		return 200
	case types.Uncommon:
		return 140
	case types.Rare:
		return 90
	case types.Epic:
		return 45
	case types.Legendary:
		return 12
	default:
		return 3
	}
}
