package mysql

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/stablyai/orca-go/common/outbox"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

func (r *Repository) InsertOutboxEvent(ctx context.Context, ev domain.OutboxEvent) error {
	_, err := r.exec(ctx).ExecContext(ctx, `
		INSERT INTO outbox_events (id, tenant_id, subject, occurred_at, version, payload)
		VALUES (?, ?, ?, ?, ?, ?)
	`, ev.ID, ev.TenantID, ev.Subject, ev.OccurredAt, ev.Version, ev.Payload)
	if err != nil {
		return fmt.Errorf("mysql: insert outbox event: %w", err)
	}
	return nil
}

func (r *Repository) FetchUnpublished(ctx context.Context, limit int) ([]outbox.Record, error) {
	query := `
		SELECT id, subject, payload 
		FROM outbox_events 
		WHERE published_at IS NULL 
		ORDER BY created_at, seq 
		LIMIT ?
	`
	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("mysql: fetch unpublished: %w", err)
	}
	defer rows.Close()

	var records []outbox.Record
	for rows.Next() {
		var rec outbox.Record
		if err := rows.Scan(&rec.ID, &rec.Subject, &rec.Event.Payload); err != nil {
			return nil, fmt.Errorf("mysql: scan unpublished: %w", err)
		}
		records = append(records, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("mysql: rows unpublished: %w", err)
	}
	return records, nil
}

func (r *Repository) MarkPublished(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	placeholders := strings.Repeat("?,", len(ids)-1) + "?"
	args := make([]any, 0, len(ids)+1)
	args = append(args, time.Now().UTC())
	for _, id := range ids {
		args = append(args, id)
	}

	query := fmt.Sprintf(`
		UPDATE outbox_events 
		SET published_at = ? 
		WHERE id IN (%s)
	`, placeholders)

	_, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("mysql: mark published: %w", err)
	}
	return nil
}
