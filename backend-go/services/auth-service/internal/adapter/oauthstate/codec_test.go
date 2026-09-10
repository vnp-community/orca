package oauthstate

import (
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
	"github.com/stablyai/orca-go/services/auth-service/internal/usecase"
)

func testState() usecase.SsoState {
	return usecase.SsoState{
		Provider:     domain.SsoProviderGitHub,
		RedirectURI:  "https://app.example.com/auth/callback",
		CodeVerifier: "verifier-value",
		ExpiresAt:    time.Now().Add(15 * time.Minute).Truncate(time.Second),
	}
}

func TestEncodeDecode_RoundTrips(t *testing.T) {
	c := New("secret-key")
	token, err := c.Encode(testState())
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	got, err := c.Decode(token)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	want := testState()
	if got.Provider != want.Provider || got.RedirectURI != want.RedirectURI || got.CodeVerifier != want.CodeVerifier || !got.ExpiresAt.Equal(want.ExpiresAt) {
		t.Errorf("decoded state = %+v, want %+v", got, want)
	}
}

func TestDecode_RejectsTamperedPayload(t *testing.T) {
	c := New("secret-key")
	token, err := c.Encode(testState())
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	tampered := token[:len(token)-2] + "xx"
	if _, err := c.Decode(tampered); err == nil {
		t.Fatal("expected an error for a tampered token")
	}
}

func TestDecode_RejectsWrongSecret(t *testing.T) {
	token, err := New("secret-a").Encode(testState())
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	if _, err := New("secret-b").Decode(token); err != ErrInvalidSignature {
		t.Errorf("err = %v, want %v", err, ErrInvalidSignature)
	}
}

func TestDecode_RejectsExpiredToken(t *testing.T) {
	c := New("secret-key")
	expired := testState()
	expired.ExpiresAt = time.Now().Add(-time.Minute)
	token, err := c.Encode(expired)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	if _, err := c.Decode(token); err != ErrExpired {
		t.Errorf("err = %v, want %v", err, ErrExpired)
	}
}

func TestDecode_RejectsMalformedToken(t *testing.T) {
	c := New("secret-key")
	for _, malformed := range []string{"", "no-dot-here", ".", "abc."} {
		if _, err := c.Decode(malformed); err == nil {
			t.Errorf("token %q: expected an error", malformed)
		}
	}
}
