# Folder Structure

Single Go module monorepo + Godot client + infra-as-code. Services share the
domain packages under `pkg/`; nothing in `pkg/` imports a service (acyclic).

```
beastbound/
├── README.md                      # entry point + verification commands
├── go.mod                         # single module: github.com/aurelia/beastbound
├── pkg/                           # shared, dependency-light domain (engine-agnostic)
│   ├── types/                     # elements, 12x12 effectiveness matrix, stats, enums
│   └── creatures/                 # species/instance model, stat & XP formulas, dex loader
├── services/
│   ├── gateway/                   # public ingress: auth, rate limit, routing
│   │   ├── cmd/                    # main()
│   │   └── internal/gateway.go
│   ├── battle/                    # authoritative real-time battle server
│   │   ├── cmd/main.go
│   │   └── internal/
│   │       ├── engine/             # PURE battle engine (rng, battler, damage, status, turn, digest) + tests
│   │       ├── netcode/            # server-authoritative session, wire protocol, broadcast, reconnect
│   │       └── ai/                 # server-side PvE AI (expectimax + heuristics) + tests
│   ├── profile/                   # account/collection/inventory/team + offline save system + tests
│   └── matchmaking/               # region + MMR-band pairing + tests
├── tools/creaturegen/             # deterministic 300-creature generator (Go canonical + Node mirror)
├── data/creatures/                # seed.json (generated 300) + flagships.json (20 hand-authored)
├── db/
│   ├── migrations/0001_init.sql   # Postgres schema (system of record)
│   └── redis_keys.md              # Redis keyspace + rationale
├── deploy/
│   ├── docker/                    # multi-stage distroless Dockerfile + compose
│   ├── k8s/                       # Deployments, Service, HPA, Ingress
│   └── observability/             # Prometheus SLO/error-budget alert rules
├── .github/workflows/             # CI (test/coverage/parity) + CD (canary deploy)
├── client-godot/scripts/          # Godot 4 client (thin view over the event stream)
├── test/reference/                # language-independent verifiers (engine math)
└── docs/                          # phases 0–10, diagrams, openapi, cost, roadmap
```

## Why this shape

- **`pkg/` is the stable core** the whole product depends on and is the most
  heavily tested; it has zero third-party and zero service dependencies so it
  compiles anywhere (server, CI, future WASM client mirror).
- **`engine/` is pure**; `netcode/` and `ai/` wrap it. Time/trust/IO live outside
  the engine so the engine stays a unit-testable function (SRP / clean architecture).
- **One module, many services** keeps refactors atomic and shared types in lockstep,
  at the cost of a larger build graph — acceptable at this size; splits cleanly later.
