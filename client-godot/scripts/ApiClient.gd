# ApiClient.gd — REST client for the gateway (autoload singleton).
# Wraps the endpoints in docs/openapi.yaml: auth, collection, team, matchmaking.
# Holds the access/refresh tokens and transparently retries once on 401 by
# refreshing — so gameplay code never deals with token lifetime.
extends Node

const BASE_URL := "http://15.207.157.71:8088/v1"   # AWS backend (free-tier EC2)

var _access := ""
var _refresh := ""
var _account_id := ""

func _headers(auth := true) -> PackedStringArray:
	var h := PackedStringArray(["Content-Type: application/json"])
	if auth and _access != "":
		h.append("Authorization: Bearer " + _access)
	return h

# _request is a small async helper over HTTPRequest. Each call spins a one-shot
# node so concurrent calls don't clobber each other.
func _request(method: int, path: String, body: Dictionary, auth := true) -> Dictionary:
	var http := HTTPRequest.new()
	add_child(http)
	var payload := JSON.stringify(body) if not body.is_empty() else ""
	http.request(BASE_URL + path, _headers(auth), method, payload)
	var res: Array = await http.request_completed
	http.queue_free()
	var code: int = res[1]
	var text := (res[3] as PackedByteArray).get_string_from_utf8()
	var parsed = JSON.parse_string(text)
	return {"code": code, "body": parsed if parsed != null else {}}

# --- auth ---
func register(email: String, password: String, display_name: String) -> bool:
	var r := await _request(HTTPClient.METHOD_POST, "/auth/register",
		{"email": email, "password": password, "display_name": display_name}, false)
	return _consume_tokens(r)

func login(email: String, password: String) -> bool:
	var r := await _request(HTTPClient.METHOD_POST, "/auth/login",
		{"email": email, "password": password, "device_id": OS.get_unique_id()}, false)
	if _consume_tokens(r):
		EventBus.logged_in.emit(_account_id)
		return true
	EventBus.login_failed.emit("invalid credentials")
	return false

func _refresh_tokens() -> bool:
	var r := await _request(HTTPClient.METHOD_POST, "/auth/refresh", {"refresh_token": _refresh}, false)
	return _consume_tokens(r)

func _consume_tokens(r: Dictionary) -> bool:
	if r.code != 200 and r.code != 201:
		return false
	_access = r.body.get("access_token", "")
	_refresh = r.body.get("refresh_token", "")
	return _access != ""

# --- collection / team ---
func list_creatures(cursor := "") -> Array:
	var path := "/creatures" + ("?cursor=" + cursor if cursor != "" else "")
	var r := await _authed(HTTPClient.METHOD_GET, path, {})
	return r.body if r.body is Array else []

func set_team(creature_ids: Array) -> bool:
	var r := await _authed(HTTPClient.METHOD_PUT, "/team", {"creatures": creature_ids})
	return r.code == 200

func evolve(creature_id: String) -> bool:
	var r := await _authed(HTTPClient.METHOD_POST, "/creatures/%s/evolve" % creature_id, {})
	return r.code == 200

# --- matchmaking ---
func queue_ranked() -> void:
	var r := await _authed(HTTPClient.METHOD_POST, "/matchmaking/queue", {"mode": "ranked"})
	if r.code == 202:
		var b: Dictionary = r.body
		EventBus.match_found.emit(b.get("session_id", ""), b.get("battle_ws", ""), b.get("ticket", ""))

# _authed wraps a request with a single transparent refresh-and-retry on 401.
func _authed(method: int, path: String, body: Dictionary) -> Dictionary:
	var r := await _request(method, path, body)
	if r.code == 401 and _refresh != "":
		if await _refresh_tokens():
			r = await _request(method, path, body)
	return r
