# Getting Started — run the whole thing

Three ways to run, smallest to largest.

## 1. Fastest: verify the logic (only Node needed)

```bash
make quick            # or:  .\test.ps1
```

Runs the engine, session, and backend reference checks plus the creature-balance
audit. No Go, no Docker, no database. This is the quickest proof the core logic is
sound.

## 2. Run the backend stack (Docker)

```bash
docker compose -f deploy/docker/docker-compose.yml up --build
```

Brings up **Postgres** (auto-seeded from `db/migrations`), **Redis**, and the
**auth**, **profile**, **battle**, and **gateway** services. Then:

```bash
# Register (through the gateway)
curl -s localhost:8088/v1/auth/register \
  -H 'content-type: application/json' \
  -d '{"email":"ash@aurelia.gg","password":"trainerpass123","display_name":"Ash"}'
# -> {"access_token":"...","refresh_token":"...","expires_in":900}

# Use the access token on an authenticated route
curl -s localhost:8088/v1/creatures -H "authorization: Bearer <access_token>"
```

Auth and profile use Postgres because `DATABASE_URL` is set in compose; without it
they fall back to in-memory (handy for `go run` during development).

## 3. Run the client (Godot)

1. Install **Godot 4.2+**.
2. Import the `client-godot/` folder and press **Play**.
3. Title screen → *Play as Guest* to walk the overworld offline, or *Register/Log
   in* to hit the running backend. Press **Enter** in the overworld to open the
   collection / team builder. Walking in grass triggers battles.

Point the client at your backend by editing `ApiClient.BASE_URL` (defaults to the
production host; use `http://localhost:8088/v1` for local).

## Prove the anti-cheat replay end-to-end (Go)

```bash
go run ./services/battle/cmd/demo      # plays a match, writes demo-match-record.json
go run ./tools/replay --match demo-match-record.json
# -> VERDICT: AUTHENTIC ✔   (re-simulated winner + digest match the record)
```

Then hand-edit the `winner` field in `demo-match-record.json` and re-run the
replay: it prints `VERDICT: MISMATCH ✘`. That is the deterministic anti-cheat model
working.

## Full test suite (Go)

```bash
make tidy             # once: fetch deps, write go.sum
make test             # go vet + go test -race + coverage gate
make ci               # everything CI runs

# Postgres integration tests (needs the compose DB up):
DATABASE_URL=postgres://postgres:dev@localhost:5432/beastbound \
  go test -tags=integration ./services/profile/internal/
```

## Ports

| Service | Port |
|---|---|
| gateway (public) | 8088 |
| auth | 8080 |
| profile | 8081 |
| battle | 8082 |
| Postgres | 5432 |
| Redis | 6379 |
