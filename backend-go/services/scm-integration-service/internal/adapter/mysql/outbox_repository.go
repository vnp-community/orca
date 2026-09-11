package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/stablyai/orca-go/common/outbox"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
)

// OutboxRepository implements both usecase.OutboxEnqueuer (Enqueue) and
// common/outbox.Store (FetchUnpublished/MarkPublished) against this
// service's own outbox_events table — identical shape to
// internal/adapter/postgres.OutboxRepository (see that file's doc
// comment): CreatePullRequest/MergePullRequest/ReceiveWebhook have no local
// domain-state row of their own to share a transaction with, so Enqueue is
// a single, already-atomic INSERT.
type OutboxRepository struct {
	db *sql.DB
}

func NewOutboxRepository(db *sql.DB) *OutboxRepository {
	return &OutboxRepository{db: db}
}

func (r *OutboxRepository) Enqueue(ctx context.Context, tenantID string, event domain.OutboxEvent) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO outbox_events (id, tenant_id, subject, occurred_at, version, payload)
		VALUES (?, ?, ?, ?, 1, ?)
	`, event.ID, tenantID, event.Subject, event.OccurredAt, event.PayloadJSON)
	if err != nil {
		return fmt.Errorf("mysql: insert outbox event: %w", err)
	}
	return nil
}

func (r *OutboxRepository) FetchUnpublished(ctx context.Context, limit int) ([]outbox.Record, error) {
	rows, err := r.db.QueryContext(ctx, `
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

// MarkPublished builds a dynamic IN (?,...) placeholder list —
// Postgres's `id = ANY($1)` (array bind) has no MySQL equivalent, same
// translation usage-service's own MarkPublished already established. The
// len(ids)==0 guard above the call site (usecase/common/outbox.Relay never
// calls this with an empty slice, but this method is also called directly
// by tests) avoids MySQL's `IN ()` syntax error.
func (r *OutboxRepository) MarkPublished(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	args := make([]any, 0, len(ids))
	for _, id := range ids {
		args = append(args, id)
	}
	query := fmt.Sprintf(`UPDATE outbox_events SET published_at = NOW(6) WHERE id IN (%s)`, placeholders)
	if _, err := r.db.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("mysql: mark outbox events published: %w", err)
	}
	return nil
}
