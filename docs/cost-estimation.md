# Cost Estimation

Order-of-magnitude planning numbers (USD), GCP list-price-ish, for the **online
infrastructure**. Real costs depend heavily on concurrency and committed-use discounts
(CUDs typically cut compute 30–55%). Three scale scenarios.

## Monthly infra (steady-state)

| Component | Soft launch (~5k CCU) | Growth (~50k CCU) | Scale (~250k CCU) |
|---|---|---|---|
| GKE battle fleet (Agones) | $1.2k | $9k | $40k |
| Core services (gateway/profile/MM/etc.) | $0.8k | $4k | $16k |
| Cloud SQL (Postgres HA + replicas) | $0.6k | $3.5k | $14k |
| Redis (Memorystore cluster) | $0.4k | $2k | $9k |
| BigQuery (analytics storage+query) | $0.3k | $1.5k | $7k |
| GCS + CDN (assets, replays) | $0.5k | $3k | $18k |
| Load balancing + Cloud Armor (DDoS) | $0.3k | $1k | $4k |
| Logging/monitoring/tracing | $0.4k | $2k | $8k |
| **Infra subtotal/mo** | **~$5.5k** | **~$26k** | **~$116k** |

Notes: the battle fleet is the realtime cost driver and scales ~linearly with concurrent
matches (~400 matches/pod). CDN dominates at scale (client + asset delivery) and is the
first place to negotiate committed pricing. Replays are seed+intent logs (KB each), so
replay storage is negligible.

## One-time / non-infra (not cloud)

| Item | Rough range |
|---|---|
| Art production (300 creatures + 5 biomes + UI/VFX) | the dominant content cost |
| Team (eng/design/art/QA/LiveOps) over an 18-mo build | the dominant overall cost |
| Audio (music + SFX) | moderate |
| Store/platform fees | 15–30% of revenue (post-launch) |
| Third-party SDKs (crash/attestation/payments) | low/usage-based |

Engineering and art labor, not cloud, are the budget — cloud is a single-digit-percent
line until very large scale. The architecture deliberately keeps the expensive realtime
tier (battle fleet) decoupled and independently scalable so spend tracks actual CCU.

## Cost-control levers (engineered in)

- HPA scales the battle fleet on **session count**, not CPU, so we don't over-provision.
- Stateless services on **spot/preemptible** nodes; only battle nodes on stable nodes.
- Immutable distroless images = fast scale-up, no over-provisioned headroom.
- Redis/CDN caching keeps Postgres (the priciest consistent tier) read-light.
- Analytics isolated in BigQuery (pay-per-query) instead of inflating the OLTP DB.
- Per-service budgets + alerts; quarterly CUD purchasing once usage stabilizes.
