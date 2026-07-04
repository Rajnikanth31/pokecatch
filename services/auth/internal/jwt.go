package auth

// The JWT implementation now lives in the shared pkg/token package so the gateway
// can verify tokens without importing this service's internal tree (Go forbids
// importing another module path's internal/ packages). These aliases keep the
// auth package's public surface (and its tests) unchanged.

import "github.com/aurelia/beastbound/pkg/token"

// Claims re-exports the shared token claims.
type Claims = token.Claims

// Signing/verification are re-exported so callers in this package (service.go)
// and this package's tests keep using auth.SignAccessToken / auth.VerifyAccessToken.
var (
	SignAccessToken   = token.SignAccessToken
	VerifyAccessToken = token.VerifyAccessToken
)

// Error values re-exported for the auth package's tests and error mapping.
var (
	ErrTokenMalformed = token.ErrTokenMalformed
	ErrTokenSignature = token.ErrTokenSignature
	ErrTokenExpired   = token.ErrTokenExpired
)
