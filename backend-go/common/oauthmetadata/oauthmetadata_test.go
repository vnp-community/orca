package oauthmetadata

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBuildAuthorizationServer_AdvertisesOnlyImplementedFeatures(t *testing.T) {
	doc, err := BuildAuthorizationServer("https://orca.example.com/", true, []string{"orca:read"})
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(doc)
	got := string(b)
	for _, want := range []string{
		`"issuer":"https://orca.example.com"`,
		`"authorization_endpoint":"https://orca.example.com/oauth/authorize"`,
		`"registration_endpoint":"https://orca.example.com/oauth/register"`,
		`"code_challenge_methods_supported":["S256"]`,
		`"token_endpoint_auth_methods_supported":["none"]`,
		`"authorization_response_iss_parameter_supported":true`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("metadata missing %s in %s", want, got)
		}
	}
	if strings.Contains(got, "jwks_uri") || strings.Contains(got, `"plain"`) {
		t.Errorf("metadata advertises unsupported features: %s", got)
	}
}

func TestBuildAuthorizationServer_NoRegistrationWhenDCRDisabled(t *testing.T) {
	doc, _ := BuildAuthorizationServer("https://orca.example.com", false, nil)
	b, _ := json.Marshal(doc)
	if strings.Contains(string(b), "registration_endpoint") {
		t.Fatalf("registration_endpoint must be omitted: %s", b)
	}
}

func TestBuildProtectedResource(t *testing.T) {
	doc, err := BuildProtectedResource("https://orca.example.com", []string{"orca:read"})
	if err != nil {
		t.Fatal(err)
	}
	if doc.Resource != "https://orca.example.com/mcp" || doc.AuthorizationServers[0] != "https://orca.example.com" {
		t.Fatalf("unexpected doc %+v", doc)
	}
	if got := ProtectedResourceMetadataURL("https://orca.example.com/"); got != "https://orca.example.com/.well-known/oauth-protected-resource/mcp" {
		t.Fatalf("metadata url %q", got)
	}
}

func TestValidateBaseURL(t *testing.T) {
	ok := []string{"https://orca.example.com", "http://localhost:8080", "http://127.0.0.1:3000/"}
	bad := []string{"", "http://orca.example.com", "https://u:p@orca.example.com", "https://orca.example.com/x", "https://orca.example.com?a=b", "https://orca.example.com#f", "ftp://x"}
	for _, u := range ok {
		if err := ValidateBaseURL(u); err != nil {
			t.Errorf("%q should be valid: %v", u, err)
		}
	}
	for _, u := range bad {
		if ValidateBaseURL(u) == nil {
			t.Errorf("%q should be rejected", u)
		}
	}
}
