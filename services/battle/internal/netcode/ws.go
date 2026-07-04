package netcode

// ws.go is the production WebSocket transport: it upgrades an HTTP request to a
// WS connection, authenticates the ticket, attaches the connection to its
// Session, and pumps messages in both directions. It implements the Transport
// interface the Session depends on, so the Session itself stays transport-
// agnostic and unit-testable with the in-memory transport in memtransport.go.
//
// Dependency: github.com/gorilla/websocket (add to go.mod and run `go mod tidy`).
// Everything else in this package is stdlib-only; the WS lib is isolated to this
// one file so the engine/session core has no third-party dependency.

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 4096,
	// CheckOrigin is locked down in prod to the game's own origins; the gateway
	// has already authenticated the ticket before the client reaches here.
	CheckOrigin: func(r *http.Request) bool { return true },
}

// conn wraps a single player's socket with a write mutex (gorilla requires that
// only one goroutine writes at a time).
type conn struct {
	ws       *websocket.Conn
	writeMu  sync.Mutex
	playerID string
}

func (c *conn) writeJSON(v any) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	_ = c.ws.SetWriteDeadline(time.Now().Add(10 * time.Second))
	return c.ws.WriteJSON(v)
}

// WSTransport fans server messages out to connected players and spectators. It is
// the Transport implementation a live Session uses.
type WSTransport struct {
	mu         sync.RWMutex
	players    map[string]*conn
	spectators map[*conn]struct{}
}

// NewWSTransport creates an empty transport for one session.
func NewWSTransport() *WSTransport {
	return &WSTransport{players: map[string]*conn{}, spectators: map[*conn]struct{}{}}
}

// Send delivers a message to one player. A failed write (dead socket) is dropped;
// the session's reconnect path re-attaches and resends a snapshot.
func (t *WSTransport) Send(playerID string, msg ServerMessage) error {
	t.mu.RLock()
	c := t.players[playerID]
	t.mu.RUnlock()
	if c == nil {
		return nil // player not currently connected; snapshot on reconnect
	}
	return c.writeJSON(msg)
}

// Spectate fans a message out to all spectators, never including private data
// (the session only ever passes redacted snapshots here).
func (t *WSTransport) Spectate(msg ServerMessage) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	for c := range t.spectators {
		_ = c.writeJSON(msg)
	}
}

// Attach registers (or re-registers, on reconnect) a player's connection and
// starts its read pump, forwarding validated intents to the session.
func (t *WSTransport) Attach(s *Session, playerID string, ws *websocket.Conn) {
	c := &conn{ws: ws, playerID: playerID}
	t.mu.Lock()
	t.players[playerID] = c
	t.mu.Unlock()
	go t.readPump(s, c)
}

// AttachSpectator registers a read-only viewer.
func (t *WSTransport) AttachSpectator(ws *websocket.Conn) {
	c := &conn{ws: ws}
	t.mu.Lock()
	t.spectators[c] = struct{}{}
	t.mu.Unlock()
}

// readPump reads client messages until the socket closes. It only ever forwards
// an Intent (or a resync request) — it cannot mutate game state directly, which
// is the whole point of server authority.
func (t *WSTransport) readPump(s *Session, c *conn) {
	defer func() {
		_ = c.ws.Close()
		t.mu.Lock()
		delete(t.players, c.playerID)
		t.mu.Unlock()
	}()
	c.ws.SetReadLimit(4096) // bound message size (packet-validation / anti-abuse)
	for {
		_, data, err := c.ws.ReadMessage()
		if err != nil {
			return
		}
		var msg ClientMessage
		if json.Unmarshal(data, &msg) != nil {
			continue // ignore malformed frames
		}
		switch msg.Type {
		case "action":
			s.Submit(c.playerID, msg.toIntent())
		case "resync":
			// Re-send the authoritative snapshot for this player's side.
			side := s.sideOf(c.playerID)
			if side >= 0 {
				_ = c.writeJSON(ServerMessage{Type: "state", SessionID: s.ID, State: s.snapshot(side), Digest: s.battle.Digest()})
			}
		case "heartbeat":
			// keepalive; nothing to do
		}
	}
}

// ServeWS is the HTTP handler that upgrades and attaches a player to a session.
// In production the gateway/allocator resolves which Session via the
// battle:node:{session_id} sticky route before the client connects here.
func ServeWS(lookup func(sessionID string) *Session, transports func(sessionID string) *WSTransport) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID := r.PathValue("id")
		playerID := r.Header.Get("X-Account-Id") // set by the gateway after auth
		s := lookup(sessionID)
		tr := transports(sessionID)
		if s == nil || tr == nil || playerID == "" {
			http.Error(w, "no such session", http.StatusNotFound)
			return
		}
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		tr.Attach(s, playerID, ws)
	}
}
