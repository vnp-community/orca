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
	tenantv1 "github.com/stablyai/orca-go/proto/gen/go/orca/tenant/v1"
)

// fakeTenantServiceClient implements tenantv1.TenantServiceClient directly
// — same fake-the-generated-client-port convention as
// fakeAiProviderServiceClient/fakeInfraFleetServiceClient above.
type fakeTenantServiceClient struct {
	tenantv1.TenantServiceClient // embed: panics on any unimplemented method, intentional for these tests

	listTeamsForUserResp *tenantv1.ListTeamsForUserResponse
	listTeamsForUserErr  error
	gotListTeamsForUser  *tenantv1.ListTeamsForUserRequest
}

func (f *fakeTenantServiceClient) ListTeamsForUser(ctx context.Context, in *tenantv1.ListTeamsForUserRequest, _ ...grpc.CallOption) (*tenantv1.ListTeamsForUserResponse, error) {
	f.gotListTeamsForUser = in
	if f.listTeamsForUserErr != nil {
		return nil, f.listTeamsForUserErr
	}
	return f.listTeamsForUserResp, nil
}

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

	connID, worktreePath, connected, err := r.ResolveConnection(ctxWithTenant(t), "tenant-1", "p1")
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
	fake := &fakeInfraFleetServiceClient{resolveConnectionResp: &infrafleetv1.ResolveConnectionResponse{Connected: true, RepoPath: "/srv/worktrees/p1"}}
	r := NewProjectExecutionResolver(fake)

	connID, worktreePath, connected, err := r.ResolveConnection(ctxWithTenant(t), "tenant-1", "p1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !connected || connID != "p1" {
		t.Errorf("expected connected=true, connectionID=p1, got connected=%v connID=%q", connected, connID)
	}
	if worktreePath != "/srv/worktrees/p1" {
		t.Errorf("expected worktreePath to pass through from repo_path, got %q", worktreePath)
	}
}

func TestProjectExecutionResolver_NoTenantInContext(t *testing.T) {
	fake := &fakeInfraFleetServiceClient{}
	r := NewProjectExecutionResolver(fake)

	if _, _, _, err := r.ResolveConnection(context.Background(), "", "p1"); !errors.Is(err, tenant.ErrNoTenant) {
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

// TestTeamScopeResolver_ResolveTeams_CallsListTeamsForUserWithOnlyUserID is
// TASK-TG-003-01's core regression test: the real RPC is ListTeamsForUser
// (not ListUserTeams, BE-SOL-003's own guessed name), and its request must
// carry ONLY user_id — tenant_id must never be added back to the wire
// request, since ListTeamsForUserRequest has no such field (tenant.proto:221-227).
func TestTeamScopeResolver_ResolveTeams_CallsListTeamsForUserWithOnlyUserID(t *testing.T) {
	fake := &fakeTenantServiceClient{listTeamsForUserResp: &tenantv1.ListTeamsForUserResponse{TeamIds: []string{"team-1", "team-2"}}}
	r := NewTeamScopeResolver(fake)

	got, err := r.ResolveTeams(ctxWithTenant(t), "tenant-1", "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 || got[0] != "team-1" || got[1] != "team-2" {
		t.Errorf("expected team IDs to pass through, got %+v", got)
	}
	if fake.gotListTeamsForUser.GetUserId() != "user-1" {
		t.Errorf("expected user_id=user-1, got %q", fake.gotListTeamsForUser.GetUserId())
	}
}

// TestTeamScopeResolver_ResolveErrorPropagates_NotSilentEmptyList is the
// regression test versus the old StubTeamScopeResolver's always-succeeds-
// with-nil behavior: a real tenant-service error must be a real, wrapped
// error, never silently swallowed into an empty team list.
func TestTeamScopeResolver_ResolveErrorPropagates_NotSilentEmptyList(t *testing.T) {
	fake := &fakeTenantServiceClient{listTeamsForUserErr: errors.New("tenant-service unavailable")}
	r := NewTeamScopeResolver(fake)

	got, err := r.ResolveTeams(ctxWithTenant(t), "tenant-1", "user-1")
	if err == nil {
		t.Fatal("expected an error to propagate from tenant-service, not a silent empty list")
	}
	if got != nil {
		t.Errorf("expected a nil team list on error, got %+v", got)
	}
}

func TestTeamScopeResolver_NoTenantInContext(t *testing.T) {
	fake := &fakeTenantServiceClient{}
	r := NewTeamScopeResolver(fake)

	if _, err := r.ResolveTeams(context.Background(), "", "user-1"); !errors.Is(err, tenant.ErrNoTenant) {
		t.Errorf("expected tenant.ErrNoTenant, got %v", err)
	}
	if fake.gotListTeamsForUser != nil {
		t.Error("expected ListTeamsForUser not to be called without a tenant in context")
	}
}
