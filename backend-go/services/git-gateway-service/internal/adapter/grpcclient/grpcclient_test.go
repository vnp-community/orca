package grpcclient

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"google.golang.org/grpc"

	"github.com/stablyai/orca-go/common/tenant"
	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
)

// fakeInfraFleetServiceClient implements infrafleetv1.InfraFleetServiceClient
// directly (no bufconn/wire-level gRPC) — this package's tests fake the
// port (the generated client interface), not the transport, matching this
// repo's usecase-test convention (e.g. internal/usecase/dispatch_test.go's
// fakeConnectionResolver/fakeGitExecutor).
type fakeInfraFleetServiceClient struct {
	infrafleetv1.InfraFleetServiceClient // embed: panics on any unimplemented method, which is intentional for these tests

	resolveConnectionResp *infrafleetv1.ResolveConnectionResponse
	resolveConnectionErr  error
	gotResolveConnection  *infrafleetv1.ResolveConnectionRequest

	relayResp *infrafleetv1.RelayResponse
	relayErr  error
	gotRelay  *infrafleetv1.RelayRequest
}

func (f *fakeInfraFleetServiceClient) ResolveConnection(ctx context.Context, in *infrafleetv1.ResolveConnectionRequest, _ ...grpc.CallOption) (*infrafleetv1.ResolveConnectionResponse, error) {
	f.gotResolveConnection = in
	if f.resolveConnectionErr != nil {
		return nil, f.resolveConnectionErr
	}
	return f.resolveConnectionResp, nil
}

func (f *fakeInfraFleetServiceClient) Relay(ctx context.Context, in *infrafleetv1.RelayRequest, _ ...grpc.CallOption) (*infrafleetv1.RelayResponse, error) {
	f.gotRelay = in
	if f.relayErr != nil {
		return nil, f.relayErr
	}
	return f.relayResp, nil
}

func ctxWithTenant(t *testing.T) context.Context {
	t.Helper()
	return tenant.WithTenantID(context.Background(), "tenant-1")
}

func TestConnectionResolver_ResolveConnection_NotConnected(t *testing.T) {
	fake := &fakeInfraFleetServiceClient{
		resolveConnectionResp: &infrafleetv1.ResolveConnectionResponse{Connected: false},
	}
	r := NewConnectionResolver(fake)

	conn, err := r.ResolveConnection(ctxWithTenant(t), "wt-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if conn.Connected {
		t.Error("expected Connected=false")
	}
	if conn.RepoPath != "wt-1" {
		t.Errorf("expected RepoPath to fall back to worktreeID %q, got %q", "wt-1", conn.RepoPath)
	}
	if fake.gotResolveConnection.GetConnectionId() != "wt-1" {
		t.Errorf("expected ConnectionId=wt-1 on the request, got %q", fake.gotResolveConnection.GetConnectionId())
	}
}

func TestConnectionResolver_ResolveConnection_Connected(t *testing.T) {
	fake := &fakeInfraFleetServiceClient{
		resolveConnectionResp: &infrafleetv1.ResolveConnectionResponse{
			Connected: true,
			RepoPath:  "/remote/repo",
		},
	}
	r := NewConnectionResolver(fake)

	conn, err := r.ResolveConnection(ctxWithTenant(t), "wt-2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !conn.Connected {
		t.Error("expected Connected=true")
	}
	if conn.ConnectionID != "wt-2" {
		t.Errorf("expected ConnectionID=wt-2, got %q", conn.ConnectionID)
	}
	if conn.RepoPath != "/remote/repo" {
		t.Errorf("expected RepoPath from response, got %q", conn.RepoPath)
	}
}

func TestConnectionResolver_ResolveConnection_NoTenantInContext(t *testing.T) {
	fake := &fakeInfraFleetServiceClient{}
	r := NewConnectionResolver(fake)

	if _, err := r.ResolveConnection(context.Background(), "wt-1"); !errors.Is(err, tenant.ErrNoTenant) {
		t.Errorf("expected tenant.ErrNoTenant, got %v", err)
	}
	if fake.gotResolveConnection != nil {
		t.Error("expected ResolveConnection not to be called without a tenant in context")
	}
}

func TestRelayExecutor_GetStatus_Success(t *testing.T) {
	resultJSON, err := json.Marshal(map[string]any{
		"branch": "main",
		"files": []map[string]any{
			{"path": "a.txt", "state": "modified"},
		},
	})
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}
	fake := &fakeInfraFleetServiceClient{
		relayResp: &infrafleetv1.RelayResponse{ResultJson: string(resultJSON)},
	}
	r := NewRelayExecutor(fake)

	status, err := r.GetStatus(ctxWithTenant(t), "/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.Branch != "main" {
		t.Errorf("expected branch=main, got %q", status.Branch)
	}
	if len(status.Files) != 1 || status.Files[0].Path != "a.txt" {
		t.Errorf("unexpected files: %+v", status.Files)
	}
	if fake.gotRelay.GetMethod() != "git.status" {
		t.Errorf("expected method=git.status, got %q", fake.gotRelay.GetMethod())
	}
	if fake.gotRelay.GetConnectionId() != "/repo" {
		t.Errorf("expected connectionId=/repo, got %q", fake.gotRelay.GetConnectionId())
	}

	var params map[string]any
	if err := json.Unmarshal([]byte(fake.gotRelay.GetParamsJson()), &params); err != nil {
		t.Fatalf("unmarshal params: %v", err)
	}
	if params["repoPath"] != "/repo" {
		t.Errorf("expected repoPath param, got %+v", params)
	}
}

func TestRelayExecutor_Commit_Success(t *testing.T) {
	resultJSON, err := json.Marshal(map[string]any{"commitSha": "abc123"})
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}
	fake := &fakeInfraFleetServiceClient{
		relayResp: &infrafleetv1.RelayResponse{ResultJson: string(resultJSON)},
	}
	r := NewRelayExecutor(fake)

	result, err := r.Commit(ctxWithTenant(t), "/repo", "my message", []string{"a.txt"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.CommitSHA != "abc123" {
		t.Errorf("expected commitSha=abc123, got %q", result.CommitSHA)
	}
	if fake.gotRelay.GetMethod() != "git.commit" {
		t.Errorf("expected method=git.commit, got %q", fake.gotRelay.GetMethod())
	}

	var params map[string]any
	if err := json.Unmarshal([]byte(fake.gotRelay.GetParamsJson()), &params); err != nil {
		t.Fatalf("unmarshal params: %v", err)
	}
	if params["message"] != "my message" {
		t.Errorf("expected message param, got %+v", params)
	}
}

func TestRelayExecutor_Push_RelayErrorPropagates(t *testing.T) {
	wantErr := errors.New("boom")
	fake := &fakeInfraFleetServiceClient{relayErr: wantErr}
	r := NewRelayExecutor(fake)

	_, err := r.Push(ctxWithTenant(t), "/repo", "origin", "main")
	if err == nil || !errors.Is(err, wantErr) {
		t.Fatalf("expected wrapped %v, got %v", wantErr, err)
	}
}

func TestRelayExecutor_NoTenantInContext(t *testing.T) {
	fake := &fakeInfraFleetServiceClient{}
	r := NewRelayExecutor(fake)

	if _, err := r.GetStatus(context.Background(), "/repo"); !errors.Is(err, tenant.ErrNoTenant) {
		t.Errorf("expected tenant.ErrNoTenant, got %v", err)
	}
	if fake.gotRelay != nil {
		t.Error("expected Relay not to be called without a tenant in context")
	}
}
