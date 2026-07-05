# GameData.gd — autoload singleton. Loads the dex from res://data/dex.json, holds
# the player's team, spawns wild creatures, and saves/loads progress.
# Dictionaries accessed with brackets ["key"] throughout (Godot-safe).
extends Node

var species_by_id := {}     # dex_id -> species dict
var skills_by_id := {}       # skill id -> skill dict
var starters := [1, 4, 7]    # Pyrolt (Fire), Driblet (Water), Sproutle (Grass)

var team: Array = []          # array of battler dicts
var seen := {}                # dex_id -> true
var caught_count := 0

func _ready() -> void:
	_load_dex()
	_load_save()

func _load_dex() -> void:
	var f := FileAccess.open("res://data/dex.json", FileAccess.READ)
	if f == null:
		push_error("dex.json not found")
		return
	var parsed = JSON.parse_string(f.get_as_text())
	f.close()
	if typeof(parsed) != TYPE_DICTIONARY:
		push_error("dex.json malformed")
		return
	for s in parsed.get("species", []):
		species_by_id[int(s["dex_id"])] = s
	for sk in parsed.get("skills", []):
		skills_by_id[String(sk["id"])] = sk

func moves_for(species: Dictionary, level: int) -> Array:
	var learned: Array = []
	for entry in species.get("learnset", []):
		if int(entry["level"]) <= level and skills_by_id.has(String(entry["skill_id"])):
			learned.append(skills_by_id[String(entry["skill_id"])])
	if learned.is_empty() and not skills_by_id.is_empty():
		learned = [skills_by_id.values()[0]]
	if learned.size() > 4:
		learned = learned.slice(learned.size() - 4, learned.size())
	return learned

func new_creature(dex_id: int, level: int) -> Dictionary:
	var sp: Dictionary = species_by_id[dex_id]
	var moves := moves_for(sp, level)
	return BattleEngine.make_battler(sp, level, moves)

func spawn_wild(player_level: int) -> Dictionary:
	# Weight heavily toward commoner rarities ((6-rarity)^3) so legendaries are a
	# rare thrill, and spawn wilds 1-3 levels BELOW the player so encounters are
	# winnable while your bonded lead keeps a training edge (balance-tested ~59% win).
	var pool: Array = []
	for id in species_by_id:
		var rarity := int(species_by_id[id].get("rarity", 0))
		var w: int = maxi(1, 6 - rarity)
		w = w * w * w
		for i in w:
			pool.append(id)
	var dex_id: int = pool[randi() % pool.size()]
	var lvl: int = maxi(2, player_level - (1 + randi() % 3))
	seen[dex_id] = true
	return new_creature(dex_id, lvl)

func add_to_team(battler: Dictionary) -> void:
	team.append(battler)
	caught_count += 1
	seen[int(battler["species"]["dex_id"])] = true
	save()

func heal_team() -> void:
	for c in team:
		c["cur_hp"] = int(c["max_hp"])
		c["status"] = 0
	save()

func save() -> void:
	var f := FileAccess.open("user://save.json", FileAccess.WRITE)
	if f == null:
		return
	var slim: Array = []
	for c in team:
		slim.append({"dex_id": int(c["species"]["dex_id"]), "level": int(c["level"]), "cur_hp": int(c["cur_hp"])})
	f.store_string(JSON.stringify({"team": slim, "seen": seen.keys(), "caught": caught_count}))
	f.close()

func _load_save() -> void:
	if not FileAccess.file_exists("user://save.json"):
		return
	var f := FileAccess.open("user://save.json", FileAccess.READ)
	var parsed = JSON.parse_string(f.get_as_text())
	f.close()
	if typeof(parsed) != TYPE_DICTIONARY:
		return
	caught_count = int(parsed.get("caught", 0))
	for id in parsed.get("seen", []):
		seen[int(id)] = true
	for entry in parsed.get("team", []):
		var c := new_creature(int(entry["dex_id"]), int(entry["level"]))
		c["cur_hp"] = int(entry.get("cur_hp", c["max_hp"]))
		team.append(c)

func has_save() -> bool:
	return not team.is_empty()

func reset() -> void:
	team.clear()
	seen.clear()
	caught_count = 0
	save()
