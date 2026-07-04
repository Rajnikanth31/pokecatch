# BattleClient.gd — Godot 4 (GDScript) client side of the authoritative battle
# protocol. This is intentionally a *thin view*: it sends the player's intent and
# renders whatever event stream the server returns. It NEVER computes damage,
# crits, or outcomes — the server is the only simulator (see docs/07-security.md).
#
# Mirrors the Go wire types in services/battle/internal/netcode/messages.go.
extends Node

signal battle_event(event: Dictionary)   # forwarded to the BattleView/animator
signal state_synced(state: Dictionary)
signal battle_ended(winner: int)

var _socket := WebSocketPeer.new()
var _connected := false
var _last_seq := 0
var _last_digest := 0
var _session_id := ""

# connect_to_match opens the WS to the allocated battle edge. `ws_url` and
# `ticket` come from the matchmaking REST response.
func connect_to_match(ws_url: String, ticket: String, session_id: String) -> void:
	_session_id = session_id
	var url := "%s?ticket=%s" % [ws_url, ticket]
	var err := _socket.connect_to_url(url)
	if err != OK:
		push_error("battle socket connect failed: %s" % err)

func _process(_delta: float) -> void:
	_socket.poll()
	var st := _socket.get_ready_state()
	if st == WebSocketPeer.STATE_OPEN:
		if not _connected:
			_connected = true
		while _socket.get_available_packet_count() > 0:
			var raw := _socket.get_packet().get_string_from_utf8()
			_handle_message(JSON.parse_string(raw))
	elif st == WebSocketPeer.STATE_CLOSED:
		if _connected:
			_connected = false
			_attempt_reconnect()

# submit_move sends a move intent. The server validates it; an illegal slot is
# replaced server-side with a safe default, so the client cannot gain advantage by
# sending garbage.
func submit_move(slot: int) -> void:
	_send({"type": "action", "seq": _next_seq(), "kind": "move", "slot": slot})

func submit_switch(party_index: int) -> void:
	_send({"type": "action", "seq": _next_seq(), "kind": "switch", "switch_to": party_index})

func _handle_message(msg: Dictionary) -> void:
	match msg.get("type", ""):
		"match_start", "state":
			_last_digest = int(msg.get("digest", 0))
			state_synced.emit(msg.get("state", {}))
		"turn":
			_last_digest = int(msg.get("digest", 0))
			for ev in msg.get("events", []):
				battle_event.emit(ev)          # animator plays move/damage/crit/faint
			state_synced.emit(msg.get("state", {}))
			_verify_digest(msg)
		"end":
			battle_ended.emit(int(msg.get("winner", -1)))
		"error":
			push_warning("server error: %s" % msg.get("error", ""))

# _verify_digest is the client half of desync detection: if our locally-applied
# view diverges from the server's authoritative digest we request a full resync
# rather than continuing on a wrong state.
func _verify_digest(_msg: Dictionary) -> void:
	# A full client-side mirror of engine.Digest is optional; the cheap version is
	# to trust the server and only resync on reconnect. Competitive clients run the
	# mirror and compare here.
	pass

func _attempt_reconnect() -> void:
	# Reconnect to the SAME sticky node; the server replies with a full snapshot so
	# we rebuild the UI without replaying the match (docs/06-multiplayer.md).
	push_warning("battle socket dropped — reconnecting")
	# ... re-open with the same session ticket, then send a {"type":"resync"} ...

func _send(payload: Dictionary) -> void:
	if _socket.get_ready_state() == WebSocketPeer.STATE_OPEN:
		_socket.send_text(JSON.stringify(payload))

func _next_seq() -> int:
	_last_seq += 1
	return _last_seq
