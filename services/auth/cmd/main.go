// Command auth runs the authentication service. In production the Store is the
// Postgres implementation and the secret comes from the secret manager; this
// wiring uses env + the in-memory store for local runs.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/aurelia/beastbound/pkg/db"
	auth "github.com/aurelia/beastbound/services/auth/internal"
)

func main() {
	ctx := context.Background()
	secret := os.Getenv("AUTH_JWT_SECRET")
	if secret == "" {
		secret = "dev-only-insecure-secret-change-me"
		log.Println("WARNING: using insecure dev JWT secret")
	}

	// Select persistence: Postgres in prod (DATABASE_URL), memory for local dev.
	var store auth.Store
	if url := os.Getenv("DATABASE_URL"); url != "" {
		pool, err := db.Connect(ctx, url)
		if err != nil {
			log.Fatalf("postgres connect: %v", err)
		}
		defer pool.Close()
		store = auth.NewPGStore(pool)
		log.Println("auth: using Postgres persistence")
	} else {
		store = auth.NewMemStore()
		log.Println("auth: using in-memory persistence (dev)")
	}

	svc := auth.NewService(store, auth.Config{
		Secret: []byte(secret), AccessTTL: 15 * time.Minute, RefreshTTL: 30 * 24 * time.Hour,
	})

	mux := http.NewServeMux()
	auth.NewHandlers(svc).Routes(mux)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) })
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) })

	addr := envOr("LISTEN_ADDR", ":8080")
	log.Printf("auth service listening on %s", addr)
	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	log.Fatal(srv.ListenAndServe())
}

func envOr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
