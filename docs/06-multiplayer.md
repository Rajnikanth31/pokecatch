# 06 — Multiplayer

Implemented core: `services/matchmaking/internal` (pairing) and
`services/battle/internal/netcode` (authoritative session, protocol, broadcast,
reconnection). This doc covers the full online feature set and the realtime guarantees.

## Feature set

- **Friend system:** bidirectional friends with presence (`presence:{account_id}` in
  Redis, 60s heartbeat); online friends surface for direct-challenge and spectate.
- **Trading:** two-phase, escrowed, audited (`trades` + `trade_items`). Both parties
  stage items/creatures, both must lock, then a single Postgres transaction commits the
  swap — this is the canonical dupe-attack surface, so commit is atomic and the
  currency/collection ledgers make any anomaly auditable. Level-/region-trade evolutions
  are processed server-side at commit.
- **PvP battles:** casual (unranked) and ranked (Glicko-2). The battle WS is fully
  server-authoritative (see below).
- **Guilds:** persistent groups with shared chat, a guild leaderboard, raid grouping,
  and weekly guild objectives. Membership/roles in Postgres; live guild chat in Redis
  streams.
- **Global tournaments:** scheduled bracket events (Swiss → single-elim) run by a
  tournament service over the same battle engine; results feed a separate ladder.
- **Spectator mode:** because battles are deterministic event streams, spectating is just
  a fan-out subscription to a match's `turn` broadcasts with **redacted** state
  (`Session.snapshot` / `Transport.Spectate`) on a short delay to deter ghosting.

## Realtime requirements & how they're met

- **Low latency:** region-based matchmaking pairs players in the same region
  (`matcher.go` isolates by region) and the battle WS connects to the **nearest Agones
  edge** via regional L4 LBs. Turn-based combat is latency-tolerant by nature; we target
  WS RTT < 80 ms intra-region and a turn-resolution server budget p99 < 50 ms.
- **Reconnection logic:** sessions are sticky-routed (`battle:node:{session_id}` in
  Redis). On socket drop the client reconnects to the same authoritative node and
  receives a full `StateSnapshot` to rebuild UI without replaying the match; a
  per-client turn budget (default 45 s) plus AFK defaults keep the match progressing
  during brief disconnects.
- **Sync validation:** every `turn` broadcast carries the engine `Digest`. The client
  replays the event deltas locally and compares digests; a mismatch triggers a `resync`
  request answered with an authoritative snapshot. The server is always the source of
  truth — clients are corrected, never trusted.
- **Lag compensation:** combat is turn-based, so there is no positional rollback to do.
  Compensation is instead about *input fairness*: the turn timer starts on
  `match_start`/`turn` delivery (server timestamp), both intents are collected before
  resolution (simultaneous reveal), and a small input grace window absorbs jitter so a
  laggy player isn't defaulted unfairly.
- **Region-based matchmaking:** the pool is partitioned by region; cross-region matches
  never happen under normal policy, and only the hard-timeout path (after 30 s) widens
  selection — and even then stays within region (`TestRegionIsolation`). Players may opt
  into a "global" pool that accepts higher latency for faster queues.

## Authoritative session model (recap of the implementation)

One goroutine owns one `Session`; all client input arrives via an inbox channel, so the
engine is never touched concurrently and needs no locks around battle state. The session
collects both **intents** (move slot / switch — the only thing a client may send),
validates them server-side (illegal → safe default), resolves the turn through the pure
engine, computes the digest, and broadcasts event deltas + redacted snapshots to both
players and spectators. This is the same code path tested by the engine and matchmaking
suites.
