package usecase

import (
	"context"

	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

// TaskSourceRepository persists the task → external issue link
// (task.task_sources). Separate from TaskRepository on purpose: no existing
// Task column list changes, and a task without a source has no row.
type TaskSourceRepository interface {
	// LinkSource stores src for src.TaskID. Returns domain.ErrSourceAlreadyLinked
	// when another task already holds the same (tenant, project, provider, site, ref).
	LinkSource(ctx context.Context, src domain.TaskSource) error
	// FindTaskIDBySource resolves the task started from an issue; ok is false
	// when none exists. A non-empty site also matches legacy rows stored with
	// site "" (exact-site rows win).
	FindTaskIDBySource(ctx context.Context, tenantID, projectID string, provider domain.SourceProvider, site, ref string) (taskID string, ok bool, err error)
	// GetSource returns the link for taskID; ok is false for a task that was
	// not started from an external issue.
	GetSource(ctx context.Context, tenantID, taskID string) (src domain.TaskSource, ok bool, err error)
}
