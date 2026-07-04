// Package gateway is the single public ingress. Responsibilities (and ONLY
// these — it owns no game logic): TLS termination (handled by the LB in front),
// authentication, rate limiting, request id propagation, and reverse-proxying to
// the owning microservice. Keeping it logic-free means it can be scaled and
// redeployed independently of game changes.
package gateway

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"sync"
	"time"
)

// TokenVerifier validates a bearer token and returns the account id. In
// production this checks a JWT signature (offline, no network) and falls back to
// a Redis session lookup for revocation. The interface keeps the gateway
// testable and the crypto swappable.
type TokenVerifier interface {
	Verify(ctx context.Context, token string) (accountID string, err error)
}

// VerifierFunc adapts a plain func(token) (accountID, error) — such as a JWT
// verification closure — to the TokenVerifier interface, so callers don't need a
// named type just to plug in verification.
type VerifierFunc func(token string) (string, error)

// Verify implements TokenVerifier.
func (f VerifierFunc) Verify(_ context.Context, token string) (string, error) { return f(token) }

// RateLimiter is a token-bucket abstraction backed by Redis in prod, in-memory
// in tests.
type RateLimiter interface {
	Allow(ctx context.Context, key string, route string) bool
}

// Router maps a path prefix to an upstream base URL.
type Router struct {
	routes map[string]string // prefix -> upstream
}

func NewRouter() *Router {
	return &Router{routes: map[string]string{
		"/v1/auth":         "http://auth:8080",
		"/v1/profile":      "http://profile:8081",
		"/v1/creatures":    "http://profile:8081",
		"/v1/team":         "http://profile:8081",
		"/v1/matchmaking":  "http://matchmaking:8083",
		"/v1/trades":       "http://profile:8081",
		"/v1/leaderboard":  "http://leaderboard:8084",
	}}
}

func (r *Router) upstream(path string) (string, bool) {
	for prefix, up := range r.routes {
		if strings.HasPrefix(path, prefix) {
			return up, true
		}
	}
	return "", false
}

// Gateway is the http.Handler chain.
type Gateway struct {
	verifier TokenVerifier
	limiter  RateLimiter
	router   *Router
	proxy    func(upstream string, w http.ResponseWriter, r *http.Request)
}

func New(v TokenVerifier, l RateLimiter, p func(string, http.ResponseWriter, *http.Request)) *Gateway {
	return &Gateway{verifier: v, limiter: l, router: NewRouter(), proxy: p}
}

// publicPrefixes need no auth.
var publicPrefixes = []string{"/v1/auth", "/healthz", "/readyz"}

func isPublic(path string) bool {
	for _, p := range publicPrefixes {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	return false
}

func (g *Gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// 1. Auth (unless public).
	accountID := ""
	if !isPublic(r.URL.Path) {
		tok := bearer(r.Header.Get("Authorization"))
		if tok == "" {
			http.Error(w, `{"code":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		id, err := g.verifier.Verify(ctx, tok)
		if err != nil {
			http.Error(w, `{"code":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		accountID = id
		r.Header.Set("X-Account-Id", accountID) // trusted, set by gateway only
	}

	// 2. Rate limit (keyed by account when known, else client IP).
	key := accountID
	if key == "" {
		key = clientIP(r)
	}
	if !g.limiter.Allow(ctx, key, r.URL.Path) {
		w.Header().Set("Retry-After", "1")
		http.Error(w, `{"code":"rate_limited"}`, http.StatusTooManyRequests)
		return
	}

	// 3. Route.
	up, ok := g.router.upstream(r.URL.Path)
	if !ok {
		http.Error(w, `{"code":"not_found"}`, http.StatusNotFound)
		return
	}
	g.proxy(up, w, r)
}

func bearer(h string) string {
	const p = "Bearer "
	if strings.HasPrefix(h, p) {
		return strings.TrimSpace(h[len(p):])
	}
	return ""
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return strings.TrimSpace(strings.Split(xff, ",")[0])
	}
	host := r.RemoteAddr
	if i := strings.LastIndex(host, ":"); i > 0 {
		host = host[:i]
	}
	return host
}

// --- A simple in-memory token-bucket limiter for local/dev and tests --------

type MemoryLimiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	rate    int           // tokens per window
	window  time.Duration
}

type bucket struct {
	tokens int
	reset  time.Time
}

func NewMemoryLimiter(rate int, window time.Duration) *MemoryLimiter {
	return &MemoryLimiter{buckets: map[string]*bucket{}, rate: rate, window: window}
}

func (m *MemoryLimiter) Allow(_ context.Context, key, route string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	k := key + "|" + route
	b, ok := m.buckets[k]
	now := time.Now()
	if !ok || now.After(b.reset) {
		m.buckets[k] = &bucket{tokens: m.rate - 1, reset: now.Add(m.window)}
		return true
	}
	if b.tokens <= 0 {
		return false
	}
	b.tokens--
	return true
}

// HashToken is used when persisting refresh tokens so the DB never holds a usable
// secret (defense in depth for the account-security requirement).
func HashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
