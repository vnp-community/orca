package grpcclient

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/stablyai/orca-go/common/grpcmw"
	"github.com/stablyai/orca-go/common/tenant"
	gitgatewayv1 "github.com/stablyai/orca-go/proto/gen/go/orca/gitgateway/v1"
	projectv1 "github.com/stablyai/orca-go/proto/gen/go/orca/project/v1"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

// fakeGitGatewayCreateWorktreeClient implements
// gitgatewayv1.GitGatewayServiceClient directly (panics on any
// unimplemented method via the embed) — same convention as
// fakeGitGatewayServiceClient (tech_stack_detector_test.go), kept separate
// here (distinct name) since that fake already owns ReadFile for a
// different file's tests and this one only needs CreateWorktree.
type fakeGitGatewayCreateWorktreeClient struct {
	gitgatewayv1.GitGatewayServiceClient // embed: panics on any unimplemented method, intentional for these tests

	createWorktreeResp   *gitgatewayv1.CreateWorktreeResponse
	createWorktreeErr    error
	gotCreateWorktree    *gitgatewayv1.CreateWorktreeRequest
	createWorktreeCalled bool
}

func (f *fakeGitGatewayCreateWorktreeClient) CreateWorktree(ctx context.Context, in *gitgatewayv1.CreateWorktreeRequest, _ ...grpc.CallOption) (*gitgatewayv1.CreateWorktreeResponse, error) {
	f.createWorktreeCalled = true
	f.gotCreateWorktree = in
	if f.createWorktreeErr != nil {
		return nil, f.createWorktreeErr
	}
	return f.createWorktreeResp, nil
}

// fakeProjectServiceClient implements projectv1.ProjectServiceClient
// directly — same convention as fakeGitGatewayCreateWorktreeClient above.
type fakeProjectServiceClient struct {
	projectv1.ProjectServiceClient // embed: panics on any unimplemented method, intentional for these tests

	listReposResp *projectv1.ListReposResponse
	listReposErr  error
	gotListRepos  *projectv1.ListReposRequest
	// gotListReposCtx captures the outgoing context ListRepos was called
	// with — BUG-026 follow-up's regression test reads its outgoing
	// metadata to confirm both tenant AND user id were forwarded (real
	// project-service rejects a tenant-only call with PROJECT_NO_USER).
	gotListReposCtx context.Context

	// getWorktreeResp/Err back BUG-028's resolveWorktreePath (the reuse
	// branch's real-path resolution) — gotGetWorktreeCtx mirrors
	// gotListReposCtx's same regression-test purpose for this RPC.
	getWorktreeResp   *projectv1.Worktree
	getWorktreeErr    error
	gotGetWorktree    *projectv1.GetWorktreeRequest
	gotGetWorktreeCtx context.Context
}

func (f *fakeProjectServiceClient) GetWorktree(ctx context.Context, in *projectv1.GetWorktreeRequest, _ ...grpc.CallOption) (*projectv1.Worktree, error) {
	f.gotGetWorktree = in
	f.gotGetWorktreeCtx = ctx
	if f.getWorktreeErr != nil {
		return nil, f.getWorktreeErr
	}
	return f.getWorktreeResp, nil
}

func (f *fakeProjectServiceClient) ListRepos(ctx context.Context, in *projectv1.ListReposRequest, _ ...grpc.CallOption) (*projectv1.ListReposResponse, error) {
	f.gotListRepos = in
	f.gotListReposCtx = ctx
	if f.listReposErr != nil {
		return nil, f.listReposErr
	}
	return f.listReposResp, nil
}

