package mysql

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/stablyai/orca-go/common/outbox"
)

// EnqueueOutboxEvent implements adapter/eventbus.OutboxEnqueuer — a direct
// (non-transactional) INSERT, mirroring internal/adapter/postgres's
// identical fire-and-forget rationale (see that file's doc comment).
func (r *Repository) EnqueueOutboxEvent(ctx context.Context, id, tenantID, subject string, occurredAt time.Time, version int, payload []byte) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO outbox_events (id, tenant_id, subject, occurred_at, version, payload)
		VALUES (?, ?, ?, ?, ?, ?)
	`, id, tenantID, subject, occurredAt, version, payload)
	if err != nil {
		return fmt.Errorf("mysql: insert outbox event: %w", err)
	}
	return nil
}

// FetchUnpublished and MarkPublished implement common/outbox.Store — see
// cmd/server/main.go for where the relay is wired.
func (r *Repository) FetchUnpublished(ctx context.Context, limit int) ([]outbox.Record, error) {
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

// MarkPublished builds a dynamic IN (?,?,...) — see
// agent_rate_limited_outbox_repository.go's MarkPublished for the same
// translation and the len(ids) == 0 guard's rationale.
func (r *Repository) MarkPublished(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	q := fmt.Sprintf(`UPDATE outbox_events SET published_at = CURRENT_TIMESTAMP(6) WHERE id IN (%s)`, placeholders)
	_, err := r.db.ExecContext(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("mysql: mark outbox events published: %w", err)
	}
	return nil
}
