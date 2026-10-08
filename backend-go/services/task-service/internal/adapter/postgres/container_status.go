package postgres

import (
	"context"
	"fmt"

	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

// ListChildStatuses returns the status of every direct child of parentID.
func (r *Repository) ListChildStatuses(ctx context.Context, tenantID, parentID string) ([]domain.Status, error) {
	rows, err := r.db.Query(ctx, `SELECT status FROM task.tasks WHERE tenant_id = $1 AND parent_id = $2::uuid`, tenantID, parentID)
	if err != nil {
		return nil, fmt.Errorf("postgres: list child statuses: %w", err)
	}
	defer rows.Close()
	var out []domain.Status
	for rows.Next() {
		var s domain.Status
		if err := rows.Scan(&s); err != nil {
			return nil, fmt.Errorf("postgres: scan child status: %w", err)
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
		tag, err := db.Exec(ctx, `
			UPDATE task.tasks SET status = $4, updated_at = now()
			WHERE tenant_id = $1 AND id = $2::uuid AND status = $3 AND task_type IN ('plan','phase')
		`, tenantID, id, string(from), string(to))
		if err != nil {
			return fmt.Errorf("postgres: update container status: %w", err)
		}
		if tag.RowsAffected() == 0 {
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
// SKIP LOCKED lets several instances take disjoint rows if the caller holds a transaction.
func (r *Repository) ListStaleContainers(ctx context.Context, limit int) ([]domain.Task, error) {
	rows, err := r.db.Query(ctx, `
		SELECT `+prefixedTaskColumns("p")+`
		FROM task.tasks p
		WHERE p.task_type IN ('plan','phase') AND p.status <> 'cancelled'
		  AND p.updated_at < (SELECT MAX(c.updated_at) FROM task.tasks c WHERE c.tenant_id = p.tenant_id AND c.parent_id = p.id)
		ORDER BY p.updated_at
		LIMIT $1
		FOR UPDATE OF p SKIP LOCKED
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("postgres: list stale containers: %w", err)
	}
	defer rows.Close()
	var out []domain.Task
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres: scan stale container: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// TouchContainer bumps updated_at of a plan/phase so a checked, unchanged container leaves the stale set.
func (r *Repository) TouchContainer(ctx context.Context, tenantID, id string) error {
	if _, err := r.db.Exec(ctx, `UPDATE task.tasks SET updated_at = now() WHERE tenant_id = $1 AND id = $2::uuid AND task_type IN ('plan','phase')`, tenantID, id); err != nil {
		return fmt.Errorf("postgres: touch container: %w", err)
	}
	return nil
}
