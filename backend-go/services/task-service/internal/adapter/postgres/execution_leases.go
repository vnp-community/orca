package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

func (r *Repository) StartLease(ctx context.Context, tenantID, linkID, owner, previousStatus string, ttl time.Duration) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE task.execution_links
		SET lease_expires_at = now() + make_interval(secs => $1), lease_owner = $2, previous_status = $3
		WHERE id = $4 AND tenant_id = $5
	`, ttl.Seconds(), owner, previousStatus, linkID, tenantID)
	if err != nil {
		return fmt.Errorf("postgres: start execution lease: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("postgres: start execution lease: no link %s", linkID)
	}
	return nil
}

// RenewLease only extends a lease that is still in progress under this owner,
// so a run that was already swept cannot resurrect its own lease.
func (r *Repository) RenewLease(ctx context.Context, tenantID, linkID, owner string, ttl time.Duration) (bool, error) {
	tag, err := r.db.Exec(ctx, `
		UPDATE task.execution_links
		SET lease_expires_at = now() + make_interval(secs => $1)
		WHERE id = $2 AND tenant_id = $3 AND lease_owner = $4 AND status_mirror = 'in_progress'
	`, ttl.Seconds(), linkID, tenantID, owner)
	if err != nil {
		return false, fmt.Errorf("postgres: renew execution lease: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

// ClaimExpired is one statement: the subquery locks candidates with SKIP
// LOCKED so concurrent sweepers on different instances never claim a row
// twice, and RETURNING hands back exactly the rows this caller flipped.
func (r *Repository) ClaimExpired(ctx context.Context, limit int) ([]domain.ExpiredRun, error) {
	rows, err := r.pool.Query(ctx, `
		UPDATE task.execution_links
		SET status_mirror = 'failed', completed_at = now()
		WHERE id IN (
			SELECT id FROM task.execution_links
			WHERE engine = 'direct_agent' AND status_mirror = 'in_progress'
			  AND lease_expires_at IS NOT NULL AND lease_expires_at < now()
			ORDER BY lease_expires_at
			LIMIT $1
			FOR UPDATE SKIP LOCKED
		)
		RETURNING id::text, tenant_id::text, task_id::text, previous_status
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("postgres: claim expired execution leases: %w", err)
	}
	defer rows.Close()
	var out []domain.ExpiredRun
	for rows.Next() {
		var run domain.ExpiredRun
		if err := rows.Scan(&run.LinkID, &run.TenantID, &run.TaskID, &run.PreviousStatus); err != nil {
			return nil, fmt.Errorf("postgres: scan expired execution lease: %w", err)
		}
		out = append(out, run)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: iterate expired execution leases: %w", err)
	}
	return out, nil
}

// ClaimForExecution is a compare-and-set on status: one statement, so of two
// concurrent Execute calls only one matches the WHERE and gets a row.
func (r *Repository) ClaimForExecution(ctx context.Context, tenantID, taskID string, from domain.Status) (bool, error) {
	tag, err := r.db.Exec(ctx, `
		UPDATE task.tasks SET status = 'in_progress', updated_at = now()
		WHERE tenant_id = $1 AND id = $2 AND status = $3
	`, tenantID, taskID, string(from))
	if err != nil {
		return false, fmt.Errorf("postgres: claim task for execution: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}
