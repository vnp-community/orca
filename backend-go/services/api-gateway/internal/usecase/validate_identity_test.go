package usecase

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"
)

// fakeJWT builds a structurally-valid (unsigned) JWT string carrying the
// given claims — enough to exercise AuthValidator's unverified parsing
// path, which is exactly what it's documented to do (and not more).
func fakeJWT(t *testing.T, claimsJSON string) string {
	t.Helper()
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(claimsJSON))
	return header + "." + payload + ".fakesig"
}

func TestAuthValidator_ValidBearerToken(t *testing.T) {
	v := NewAuthValidator()
	token := fakeJWT(t, `{"tenant_id":"tenant-1","sub":"user-1"}`)

	r := httptest.NewRequest(http.MethodGet, "/v1/usage/daily", nil)
	r.Header.Set("Authorization", "Bearer "+token)

	id, err := v.Validate(r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id.TenantID != "tenant-1" || id.UserID != "user-1" {
		t.Fatalf("got Identity{%q, %q}, want {tenant-1, user-1}", id.TenantID, id.UserID)
	}
}

func TestAuthValidator_SessionCookieFallback(t *testing.T) {
	v := NewAuthValidator()
	token := fakeJWT(t, `{"tenant_id":"tenant-2","user_id":"user-2"}`)

	r := httptest.NewRequest(http.MethodGet, "/v1/usage/daily", nil)
	r.AddCookie(&http.Cookie{Name: SessionCookieName, Value: token})

	id, err := v.Validate(r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id.TenantID != "tenant-2" || id.UserID != "user-2" {
		t.Fatalf("got Identity{%q, %q}, want {tenant-2, user-2}", id.TenantID, id.UserID)
	}
}

func TestAuthValidator_NoCredential(t *testing.T) {
	v := NewAuthValidator()
	r := httptest.NewRequest(http.MethodGet, "/v1/usage/daily", nil)

	if _, err := v.Validate(r); err != ErrNoCredential {
		t.Fatalf("got error %v, want ErrNoCredential", err)
	}
}

func TestAuthValidator_MalformedToken(t *testing.T) {
	v := NewAuthValidator()
	r := httptest.NewRequest(http.MethodGet, "/v1/usage/daily", nil)
	r.Header.Set("Authorization", "Bearer not-a-jwt")

	if _, err := v.Validate(r); err != ErrMalformedToken {
		t.Fatalf("got error %v, want ErrMalformedToken", err)
	}
}

func TestAuthValidator_MissingClaims(t *testing.T) {
	v := NewAuthValidator()
	token := fakeJWT(t, `{"some_other_claim":"x"}`)

	r := httptest.NewRequest(http.MethodGet, "/v1/usage/daily", nil)
	r.Header.Set("Authorization", "Bearer "+token)

	if _, err := v.Validate(r); err != ErrMissingIdentityClaims {
		t.Fatalf("got error %v, want ErrMissingIdentityClaims", err)
	}
}
