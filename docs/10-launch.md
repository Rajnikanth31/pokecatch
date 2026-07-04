# 10 — Launch Strategy

## Phased rollout

| Phase | Audience | Goal | Exit criteria |
|---|---|---|---|
| **Closed Alpha** | ~500 invited, NDA | Validate core loop + battle feel; find crash classes | Crash-free > 98%, core loop fun-verified, no P0 economy bugs |
| **Closed Beta** | ~20k, key-gated | Stress matchmaking/battle at scale; first ranked season test | p99 turn < 50 ms under load, MMR distribution healthy, D1 > 35% |
| **Open Beta** | Region-limited, open | Server capacity, anti-cheat under adversarial load, store flows | Availability 99.9% sustained, payment funnel works, no dupe exploits |
| **Soft Launch** | 2–3 representative countries | Tune monetization/retention with real spend; LiveOps dry-run | D30 > 12%, ARPDAU on target, support load sustainable |
| **Global Launch** | Worldwide, all platforms | Scale + marketing peak | — |

Each gate is a go/no-go on the metrics, not a calendar date. Soft-launch geos are chosen
for representativeness and lower CPI so we can iterate cheaply before global spend.

## Marketing strategy

- **Pre-launch:** reveal trailer + creature-design dev-blogs (the original Animae are the
  hook), wishlist/pre-register funnel with milestone rewards, press/preview embargo lift
  around Open Beta.
- **Creator-first:** the game is built to be watched — **spectator mode + deterministic
  replays** make ranked matches shareable. Seed top creators early with exclusive
  cosmetic codes (cosmetic only — never competitive advantage, consistent with the
  monetization promise).
- **Launch beats:** global launch trailer, a launch ranked season with a prestige
  cosmetic, and a community catch-event.
- **ASO/store:** localized listings, platform feature pitches (Apple/Google/Steam),
  controller-support and accessibility badges.

## Community building

- **Discord** as the home: onboarding/role-select, region channels, LFG for raids/trades,
  bug-report + feedback pipelines that feed Jira, creator and competitive sub-communities,
  and dev office-hours/patch-note AMAs. Moderation + anti-toxicity tooling from day one.
- **In-game social** (guilds, trading, spectating) cross-promoted with Discord.
- **Competitive scene:** seasonal ladders → community tournaments → official invitational
  once the meta stabilizes; deterministic replays make casting and integrity easy.

## Streamer/creator campaigns

Tiered creator program: early access + cosmetic drops for codes their audience can
redeem, "creator showdown" exhibition tournaments at each season start, and a replay-
clip toolkit so creators can export highlight GIFs (the engine's replay system makes this
cheap). Strictly cosmetic incentives to protect competitive integrity and trust.

## Analytics KPIs

- **Acquisition:** installs, CPI, store conversion, pre-reg → install.
- **Activation:** tutorial completion, first-catch, first-battle-win, D1 retention.
- **Engagement:** DAU/MAU (stickiness), session length/frequency, loop completions,
  Dex-completion curve, ranked participation.
- **Retention:** D1/D7/D30, season-over-season return, churn cohorts.
- **Monetization:** ARPDAU, ARPPU, conversion-to-payer, Battle-Pass attach, LTV vs CPI
  (must trend > 1 before scaling spend).
- **Health:** crash-free sessions, p99 latency, matchmaking wait, cheat-flag rate.

All emitted to BigQuery and surfaced on a LiveOps dashboard; soft-launch decisions
(scale or iterate) are made against LTV:CPI and D30, not vanity metrics.
