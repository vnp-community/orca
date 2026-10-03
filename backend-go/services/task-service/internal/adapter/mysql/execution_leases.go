package mysql

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

func (r *Repository) StartLease(ctx context.Context, tenantID, linkID, owner, previousStatus string, ttl time.Duration) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE execution_links
		SET lease_expires_at = DATE_ADD(NOW(6), INTERVAL ? MICROSECOND), lease_owner = ?, previous_status = ?
		WHERE id = ? AND tenant_id = ?
	`, ttl.Microseconds(), owner, previousStatus, linkID, tenantID)
	if err != nil {
		return fmt.Errorf("mysql: start execution lease: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("mysql: start execution lease: no link %s", linkID)
	}
	return nil
}

func (r *Repository) RenewLease(ctx context.Context, tenantID, linkID, owner string, ttl time.Duration) (bool, error) {
	res, err := r.db.ExecContext(ctx, `
		UPDATE execution_links
		SET lease_expires_at = DATE_ADD(NOW(6), INTERVAL ? MICROSECOND)
		WHERE id = ? AND tenant_id = ? AND lease_owner = ? AND status_mirror = 'in_progress'
	`, ttl.Microseconds(), linkID, tenantID, owner)
	if err != nil {
		return false, fmt.Errorf("mysql: renew execution lease: %w", err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// claimLinks: MySQL has no UPDATE ... RETURNING, so a claim is a short
// transaction — SELECT ... FOR UPDATE SKIP LOCKED (MySQL 8.0.1+) pins the rows
// for this caller, then one UPDATE flips them. Concurrent sweepers skip each
// other's locked rows, so no row is returned twice. where selects candidates.
func (r *Repository) claimLinks(ctx context.Context, where string, orderBy string, limit int, args ...any) ([]domain.ExpiredRun, error) {
	tx, err := r.pool.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("mysql: begin claim execution links: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	rows, err := tx.QueryContext(ctx, `
		SELECT id, tenant_id, task_id, previous_status FROM execution_links
		WHERE `+where+`
		ORDER BY `+orderBy+`
		LIMIT ?
		FOR UPDATE SKIP LOCKED
	`, append(args, limit)...)
	if err != nil {
		return nil, fmt.Errorf("mysql: select execution links to claim: %w", err)
	}
	var out []domain.ExpiredRun
	for rows.Next() {
		var run domain.ExpiredRun
		if err := rows.Scan(&run.LinkID, &run.TenantID, &run.TaskID, &run.PreviousStatus); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("mysql: scan execution link to claim: %w", err)
		}
		out = append(out, run)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, fmt.Errorf("mysql: iterate execution links to claim: %w", err)
	}
	_ = rows.Close()
	if len(out) == 0 {
		return nil, nil
	}

	ids := make([]any, len(out))
	marks := make([]string, len(out))
	for i, run := range out {
		ids[i], marks[i] = run.LinkID, "?"
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE execution_links SET status_mirror = 'failed', completed_at = NOW(6) WHERE id IN (`+strings.Join(marks, ",")+`)`,
		ids...); err != nil {
		return nil, fmt.Errorf("mysql: mark claimed execution links failed: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("mysql: commit claim execution links: %w", err)
	}
	return out, nil
}

func (r *Repository) ClaimExpired(ctx context.Context, limit int) ([]domain.ExpiredRun, error) {
	return r.claimLinks(ctx,
		`engine = 'direct_agent' AND status_mirror = 'in_progress' AND lease_expires_at IS NOT NULL AND lease_expires_at < NOW(6)`,
		`lease_expires_at`, limit)
}

func (r *Repository) SetPreviousStatus(ctx context.Context, tenantID, linkID, status string) error {
	if _, err := r.db.ExecContext(ctx, `UPDATE execution_links SET previous_status = ? WHERE id = ? AND tenant_id = ?`, status, linkID, tenantID); err != nil {
		return fmt.Errorf("mysql: set execution link previous status: %w", err)
	}
	return nil
}

// ClaimLegacyStuck sweeps direct_agent runs written before leases existed
// (lease_expires_at IS NULL); olderThan must exceed the executor's hard cap.
func (r *Repository) ClaimLegacyStuck(ctx context.Context, olderThan time.Duration, limit int) ([]domain.ExpiredRun, error) {
	return r.claimLinks(ctx,
		`engine = 'direct_agent' AND status_mirror = 'in_progress' AND lease_expires_at IS NULL AND started_at < DATE_SUB(NOW(6), INTERVAL ? MICROSECOND)`,
		`started_at`, limit, olderThan.Microseconds())
}

// ListOrphanedRuns: see the Postgres adapter for the engine/status rationale.
func (r *Repository) ListOrphanedRuns(ctx context.Context, grace time.Duration, limit int) ([]domain.StuckTask, error) {
	rows, err := r.pool.QueryContext(ctx, `
		SELECT t.tenant_id, t.id, l.id, l.status_mirror, l.previous_status
		FROM tasks t
		JOIN execution_links l ON l.id = t.active_execution_link_id
		WHERE t.status = 'in_progress'
		  AND l.completed_at IS NOT NULL AND l.completed_at < DATE_SUB(NOW(6), INTERVAL ? MICROSECOND)
		  AND ((l.engine = 'direct_agent' AND l.status_mirror IN ('failed','completed'))
		    OR (l.engine IN ('orchestration','workflow') AND l.status_mirror = 'failed'))
		ORDER BY l.completed_at
		LIMIT ?
	`, grace.Microseconds(), limit)
	if err != nil {
		return nil, fmt.Errorf("mysql: list orphaned runs: %w", err)
	}
	defer rows.Close()
	var out []domain.StuckTask
	for rows.Next() {
		var s domain.StuckTask
		if err := rows.Scan(&s.TenantID, &s.TaskID, &s.LinkID, &s.LinkStatus, &s.PreviousStatus); err != nil {
			return nil, fmt.Errorf("mysql: scan orphaned run: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// ReleaseExecution moves an in_progress task to `to` only while the given link
// is still its active one. RowsAffected is reliable: status always changes.
func (r *Repository) ReleaseExecution(ctx context.Context, tenantID, taskID, linkID string, to domain.Status) (bool, error) {
	res, err := r.db.ExecContext(ctx, `
		UPDATE tasks SET status = ?, updated_at = NOW(6)
		WHERE tenant_id = ? AND id = ? AND status = 'in_progress' AND active_execution_link_id = ?
	`, string(to), tenantID, taskID, linkID)
	if err != nil {
		return false, fmt.Errorf("mysql: release task execution: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("mysql: release task execution rows affected: %w", err)
	}
	return n > 0, nil
}

// ClaimForExecution is a compare-and-set on status. RowsAffected is reliable
// here: the statement always changes status (from is never in_progress), so a
// matched row is always a changed row.
func (r *Repository) ClaimForExecution(ctx context.Context, tenantID, taskID string, from domain.Status) (bool, error) {
	res, err := r.db.ExecContext(ctx, `
		UPDATE tasks SET status = 'in_progress', updated_at = NOW(6)
		WHERE tenant_id = ? AND id = ? AND status = ?
	`, tenantID, taskID, string(from))
	if err != nil {
		return false, fmt.Errorf("mysql: claim task for execution: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("mysql: claim task for execution rows affected: %w", err)
	}
	return n > 0, nil
}
