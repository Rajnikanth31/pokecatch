package netcode

// MemTransport is an in-process Transport implementation used by the local battle
// harness (services/battle/cmd/demo) and by session tests. It lets us exercise
// the entire authoritative turn loop — intent collection, resolution, broadcast,
// reconnection — with zero network, which is exactly what makes the netcode layer
// testable without sockets.

// MemTransport delivers ServerMessages to per-player buffered channels.
type MemTransport struct {
	Inbox     map[string]chan ServerMessage
	Spectator chan ServerMessage
}

// NewMemTransport wires buffered channels for the two named players.
func NewMemTransport(playerA, playerB string) *MemTransport {
	return &MemTransport{
		Inbox: map[string]chan ServerMessage{
			playerA: make(chan ServerMessage, 64),
			playerB: make(chan ServerMessage, 64),
		},
		Spectator: make(chan ServerMessage, 128),
	}
}

// Send implements Transport. Drops if the buffer is full (a stalled consumer must
// not block the authoritative loop) — the real WS transport behaves the same and
// relies on snapshot-on-resync to recover.
func (m *MemTransport) Send(playerID string, msg ServerMessage) error {
	if ch, ok := m.Inbox[playerID]; ok {
		select {
		case ch <- msg:
		default:
		}
	}
	return nil
}

// Spectate implements Transport.
func (m *MemTransport) Spectate(msg ServerMessage) {
	select {
	case m.Spectator <- msg:
	default:
	}
}
