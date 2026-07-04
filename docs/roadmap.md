# Development Roadmap

~18 months to global launch, milestone-gated (dates are relative, gates are on criteria
not calendar). Team assumed ~25–40 across eng/design/art/QA/LiveOps.

## M0 — Foundations (months 0–2)
- Repo, CI/CD, environments, observability skeleton.
- **Battle engine core** (this repo: types, stats, damage, status, turn loop, RNG,
  digest) + unit tests + reference verifiers. ✅ delivered here.
- Creature data model + **generator** for 300 + 20 flagships. ✅ delivered here.
- Postgres schema + Redis keyspace. ✅ delivered here.

## M1 — Vertical slice (months 2–5)
- Godot client: login, overworld, battle view, collection/team builder ✅ (art is
  placeholder primitives; one biome's content still to author).
- Authoritative **battle WS** ✅ (netcode session + gorilla transport + in-memory
  transport + two-bot demo). Agones fleet integration still to wire.
- Auth ✅ (argon2id, JWT, refresh rotation), Profile ✅ (collection, team, evolve,
  escrowed trades) with **Postgres persistence** ✅ and replay-based anti-cheat ✅.
  Matchmaking pairing ✅ (Redis pool wiring remaining).
- **Gate:** play start→catch→battle→win against a real server. Internal playtest fun-check.

## M2 — Core game complete (months 5–9)
- All 5 regions, main-story Acts I–II, Sanctums, evolutions, EV/nature systems.
- Ranked PvP (Glicko-2), leaderboards, friends, trading (escrowed).
- Battle AI (expectimax + behavior-tree personalities + difficulty).
- **Gate: Closed Alpha** (crash-free > 98%, loop validated).

## M3 — Content & social (months 9–12)
- Acts III–IV, endgame (raids: Gaiathos 3-phase, Chronovex superboss), guilds, towers.
- Spectator mode + replay system, tournaments service.
- Monetization (cosmetics, Battle Pass), store/payment integration.
- LiveOps tooling + first seasonal event built.
- **Gate: Closed Beta** (p99 turn < 50 ms at scale, D1 > 35%).

## M4 — Hardening (months 12–15)
- Load/chaos testing, anti-cheat adversarial pass + fraud model, accessibility pass,
  localization, platform certification (Apple/Google/Steam).
- Performance: hit 60 FPS mobile / 120 FPS PC / 60 FPS web budgets.
- **Gate: Open Beta** then **Soft Launch** (D30 > 12%, LTV:CPI trending > 1).

## M5 — Launch & LiveOps (months 15–18+)
- Global launch, marketing peak, launch ranked season.
- Establish the **quarterly season cadence** (new split, event, ruleset, cosmetics).
- Post-launch: balance patches informed by RL self-play + analytics, new creature
  batches via the generator + bespoke flagships, competitive circuit.

## Parallel tracks (continuous)
- **Art** runs ahead of eng from M0 (longest lead time; 300 creatures + biomes).
- **Balance** via offline RL self-play once the engine stabilizes (M1+).
- **Reliability** budgeted continuously; error-budget policy governs feature-vs-stability
  tradeoffs post-beta.
