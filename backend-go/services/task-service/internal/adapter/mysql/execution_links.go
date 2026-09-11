package mysql

import (
	"context"
	"fmt"

	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

// CreateExecutionLink inserts a new execution_links row. MySQL has no
// RETURNING — id/started_at/status_mirror defaults are generated in Go and
// bound explicitly instead of relying on the table's own DEFAULT
// expressions, so the returned domain.ExecutionLink is guaranteed to match
// exactly what was persisted without a second round trip.
func (r *Repository) CreateExecutionLink(ctx context.Context, tenantID, taskID string, engine domain.ExecutionEngine, externalRefID string) (domain.ExecutionLink, error) {
	link := domain.ExecutionLink{
		ID:            newUUID(),
		TenantID:      tenantID,
		TaskID:        taskID,
		Engine:        engine,
		ExternalRefID: externalRefID,
		StatusMirror:  "in_progress",
	}
	// Insert then read back started_at as two separate calls — MySQL has no
	// RETURNING, and go-sql-driver/mysql doesn't support multi-statement
	// queries by default (multiStatements is off unless explicitly enabled
	// in the DSN, and even then QueryRow only sees the first result set).
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO execution_links (id, tenant_id, task_id, engine, external_ref_id, status_mirror)
		VALUES (?, ?, ?, ?, ?, ?)
	`, link.ID, tenantID, taskID, string(engine), externalRefID, link.StatusMirror)
	if err != nil {
		return domain.ExecutionLink{}, fmt.Errorf("mysql: insert execution link: %w", err)
	}
	if err := r.db.QueryRowContext(ctx, `SELECT started_at FROM execution_links WHERE tenant_id = ? AND id = ?`, tenantID, link.ID).Scan(&link.StartedAt); err != nil {
		return domain.ExecutionLink{}, fmt.Errorf("mysql: read back inserted execution link: %w", err)
	}
	return link, nil
}

func (r *Repository) GetExecutionLink(ctx context.Context, tenantID, id string) (domain.ExecutionLink, error) {
	var link domain.ExecutionLink
	var engine string
	err := r.db.QueryRowContext(ctx, `
		SELECT id, tenant_id, task_id, engine, external_ref_id, status_mirror, started_at, completed_at
		FROM execution_links
		WHERE tenant_id = ? AND id = ?
	`, tenantID, id).Scan(
		&link.ID, &link.TenantID, &link.TaskID, &engine, &link.ExternalRefID, &link.StatusMirror, &link.StartedAt, &link.CompletedAt,
	)
	if err != nil {
		return domain.ExecutionLink{}, fmt.Errorf("mysql: query execution link %s: %w", id, err)
	}
	link.Engine = domain.ExecutionEngine(engine)
	return link, nil
}

func (r *Repository) SetExternalRef(ctx context.Context, tenantID, linkID, externalRefID string) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE execution_links SET external_ref_id = ? WHERE id = ? AND tenant_id = ?
	`, externalRefID, linkID, tenantID)
	if err != nil {
		return fmt.Errorf("mysql: set execution link external ref: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("mysql: set execution link external ref rows affected: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("mysql: set execution link external ref: no row for id %s", linkID)
	}
	return nil
}

func (r *Repository) Complete(ctx context.Context, tenantID, linkID, statusMirror string) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE execution_links SET status_mirror = ?, completed_at = NOW(6) WHERE id = ? AND tenant_id = ?
	`, statusMirror, linkID, tenantID)
	if err != nil {
		return fmt.Errorf("mysql: complete execution link: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("mysql: complete execution link rows affected: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("mysql: complete execution link: no row for id %s", linkID)
	}
	return nil
}

// UpdateStatusMirror deliberately does NOT check RowsAffected — see
// internal/adapter/postgres's identical method for why a stale/duplicate/
// unrelated externalRefID is a legitimate no-op here, not an error.
func (r *Repository) UpdateStatusMirror(ctx context.Context, tenantID, externalRefID, newStatus string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE execution_links SET status_mirror = ? WHERE external_ref_id = ? AND tenant_id = ?
	`, newStatus, externalRefID, tenantID)
	if err != nil {
		return fmt.Errorf("mysql: update execution link status mirror: %w", err)
	}
	return nil
}
