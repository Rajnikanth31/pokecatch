package auth

// Password hashing uses argon2id — the current best-practice memory-hard KDF and
// the right default for new systems (resistant to GPU/ASIC cracking in a way
// bcrypt/PBKDF2 are not). Parameters follow OWASP guidance and are encoded into
// the hash string so they can be raised over time without breaking old hashes
// (each hash is self-describing; verification reads its own params).
//
// Dependency: golang.org/x/crypto/argon2 (add via `go mod tidy`). This is the
// only crypto dep; everything else in auth is stdlib.

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// argon2Params are the tunable cost parameters. These are conservative defaults;
// bump memory/time as hardware improves — old hashes remain verifiable because
// their params travel with them.
type argon2Params struct {
	memoryKiB uint32
	time      uint32
	threads   uint8
	keyLen    uint32
	saltLen   uint32
}

var defaultParams = argon2Params{memoryKiB: 64 * 1024, time: 3, threads: 4, keyLen: 32, saltLen: 16}

var (
	ErrHashFormat    = errors.New("auth: unrecognized password hash format")
	ErrPasswordShort = errors.New("auth: password must be at least 10 characters")
)

// HashPassword returns a self-describing PHC-style argon2id hash string:
//   $argon2id$v=19$m=65536,t=3,p=4$<saltB64>$<hashB64>
func HashPassword(plaintext string) (string, error) {
	if len(plaintext) < 10 {
		return "", ErrPasswordShort
	}
	p := defaultParams
	salt := make([]byte, p.saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(plaintext), salt, p.time, p.memoryKiB, p.threads, p.keyLen)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, p.memoryKiB, p.time, p.threads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	), nil
}

// VerifyPassword checks a plaintext against a stored hash in constant time,
// re-deriving using the params embedded in the hash itself.
func VerifyPassword(plaintext, encoded string) (bool, error) {
	parts := strings.Split(encoded, "$")
	// ["", "argon2id", "v=19", "m=..,t=..,p=..", salt, hash]
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false, ErrHashFormat
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return false, ErrHashFormat
	}
	var p argon2Params
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.memoryKiB, &p.time, &p.threads); err != nil {
		return false, ErrHashFormat
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, ErrHashFormat
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, ErrHashFormat
	}
	got := argon2.IDKey([]byte(plaintext), salt, p.time, p.memoryKiB, p.threads, uint32(len(want)))
	return subtle.ConstantTimeCompare(want, got) == 1, nil
}
