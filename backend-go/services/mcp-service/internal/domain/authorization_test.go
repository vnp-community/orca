package domain

import (
	"encoding/json"
	"net/url"
	"testing"
	"time"
)

func TestScopeCeilingMatrix(t *testing.T) {
	want := map[string]map[string]bool{
		RoleAdmin: {ScopeRead: true, ScopeWrite: true, ScopeExec: true, ScopeAdmin: true},
		RoleUser:  {ScopeRead: true, ScopeWrite: true, ScopeExec: true, ScopeAdmin: false},
		"":        {ScopeRead: false, ScopeWrite: false, ScopeExec: false, ScopeAdmin: false},
		"root":    {ScopeRead: false},
	}
	for role, scopes := range want {
		for s, ok := range scopes {
			if ScopesSubset([]string{s}, ScopeCeilingForRole(role)) != ok {
				t.Errorf("role %q scope %s allowed=%v", role, s, ok)
			}
		}
	}
}

func TestNormalizeScopes(t *testing.T) {
	got, err := NormalizeScopes([]string{"orca:write", "orca:read", "orca:write"})
	if err != nil || len(got) != 2 || got[0] != ScopeRead || got[1] != ScopeWrite {
		t.Fatalf("got %v %v", got, err)
	}
	if _, err := NormalizeScopes([]string{"orca:read", "orca:root"}); err == nil {
		t.Fatal("unknown scope accepted")
	}
}

func TestBuildAuthorizeRedirect(t *testing.T) {
	got, err := BuildAuthorizeRedirect("https://c.example.com/cb?keep=1", map[string]string{"code": "abc"}, "s t&ate", "https://orca.example.com")
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(got)
	q := u.Query()
	if u.Host != "c.example.com" || q.Get("keep") != "1" || q.Get("code") != "abc" || q.Get("state") != "s t&ate" || q.Get("iss") != "https://orca.example.com" {
		t.Fatalf("redirect = %s", got)
	}
	// An error response carries state and iss but no code.
	got, _ = BuildAuthorizeRedirect("http://127.0.0.1:5000/cb", map[string]string{"error": "access_denied"}, "", "https://orca.example.com")
	q = mustQuery(t, got)
	if q.Get("error") != "access_denied" || q.Has("state") || q.Has("code") || q.Get("iss") == "" {
		t.Fatalf("redirect = %s", got)
	}
}

func mustQuery(t *testing.T, raw string) url.Values {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return u.Query()
}

func TestRedirectHostIsHostOnly(t *testing.T) {
	if got := RedirectHost("https://user@evil.example.com:8443/path?x=1"); got != "evil.example.com:8443" {
		t.Fatalf("host = %q", got)
	}
}

func TestOutboxEventEnvelope(t *testing.T) {
	at := time.Date(2026, 10, 2, 1, 2, 3, 0, time.UTC)
	ev, err := NewOutboxEvent("e1", SubjectGrantRevoked, "t1", at, map[string]any{"grant_id": "g1", "by": "u1"})
	if err != nil {
		t.Fatal(err)
	}
	var p map[string]any
	if err := json.Unmarshal(ev.PayloadJSON, &p); err != nil {
		t.Fatal(err)
	}
	if p["event_id"] != "e1" || p["tenant_id"] != "t1" || p["schema_version"] != float64(1) || p["grant_id"] != "g1" || p["occurred_at"] == "" {
		t.Fatalf("payload = %v", p)
	}
	if ev.Subject != "orca.mcp.grant.revoked" || ev.Version != 1 || ev.ID != "e1" {
		t.Fatalf("event = %+v", ev)
	}
}
