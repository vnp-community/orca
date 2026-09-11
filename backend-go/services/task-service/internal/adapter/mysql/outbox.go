package mysql

import (
	"context"
	"fmt"
	"strings"

	"github.com/stablyai/orca-go/common/outbox"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

// WriteOutboxEvent durably enqueues one event row — mirrors
// internal/adapter/postgres's identical method (not wrapped in the same
// transaction as the write that preceded it, same follow-up flagged there).
func (r *Repository) WriteOutboxEvent(ctx context.Context, tenantID string, event domain.OutboxEvent) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO outbox_events (id, tenant_id, subject, occurred_at, version, payload)
		VALUES (?, ?, ?, ?, 1, ?)
	`, event.ID, tenantID, event.Subject, event.OccurredAt, event.PayloadJSON)
	if err != nil {
		return fmt.Errorf("mysql: insert outbox event: %w", err)
	}
	return nil
}

// InsertOutboxEvent implements usecase.OutboxWriter — see
// internal/adapter/postgres's identical method for why this is a
// deliberately standalone, non-transactional insert.
func (r *Repository) InsertOutboxEvent(ctx context.Context, id, tenantID, subject string, payload []byte) error {
	_, err := r.pool.ExecContext(ctx, `
		INSERT INTO outbox_events (id, tenant_id, subject, occurred_at, version, payload)
		VALUES (?, ?, ?, NOW(6), 1, ?)
	`, id, tenantID, subject, payload)
	if err != nil {
		return fmt.Errorf("mysql: insert outbox event: %w", err)
	}
	return nil
}

// ---- common/outbox.Store -------------------------------------------------

func (r *Repository) FetchUnpublished(ctx context.Context, limit int) ([]outbox.Record, error) {
	rows, err := r.pool.QueryContext(ctx, `
		SELECT id, tenant_id, subject, occurred_at, version, payload
		FROM outbox_events
		WHERE published_at IS NULL
		ORDER BY created_at
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("mysql: query unpublished outbox events: %w", err)
	}
	defer rows.Close()

	var out []outbox.Record
	for rows.Next() {
		var rec outbox.Record
		if err := rows.Scan(&rec.ID, &rec.Event.TenantID, &rec.Subject, &rec.Event.OccurredAt, &rec.Event.Version, &rec.Event.Payload); err != nil {
			return nil, fmt.Errorf("mysql: scan outbox event row: %w", err)
		}
		rec.Event.ID = rec.ID
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("mysql: iterate outbox event rows: %w", err)
	}
	return out, nil
}

// MarkPublished builds a dynamic `IN (?,...)` — MySQL has no `= ANY($1)`
// equivalent, same translation as this service's other multi-id writers
// (grants.go's ListGrantsForAncestors, BE-DB-SOL-002 §3's MarkPublished
// precedent). Guarded for len(ids) == 0 (MySQL's `IN ()` is a syntax
// error).
func (r *Repository) MarkPublished(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	args := make([]any, 0, len(ids))
	for _, id := range ids {
		args = append(args, id)
	}
	query := fmt.Sprintf(`UPDATE outbox_events SET published_at = NOW(6) WHERE id IN (%s)`, placeholders)
	_, err := r.pool.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("mysql: mark outbox events published: %w", err)
	}
	return nil
}
