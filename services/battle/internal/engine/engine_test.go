package engine

import (
	"testing"

	"github.com/aurelia/beastbound/pkg/creatures"
	"github.com/aurelia/beastbound/pkg/types"
)

// --- test fixtures ---------------------------------------------------------

func tackle() *creatures.Skill {
	return &creatures.Skill{ID: "tackle", Name: "Tackle", Element: types.Neutral, Class: types.Physical, Power: 40, Accuracy: 100, PP: 35}
}

func ember() *creatures.Skill {
	return &creatures.Skill{ID: "ember", Name: "Ember", Element: types.Fire, Class: types.Special, Power: 40, Accuracy: 100, PP: 25,
		Effect: &creatures.SkillEffect{Status: types.Burn, StatusChance: 10}}
}

func mkSpecies(dex int, e1, e2 types.Element, base types.Stats) *creatures.Species {
	return &creatures.Species{DexID: dex, Name: "Mon" + string(rune('A'+dex)), Element1: e1, Element2: e2, Base: base, CatchRate: 45, XPYield: 64}
}

func mkBattler(sp *creatures.Species, level int, skills ...string) *Battler {
	var ids [4]string
	copy(ids[:], skills)
	maxIV := types.Stats{HP: 31, Attack: 31, Defense: 31, SpAttack: 31, SpDefense: 31, Speed: 31}
	inst := &creatures.Instance{DexID: sp.DexID, Level: level, IVs: maxIV, Nature: Nature(creatures.Nature{Name: "Hardy", Up: "atk", Down: "atk"}), SkillIDs: ids}
	return NewBattler(sp, inst)
}

// Nature alias keeps the fixture readable.
type Nature = creatures.Nature

// --- type matrix -----------------------------------------------------------

func TestEffectivenessMatrixSymmetryAndValues(t *testing.T) {
	if got := types.Effectiveness(types.Water, types.Fire); got != 2.0 {
		t.Fatalf("Water vs Fire = %v, want 2.0", got)
	}
	if got := types.Effectiveness(types.Fire, types.Water); got != 0.5 {
		t.Fatalf("Fire vs Water = %v, want 0.5", got)
	}
	if got := types.Effectiveness(types.Electric, types.Earth); got != 0.0 {
		t.Fatalf("Electric vs Earth = %v, want 0.0 (immune)", got)
	}
	// Dual-type product: Grass attacking Water/Earth should stack to 4x.
	if got := types.DualEffectiveness(types.Grass, types.Water, types.Earth); got != 4.0 {
		t.Fatalf("Grass vs Water/Earth = %v, want 4.0", got)
	}
}

// --- stat computation ------------------------------------------------------

func TestComputeStatsKnownValue(t *testing.T) {
	// Base 100 HP, max IV, 0 EV, level 100 -> 2*100 +31 +0 =231; *100/100=231; +110 =341.
	sp := mkSpecies(1, types.Neutral, types.Neutral, types.Stats{HP: 100, Attack: 100, Defense: 100, SpAttack: 100, SpDefense: 100, Speed: 100})
	inst := &creatures.Instance{Level: 100, IVs: types.Stats{HP: 31, Attack: 31, Defense: 31, SpAttack: 31, SpDefense: 31, Speed: 31}, Nature: Nature{Name: "Hardy", Up: "atk", Down: "atk"}}
	got := creatures.ComputeStats(sp, inst)
	if got.HP != 341 {
		t.Fatalf("HP = %d, want 341", got.HP)
	}
	// Non-HP: 2*100+31=231; *100/100=231; +5=236.
	if got.Attack != 236 {
		t.Fatalf("Attack = %d, want 236", got.Attack)
	}
}

