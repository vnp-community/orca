package wsbridge

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stablyai/orca-go/common/jwtauth"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/wscompat"
	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"

	"github.com/go-jose/go-jose/v4/jwt"
)

// fakeCookieValidator lets tests control whether the cookie-first path
// succeeds, fails, or (via nil Handler.Cookie) is skipped entirely.
type fakeCookieValidator struct {
	identity wscompat.Identity
	err      error
	calls    int
}

func (f *fakeCookieValidator) ValidateCookie(_ context.Context, _ *http.Request) (wscompat.Identity, error) {
	f.calls++
	if f.err != nil {
		return wscompat.Identity{}, f.err
	}
	return f.identity, nil
}

// fakeJWKSClient / signed-token helpers, same pattern as
// usecase.validate_identity_test.go and httpgateway.router_test.go — a
// fresh RSA key signed the way Vault Transit would, plus a JWKSClient that
// resolves its kid.
type fakeJWKSClient struct {
	kid string
	key any
}

func (f *fakeJWKSClient) PublicKey(_ context.Context, kid string) (any, error) {
	if kid != f.kid {
		return nil, fmt.Errorf("fakeJWKSClient: no key for kid %q", kid)
	}
	return f.key, nil
}

func newBearerAuthValidator(t *testing.T) (*usecase.AuthValidator, string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating test RSA key: %v", err)
	}
	der, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatalf("marshaling public key: %v", err)
	}
	pemBlock := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})

	sign := func(_ context.Context, _ string, input []byte) (string, error) {
		digest := sha256.Sum256(input)
		sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
		if err != nil {
			return "", err
		}
		return "vault:v1:" + base64.StdEncoding.EncodeToString(sig), nil
	}
	pubKey := func(_ context.Context, _ string) (map[int]string, int, error) {
		return map[int]string{1: string(pemBlock)}, 1, nil
	}

	signer := jwtauth.NewTransitSigner("jwt-signing", sign, pubKey)
	token, err := jwtauth.Sign(context.Background(), signer, jwtauth.Claims{
		Claims: jwt.Claims{
			Issuer:   jwtauth.Issuer,
			Subject:  "user-bearer",
			IssuedAt: jwt.NewNumericDate(time.Now()),
			Expiry:   jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
		TenantID: "tenant-bearer",
	})
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	jwks, err := signer.PublicJWKS(context.Background())
	if err != nil {
		t.Fatalf("PublicJWKS: %v", err)
	}

	v := usecase.NewAuthValidator(&fakeJWKSClient{kid: jwks.Keys[0].KeyID, key: jwks.Keys[0].Key})
	return v, token
}

func TestHandler_ResolveIdentity_PrefersCookieOverBearerJWT(t *testing.T) {
	auth, token := newBearerAuthValidator(t)
	cookie := &fakeCookieValidator{identity: wscompat.Identity{TenantID: "tenant-cookie", UserID: "user-cookie"}}
	h := &Handler{Auth: auth, Cookie: cookie}

	r := httptest.NewRequest(http.MethodGet, "/v1/notifications/stream", nil)
	r.Header.Set("Authorization", "Bearer "+token)

	id, err := h.resolveIdentity(r)
	if err != nil {
		t.Fatalf("resolveIdentity: %v", err)
	}
	if id.TenantID != "tenant-cookie" || id.UserID != "user-cookie" {
		t.Fatalf("got Identity{%q,%q}, want cookie-resolved identity", id.TenantID, id.UserID)
	}
	if cookie.calls != 1 {
		t.Fatalf("cookie validator calls = %d, want 1", cookie.calls)
	}
}

func TestHandler_ResolveIdentity_FallsBackToBearerJWTWhenCookieNil(t *testing.T) {
	auth, token := newBearerAuthValidator(t)
	h := &Handler{Auth: auth, Cookie: nil}

	r := httptest.NewRequest(http.MethodGet, "/v1/notifications/stream", nil)
	r.Header.Set("Authorization", "Bearer "+token)

	id, err := h.resolveIdentity(r)
	if err != nil {
		t.Fatalf("resolveIdentity: %v", err)
	}
	if id.TenantID != "tenant-bearer" || id.UserID != "user-bearer" {
		t.Fatalf("got Identity{%q,%q}, want bearer-resolved identity", id.TenantID, id.UserID)
	}
}

func TestHandler_ResolveIdentity_FallsBackToBearerJWTWhenCookieFails(t *testing.T) {
	auth, token := newBearerAuthValidator(t)
	cookie := &fakeCookieValidator{err: fmt.Errorf("no session cookie present")}
	h := &Handler{Auth: auth, Cookie: cookie}

	r := httptest.NewRequest(http.MethodGet, "/v1/notifications/stream", nil)
	r.Header.Set("Authorization", "Bearer "+token)

	id, err := h.resolveIdentity(r)
	if err != nil {
		t.Fatalf("resolveIdentity: %v", err)
	}
	if id.TenantID != "tenant-bearer" || id.UserID != "user-bearer" {
		t.Fatalf("got Identity{%q,%q}, want bearer-resolved identity", id.TenantID, id.UserID)
	}
	if cookie.calls != 1 {
		t.Fatalf("cookie validator calls = %d, want 1", cookie.calls)
	}
}

func TestHandler_ResolveIdentity_BothFail(t *testing.T) {
	auth, _ := newBearerAuthValidator(t)
	cookie := &fakeCookieValidator{err: fmt.Errorf("no session cookie present")}
	h := &Handler{Auth: auth, Cookie: cookie}

	r := httptest.NewRequest(http.MethodGet, "/v1/notifications/stream", nil)
	// No Authorization header and no cookie present at all.

	if _, err := h.resolveIdentity(r); err == nil {
		t.Fatal("expected an error when neither cookie nor bearer JWT resolve")
	}
}
