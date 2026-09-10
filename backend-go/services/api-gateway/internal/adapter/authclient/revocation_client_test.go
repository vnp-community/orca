package authclient

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc"

	authv1 "github.com/stablyai/orca-go/proto/gen/go/orca/auth/v1"
)

// fakeAuthServiceClientForRevocation embeds authv1.AuthServiceClient (the
// interface itself, left nil) so only IsServiceTokenRevoked needs a real
// implementation — same trick jwks_client_test.go's own fakeAuthServiceClient
// uses; every other method panics on a nil embedded interface if ever
// called, which this test never does.
type fakeAuthServiceClientForRevocation struct {
	authv1.AuthServiceClient
	revoked map[string]bool
	err     error
	calls   map[string]int
}

func (f *fakeAuthServiceClientForRevocation) IsServiceTokenRevoked(_ context.Context, in *authv1.IsServiceTokenRevokedRequest, _ ...grpc.CallOption) (*authv1.IsServiceTokenRevokedResponse, error) {
	f.calls[in.GetJti()]++
	if f.err != nil {
		return nil, f.err
	}
	return &authv1.IsServiceTokenRevokedResponse{Revoked: f.revoked[in.GetJti()]}, nil
}

func TestRevocationClient_ReturnsRevokedTrue(t *testing.T) {
	fake := &fakeAuthServiceClientForRevocation{revoked: map[string]bool{"jti-revoked": true}, calls: map[string]int{}}
	c := NewRevocationClient(fake)

	revoked, err := c.IsRevoked(context.Background(), "jti-revoked")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !revoked {
		t.Fatal("got revoked=false, want true")
	}
}

func TestRevocationClient_DistinctJTIsCachedSeparately(t *testing.T) {
	fake := &fakeAuthServiceClientForRevocation{revoked: map[string]bool{"jti-b": true}, calls: map[string]int{}}
	c := NewRevocationClient(fake)

	revokedA, err := c.IsRevoked(context.Background(), "jti-a")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	revokedB, err := c.IsRevoked(context.Background(), "jti-b")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if revokedA {
		t.Error("jti-a: got revoked=true, want false")
	}
	if !revokedB {
		t.Error("jti-b: got revoked=false, want true")
	}
	if fake.calls["jti-a"] != 1 || fake.calls["jti-b"] != 1 {
		t.Fatalf("calls = %+v, want 1 each for jti-a and jti-b", fake.calls)
	}
}

var errRevocationRPCFailed = errors.New("fake: rpc failed")

func TestRevocationClient_PropagatesRPCError(t *testing.T) {
	fake := &fakeAuthServiceClientForRevocation{err: errRevocationRPCFailed, calls: map[string]int{}}
	c := NewRevocationClient(fake)

	if _, err := c.IsRevoked(context.Background(), "jti-1"); err == nil {
		t.Fatal("expected the RPC error to propagate")
	}
}
