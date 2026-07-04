package persistence

import (
	"testing"

	"github.com/aurelia/beastbound/pkg/creatures"
	"github.com/aurelia/beastbound/pkg/types"
	"github.com/aurelia/beastbound/services/battle/internal/engine"
)

// buildDex creates a minimal Dex with one tackle-like skill and two species.
func buildDex() *creatures.Dex {
	return &creatures.Dex{
		Species: map[int]*creatures.Species{
			1: {DexID: 1, Name: "A", Element1: types.Fire, Element2: types.Fire,
				Base: types.Stats{HP: 60, Attack: 90, Defense: 55, SpAttack: 70, SpDefense: 55, Speed: 90}},
			2: {DexID: 2, Name: "B", Element1: types.Grass, Element2: types.Grass,
				Base: types.Stats{HP: 60, Attack: 80, Defense: 60, SpAttack: 70, SpDefense: 60, Speed: 70}},
		},
		Skills: map[string]*creatures.Skill{
			"strike": {ID: "strike", Name: "Strike", Element: types.Fire, Class: types.Physical, Power: 60, Accuracy: 100},
		},
	}
}

func inst(id string, dexID, level int) creatures.Instance {
	return creatures.Instance{
		InstanceID: id, DexID: dexID, Level: level,
		IVs:      creatures.UniformStats(31),
		Nature:   creatures.Nature{Name: "Hardy", Up: "atk", Down: "atk"},
		SkillIDs: [4]string{"strike"},
	}
}

// playAndRecord runs a real battle while recording its intents, returning the
// record — exactly what the battle service would persist.
func playAndRecord(seed uint64, dex *creatures.Dex) MatchRecord {
	engine.SetRegistry(&engine.Registry{Skills: dex.Skills, Species: dex.Species})
	teamA := []creatures.Instance{inst("a1", 1, 30)}
	teamB := []creatures.Instance{inst("b1", 2, 30)}

	rec := NewRecorder("m1", "ranked", seed, teamA, teamB)
	a := &engine.Side{PlayerID: "A", Party: []*engine.Battler{engine.NewBattler(dex.Species[1], &teamA[0])}}
	b := &engine.Side{PlayerID: "B", Party: []*engine.Battler{engine.NewBattler(dex.Species[2], &teamB[0])}}
	bt := engine.NewBattle(seed, a, b)
	for !bt.Over && bt.Turn < 100 {
		actions := [2]engine.MoveAction{{Kind: engine.ActMove, SkillSlot: 0}, {Kind: engine.ActMove, SkillSlot: 0}}
		_, _ = bt.ResolveTurn(actions)
		rec.RecordTurn(actions)
	}
	return rec.Finish(bt.Winner, bt.Digest())
}

func TestReplayReproducesOutcome(t *testing.T) {
	dex := buildDex()
	rec := playAndRecord(0xABCDEF, dex)

	ok, err := Verify(rec, dex)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !ok {
		t.Fatal("a genuine record must verify as authentic")
	}
}

func TestReplayDetectsTamperedWinner(t *testing.T) {
	dex := buildDex()
	rec := playAndRecord(0xABCDEF, dex)

	// Simulate a cheater flipping the recorded result.
	rec.Winner = 1 - rec.Winner
	ok, _ := Verify(rec, dex)
	if ok {
		t.Fatal("tampered winner must fail verification")
	}
}

func TestReplayDetectsTamperedDigest(t *testing.T) {
	dex := buildDex()
	rec := playAndRecord(0xABCDEF, dex)
	rec.FinalDigest ^= 0xDEAD // corrupt the stored digest
	ok, _ := Verify(rec, dex)
	if ok {
		t.Fatal("tampered digest must fail verification")
	}
}

func TestReplayIsDeterministic(t *testing.T) {
	dex := buildDex()
	rec := playAndRecord(0x1234, dex)
	w1, d1, _, _ := Replay(rec, dex)
	w2, d2, _, _ := Replay(rec, dex)
	if w1 != w2 || d1 != d2 {
		t.Fatalf("replay not deterministic: (%d,%d) vs (%d,%d)", w1, d1, w2, d2)
	}
}
