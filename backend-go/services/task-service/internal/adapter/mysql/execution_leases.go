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

// ClaimExpired: MySQL has no UPDATE ... RETURNING, so the claim is a short
// transaction — SELECT ... FOR UPDATE SKIP LOCKED (MySQL 8.0.1+) pins the rows
// for this caller, then one UPDATE flips them. Concurrent sweepers skip each
// other's locked rows, so no row is returned twice.
func (r *Repository) ClaimExpired(ctx context.Context, limit int) ([]domain.ExpiredRun, error) {
	tx, err := r.pool.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("mysql: begin claim expired leases: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	rows, err := tx.QueryContext(ctx, `
		SELECT id, tenant_id, task_id, previous_status FROM execution_links
		WHERE engine = 'direct_agent' AND status_mirror = 'in_progress'
		  AND lease_expires_at IS NOT NULL AND lease_expires_at < NOW(6)
		ORDER BY lease_expires_at
		LIMIT ?
		FOR UPDATE SKIP LOCKED
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("mysql: select expired execution leases: %w", err)
	}
	var out []domain.ExpiredRun
	for rows.Next() {
		var run domain.ExpiredRun
		if err := rows.Scan(&run.LinkID, &run.TenantID, &run.TaskID, &run.PreviousStatus); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("mysql: scan expired execution lease: %w", err)
		}
		out = append(out, run)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, fmt.Errorf("mysql: iterate expired execution leases: %w", err)
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
		return nil, fmt.Errorf("mysql: mark expired execution leases failed: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("mysql: commit claim expired leases: %w", err)
	}
	return out, nil
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
