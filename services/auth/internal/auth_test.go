package auth

import (
	"context"
	"testing"
	"time"
)

func newTestService() *Service {
	return NewService(NewMemStore(), Config{Secret: []byte("test-secret-please-change"), AccessTTL: time.Minute, RefreshTTL: time.Hour})
}

func TestJWTRoundTrip(t *testing.T) {
	secret := []byte("s3cr3t")
	tok, err := SignAccessToken(secret, "acc-1", time.Minute, 1)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	claims, err := VerifyAccessToken(secret, tok)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if claims.Sub != "acc-1" || claims.Ver != 1 {
		t.Fatalf("claims mismatch: %+v", claims)
	}
}

func TestJWTRejectsTamperAndWrongKey(t *testing.T) {
	tok, _ := SignAccessToken([]byte("right"), "acc-1", time.Minute, 1)
	if _, err := VerifyAccessToken([]byte("wrong"), tok); err != ErrTokenSignature {
		t.Fatalf("wrong key should fail signature, got %v", err)
	}
	if _, err := VerifyAccessToken([]byte("right"), tok+"x"); err == nil {
		t.Fatal("tampered token should not verify")
	}
}

func TestJWTExpiry(t *testing.T) {
	tok, _ := SignAccessToken([]byte("k"), "acc-1", -time.Second, 1) // already expired
	if _, err := VerifyAccessToken([]byte("k"), tok); err != ErrTokenExpired {
		t.Fatalf("expected expired, got %v", err)
	}
}

func TestPasswordHashVerify(t *testing.T) {
	h, err := HashPassword("correct horse battery")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	ok, _ := VerifyPassword("correct horse battery", h)
	if !ok {
		t.Fatal("correct password should verify")
	}
	bad, _ := VerifyPassword("wrong password!!", h)
	if bad {
		t.Fatal("wrong password must not verify")
	}
}

func TestPasswordMinLength(t *testing.T) {
	if _, err := HashPassword("short"); err != ErrPasswordShort {
		t.Fatalf("expected ErrPasswordShort, got %v", err)
	}
}

func TestRegisterLoginFlow(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()
	if _, err := svc.Register(ctx, "Ash@Aurelia.gg ", "trainerpass123", "Ash"); err != nil {
		t.Fatalf("register: %v", err)
	}
	// duplicate (email normalized) rejected
	if _, err := svc.Register(ctx, "ash@aurelia.gg", "trainerpass123", "Ash2"); err != ErrEmailTaken {
		t.Fatalf("expected ErrEmailTaken, got %v", err)
	}
	pair, err := svc.Login(ctx, "ash@aurelia.gg", "trainerpass123", "dev-1")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if _, err := VerifyAccessToken(svc.cfg.Secret, pair.AccessToken); err != nil {
		t.Fatalf("issued access token should verify: %v", err)
	}
	if _, err := svc.Login(ctx, "ash@aurelia.gg", "WRONG", "dev-1"); err != ErrBadCreds {
		t.Fatalf("bad password should fail, got %v", err)
	}
}

func TestRefreshRotationInvalidatesOldToken(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()
	pair, _ := svc.Register(ctx, "misty@aurelia.gg", "waterlover99", "Misty")

	rotated, err := svc.Refresh(ctx, pair.RefreshToken)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	// The old refresh token must now be single-use / revoked.
	if _, err := svc.Refresh(ctx, pair.RefreshToken); err != ErrRefreshBad {
		t.Fatalf("reused refresh token must be rejected, got %v", err)
	}
	// The new one works.
	if _, err := svc.Refresh(ctx, rotated.RefreshToken); err != nil {
		t.Fatalf("rotated token should work: %v", err)
	}
}
