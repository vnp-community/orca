package domain

import (
	"crypto/sha256"
	"encoding/base64"
	"strings"
	"testing"
)

func TestValidateRedirectURI(t *testing.T) {
	ok := []string{
		"https://app.example.com/cb",
		"https://app.example.com:8443/cb?x=1",
		"http://127.0.0.1/cb",
		"http://127.0.0.1:53211/cb",
		"http://[::1]:8080/cb",
		"http://localhost:3000/callback",
	}
	bad := map[string]string{
		"empty":             "",
		"http non-loopback": "http://evil.example.com/cb",
		"http lookalike":    "http://localhost.evil.com/cb",
		"http 127.0.0.2":    "http://127.0.0.2/cb",
		"wildcard host":     "https://*.example.com/cb",
		"wildcard path":     "https://app.example.com/*",
		"fragment":          "https://app.example.com/cb#frag",
		"empty fragment":    "https://app.example.com/cb#",
		"userinfo":          "https://user:pw@app.example.com/cb",
		"userinfo trick":    "https://app.example.com@evil.com/cb",
		"dotdot":            "https://app.example.com/a/../cb",
		"encoded dotdot":    "https://app.example.com/a/%2e%2e/cb",
		"custom scheme":     "myapp://callback",
		"javascript":        "javascript:alert(1)",
		"data":              "data:text/html,x",
		"relative":          "/cb",
		"no host":           "https:///cb",
		"backslash":         "https://app.example.com\\@evil.com/cb",
		"space":             "https://app.example.com/c b",
		"newline":           "https://app.example.com/cb\n",
		"too long":          "https://app.example.com/" + strings.Repeat("a", 2100),
		"opaque":            "https:app.example.com",
	}
	for _, u := range ok {
		if err := ValidateRedirectURI(u); err != nil {
			t.Errorf("%q should be valid: %v", u, err)
		}
	}
	for name, u := range bad {
		if ValidateRedirectURI(u) == nil {
			t.Errorf("%s: %q should be rejected", name, u)
		}
	}
}

func TestValidateRedirectURIs_CountAndDuplicates(t *testing.T) {
	if ValidateRedirectURIs(nil) == nil {
		t.Error("empty list accepted")
	}
	if ValidateRedirectURIs([]string{"https://a.example.com/1", "https://a.example.com/1"}) == nil {
		t.Error("duplicate accepted")
	}
	if err := ValidateRedirectURIs([]string{"https://a.example.com/1", "https://a.example.com/2"}); err != nil {
		t.Error(err)
	}
}

func TestMatchRedirectURI(t *testing.T) {
	reg := []string{"https://app.example.com/cb", "http://127.0.0.1/loop", "http://localhost:9000/x"}
	match := []string{
		"https://app.example.com/cb",
		"http://127.0.0.1/loop",
		"http://127.0.0.1:61000/loop", // RFC 8252 section 7.3: port is free for loopback
		"http://localhost:9000/x",
		"http://localhost:9001/x",
	}
	noMatch := []string{
		"https://app.example.com/cb/",
		"https://app.example.com/cB",
		"https://app.example.com/cb?a=1",
		"https://app.example.com:444/cb",
		"https://app.example.com/c",
		"http://app.example.com/cb",
		"http://127.0.0.1:61000/other",
		"http://[::1]:61000/loop",
		"http://localhost:9000/x?y=1",
		"https://127.0.0.1/loop",
		"",
	}
	for _, u := range match {
		if !MatchRedirectURI(reg, u) {
			t.Errorf("%q should match", u)
		}
	}
	for _, u := range noMatch {
		if MatchRedirectURI(reg, u) {
			t.Errorf("%q must not match", u)
		}
	}
}

func TestSanitizeClientName(t *testing.T) {
	if n, err := SanitizeClientName("  Claude\tCode \u200b"); err != nil || n != "Claude Code" {
		t.Errorf("got %q %v", n, err)
	}
	for _, bad := range []string{"", "   ", "\x00\x01", "a://b", strings.Repeat("é", 101)} {
		if _, err := SanitizeClientName(bad); err == nil {
			t.Errorf("%q should be rejected", bad)
		}
	}
	if _, err := SanitizeClientName(strings.Repeat("é", 100)); err != nil {
		t.Errorf("100 runes should be accepted: %v", err)
	}
}

func TestPKCE(t *testing.T) {
	verifier := strings.Repeat("a", 43)
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])

	if err := ValidatePKCEChallenge(challenge, "S256"); err != nil {
		t.Fatal(err)
	}
	for _, m := range []string{"", "plain", "s256", "S512"} {
		if ValidatePKCEChallenge(challenge, m) == nil {
			t.Errorf("method %q accepted", m)
		}
	}
	for _, c := range []string{"", "short", strings.Repeat("a", 129), challenge + "=", strings.Repeat("!", 43)} {
		if ValidatePKCEChallenge(c, "S256") == nil {
			t.Errorf("challenge %q accepted", c)
		}
	}
	if !VerifyPKCES256(verifier, challenge) {
		t.Error("valid pair rejected")
	}
	if VerifyPKCES256(verifier+"a", challenge) || VerifyPKCES256("", challenge) || VerifyPKCES256(challenge, challenge) {
		t.Error("invalid verifier accepted")
	}
	// RFC 7636 appendix B test vector.
	if !VerifyPKCES256("dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk", "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM") {
		t.Error("RFC 7636 vector failed")
	}
}

func TestOAuthScopes(t *testing.T) {
	got, err := ParseOAuthScopes("orca:write orca:read orca:read")
	if err != nil || FormatOAuthScopes(got) != "orca:read orca:write" {
		t.Fatalf("got %v %v", got, err)
	}
	if _, err := ParseOAuthScopes("orca:read orca:root"); err == nil {
		t.Error("unknown scope accepted")
	}
	if g, _ := ParseOAuthScopes(""); len(g) != 0 {
		t.Error("empty must parse to empty")
	}

	matrix := map[Role]map[string]bool{
		RoleAdmin: {OAuthScopeRead: true, OAuthScopeWrite: true, OAuthScopeExec: true, OAuthScopeAdmin: true},
		RoleUser:  {OAuthScopeRead: true, OAuthScopeWrite: true, OAuthScopeExec: true, OAuthScopeAdmin: false},
		Role(""):  {OAuthScopeRead: false, OAuthScopeWrite: false, OAuthScopeExec: false, OAuthScopeAdmin: false},
		Role("x"): {OAuthScopeRead: false},
	}
	for role, scopes := range matrix {
		for s, allowed := range scopes {
			if IsScopeSubset([]string{s}, OAuthScopeCeilingForRole(role)) != allowed {
				t.Errorf("role %q scope %s allowed=%v", role, s, allowed)
			}
		}
	}
}
