# Aurelia: Beastbound

A commercial-quality, monster-catching open-world RPG for **PC, Mobile, and Web**, with a **server-authoritative real-time PvP** backend. This repository is a production-grade *vertical slice of the architecture*: a working, tested Go battle engine and multiplayer core, a 300-creature catalog with a deterministic generator, the full backend/cloud/database design, and the design + ops documentation a studio would build the rest of the game against.

It is **not** copyrighted Pokémon assets — all creatures, types, lore, and names are original.

## What's actually here (runnable)

| Area | Path | Status |
|---|---|---|
| Shared domain (types, type-matrix) | `pkg/types` | Implemented + tested |
| Creature model, stat/XP formulas, loader | `pkg/creatures` | Implemented + tested |
| Deterministic battle engine (damage, status, turns, RNG, digest) | `services/battle/internal/engine` | Implemented + unit tested |
| Real-time PvP session + netcode protocol | `services/battle/internal/netcode` | Implemented |
| WebSocket transport (gorilla) + in-memory transport | `services/battle/internal/netcode/ws.go`, `memtransport.go` | Implemented |
| Server-side battle AI (expectimax + heuristics) | `services/battle/internal/ai` | Implemented + tested |
| End-to-end two-bot match demo (runs through the Session) | `services/battle/cmd/demo` | Runnable |
| Offline save system (versioned, checksummed, migrating) | `services/profile/internal/save.go` | Implemented + tested |
| Matchmaking (region + MMR band) | `services/matchmaking/internal` | Implemented + tested |
| API gateway (auth, rate limit, routing) | `services/gateway/internal` | Implemented |
| 300-creature generator + 20 flagships | `tools/creaturegen`, `data/creatures` | Generated + validated |
| Auth service (argon2id, HS256 JWT, refresh rotation) | `services/auth` | Implemented + tested |
| Profile: collection, team, evolve, escrowed trades | `services/profile` | Implemented + tested |
| Postgres persistence (pgx, transactional trades) | `*_postgres.go`, `pkg/db` | Implemented + integration tests |
| API gateway wired (JWT verify, rate limit, proxy) | `services/gateway` | Implemented |
| Match persistence + replay anti-cheat CLI | `.../persistence`, `tools/replay` | Implemented + tested |
| Godot 4 client (login, overworld, battle, team builder) | `client-godot/` | Runnable project |
| PostgreSQL schema + Redis keyspace | `db/` | Implemented (structure-validated) |
| Docker / Kubernetes / CI-CD / SLO alerts | `deploy/`, `.github/` | Implemented |
| Reference verifiers (engine + session + backend) | `test/reference` | Passing |

## Verification

The Go unit tests live beside the code (`*_test.go`). Because not every environment
has the Go toolchain, the core math (damage, stats, type matrix, matchmaking) is
*independently* re-implemented and asserted in `test/reference/*.js`. CI runs both
and fails on any divergence.

```bash
# math/logic parity check (no Go required)
node test/reference/verify_engine.js

# full match path: turn loop, status, faint -> forced switch, victory (no Go required)
node test/reference/verify_session.mjs

# (re)generate the balanced 300-creature seed
node tools/creaturegen/generate.mjs --seed 42 --count 300 --out data/creatures/seed.json

# full Go suite (in CI or with a local toolchain)
go test -race ./...

# run a complete two-bot PvP match end-to-end through the netcode Session
go run ./services/battle/cmd/demo
```

## Documentation map

| Phase | Doc |
|---|---|
| 0 — Engineering decisions & tradeoffs | [docs/00-overview.md](docs/00-overview.md) |
| 1 — Product vision, lore, world, monetization, retention | [docs/01-product-vision.md](docs/01-product-vision.md) |
| 2 — Game Design Document (loop, creatures, battle, balance formulas, anti-cheat) | [docs/02-gdd.md](docs/02-gdd.md) |
| 3 — Technical architecture (frontend, ECS, backend, cloud, DB) | [docs/03-architecture.md](docs/03-architecture.md) |
| 4 — AI systems (behavior trees, RL, pathfinding, fraud) | [docs/04-ai-systems.md](docs/04-ai-systems.md) |
| 5 — Graphics & art direction | [docs/05-graphics.md](docs/05-graphics.md) |
| 6 — Multiplayer systems | [docs/06-multiplayer.md](docs/06-multiplayer.md) |
| 7 — Security & anti-cheat | [docs/07-security.md](docs/07-security.md) |
| 8 — DevOps & SRE | [docs/08-devops-sre.md](docs/08-devops-sre.md) |
| 10 — Launch strategy | [docs/10-launch.md](docs/10-launch.md) |
| Diagrams (mermaid) | [docs/architecture-diagrams.md](docs/architecture-diagrams.md) |
| Cost estimation | [docs/cost-estimation.md](docs/cost-estimation.md) |
| Development roadmap | [docs/roadmap.md](docs/roadmap.md) |
| API spec | [docs/openapi.yaml](docs/openapi.yaml) |

## Headline engineering decisions (full rationale in docs/00)

- **Engine: Godot 4.** Free/open-source (no royalty), first-class 2.5D, exports PC + iOS/Android + **HTML5/WebGL** from one codebase — the only "PC+Mobile+Web" requirement that one engine satisfies without a separate web port. Unity/Unreal compared in docs/00.
- **Backend: Go.** Goroutine-per-match fits thousands of concurrent authoritative battles; static binaries + distroless images; GC pauses bounded well under our 50ms turn-resolution SLO.
- **Cloud: GCP.** GKE + Agones (game-server orchestration built for exactly this), global L7 LB with anycast, BigQuery for analytics. AWS/Azure compared in docs/00.
- **Database: Postgres (system of record) + Redis (realtime tier) + BigQuery (analytics).** Polyglot persistence chosen per access pattern, not dogma.
- **Authority: the server simulates every battle from a seed.** Clients send *intents only*; the server owns RNG and re-simulates to detect cheats. This single decision underpins fairness, anti-cheat, replays, and spectating.
