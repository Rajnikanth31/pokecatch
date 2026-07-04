# 03 — Technical Architecture

See `docs/architecture-diagrams.md` for the rendered system, sequence, and CI diagrams.

## Frontend (Godot 4 client)

- **UI/UX & HUD:** scene-composed UI with a global `UIController` autoload; HUD shows
  active Anima HP/status, party tray, minimap, and contextual action prompts. Mobile
  uses a thumb-zone action wheel; PC/web use hotkeys + pointer. One responsive layout
  driven by an anchor/container system, not per-platform forks.
- **Inventory & collection:** virtualized grid (only visible cells instantiated) so the
  300-Dex + items render at 60 FPS on mobile; cursor-paginated from the profile service.
- **Battle screen:** purely a *view* over the server's event stream — it animates
  `Event`s (move/damage/crit/status/faint) and never computes outcomes. This is what
  makes the client un-authoritative by design.
- **Animation pipeline:** skeletal 2.5D (Godot `AnimationTree` state machines) with
  additive hit-reactions; VFX via `GPUParticles2D`/3D. Battle animations are data-driven
  off skill element + class so new moves reuse VFX templates.
- **Input system:** Godot `InputMap` actions abstract keyboard/gamepad/touch into the
  same action names; remappable, with full controller support.
- **Accessibility:** colorblind-safe type palette (shape + color icons, never color
  alone), scalable UI/text, reduce-motion toggle (caps screen shake/flash — also
  reduces photosensitivity risk), remappable inputs, hold-vs-toggle options, subtitles
  for all audio cues, and a battle "slow log" mode.

## Game-engine architecture (client-side)

- **ECS / state:** Godot is node-first, so we use a hybrid — nodes for scene/render,
  with a lightweight component layer (`Stats`, `Movement`, `Interactable`, `Spawner`)
  and systems for overworld logic to keep behavior data-driven and testable. Battle
  state is **not** in the client ECS; it mirrors the server snapshot.
- **Save/load:** local saves are a versioned, checksummed blob (overworld position,
  flags, settings) used for offline PvE; the **authoritative** collection/inventory/
  progress live server-side and sync on login. Saves are migration-versioned so old
  files load forward.
- **Physics & pathfinding:** Godot physics for overworld traversal; navigation meshes +
  A* for NPC movement and wild-creature wander (see AI doc).
- **AI systems:** behavior trees for NPC/wild behavior, the expectimax battle AI runs
  **server-side** (so PvE bosses can't be cheesed by reading client memory).
- **Audio engine:** Godot buses with ducking (music dips under battle SFX), adaptive
  layers per region, and a small FMOD-style cue system implemented over `AudioStream`.
- **Event bus:** a global typed signal hub (`EventBus` autoload) decouples systems —
  UI subscribes to `battle_event`, quests to `creature_caught`, etc.

## Backend (microservices)

| Service | Owns | Store |
|---|---|---|
| **Gateway** | TLS edge, auth, rate limit, routing | Redis (sessions, limits) |
| **Auth** | register/login/refresh, token rotation | Postgres + Redis |
| **Profile** | account, collection, inventory, team, trades, evolve | Postgres + Redis |
| **Matchmaking** | region+MMR pairing, ticket lifecycle | Redis (sorted-set pool) |
| **Battle** | authoritative real-time matches (WS) | in-memory + Redis routing + Postgres results |
| **Leaderboards** | seasonal ranks, top-N, player rank | Redis (`ZSET`) + Postgres snapshots |
| **Chat** | channels, DMs, moderation hooks | Redis streams + Postgres history |
| **Notifications** | push (FCM/APNs), in-game inbox | Postgres + queue |

Boundaries follow **ownership of an invariant**, not CRUD nouns: the service that can
violate a rule owns the data and the only write path to it (e.g. only Profile mutates
the collection; only Battle writes match results). Services communicate over gRPC
internally; the Gateway speaks REST/WS to clients.

- **Microservices / k8s / Docker:** each service is a distroless static Go binary
  (`deploy/docker/battle.Dockerfile`, `ARG SERVICE` selects which) deployed to GKE.
  Battle servers run as an **Agones fleet** (game-server-aware lifecycle) rather than a
  plain Deployment, because matches are stateful and must drain cleanly.
- **CI/CD:** GitHub Actions — vet, race tests, coverage gate, reference parity, image
  build/push, Argo Rollouts canary (`.github/workflows`).
- **CDN:** GCS-backed global CDN for client assets and replay blobs.
- **Load balancers:** global anycast L7 for REST; regional L4 + Agones allocation for
  battle WS so players connect to the nearest authoritative edge.
- **Caching:** Redis for hot reads (profile summaries, leaderboards), CDN for static,
  and an in-process immutable Dex (species/skills loaded once at boot).

## Cloud & database

Chosen stack and full rationale are in `docs/00-overview.md`: **GCP (GKE + Agones +
BigQuery)**, **Postgres (system of record) + Redis (realtime) + BigQuery (analytics) +
GCS/CDN (assets/replays)**. Schema in `db/migrations/0001_init.sql`; Redis keyspace in
`db/redis_keys.md`. Player profiles, inventory ledger, match history → Postgres;
sessions/queue/leaderboard/presence → Redis; event analytics → BigQuery.
