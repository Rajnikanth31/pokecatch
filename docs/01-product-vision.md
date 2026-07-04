# 01 — Product Vision

## Premise & lore

In the world of **Aurelia**, a cataclysm called the **Sundering** shattered a single
supercontinent into floating shards held aloft by ambient energy. The creatures of
Aurelia — called **Animae** (sing. *Anima*) — are living concentrations of that energy;
catching and bonding with one lets a person draw on the shard's power. Players are
**Wardens**, travelers who restabilize the drifting shards by rebuilding the bonds
between people and Animae that the Sundering severed.

The emotional spine of the story is **restoration, not conquest**: you are not
collecting weapons, you are healing a broken world by forming relationships. The
antagonist faction, the **Hollow Concord**, believes Animae should be drained for raw
power rather than bonded with — a deliberate thematic foil to the player's path that
also motivates the anti-pay-to-win economy (power-through-relationship, not purchase).

## World map

Five regions across the shard-archipelago, each a biome cluster with towns, wilds,
caves, and a Sanctum (gym-equivalent):

1. **Verdant Reach** — starting region. Meadows, the town of **Hearthvale**, the
   Whispering Canopy forest, beginner caves. Tutorializes catching and bonding.
2. **Emberfall** — volcanic shard. **Cinderhold** town, Obsidian Caldera (Fire/Metal
   wilds), lava-tube caves with a heat-management traversal mechanic.
3. **Tidewreck** — sunken coastal shard. **Saltmere** town, tide-gated ruins (the
   Sunken Archive endgame dungeon), water/ice wilds.
4. **Stormspire** — vertical mountain shard. **Aerie** town, lightning fields,
   gliding traversal, electric/air wilds.
5. **Aurora Expanse** — endgame frontier shard. **The Lattice** hub, the Aurora wilds
   where mythics roam, and the Chronovex superboss arena.

A connective **Skyway** (airship/glider network) unlocks fast travel between
stabilized shards, gating exploration behind story progress.

## Main storyline (act structure)

- **Act I — Bond:** choose a starter line (Pyrolt/Driblet/Sproutle), learn to catch,
  clear two Verdant Sanctums, first encounter with the Hollow Concord draining a wild
  Anima.
- **Act II — Wander:** open up Emberfall, Tidewreck, Stormspire in player-chosen order
  (non-linear; level-scaled Sanctums keep all orders viable). Recruit two companion
  NPCs whose questlines deepen the Concord conflict.
- **Act III — Sunder's Truth:** the Concord is revealed to be ex-Wardens who lost their
  Animae in the Sundering and chose power over grief. The Aurora Expanse opens.
- **Act IV — Champion:** the Lattice Tournament (the "Elite Four + Champion" beat),
  then the choice that resolves the theme — restore the Concord leader's lost bond
  rather than defeat them.

## Side quests (systemic, not just fetch)

- **Restoration contracts:** stabilize a named wild area by completing an ecological
  objective (e.g. relocate an apex Anima) — rewards habitat-specific spawns.
- **Bond trials:** per-creature affection quests that unlock a creature's hidden
  passive or signature skill.
- **Concord defectors:** moral-choice mini-arcs; choices feed a hidden reputation stat
  that gates an alternate ending.
- **Field research:** Dex-completion tasks (catch N of a type, win with a mono-team)
  that teach mechanics through play.

## NPC design

Three tiers: **Wardens** (rivals/companions with persistent teams that grow as you do),
**Sanctum Keepers** (themed bosses with puzzle-gated arenas), and **townsfolk** (quest
givers + ambient life via a daily schedule system, see AI doc). Companion NPCs use the
same battle engine and behavior trees as opponents — no special-cased AI.

## Difficulty progression

Level-scaled but floored: Sanctums scale to within a band of the player's team to keep
non-linear order viable, but never trivialize. Optional **Warden's Oath** modifiers
(set-mode, no items, level caps) give challenge-seekers depth without punishing casual
players. Endgame raids are fixed-difficulty and tuned for coordinated teams.

## Endgame content (retention past the credits)

- **Lattice Ranked PvP** — seasonal Glicko-2 ladder, the competitive core.
- **Raids** — Gaiathos (3-phase) and Chronovex (acts-twice superboss) cooperative
  encounters for guild groups.
- **Aurora hunts** — rotating mythic spawns with catch-puzzle mechanics.
- **Dex perfection + shiny hunting** — long-tail collection goals.
- **Battle Frontier–style towers** — escalating AI gauntlets with curated rulesets.

## Seasonal events (LiveOps cadence)

Quarterly themed seasons (each ~12 weeks): a new ranked split, a limited-time biome
event with event-only cosmetic Animae forms, a story vignette, and a battle ruleset.
Examples: **Emberfall Festival** (Fire spawns boosted, fireworks cosmetics), **Aurora
Convergence** (mythic hunt window). Events drive the 12-week retention spikes.

## Monetization — fair by construction

**Principle: you can buy time and expression, never power.** Nothing purchasable
affects a competitive outcome.

- **Cosmetics:** Warden outfits, Anima skins/shinies, arena themes, emotes. Primary
  revenue.
- **Battle Pass (seasonal):** free + premium tracks; premium track is cosmetics +
  convenience (XP-share, extra storage), **no stat items, no exclusive competitive
  Animae**. Earned premium currency carries the pass partly self-funding.
- **Convenience:** extra storage boxes, additional daily restoration contracts,
  account-wide cosmetics. Capped so it never becomes "pay to grind less than is fair."
- **No loot boxes for power, no stat microtransactions, no creature sales.** Premium
  currency (**Aurelium**) buys only the above and is fully ledgered (`currency_ledger`).

Ranked rewards and all competitively-relevant Animae are **earnable through play only**,
which is also why the creature catalog is generated under a strict per-rarity BST ceiling
(see GDD) — no "premium tier" of stronger creatures can exist.

## Player journey (beginner → champion)

Onboarding (first session: catch, name, win a battle, feel the bond) → guided Act I →
opening of non-linear exploration (the "I have my own adventure" moment) → first PvP
taste via casual → mastery systems (EV training, natures, team-building) surfaced
gradually → ranked ladder → endgame raids/hunts → seasonal re-engagement.

## Emotional engagement mechanics

- **Bonding/affection:** creatures you battle with develop visible affection that
  unlocks personality (idle animations, a signature move, occasional crit/endure
  procs framed as "fighting for you"). This is the core attachment loop.
- **Named loss stakes:** the Concord arc personalizes what it means to lose a bond,
  raising the emotional weight of your own team.
- **Restoration feedback:** stabilizing a shard visibly heals the world (color returns,
  music swells, NPCs resettle) — tangible progress, not a number.

## Retention mechanics

- **Daily:** restoration contracts, a daily ranked bonus, rotating wild spawns.
- **Weekly:** raid lockouts, ranked decay protection, field-research resets.
- **Seasonal:** the 12-week event cadence above.
- **Social hooks:** guilds, trading, friend battles, spectating — covered in the
  multiplayer doc; social ties are the strongest long-term retention lever and are
  intentionally front-loaded once the player finishes Act I.
