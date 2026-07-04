package auth

import (
	"crypto/sha256"
	"encoding/hex"
)

// hashToken hashes a raw refresh token for storage/lookup. Refresh tokens are
// high-entropy random strings (not user secrets), so a fast SHA-256 is the right
// tool here — we need constant-cost lookup by hash, and there is nothing to
// brute-force. (Passwords, which ARE low-entropy, use argon2id instead.)
func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