// TestWorktreeProvisioner_ReusesExistingWorktree is the core reuse
// regression: a task with a non-empty WorktreeID must never call
// CreateWorktree (or look up a repo via ListRepos) — SOL-TG-04's "IF
// task.worktreeId exists: use existing worktree". BUG-028: the reuse
// branch now resolves the worktree's REAL, isolated path via
// project-service's GetWorktree, instead of returning an empty path that
// callers used to (wrongly) fall back to the project's shared repo root
// for — live-confirmed as agent dispatches running in the wrong, shared
// checkout on every task re-execution.
func TestWorktreeProvisioner_ReusesExistingWorktree(t *testing.T) {
	git := &fakeGitGatewayCreateWorktreeClient{}
	projects := &fakeProjectServiceClient{getWorktreeResp: &projectv1.Worktree{Id: "wt-existing", Path: "/opt/repos/proj-1-task-task-1"}}
	p := NewWorktreeProvisioner(git, projects)

	worktreeID, path, err := p.EnsureWorktree(ctxWithTenant(t), "tenant-1", domain.Task{ID: "task-1", ProjectID: "proj-1", WorktreeID: "wt-existing"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if worktreeID != "wt-existing" {
		t.Errorf("expected the task's existing worktree id to be reused, got %q", worktreeID)
	}
	if path != "/opt/repos/proj-1-task-task-1" {
		t.Errorf("expected the worktree's own real, isolated path (BUG-028), got %q", path)
	}
	if projects.gotGetWorktree.GetWorktreeId() != "wt-existing" {
		t.Errorf("expected GetWorktree to be called with the task's existing worktree id, got %q", projects.gotGetWorktree.GetWorktreeId())
	}
	if git.createWorktreeCalled {
		t.Error("expected CreateWorktree NOT to be called when the task already has a worktree")
	}
	if projects.gotListRepos != nil {
		t.Error("expected ListRepos NOT to be called when the task already has a worktree")
	}
}

// TestWorktreeProvisioner_ReuseExistingWorktree_GetWorktreeError_FailsClosed
// is BUG-028's fail-closed regression: a GetWorktree error must propagate
// as EnsureWorktree's own error, never silently degrade to a guessed path
// (a silent guess is exactly the bug being fixed).
func TestWorktreeProvisioner_ReuseExistingWorktree_GetWorktreeError_FailsClosed(t *testing.T) {
	git := &fakeGitGatewayCreateWorktreeClient{}
	projects := &fakeProjectServiceClient{getWorktreeErr: errors.New("project-service unavailable")}
	p := NewWorktreeProvisioner(git, projects)

	if _, _, err := p.EnsureWorktree(ctxWithTenant(t), "tenant-1", domain.Task{ID: "task-1", ProjectID: "proj-1", WorktreeID: "wt-existing"}); err == nil {
		t.Fatal("expected an error when GetWorktree fails, not a silently guessed path")
	}
	if git.createWorktreeCalled {
		t.Error("expected CreateWorktree NOT to be called when the worktree path resolution fails")
	}
}

// TestWorktreeProvisioner_CreatesWorktreeForTaskWithNoExistingOne is the
// create-branch regression: an empty WorktreeID resolves the project's
// default repo (lowest position) via ListRepos, then calls CreateWorktree
// with it, returning CreateWorktreeResponse's id/path verbatim.
func TestWorktreeProvisioner_CreatesWorktreeForTaskWithNoExistingOne(t *testing.T) {
	git := &fakeGitGatewayCreateWorktreeClient{createWorktreeResp: &gitgatewayv1.CreateWorktreeResponse{WorktreeId: "wt-new", Path: "/srv/worktrees/wt-new", HeadSha: "abc123"}}
	projects := &fakeProjectServiceClient{listReposResp: &projectv1.ListReposResponse{Repos: []*projectv1.Repo{
		{Id: "repo-1", ProjectId: "proj-1", Position: 0},
		{Id: "repo-2", ProjectId: "proj-1", Position: 1},
	}}}
	p := NewWorktreeProvisioner(git, projects)

	worktreeID, path, err := p.EnsureWorktree(ctxWithTenant(t), "tenant-1", domain.Task{ID: "task-1", ProjectID: "proj-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if worktreeID != "wt-new" || path != "/srv/worktrees/wt-new" {
		t.Errorf("expected CreateWorktreeResponse's id/path to pass through, got id=%q path=%q", worktreeID, path)
	}
	if !git.createWorktreeCalled {
		t.Fatal("expected CreateWorktree to be called for a task with no existing worktree")
	}
	if got := git.gotCreateWorktree.GetRepoId(); got != "repo-1" {
		t.Errorf("expected the lowest-position repo (repo-1) to be resolved, got %q", got)
	}
	if got := git.gotCreateWorktree.GetProjectId(); got != "proj-1" {
		t.Errorf("expected project_id to pass through, got %q", got)
	}
	if got := git.gotCreateWorktree.GetBranch(); got != "task/task-1" {
		t.Errorf(`expected branch "task/task-1", got %q`, got)
	}
	if got := git.gotCreateWorktree.GetTaskId(); got != "task-1" {
		t.Errorf("expected task_id to be set, got %q", got)
	}
}

// TestWorktreeProvisioner_ResolveRepoIDForwardsUserIDToListRepos is the
// regression test for BUG-026's sibling bug: resolveRepoID's ListRepos call
// used tenant-only metadata (withTenantMetadata), but project-service's
// real ListRepos is membership-gated and rejects a tenant-only call with
// PROJECT_NO_USER — live-confirmed as TASK_EXECUTE_WORKTREE_FAILED on every
// first-time task execution, the exact same root cause
// ProjectExecutionResolver.resolveViaDefaultRepo already had fixed for it.
func TestWorktreeProvisioner_ResolveRepoIDForwardsUserIDToListRepos(t *testing.T) {
	git := &fakeGitGatewayCreateWorktreeClient{createWorktreeResp: &gitgatewayv1.CreateWorktreeResponse{WorktreeId: "wt-new", Path: "/srv/worktrees/wt-new"}}
	projects := &fakeProjectServiceClient{listReposResp: &projectv1.ListReposResponse{
		Repos: []*projectv1.Repo{{Id: "repo-1", ProjectId: "proj-1", Position: 0}},
	}}
	p := NewWorktreeProvisioner(git, projects)

	ctx := tenant.WithUserID(ctxWithTenant(t), "user-1")
	if _, _, err := p.EnsureWorktree(ctx, "tenant-1", domain.Task{ID: "task-1", ProjectID: "proj-1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
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

func TestWorktreeProvisioner_NoReposForProject_ReturnsError(t *testing.T) {
	git := &fakeGitGatewayCreateWorktreeClient{}
	projects := &fakeProjectServiceClient{listReposResp: &projectv1.ListReposResponse{}}
	p := NewWorktreeProvisioner(git, projects)

	if _, _, err := p.EnsureWorktree(ctxWithTenant(t), "tenant-1", domain.Task{ID: "task-1", ProjectID: "proj-1"}); err == nil {
		t.Fatal("expected an error when the project has no repos")
	}
	if git.createWorktreeCalled {
		t.Error("expected CreateWorktree NOT to be called when repo resolution fails")
	}
}

func TestWorktreeProvisioner_ListReposError_Propagates(t *testing.T) {
	git := &fakeGitGatewayCreateWorktreeClient{}
	projects := &fakeProjectServiceClient{listReposErr: errors.New("project-service unavailable")}
	p := NewWorktreeProvisioner(git, projects)

	if _, _, err := p.EnsureWorktree(ctxWithTenant(t), "tenant-1", domain.Task{ID: "task-1", ProjectID: "proj-1"}); err == nil {
		t.Fatal("expected an error when ListRepos fails")
	}
}

func TestWorktreeProvisioner_CreateWorktreeError_Propagates(t *testing.T) {
	git := &fakeGitGatewayCreateWorktreeClient{createWorktreeErr: errors.New("git-gateway-service unavailable")}
	projects := &fakeProjectServiceClient{listReposResp: &projectv1.ListReposResponse{Repos: []*projectv1.Repo{{Id: "repo-1", ProjectId: "proj-1"}}}}
	p := NewWorktreeProvisioner(git, projects)

	if _, _, err := p.EnsureWorktree(ctxWithTenant(t), "tenant-1", domain.Task{ID: "task-1", ProjectID: "proj-1"}); err == nil {
		t.Fatal("expected an error when CreateWorktree fails")
	}
}
