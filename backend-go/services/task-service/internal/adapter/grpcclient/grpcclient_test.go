package grpcclient

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/stablyai/orca-go/common/grpcmw"
	"github.com/stablyai/orca-go/common/tenant"
	aiproviderv1 "github.com/stablyai/orca-go/proto/gen/go/orca/aiprovider/v1"
	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	projectv1 "github.com/stablyai/orca-go/proto/gen/go/orca/project/v1"
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

// fakeDevServerReachability implements usecase.DevServerReachability
// directly — no embed-and-panic needed, the real interface is this
// codebase's own single-method port, not a generated gRPC client.
type fakeDevServerReachability struct {
	reachable      bool
	err            error
	gotDevServerID string
}

func (f *fakeDevServerReachability) IsReachable(ctx context.Context, devServerID string) (bool, error) {
	f.gotDevServerID = devServerID
	return f.reachable, f.err
}

// TestProjectExecutionResolver_NotConnected_NoFallbackAvailable is the
// "genuinely nothing to fall back to" case — the project has no repos at
// all, so resolveViaDefaultRepo also reports not connected. Distinct from
// TestProjectExecutionResolver_NotConnected_FallsBackToReachableDefaultRepo
// below, which is the actually-common case (BUG-025 follow-up).
func TestProjectExecutionResolver_NotConnected_NoFallbackAvailable(t *testing.T) {
	infra := &fakeInfraFleetServiceClient{resolveConnectionResp: &infrafleetv1.ResolveConnectionResponse{Connected: false}}
	projects := &fakeProjectServiceClient{listReposResp: &projectv1.ListReposResponse{Repos: nil}}
	reach := &fakeDevServerReachability{}
	r := NewProjectExecutionResolver(infra, projects, reach)

	connID, worktreePath, _, _, connected, err := r.ResolveConnection(ctxWithTenant(t), "tenant-1", "p1")
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
	if infra.gotResolveConnection.GetConnectionId() != "p1" {
		t.Errorf("expected project_id to pass through verbatim as connection_id, got %q", infra.gotResolveConnection.GetConnectionId())
	}
	if projects.gotListRepos.GetProjectId() != "p1" {
		t.Errorf("expected the fallback to list repos for the same project id, got %q", projects.gotListRepos.GetProjectId())
	}
}

// TestProjectExecutionResolver_FallbackForwardsUserIDToListRepos is the
// regression test for BUG-026's second bite: the fallback's ListRepos call
// initially forwarded only tenant id (withTenantMetadata, deliberately
// scoped to infra-fleet-service calls) — project-service's real ListRepos
// is membership-gated and rejects a tenant-only call with PROJECT_NO_USER,
// live-confirmed as the actual reason the fallback kept reporting
// not-connected even with a real, reachable dev server bound.
func TestProjectExecutionResolver_FallbackForwardsUserIDToListRepos(t *testing.T) {
	infra := &fakeInfraFleetServiceClient{resolveConnectionResp: &infrafleetv1.ResolveConnectionResponse{Connected: false}}
	projects := &fakeProjectServiceClient{listReposResp: &projectv1.ListReposResponse{
		Repos: []*projectv1.Repo{{Id: "repo-1", DevServerId: "ds-1", Url: "/opt/repos/proj"}},
	}}
	reach := &fakeDevServerReachability{reachable: true}
	r := NewProjectExecutionResolver(infra, projects, reach)

	ctx := tenant.WithUserID(ctxWithTenant(t), "user-1")
	if _, _, _, _, connected, err := r.ResolveConnection(ctx, "tenant-1", "p1"); err != nil || !connected {
		t.Fatalf("expected connected=true, nil error, got connected=%v err=%v", connected, err)
	}

	md, ok := metadata.FromOutgoingContext(projects.gotListReposCtx)
	if !ok {
		t.Fatal("expected ListRepos to receive outgoing gRPC metadata")
	}
	if got := md.Get(grpcmw.MetadataTenantID); len(got) == 0 || got[0] != "tenant-1" {
		t.Errorf("expected tenant id forwarded to ListRepos, got %v", got)
	}
	if got := md.Get(grpcmw.MetadataUserID); len(got) == 0 || got[0] != "user-1" {
		t.Errorf("expected user id forwarded to ListRepos, got %v", got)
	}
}

