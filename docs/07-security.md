# 07 — Security & Anti-Cheat

## Server-authoritative architecture (the foundation)

Every competitively-relevant action is decided by the server. Clients send **intents**,
never outcomes. The battle server owns the per-match RNG seed (crypto/rand, withheld
until the match ends), simulates each turn (`engine.ResolveTurn`), and stores seed +
per-turn intents (`match_actions`). This makes the entire match a pure function of
`(seed, intents)` and therefore **re-simulatable**: anti-cheat is "re-run it and compare
the Digest," not heuristic guessing. No client-reported damage, catch, or result is ever
trusted.

## Anti-cheat layers

1. **Determinism + digest reconciliation:** per-turn `Digest` (FNV-1a over RNG state +
   HP + status + active indices). Any client claiming a divergent state is corrected;
   any stored match can be re-simulated to verify its outcome before awarding ranked
   points.
2. **Information hiding:** opponents see redacted `BattlerView` (HP/level/status only) —
   never IVs/EVs/exact stats or bench details until revealed in play, defeating
   information-based cheats.
3. **Input legality validation:** the session validates every intent (slot range, switch
   legality) and substitutes a safe default for illegal/late input, giving cheaters no
   signal and no advantage.
4. **Economy invariants:** trades are two-phase + atomic; currency is an append-only
   ledger with a UNIQUE receipt id (replay-proof); the collection has strict single-owner
   FKs. Dupes are structurally prevented, not merely detected.
5. **Behavioral fraud detection:** ML anomaly scoring (see AI doc) for win/MMR
   inconsistencies, inhuman timing, impossible economy/catch rates, and trade-ring graphs.
   High scores → shadow flag → human review (bounded false-positive harm).

## Packet validation

All client messages are size-bounded, schema-validated, and sequence-numbered (`seq`)
for idempotency. Out-of-range fields are rejected at the edge; the WS reader enforces
max message size and a per-connection message rate. gRPC between services uses mTLS and
strict proto validation.

## Bot detection

Device attestation (Play Integrity / App Attest) on mobile, behavioral biometrics
(timing/entropy of inputs) feeding the fraud model, and progressive friction (CAPTCHA-
equivalents only on high-suspicion accounts) rather than blanket challenges.

## Exploit prevention

- No client-side authority to exploit (the core mitigation).
- Idempotent, ledgered economy operations (no double-spend/dupe windows).
- Save files are for offline PvE only and never feed authoritative state.
- Feature flags + kill switches to disable a broken system in seconds without a deploy.

## DDoS protection

Anycast edge + cloud DDoS (GCP Cloud Armor) absorbs L3/4. L7: per-account and per-IP
token-bucket rate limiting at the gateway (`MemoryLimiter`/Redis), connection caps on
the battle WS, and autoscaling with shed-load (return 429/503 with Retry-After under
pressure rather than collapse). Matchmaking and battle nodes are not directly reachable —
only via allocated, ticketed connections.

## Account security

- Passwords hashed with **argon2id**; refresh tokens stored **hashed** (`HashToken`) so
  a DB leak can't mint sessions.
- Short-lived access JWTs + rotating refresh tokens with device binding and revocation
  (`auth_tokens.revoked`, Redis session check).
- TOTP 2FA, email-verified sensitive actions (trades over a value threshold, email
  change), and anomalous-login alerts.
- Least-privilege service accounts, secrets in a managed secret store (not env files in
  images), and audit logging of admin/economy actions.

## Privacy & compliance

PII minimized and isolated in Postgres; analytics uses pseudonymous ids. GDPR/CCPA
data-export and delete flows, regional data residency where required, and age-gating for
younger players (COPPA-aware) with restricted social/chat features.
