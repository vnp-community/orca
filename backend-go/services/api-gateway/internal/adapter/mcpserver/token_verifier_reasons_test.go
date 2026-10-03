package mcpserver_test

import (
	"crypto/rand"
	"crypto/rsa"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-jose/go-jose/v4/jwt"

	"github.com/stablyai/orca-go/common/jwtauth"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpserver"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpserver/mcpservertest"
	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"
)

// Distinct metric reasons must not change what the client sees: expired,
// wrong-audience and revoked tokens all stay 401 invalid_token.
func TestBearerTokenVerifier_AuthFailureReasonsAreDistinctButResponseIsNot(t *testing.T) {
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	keys := testKeys{key: k}
	res := &stubResolver{res: usecase.McpResolveResult{Active: true, Role: "user"}}
	v := usecase.NewAuthValidator(keys)
	v.McpPrincipals = res
	rec := &mcpservertest.CountingRecorder{}
	h := mcpserver.NewHandler(mcpserver.Deps{
		Config:   mcpserver.Config{ResourceURL: verifierResource, IssuerURL: "https://orca.example.com", ScopesSupported: []string{"orca:read"}},
		Verifier: mcpserver.NewBearerTokenVerifier(v, verifierResource),
		Recorder: rec,
	})
	r := chi.NewRouter()
	h.Mount(r)
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	expired := keys.sign(t, claims(func(c *jwtauth.Claims) {
		c.IssuedAt = jwt.NewNumericDate(time.Now().Add(-2 * time.Hour))
		c.Expiry = jwt.NewNumericDate(time.Now().Add(-time.Hour))
	}))
	wrongAud := keys.sign(t, claims(func(c *jwtauth.Claims) { c.Audience = jwt.Audience{"orca-cli"} }))
	good := keys.sign(t, claims(nil))

	cases := []struct {
		name, token, reason string
		inactive            string
	}{
		{"expired jwt", expired, "expired", ""},
		{"wrong audience", wrongAud, "audience", ""},
		{"revoked", good, "revoked", "revoked"},
		{"grant revoked", good, "revoked", "grant_revoked"},
		{"expired at auth-service", good, "expired", "expired"},
		{"user inactive stays generic", good, "invalid_token", "user_inactive"},
		{"garbage stays generic", "not-a-jwt", "invalid_token", ""},
	}
	for _, c := range cases {
		before := rec.AuthFailures(c.reason)
		res.res = usecase.McpResolveResult{Active: c.inactive == "", Role: "user", InactiveReason: c.inactive}
		code, _, hdr := post(t, srv.URL, map[string]string{"Authorization": "Bearer " + c.token}, "")
		if code != http.StatusUnauthorized || !strings.Contains(hdr.Get("WWW-Authenticate"), `error="invalid_token"`) {
			t.Errorf("%s: response changed: %d %q", c.name, code, hdr.Get("WWW-Authenticate"))
		}
		if got := rec.AuthFailures(c.reason) - before; got != 1 {
			t.Errorf("%s: reason %q counted %d times, want 1", c.name, c.reason, got)
		}
	}
}

// PATs suspended by the tenant kill switch answer exactly like the kill guard.
func TestBearerTokenVerifier_SuspendedPATIsKillSwitch403(t *testing.T) {
	k, _ := rsa.GenerateKey(rand.Reader, 2048)
	keys := testKeys{key: k}
	res := &stubResolver{res: usecase.McpResolveResult{Active: false, InactiveReason: "suspended"}}
	v := usecase.NewAuthValidator(keys)
	v.McpPrincipals = res
	rec := &mcpservertest.CountingRecorder{}
	h := mcpserver.NewHandler(mcpserver.Deps{
		Config:   mcpserver.Config{ResourceURL: verifierResource, IssuerURL: "https://orca.example.com", ScopesSupported: []string{"orca:read"}},
		Verifier: mcpserver.NewBearerTokenVerifier(v, verifierResource),
		Recorder: rec,
	})
	r := chi.NewRouter()
	h.Mount(r)
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	hdr := map[string]string{"Authorization": "Bearer omp_" + keys.sign(t, claims(nil))}
	code, body, _ := post(t, srv.URL, hdr, "")
	if code != http.StatusForbidden || !strings.Contains(body, "MCP_KILL_SWITCH_ACTIVE") {
		t.Fatalf("suspended PAT: %d %s", code, body)
	}
	if rec.AuthFailures("kill_switch") != 1 {
		t.Fatalf("kill_switch failures = %d", rec.AuthFailures("kill_switch"))
	}
	// Switch off: the very same token authenticates again.
	res.res = usecase.McpResolveResult{Active: true, Role: "user"}
	if code, body, _ := post(t, srv.URL, hdr, ""); code != http.StatusOK {
		t.Fatalf("PAT after kill switch off: %d %s", code, body)
	}
}
