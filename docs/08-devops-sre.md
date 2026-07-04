# 08 — DevOps & SRE

## Git workflow

Trunk-based with short-lived feature branches and PRs into `main`. `main` is always
releasable; releases are cut by semver tag (`v*`) which triggers the deploy pipeline.
Conventional Commits drive automated changelogs. Protected `main`: required CI green +
one review. Feature flags (not long-lived branches) gate unfinished work.

## CI/CD (`.github/workflows`)

- **CI (every PR):** `go vet`, `go test -race` with an atomic coverage profile, a
  **70% coverage gate**, the **reference-parity** checks (`verify_engine.js`), and a
  **deterministic-seed check** (regenerate the Dex and assert a clean diff against the
  committed seed — catches accidental balance drift).
- **Lint:** golangci-lint.
- **Deploy (on tag):** matrix-build distroless images for all services, push to GHCR,
  then **Argo Rollouts canary**: 10% → 10-min SLO bake → 50% → 100%, with automatic
  rollback if an error-budget-burn alert fires during the bake.

## Test automation

Unit tests beside code (engine, stats, matchmaking), integration tests at the session
layer (full-battle-to-completion determinism, digest progression), and the language-
independent reference verifiers in `test/reference`. Load tests (k6 against the WS) and
chaos drills (kill battle pods mid-match to validate drain + reconnect) run pre-release.

## Canary & rollback

Argo Rollouts manages progressive delivery; the battle fleet additionally respects
Agones drain so a rollout never kills a match mid-turn (`terminationGracePeriodSeconds`
> max turn budget, `preStop` LB deregistration delay). Rollback is a single
`argo rollouts undo` (or automatic on SLO burn) and is fast because images are immutable
and config is declarative.

## Monitoring, crash analytics, observability

- **Metrics:** Prometheus (RED for services, USE for nodes) + custom game metrics
  (`active_battle_sessions`, `battle_turn_seconds`, matchmaking wait, catch/economy
  rates). Grafana dashboards per service + a LiveOps board.
- **Tracing:** OpenTelemetry traces across gateway → service → DB, sampled, to debug
  tail latency.
- **Logs:** structured JSON to Cloud Logging, correlated by request id propagated from
  the gateway.
- **Crash analytics:** client crashes via a Sentry-style SDK (symbolicated per
  platform), backend panics captured with stack + match context; crash-free-session rate
  is an SLO.
- **Alerting:** multi-window multi-burn-rate SLO alerts (`deploy/observability/slo-
  alerts.yaml`) page on fast burn, ticket on slow.

## SRE metrics & targets

| Metric | Target (SLO) |
|---|---|
| API availability (gateway) | 99.9% (28-day) |
| REST latency p99 | < 250 ms |
| Battle turn resolution p99 | < 50 ms |
| WS RTT intra-region p95 | < 80 ms |
| Crash-free sessions | > 99.5% |
| Matchmaking time-to-match p95 (ranked) | < 30 s |

- **SLA:** external commitment of 99.5% monthly availability for the core service (kept
  below the internal SLO so we have margin).
- **SLO:** the internal targets above.
- **Error budget:** `1 − SLO` (e.g. 0.1% over 28 days for availability). Budget spent →
  feature freeze, reliability work prioritized; budget healthy → ship faster. The fast-
  burn alert fires at 14.4× burn (2% of a 28-day budget in 1h).
- **Uptime / latency / crash rate** are dashboarded continuously and reviewed weekly in
  an ops review; postmortems are blameless and tracked to action items.

## Capacity & cost controls

HPA on the battle fleet scales on `active_battle_sessions` (~400 matches/pod), not CPU
alone, because realtime nodes care about session count. Non-realtime services scale on
RED metrics. Spot/preemptible nodes back stateless workloads; battle nodes run on stable
nodes to avoid mid-match eviction. Cost guardrails and per-service budgets in
`docs/cost-estimation.md`.