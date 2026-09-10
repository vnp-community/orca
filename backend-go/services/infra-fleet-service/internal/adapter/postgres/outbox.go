package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/stablyai/orca-go/common/outbox"
)

// EnqueueOutboxEvent implements adapter/eventbus.OutboxEnqueuer — a direct
// (non-transactional) INSERT, since HealthPublisher.PublishStatusChange has
// no error return and does not run inside UpsertFleetHealth's own write
// (see that method's doc comment for why this is fire-and-forget rather
// than the SAME-transaction outbox write usage-service's SaveSession
// performs). A crash between UpsertFleetHealth and this call just means the
// status_change event is silently missed for that one tick — acceptable
// for an alerting side-channel, unlike a domain-of-record write.
func (r *Repository) EnqueueOutboxEvent(ctx context.Context, id, tenantID, subject string, occurredAt time.Time, version int, payload []byte) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO infra.outbox_events (id, tenant_id, subject, occurred_at, version, payload)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, id, tenantID, subject, occurredAt, version, payload)
	if err != nil {
		return fmt.Errorf("postgres: insert outbox event: %w", err)
	}
	return nil
}

// FetchUnpublished and MarkPublished implement common/outbox.Store — see
// cmd/server/main.go for where the relay is wired. Kept on the same
// Repository as EnqueueOutboxEvent since both operate on this service's own
// database, matching usage-service's Repository precedent (no domain
// reason to split them into a separate type).
func (r *Repository) FetchUnpublished(ctx context.Context, limit int) ([]outbox.Record, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, tenant_id, subject, occurred_at, version, payload
		FROM infra.outbox_events
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

func (r *Repository) MarkPublished(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := r.pool.Exec(ctx, `UPDATE infra.outbox_events SET published_at = now() WHERE id = ANY($1)`, ids)
	if err != nil {
		return fmt.Errorf("postgres: mark outbox events published: %w", err)
	}
	return nil
}
