package authclient

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"google.golang.org/grpc"

	jose "github.com/go-jose/go-jose/v4"

	authv1 "github.com/stablyai/orca-go/proto/gen/go/orca/auth/v1"
)

// fakeAuthServiceClient stubs GetJWKS (JWKSClient's needs) and
// ValidateSession (session_validator_test.go's needs) — shared across this
// package's test files, every other AuthServiceClient method left
// nil-embedded and unused.
type fakeAuthServiceClient struct {
	authv1.AuthServiceClient
	jwksJSON string
	err      error
	calls    int

	validateSessionFunc func(ctx context.Context, in *authv1.ValidateSessionRequest) (*authv1.ValidateSessionResponse, error)
}

func (f *fakeAuthServiceClient) GetJWKS(_ context.Context, _ *authv1.GetJWKSRequest, _ ...grpc.CallOption) (*authv1.GetJWKSResponse, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return &authv1.GetJWKSResponse{JwksJson: f.jwksJSON}, nil
}

func (f *fakeAuthServiceClient) ValidateSession(ctx context.Context, in *authv1.ValidateSessionRequest, _ ...grpc.CallOption) (*authv1.ValidateSessionResponse, error) {
	return f.validateSessionFunc(ctx, in)
}

func testJWKSJSON(t *testing.T, kid string) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating test RSA key: %v", err)
	}
	set := jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{
		Key: &key.PublicKey, KeyID: kid, Algorithm: "RS256", Use: "sig",
	}}}
	b, err := json.Marshal(set)
	if err != nil {
		t.Fatalf("marshaling JWKS: %v", err)
	}
	return string(b)
}

func TestJWKSClient_PublicKey_ResolvesKnownKid(t *testing.T) {
	fake := &fakeAuthServiceClient{jwksJSON: testJWKSJSON(t, "kid-1")}
	c := NewJWKSClient(fake)

	key, err := c.PublicKey(context.Background(), "kid-1")
	if err != nil {
		t.Fatalf("PublicKey: %v", err)
	}
	if _, ok := key.(*rsa.PublicKey); !ok {
		t.Fatalf("got key of type %T, want *rsa.PublicKey", key)
	}
	if fake.calls != 1 {
		t.Fatalf("GetJWKS calls = %d, want 1", fake.calls)
	}
}

func TestJWKSClient_PublicKey_UnknownKidFails(t *testing.T) {
	fake := &fakeAuthServiceClient{jwksJSON: testJWKSJSON(t, "kid-1")}
	c := NewJWKSClient(fake)

	if _, err := c.PublicKey(context.Background(), "does-not-exist"); err == nil {
		t.Fatal("expected an error for an unknown kid")
	}
}

func TestJWKSClient_PublicKey_CachesWithinTTL(t *testing.T) {
	fake := &fakeAuthServiceClient{jwksJSON: testJWKSJSON(t, "kid-1")}
	c := NewJWKSClient(fake)

	if _, err := c.PublicKey(context.Background(), "kid-1"); err != nil {
		t.Fatalf("PublicKey (1st): %v", err)
	}
	if _, err := c.PublicKey(context.Background(), "kid-1"); err != nil {
		t.Fatalf("PublicKey (2nd): %v", err)
	}
	if fake.calls != 1 {
		t.Fatalf("GetJWKS calls = %d, want 1 (second call should be served from cache)", fake.calls)
	}
}

func TestJWKSClient_PublicKey_RefetchesAfterTTLExpires(t *testing.T) {
	fake := &fakeAuthServiceClient{jwksJSON: testJWKSJSON(t, "kid-1")}
	c := NewJWKSClient(fake)

	if _, err := c.PublicKey(context.Background(), "kid-1"); err != nil {
		t.Fatalf("PublicKey (1st): %v", err)
	}
	// Force the cache to look stale without sleeping in the test.
	c.mu.Lock()
	c.cachedAt = time.Now().Add(-jwksCacheTTL - time.Second)
	c.mu.Unlock()

	if _, err := c.PublicKey(context.Background(), "kid-1"); err != nil {
		t.Fatalf("PublicKey (2nd, post-TTL): %v", err)
	}
	if fake.calls != 2 {
		t.Fatalf("GetJWKS calls = %d, want 2 (TTL expiry should trigger a re-fetch)", fake.calls)
	}
}

func TestJWKSClient_PublicKey_StaleCacheSurvivesFetchError(t *testing.T) {
	fake := &fakeAuthServiceClient{jwksJSON: testJWKSJSON(t, "kid-1")}
	c := NewJWKSClient(fake)

	if _, err := c.PublicKey(context.Background(), "kid-1"); err != nil {
		t.Fatalf("PublicKey (1st): %v", err)
	}

	// Expire the cache and make the next fetch fail — the stale-but-cached
	// set should still be served, not an error.
	c.mu.Lock()
	c.cachedAt = time.Now().Add(-jwksCacheTTL - time.Second)
	c.mu.Unlock()
	fake.err = errors.New("auth-service unreachable")

	key, err := c.PublicKey(context.Background(), "kid-1")
	if err != nil {
		t.Fatalf("PublicKey should serve stale cache on fetch error, got: %v", err)
	}
	if _, ok := key.(*rsa.PublicKey); !ok {
		t.Fatalf("got key of type %T, want *rsa.PublicKey", key)
	}
}

func TestJWKSClient_PublicKey_FetchErrorWithNoCacheFails(t *testing.T) {
	fake := &fakeAuthServiceClient{err: errors.New("auth-service unreachable")}
	c := NewJWKSClient(fake)

	if _, err := c.PublicKey(context.Background(), "kid-1"); err == nil {
		t.Fatal("expected an error when there is nothing cached and the fetch fails")
	}
}

func TestJWKSClient_PublicKey_MalformedJWKSJSONFails(t *testing.T) {
	fake := &fakeAuthServiceClient{jwksJSON: "not json"}
	c := NewJWKSClient(fake)

	if _, err := c.PublicKey(context.Background(), "kid-1"); err == nil {
		t.Fatal("expected an error for malformed jwks_json")
	}
}
