package grpcclient

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"google.golang.org/grpc"

	"github.com/stablyai/orca-go/common/tenant"
	aiproviderv1 "github.com/stablyai/orca-go/proto/gen/go/orca/aiprovider/v1"
	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
)

// fakeAiProviderServiceClient implements aiproviderv1.AiProviderServiceClient
// directly — same fake-the-generated-client-port convention as
// fakeInfraFleetServiceClient (simple_executor_test.go).
type fakeAiProviderServiceClient struct {
	aiproviderv1.AiProviderServiceClient // embed: panics on any unimplemented method, intentional for these tests

	resolveProviderResp *aiproviderv1.ResolveProviderResponse
	resolveProviderErr  error
	gotResolveProvider  *aiproviderv1.ResolveProviderRequest
}

func (f *fakeAiProviderServiceClient) ResolveProvider(ctx context.Context, in *aiproviderv1.ResolveProviderRequest, _ ...grpc.CallOption) (*aiproviderv1.ResolveProviderResponse, error) {
	f.gotResolveProvider = in
	if f.resolveProviderErr != nil {
		return nil, f.resolveProviderErr
	}
	return f.resolveProviderResp, nil
}

func ctxWithTenant(t *testing.T) context.Context {
	t.Helper()
	return tenant.WithTenantID(context.Background(), "tenant-1")
}

func TestProjectExecutionResolver_NotConnected(t *testing.T) {
	fake := &fakeInfraFleetServiceClient{resolveConnectionResp: &infrafleetv1.ResolveConnectionResponse{Connected: false}}
	r := NewProjectExecutionResolver(fake)

	connID, worktreePath, _, connected, err := r.ResolveConnection(ctxWithTenant(t), "tenant-1", "p1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if connected {
		t.Error("expected connected=false")
	}
	if connID != "" {
		t.Errorf("expected empty connectionID when not connected, got %q", connID)
	}
	if worktreePath != "" {
		t.Errorf("expected empty worktreePath when not connected, got %q", worktreePath)
	}
	if fake.gotResolveConnection.GetConnectionId() != "p1" {
		t.Errorf("expected project_id to pass through verbatim as connection_id, got %q", fake.gotResolveConnection.GetConnectionId())
	}
}

func TestProjectExecutionResolver_Connected(t *testing.T) {
	fake := &fakeInfraFleetServiceClient{resolveConnectionResp: &infrafleetv1.ResolveConnectionResponse{Connected: true, RepoPath: "/srv/worktrees/p1", WorktreeId: "wt-1"}}
	r := NewProjectExecutionResolver(fake)

	connID, worktreePath, worktreeID, connected, err := r.ResolveConnection(ctxWithTenant(t), "tenant-1", "p1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !connected || connID != "p1" {
		t.Errorf("expected connected=true, connectionID=p1, got connected=%v connID=%q", connected, connID)
	}
	if worktreePath != "/srv/worktrees/p1" {
		t.Errorf("expected worktreePath to pass through from repo_path, got %q", worktreePath)
	}
	if worktreeID != "wt-1" {
		t.Errorf("expected worktreeID to pass through from worktree_id, got %q", worktreeID)
	}
}

func TestProjectExecutionResolver_NoTenantInContext(t *testing.T) {
	fake := &fakeInfraFleetServiceClient{}
	r := NewProjectExecutionResolver(fake)

	if _, _, _, _, err := r.ResolveConnection(context.Background(), "", "p1"); !errors.Is(err, tenant.ErrNoTenant) {
		t.Errorf("expected tenant.ErrNoTenant, got %v", err)
	}
	if fake.gotResolveConnection != nil {
		t.Error("expected ResolveConnection not to be called without a tenant in context")
	}
}

func TestAICompleter_Complete_Success(t *testing.T) {
	fake := &fakeInfraFleetServiceClient{
		relayResp: &infrafleetv1.RelayResponse{ResultJson: `{"content":"hello"}`},
	}
	c := NewAICompleter(fake)

	got, err := c.Complete(ctxWithTenant(t), "conn-1", "prompt text")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "hello" {
		t.Errorf("expected content=hello, got %q", got)
	}
	if fake.gotRelay.GetMethod() != "ai.complete" {
		t.Errorf("expected method=ai.complete, got %q", fake.gotRelay.GetMethod())
	}
	var params map[string]any
	if err := json.Unmarshal([]byte(fake.gotRelay.GetParamsJson()), &params); err != nil {
		t.Fatalf("unmarshal params: %v", err)
	}
	if params["prompt"] != "prompt text" {
		t.Errorf("expected prompt param, got %+v", params)
	}
}

func TestAICompleter_NoTenantInContext(t *testing.T) {
	fake := &fakeInfraFleetServiceClient{}
	c := NewAICompleter(fake)

	if _, err := c.Complete(context.Background(), "conn-1", "prompt"); !errors.Is(err, tenant.ErrNoTenant) {
		t.Errorf("expected tenant.ErrNoTenant, got %v", err)
	}
}

func TestAIProviderContextResolver_ResolveContext_Success(t *testing.T) {
	fake := &fakeAiProviderServiceClient{
		resolveProviderResp: &aiproviderv1.ResolveProviderResponse{
			Account: &aiproviderv1.ProviderAccount{Id: "acct-1", CredentialRef: "cred-ref-1"},
		},
	}
	r := NewAIProviderContextResolver(fake)

	got, err := r.ResolveContext(ctxWithTenant(t), "tenant-1", "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "cred-ref-1" {
		t.Errorf("expected credential_ref to pass through, got %q", got)
	}
	if fake.gotResolveProvider.GetTenantId() != "tenant-1" || fake.gotResolveProvider.GetUserId() != "user-1" {
		t.Errorf("unexpected request: %+v", fake.gotResolveProvider)
	}
}

func TestAIProviderContextResolver_ResolveErrorPropagates(t *testing.T) {
	fake := &fakeAiProviderServiceClient{resolveProviderErr: errors.New("boom")}
	r := NewAIProviderContextResolver(fake)

	if _, err := r.ResolveContext(ctxWithTenant(t), "tenant-1", "user-1"); err == nil {
		t.Fatal("expected an error when ResolveProvider fails")
	}
}