func TestNatureModifier(t *testing.T) {
	sp := mkSpecies(1, types.Neutral, types.Neutral, types.Stats{HP: 100, Attack: 100, Defense: 100, SpAttack: 100, SpDefense: 100, Speed: 100})
	inst := &creatures.Instance{Level: 100, IVs: types.Stats{HP: 31, Attack: 31, Defense: 31, SpAttack: 31, SpDefense: 31, Speed: 31}, Nature: Nature{Name: "Adamant", Up: "attack", Down: "sp_attack"}}
	got := creatures.ComputeStats(sp, inst)
	// Base non-HP stat at L100/maxIV is 236; Adamant applies +10%/-10% then
	// truncates: floor(236*1.1)=259, floor(236*0.9)=212.
	const wantAtk = 259
	const wantSpA = 212
	if got.Attack != wantAtk {
		t.Fatalf("boosted Attack = %d, want %d", got.Attack, wantAtk)
	}
	if got.SpAttack != wantSpA {
		t.Fatalf("reduced SpAttack = %d, want %d", got.SpAttack, wantSpA)
	}
}

// --- damage determinism & bounds ------------------------------------------

func TestDamageDeterministicForSeed(t *testing.T) {
	sp := mkSpecies(1, types.Fire, types.Neutral, types.Stats{HP: 80, Attack: 90, Defense: 70, SpAttack: 100, SpDefense: 70, Speed: 80})
	atk := mkBattler(sp, 50, "ember")
	def := mkBattler(mkSpecies(2, types.Grass, types.Neutral, sp.Base), 50, "tackle")

	r1, r2 := NewRNG(12345), NewRNG(12345)
	d1 := ComputeDamage(r1, atk, def, ember())
	d2 := ComputeDamage(r2, atk, def, ember())
	if d1.Damage != d2.Damage || d1.Crit != d2.Crit {
		t.Fatalf("non-deterministic: %+v vs %+v", d1, d2)
	}
	if d1.Effectiveness != 2.0 {
		t.Fatalf("Fire vs Grass effectiveness = %v, want 2.0", d1.Effectiveness)
	}
	if d1.STAB != 1.5 {
		t.Fatalf("STAB = %v, want 1.5", d1.STAB)
	}
}

func TestImmuneDealsNoDamage(t *testing.T) {
	atkSp := mkSpecies(1, types.Spectre, types.Neutral, types.Stats{HP: 80, Attack: 90, Defense: 70, SpAttack: 90, SpDefense: 70, Speed: 80})
	// Spectre attacking Neutral is 0x per the matrix.
	defSp := mkSpecies(2, types.Neutral, types.Neutral, atkSp.Base)
	atk := mkBattler(atkSp, 50)
	def := mkBattler(defSp, 50)
	sk := &creatures.Skill{Name: "Shadowtouch", Element: types.Spectre, Class: types.Special, Power: 60, Accuracy: 100}
	res := ComputeDamage(NewRNG(1), atk, def, sk)
	if res.Damage != 0 {
		t.Fatalf("immune hit dealt %d damage, want 0", res.Damage)
	}
}

func TestDamageScalesWithEffectiveness(t *testing.T) {
	// Same attacker/move, two defenders: super-effective should exceed resisted.
	base := types.Stats{HP: 100, Attack: 100, Defense: 100, SpAttack: 100, SpDefense: 100, Speed: 100}
	atk := mkBattler(mkSpecies(1, types.Water, types.Neutral, base), 50)
	weakDef := mkBattler(mkSpecies(2, types.Fire, types.Neutral, base), 50)   // Water>Fire 2x
	strongDef := mkBattler(mkSpecies(3, types.Grass, types.Neutral, base), 50) // Water<Grass 0.5x
	sk := &creatures.Skill{Name: "Jet", Element: types.Water, Class: types.Special, Power: 80, Accuracy: 100}

	// Fix seed so the random spread is identical for both.
	weak := ComputeDamage(NewRNG(99), atk, weakDef, sk)
	strong := ComputeDamage(NewRNG(99), atk, strongDef, sk)
	if !(weak.Damage > strong.Damage) {
		t.Fatalf("super-effective %d should exceed resisted %d", weak.Damage, strong.Damage)
	}
}

// --- status ----------------------------------------------------------------

func TestBurnHalvesPhysicalAndChips(t *testing.T) {
	base := types.Stats{HP: 100, Attack: 120, Defense: 80, SpAttack: 80, SpDefense: 80, Speed: 80}
	b := mkBattler(mkSpecies(1, types.Neutral, types.Neutral, base), 50)
	healthy := b.effAttack(types.Physical)
	b.Status = types.Burn
	burned := b.effAttack(types.Physical)
	if burned != healthy*0.5 {
		t.Fatalf("burn attack = %v, want half of %v", burned, healthy)
	}
	before := b.CurHP
	b.EndOfTurnTick()
	if b.CurHP >= before {
		t.Fatalf("burn should chip HP: before %d after %d", before, b.CurHP)
	}
}

