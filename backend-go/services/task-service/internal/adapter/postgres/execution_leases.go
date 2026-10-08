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
func (r *Repository) ClaimForExecution(ctx context.Context, tenantID, taskID string, from domain.Status, events []domain.OutboxEvent) (bool, error) {
	claimed := false
	err := r.inOptionalTx(ctx, len(events) > 0, func(db dbtx) error {
		tag, err := db.Exec(ctx, `
			UPDATE task.tasks SET status = 'in_progress', updated_at = now()
			WHERE tenant_id = $1 AND id = $2 AND status = $3
		`, tenantID, taskID, string(from))
		if err != nil {
			return fmt.Errorf("postgres: claim task for execution: %w", err)
		}
		claimed = tag.RowsAffected() > 0
		if !claimed {
			return nil // lost the race: no outbox row
		}
		return insertOutboxEvents(ctx, db, tenantID, events)
	})
	if err != nil {
		return false, err
	}
	return claimed, nil
}

func (r *Repository) SetPreviousStatus(ctx context.Context, tenantID, linkID, status string) error {
	_, err := r.db.Exec(ctx, `UPDATE task.execution_links SET previous_status = $1 WHERE id = $2 AND tenant_id = $3`, status, linkID, tenantID)
	if err != nil {
		return fmt.Errorf("postgres: set execution link previous status: %w", err)
	}
	return nil
}

// ClaimLegacyStuck sweeps direct_agent runs written before leases existed
// (lease_expires_at IS NULL). Such a run cannot be alive past the executor's
// own hard cap, so olderThan must exceed that cap with margin.
func (r *Repository) ClaimLegacyStuck(ctx context.Context, olderThan time.Duration, limit int) ([]domain.ExpiredRun, error) {
	rows, err := r.pool.Query(ctx, `
		UPDATE task.execution_links
		SET status_mirror = 'failed', completed_at = now()
		WHERE id IN (
			SELECT id FROM task.execution_links
			WHERE engine = 'direct_agent' AND status_mirror = 'in_progress'
			  AND lease_expires_at IS NULL AND started_at < now() - make_interval(secs => $1)
			ORDER BY started_at
			LIMIT $2
			FOR UPDATE SKIP LOCKED
		)
		RETURNING id::text, tenant_id::text, task_id::text, previous_status
	`, olderThan.Seconds(), limit)
	if err != nil {
		return nil, fmt.Errorf("postgres: claim legacy stuck execution links: %w", err)
	}
	defer rows.Close()
	var out []domain.ExpiredRun
	for rows.Next() {
		var run domain.ExpiredRun
		if err := rows.Scan(&run.LinkID, &run.TenantID, &run.TaskID, &run.PreviousStatus); err != nil {
			return nil, fmt.Errorf("postgres: scan legacy stuck link: %w", err)
		}
		out = append(out, run)
	}
	return out, rows.Err()
}

// ListOrphanedRuns finds tasks still in_progress whose active link already
// ended. Engines 2/3 mark the link 'completed' right after dispatch (task-service
// bookkeeping while the run is still going), so only 'failed' is a real end for
// them; direct_agent is synchronous in-process, so 'completed' is a real end too.
// Not claimed here — ReleaseExecution is a compare-and-set, so concurrent
// sweepers are safe.
func (r *Repository) ListOrphanedRuns(ctx context.Context, grace time.Duration, limit int) ([]domain.StuckTask, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT t.tenant_id::text, t.id::text, l.id::text, l.status_mirror, l.previous_status
		FROM task.tasks t
		JOIN task.execution_links l ON l.id = t.active_execution_link_id
		WHERE t.status = 'in_progress'
		  AND l.completed_at IS NOT NULL AND l.completed_at < now() - make_interval(secs => $1)
		  AND ((l.engine = 'direct_agent' AND l.status_mirror IN ('failed','completed'))
		    OR (l.engine IN ('orchestration','workflow') AND l.status_mirror = 'failed'))
		ORDER BY l.completed_at
		LIMIT $2
	`, grace.Seconds(), limit)
	if err != nil {
		return nil, fmt.Errorf("postgres: list orphaned runs: %w", err)
	}
	defer rows.Close()
	var out []domain.StuckTask
	for rows.Next() {
		var s domain.StuckTask
		if err := rows.Scan(&s.TenantID, &s.TaskID, &s.LinkID, &s.LinkStatus, &s.PreviousStatus); err != nil {
			return nil, fmt.Errorf("postgres: scan orphaned run: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// ReleaseExecution moves an in_progress task to `to`, but only while the given
// link is still its active one — so it can never undo a newer dispatch.
func (r *Repository) ReleaseExecution(ctx context.Context, tenantID, taskID, linkID string, to domain.Status, events []domain.OutboxEvent) (bool, error) {
	released := false
	err := r.inOptionalTx(ctx, len(events) > 0, func(db dbtx) error {
		tag, err := db.Exec(ctx, `
			UPDATE task.tasks SET status = $4, updated_at = now()
			WHERE tenant_id = $1 AND id = $2 AND status = 'in_progress' AND active_execution_link_id = $3
		`, tenantID, taskID, linkID, string(to))
		if err != nil {
			return fmt.Errorf("postgres: release task execution: %w", err)
		}
		released = tag.RowsAffected() > 0
		if !released {
			return nil // stale link: no outbox row
		}
		return insertOutboxEvents(ctx, db, tenantID, events)
	})
	if err != nil {
		return false, err
	}
	return released, nil
}

// ReleaseUnlinkedInProgress skips plan/phase: their in_progress is derived and never has a link.
// It is one guarded UPDATE; the SKIP LOCKED subquery
// lets concurrent sweepers take disjoint rows and the repeated predicates
// keep it a compare-and-set against a task that was just dispatched.
func (r *Repository) ReleaseUnlinkedInProgress(ctx context.Context, grace time.Duration, limit int) (int, error) {
	tag, err := r.db.Exec(ctx, `
		UPDATE task.tasks SET status = 'open', updated_at = now()
		WHERE id IN (
			SELECT id FROM task.tasks
			WHERE status = 'in_progress' AND active_execution_link_id IS NULL
			  AND task_type NOT IN ('plan','phase')
			  AND updated_at < now() - make_interval(secs => $1)
			ORDER BY updated_at
			LIMIT $2
			FOR UPDATE SKIP LOCKED
		)
		AND status = 'in_progress' AND active_execution_link_id IS NULL
		AND task_type NOT IN ('plan','phase')
	`, grace.Seconds(), limit)
	if err != nil {
		return 0, fmt.Errorf("postgres: release unlinked in_progress tasks: %w", err)
	}
	return int(tag.RowsAffected()), nil
}
