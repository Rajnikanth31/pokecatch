# 02 — Game Design Document

## Core gameplay loop

```
Explore  ->  Catch  ->  Train  ->  Evolve  ->  Battle  ->  Upgrade  ->  Compete
   ^                                                                        |
   +------------------------------------------------------------------------+
```

- **Explore:** traverse biome-gated shards; spawns are biome- and time-of-day-weighted.
- **Catch:** weaken then attempt capture; success probability is a function of catch
  rate, remaining HP fraction, status, and ball tier (formula below).
- **Train:** battles grant XP (cubic curve, `XPForLevel = level³`) and EVs that
  permanently shape stats; natures and ability choice add build identity.
- **Evolve:** level- or item-gated transforms that raise the BST budget and may add a
  second type, with branching final forms for flagships.
- **Battle:** turn-based tactical combat (engine spec below).
- **Upgrade:** EV redistribution, skill relearning, held items (utility only),
  bond/affection unlocks.
- **Compete:** ranked PvP, raids, towers, seasonal events.

## Creature system

### Catalog: 300 Animae

Generated deterministically by `tools/creaturegen` (canonical Go) / `generate.mjs`
(runnable mirror) into `data/creatures/seed.json`, with **20 hand-authored flagships**
in `flagships.json` overlaid by Dex ID. Each species defines: name, one or two of the
**12 elements** (Neutral, Fire, Water, Grass, Electric, Earth, Air, Ice, Toxin, Mind,
Spectre, Metal), the six base stats (HP, Attack, Defense, Sp.Attack, Sp.Defense,
Speed), evolution target + level, learnset, possible passive abilities, rarity, catch
rate, XP yield, and spawn biomes. Flagships additionally carry lore and signature
skills.

### Rarity & BST budgets (anti-power-creep guardrail)

The generator samples each creature's **Base Stat Total** from a per-rarity window and
splits it by an archetype template (sweeper/specialist/tank/wall/bruiser/balanced).
Critically, the windows **overlap** so a strong Common stays relevant into the midgame,
and there is a hard ceiling per tier so no creature — purchasable or otherwise — can
exceed its bracket. Verified at generation time (`BST ceiling violations: 0`).

| Rarity | BST window | Catch rate | Role |
|---|---|---|---|
| Common | 290–360 | 200 | early team, evolution fodder |
| Uncommon | 340–430 | 140 | midgame staples |
| Rare | 410–500 | 90 | strong fully-evolved |
| Epic | 480–540 | 45 | pseudo-legendary tier |
| Legendary | 560–600 | 12 | story/box legends |
| Mythic | 580–640 | 3 | event/superboss |

### Weakness / resistance matrix

The 12×12 type-effectiveness matrix is the source of metagame structure (hand-tuned in
`pkg/types/types.go`, not auto-generated). Multipliers are 0 (immune), 0.5 (resisted),
1 (neutral), 2 (super-effective); dual types multiply, so 0.25× and 4× exist. Examples:
Water→Fire 2×, Fire→Water 0.5×, Electric→Earth 0× (immune), Grass→Water/Earth 4×,
Spectre→Neutral 0×. Status immunities are also type-based (Fire can't burn, Ice can't
freeze, Toxin/Metal can't be poisoned, Electric can't be paralyzed).

### Balancing formulas

Stat computation (`pkg/creatures/stats.go`), genre-standard so balance intuition
transfers:

```
core   = floor( (2*Base + IV + floor(EV/4)) * Level / 100 )
HP     = core + Level + 10
Other  = floor( (core + 5) * natureMultiplier )      // nature ∈ {0.9, 1.0, 1.1}
```

IVs are 0–31 (rolled at capture), EVs 0–252 per stat capped at 510 total, natures give
+10%/−10% to one stat pair. **Verified:** base 100 / L100 / max IV → HP 341, other 236;
Adamant (+Atk/−SpA) → Atk 259, SpA 212. See `engine_test.go` + `verify_engine.js`.

