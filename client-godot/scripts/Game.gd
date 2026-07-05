# Game.gd — the whole playable game in one touch-friendly screen.
# States: TITLE -> STARTER -> HUB -> BATTLE. Built from UI controls so it works on
# a phone with no art and no keyboard. Offline, using BattleEngine + GameData.
# Dictionaries accessed with brackets ["key"] (Godot-safe).
extends Control

var content: VBoxContainer
var status_label: Label
var log_label: RichTextLabel
var enemy: Dictionary = {}
var active_idx := 0

var enemy_bar: ProgressBar
var enemy_lbl: Label
var player_bar: ProgressBar
var player_lbl: Label
var action_nodes: Array = []

const BIG := 30
const MED := 24

func _ready() -> void:
	var bg := ColorRect.new()
	bg.color = Color(0.09, 0.11, 0.16)
	bg.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	add_child(bg)

	# Persistent status line (never cleared). If the script runs at all, this
	# shows — so a truly blank screen means the APK didn't load this build.
	status_label = Label.new()
	status_label.text = "Aurelia: Beastbound"
	status_label.position = Vector2(16, 8)
	status_label.add_theme_font_size_override("font_size", 16)
	add_child(status_label)

	# Simple full-rect column with margins (no ScrollContainer — that nesting was
	# fragile). VBox stretches children to full width, so buttons are tappable.
	content = VBoxContainer.new()
	content.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	content.offset_left = 20
	content.offset_top = 44
	content.offset_right = -20
	content.offset_bottom = -20
	content.add_theme_constant_override("separation", 12)
	add_child(content)

	# Guard: if the creature data didn't ship in the APK, say so on screen instead
	# of failing silently later.
	if GameData.species_by_id.is_empty():
		status_label.text = "ERROR: dex.json not loaded"
		title("Creature data missing from build.", MED)
		title("(dex.json wasn't packaged)", 18)
		return

	if GameData.has_save():
		show_hub()
	else:
		show_title()

# ---------- UI helpers ----------
func clear() -> void:
	action_nodes.clear()
	for c in content.get_children():
		c.queue_free()

func title(text: String, size := BIG) -> void:
	var l := Label.new()
	l.text = text
	l.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	l.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	l.add_theme_font_size_override("font_size", size)
	content.add_child(l)

func button(text: String, cb: Callable) -> Button:
	var b := Button.new()
	b.text = text
	b.custom_minimum_size = Vector2(0, 60)
	# NOTE: Button has no autowrap_mode in Godot 4.2 (added in 4.3). Setting it
	# throws and the button never gets added — which is exactly why an earlier
	# build showed labels but no tappable buttons. Keep labels short instead.
	b.clip_text = true
	b.add_theme_font_size_override("font_size", MED)
	b.pressed.connect(cb)
	content.add_child(b)
	return b

func spacer(h := 8) -> void:
	var s := Control.new()
	s.custom_minimum_size = Vector2(0, h)
	content.add_child(s)

# ---------- TITLE ----------
func show_title() -> void:
	clear()
	title("AURELIA")
	title("Beastbound", MED)
	spacer(16)
	title("Catch. Train. Battle.", MED)
	spacer(20)
	button("New Game", show_starter)

# ---------- STARTER ----------
func show_starter() -> void:
	clear()
	title("Choose your first Anima")
	spacer(8)
	for id in GameData.starters:
		var sp: Dictionary = GameData.species_by_id[id]
		var t1 := BattleEngine.element_name(int(sp["element1"]))
		var did := int(id)
		button("%s   [%s]" % [String(sp["name"]), t1], func(): _pick_starter(did))

func _pick_starter(id: int) -> void:
	GameData.add_to_team(GameData.new_creature(id, 5))
	active_idx = 0
	show_hub()

# ---------- HUB ----------
func show_hub() -> void:
	clear()
	active_idx = _first_alive()
	if active_idx == -1:
		GameData.heal_team()
		active_idx = 0
	var lead: Dictionary = GameData.team[active_idx]
	title("Lead: %s  Lv%d" % [String(lead["name"]), int(lead["level"])])
	title("%s   HP %d/%d" % [_types_str(lead), int(lead["cur_hp"]), int(lead["max_hp"])], MED)
	spacer(4)
	title("Team %d   Seen %d/20" % [GameData.team.size(), GameData.seen.size()], MED)
	spacer(14)
	button("Explore", _explore)
	button("Team", show_team)
	button("Heal Team", func(): GameData.heal_team(); show_hub())

