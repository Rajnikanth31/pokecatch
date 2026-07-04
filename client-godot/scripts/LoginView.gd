# LoginView.gd — title/login screen. Builds its UI in code (no texture deps) so
# the project runs immediately. On success it stores tokens in the Api singleton
# and advances to the overworld. Offline "Play as Guest" skips straight in for
# local PvE against the AI.
extends Control

var _email: LineEdit
var _pass: LineEdit
var _status: Label

func _ready() -> void:
	var box := VBoxContainer.new()
	box.position = Vector2(440, 220)
	box.custom_minimum_size = Vector2(400, 0)
	add_child(box)

	var title := Label.new()
	title.text = "AURELIA: BEASTBOUND"
	box.add_child(title)

	_email = LineEdit.new(); _email.placeholder_text = "email"; box.add_child(_email)
	_pass = LineEdit.new(); _pass.placeholder_text = "password"; _pass.secret = true; box.add_child(_pass)

	var login_btn := Button.new(); login_btn.text = "Log in"; box.add_child(login_btn)
	var reg_btn := Button.new(); reg_btn.text = "Register"; box.add_child(reg_btn)
	var guest_btn := Button.new(); guest_btn.text = "Play as Guest (offline)"; box.add_child(guest_btn)

	_status = Label.new(); box.add_child(_status)

	login_btn.pressed.connect(_on_login)
	reg_btn.pressed.connect(_on_register)
	guest_btn.pressed.connect(func() -> void: _go_overworld())

	EventBus.login_failed.connect(func(reason: String) -> void: _status.text = "Login failed: " + reason)

func _on_login() -> void:
	_status.text = "Signing in..."
	if await Api.login(_email.text, _pass.text):
		_go_overworld()

func _on_register() -> void:
	_status.text = "Creating account..."
	if await Api.register(_email.text, _pass.text, _email.text.split("@")[0]):
		_go_overworld()
	else:
		_status.text = "Registration failed (email taken or password < 10 chars)"

func _go_overworld() -> void:
	get_tree().change_scene_to_file("res://scenes/Overworld.tscn")
