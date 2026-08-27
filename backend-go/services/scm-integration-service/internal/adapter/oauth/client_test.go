package oauth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestAuthorizationURL_BuildsExpectedQueryParams(t *testing.T) {
	c := New(nil, Config{AuthorizeURL: "https://github.com/login/oauth/authorize", ClientID: "cid", Scope: "repo"})

	got := c.AuthorizationURL("state-123", "https://gateway.example.com/auth/github/callback")

	u, err := url.Parse(got)
	if err != nil {
		t.Fatalf("unexpected error parsing url: %v", err)
	}
	q := u.Query()
	if q.Get("client_id") != "cid" {
		t.Errorf("expected client_id=cid, got %q", q.Get("client_id"))
	}
	if q.Get("state") != "state-123" {
		t.Errorf("expected state=state-123, got %q", q.Get("state"))
	}
	if q.Get("redirect_uri") != "https://gateway.example.com/auth/github/callback" {
		t.Errorf("unexpected redirect_uri: %q", q.Get("redirect_uri"))
	}
	if q.Get("response_type") != "code" {
		t.Errorf("expected response_type=code, got %q", q.Get("response_type"))
	}
	if q.Get("scope") != "repo" {
		t.Errorf("expected scope=repo, got %q", q.Get("scope"))
	}
}

func TestExchangeCode_RealHTTPCall(t *testing.T) {
	var gotMethod string
	var gotForm url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		_ = r.ParseForm()
		gotForm = r.PostForm

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"gho_exchanged","scope":"repo"}`))
	}))
	defer server.Close()

	c := New(server.Client(), Config{TokenURL: server.URL, ClientID: "cid", ClientSecret: "csecret"})
	token, err := c.ExchangeCode(context.Background(), "auth-code", "https://gateway.example.com/auth/github/callback")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("expected POST, got %s", gotMethod)
	}
	if gotForm.Get("code") != "auth-code" || gotForm.Get("client_id") != "cid" || gotForm.Get("client_secret") != "csecret" || gotForm.Get("grant_type") != "authorization_code" {
		t.Errorf("unexpected form body: %+v", gotForm)
	}
	if token.AccessToken != "gho_exchanged" || token.Scope != "repo" {
		t.Errorf("unexpected token: %+v", token)
	}
}

func TestExchangeCode_ProviderErrorIsAnError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"error":"bad_verification_code","error_description":"the code has expired"}`))
	}))
	defer server.Close()

	c := New(server.Client(), Config{TokenURL: server.URL})
	_, err := c.ExchangeCode(context.Background(), "stale-code", "https://gateway.example.com/callback")
	if err == nil {
		t.Fatal("expected an error when the provider's token response carries an error field")
	}
}

func TestExchangeCode_NonOKStatusIsAnError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	c := New(server.Client(), Config{TokenURL: server.URL})
	_, err := c.ExchangeCode(context.Background(), "code", "https://gateway.example.com/callback")
	if err == nil {
		t.Fatal("expected an error for a non-200 response")
	}
}
