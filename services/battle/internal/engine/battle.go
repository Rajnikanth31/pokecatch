package engine

import (
	"errors"
	"hash/fnv"

	"github.com/aurelia/beastbound/pkg/creatures"
	"github.com/aurelia/beastbound/pkg/types"
)

// MoveAction is the only thing a client may submit per turn. The server never
// trusts anything else about state — it owns the Battlers and the RNG.
type MoveAction struct {
	Kind      ActionKind `json:"kind"`
	SkillSlot int        `json:"skill_slot"` // 0..3 for ActMove
	SwitchTo  int        `json:"switch_to"`  // party index for ActSwitch
}

type ActionKind uint8

const (
	ActMove ActionKind = iota
	ActSwitch
)

// Side is one player's team and active battler.
type Side struct {
	PlayerID string
	Party    []*Battler
	Active   int // index into Party
}

func (s *Side) activeBattler() *Battler { return s.Party[s.Active] }

func (s *Side) hasUsable() bool {
	for _, b := range s.Party {
		if !b.Fainted() {
			return true
		}
	}
	return false
}

// Battle is the authoritative match state. One goroutine owns one Battle; all
// mutation happens through ResolveTurn, which is pure given (state, actions).
type Battle struct {
	rng   *RNG
	Sides [2]*Side
	Turn  int
	Over  bool
	// Winner is the index of the winning side once Over; -1 while in progress.
	Winner int
	// log accumulates events for the current turn; drained by the netcode layer.
	log []Event
}

// Event is a structured battle log entry streamed to both clients and the
// spectator feed. Animations are driven entirely off these.
type Event struct {
	Type   string `json:"type"`
	Side   int    `json:"side,omitempty"`
	Text   string `json:"text"`
	Amount int    `json:"amount,omitempty"`
}

// NewBattle wires two sides together with a per-match seed.
func NewBattle(seed uint64, a, b *Side) *Battle {
	return &Battle{rng: NewRNG(seed), Sides: [2]*Side{a, b}, Winner: -1}
}

func (bt *Battle) emit(e Event) { bt.log = append(bt.log, e) }

// DrainLog returns and clears the accumulated events.
func (bt *Battle) DrainLog() []Event {
	out := bt.log
	bt.log = nil
	return out
}

var errBattleOver = errors.New("engine: battle already concluded")

// ResolveTurn applies one action from each side and advances the battle. Order
// is: switches first, then moves by (priority desc, effective speed desc, RNG
// tiebreak). Returns the events produced this turn.
func (bt *Battle) ResolveTurn(actions [2]MoveAction) ([]Event, error) {
	if bt.Over {
		return nil, errBattleOver
	}
	bt.Turn++

	// Phase 1: switches resolve before any attack, in side-index order.
	for i := 0; i < 2; i++ {
		if actions[i].Kind == ActSwitch {
			bt.doSwitch(i, actions[i].SwitchTo)
		}
	}

	// Phase 2: order the two attackers.
	order := bt.moveOrder(actions)
	for _, side := range order {
		if actions[side].Kind != ActMove {
			continue
		}
		if bt.Sides[side].activeBattler().Fainted() {
			continue // fainted before acting (e.g. from the other's move)
		}
		bt.executeMove(side, actions[side].SkillSlot)
		if bt.checkVictory() {
			break
		}
	}

	// Phase 3: end-of-turn residuals (status, regen) in speed order.
	if !bt.Over {
		bt.endOfTurn(order)
		bt.checkVictory()
	}

	// Tick cooldowns for both active battlers.
	for i := 0; i < 2; i++ {
		bt.Sides[i].activeBattler().tickCooldowns()
	}
	return bt.DrainLog(), nil
}

func (b *Battler) tickCooldowns() {
	for i := range b.cooldowns {
		if b.cooldowns[i] > 0 {
			b.cooldowns[i]--
		}
	}
}

// moveOrder returns side indices sorted by who acts first this turn.
func (bt *Battle) moveOrder(actions [2]MoveAction) [2]int {
	a, b := bt.Sides[0].activeBattler(), bt.Sides[1].activeBattler()
	pa, pb := priorityOf(actions[0], a), priorityOf(actions[1], b)
	first := 0
	switch {
	case pa != pb:
		if pb > pa {
			first = 1
		}
	default:
		sa, sb := a.effSpeed(), b.effSpeed()
		if sb > sa {
			first = 1
		} else if sb == sa && bt.rng.Chance(50) {
			first = 1 // speed tie broken by match RNG (deterministic)
		}
	}
	return [2]int{first, 1 - first}
}

