// project_client.go implements usecase.ProjectClient against
// project-service's gRPC surface. RecordWorktreeCreated/RecordWorktreeRemoved
// need no project-service proto change — those RPCs already exist
// (proto/orca/project/v1/project.proto). GetRepo is this task's own
// addition on top of that surface — see its doc comment below for the
// confirmed gap.
package grpcclient

import (
	"context"

	projectv1 "github.com/stablyai/orca-go/proto/gen/go/orca/project/v1"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

type ProjectClient struct {
	client projectv1.ProjectServiceClient
}

func NewProjectClient(client projectv1.ProjectServiceClient) *ProjectClient {
	return &ProjectClient{client: client}
}

// GetRepo is a CONFIRMED GAP, not a scaffold stub: project.proto's
// ProjectService has no RPC to look up a single Repo by id — only
// ListRepos(project_id), which needs a project id this call doesn't have,
// and AddRepo/ReorderRepos/RemoveRepo, none of which answer "does this
// repo exist / what project is it under" for an arbitrary repo id.
// TASK-193 originally sketched this against a project.proto GetRepo RPC
// and Repo.dev_server_id/Repo.path fields that don't exist in the real
// proto (backend-go/proto/orca/project/v1/project.proto's Repo message is
// just {id, project_id, url, display_name, position}). Rather than invent
// a request/response shape or add an out-of-scope RPC to a different
// service's proto (git-gateway-service.md's own service boundary, and
// this task's stated scope, is git-gateway-service only), this returns a
// typed, catchable error — the same treatment internal/adapter/scmclient
// gives its own confirmed proto gap. Every caller (CreateWorktree,
// DetectWorktrees, PrefetchCreateBase, ResolvePrBase, ResolveMrBase) maps
// this to WORKTREE_REPO_NOT_FOUND today; a real answer needs a follow-up
// project-service proto task to add a by-id repo lookup.
func (p *ProjectClient) GetRepo(ctx context.Context, repoID string) (domain.RepoInfo, error) {
	return domain.RepoInfo{}, apperrors.New(apperrors.KindInternal, "PROJECT_GET_REPO_UNIMPLEMENTED",
		"project-service has no RPC to fetch a single repo by id yet (only ListRepos(project_id)); see project_client.go's GetRepo doc comment", nil)
}

func (p *ProjectClient) RecordWorktreeCreated(ctx context.Context, projectID, repoID, path, branch string, lineage domain.WorktreeLineageCapture) (domain.WorktreeRecord, error) {
	ctx, err := withTenantMetadata(ctx)
	if err != nil {
		return domain.WorktreeRecord{}, err
	}
	req := &projectv1.RecordWorktreeCreatedRequest{
		ProjectId: projectID, RepoId: repoID, Path: path, Branch: branch,
	}
	// Optional string proto fields want a *string, not "" — nonEmptyPtr
	// keeps every unsupplied lineage field genuinely unset on the wire
	// rather than an empty-but-present string.
	req.ParentWorktreeId = nonEmptyPtr(lineage.ParentWorktreeID)
	req.Origin = nonEmptyPtr(lineage.Origin)
	req.CaptureSource = nonEmptyPtr(lineage.CaptureSource)
	req.TaskId = nonEmptyPtr(lineage.TaskID)
	req.OrchestrationRunId = nonEmptyPtr(lineage.OrchestrationRunID)
	req.CoordinatorHandle = nonEmptyPtr(lineage.CoordinatorHandle)
	req.CreatedByTerminalHandle = nonEmptyPtr(lineage.CreatedByTerminalHandle)

	resp, err := p.client.RecordWorktreeCreated(ctx, req)
	if err != nil {
		return domain.WorktreeRecord{}, err
	}
	wt := resp.GetWorktree()
	return domain.WorktreeRecord{ID: wt.GetId(), Path: wt.GetPath(), Branch: wt.GetBranch()}, nil
}

// nonEmptyPtr returns nil for an empty string, else a pointer to it —
// distinguishes "not supplied" from a genuinely empty value for every
// optional lineage field on RecordWorktreeCreatedRequest.
func nonEmptyPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func (p *ProjectClient) RecordWorktreeRemoved(ctx context.Context, worktreeID string) error {
	ctx, err := withTenantMetadata(ctx)
	if err != nil {
		return err
	}
	_, err = p.client.RecordWorktreeRemoved(ctx, &projectv1.RecordWorktreeRemovedRequest{WorktreeId: worktreeID})
	return err
}
