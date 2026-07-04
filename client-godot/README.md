# Aurelia: Beastbound — Godot client

A real Godot 4 project (not a stub). One codebase exports to **PC** (Windows/
macOS/Linux), **Mobile** (iOS/Android), and **Web** (HTML5/WASM) — the reason
Godot was chosen (see `../docs/00-overview.md`).

## Run it

1. Install **Godot 4.2+** (standard build).
2. In Godot: *Import* → select this `client-godot/` folder → open.
3. Press **Play**. `Main.tscn` boots → drops into the overworld.

No backend is required to walk around and trigger encounters. To exercise online
login/matchmaking, run the services (`docker compose -f ../deploy/docker/docker-compose.yml up`)
and set `BB_DEV_EMAIL` / `BB_DEV_PASS` env vars, or point `ApiClient.BASE_URL` at
your gateway.

## What's here

| File | Role |
|---|---|
| `project.godot` | Project config, autoloads, GL-compatibility renderer (web/mobile-friendly) |
| `scripts/EventBus.gd` | App-wide typed signal hub (decouples systems) |
| `scripts/ApiClient.gd` | REST client for the gateway (auth, collection, team, matchmaking) with transparent token refresh |
| `scripts/BattleClient.gd` | Authoritative battle WebSocket: sends **intents only**, renders the server event stream |
| `scripts/Main.gd` | Bootstrap → overworld |
| `scripts/Overworld.gd` | Movement + wild-encounter trigger |
| `scripts/BattleView.gd` | Battle UI: HP bars + event log + move buttons (pure view) |
| `scenes/*.tscn` | Main / Overworld / Battle scenes |

## Design notes

- **The client never simulates battles.** It sends a move/switch intent and
  animates whatever `turn` events the server returns, comparing the server's
  state digest to detect desync. This is the client half of the server-
  authoritative model (`../docs/07-security.md`).
- **Thin views over singletons.** Gameplay scenes hold no networking logic; they
  talk to the `Api` and `Battle` autoloads and subscribe to `EventBus`.
- **One input vocabulary.** Movement uses Godot's `ui_*` actions so keyboard,
  gamepad, and touch all map to the same code path.

## Not yet built (next steps)

Real art/animation (currently primitives), the collection/team-builder UI, the
title/login screen, and the trading UI. The systems they'd bind to (ApiClient,
EventBus, BattleClient) are already in place.