func priorityOf(act MoveAction, b *Battler) int8 {
	if act.Kind != ActMove {
		return 6 // switches/items effectively act before moves
	}
	if act.SkillSlot < 0 || act.SkillSlot >= 4 {
		return 0
	}
	if sk := skillFor(b, act.SkillSlot); sk != nil {
		return sk.Priority
	}
	return 0
}

func (bt *Battle) doSwitch(side, to int) {
	s := bt.Sides[side]
	if to < 0 || to >= len(s.Party) || s.Party[to].Fainted() || to == s.Active {
		return // illegal switch ignored (server-authoritative validation)
	}
	// Reset volatile state on the outgoing battler.
	out := s.activeBattler()
	out.stages = stageBlock{}
	out.flinched = false
	s.Active = to
	bt.emit(Event{Type: "switch", Side: side, Text: s.Party[to].Species.Name + " enters the field"})
}

// executeMove runs one battler's chosen skill against the opposing active.
func (bt *Battle) executeMove(side, slot int) {
	atkSide := bt.Sides[side]
	defSide := bt.Sides[1-side]
	attacker := atkSide.activeBattler()
	defender := defSide.activeBattler()

	canAct, msg := attacker.PreMoveGate(bt.rng)
	if msg != "" {
		bt.emit(Event{Type: "status", Side: side, Text: attacker.Species.Name + " " + msg})
	}
	if !canAct {
		return
	}

	skill := skillFor(attacker, slot)
	if skill == nil || attacker.cooldowns[slot] > 0 {
		bt.emit(Event{Type: "fizzle", Side: side, Text: "no usable skill in slot"})
		return
	}
	if skill.Cooldown > 0 {
		attacker.cooldowns[slot] = skill.Cooldown
	}
	bt.emit(Event{Type: "move", Side: side, Text: attacker.Species.Name + " used " + skill.Name})

	// Levitate / Insulate hard immunities checked before damage.
	if isImmuneByAbility(defender, skill) {
		bt.emit(Event{Type: "immune", Side: 1 - side, Text: defender.Species.Name + " is unaffected"})
		return
	}

	res := ComputeDamage(bt.rng, attacker, defender, skill)
	if res.Missed {
		bt.emit(Event{Type: "miss", Side: side, Text: attacker.Species.Name + "'s attack missed"})
		return
	}

	if res.Damage > 0 {
		defender.CurHP -= res.Damage
		if defender.CurHP < 0 {
			defender.CurHP = 0
		}
		bt.emit(Event{Type: "damage", Side: 1 - side, Text: defender.Species.Name + " took damage", Amount: res.Damage})
		if res.Crit {
			bt.emit(Event{Type: "crit", Side: 1 - side, Text: "A critical hit!"})
		}
		if res.Effectiveness > 1 {
			bt.emit(Event{Type: "effective", Side: 1 - side, Text: "It's super effective!"})
		} else if res.Effectiveness < 1 && res.Effectiveness > 0 {
			bt.emit(Event{Type: "effective", Side: 1 - side, Text: "It's not very effective..."})
		}
	}

	bt.applyRiders(side, attacker, defender, skill, res)

	if defender.Fainted() {
		bt.emit(Event{Type: "faint", Side: 1 - side, Text: defender.Species.Name + " fainted"})
	}
	if attacker.Fainted() { // recoil KO
		bt.emit(Event{Type: "faint", Side: side, Text: attacker.Species.Name + " fainted"})
	}
}

