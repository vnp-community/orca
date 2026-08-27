package oauthstate

import (
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/usecase"
)

func TestEncodeDecode_RoundTrips(t *testing.T) {
	c := New("test-secret")
	want := usecase.OAuthState{
		TenantID:    "tenant-1",
		UserID:      "user-1",
		Provider:    domain.ScmProviderGitHub,
		RedirectURI: "https://gateway.example.com/auth/github/callback",
		ExpiresAt:   time.Now().Add(15 * time.Minute).Truncate(time.Second),
	}

	token, err := c.Encode(want)
	if err != nil {
		t.Fatalf("unexpected error encoding: %v", err)
	}
	got, err := c.Decode(token)
	if err != nil {
		t.Fatalf("unexpected error decoding: %v", err)
	}
	if got.TenantID != want.TenantID || got.UserID != want.UserID || got.Provider != want.Provider || got.RedirectURI != want.RedirectURI {
		t.Errorf("round trip mismatch: got %+v, want %+v", got, want)
	}
}

func TestDecode_RejectsTamperedPayload(t *testing.T) {
	c := New("test-secret")
	token, err := c.Encode(usecase.OAuthState{TenantID: "tenant-1", Provider: domain.ScmProviderGitHub, ExpiresAt: time.Now().Add(time.Hour)})
	if err != nil {
		t.Fatalf("unexpected error encoding: %v", err)
	}

	if _, err := c.Decode(token + "tampered"); err == nil {
		t.Fatal("expected an error decoding a tampered token")
	}
}

func TestDecode_RejectsWrongSecret(t *testing.T) {
	token, err := New("secret-a").Encode(usecase.OAuthState{TenantID: "tenant-1", Provider: domain.ScmProviderGitHub, ExpiresAt: time.Now().Add(time.Hour)})
	if err != nil {
		t.Fatalf("unexpected error encoding: %v", err)
	}

	if _, err := New("secret-b").Decode(token); err != ErrInvalidSignature {
		t.Fatalf("expected ErrInvalidSignature, got %v", err)
	}
}

func TestDecode_RejectsExpiredToken(t *testing.T) {
	c := New("test-secret")
	token, err := c.Encode(usecase.OAuthState{TenantID: "tenant-1", Provider: domain.ScmProviderGitHub, ExpiresAt: time.Now().Add(-time.Minute)})
	if err != nil {
		t.Fatalf("unexpected error encoding: %v", err)
	}

	if _, err := c.Decode(token); err != ErrExpired {
		t.Fatalf("expected ErrExpired, got %v", err)
	}
}

func TestDecode_RejectsMalformedToken(t *testing.T) {
	c := New("test-secret")
	if _, err := c.Decode("not-a-valid-token"); err == nil {
		t.Fatal("expected an error decoding a malformed token")
	}
}