func _explore() -> void:
	enemy = GameData.spawn_wild(int(GameData.team[active_idx]["level"]))
	show_battle("A wild %s (Lv%d) appeared!" % [String(enemy["name"]), int(enemy["level"])])

# ---------- TEAM ----------
func show_team() -> void:
	clear()
	title("Your Team", BIG)
	title("(tap to make lead)", MED)
	spacer(6)
	for i in GameData.team.size():
		var c: Dictionary = GameData.team[i]
		var mark := "★ " if i == active_idx else ""
		var idx := i
		button("%s%s  Lv%d  HP %d/%d  [%s]" % [mark, String(c["name"]), int(c["level"]),
			int(c["cur_hp"]), int(c["max_hp"]), _types_str(c)],
			func(): _swap_lead(idx))
	spacer(10)
	button("Back", show_hub)

func _swap_lead(idx: int) -> void:
	var c: Dictionary = GameData.team[idx]
	GameData.team.remove_at(idx)
	GameData.team.insert(0, c)
	active_idx = 0
	GameData.save()
	show_team()

# ---------- BATTLE ----------
func show_battle(intro: String) -> void:
	clear()
	var me: Dictionary = GameData.team[active_idx]
	title("Wild %s  Lv%d" % [String(enemy["name"]), int(enemy["level"])], MED)
	enemy_lbl = Label.new()
	enemy_lbl.add_theme_font_size_override("font_size", 20)
	content.add_child(enemy_lbl)
	enemy_bar = _make_bar(int(enemy["max_hp"]), int(enemy["cur_hp"]))
	spacer(4)
	log_label = RichTextLabel.new()
	log_label.bbcode_enabled = true
	log_label.fit_content = true
	log_label.custom_minimum_size = Vector2(0, 130)
	log_label.add_theme_font_size_override("normal_font_size", 20)
	content.add_child(log_label)
	_log(intro)
	spacer(4)
	title("%s  Lv%d" % [String(me["name"]), int(me["level"])], MED)
	player_lbl = Label.new()
	player_lbl.add_theme_font_size_override("font_size", 20)
	content.add_child(player_lbl)
	player_bar = _make_bar(int(me["max_hp"]), int(me["cur_hp"]))
	spacer(8)
	_refresh_bars()
	_show_actions()

func _make_bar(maxv: int, val: int) -> ProgressBar:
	var pb := ProgressBar.new()
	pb.max_value = maxv
	pb.value = val
	pb.show_percentage = false
	pb.custom_minimum_size = Vector2(0, 24)
	content.add_child(pb)
	return pb

func _show_actions() -> void:
	_clear_actions()
	var me: Dictionary = GameData.team[active_idx]
	var moves: Array = me["moves"]
	for i in moves.size():
		var mv: Dictionary = moves[i]
		var idx := i
		var pw := int(mv.get("power", 0))
		var label := "%s  [%s]%s" % [String(mv["name"]), BattleEngine.element_name(int(mv["element"])),
			("  Pw%d" % pw) if pw > 0 else "  (status)"]
		_action_button(label, func(): _player_move(idx))
	_action_button("Catch", _try_catch)
	_action_button("Switch", _show_switch)
	_action_button("Run", _run)

func _action_button(text: String, cb: Callable) -> void:
	action_nodes.append(button(text, cb))

func _clear_actions() -> void:
	for n in action_nodes:
		if is_instance_valid(n):
			n.queue_free()
	action_nodes.clear()

func _player_move(move_idx: int) -> void:
	var me: Dictionary = GameData.team[active_idx]
	var mv: Dictionary = me["moves"][move_idx]
	if int(enemy["stats"]["spe"]) > int(me["stats"]["spe"]):
		_enemy_attack()
		if int(me["cur_hp"]) > 0:
			_apply(me, enemy, mv)
	else:
		_apply(me, enemy, mv)
		if int(enemy["cur_hp"]) > 0:
			_enemy_attack()
	_post_turn()

func _apply(attacker: Dictionary, defender: Dictionary, mv: Dictionary) -> void:
	var r := BattleEngine.damage(attacker, defender, mv)
	if bool(r["missed"]):
		_log("%s used %s — missed!" % [String(attacker["name"]), String(mv["name"])])
		return
	defender["cur_hp"] = max(0, int(defender["cur_hp"]) - int(r["damage"]))
	var extra := ""
	var eff := float(r["eff"])
	if eff > 1.0: extra = "  [color=yellow]Super effective![/color]"
	elif eff == 0.0: extra = "  [color=gray]No effect...[/color]"
	elif eff < 1.0: extra = "  [color=gray]Not very effective.[/color]"
	var crit := "  [color=orange]Critical![/color]" if bool(r["crit"]) else ""
	_log("%s used %s — %d dmg%s%s" % [String(attacker["name"]), String(mv["name"]), int(r["damage"]), crit, extra])
	_refresh_bars()

