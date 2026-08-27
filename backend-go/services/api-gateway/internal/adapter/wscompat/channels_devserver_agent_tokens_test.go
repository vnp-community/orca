package wscompat

import (
	"context"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
)

// fakeAgentTokenClient is a minimal test double scoped to the 3 RPCs
// devServer.agentTokens.* channel handlers call.
type fakeAgentTokenClient struct {
	infrafleetv1.InfraFleetServiceClient

	createFunc func(ctx context.Context, in *infrafleetv1.CreateAgentTokenRequest) (*infrafleetv1.CreateAgentTokenResponse, error)
	listFunc   func(ctx context.Context, in *infrafleetv1.ListAgentTokensRequest) (*infrafleetv1.ListAgentTokensResponse, error)
	revokeFunc func(ctx context.Context, in *infrafleetv1.RevokeAgentTokenRequest) error
}

func (f *fakeAgentTokenClient) CreateAgentToken(ctx context.Context, in *infrafleetv1.CreateAgentTokenRequest, _ ...grpc.CallOption) (*infrafleetv1.CreateAgentTokenResponse, error) {
	return f.createFunc(ctx, in)
}

func (f *fakeAgentTokenClient) ListAgentTokens(ctx context.Context, in *infrafleetv1.ListAgentTokensRequest, _ ...grpc.CallOption) (*infrafleetv1.ListAgentTokensResponse, error) {
	return f.listFunc(ctx, in)
}

func (f *fakeAgentTokenClient) RevokeAgentToken(ctx context.Context, in *infrafleetv1.RevokeAgentTokenRequest, _ ...grpc.CallOption) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, f.revokeFunc(ctx, in)
}

func TestDevServerAgentTokenChannels_Create(t *testing.T) {
	var gotReq *infrafleetv1.CreateAgentTokenRequest
	fake := &fakeAgentTokenClient{
		createFunc: func(_ context.Context, in *infrafleetv1.CreateAgentTokenRequest) (*infrafleetv1.CreateAgentTokenResponse, error) {
			gotReq = in
			return &infrafleetv1.CreateAgentTokenResponse{Id: "tok-1", Token: "plaintext-secret", Name: "my token", CreatedAtUnixMs: 123}, nil
		},
	}
	r := NewRegistry()
	registerDevServerAgentTokenChannels(r, fake)

	args := argsJSON(t, map[string]any{"devServerId": "dev-1", "name": "my token"})
	result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1", UserID: "u1"}, "devServer.agentTokens.create", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotReq.GetDevServerId() != "dev-1" || gotReq.GetName() != "my token" {
		t.Fatalf("unexpected request: %+v", gotReq)
	}
	out, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("unexpected result type: %T", result)
	}
	if out["id"] != "tok-1" || out["token"] != "plaintext-secret" {
		t.Fatalf("unexpected result: %+v", out)
	}
}

func TestDevServerAgentTokenChannels_CreateNoDevServerID(t *testing.T) {
	fake := &fakeAgentTokenClient{
		createFunc: func(context.Context, *infrafleetv1.CreateAgentTokenRequest) (*infrafleetv1.CreateAgentTokenResponse, error) {
			t.Fatal("RPC should not be called when devServerId is missing")
			return nil, nil
		},
	}
	r := NewRegistry()
	registerDevServerAgentTokenChannels(r, fake)

	args := argsJSON(t, map[string]any{"name": "x"})
	_, err := r.Dispatch(context.Background(), Identity{TenantID: "t1", UserID: "u1"}, "devServer.agentTokens.create", args)
	if err == nil {
		t.Fatal("expected error for missing devServerId")
	}
}

func TestDevServerAgentTokenChannels_ListNeverExposesPlaintextToken(t *testing.T) {
	lastUsed := int64(456)
	fake := &fakeAgentTokenClient{
		listFunc: func(_ context.Context, in *infrafleetv1.ListAgentTokensRequest) (*infrafleetv1.ListAgentTokensResponse, error) {
			if in.GetDevServerId() != "dev-1" {
				t.Fatalf("unexpected devServerId: %q", in.GetDevServerId())
			}
			return &infrafleetv1.ListAgentTokensResponse{Tokens: []*infrafleetv1.AgentTokenSummary{
				{Id: "tok-1", Name: "a", CreatedAtUnixMs: 100},
				{Id: "tok-2", Name: "b", CreatedAtUnixMs: 200, LastUsedAtUnixMs: &lastUsed},
			}}, nil
		},
	}
	r := NewRegistry()
	registerDevServerAgentTokenChannels(r, fake)

	args := argsJSON(t, map[string]any{"devServerId": "dev-1"})
	result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1", UserID: "u1"}, "devServer.agentTokens.list", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("unexpected result type: %T", result)
	}
	tokens, ok := out["tokens"].([]map[string]any)
	if !ok || len(tokens) != 2 {
		t.Fatalf("unexpected tokens: %+v", out["tokens"])
	}
	for _, entry := range tokens {
		if _, hasToken := entry["token"]; hasToken {
			t.Fatalf("list response must never re-expose the plaintext token, got: %+v", entry)
		}
	}
	if _, ok := tokens[0]["lastUsedAtUnixMs"]; ok {
		t.Fatalf("tok-1 has no LastUsedAtUnixMs and should omit the key, got: %+v", tokens[0])
	}
	if tokens[1]["lastUsedAtUnixMs"] != int64(456) {
		t.Fatalf("tok-2's lastUsedAtUnixMs mismatch: %+v", tokens[1])
	}
}

func TestDevServerAgentTokenChannels_Revoke(t *testing.T) {
	var gotReq *infrafleetv1.RevokeAgentTokenRequest
	r := NewRegistry()
	registerDevServerAgentTokenChannels(r, &fakeAgentTokenClient{
		revokeFunc: func(_ context.Context, in *infrafleetv1.RevokeAgentTokenRequest) error {
			gotReq = in
			return nil
		},
	})

	args := argsJSON(t, map[string]any{"devServerId": "dev-1", "id": "tok-1"})
	result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1", UserID: "u1"}, "devServer.agentTokens.revoke", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotReq.GetDevServerId() != "dev-1" || gotReq.GetId() != "tok-1" {
		t.Fatalf("unexpected request: %+v", gotReq)
	}
	out, ok := result.(map[string]bool)
	if !ok || !out["ok"] {
		t.Fatalf("unexpected result: %+v", result)
	}
}