// TestProjectExecutionResolver_NotConnected_FallsBackToReachableDefaultRepo
// is BUG-025 follow-up's regression test — the system-wide norm
// (infra.connections has no row for any project id) must not fail Execute
// outright when the project's default repo has a real, reachable dev
// server bound to it.
func TestProjectExecutionResolver_NotConnected_FallsBackToReachableDefaultRepo(t *testing.T) {
	infra := &fakeInfraFleetServiceClient{resolveConnectionResp: &infrafleetv1.ResolveConnectionResponse{Connected: false}}
	projects := &fakeProjectServiceClient{listReposResp: &projectv1.ListReposResponse{
		Repos: []*projectv1.Repo{{Id: "repo-1", DevServerId: "ds-1", Url: "/opt/repos/proj"}},
	}}
	reach := &fakeDevServerReachability{reachable: true}
	r := NewProjectExecutionResolver(infra, projects, reach)

	_, worktreePath, _, devServerID, connected, err := r.ResolveConnection(ctxWithTenant(t), "tenant-1", "p1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !connected {
		t.Error("expected connected=true via the default-repo reachability fallback")
	}
	if worktreePath != "/opt/repos/proj" {
		t.Errorf("expected worktreePath to fall back to the default repo's own path, got %q", worktreePath)
	}
	if reach.gotDevServerID != "ds-1" {
		t.Errorf("expected reachability to be checked against the default repo's dev server, got %q", reach.gotDevServerID)
	}
	// devServerID must be returned so callers (SimpleExecutor) can relay via
	// RelayByDevServer instead of the connectionId-keyed Relay RPC — this
	// fallback path has no real infra.connections row/connectionID for that
	// RPC to key on. Live-confirmed as INFRA_RELAY_NO_CONNECTION
	// ("connectionId is required") until this was returned (BUG-026 4th bite).
	if devServerID != "ds-1" {
		t.Errorf("expected devServerID to be returned for the fallback path, got %q", devServerID)
	}
}

// TestProjectExecutionResolver_NotConnected_DefaultRepoUnreachable covers
// the fallback's own negative case — a bound-but-unreachable dev server
// must not be treated as connected.
func TestProjectExecutionResolver_NotConnected_DefaultRepoUnreachable(t *testing.T) {
	infra := &fakeInfraFleetServiceClient{resolveConnectionResp: &infrafleetv1.ResolveConnectionResponse{Connected: false}}
	projects := &fakeProjectServiceClient{listReposResp: &projectv1.ListReposResponse{
		Repos: []*projectv1.Repo{{Id: "repo-1", DevServerId: "ds-1", Url: "/opt/repos/proj"}},
	}}
	reach := &fakeDevServerReachability{reachable: false}
	r := NewProjectExecutionResolver(infra, projects, reach)

	_, _, _, _, connected, err := r.ResolveConnection(ctxWithTenant(t), "tenant-1", "p1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if connected {
		t.Error("expected connected=false when the default repo's dev server is not reachable")
	}
}

// TestProjectExecutionResolver_NotConnected_DefaultRepoHasNoDevServer
// covers the fallback's other negative case — a repo that has never been
// bound to any dev server (Repo.DevServerId == "") has nothing to check
// reachability against.
func TestProjectExecutionResolver_NotConnected_DefaultRepoHasNoDevServer(t *testing.T) {
	infra := &fakeInfraFleetServiceClient{resolveConnectionResp: &infrafleetv1.ResolveConnectionResponse{Connected: false}}
	projects := &fakeProjectServiceClient{listReposResp: &projectv1.ListReposResponse{
		Repos: []*projectv1.Repo{{Id: "repo-1", DevServerId: ""}},
	}}
	reach := &fakeDevServerReachability{reachable: true}
	r := NewProjectExecutionResolver(infra, projects, reach)

	_, _, _, _, connected, err := r.ResolveConnection(ctxWithTenant(t), "tenant-1", "p1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if connected {
		t.Error("expected connected=false when the default repo has no dev server bound")
	}
	if reach.gotDevServerID != "" {
		t.Error("expected IsReachable never to be called with no dev server to check")
	}
}

func TestProjectExecutionResolver_Connected(t *testing.T) {
	infra := &fakeInfraFleetServiceClient{resolveConnectionResp: &infrafleetv1.ResolveConnectionResponse{Connected: true, RepoPath: "/srv/worktrees/p1", WorktreeId: "wt-1"}}
	r := NewProjectExecutionResolver(infra, &fakeProjectServiceClient{}, &fakeDevServerReachability{})

	connID, worktreePath, worktreeID, _, connected, err := r.ResolveConnection(ctxWithTenant(t), "tenant-1", "p1")
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
	r := NewProjectExecutionResolver(fake, &fakeProjectServiceClient{}, &fakeDevServerReachability{})

	if _, _, _, _, _, err := r.ResolveConnection(context.Background(), "", "p1"); !errors.Is(err, tenant.ErrNoTenant) {
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
