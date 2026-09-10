package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stablyai/orca-go/common/outbox"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
)

// OutboxRepository implements both usecase.OutboxEnqueuer (Enqueue) and
// common/outbox.Store (FetchUnpublished/MarkPublished) against this
// service's own scm.outbox_events table — identical shape to
// issue-tracking-service's own Repository (SOL-PI-03): CreatePullRequest/
// MergePullRequest/ReceiveWebhook have no local domain-state row to share a
// transaction with, so Enqueue is a single, already-atomic INSERT.
type OutboxRepository struct {
	pool *pgxpool.Pool
}

func NewOutboxRepository(pool *pgxpool.Pool) *OutboxRepository {
	return &OutboxRepository{pool: pool}
}

func (r *OutboxRepository) Enqueue(ctx context.Context, tenantID string, event domain.OutboxEvent) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO scm.outbox_events (id, tenant_id, subject, occurred_at, version, payload)
		VALUES ($1, $2, $3, $4, 1, $5)
	`, event.ID, tenantID, event.Subject, event.OccurredAt, event.PayloadJSON)
	if err != nil {
		return fmt.Errorf("postgres: insert outbox event: %w", err)
	}
	return nil
}

func (r *OutboxRepository) FetchUnpublished(ctx context.Context, limit int) ([]outbox.Record, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, tenant_id, subject, occurred_at, version, payload
		FROM scm.outbox_events
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

func (r *OutboxRepository) MarkPublished(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := r.pool.Exec(ctx, `UPDATE scm.outbox_events SET published_at = now() WHERE id = ANY($1)`, ids)
	if err != nil {
		return fmt.Errorf("postgres: mark outbox events published: %w", err)
	}
	return nil
}
