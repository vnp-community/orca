package grpcclient

import (
	"context"
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
// here since this file only needs CreateWorktree.
type fakeGitGatewayCreateWorktreeClient struct {
	gitgatewayv1.GitGatewayServiceClient

	createWorktreeResp *gitgatewayv1.CreateWorktreeResponse
	createWorktreeErr  error
	gotCreateWorktree  *gitgatewayv1.CreateWorktreeRequest
}

func (f *fakeGitGatewayCreateWorktreeClient) CreateWorktree(ctx context.Context, in *gitgatewayv1.CreateWorktreeRequest, _ ...grpc.CallOption) (*gitgatewayv1.CreateWorktreeResponse, error) {
	f.gotCreateWorktree = in
	if f.createWorktreeErr != nil {
		return nil, f.createWorktreeErr
	}
	return f.createWorktreeResp, nil
}

// fakeProjectServiceClient implements projectv1.ProjectServiceClient
// directly — same fake-the-generated-client-port convention as
// fakeInfraFleetServiceClient/fakeGitGatewayServiceClient.
type fakeProjectServiceClient struct {
	projectv1.ProjectServiceClient

	listReposResp *projectv1.ListReposResponse
	listReposErr  error
}

func (f *fakeProjectServiceClient) ListRepos(ctx context.Context, in *projectv1.ListReposRequest, _ ...grpc.CallOption) (*projectv1.ListReposResponse, error) {
	if f.listReposErr != nil {
		return nil, f.listReposErr
	}
	return f.listReposResp, nil
}

func TestWorktreeProvisioner_ExistingWorktreeID_NeverCallsCreateWorktree(t *testing.T) {
	git := &fakeGitGatewayCreateWorktreeClient{}
	project := &fakeProjectServiceClient{}
	p := NewWorktreeProvisioner(git, project)

	task := domain.Task{ID: "t1", ProjectID: "p1", WorktreeID: "existing-wt"}
	worktreeID, path, err := p.EnsureWorktree(ctxWithTenant(t), "tenant-1", task)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if worktreeID != "existing-wt" {
		t.Errorf("expected the existing worktree id to be reused, got %q", worktreeID)
	}
	if path != "" {
		t.Errorf("expected an empty path for the reuse case, got %q", path)
	}
	if git.gotCreateWorktree != nil {
		t.Error("expected CreateWorktree to never be called when task.WorktreeID is already set")
	}
}

func TestWorktreeProvisioner_EmptyWorktreeID_CreatesNewOne(t *testing.T) {
	git := &fakeGitGatewayCreateWorktreeClient{
		createWorktreeResp: &gitgatewayv1.CreateWorktreeResponse{WorktreeId: "new-wt", Path: "/srv/worktrees/new-wt"},
	}
	project := &fakeProjectServiceClient{
		listReposResp: &projectv1.ListReposResponse{Repos: []*projectv1.Repo{{Id: "repo-1", Position: 0}, {Id: "repo-2", Position: 1}}},
	}
	p := NewWorktreeProvisioner(git, project)

	task := domain.Task{ID: "t1", ProjectID: "p1"}
	worktreeID, path, err := p.EnsureWorktree(ctxWithTenant(t), "tenant-1", task)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if worktreeID != "new-wt" || path != "/srv/worktrees/new-wt" {
		t.Errorf("expected the created worktree's id/path, got %q/%q", worktreeID, path)
	}
	if git.gotCreateWorktree.GetProjectId() != "p1" {
		t.Errorf("expected project_id=p1, got %q", git.gotCreateWorktree.GetProjectId())
	}
	if git.gotCreateWorktree.GetRepoId() != "repo-1" {
		t.Errorf("expected the first repo (by position) to be used as repo_id, got %q", git.gotCreateWorktree.GetRepoId())
	}
	if git.gotCreateWorktree.GetBranch() != "task/t1" {
		t.Errorf("expected branch=task/t1, got %q", git.gotCreateWorktree.GetBranch())
	}
}

func TestWorktreeProvisioner_NoReposConfigured_ReturnsError(t *testing.T) {
	git := &fakeGitGatewayCreateWorktreeClient{}
	project := &fakeProjectServiceClient{listReposResp: &projectv1.ListReposResponse{}}
	p := NewWorktreeProvisioner(git, project)

	task := domain.Task{ID: "t1", ProjectID: "p1"}
	if _, _, err := p.EnsureWorktree(ctxWithTenant(t), "tenant-1", task); err == nil {
		t.Fatal("expected an error when the project has no repos configured")
	}
	if git.gotCreateWorktree != nil {
		t.Error("expected CreateWorktree to never be called when repo resolution fails")
	}
}
