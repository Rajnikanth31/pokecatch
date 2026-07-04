# 00 — Engineering decisions & tradeoffs

This document is the "why." Every major technology choice is recorded with the
alternatives considered and the tradeoff accepted, so future maintainers can tell
which decisions are load-bearing and which are reversible.

## Game engine: Godot 4 (chosen) vs Unity vs Unreal

The hard constraint is **one codebase shipping to PC + Mobile + Web**. That single
requirement does most of the deciding.

| Criterion | Godot 4 | Unity | Unreal 5 |
|---|---|---|---|
| Web (WebGL/WASM) export | First-class, small builds | Supported but heavy, deprecating WebGL path | Effectively unviable for web |
| Mobile 60 FPS for stylized 2.5D | Excellent, lightweight runtime | Excellent, mature | Overkill, large binaries |
| Licensing / royalties | MIT, **zero royalty/seat cost** | Per-seat + runtime-fee history (trust risk) | 5% royalty over threshold |
| 2.5D pipeline | Native 2D + 3D in one scene tree | Good | 3D-first, 2.5D is swimming upstream |
| Team ramp & iteration speed | Fast, small editor | Fast, huge talent pool | Steeper (C++/Blueprints) |
| Raw 3D fidelity | Good | Very good | Best-in-class |

**Decision: Godot 4.** It is the only option that satisfies "PC + Mobile + Web from
one project" cleanly, carries no royalty/seat cost (material for a commercial title's
margins), and its 2.5D model fits a stylized monster-catcher. We give up Unreal's
top-tier 3D fidelity and Unity's larger hiring pool — acceptable for this art
direction. **Reversibility:** medium. The battle engine and all backend are engine-
agnostic Go; only the `client-godot/` layer is engine-bound.

## Backend language: Go (chosen) vs Node.js vs Python

The realtime authoritative battle server is the demanding component: thousands of
concurrent matches, each a small state machine, with a **p99 turn-resolution budget
under 50 ms**.

- **Go (chosen):** goroutine-per-match is a natural fit; one match = one cheap
  stack, no callback gymnastics. Static binaries → distroless images → fast cold
  start for Agones game-server scaling. GC is concurrent and pauses stay well under
  budget at our allocation profile (the engine core is deliberately allocation-light
  and dependency-free). Strong stdlib for HTTP/WS, first-class in the k8s ecosystem.
- **Node.js:** great iteration speed and shared types with the web client, but a
  single-threaded event loop plus GC make tail latency for CPU-bound turn resolution
  harder to bound; you end up sharding processes anyway.
- **Python:** best for the *tooling* (balance simulation, ML training) and we use it
  there, but the GIL and per-op overhead make it the wrong choice for the hot battle
  loop.

**Decision: Go for all services; Python for offline ML/analysis tooling.**

## Cloud: GCP (chosen) vs AWS vs Azure

- **GCP (chosen):** **Agones** (open-source, CNCF) is purpose-built game-server
  orchestration on GKE — fleet management, allocation, and drain semantics for exactly
  our "in-memory authoritative match" model. Global anycast L7 LB simplifies low-latency
  edge routing. BigQuery is the strongest managed analytics warehouse for our
  event-stream KPIs. Spanner is available if we ever outgrow single-region Postgres.
- **AWS:** most mature overall and GameLift exists, but GameLift is more opinionated
  and proprietary than Agones; we prefer the portable, open option.
- **Azure:** PlayFab is attractive for LiveOps, but its game-server fleet tooling and
  analytics are a weaker fit than Agones + BigQuery.

**Decision: GCP (GKE + Agones + BigQuery).** Tradeoff: smaller third-party ops talent
pool than AWS. **Reversibility:** the cluster workloads are vanilla Kubernetes +
Agones CRDs, portable to EKS/AKS; managed-service bindings (BigQuery, Cloud SQL) are
the lock-in points and are isolated behind repository interfaces.

## Database: polyglot persistence

Chosen per access pattern rather than one store for everything:

- **PostgreSQL — system of record.** Accounts, the creature collection, the inventory
  *ledger*, trades, match results. These have invariants (no item dupes, single
  ownership) that demand transactions and FK constraints. Cloud SQL HA with read
  replicas; the collection table is the dupe-attack surface, so trades are two-phase
  and currency is an append-only ledger with a unique receipt id.
- **Redis — realtime tier.** Sessions, presence, the matchmaking sorted-set pool,
  leaderboards (`ZREVRANK`), sticky battle routing, rate limiting. Ephemeral and
  reconstructable; AOF `everysec` is sufficient durability. See `db/redis_keys.md`.
- **BigQuery — analytics.** Append-only event firehose (catches, battles, funnels,
  spend) for KPIs and ML features. Kept off the OLTP path entirely.
- **Object storage (GCS) + CDN — assets and battle replays.** Replays are just the
  match seed + per-turn intents, so they are tiny and re-simulated on demand.

**Why not a single document DB (e.g. Mongo) for everything:** the economy invariants
are the whole ballgame for a trading game; we are not giving up transactional
integrity to avoid a join.

## Cross-cutting principle: server authority + determinism

The most important architectural decision is that **the server simulates every battle
as a pure function of `(seed, intents)`** (`engine.NewRNG`, `engine.Battle.Digest`).
This one choice cascades into:

- **Anti-cheat** — the server can re-simulate any match from its stored seed + intents
  and compare the resulting `Digest`; a tampered client cannot forge an outcome.
- **Replays & spectating** — a replay is just the seed and intent log (kilobytes).
- **Bandwidth** — clients receive event deltas, not state dumps, during play.
- **Testability** — the engine is unit-tested as a pure function.

The cost is that clients cannot run "trusted" local simulation for offline PvP; PvE is
client-predicted then server-reconciled. We accept that for a competitive title.
