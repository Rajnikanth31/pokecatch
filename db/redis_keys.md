# Redis keyspace & rationale

Redis is the low-latency tier for data that is **ephemeral, high-churn, or
read-dominated**, where Postgres round-trips would add unacceptable latency to
the realtime path. Nothing here is a system of record; every key is either
reconstructable or expendable.

| Pattern | Type | TTL | Purpose |
|---|---|---|---|
| `session:{token}` | string (account_id) | 30m sliding | Auth session lookup on every gateway request. |
| `presence:{account_id}` | string (gateway_node) | 60s heartbeat | Which edge node holds the player's socket; powers friends-online and routing. |
| `mm:queue:{region}:{mode}` | sorted set (score = mmr) | — | Matchmaking pool; range queries find near-MMR opponents. |
| `mm:lock:{account_id}` | string | 10s | Prevents a player being matched into two games at once. |
| `lb:ranked:{season}` | sorted set (score = mmr) | season | Leaderboard top-N + player rank via `ZREVRANK`. |
| `battle:node:{session_id}` | string (battle_node) | match life | Sticky routing so reconnects land on the authoritative server. |
| `rate:{account_id}:{route}` | string counter | 1s–1m | Token-bucket rate limiting at the gateway. |
| `chat:channel:{id}` | stream | capped | Recent chat backlog; durable history lands in Postgres async. |

**Why a sorted set for matchmaking:** an O(log N) range query around the
player's MMR (`ZRANGEBYSCORE mmr-band`) gives fair, fast opponent selection and
naturally widens the band over time by expanding the range — no scan, no lock
contention. The score is the Glicko-2 rating; RD (deviation) is read from
Postgres only at enqueue time.

**Durability stance:** Redis runs with AOF `everysec`. Losing ≤1s of presence or
queue state is acceptable (clients re-enqueue, re-heartbeat). The ledger, the
collection and match results are never trusted to Redis.
