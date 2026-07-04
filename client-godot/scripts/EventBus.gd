# EventBus.gd — app-wide typed signal hub (autoload singleton).
# Decouples systems: the overworld emits, the UI and quest systems subscribe,
# and nothing holds hard references to anything else. This is the client-side
# mirror of the server's event-driven design (docs/03-architecture.md).
extends Node

# --- session / networking ---
signal logged_in(account_id: String)
signal login_failed(reason: String)
signal match_found(session_id: String, ws_url: String, ticket: String)

# --- overworld ---
signal wild_encounter(dex_id: int, level: int)
signal creature_caught(dex_id: int)
signal region_changed(region_id: String)

# --- battle (forwarded from BattleClient) ---
signal battle_event(event: Dictionary)
signal battle_state(state: Dictionary)
signal battle_ended(winner: int)
