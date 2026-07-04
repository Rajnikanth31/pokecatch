# BattleView.gd — the battle UI. It is a pure VIEW: it renders the authoritative
# event stream from the Battle autoload (BattleClient) and sends move intents back.
# It never computes damage or outcomes (server-authoritative; docs/07-security.md).
# For PvE the same view drives against a local/server AI opponent; for PvP the
# Battle socket is connected to a matched session.
extends Control

var _log: RichTextLabel
var _hp_you: ProgressBar
var _hp_foe: ProgressBar

func _ready() -> void:
	_hp_foe = _make_bar(Vector2(900, 60))
	_hp_you = _make_bar(Vector2(80, 520))

	_log = RichTextLabel.new()
	_log.bbcode_enabled = true
	_log.position = Vector2(80, 120)
	_log.size = Vector2(1120, 360)
	add_child(_log)

	for i in range(4):
		var b := Button.new()
		b.text = "Move %d" % (i + 1)
		b.position = Vector2(80 + i * 180, 620)
		b.custom_minimum_size = Vector2(160, 48)
		var slot := i
		b.pressed.connect(func() -> void: Battle.submit_move(slot))
		add_child(b)

	# Subscribe to the authoritative feed.
	Battle.battle_event.connect(_on_event)
	Battle.state_synced.connect(_on_state)
	Battle.battle_ended.connect(_on_end)
	_append("[i]Battle started — waiting for the server...[/i]")

func _make_bar(pos: Vector2) -> ProgressBar:
	var pb := ProgressBar.new()
	pb.position = pos
	pb.custom_minimum_size = Vector2(300, 24)
	pb.max_value = 100
	pb.value = 100
	add_child(pb)
	return pb

func _on_event(ev: Dictionary) -> void:
	var t: String = ev.get("type", "")
	var text: String = ev.get("text", "")
	var color := "white"
	match t:
		"crit", "effective": color = "yellow"
		"faint": color = "red"
		"heal": color = "lightgreen"
	_append("[color=%s]%s[/color]" % [color, text])

func _on_state(state: Dictionary) -> void:
	# Update HP bars from the redacted snapshot the server sent.
	var a: Dictionary = state.get("active_a", {})
	var b: Dictionary = state.get("active_b", {})
	if a.has("hp_max") and a.hp_max > 0:
		_hp_you.value = 100.0 * float(a.hp_cur) / float(a.hp_max)
	if b.has("hp_max") and b.hp_max > 0:
		_hp_foe.value = 100.0 * float(b.hp_cur) / float(b.hp_max)

func _on_end(winner: int) -> void:
	_append("[b]Battle over — winner: side %d[/b]" % winner)
	await get_tree().create_timer(2.0).timeout
	get_tree().change_scene_to_file("res://scenes/Overworld.tscn")

func _append(s: String) -> void:
	_log.append_text(s + "\n")
