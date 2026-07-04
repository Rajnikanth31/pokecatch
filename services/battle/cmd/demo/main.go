// Command demo runs a full PvP match END-TO-END through the authoritative netcode
// session — not the raw engine — with two AI bots as the "clients". It proves the
// real path: each bot only ever submits an *intent*, the Session collects both,
// resolves the turn via the pure engine, and broadcasts the event stream. This is
// the same code that a live WebSocket match uses; only the Transport differs
// (in-memory here vs gorilla WS in production).
//
//	go run ./services/battle/cmd/demo
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/aurelia/beastbound/pkg/creatures"
	"github.com/aurelia/beastbound/services/battle/internal/ai"
	"github.com/aurelia/beastbound/services/battle/internal/engine"
	"github.com/aurelia/beastbound/services/battle/internal/netcode"
	"github.com/aurelia/beastbound/services/battle/internal/persistence"
)

func main() {
	seedPath := envOr("DEX_SEED_PATH", "data/creatures/seed.json")
	flagPath := envOr("DEX_FLAGSHIP_PATH", "data/creatures/flagships.json")

	dex, err := creatures.Load(seedPath, flagPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "load dex:", err)
		os.Exit(1)
	}
	engine.SetRegistry(&engine.Registry{Skills: dex.Skills, Species: dex.Species})

	// Two mixed-type teams so type-effectiveness and switching matter.
	sideA := &engine.Side{PlayerID: "A", Party: []*engine.Battler{
		buildBattler(dex, 1, 32),  // Pyrolt (Fire)
		buildBattler(dex, 4, 32),  // Driblet (Water)
	}}
	sideB := &engine.Side{PlayerID: "B", Party: []*engine.Battler{
		buildBattler(dex, 7, 32),  // Sproutle (Grass)
		buildBattler(dex, 40, 32), // Voltail (Electric)
	}}

	// Collect the concrete instances so we can record a replayable match record.
	const seed = 0xC0FFEE
	teamA := instancesOf(sideA)
	teamB := instancesOf(sideB)
	rec := persistence.NewRecorder("demo-match", "casual", seed, teamA, teamB)

	mem := netcode.NewMemTransport("A", "B")
	sess := netcode.NewSession(netcode.Config{
		ID: "demo-match", Seed: seed, PlayerA: "A", PlayerB: "B",
		SideA: sideA, SideB: sideB, Transport: mem, TurnBudget: 2 * time.Second,
		// Record every resolved turn + the final result, then persist a replay
		// blob the anti-cheat tool (tools/replay) can re-simulate and verify.
		OnTurn: rec.RecordTurn,
		OnEnd: func(winner int, digest uint64) {
			record := rec.Finish(winner, digest)
			if blob, err := json.MarshalIndent(record, "", "  "); err == nil {
				if os.WriteFile("demo-match-record.json", blob, 0o644) == nil {
					fmt.Println("\nwrote demo-match-record.json — verify it with:")
					fmt.Println("  go run ./tools/replay --match demo-match-record.json")
				}
			}
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	go sess.Run(ctx)

	// Two bots act as clients: read their inbox, choose an intent, submit it.
	go runBot(sess, mem, "A", sideA, sideB, dex, 1)
	go runBot(sess, mem, "B", sideB, sideA, dex, 2)

	// Main goroutine narrates the spectator feed until the match ends.
	turn := 0
	for msg := range mem.Spectator {
		switch msg.Type {
		case "turn":
			turn = msg.Turn
			for _, ev := range msg.Events {
				fmt.Printf("  [T%02d] %-9s %s%s\n", turn, ev.Type, ev.Text, amount(ev.Amount))
			}
		case "end":
			fmt.Printf("\n=== MATCH OVER after %d turns — winner: side %d ===\n", turn, msg.Winner)
			return
		}
	}
}

// runBot is a minimal "client": on each state/turn message it picks an action via
// the server-side AI and submits it as an intent. A bot only ever sends intents —
// it never touches battle state, exactly like a real client.
func runBot(sess *netcode.Session, mem *netcode.MemTransport, pid string, self, opp *engine.Side, dex *creatures.Dex, seed uint64) {
	rng := engine.NewRNG(seed * 0x9E3779B1)
	for msg := range mem.Inbox[pid] {
		switch msg.Type {
		case "match_start", "turn", "state":
			act := decide(rng, self, opp, dex)
			sess.Submit(pid, netcode.Intent{Action: act})
		case "end":
			return
		}
	}
}

// decide turns the bot's current view into an engine action. If the active is
// fainted it force-switches to the first healthy bench member; otherwise it asks
// the battle AI for a move/switch.
func decide(rng ai.Randoms, self, opp *engine.Side, dex *creatures.Dex) engine.MoveAction {
	active := self.Party[self.Active]
	if active.Fainted() {
		for i, b := range self.Party {
			if !b.Fainted() {
				return engine.MoveAction{Kind: engine.ActSwitch, SwitchTo: i}
			}
		}
		return engine.MoveAction{Kind: engine.ActMove, SkillSlot: 0}
	}
	selfView, moveIdx := toView(active, dex)
	var bench []ai.View
	benchMap := []int{}
	for i, b := range self.Party {
		if i == self.Active {
			continue
		}
		v, _ := toView(b, dex)
		bench = append(bench, v)
		benchMap = append(benchMap, i)
	}
	oppView, _ := toView(opp.Party[opp.Active], dex)

	dec := ai.ChooseAction(rng, ai.Normal, selfView, bench, oppView)
	if dec.Switch {
		return engine.MoveAction{Kind: engine.ActSwitch, SwitchTo: benchMap[dec.SwitchTo]}
	}
	// Map the AI's move index (into the usable-moves list) back to the battler's
	// real skill slot.
	return engine.MoveAction{Kind: engine.ActMove, SkillSlot: moveIdx[dec.SkillIdx]}
}

// toView projects a Battler into the AI's View and returns the slot mapping from
// "usable move index" -> "battler skill slot".
func toView(b *engine.Battler, dex *creatures.Dex) (ai.View, []int) {
	var moves []creatures.Skill
	var slots []int
	for slot, id := range b.Inst.SkillIDs {
		if id == "" {
			continue
		}
		if sk, ok := dex.Skills[id]; ok {
			moves = append(moves, *sk)
			slots = append(slots, slot)
		}
	}
	hpFrac := 0.0
	if b.MaxHP > 0 {
		hpFrac = float64(b.CurHP) / float64(b.MaxHP)
	}
	return ai.View{
		DexID: b.Species.DexID, Element1: b.Species.Element1, Element2: b.Species.Element2,
		HPFrac: hpFrac, Speed: b.Stats.Speed, Status: b.Status, Moves: moves, Fainted: b.Fainted(),
	}, slots
}

// buildBattler instantiates a level-N member of a species, equipping up to four
// learnset moves it would know by that level and its first listed ability.
func buildBattler(dex *creatures.Dex, dexID, level int) *engine.Battler {
	sp := dex.Species[dexID]
	var ids [4]string
	n := 0
	for _, le := range sp.Learnset {
		if le.Level <= level && n < 4 {
			ids[n] = le.SkillID
			n++
		}
	}
	if n == 0 { // generated species have no learnset; give a generic strike
		ids[0] = firstSkill(dex)
	}
	ability := creatures.NoAbility
	if len(sp.Abilities) > 0 {
		ability = sp.Abilities[0]
	}
	maxIV := 31
	inst := &creatures.Instance{
		DexID: dexID, Level: level, Ability: ability, SkillIDs: ids,
		IVs:    creatures.UniformStats(maxIV),
		Nature: creatures.Nature{Name: "Hardy", Up: "atk", Down: "atk"},
	}
	return engine.NewBattler(sp, inst)
}

func firstSkill(dex *creatures.Dex) string {
	for id := range dex.Skills {
		return id
	}
	return ""
}

// instancesOf snapshots a side's battler instances for the match recorder.
func instancesOf(s *engine.Side) []creatures.Instance {
	out := make([]creatures.Instance, 0, len(s.Party))
	for _, b := range s.Party {
		out = append(out, *b.Inst)
	}
	return out
}

func amount(a int) string {
	if a == 0 {
		return ""
	}
	return fmt.Sprintf(" (%d)", a)
}

func envOr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
