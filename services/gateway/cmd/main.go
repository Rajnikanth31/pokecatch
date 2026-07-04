// Command gateway is the public ingress. It verifies JWTs (offline, using the
// shared secret), rate-limits, and reverse-proxies to the owning microservice.
// It holds no game logic — see services/gateway/internal.
package main

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"time"

	"github.com/aurelia/beastbound/pkg/token"
	gw "github.com/aurelia/beastbound/services/gateway/internal"
)

func main() {
	secret := []byte(envOr("AUTH_JWT_SECRET", "dev-only-secret"))

	// The gateway verifies access tokens statelessly (pure CPU, no DB) using the
	// shared HS256 secret from pkg/token. VerifierFunc adapts that to the gateway's
	// TokenVerifier interface.
	verifier := gw.VerifierFunc(func(tok string) (string, error) {
		claims, err := token.VerifyAccessToken(secret, tok)
		if err != nil {
			return "", err
		}
		return claims.Sub, nil
	})

	limiter := gw.NewMemoryLimiter(120, time.Minute) // 120 req/min/key/route (dev)

	// proxy forwards to an upstream base URL, preserving the path.
	proxy := func(upstream string, w http.ResponseWriter, r *http.Request) {
		target, err := url.Parse(upstream)
		if err != nil {
			http.Error(w, `{"code":"bad_gateway"}`, http.StatusBadGateway)
			return
		}
		rp := httputil.NewSingleHostReverseProxy(target)
		rp.ServeHTTP(w, r)
	}

	handler := gw.New(verifier, limiter, proxy)

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) })
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) })
	mux.Handle("/", handler)

	addr := envOr("LISTEN_ADDR", ":8088")
	log.Printf("gateway listening on %s", addr)
	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	log.Fatal(srv.ListenAndServe())
}

func envOr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
