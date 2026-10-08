package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/stablyai/orca-go/common/outbox"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// InsertOutboxEvent joins the caller's transaction so the event commits or rolls back with the state change.
func (r *Repository) InsertOutboxEvent(ctx context.Context, ev domain.OutboxEvent) error {
	return r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		if ev.TenantID != tenantID {
			return domain.ErrRequestTenantRequired()
		}
		_, err := db.Exec(ctx, `
			INSERT INTO request.outbox_events (id, tenant_id, subject, occurred_at, version, payload)
			VALUES ($1, $2, $3, $4, $5, $6::jsonb)
		`, ev.ID, ev.TenantID, ev.Subject, ev.OccurredAt, ev.Version, ev.Payload)
		if err != nil {
			return fmt.Errorf("postgres: insert outbox event: %w", err)
		}
		return nil
	})
}

func (r *Repository) FetchUnpublished(ctx context.Context, limit int) ([]outbox.Record, error) {
	var records []outbox.Record
	err := r.withRelayTx(ctx, func(txCtx context.Context, db dbExecer) error {
		query := `
			SELECT id, subject, tenant_id, occurred_at, version, payload
			FROM request.outbox_events 
			WHERE published_at IS NULL 
			ORDER BY created_at, seq 
			LIMIT $1
		`
		rows, err := db.Query(txCtx, query, limit)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var rec outbox.Record
			if err := rows.Scan(&rec.ID, &rec.Subject, &rec.Event.TenantID, &rec.Event.OccurredAt, &rec.Event.Version, &rec.Event.Payload); err != nil {
				return err
			}
			rec.Event.ID = rec.ID
			rec.Event.OccurredAt = rec.Event.OccurredAt.UTC()
			records = append(records, rec)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, fmt.Errorf("postgres: fetch unpublished: %w", err)
	}
	return records, nil
}

func (r *Repository) MarkPublished(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	return r.withRelayTx(ctx, func(txCtx context.Context, db dbExecer) error {
		_, err := db.Exec(txCtx, `
			UPDATE request.outbox_events 
			SET published_at = $1 
			WHERE id = ANY($2::uuid[])
		`, time.Now().UTC(), ids)
		return err
	})
}
