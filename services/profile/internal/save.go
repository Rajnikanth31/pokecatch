// Package profile owns the player's persistent, authoritative state (account,
// collection, inventory, team) and the offline-save format. This file implements
// the save/load system. Two distinct concepts live here on purpose:
//
//   - AuthoritativeState: the server-owned record (collection, inventory, ranked
//     standing). The client never writes this directly; it is mutated only through
//     validated service operations and stored in Postgres.
//   - SaveBlob: a client-side, OFFLINE-ONLY snapshot (overworld position, story
//     flags, settings) used for single-player PvE continuity. It is versioned and
//     checksummed so it can't silently corrupt or be naively tampered into
//     authoritative advantage — anything competitive is reconciled from the server
//     on login, never trusted from the blob.
package profile

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
)

// CurrentSaveVersion is bumped whenever the SaveBlob shape changes; migrations in
// migrateSave bring older blobs forward so existing players never lose progress.
const CurrentSaveVersion = 3

// SaveBlob is the offline snapshot. Only non-authoritative fields belong here.
type SaveBlob struct {
	Version    int            `json:"version"`
	AccountID  string         `json:"account_id"`
	Region     string         `json:"region"`        // current overworld region id
	PosX       float64        `json:"pos_x"`
	PosY       float64        `json:"pos_y"`
	StoryFlags map[string]int `json:"story_flags"`   // quest/progress flags
	Settings   map[string]any `json:"settings"`      // accessibility, audio, input
	PlaytimeS  int64          `json:"playtime_s"`
}

// Envelope wraps a serialized SaveBlob with an integrity checksum. The checksum
// is over (version || payload) so truncation or bit-flips are detected on load.
type Envelope struct {
	Checksum string          `json:"checksum"` // hex sha256
	Payload  json.RawMessage `json:"payload"`
}

var (
	ErrCorruptSave  = errors.New("profile: save checksum mismatch (corrupt or tampered)")
	ErrFutureSave   = errors.New("profile: save version newer than this client")
	ErrEmptyPayload = errors.New("profile: empty save payload")
)

// Encode serializes a SaveBlob into a checksummed envelope ready to write to disk.
func Encode(blob SaveBlob) ([]byte, error) {
	blob.Version = CurrentSaveVersion
	payload, err := json.Marshal(blob)
	if err != nil {
		return nil, fmt.Errorf("marshal save: %w", err)
	}
	env := Envelope{Checksum: checksum(blob.Version, payload), Payload: payload}
	return json.Marshal(env)
}

// Decode validates the checksum, then migrates the payload to the current version.
// A corrupt or future-versioned save returns a typed error the client can surface
// gracefully (offer cloud-restore rather than crash).
func Decode(raw []byte) (SaveBlob, error) {
	var env Envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return SaveBlob{}, fmt.Errorf("unmarshal envelope: %w", err)
	}
	if len(env.Payload) == 0 {
		return SaveBlob{}, ErrEmptyPayload
	}
	// Peek the version to validate the checksum (which is over version||payload).
	var peek struct {
		Version int `json:"version"`
	}
	if err := json.Unmarshal(env.Payload, &peek); err != nil {
		return SaveBlob{}, fmt.Errorf("peek version: %w", err)
	}
	if checksum(peek.Version, env.Payload) != env.Checksum {
		return SaveBlob{}, ErrCorruptSave
	}
	if peek.Version > CurrentSaveVersion {
		return SaveBlob{}, ErrFutureSave
	}

	var blob SaveBlob
	if err := json.Unmarshal(env.Payload, &blob); err != nil {
		return SaveBlob{}, fmt.Errorf("unmarshal blob: %w", err)
	}
	return migrateSave(blob), nil
}

// migrateSave brings an older blob forward field-by-field. Each case falls
// through to the next so a v1 save is migrated v1->v2->v3 in one pass.
func migrateSave(b SaveBlob) SaveBlob {
	switch b.Version {
	case 1:
		// v1 -> v2: story flags became int-valued (were bool); default missing map.
		if b.StoryFlags == nil {
			b.StoryFlags = map[string]int{}
		}
		fallthrough
	case 2:
		// v2 -> v3: settings map introduced for accessibility options.
		if b.Settings == nil {
			b.Settings = map[string]any{}
		}
		fallthrough
	default:
		b.Version = CurrentSaveVersion
	}
	return b
}

func checksum(version int, payload []byte) string {
	h := sha256.New()
	var v [8]byte
	binary.LittleEndian.PutUint64(v[:], uint64(version))
	h.Write(v[:])
	h.Write(payload)
	return fmt.Sprintf("%x", h.Sum(nil))
}
