// Command profile runs the collection/inventory/team/trade service. It selects
// its persistence at boot: Postgres when DATABASE_URL is set (production), the
// in-memory repo otherwise (local/dev). Because both satisfy the same Repo
// interface, nothing downstream changes.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/aurelia/beastbound/pkg/creatures"
	"github.com/aurelia/beastbound/pkg/db"
	profile "github.com/aurelia/beastbound/services/profile/internal"
)

func main() {
	ctx := context.Background()

	dex, err := creatures.Load(
		envOr("DEX_SEED_PATH", "data/creatures/seed.json"),
		envOr("DEX_FLAGSHIP_PATH", "data/creatures/flagships.json"),
	)
	if err != nil {
		log.Fatalf("load dex: %v", err)
	}

	var repo profile.Repo
	if url := os.Getenv("DATABASE_URL"); url != "" {
		pool, err := db.Connect(ctx, url)
		if err != nil {
			log.Fatalf("postgres connect: %v", err)
		}
		defer pool.Close()
		repo = profile.NewPGRepo(pool)
		log.Println("profile: using Postgres persistence")
	} else {
		repo = profile.NewMemRepo()
		log.Println("profile: using in-memory persistence (dev)")
	}

	col := profile.NewCollection(repo, dex)
	trades := profile.NewTradeManager(repo, randomID)

	mux := http.NewServeMux()
	profile.NewHandlers(col, trades).Routes(mux)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) })
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) })

	addr := envOr("LISTEN_ADDR", ":8081")
	log.Printf("profile service listening on %s", addr)
	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	log.Fatal(srv.ListenAndServe())
}

func envOr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

// randomID is the trade-id generator; hex of a timestamp+counter is enough for a
// transient id (the DB assigns the durable UUID).
var counter int

func randomID() string {
	counter++
	return time.Now().Format("20060102T150405") + "-" + itoa(counter)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
