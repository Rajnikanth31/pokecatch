//go:build integration

// Integration tests for the Postgres Repo. These run ONLY with the `integration`
// build tag and a live database:
//
//	DATABASE_URL=postgres://postgres:dev@localhost:5432/beastbound \
//	  go test -tags=integration ./services/profile/internal/
//
// The compose stack (deploy/docker/docker-compose.yml) provides a suitable DB.
// The unit tests (repo-agnostic, memory-backed) run without any of this; these
// exist specifically to prove the SQL and the transactional guarantees are real.
package profile

import (
	"context"
	"os"
	"testing"

	"github.com/aurelia/beastbound/pkg/creatures"
	"github.com/aurelia/beastbound/pkg/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

func mustPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set; skipping Postgres integration test")
	}
	pool, err := db.Connect(context.Background(), url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	return pool
}

// seed inserts the account + species rows the FKs require, and returns a cleanup.
func seed(t *testing.T, pool *pgxpool.Pool) func() {
	t.Helper()
	ctx := context.Background()
	exec := func(sql string, args ...any) {
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("seed exec: %v", err)
		}
	}
	exec(`INSERT INTO species (dex_id, name, rarity, element1, element2) VALUES
	        (1,'A',0,1,1),(2,'B',0,3,3) ON CONFLICT DO NOTHING`)
	exec(`INSERT INTO accounts (id, email, display_name) VALUES
	        ('11111111-1111-1111-1111-111111111111','ash-it@test.gg','Ash'),
	        ('22222222-2222-2222-2222-222222222222','gary-it@test.gg','Gary')
	      ON CONFLICT DO NOTHING`)
	return func() {
		exec(`DELETE FROM creature_instances WHERE owner_id IN
		        ('11111111-1111-1111-1111-111111111111','22222222-2222-2222-2222-222222222222')`)
	}
}

func itInst(id string, dex, level int) creatures.Instance {
	return creatures.Instance{InstanceID: id, DexID: dex, Level: level, IVs: creatures.UniformStats(31),
		Nature: creatures.Nature{Name: "Hardy", Up: "atk", Down: "atk"}, SkillIDs: [4]string{"strike"}}
}

func TestPGAddGetList(t *testing.T) {
	pool := mustPool(t)
	defer pool.Close()
	cleanup := seed(t, pool)
	defer cleanup()
	repo := NewPGRepo(pool)
	ctx := context.Background()

	const ash = "11111111-1111-1111-1111-111111111111"
	oc := OwnedCreature{OwnerID: ash, Instance: itInst("c1", 1, 12)}
	if err := repo.AddCreature(ctx, oc); err != nil {
		t.Fatalf("add: %v", err)
	}
	got, err := repo.GetCreature(ctx, "c1")
	if err != nil || got.OwnerID != ash || got.Instance.Level != 12 {
		t.Fatalf("get mismatch: %+v err=%v", got, err)
	}
	list, _, err := repo.ListCreatures(ctx, ash, "", 10)
	if err != nil || len(list) == 0 {
		t.Fatalf("list: %v (n=%d)", err, len(list))
	}
}

// TestPGTransferBatchAtomic is the critical one: a batch where the second
// transfer is invalid must roll back the first — no partial swap, no dupe.
func TestPGTransferBatchAtomic(t *testing.T) {
	pool := mustPool(t)
	defer pool.Close()
	cleanup := seed(t, pool)
	defer cleanup()
	repo := NewPGRepo(pool)
	ctx := context.Background()
	const ash = "11111111-1111-1111-1111-111111111111"
	const gary = "22222222-2222-2222-2222-222222222222"

	_ = repo.AddCreature(ctx, OwnedCreature{OwnerID: ash, Instance: itInst("ash1", 1, 5)})
	_ = repo.AddCreature(ctx, OwnedCreature{OwnerID: gary, Instance: itInst("gary1", 2, 5)})

	// Batch: valid transfer + an invalid one (wrong current owner). Must all fail.
	bad := []OwnershipTransfer{
		{CreatureID: "ash1", FromID: ash, ToID: gary},
		{CreatureID: "gary1", FromID: ash, ToID: gary}, // gary1 is owned by gary, not ash
	}
	if err := repo.TransferBatch(ctx, bad); err == nil {
		t.Fatal("batch with an invalid transfer must fail")
	}
	// ash1 must NOT have moved (rollback).
	ash1, _ := repo.GetCreature(ctx, "ash1")
	if ash1.OwnerID != ash {
		t.Fatalf("rollback failed: ash1 owner=%s want %s", ash1.OwnerID, ash)
	}

	// Now a valid swap commits fully.
	good := []OwnershipTransfer{
		{CreatureID: "ash1", FromID: ash, ToID: gary},
		{CreatureID: "gary1", FromID: gary, ToID: ash},
	}
	if err := repo.TransferBatch(ctx, good); err != nil {
		t.Fatalf("valid batch: %v", err)
	}
	ash1, _ = repo.GetCreature(ctx, "ash1")
	gary1, _ := repo.GetCreature(ctx, "gary1")
	if ash1.OwnerID != gary || gary1.OwnerID != ash {
		t.Fatalf("swap not applied: ash1=%s gary1=%s", ash1.OwnerID, gary1.OwnerID)
	}
}
