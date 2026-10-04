package postgres

import (
	"context"
	"fmt"

	"github.com/stablyai/orca-go/common/outbox"
	"github.com/stablyai/orca-go/services/credential-broker-service/internal/domain"
)

// EnqueueEvent implements usecase.EventEnqueuer. Called on the
// transaction-scoped Repository, so the row commits with the change that caused it.
func (r *Repository) EnqueueEvent(ctx context.Context, e domain.OutboxEvent) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO credential.outbox_events (id, tenant_id, subject, occurred_at, version, payload)
		VALUES ($1,$2,$3,$4,1,$5)
	`, e.ID, e.TenantID, e.Subject, e.OccurredAt, e.Payload)
	if err != nil {
		return fmt.Errorf("postgres: enqueue outbox event: %w", err)
	}
	return nil
}

// FetchUnpublished implements common/outbox.Store.
func (r *Repository) FetchUnpublished(ctx context.Context, limit int) ([]outbox.Record, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, tenant_id, subject, occurred_at, version, payload
		FROM credential.outbox_events
		WHERE published_at IS NULL
		ORDER BY created_at
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("postgres: query unpublished outbox events: %w", err)
	}
	defer rows.Close()

	var out []outbox.Record
	for rows.Next() {
		var rec outbox.Record
		if err := rows.Scan(&rec.ID, &rec.Event.TenantID, &rec.Subject, &rec.Event.OccurredAt, &rec.Event.Version, &rec.Event.Payload); err != nil {
			return nil, fmt.Errorf("postgres: scan outbox event row: %w", err)
		}
		rec.Event.ID = rec.ID
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: iterate outbox event rows: %w", err)
	}
	return out, nil
}

// MarkPublished implements common/outbox.Store.
func (r *Repository) MarkPublished(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	if _, err := r.db.Exec(ctx, `UPDATE credential.outbox_events SET published_at = now() WHERE id = ANY($1)`, ids); err != nil {
		return fmt.Errorf("postgres: mark outbox events published: %w", err)
	}
	return nil
}
