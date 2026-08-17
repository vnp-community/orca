package httpgateway

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stablyai/orca-go/common/jwtauth"
	"github.com/stablyai/orca-go/services/api-gateway/internal/domain"
	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"

	"github.com/go-jose/go-jose/v4/jwt"
)

// fakeTransit signs with an in-memory RSA key exactly the way Vault
// Transit would — the same helper common/jwtauth's own tests use — so
// these router tests exercise AuthValidator's real signature verification
// rather than the unverified placeholder it replaced.
type fakeTransit struct {
	key *rsa.PrivateKey
	pem string
}

func newFakeTransit(t *testing.T) *fakeTransit {
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
	return &fakeTransit{key: key, pem: string(pemBlock)}
}

func (f *fakeTransit) sign(_ context.Context, _ string, input []byte) (string, error) {
	digest := sha256.Sum256(input)
	sig, err := rsa.SignPKCS1v15(rand.Reader, f.key, crypto.SHA256, digest[:])
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("vault:v1:%s", base64.StdEncoding.EncodeToString(sig)), nil
}

func (f *fakeTransit) publicKeyVersions(_ context.Context, _ string) (map[int]string, int, error) {
	return map[int]string{1: f.pem}, 1, nil
}

// fakeJWKSClient implements usecase.JWKSClient over a single already-known
// kid -> key mapping.
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

// testAuth mints real, signed JWTs from one fixed signing key and a
// JWKSClient resolving that key's kid — the router tests use this shared
// pair everywhere so every minted token verifies against the same
// AuthValidator's JWKSClient (same helper shape as usecase's own test, kept
// package-local to avoid an internal test-only export).
type testAuth struct {
	signer *jwtauth.TransitSigner
	jwks   usecase.JWKSClient
}

func newTestAuth(t *testing.T) *testAuth {
	t.Helper()
	transit := newFakeTransit(t)
	signer := jwtauth.NewTransitSigner("jwt-signing", transit.sign, transit.publicKeyVersions)
	jwks, err := signer.PublicJWKS(context.Background())
	if err != nil {
		t.Fatalf("PublicJWKS: %v", err)
	}
	return &testAuth{
		signer: signer,
		jwks:   &fakeJWKSClient{kid: jwks.Keys[0].KeyID, key: jwks.Keys[0].Key},
	}
}

func (a *testAuth) token(t *testing.T, tenantID, subject string) string {
	t.Helper()
	claims := jwtauth.Claims{
		Claims: jwt.Claims{
			Issuer:   jwtauth.Issuer,
			Subject:  subject,
			IssuedAt: jwt.NewNumericDate(time.Now()),
			Expiry:   jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
		TenantID: tenantID,
	}
	token, err := jwtauth.Sign(context.Background(), a.signer, claims)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	return token
}

func testRouter(t *testing.T, auth *testAuth) http.Handler {
	t.Helper()
	return NewRouter(Deps{
		Logger:        slog.Default(),
		Registry:      domain.NewDefaultServiceRegistry(),
		AuthValidator: usecase.NewAuthValidator(auth.jwks),
		RateLimiter:   usecase.NewRateLimiter(1000, 1000), // effectively unlimited for routing tests
		UsageClient:   nil,                                // not exercised by these tests
	})
}

func authedRequest(t *testing.T, auth *testAuth, method, path string) *http.Request {
	t.Helper()
	r := httptest.NewRequest(method, path, nil)
	r.Header.Set("Authorization", "Bearer "+auth.token(t, "tenant-1", "user-1"))
	return r
}

func TestRouter_StubbedServiceReturns501WithExplanatoryBody(t *testing.T) {
	auth := newTestAuth(t)
	// By Phase 5 (execution-plan.md), NewDefaultServiceRegistry() no
	// longer has any RouteStubbed entry — every prefix is wired for real.
	// mountStubRoutes' 501 fallback still needs coverage (it's the
	// contract any future not-yet-mature service relies on), so this test
	// builds its own synthetic registry with one stubbed rule instead of
	// depending on a specific real service staying unwired.
	registry := domain.NewServiceRegistry([]domain.RoutingRule{
		{PathPrefix: "/v1/not-yet-real", ServiceName: "not-yet-real-service", ProtoPackage: "orca.notyetreal.v1", Status: domain.RouteStubbed},
	})
	router := NewRouter(Deps{
		Logger:        slog.Default(),
		Registry:      registry,
		AuthValidator: usecase.NewAuthValidator(auth.jwks),
		RateLimiter:   usecase.NewRateLimiter(1000, 1000),
	})

	cases := []struct {
		name        string
		path        string
		wantService string
	}{
		{"catch-all", "/v1/not-yet-real/123", "not-yet-real-service"},
		{"bare prefix", "/v1/not-yet-real", "not-yet-real-service"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, authedRequest(t, auth, http.MethodGet, tc.path))

			if rec.Code != http.StatusNotImplemented {
				t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusNotImplemented, rec.Body.String())
			}

			var body errorBody
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("response body is not the expected JSON shape: %v; body=%s", err, rec.Body.String())
			}
			if body.Error.Code != "NOT_IMPLEMENTED" {
				t.Fatalf("error.code = %q, want NOT_IMPLEMENTED", body.Error.Code)
			}
			if !strings.Contains(body.Error.Message, tc.wantService) {
				t.Fatalf("error.message = %q, want it to mention %q", body.Error.Message, tc.wantService)
			}
			if !strings.Contains(body.Error.Message, "once its gRPC contract stabilizes") {
				t.Fatalf("error.message = %q, want it to explain the stub", body.Error.Message)
			}
		})
	}
}

func TestRouter_UnauthenticatedRequestReturns401(t *testing.T) {
	router := testRouter(t, newTestAuth(t))

	// /v1/usage/daily is the one path testRouter always mounts
	// unconditionally (mountUsageRoutes runs regardless of UsageClient
	// being nil) — every Phase 5 mountXRoutes prefix is guarded on its
	// client being non-nil, and testRouter leaves them all nil, so those
	// would 404 before ever reaching authMiddleware.
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/usage/daily", nil))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}
