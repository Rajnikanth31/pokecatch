// Command battle is the authoritative real-time battle server. It loads the
// immutable creature Dex once, then accepts WebSocket connections for matches
// that the matchmaking service has already paired. Each match runs in its own
// goroutine via netcode.Session; the server is horizontally scalable because a
// match's authoritative state lives entirely in one process and clients are
// sticky-routed to it via Redis (battle:node:{session_id}).
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aurelia/beastbound/pkg/creatures"
	"github.com/aurelia/beastbound/services/battle/internal/engine"
)

func main() {
	seed := envOr("DEX_SEED_PATH", "data/creatures/seed.json")
	flag := envOr("DEX_FLAGSHIP_PATH", "data/creatures/flagships.json")

	dex, err := creatures.Load(seed, flag)
	if err != nil {
		log.Fatalf("failed to load creature dex: %v", err)
	}
	// Install the immutable registry the engine reads from. Done once; the Dex is
	// read-only for the process lifetime so this is concurrency-safe.
	engine.SetRegistry(&engine.Registry{Skills: dex.Skills, Species: dex.Species})
	log.Printf("loaded dex: %d species, %d skills", len(dex.Species), len(dex.Skills))

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) })
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) })
	// /v1/battle/{id} upgrade handler is wired in netcode/ws.go (omitted: requires
	// gorilla/websocket); here we expose the lifecycle + health surface.

	srv := &http.Server{
		Addr:              envOr("LISTEN_ADDR", ":8082"),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("battle server listening on %s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	<-ctx.Done()
	// Graceful drain: stop accepting, let in-flight matches finish a turn.
	shutCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutCtx)
	log.Println("battle server stopped")
}

func envOr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
