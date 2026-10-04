package mysql

import (
	"context"
	"fmt"
	"strings"

	"github.com/stablyai/orca-go/common/outbox"
	"github.com/stablyai/orca-go/services/credential-broker-service/internal/domain"
)

// EnqueueEvent implements usecase.EventEnqueuer; see postgres.Repository.EnqueueEvent.
func (r *Repository) EnqueueEvent(ctx context.Context, e domain.OutboxEvent) error {
	_, err := r.conn.ExecContext(ctx, `
		INSERT INTO outbox_events (id, tenant_id, subject, occurred_at, version, payload)
		VALUES (?,?,?,?,1,?)
	`, e.ID, e.TenantID, e.Subject, e.OccurredAt, e.Payload)
	if err != nil {
		return fmt.Errorf("mysql: enqueue outbox event: %w", err)
	}
	return nil
}

// FetchUnpublished implements common/outbox.Store.
func (r *Repository) FetchUnpublished(ctx context.Context, limit int) ([]outbox.Record, error) {
	rows, err := r.conn.QueryContext(ctx, `
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
		var payload []byte
		if err := rows.Scan(&rec.ID, &rec.Event.TenantID, &rec.Subject, &rec.Event.OccurredAt, &rec.Event.Version, &payload); err != nil {
			return nil, fmt.Errorf("mysql: scan outbox event row: %w", err)
		}
		rec.Event.Payload = payload
		rec.Event.ID = rec.ID
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("mysql: iterate outbox event rows: %w", err)
	}
	return out, nil
}

// MarkPublished implements common/outbox.Store; MySQL has no array bind, so IN (?,...).
func (r *Repository) MarkPublished(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i], args[i] = "?", id
	}
	if _, err := r.conn.ExecContext(ctx, `UPDATE outbox_events SET published_at = NOW(6) WHERE id IN (`+strings.Join(placeholders, ",")+`)`, args...); err != nil {
		return fmt.Errorf("mysql: mark outbox events published: %w", err)
	}
	return nil
}