func TestFireIsImmuneToBurn(t *testing.T) {
	b := mkBattler(mkSpecies(1, types.Fire, types.Neutral, types.Stats{HP: 100, Attack: 100, Defense: 100, SpAttack: 100, SpDefense: 100, Speed: 100}), 50)
	if b.applyStatus(types.Burn) {
		t.Fatal("Fire type should be immune to burn")
	}
}

func TestToxicEscalates(t *testing.T) {
	b := mkBattler(mkSpecies(1, types.Neutral, types.Neutral, types.Stats{HP: 160, Attack: 80, Defense: 80, SpAttack: 80, SpDefense: 80, Speed: 80}), 50)
	b.Status = types.Toxic
	b.StatusCtr = 1
	d1 := -b.EndOfTurnTick()
	d2 := -b.EndOfTurnTick()
	if !(d2 > d1) {
		t.Fatalf("toxic should escalate: %d then %d", d1, d2)
	}
}

// --- full turn loop --------------------------------------------------------

func TestBattleRunsToCompletionDeterministically(t *testing.T) {
	// Install a minimal registry so skillFor resolves.
	SetRegistry(&Registry{Skills: map[string]*creatures.Skill{
		"tackle": tackle(),
		"ember":  ember(),
	}})

	run := func(seed uint64) (int, int) {
		baseA := types.Stats{HP: 60, Attack: 90, Defense: 50, SpAttack: 70, SpDefense: 50, Speed: 90}
		baseB := types.Stats{HP: 60, Attack: 80, Defense: 55, SpAttack: 70, SpDefense: 55, Speed: 70}
		sideA := &Side{PlayerID: "A", Party: []*Battler{mkBattler(mkSpecies(1, types.Fire, types.Neutral, baseA), 30, "ember", "tackle")}}
		sideB := &Side{PlayerID: "B", Party: []*Battler{mkBattler(mkSpecies(2, types.Grass, types.Neutral, baseB), 30, "tackle")}}
		bt := NewBattle(seed, sideA, sideB)
		turns := 0
		for !bt.Over && turns < 100 {
			_, err := bt.ResolveTurn([2]MoveAction{{Kind: ActMove, SkillSlot: 0}, {Kind: ActMove, SkillSlot: 0}})
			if err != nil {
				t.Fatalf("turn error: %v", err)
			}
			turns++
		}
		if !bt.Over {
			t.Fatalf("battle did not conclude within 100 turns")
		}
		return bt.Winner, turns
	}

	w1, t1 := run(777)
	w2, t2 := run(777)
	if w1 != w2 || t1 != t2 {
		t.Fatalf("same seed diverged: (%d,%d) vs (%d,%d)", w1, t1, w2, t2)
	}
	// Fire side hits Grass super-effectively with STAB Ember and is faster, so it
	// should win this matchup.
	if w1 != 0 {
		t.Fatalf("expected Fire side (0) to win, got winner %d", w1)
	}
}

func TestDigestChangesAsBattleProgresses(t *testing.T) {
	SetRegistry(&Registry{Skills: map[string]*creatures.Skill{"tackle": tackle()}})
	base := types.Stats{HP: 120, Attack: 80, Defense: 60, SpAttack: 60, SpDefense: 60, Speed: 70}
	sideA := &Side{PlayerID: "A", Party: []*Battler{mkBattler(mkSpecies(1, types.Neutral, types.Neutral, base), 40, "tackle")}}
	sideB := &Side{PlayerID: "B", Party: []*Battler{mkBattler(mkSpecies(2, types.Neutral, types.Neutral, base), 40, "tackle")}}
	bt := NewBattle(2024, sideA, sideB)
	d0 := bt.Digest()
	bt.ResolveTurn([2]MoveAction{{Kind: ActMove, SkillSlot: 0}, {Kind: ActMove, SkillSlot: 0}})
	if bt.Digest() == d0 {
		t.Fatal("digest should change after a turn of damage")
	}
}