func _enemy_attack() -> void:
	var me: Dictionary = GameData.team[active_idx]
	if (enemy["moves"] as Array).is_empty():
		return
	var idx := BattleEngine.ai_pick_move(enemy, me)
	_apply(enemy, me, enemy["moves"][idx])

func _post_turn() -> void:
	_refresh_bars()
	if int(enemy["cur_hp"]) <= 0:
		_win()
	elif int(GameData.team[active_idx]["cur_hp"]) <= 0:
		_on_faint()
	else:
		_show_actions()

func _win() -> void:
	var me: Dictionary = GameData.team[active_idx]
	me["level"] = int(me["level"]) + 1
	me["max_hp"] = int(me["max_hp"]) + 3
	me["cur_hp"] = min(int(me["max_hp"]), int(me["cur_hp"]) + 3)
	GameData.save()
	_log("[color=lightgreen]%s fainted! %s grew to Lv%d![/color]" % [String(enemy["name"]), String(me["name"]), int(me["level"])])
	_continue_button()

func _try_catch() -> void:
	var chance := BattleEngine.catch_chance(int(enemy["cur_hp"]), int(enemy["max_hp"]), int(enemy["species"].get("catch_rate", 45)))
	_log("You throw a Bond Orb... (%d%% chance)" % int(chance * 100))
	if randf() < chance:
		var caught: Dictionary = enemy.duplicate(true)
		caught["cur_hp"] = int(caught["max_hp"])
		GameData.add_to_team(caught)
		_log("[color=lightgreen]Gotcha! %s was caught![/color]" % String(enemy["name"]))
		_continue_button()
	else:
		_log("[color=orange]%s broke free![/color]" % String(enemy["name"]))
		_enemy_attack()
		_post_turn()

func _on_faint() -> void:
	var nxt := _first_alive()
	if nxt == -1:
		_log("[color=red]All your Anima fainted! You scramble home...[/color]")
		GameData.heal_team()
		_continue_button()
	else:
		active_idx = nxt
		show_battle("Go, %s!" % String(GameData.team[active_idx]["name"]))

func _show_switch() -> void:
	_clear_actions()
	for i in GameData.team.size():
		var c: Dictionary = GameData.team[i]
		if int(c["cur_hp"]) <= 0 or i == active_idx:
			continue
		var idx := i
		_action_button("Send out %s (HP %d/%d)" % [String(c["name"]), int(c["cur_hp"]), int(c["max_hp"])],
			func(): _do_switch(idx))
	_action_button("Back", _show_actions)

func _do_switch(idx: int) -> void:
	active_idx = idx
	show_battle("You send out %s!" % String(GameData.team[idx]["name"]))
	_enemy_attack()
	_post_turn()

func _run() -> void:
	if randi() % 100 < 70:
		show_hub()
	else:
		_log("Couldn't get away!")
		_enemy_attack()
		_post_turn()

func _continue_button() -> void:
	_clear_actions()
	_action_button("Continue", show_hub)

# ---------- utils ----------
func _refresh_bars() -> void:
	if enemy_bar and is_instance_valid(enemy_bar):
		enemy_bar.value = int(enemy["cur_hp"])
		enemy_lbl.text = "HP  %d / %d" % [int(enemy["cur_hp"]), int(enemy["max_hp"])]
	var me: Dictionary = GameData.team[active_idx]
	if player_bar and is_instance_valid(player_bar):
		player_bar.value = int(me["cur_hp"])
		player_lbl.text = "HP  %d / %d" % [int(me["cur_hp"]), int(me["max_hp"])]

func _first_alive() -> int:
	for i in GameData.team.size():
		if int(GameData.team[i]["cur_hp"]) > 0:
			return i
	return -1

func _types_str(c: Dictionary) -> String:
	var a := BattleEngine.element_name(int(c["e1"]))
	var b := BattleEngine.element_name(int(c["e2"]))
	return a if a == b else "%s/%s" % [a, b]

func _log(msg: String) -> void:
	if log_label and is_instance_valid(log_label):
		log_label.append_text(msg + "\n")
