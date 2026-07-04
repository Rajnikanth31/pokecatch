package profile

import (
	"context"
	"strconv"
	"testing"

	"github.com/aurelia/beastbound/pkg/creatures"
	"github.com/aurelia/beastbound/pkg/types"
)

// testDex builds a tiny 2-stage evolution line: dex 1 --L16--> dex 2.
func testDex() *creatures.Dex {
	return &creatures.Dex{
		Species: map[int]*creatures.Species{
			1: {DexID: 1, Name: "Sproutle", Element1: types.Grass, Element2: types.Grass, EvolvesToID: 2, EvolveLevel: 16, Abilities: []creatures.PassiveAbility{creatures.Overgrow}},
			2: {DexID: 2, Name: "Bramblux", Element1: types.Grass, Element2: types.Grass, Abilities: []creatures.PassiveAbility{creatures.Overgrow}},
		},
		Skills: map[string]*creatures.Skill{},
	}
}

func addCreature(t *testing.T, repo Repo, id, owner string, dexID, level int) {
	t.Helper()
	oc := OwnedCreature{OwnerID: owner, Instance: creatures.Instance{InstanceID: id, DexID: dexID, Level: level}}
	if err := repo.AddCreature(context.Background(), oc); err != nil {
		t.Fatalf("add: %v", err)
	}
}

func TestSetTeamValidation(t *testing.T) {
	repo := NewMemRepo()
	col := NewCollection(repo, testDex())
	ctx := context.Background()
	for i := 0; i < 7; i++ {
		addCreature(t, repo, "c"+strconv.Itoa(i), "ash", 1, 5)
	}
	// >6 rejected
	if err := col.SetTeam(ctx, "ash", []string{"c0", "c1", "c2", "c3", "c4", "c5", "c6"}); err != ErrTeamTooBig {
		t.Fatalf("want ErrTeamTooBig, got %v", err)
	}
	// dup rejected
	if err := col.SetTeam(ctx, "ash", []string{"c0", "c0"}); err != ErrTeamDup {
		t.Fatalf("want ErrTeamDup, got %v", err)
	}
	// not-owner rejected
	addCreature(t, repo, "enemy", "gary", 1, 5)
	if err := col.SetTeam(ctx, "ash", []string{"c0", "enemy"}); err != ErrNotOwner {
		t.Fatalf("want ErrNotOwner, got %v", err)
	}
	// valid
	if err := col.SetTeam(ctx, "ash", []string{"c0", "c1", "c2"}); err != nil {
		t.Fatalf("valid team should save: %v", err)
	}
}

func TestEvolveRules(t *testing.T) {
	repo := NewMemRepo()
	col := NewCollection(repo, testDex())
	ctx := context.Background()

	addCreature(t, repo, "low", "ash", 1, 10)  // below L16
	addCreature(t, repo, "ready", "ash", 1, 20) // meets L16
	addCreature(t, repo, "final", "ash", 2, 40) // already final form

	if _, err := col.Evolve(ctx, "ash", "low"); err != ErrLevelTooLow {
		t.Fatalf("under-level should fail, got %v", err)
	}
	if _, err := col.Evolve(ctx, "ash", "final"); err != ErrCannotEvolve {
		t.Fatalf("final form should fail, got %v", err)
	}
	if _, err := col.Evolve(ctx, "gary", "ready"); err != ErrNotOwner {
		t.Fatalf("non-owner should fail, got %v", err)
	}
	out, err := col.Evolve(ctx, "ash", "ready")
	if err != nil {
		t.Fatalf("valid evolve: %v", err)
	}
	if out.Instance.DexID != 2 {
		t.Fatalf("should have evolved to dex 2, got %d", out.Instance.DexID)
	}
}

func TestPagination(t *testing.T) {
	repo := NewMemRepo()
	col := NewCollection(repo, testDex())
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		addCreature(t, repo, "c"+strconv.Itoa(i), "ash", 1, 5)
	}
	page1, cur, _ := col.List(ctx, "ash", "", 2)
	if len(page1) != 2 || cur == "" {
		t.Fatalf("page1 size %d cursor %q", len(page1), cur)
	}
	page2, _, _ := col.List(ctx, "ash", cur, 2)
	if len(page2) != 2 {
		t.Fatalf("page2 size %d", len(page2))
	}
	if page1[0].Instance.InstanceID == page2[0].Instance.InstanceID {
		t.Fatal("pages should not overlap")
	}
}

// TestTradeSwapAndAntiDupe is the important one: a successful escrowed swap, plus
// the anti-scam property that changing an offer clears prior locks.
func TestTradeSwapAndAntiDupe(t *testing.T) {
	repo := NewMemRepo()
	ctx := context.Background()
	addCreature(t, repo, "ash-mon", "ash", 1, 10)
	addCreature(t, repo, "gary-mon", "gary", 2, 40)

	id := 0
	tm := NewTradeManager(repo, func() string { id++; return "trade-" + strconv.Itoa(id) })
	tr := tm.Open("ash", "gary")

	if err := tm.Stage(ctx, tr.ID, "ash", []string{"ash-mon"}); err != nil {
		t.Fatalf("stage ash: %v", err)
	}
	if err := tm.Stage(ctx, tr.ID, "gary", []string{"gary-mon"}); err != nil {
		t.Fatalf("stage gary: %v", err)
	}
	// Ash can't stage a creature he doesn't own.
	if err := tm.Stage(ctx, tr.ID, "ash", []string{"gary-mon"}); err != ErrNotOwner {
		t.Fatalf("staging unowned should fail, got %v", err)
	}

	// Both lock, but then Ash changes his offer -> gary's lock must clear.
	_ = tm.Lock(tr.ID, "ash")
	_ = tm.Lock(tr.ID, "gary")
	_ = tm.Stage(ctx, tr.ID, "ash", []string{"ash-mon"}) // re-stage (change)
	if err := tm.Commit(ctx, tr.ID); err != ErrNotLocked {
		t.Fatalf("commit after offer change must fail (locks reset), got %v", err)
	}

	// Re-lock both and commit for real.
	_ = tm.Lock(tr.ID, "ash")
	_ = tm.Lock(tr.ID, "gary")
	if err := tm.Commit(ctx, tr.ID); err != nil {
		t.Fatalf("commit: %v", err)
	}
	// Ownership swapped.
	ashMon, _ := repo.GetCreature(ctx, "ash-mon")
	garyMon, _ := repo.GetCreature(ctx, "gary-mon")
	if ashMon.OwnerID != "gary" || garyMon.OwnerID != "ash" {
		t.Fatalf("ownership not swapped: ash-mon=%s gary-mon=%s", ashMon.OwnerID, garyMon.OwnerID)
	}
	// Double-commit must not re-run (anti-dupe): state is committed now.
	if err := tm.Commit(ctx, tr.ID); err != ErrTradeClosed {
		t.Fatalf("double commit should be rejected as closed, got %v", err)
	}
}
