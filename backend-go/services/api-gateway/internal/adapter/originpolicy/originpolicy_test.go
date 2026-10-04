package originpolicy

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func req(origin, host string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "http://"+host+"/ws", nil)
	r.Host = host
	if origin != "" {
		r.Header.Set("Origin", origin)
	}
	return r
}

func TestEmptyPolicyAllowsEverything(t *testing.T) {
	p, err := Parse("")
	if err != nil {
		t.Fatal(err)
	}
	if p.Enforced() {
		t.Fatal("empty list must not enforce")
	}
	if !p.Allow(req("https://evil.example", "orca.example.com")) {
		t.Fatal("legacy permissive behavior expected")
	}
	var nilPolicy *Policy
	if !nilPolicy.Allow(req("https://evil.example", "orca.example.com")) {
		t.Fatal("nil policy must be permissive")
	}
}

func TestEnforcedPolicy(t *testing.T) {
	p, err := Parse("https://orca.example.com, https://*.corp.example.com ,http://localhost:5173")
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name   string
		origin string
		host   string
		want   bool
	}{
		{"exact match", "https://orca.example.com", "gw.internal", true},
		{"case-insensitive host", "https://ORCA.example.com", "gw.internal", true},
		{"wrong scheme", "http://orca.example.com", "gw.internal", false},
		{"other host", "https://evil.example", "gw.internal", false},
		{"suffix trick", "https://evilorca.example.com", "gw.internal", false},
		{"wildcard subdomain", "https://a.corp.example.com", "gw.internal", true},
		{"wildcard multi-label", "https://a.b.corp.example.com", "gw.internal", true},
		{"wildcard excludes bare domain", "https://corp.example.com", "gw.internal", false},
		{"dev origin with port", "http://localhost:5173", "gw.internal", true},
		{"dev origin wrong port", "http://localhost:9999", "gw.internal", false},
		{"no origin header (non-browser)", "", "gw.internal", true},
		{"malformed origin", "null", "gw.internal", false},
		{"same origin http", "http://gw.internal", "gw.internal", true},
	}
	for _, c := range cases {
		if got := p.Allow(req(c.origin, c.host)); got != c.want {
			t.Errorf("%s: Allow(%q)=%v want %v", c.name, c.origin, got, c.want)
		}
	}
}

func TestSameOriginRespectsForwardedProto(t *testing.T) {
	p, _ := Parse("https://other.example.com")
	r := req("https://orca.example.com", "orca.example.com")
	if p.Allow(r) {
		t.Fatal("https origin vs plain http request must not count as same-origin")
	}
	r.Header.Set("X-Forwarded-Proto", "https")
	if !p.Allow(r) {
		t.Fatal("behind a TLS-terminating proxy the same origin must be accepted")
	}
}

func TestParseRejectsBadEntries(t *testing.T) {
	for _, bad := range []string{"orca.example.com", "ftp://x.example", "https://a.*.example.com", "https://x.example/path", "*"} {
		if _, err := Parse(bad); err == nil {
			t.Errorf("Parse(%q) should fail", bad)
		}
	}
}

func TestReportOnlyFlag(t *testing.T) {
	p, _ := Parse("https://orca.example.com")
	if p.ReportOnly() {
		t.Fatal("default must be enforce")
	}
	if !p.WithReportOnly(true).ReportOnly() {
		t.Fatal("WithReportOnly(true) must set the flag")
	}
	// The verdict itself is unchanged; handlers decide what to do with it.
	if p.Allow(req("https://evil.example", "gw.internal")) {
		t.Fatal("report-only must not change Allow's verdict")
	}
	var nilPolicy *Policy
	if nilPolicy.WithReportOnly(true) != nil || nilPolicy.ReportOnly() {
		t.Fatal("nil policy stays nil and not report-only")
	}
}
