package profile

import (
	"encoding/json"
	"testing"
)

func TestSaveRoundTrip(t *testing.T) {
	in := SaveBlob{
		AccountID: "abc", Region: "verdant_reach", PosX: 12.5, PosY: -3.0,
		StoryFlags: map[string]int{"act1_complete": 1}, Settings: map[string]any{"reduce_motion": true},
		PlaytimeS: 3600,
	}
	raw, err := Encode(in)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	out, err := Decode(raw)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Region != in.Region || out.PosX != in.PosX || out.StoryFlags["act1_complete"] != 1 {
		t.Fatalf("round trip mismatch: %+v", out)
	}
	if out.Version != CurrentSaveVersion {
		t.Fatalf("version = %d, want %d", out.Version, CurrentSaveVersion)
	}
}

func TestCorruptSaveDetected(t *testing.T) {
	raw, _ := Encode(SaveBlob{AccountID: "x"})
	// Flip a byte in the envelope payload to simulate corruption/tampering.
	var env Envelope
	_ = json.Unmarshal(raw, &env)
	tampered := append([]byte{}, env.Payload...)
	// Mutate the region by re-marshalling a different payload but keeping checksum.
	env.Payload = json.RawMessage(`{"version":3,"account_id":"x","region":"HACKED"}`)
	_ = tampered
	bad, _ := json.Marshal(env)
	if _, err := Decode(bad); err != ErrCorruptSave {
		t.Fatalf("expected ErrCorruptSave, got %v", err)
	}
}

func TestMigrationFromV1(t *testing.T) {
	// Hand-craft a v1 payload (no settings map) with a valid checksum.
	v1 := SaveBlob{Version: 1, AccountID: "old", Region: "emberfall"}
	payload, _ := json.Marshal(v1)
	env := Envelope{Checksum: checksum(1, payload), Payload: payload}
	raw, _ := json.Marshal(env)

	out, err := Decode(raw)
	if err != nil {
		t.Fatalf("decode v1: %v", err)
	}
	if out.Version != CurrentSaveVersion {
		t.Fatalf("migration should upgrade version, got %d", out.Version)
	}
	if out.Settings == nil || out.StoryFlags == nil {
		t.Fatalf("migration should backfill maps: %+v", out)
	}
}
