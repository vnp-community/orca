package postgres

import (
	"context"
	"fmt"

	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

// CreateExecutionLink inserts a new task.execution_links row — see
// usecase.ExecutionLinkRepository's doc comment (BE-SOL-001) for why this
// isn't named Create (Repository.Create already exists for TaskRepository).
func (r *Repository) CreateExecutionLink(ctx context.Context, tenantID, taskID string, engine domain.ExecutionEngine, externalRefID string) (domain.ExecutionLink, error) {
	var link domain.ExecutionLink
	err := r.db.QueryRow(ctx, `
		INSERT INTO task.execution_links (tenant_id, task_id, engine, external_ref_id)
		VALUES ($1, $2, $3, $4)
		RETURNING id, tenant_id, task_id, engine, external_ref_id, status_mirror, started_at, completed_at
	`, tenantID, taskID, string(engine), externalRefID).Scan(
		&link.ID, &link.TenantID, &link.TaskID, (*string)(&link.Engine), &link.ExternalRefID, &link.StatusMirror, &link.StartedAt, &link.CompletedAt,
	)
	if err != nil {
		return domain.ExecutionLink{}, fmt.Errorf("postgres: insert execution link: %w", err)
	}
	return link, nil
}

// GetExecutionLink returns one task.execution_links row by id — added for
// ReportTaskExecutionResult (TASK-FT-002-04) to load the task's currently
// active link and validate an inbound callback's execution_ref/engine
// against it. Named GetExecutionLink, not Get, because *Repository already
// has a Get method for TaskRepository with a different signature — same
// naming-collision rationale as CreateExecutionLink vs. Create.
func (r *Repository) GetExecutionLink(ctx context.Context, tenantID, id string) (domain.ExecutionLink, error) {
	var link domain.ExecutionLink
	err := r.db.QueryRow(ctx, `
		SELECT id, tenant_id, task_id, engine, external_ref_id, status_mirror, started_at, completed_at
		FROM task.execution_links
		WHERE tenant_id = $1 AND id = $2
	`, tenantID, id).Scan(
		&link.ID, &link.TenantID, &link.TaskID, (*string)(&link.Engine), &link.ExternalRefID, &link.StatusMirror, &link.StartedAt, &link.CompletedAt,
	)
	if err != nil {
		return domain.ExecutionLink{}, fmt.Errorf("postgres: query execution link %s: %w", id, err)
	}
	return link, nil
}

// SetExternalRef backfills external_ref_id once a dispatch returns its
// coordinator_run_id / workflow execution id.
func (r *Repository) SetExternalRef(ctx context.Context, tenantID, linkID, externalRefID string) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE task.execution_links SET external_ref_id = $1 WHERE id = $2 AND tenant_id = $3
	`, externalRefID, linkID, tenantID)
	if err != nil {
		return fmt.Errorf("postgres: set execution link external ref: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("postgres: set execution link external ref: no row for id %s", linkID)
	}
	return nil
}

// Complete marks a link's status_mirror/completed_at terminal — see
// usecase.ExecutionLinkRepository's doc comment for why this is not
// necessarily the last word for the async engines.
func (r *Repository) Complete(ctx context.Context, tenantID, linkID, statusMirror string) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE task.execution_links SET status_mirror = $1, completed_at = now() WHERE id = $2 AND tenant_id = $3
	`, statusMirror, linkID, tenantID)
	if err != nil {
		return fmt.Errorf("postgres: complete execution link: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("postgres: complete execution link: no row for id %s", linkID)
	}
	return nil
}

// UpdateStatusMirror updates the execution_links row whose external_ref_id
// matches externalRefID — see usecase.ExecutionLinkRepository's doc comment
// (BE-SOL-003/TASK-FT-003-05) for why this deliberately does NOT check
// RowsAffected() the way SetExternalRef/Complete do: a status update
// targets an externalRefID this service does not own the lifecycle of, so a
// stale/duplicate/unrelated ref is a legitimate no-op, not an error.
func (r *Repository) UpdateStatusMirror(ctx context.Context, tenantID, externalRefID, newStatus string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE task.execution_links SET status_mirror = $1 WHERE external_ref_id = $2 AND tenant_id = $3
	`, newStatus, externalRefID, tenantID)
	if err != nil {
		return fmt.Errorf("postgres: update execution link status mirror: %w", err)
	}
	return nil
}
