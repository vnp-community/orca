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
// create+record saga rather than re-implementing it.
//
// Open wiring detail this task's Context flagged, resolved: CreateWorktreeRequest
// requires a repo_id (gitgateway.proto's CreateWorktreeRequest.repo_id,
// field 2) that task.ProjectID alone doesn't resolve. Checked
// project-service's actual cardinality before choosing
// (project-service.md / project.proto's AddRepo/ListRepos RPCs: a project
// can have MULTIPLE repos, ordered by position) — this adapter resolves
// RepoID itself via project-service.ListRepos and uses the first repo by
// position (the project's "primary"/default repo), the same convention
// ProjectContextResolver (TASK-TG-02-04) already uses for its repoURL.
// git-gateway-service's own CreateWorktree saga is NOT changed to add
// project_id-based resolution server-side — that would duplicate this
// lookup in two places for no benefit.
type WorktreeProvisioner struct {
	git     gitgatewayv1.GitGatewayServiceClient
	project projectv1.ProjectServiceClient
}

func NewWorktreeProvisioner(git gitgatewayv1.GitGatewayServiceClient, project projectv1.ProjectServiceClient) *WorktreeProvisioner {
	return &WorktreeProvisioner{git: git, project: project}
}

func (p *WorktreeProvisioner) EnsureWorktree(ctx context.Context, tenantID string, task domain.Task) (worktreeID, path string, err error) {
	if task.WorktreeID != "" {
		return task.WorktreeID, "", nil // reuse — spec's "IF task.worktreeId exists: use existing worktree". Caller resolves the path separately via ProjectExecutionResolver, unchanged from today.
	}

	ctx, err = withTenantMetadata(ctx)
	if err != nil {
		return "", "", err
	}

	reposResp, err := p.project.ListRepos(ctx, &projectv1.ListReposRequest{ProjectId: task.ProjectID})
	if err != nil {
		return "", "", fmt.Errorf("worktree_provisioner: list repos for project %q: %w", task.ProjectID, err)
	}
	repos := reposResp.GetRepos()
	if len(repos) == 0 {
		return "", "", fmt.Errorf("worktree_provisioner: project %q has no repos configured", task.ProjectID)
	}

	resp, err := p.git.CreateWorktree(ctx, &gitgatewayv1.CreateWorktreeRequest{
		ProjectId: task.ProjectID,
		RepoId:    repos[0].GetId(),
		Branch:    fmt.Sprintf("task/%s", task.ID),
	})
	if err != nil {
		return "", "", fmt.Errorf("worktree_provisioner: create worktree: %w", err)
	}
	return resp.GetWorktreeId(), resp.GetPath(), nil
}
