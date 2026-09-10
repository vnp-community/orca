// project_client.go implements usecase.ProjectClient against
// project-service's gRPC surface. RecordWorktreeCreated/RecordWorktreeRemoved/
// ListWorktrees/GetWorktree need no project-service proto change beyond
// SOL-WT-04's base_ref/GetWorktree additions — GetRepo is this task's own
// addition on top of that surface — see its doc comment below for the
// confirmed gap.
package grpcclient

import (
	"context"

	projectv1 "github.com/stablyai/orca-go/proto/gen/go/orca/project/v1"

	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

type ProjectClient struct {
	client projectv1.ProjectServiceClient
}

func NewProjectClient(client projectv1.ProjectServiceClient) *ProjectClient {
	return &ProjectClient{client: client}
}

// GetRepo backs project.proto's ProjectService.GetRepo RPC (added to close
// the confirmed gap this doc comment previously described: project.proto
// had no single-repo-by-id lookup, only ListRepos(project_id)/AddRepo/
// ReorderRepos/RemoveRepo, none of which answer "does this repo exist /
// what project is it under" for an arbitrary repo id). Every caller
// (CreateWorktree, DetectWorktrees, PrefetchCreateBase, ResolvePrBase,
// ResolveMrBase) still maps a not-found/unreachable result to
// WORKTREE_REPO_NOT_FOUND itself; this adapter just forwards the real
// answer (or a real error) instead of an always-failing stub.
func (p *ProjectClient) GetRepo(ctx context.Context, repoID string) (domain.RepoInfo, error) {
	ctx, err := withTenantMetadata(ctx)
	if err != nil {
		return domain.RepoInfo{}, err
	}
	resp, err := p.client.GetRepo(ctx, &projectv1.GetRepoRequest{RepoId: repoID})
	if err != nil {
		return domain.RepoInfo{}, err
	}
	r := resp.GetRepo()
	return domain.RepoInfo{
		ID:          r.GetId(),
		ProjectID:   r.GetProjectId(),
		URL:         r.GetUrl(),
		DisplayName: r.GetDisplayName(),
		DevServerID: resp.GetDevServerId(),
		// HiddenTargetID (TASK-BE-EVM-018, BE-SOL-EVM-004 §6c — closes
		// TASK-BE-EVM-015's gap #2 at the wire level): project-service's
		// GetRepo does not populate a real value yet (see
		// project.proto's GetRepoResponse.hidden_target_id doc comment for
		// the still-open architectural join), so this is always "" today —
		// but the mapping itself is real, not a stub, ready the moment
		// project-service starts populating it.
		HiddenTargetID: resp.GetHiddenTargetId(),
	}, nil
}

// RecordWorktreeCreated forwards both baseRef (SOL-WT-04's base_ref
// backfill, used by CompareWorktrees' BR-WT-13 check) and the full lineage
// capture (SOL-PI-02/SOL-PI-03 linked-issue fields plus the
// parent-worktree/orchestration lineage fields) — these are independent
// optional inputs, not competing shapes for the same field.
func (p *ProjectClient) RecordWorktreeCreated(ctx context.Context, projectID, repoID, path, branch, baseRef string, lineage domain.WorktreeLineageCapture) (domain.WorktreeRecord, error) {
	ctx, err := withTenantMetadata(ctx)
	if err != nil {
		return domain.WorktreeRecord{}, err
	}
	req := &projectv1.RecordWorktreeCreatedRequest{
		ProjectId: projectID, RepoId: repoID, Path: path, Branch: branch,
		BaseRef: nonEmptyPtr(baseRef),
	}
	// Optional string proto fields want a *string, not "" — nonEmptyPtr
	// keeps every unsupplied lineage field genuinely unset on the wire
	// rather than an empty-but-present string.
	req.LinkedIssueProvider = nonEmptyPtr(lineage.LinkedIssueProvider)
	req.LinkedIssueRef = nonEmptyPtr(lineage.LinkedIssueRef)
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
	return domain.WorktreeRecord{ID: wt.GetId(), RepoID: wt.GetRepoId(), Path: wt.GetPath(), Branch: wt.GetBranch(), Active: wt.GetActive()}, nil
}

func (p *ProjectClient) RecordWorktreeRemoved(ctx context.Context, worktreeID string) error {
	ctx, err := withTenantMetadata(ctx)
	if err != nil {
		return err
	}
	_, err = p.client.RecordWorktreeRemoved(ctx, &projectv1.RecordWorktreeRemovedRequest{WorktreeId: worktreeID})
	return err
}

// FindWorktreeByIdempotencyKey backs BR-CLI-01 — see
// usecase.ProjectClient.FindWorktreeByIdempotencyKey's doc comment.
func (p *ProjectClient) FindWorktreeByIdempotencyKey(ctx context.Context, projectID, idempotencyKey string) (domain.WorktreeRecord, bool, error) {
	ctx, err := withTenantMetadata(ctx)
	if err != nil {
		return domain.WorktreeRecord{}, false, err
	}
	resp, err := p.client.GetWorktreeByIdempotencyKey(ctx, &projectv1.GetWorktreeByIdempotencyKeyRequest{
		ProjectId: projectID, IdempotencyKey: idempotencyKey,
	})
	if err != nil {
		return domain.WorktreeRecord{}, false, err
	}
	if !resp.GetFound() {
		return domain.WorktreeRecord{}, false, nil
	}
	wt := resp.GetWorktree()
	return domain.WorktreeRecord{ID: wt.GetId(), Path: wt.GetPath(), Branch: wt.GetBranch()}, true, nil
}

// IsIssueStatusSyncEnabled reads project-service's per-project
// issue_status_sync_enabled flag (BR-PI-06/TASK-PI-02-06) via GetProject.
func (p *ProjectClient) IsIssueStatusSyncEnabled(ctx context.Context, projectID string) (bool, error) {
	ctx, err := withTenantMetadata(ctx)
	if err != nil {
		return false, err
	}
	resp, err := p.client.GetProject(ctx, &projectv1.GetProjectRequest{Id: projectID})
	if err != nil {
		return false, err
	}
	return resp.GetProject().GetIssueStatusSyncEnabled(), nil
}

// ListWorktrees backs BR-WT-04's count cap (max 20 active worktrees per
// repo) — project-service.ListWorktrees(project_id) already exists; this is
// a new call on an existing RPC, not a new proto surface.
func (p *ProjectClient) ListWorktrees(ctx context.Context, projectID string) ([]domain.WorktreeRecord, error) {
	ctx, err := withTenantMetadata(ctx)
	if err != nil {
		return nil, err
	}
	resp, err := p.client.ListWorktrees(ctx, &projectv1.ListWorktreesRequest{ProjectId: projectID})
	if err != nil {
		return nil, err
	}
	out := make([]domain.WorktreeRecord, 0, len(resp.GetWorktrees()))
	for _, wt := range resp.GetWorktrees() {
		out = append(out, domain.WorktreeRecord{ID: wt.GetId(), RepoID: wt.GetRepoId(), Path: wt.GetPath(), Branch: wt.GetBranch(), Active: wt.GetActive()})
	}
	return out, nil
}

// GetWorktree wraps project-service's GetWorktree RPC (SOL-WT-04) —
// CompareWorktrees uses it to look up each compared worktree's
// repo_id/branch/base_ref.
func (p *ProjectClient) GetWorktree(ctx context.Context, worktreeID string) (domain.WorktreeInfo, error) {
	ctx, err := withTenantMetadata(ctx)
	if err != nil {
		return domain.WorktreeInfo{}, err
	}
	wt, err := p.client.GetWorktree(ctx, &projectv1.GetWorktreeRequest{WorktreeId: worktreeID})
	if err != nil {
		return domain.WorktreeInfo{}, err
	}
	return domain.WorktreeInfo{ID: wt.GetId(), RepoID: wt.GetRepoId(), Branch: wt.GetBranch(), BaseRef: wt.GetBaseRef()}, nil
}

// nonEmptyPtr returns nil for an empty string, otherwise a pointer to s —
// this file's convention for populating a proto `optional string` field
// from a plain Go string param.
func nonEmptyPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
