# Overworld.gd — minimal but real traversal + wild-encounter trigger. Movement
# uses Godot's default ui_* actions (arrows/WASD/gamepad/touch all resolve to the
# same actions). Stepping through tall grass rolls an encounter, which is the
# entry point to the battle scene. Spawn tables would come from the region data;
# here we roll uniformly over the 300-Dex for the slice.
extends Node2D

const SPEED := 220.0
const ENCOUNTER_CHANCE := 0.015   # per movement frame while walking

var _player: ColorRect

func _ready() -> void:
	_player = ColorRect.new()
	_player.size = Vector2(24, 24)
	_player.color = Color(0.35, 0.8, 1.0)
	_player.position = Vector2(628, 348)
	add_child(_player)

	var cam := Camera2D.new()
	add_child(cam)
	cam.make_current()

	EventBus.region_changed.emit("verdant_reach")

func _unhandled_input(event: InputEvent) -> void:
	# Enter/Start opens the collection + team builder.
	if event.is_action_pressed("ui_accept"):
		get_tree().change_scene_to_file("res://scenes/Collection.tscn")

func _process(delta: float) -> void:
	var dir := Vector2.ZERO
	dir.x = Input.get_axis("ui_left", "ui_right")
	dir.y = Input.get_axis("ui_up", "ui_down")
	if dir == Vector2.ZERO:
		return
	_player.position += dir.normalized() * SPEED * delta
	if randf() < ENCOUNTER_CHANCE:
		_start_encounter()

func _start_encounter() -> void:
	var dex_id := 1 + (randi() % 300)
	var level := 2 + (randi() % 8)
	EventBus.wild_encounter.emit(dex_id, level)
	get_tree().change_scene_to_file("res://scenes/Battle.tscn")
