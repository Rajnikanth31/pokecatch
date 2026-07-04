// Package token is a tiny, dependency-free HS256 JWT implementation shared by the
// auth service (which signs tokens) and the gateway (which verifies them on every
// request). It lives under pkg/ — not inside a service's internal/ tree — precisely
// so BOTH services can import it. Go's internal-package rule forbids the gateway
// from importing services/auth/internal, which is why this is here.
package token

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// Claims is the access-token payload. Kept deliberately small — the gateway only
// needs the subject (account id) and expiry to authorize a request.
type Claims struct {
	Sub string `json:"sub"` // account id
	Iat int64  `json:"iat"` // issued-at (unix)
	Exp int64  `json:"exp"` // expiry (unix)
	Ver int    `json:"ver"` // token version, lets us invalidate a generation
}

var (
	ErrTokenMalformed = errors.New("token: malformed")
	ErrTokenSignature = errors.New("token: signature invalid")
	ErrTokenExpired   = errors.New("token: expired")
)

// b64 is base64url without padding, per the JWT spec.
var b64 = base64.RawURLEncoding

// jwtHeader is constant for HS256; precomputed and cached.
var jwtHeader = b64.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))

// SignAccessToken produces a signed HS256 JWT for the given account.
func SignAccessToken(secret []byte, accountID string, ttl time.Duration, ver int) (string, error) {
	now := time.Now()
	claims := Claims{Sub: accountID, Iat: now.Unix(), Exp: now.Add(ttl).Unix(), Ver: ver}
	payloadJSON, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	payload := b64.EncodeToString(payloadJSON)
	signingInput := jwtHeader + "." + payload
	sig := b64.EncodeToString(sign(secret, signingInput))
	return signingInput + "." + sig, nil
}

// VerifyAccessToken validates signature and expiry, returning the claims. It uses
// a constant-time comparison to avoid timing attacks on the signature. This is the
// hot path the gateway calls on every request — pure CPU, no I/O.
func VerifyAccessToken(secret []byte, tok string) (Claims, error) {
	parts := strings.Split(tok, ".")
	if len(parts) != 3 {
		return Claims{}, ErrTokenMalformed
	}
	signingInput := parts[0] + "." + parts[1]
	expected := sign(secret, signingInput)
	got, err := b64.DecodeString(parts[2])
	if err != nil {
		return Claims{}, ErrTokenMalformed
	}
	if !hmac.Equal(expected, got) {
		return Claims{}, ErrTokenSignature
	}
	payloadJSON, err := b64.DecodeString(parts[1])
	if err != nil {
		return Claims{}, ErrTokenMalformed
	}
	var claims Claims
	if err := json.Unmarshal(payloadJSON, &claims); err != nil {
		return Claims{}, ErrTokenMalformed
	}
	if time.Now().Unix() >= claims.Exp {
		return Claims{}, ErrTokenExpired
	}
	return claims, nil
}

func sign(secret []byte, input string) []byte {
	m := hmac.New(sha256.New, secret)
	m.Write([]byte(input))
	return m.Sum(nil)
}
