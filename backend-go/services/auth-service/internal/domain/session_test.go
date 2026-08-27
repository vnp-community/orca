package domain

import (
	"testing"
	"time"
)

func TestNewSession_ValidatesInvariants(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name      string
		tokenHash string
		userID    string
		tenantID  string
		expiresAt time.Time
		wantErr   error
	}{
		{"valid", "hash1", "u1", "t1", now.Add(time.Hour), nil},
		{"empty hash", "", "u1", "t1", now.Add(time.Hour), ErrEmptyTokenHash},
		{"empty user", "hash1", "", "t1", now.Add(time.Hour), ErrEmptyUser},
		{"empty tenant", "hash1", "u1", "", now.Add(time.Hour), ErrEmptyTenant},
		{"zero expiry", "hash1", "u1", "t1", time.Time{}, ErrZeroExpiry},
		{"expiry before creation", "hash1", "u1", "t1", now.Add(-time.Hour), ErrExpiryBeforeCreation},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewSession(tt.tokenHash, tt.userID, tt.tenantID, now, tt.expiresAt)
			if tt.wantErr == nil && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if tt.wantErr != nil && err != tt.wantErr {
				t.Fatalf("expected %v, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestSession_IsValid(t *testing.T) {
	now := time.Now()
	s, err := NewSession("hash1", "u1", "t1", now, now.Add(time.Hour))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !s.IsValid(now) {
		t.Error("expected a freshly created session to be valid")
	}
	if s.IsValid(now.Add(2 * time.Hour)) {
		t.Error("expected an expired session to be invalid")
	}

	revokedAt := now.Add(time.Minute)
	s.RevokedAt = &revokedAt
	if s.IsValid(now.Add(time.Second)) {
		t.Error("expected a revoked session to be invalid even before its expiry")
	}
}

func TestHashSessionToken_IsDeterministicAndDistinct(t *testing.T) {
	h1 := HashSessionToken("raw-token-a")
	h2 := HashSessionToken("raw-token-a")
	h3 := HashSessionToken("raw-token-b")

	if h1 != h2 {
		t.Error("expected hashing the same token twice to produce the same hash")
	}
	if h1 == h3 {
		t.Error("expected different tokens to produce different hashes")
	}
	if h1 == "raw-token-a" {
		t.Error("expected the hash to differ from the raw token")
	}
	if len(h1) != 64 { // hex-encoded SHA-256
		t.Errorf("expected a 64-char hex hash, got length %d", len(h1))
	}
}