## Battle system

Turn-based, single-active (6v6 with switching), server-authoritative. Modes share one
engine: **PvE** (wild + trainer), **PvP** (casual + ranked), **Sanctum/gym** (puzzle-
gated themed teams), **boss/raid** (multi-phase, acts-multiple-times), **ranked**
(Glicko-2 ladder).

### Damage formula (`services/battle/internal/engine/damage.go`)

```
base   = floor( (floor(2*L/5) + 2) * Power * A/D / 50 ) + 2
damage = round( base * STAB * TypeEff * Crit * AbilityMods * Spread )
```

- **A/D:** attacker's effective Attack/Sp.Attack vs defender's Defense/Sp.Defense,
  including stat-stage multipliers and **Burn halves physical Attack**.
- **STAB:** 1.5× when the move's element matches the user's type (2.0× with the
  *Adaptable* ability).
- **TypeEff:** dual-type product from the matrix (0 / .25 / .5 / 1 / 2 / 4).
- **Crit:** 1.5×, and crits ignore the defender's positive defensive stages and the
  attacker's negative offensive stages.
- **Spread:** uniform [0.85, 1.00], applied last, from the match RNG.
- Any connecting damaging move deals **≥ 1**; immune (0×) deals exactly 0.

### Critical-hit system

Tiered: base 4%, then 12% / 50% / 100% as crit stages stack (from high-crit moves or
buffs). Tier table in `damage.go` (`critChanceByStage`).

### Status effects (`status.go`)

Non-volatile, one at a time: **Burn** (½ physical Atk + 1/8 max HP/turn), **Poison**
(1/8 max HP/turn), **Toxic** (escalating n/16 each turn), **Freeze** (can't act, 20%
thaw/turn), **Sleep** (1–3 turns, RNG-rolled), **Paralysis** (½ Speed, 25% can't-move).
Type immunities enforced on application. Volatile **flinch** clears each turn.

### Buff/debuff engine

Seven stat stages each clamped to [−6, +6]: Attack, Defense, Sp.Atk, Sp.Def, Speed,
Accuracy, Evasion. Standard multiplier curves (`2/(2±s)` for combat stats, `3/(3±s)` for
accuracy/evasion). Applied data-drivenly via a skill's `SkillEffect.StatChanges`, so
designers add buff/debuff moves without engine changes.

### Skill cooldowns & energy

Two independent throttles so designers can pace power: **PP** (per-skill use budget,
genre-standard) and **cooldowns** (turns before reuse, for signature/ultimate moves —
e.g. Solar Pyre power 120, cooldown 2). The session layer can additionally impose a
shared **energy** budget for alternate rulesets (Frontier towers). Priority moves
(`Priority` field) act before the speed check.

### Battle AI

Layered (full detail in `docs/04-ai-systems.md`): expectimax over the legal action set
to depth 2 with a heuristic eval (type advantage, KO potential, hazard/status value,
switch value), wrapped by a behavior tree that injects "personality" (aggressive
Sanctum Keeper vs defensive wall trainer) and a difficulty knob that degrades the search
(adds noise / reduces depth) for accessible PvE.

### Anti-cheat (battle-specific; full model in `docs/07-security.md`)

The decisive property: **the server is the only simulator.** Clients submit *intents*
only (`ClientMessage` → move slot or switch). The server owns the per-match seed
(crypto/rand, never sent until the match ends), runs `ResolveTurn`, and computes a
`Digest` (FNV-1a over RNG state + HP + status + active indices) each turn. Stored seed +
per-turn intents (`match_actions`) make every match **re-simulatable**, so any claimed
outcome is verifiable offline. Illegal/late actions are replaced with a safe default,
never trusted. Information cheats are blunted by sending **redacted** opponent state
(`BattlerView` exposes HP/level/status, never IVs/EVs/exact stats).
