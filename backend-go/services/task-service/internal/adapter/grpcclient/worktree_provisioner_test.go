package grpcclient

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc"

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
}

func (f *fakeProjectServiceClient) ListRepos(ctx context.Context, in *projectv1.ListReposRequest, _ ...grpc.CallOption) (*projectv1.ListReposResponse, error) {
	f.gotListRepos = in
	if f.listReposErr != nil {
		return nil, f.listReposErr
	}
	return f.listReposResp, nil
}

// TestWorktreeProvisioner_ReusesExistingWorktree is the core reuse
// regression: a task with a non-empty WorktreeID must never call
// CreateWorktree (or even look up a repo for it) — SOL-TG-04's "IF
// task.worktreeId exists: use existing worktree".
func TestWorktreeProvisioner_ReusesExistingWorktree(t *testing.T) {
	git := &fakeGitGatewayCreateWorktreeClient{}
	projects := &fakeProjectServiceClient{}
	p := NewWorktreeProvisioner(git, projects)

	worktreeID, path, err := p.EnsureWorktree(ctxWithTenant(t), "tenant-1", domain.Task{ID: "task-1", ProjectID: "proj-1", WorktreeID: "wt-existing"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if worktreeID != "wt-existing" {
		t.Errorf("expected the task's existing worktree id to be reused, got %q", worktreeID)
	}
	if path != "" {
		t.Errorf("expected empty path on reuse (caller resolves it via ProjectExecutionResolver), got %q", path)
	}
	if git.createWorktreeCalled {
		t.Error("expected CreateWorktree NOT to be called when the task already has a worktree")
	}
	if projects.gotListRepos != nil {
		t.Error("expected ListRepos NOT to be called when the task already has a worktree")
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
