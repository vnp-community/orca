package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

type AuditOutboxRepository struct{ *Repository }

func NewAuditOutboxRepository(r *Repository) *AuditOutboxRepository {
	return &AuditOutboxRepository{Repository: r}
}

var _ usecase.AuditOutboxStore = (*AuditOutboxRepository)(nil)

// Enqueue joins the ctx transaction, so the entry commits or rolls back with the change it records.
func (r *AuditOutboxRepository) Enqueue(ctx context.Context, rec usecase.AuditOutboxRecord) error {
	return r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		if rec.TenantID != tenantID {
			return fmt.Errorf("postgres: audit entry tenant differs from the transaction tenant")
		}
		_, err := db.Exec(ctx, `
			INSERT INTO request.request_audit_outbox
				(id, tenant_id, audit_id, action, actor_id, actor_type, target_type, target_id, outcome, ip_address, metadata_json)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
			rec.ID, tenantID, rec.AuditID, rec.Action, rec.ActorID, rec.ActorType, rec.TargetType, rec.TargetID, rec.Outcome, rec.IPAddress, rec.MetadataJSON)
		if err != nil {
			return fmt.Errorf("postgres: enqueue audit: %w", err)
		}
		return nil
	})
}

// ProcessDue holds the row locks while delivering, so a second replica skips rows being delivered.
func (r *AuditOutboxRepository) ProcessDue(ctx context.Context, now time.Time, limit int, backoff func(int) time.Duration, deliver func(context.Context, usecase.AuditOutboxRecord) error) (delivered, failed int, err error) {
	err = r.withRelayTx(ctx, func(ctx context.Context, db dbExecer) error {
		rows, err := db.Query(ctx, `
			SELECT id, tenant_id, audit_id, action, actor_id, actor_type, target_type, target_id, outcome, ip_address, metadata_json, attempts, created_at
			FROM request.request_audit_outbox
			WHERE delivered_at IS NULL AND next_attempt_at <= $1
			ORDER BY next_attempt_at, id
			LIMIT $2
			FOR UPDATE SKIP LOCKED`, now, limit)
		if err != nil {
			return fmt.Errorf("postgres: claim audit outbox: %w", err)
		}
		var due []usecase.AuditOutboxRecord
		for rows.Next() {
			var rec usecase.AuditOutboxRecord
			if err := rows.Scan(&rec.ID, &rec.TenantID, &rec.AuditID, &rec.Action, &rec.ActorID, &rec.ActorType, &rec.TargetType,
				&rec.TargetID, &rec.Outcome, &rec.IPAddress, &rec.MetadataJSON, &rec.Attempts, &rec.CreatedAt); err != nil {
				rows.Close()
				return fmt.Errorf("postgres: scan audit outbox: %w", err)
			}
			due = append(due, rec)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return fmt.Errorf("postgres: read audit outbox: %w", err)
		}
		for _, rec := range due {
			if derr := deliver(ctx, rec); derr != nil {
				failed++
				next := now.Add(backoff(rec.Attempts + 1))
				if _, err := db.Exec(ctx, `UPDATE request.request_audit_outbox SET attempts = attempts + 1, next_attempt_at = $2 WHERE id = $1`, rec.ID, next); err != nil {
					return fmt.Errorf("postgres: record audit failure: %w", err)
				}
				continue
			}
			delivered++
			if _, err := db.Exec(ctx, `UPDATE request.request_audit_outbox SET delivered_at = $2 WHERE id = $1`, rec.ID, now); err != nil {
				return fmt.Errorf("postgres: mark audit delivered: %w", err)
			}
		}
		return nil
	})
	return delivered, failed, err
}

func (r *AuditOutboxRepository) PurgeDelivered(ctx context.Context, before time.Time, limit int) (int, error) {
	n := 0
	err := r.withRelayTx(ctx, func(ctx context.Context, db dbExecer) error {
		tag, err := db.Exec(ctx, `
			DELETE FROM request.request_audit_outbox WHERE id IN (
				SELECT id FROM request.request_audit_outbox WHERE delivered_at IS NOT NULL AND delivered_at < $1 LIMIT $2)`, before, limit)
		if err != nil {
			return fmt.Errorf("postgres: purge audit outbox: %w", err)
		}
		n = int(tag.RowsAffected())
		return nil
	})
	return n, err
}

func (r *AuditOutboxRepository) CountPending(ctx context.Context) (int, error) {
	n := 0
	err := r.withRelayTx(ctx, func(ctx context.Context, db dbExecer) error {
		return db.QueryRow(ctx, `SELECT count(*) FROM request.request_audit_outbox WHERE delivered_at IS NULL`).Scan(&n)
	})
	return n, err
}
