package mysql

import (
	"context"
	"fmt"

	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

// ListChildStatuses returns the status of every direct child of parentID.
func (r *Repository) ListChildStatuses(ctx context.Context, tenantID, parentID string) ([]domain.Status, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT status FROM tasks WHERE tenant_id = ? AND parent_id = ?`, tenantID, parentID)
	if err != nil {
		return nil, fmt.Errorf("mysql: list child statuses: %w", err)
	}
	defer rows.Close()
	var out []domain.Status
	for rows.Next() {
		var s domain.Status
		if err := rows.Scan(&s); err != nil {
			return nil, fmt.Errorf("mysql: scan child status: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// UpdateContainerStatus is a CAS on a plan/phase row; the outbox rows share the transaction,
// so a derived status change and its event are never observed apart.
func (r *Repository) UpdateContainerStatus(ctx context.Context, tenantID, id string, from, to domain.Status, events []domain.OutboxEvent) (bool, error) {
	changed := false
	// needTx=true: a standalone call still opens its own tx; inside RunInTx it joins the caller's.
	err := r.inOptionalTx(ctx, true, func(db dbtx) error {
		// from != to is guaranteed by the caller, so RowsAffected (changed rows) equals matched rows.
		res, err := db.ExecContext(ctx, `
			UPDATE tasks SET status = ?, updated_at = NOW(6)
			WHERE tenant_id = ? AND id = ? AND status = ? AND task_type IN ('plan','phase')
		`, string(to), tenantID, id, string(from))
		if err != nil {
			return fmt.Errorf("mysql: update container status: %w", err)
		}
		n, err := res.RowsAffected()
		if err != nil {
			return fmt.Errorf("mysql: container status rows affected: %w", err)
		}
		if n == 0 {
			return nil
		}
		changed = true
		return insertOutboxEvents(ctx, db, tenantID, events)
	})
	if err != nil {
		return false, err
	}
	return changed, nil
}

// ListStaleContainers returns plan/phase rows older than their newest child (cross-tenant).
// FOR UPDATE SKIP LOCKED needs MySQL 8.0.1+.
func (r *Repository) ListStaleContainers(ctx context.Context, limit int) ([]domain.Task, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+prefixedTaskColumns("p")+`
		FROM tasks p
		WHERE p.task_type IN ('plan','phase') AND p.status <> 'cancelled'
		  AND p.updated_at < (SELECT MAX(c.updated_at) FROM tasks c WHERE c.tenant_id = p.tenant_id AND c.parent_id = p.id)
		ORDER BY p.updated_at
		LIMIT ?
		FOR UPDATE SKIP LOCKED
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("mysql: list stale containers: %w", err)
	}
	defer rows.Close()
	var out []domain.Task
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, fmt.Errorf("mysql: scan stale container: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// TouchContainer bumps updated_at of a plan/phase so a checked, unchanged container leaves the stale set.
func (r *Repository) TouchContainer(ctx context.Context, tenantID, id string) error {
	if _, err := r.db.ExecContext(ctx, `UPDATE tasks SET updated_at = NOW(6) WHERE tenant_id = ? AND id = ? AND task_type IN ('plan','phase')`, tenantID, id); err != nil {
		return fmt.Errorf("mysql: touch container: %w", err)
	}
	return nil
}
