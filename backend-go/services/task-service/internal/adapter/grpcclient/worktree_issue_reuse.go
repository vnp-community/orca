package grpcclient

import (
	"context"
	"log/slog"

	"google.golang.org/grpc/metadata"

	"github.com/stablyai/orca-go/common/grpcmw"
	"github.com/stablyai/orca-go/common/tenant"
	projectv1 "github.com/stablyai/orca-go/proto/gen/go/orca/project/v1"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

// TaskSourceReader is the slice of the task-source store the provisioner
// needs: which external issue, if any, a task was started from.
type TaskSourceReader interface {
	GetSource(ctx context.Context, tenantID, taskID string) (domain.TaskSource, bool, error)
}

// WithTaskSources enables issue-aware worktree reuse.
func (p *WorktreeProvisioner) WithTaskSources(sources TaskSourceReader) *WorktreeProvisioner {
	p.sources = sources
	return p
}

// findIssueWorktree returns an existing worktree already linked to the task's
// external issue (and not owned by a different task), so running a task that
// was started from "Start work" on a Jira issue lands in the worktree the
// user already has instead of forking a second one.
//
// Best-effort by design: any lookup problem falls back to the old behavior
// (create a fresh worktree), never blocks execution.
func (p *WorktreeProvisioner) findIssueWorktree(ctx context.Context, tenantID string, task domain.Task) (id, path string, ok bool) {
	if task.ProjectID == "" {
		return "", "", false
	}
	src, found := p.sourceFor(ctx, tenantID, task)
	if !found {
		return "", "", false
	}

	userID, _ := tenant.UserID(ctx)
	projectCtx := metadata.AppendToOutgoingContext(ctx, grpcmw.MetadataTenantID, tenantID, grpcmw.MetadataUserID, userID)
	resp, err := p.projects.ListWorktrees(projectCtx, &projectv1.ListWorktreesRequest{ProjectId: task.ProjectID})
	if err != nil {
		slog.WarnContext(ctx, "worktree_provisioner: list worktrees failed, creating a new worktree", slog.String("task_id", task.ID), slog.Any("error", err))
		return "", "", false
	}
	for _, wt := range resp.GetWorktrees() {
		if wt.GetLinkedIssueProvider() != string(src.Provider) || wt.GetLinkedIssueRef() != src.Ref {
			continue
		}
		if wt.GetStatus() != "" && wt.GetStatus() != "active" {
			continue // completed/error/stopped worktrees are not a place to resume work
		}
		if owner := wt.GetTaskId(); owner != "" && owner != task.ID {
			continue // another task already owns it
		}
		if wt.GetPath() == "" {
			continue
		}
		return wt.GetId(), wt.GetPath(), true
	}
	return "", "", false
}

// sourceFor returns the external issue the task was started from. Best-effort:
// a lookup failure is logged and treated as "no source" so worktree
// provisioning never depends on the task-source table being reachable.
func (p *WorktreeProvisioner) sourceFor(ctx context.Context, tenantID string, task domain.Task) (domain.TaskSource, bool) {
	if p.sources == nil {
		return domain.TaskSource{}, false
	}
	src, found, err := p.sources.GetSource(ctx, tenantID, task.ID)
	if err != nil {
		slog.WarnContext(ctx, "worktree_provisioner: task source lookup failed", slog.String("task_id", task.ID), slog.Any("error", err))
		return domain.TaskSource{}, false
	}
	return src, found
}
