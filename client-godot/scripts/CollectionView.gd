# CollectionView.gd — collection browser + team builder. Fetches the player's
# creatures from the profile service (paginated), lets them pick up to 6 for the
# active team, and saves via PUT /team. Team validity (≤6, no dupes, ownership) is
# ALSO enforced server-side — the client check is just for snappy UX.
extends Control

const MAX_TEAM := 6

var _list: ItemList
var _team_label: Label
var _team: Array[String] = []
var _creatures: Array = []       # raw dicts from the API
var _cursor := ""

func _ready() -> void:
	var title := Label.new(); title.text = "Collection — pick your team (max 6)"; title.position = Vector2(40, 20)
	add_child(title)

	_list = ItemList.new()
	_list.position = Vector2(40, 60)
	_list.custom_minimum_size = Vector2(600, 560)
	_list.item_selected.connect(_on_pick)
	add_child(_list)

	_team_label = Label.new(); _team_label.position = Vector2(680, 60); add_child(_team_label)

	var save_btn := Button.new(); save_btn.text = "Save Team"; save_btn.position = Vector2(680, 560)
	save_btn.pressed.connect(_on_save); add_child(save_btn)

	var back_btn := Button.new(); back_btn.text = "Back"; back_btn.position = Vector2(800, 560)
	back_btn.pressed.connect(func() -> void: get_tree().change_scene_to_file("res://scenes/Overworld.tscn"))
	add_child(back_btn)

	_refresh_team_label()
	await _load_page()

func _load_page() -> void:
	var page := await Api.list_creatures(_cursor)
	for c in page:
		_creatures.append(c)
		var name: String = c.get("nickname", "") if c.get("nickname", "") != "" else "Dex #%d" % int(c.get("dex_id", 0))
		_list.add_item("%s  (Lv %d)" % [name, int(c.get("level", 1))])
	# In a full build we'd load more on scroll; one page is enough for the slice.

func _on_pick(index: int) -> void:
	var id: String = _creatures[index].get("id", "")
	if id == "":
		return
	if _team.has(id):
		_team.erase(id)                 # toggle off
	elif _team.size() < MAX_TEAM:
		_team.append(id)                # toggle on (client-side cap; server re-checks)
	_refresh_team_label()

func _refresh_team_label() -> void:
	_team_label.text = "Team (%d/%d):\n%s" % [_team.size(), MAX_TEAM, "\n".join(_team)]

func _on_save() -> void:
	if await Api.set_team(_team):
		_team_label.text = "Saved!\n" + _team_label.text
	else:
		_team_label.text = "Save rejected by server.\n" + _team_label.text
