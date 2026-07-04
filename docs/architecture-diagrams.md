# Architecture diagrams

Rendered with Mermaid (GitHub renders these natively).

## 1. System context (C4 level 1)

```mermaid
flowchart TB
    subgraph Clients
      PC[PC client - Godot]
      Mobile[iOS/Android - Godot]
      Web[WebGL - Godot HTML5]
    end
    CDN[Global CDN / Anycast LB]
    GW[API Gateway]
    subgraph Edge[Regional battle edge]
      BS[Battle servers - Agones fleet]
    end
    subgraph Core[Core services]
      AUTH[Auth]
      PROF[Profile/Collection/Inventory]
      MM[Matchmaking]
      LB[Leaderboards]
      CHAT[Chat]
      NOTIF[Notifications]
    end
    PG[(PostgreSQL - primary + read replicas)]
    RD[(Redis cluster)]
    BQ[(BigQuery - analytics)]
    OBJ[(Object storage - assets, replays)]

    PC & Mobile & Web --> CDN --> GW
    PC & Mobile & Web -. wss .-> BS
    GW --> AUTH & PROF & MM & LB & CHAT & NOTIF
    MM <--> RD
    BS <--> RD
    BS --> PG
    AUTH & PROF & LB & CHAT --> PG
    PROF & GW --> RD
    AUTH & PROF & BS & MM --> BQ
    CDN --> OBJ
```

## 2. Real-time PvP battle sequence (server-authoritative)

```mermaid
sequenceDiagram
    participant A as Client A
    participant B as Client B
    participant MM as Matchmaking
    participant BS as Battle Server (authoritative)
    A->>MM: POST /matchmaking/queue (ranked)
    B->>MM: POST /matchmaking/queue (ranked)
    MM->>MM: Pair() within MMR band + region
    MM->>BS: CreateSession(seed, teamA, teamB)
    MM-->>A: matched + wss URL + ticket
    MM-->>B: matched + wss URL + ticket
    A->>BS: WS connect (ticket)
    B->>BS: WS connect (ticket)
    BS-->>A: match_start + StateSnapshot
    BS-->>B: match_start + StateSnapshot
    loop each turn
        A->>BS: action{seq, move slot} (intent only)
        B->>BS: action{seq, switch}
        Note over BS: validate -> ResolveTurn(seed RNG) -> Digest
        BS-->>A: turn{events, digest, redacted state}
        BS-->>B: turn{events, digest, redacted state}
    end
    BS-->>A: end{winner, digest}
    BS->>PG: persist match + per-turn actions (replayable)
```

## 3. Reconnection / desync correction

```mermaid
flowchart LR
    C[Client] -- socket drops --> X((lost))
    C -- reconnect with last seq --> BS[Battle Server]
    BS -- session still in Redis? --> RD[(Redis battle:node)]
    RD -- yes, same node --> BS
    BS -- full StateSnapshot --> C
    C -- local replay digest != server digest --> RESYNC[request resync]
    RESYNC --> BS
    BS -- authoritative snapshot --> C
```

## 4. CI/CD + canary

```mermaid
flowchart LR
    Dev[PR] --> CI[CI: vet, race tests, coverage gate, reference parity]
    CI -->|green, merge to main| TAG[tag v*]
    TAG --> BUILD[build + push distroless images]
    BUILD --> CANARY[Argo Rollouts 10%]
    CANARY -->|SLO bake 10m ok| FIFTY[50%]
    FIFTY --> FULL[100%]
    CANARY -->|error-budget burn alert| RB[auto rollback]
```

## 5. Battle engine internal flow (one turn)

```mermaid
flowchart TB
    IN[two intents] --> SW[Phase 1: resolve switches]
    SW --> ORD[order by priority, then effSpeed, RNG tiebreak]
    ORD --> M1[attacker 1: PreMoveGate -> ComputeDamage -> riders]
    M1 --> V1{victory?}
    V1 -- no --> M2[attacker 2: same pipeline]
    V1 -- yes --> END[emit end]
    M2 --> EOT[Phase 3: end-of-turn status/regen ticks]
    EOT --> CD[tick cooldowns] --> DG[recompute Digest]
```
