# BattleEngine.gd — self-contained, offline battle math for the phone game.
# GDScript port of the Go engine (same type chart, damage and stat formulas the
# server uses and the Node tests verify). Runs on-device; no backend needed.
# NOTE: dictionaries are accessed with brackets ["key"] (Godot-safe), never dot.
class_name BattleEngine
extends RefCounted

const ELEMENTS := ["Neutral","Fire","Water","Grass","Electric","Earth","Air","Ice","Toxin","Mind","Spectre","Metal"]

const TYPE_CHART := [
	[1,1,1,1,1,1,1,1,1,1,0.5,0.5],
	[1,0.5,0.5,2,1,1,1,2,1,1,1,2],
	[1,2,0.5,0.5,1,2,1,1,1,1,1,1],
	[1,0.5,2,0.5,1,2,0.5,1,0.5,1,1,0.5],
	[1,1,2,0.5,0.5,0,2,1,1,1,1,1],
	[1,2,0.5,0.5,2,1,0,1,2,1,1,2],
	[1,1,1,2,0.5,1,1,1,1,1,1,0.5],
	[1,0.5,0.5,2,1,2,2,0.5,1,1,1,0.5],
	[1,1,1,2,1,0.5,1,1,0.5,1,0.5,0],
	[1,1,1,1,1,1,1,1,2,0.5,0,1],
	[0,1,1,1,1,1,1,1,1,2,2,1],
	[1,0.5,0.5,1,0.5,1,1,2,1,1,1,0.5],
]

static func effectiveness(atk: int, d1: int, d2: int) -> float:
	var m: float = TYPE_CHART[atk][d1]
	if d2 != d1:
		m *= TYPE_CHART[atk][d2]
	return m

static func element_name(e: int) -> String:
	return ELEMENTS[e] if e >= 0 and e < ELEMENTS.size() else "?"

static func _stat(b: int, iv: int, level: int, is_hp: bool) -> int:
	var core: float = (2.0 * b + iv) * level / 100.0
	if is_hp:
		# HP gets 2x bulk so battles last a few turns (balance-tested), not one-shots.
		return int(core * 2.0) + level + 10
	return int(core) + 5

static func compute_stats(base: Dictionary, ivs: Dictionary, level: int) -> Dictionary:
	return {
		"hp":  _stat(int(base["hp"]), int(ivs["hp"]), level, true),
		"atk": _stat(int(base["attack"]), int(ivs["attack"]), level, false),
		"def": _stat(int(base["defense"]), int(ivs["defense"]), level, false),
		"spa": _stat(int(base["sp_attack"]), int(ivs["sp_attack"]), level, false),
		"spd": _stat(int(base["sp_defense"]), int(ivs["sp_defense"]), level, false),
		"spe": _stat(int(base["speed"]), int(ivs["speed"]), level, false),
	}

static func make_battler(species: Dictionary, level: int, skills: Array) -> Dictionary:
	var ivs := {"hp":randi()%32,"attack":randi()%32,"defense":randi()%32,
		"sp_attack":randi()%32,"sp_defense":randi()%32,"speed":randi()%32}
	var st := compute_stats(species["base"], ivs, level)
	return {
		"species": species,
		"name": String(species["name"]),
		"level": level,
		"e1": int(species["element1"]),
		"e2": int(species["element2"]),
		"max_hp": int(st["hp"]),
		"cur_hp": int(st["hp"]),
		"stats": st,
		"status": 0,
		"moves": skills,
	}

static func damage(attacker: Dictionary, defender: Dictionary, skill: Dictionary) -> Dictionary:
	var res := {"damage": 0, "crit": false, "eff": 1.0, "missed": false}
	if int(skill.get("class", 0)) == 2 or int(skill.get("power", 0)) <= 0:
		return res
	var acc := int(skill.get("accuracy", 100))
	if acc <= 100 and randi() % 100 >= acc:
		res["missed"] = true
		return res
	res["crit"] = (randi() % 100) < 6
	var is_special := int(skill.get("class", 0)) == 1
	var a: float = float(attacker["stats"]["spa"]) if is_special else float(attacker["stats"]["atk"])
	var d: float = float(defender["stats"]["spd"]) if is_special else float(defender["stats"]["def"])
	var lvl := int(attacker["level"])
	var base := (floor(2.0 * lvl / 5.0) + 2.0) * float(skill["power"]) * a / d / 50.0
	var dmg := floor(base) + 2.0
	var el := int(skill["element"])
	if el == int(attacker["e1"]) or el == int(attacker["e2"]):
		dmg *= 1.5
	res["eff"] = effectiveness(el, int(defender["e1"]), int(defender["e2"]))
	if res["eff"] == 0.0:
		res["damage"] = 0
		return res
	dmg *= res["eff"]
	if res["crit"]:
		dmg *= 1.5
	dmg *= randf_range(0.85, 1.0)
	res["damage"] = max(1, int(round(dmg)))
	return res

static func catch_chance(cur_hp: int, max_hp: int, catch_rate: int, ball_bonus: float = 1.0) -> float:
	var hp_factor := float(3 * max_hp - 2 * cur_hp) / float(3 * max_hp)
	var c := (float(catch_rate) / 255.0) * hp_factor * ball_bonus
	return clamp(c, 0.02, 0.95)

static func ai_pick_move(self_b: Dictionary, opp: Dictionary) -> int:
	var best := 0
	var best_score := -1.0
	var moves: Array = self_b["moves"]
	for i in moves.size():
		var mv: Dictionary = moves[i]
		if int(mv.get("class", 0)) == 2:
			continue
		var eff := effectiveness(int(mv["element"]), int(opp["e1"]), int(opp["e2"]))
		if eff == 0.0:
			continue
		var stab := 1.5 if (int(mv["element"]) == int(self_b["e1"]) or int(mv["element"]) == int(self_b["e2"])) else 1.0
		var score := float(mv.get("power", 0)) * eff * stab
		if score > best_score:
			best_score = score
			best = i
	return best
