# 04 — AI Systems

## Wild creature behavior

Overworld Animae are driven by **behavior trees** over a navmesh: `Wander` (A* to a
random reachable point in a home radius), `Flee`/`Approach` based on a per-species
temperament and the player's "tracker" reputation, and biome/time-of-day spawn gating.
Rare/legendary spawns add an `Evade` node (teleport-on-sight catch puzzles). Trees are
authored as data (JSON) so designers tune behavior without code.

## NPC AI

Townsfolk run a **daily-schedule state machine** (home → work → social → home) layered
on the same navmesh, giving the world ambient life cheaply. Quest NPCs add interaction
nodes gated on quest flags via the event bus. Companion/rival Wardens use the **battle
AI below** — no special-cased opponent logic.

## Battle AI (server-side)

Two layers:

1. **Search:** depth-2 **expectimax** over the legal action set (4 moves + up to 5
   switches), with chance nodes for the damage spread / status rolls. Leaf states are
   scored by a heuristic eval:

   ```
   eval = w1*Σ(hp_fraction)            // team health
        + w2*type_advantage(active)     // matchup vs opponent active
        + w3*ko_threat(self, opp)       // can I KO / will I be KO'd
        + w4*status_value                // burn/para on a sweeper, etc.
        + w5*hazard_and_stage_value      // board state (buffs/debuffs)
   ```

   Depth 2 keeps per-turn cost well under the 50 ms budget while still seeing the
   immediate threat-and-response that defines competent play.

2. **Personality + difficulty (behavior tree wrapper):** a tree biases the chosen
   action toward an archetype (aggressive Keeper favors KO lines; defensive trainer
   favors pivots/status). The **difficulty knob** degrades the search for accessible
   PvE: reduce depth to 1, add Gaussian noise to leaf scores, and occasionally pick the
   2nd-best action — producing believably "weaker" opponents without dumb random play.

## Adaptive difficulty

A per-player **flow estimator** (recent win streak, average turns-to-win, party-level vs
content-level) nudges PvE encounter difficulty within a band: persistent stomping raises
opponent depth/level a notch; repeated losses lower it. Bounded so it never trivializes
or wall-blocks, and **disabled in ranked/raids** where fairness/fixed-tuning matter.

## Reinforcement learning (offline, for tuning — not live opponents)

We do **not** ship a live RL agent (latency, exploitability, and balance-opacity make
it the wrong call for player-facing opponents). Instead RL is a **balance tool**:
self-play agents (PPO over the battle engine, which is already a clean simulator with a
state digest) play millions of games to surface dominant strategies and over/under-
powered creatures before they ship. The engine's determinism and headless re-simulation
make it an ideal training environment. Outputs feed generator tuning and nerf/buff
decisions, with humans in the loop.

## Recommendation system

Lightweight, collaborative-filtering + content-based hybrid: "Wardens like you also
trained…", team-completion suggestions (fill a type/role gap in your current team), and
next-content nudges. Trained in BigQuery ML over the analytics event stream; served as
a cached, non-blocking suggestion (never gates play). Privacy: features are aggregate
behavior, no PII.

## Fraud / cheat detection

Complements the deterministic anti-cheat (see security doc) with **anomaly detection**:
features include win-rate vs MMR delta, inhuman action timing distributions, impossible
catch/economy rates, and trade-graph patterns (dupe rings, account-boosting). A gradient-
boosted model scores accounts; high scores route to shadow-flagging and manual review
rather than auto-ban, to bound false-positive harm. Bot detection adds device
attestation + behavioral biometrics on the client signal.

## Pathfinding

A* over Godot navmeshes for NPC/wild movement with hierarchical regions for the larger
shards (coarse region graph → local mesh) to keep pathfinding cheap with many agents;
flow-field pathing for crowd events (festivals). Battle has no spatial pathfinding (it's
turn/menu-based).
