# Main.gd — app bootstrap. A full build opens a title/login screen here; the
# vertical slice logs in with a dev account (if the backend is reachable) and
# drops straight into the overworld so the loop is playable immediately.
extends Node

func _ready() -> void:
	# Optional dev auto-login (skips the title screen when env vars are set).
	if OS.has_environment("BB_DEV_EMAIL"):
		if await Api.login(OS.get_environment("BB_DEV_EMAIL"), OS.get_environment("BB_DEV_PASS")):
			get_tree().change_scene_to_file("res://scenes/Overworld.tscn")
			return
	get_tree().change_scene_to_file("res://scenes/Login.tscn")