// applyRiders handles status/stat/heal/recoil effects after damage.
func (bt *Battle) applyRiders(side int, attacker, defender *Battler, skill *creatures.Skill, res DamageResult) {
	eff := skill.Effect
	// Static ability: contact-ish moves may paralyze the attacker.
	if defender.hasAbility(creatures.Static) && skill.Class == types.Physical && res.Damage > 0 && bt.rng.Chance(30) {
		if attacker.applyStatus(types.Paralysis) {
			bt.emit(Event{Type: "status", Side: side, Text: attacker.Species.Name + " was paralyzed by Static"})
		}
	}
	if eff == nil {
		return
	}
	target := defender
	tSide := 1 - side
	if eff.TargetSelf {
		target, tSide = attacker, side
	}

	if eff.Status != types.None && bt.rng.Chance(orDefault(eff.StatusChance, 100)) {
		ok := false
		if eff.Status == types.Sleep {
			ok = target.SetSleep(bt.rng)
		} else {
			ok = target.applyStatus(eff.Status)
		}
		if ok {
			bt.emit(Event{Type: "status", Side: tSide, Text: target.Species.Name + " is now " + eff.Status.String()})
		}
	}
	for stat, delta := range eff.StatChanges {
		applyStatStage(target, stat, delta)
		bt.emit(Event{Type: "stat", Side: tSide, Text: target.Species.Name + " " + stat + " changed"})
	}
	if eff.HealFrac > 0 {
		base := res.Damage
		if skill.Class == types.StatusOnly {
			base = attacker.MaxHP
		}
		heal := int(float64(base) * eff.HealFrac)
		attacker.CurHP += heal
		if attacker.CurHP > attacker.MaxHP {
			attacker.CurHP = attacker.MaxHP
		}
		bt.emit(Event{Type: "heal", Side: side, Text: attacker.Species.Name + " recovered HP", Amount: heal})
	}
	if eff.RecoilFrac > 0 && res.Damage > 0 {
		recoil := int(float64(res.Damage) * eff.RecoilFrac)
		attacker.CurHP -= recoil
		if attacker.CurHP < 0 {
			attacker.CurHP = 0
		}
		bt.emit(Event{Type: "recoil", Side: side, Text: attacker.Species.Name + " is hurt by recoil", Amount: recoil})
	}
}

func (bt *Battle) endOfTurn(order [2]int) {
	for _, side := range order {
		b := bt.Sides[side].activeBattler()
		if d := b.EndOfTurnTick(); d != 0 {
			kind := "residual-damage"
			if d > 0 {
				kind = "residual-heal"
			}
			bt.emit(Event{Type: kind, Side: side, Text: b.Species.Name, Amount: d})
		}
		if b.Fainted() {
			bt.emit(Event{Type: "faint", Side: side, Text: b.Species.Name + " fainted"})
		}
	}
}

// checkVictory ends the battle if either side has no usable battlers. If only
// the active fainted but the bench is alive, the engine waits for a switch action
// next turn (handled by the session layer forcing a switch).
func (bt *Battle) checkVictory() bool {
	a, b := bt.Sides[0].hasUsable(), bt.Sides[1].hasUsable()
	switch {
	case !a && !b:
		bt.Over, bt.Winner = true, -1 // draw (double KO)
	case !a:
		bt.Over, bt.Winner = true, 1
	case !b:
		bt.Over, bt.Winner = true, 0
	}
	if bt.Over {
		bt.emit(Event{Type: "end", Text: "battle concluded"})
	}
	return bt.Over
}

// Digest produces a deterministic 64-bit hash of the full battle state. The
// server stores this each turn; a re-simulation from the seed must reproduce it,
// which is the backbone of replay-based anti-cheat.
func (bt *Battle) Digest() uint64 {
	h := fnv.New64a()
	var buf [8]byte
	put := func(v uint64) {
		for i := 0; i < 8; i++ {
			buf[i] = byte(v >> (8 * i))
		}
		h.Write(buf[:])
	}
	put(bt.rng.State())
	put(uint64(bt.Turn))
	for _, s := range bt.Sides {
		put(uint64(s.Active))
		for _, b := range s.Party {
			put(uint64(b.CurHP))
			put(uint64(b.Status))
		}
	}
	return h.Sum64()
}

// helpers ------------------------------------------------------------------

func orDefault(v, d int) int {
	if v == 0 {
		return d
	}
	return v
}

func applyStatStage(b *Battler, stat string, delta int8) {
	switch stat {
	case "attack":
		b.stages.atk = clampStage(b.stages.atk, delta)
	case "defense":
		b.stages.def = clampStage(b.stages.def, delta)
	case "sp_attack":
		b.stages.spa = clampStage(b.stages.spa, delta)
	case "sp_defense":
		b.stages.spd = clampStage(b.stages.spd, delta)
	case "speed":
		b.stages.spe = clampStage(b.stages.spe, delta)
	case "accuracy":
		b.stages.acc = clampStage(b.stages.acc, delta)
	case "evasion":
		b.stages.eva = clampStage(b.stages.eva, delta)
	}
}

func isImmuneByAbility(def *Battler, skill *creatures.Skill) bool {
	if def.hasAbility(creatures.Levitate) && skill.Element == types.Earth {
		return true
	}
	if def.hasAbility(creatures.Insulate) && skill.Element == types.Electric {
		return true
	}
	return false
}
