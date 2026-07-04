module github.com/aurelia/beastbound

go 1.22

// Single-module monorepo. Services live under services/<name> and import the
// shared domain packages under pkg/. No third-party deps in the engine core on
// purpose: the battle engine must be portable and dependency-light so it can be
// re-simulated anywhere (server, CI, a future WASM client mirror).
//
// Third-party dependencies (run `go mod tidy` to populate go.sum):
//   - gorilla/websocket : battle WS transport, isolated to netcode/ws.go
//   - jackc/pgx/v5      : Postgres driver, isolated to *_postgres.go + pkg/db
//   - x/crypto/argon2   : password hashing, isolated to auth/password.go
// The engine core (pkg/types, pkg/creatures, engine) stays dependency-free.
require (
	github.com/gorilla/websocket v1.5.3
	github.com/jackc/pgx/v5 v5.6.0
	golang.org/x/crypto v0.24.0
)
