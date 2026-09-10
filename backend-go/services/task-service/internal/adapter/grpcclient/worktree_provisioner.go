package grpcclient

import (
	"context"
	"fmt"

	gitgatewayv1 "github.com/stablyai/orca-go/proto/gen/go/orca/gitgateway/v1"
	projectv1 "github.com/stablyai/orca-go/proto/gen/go/orca/project/v1"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

// WorktreeProvisioner implements usecase.WorktreeProvisioner against
// git-gateway-service's CreateWorktree RPC — delegates the whole
// create+record saga rather than re-implementing it (SOL-TG-04).
//
// Resolved wiring detail (TASK-TG-04-02's "open wiring detail, flagged
// rather than guessed at, not to be guessed at"): CreateWorktreeRequest
// requires a repo_id (gitgateway.proto's CreateWorktreeRequest) that
// task.ProjectID alone cannot supply. Reading
// git-gateway-service/internal/usecase/create_worktree.go settles which of
// the task file's two options is correct:
//
//   - Option 1 (server-side project_id resolution inside CreateWorktree) is
//     NOT how the saga works: CreateWorktree.Execute resolves exclusively
//     via projects.GetRepo(in.RepoID) — in.ProjectID is accepted on the wire
//     but never read by that usecase today (see that file's own doc comment:
//     "no real caller ever sends project_id on this RPC").
//   - project-service's project→repo cardinality is 1:N, not 1:1
//     (project.repos has a project_id FK, a position ordering column, and
//     ReorderRepos/ListRepos operate over a project's whole repo list) — so
//     even a hypothetical server-side resolution couldn't pick a single repo
//     from project_id alone without a stated default.
//
// This adapter therefore implements Option 2: it resolves RepoID itself via
// project-service's ListRepos, taking the lowest-position entry —
// ListReposResponse's own doc comment states repos come back "ordered by
// position", the same ordering ReorderRepos/ListRepos already establish
// elsewhere. This is a real, explicit assumption (task-service has no
// per-task repo-selection concept yet, only a project_id), not a re-guess
// of the open question — revisit once a task can target a project's
// non-default repo.
type WorktreeProvisioner struct {
	git      gitgatewayv1.GitGatewayServiceClient
	projects projectv1.ProjectServiceClient
}

func NewWorktreeProvisioner(git gitgatewayv1.GitGatewayServiceClient, projects projectv1.ProjectServiceClient) *WorktreeProvisioner {
	return &WorktreeProvisioner{git: git, projects: projects}
}

func (p *WorktreeProvisioner) EnsureWorktree(ctx context.Context, tenantID string, task domain.Task) (worktreeID, path string, err error) {
	if task.WorktreeID != "" {
		return task.WorktreeID, "", nil // reuse — spec's "IF task.worktreeId exists: use existing worktree". Caller resolves the path separately via ProjectExecutionResolver, unchanged from today.
	}

	repoID, err := p.resolveRepoID(ctx, task.ProjectID)
	if err != nil {
		return "", "", err
	}

	ctx, err = withTenantMetadata(ctx)
	if err != nil {
		return "", "", err
	}
	resp, err := p.git.CreateWorktree(ctx, &gitgatewayv1.CreateWorktreeRequest{
		ProjectId: task.ProjectID,
		RepoId:    repoID,
		Branch:    fmt.Sprintf("task/%s", task.ID),
		TaskId:    &task.ID,
	})
	if err != nil {
		return "", "", fmt.Errorf("worktree_provisioner: create worktree: %w", err)
	}
	return resp.GetWorktreeId(), resp.GetPath(), nil
}

// resolveRepoID picks the project's default repo — see this type's doc
// comment for why a lookup is needed here at all and why "lowest position"
// is the chosen default.
func (p *WorktreeProvisioner) resolveRepoID(ctx context.Context, projectID string) (string, error) {
	if projectID == "" {
		return "", fmt.Errorf("worktree_provisioner: task has no project_id, cannot resolve a repo to create a worktree against")
	}
	ctx, err := withTenantMetadata(ctx)
	if err != nil {
		return "", err
	}
	resp, err := p.projects.ListRepos(ctx, &projectv1.ListReposRequest{ProjectId: projectID})
	if err != nil {
		return "", fmt.Errorf("worktree_provisioner: list repos for project %q: %w", projectID, err)
	}
	repos := resp.GetRepos()
	if len(repos) == 0 {
		return "", fmt.Errorf("worktree_provisioner: project %q has no repos", projectID)
	}
	return repos[0].GetId(), nil
}
