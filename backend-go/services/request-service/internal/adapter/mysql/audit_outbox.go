package mysql

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
			return fmt.Errorf("mysql: audit entry tenant differs from the transaction tenant")
		}
		if len(rec.MetadataJSON) > 4096 {
			return fmt.Errorf("mysql: audit metadata exceeds 4096 bytes")
		}
		_, err := db.ExecContext(ctx, `
			INSERT INTO request_audit_outbox
				(id, tenant_id, audit_id, action, actor_id, actor_type, target_type, target_id, outcome, ip_address, metadata_json)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			rec.ID, tenantID, rec.AuditID, rec.Action, rec.ActorID, rec.ActorType, rec.TargetType, rec.TargetID, rec.Outcome, rec.IPAddress, rec.MetadataJSON)
		if err != nil {
			return fmt.Errorf("mysql: enqueue audit: %w", err)
		}
		return nil
	})
}

// ProcessDue runs across tenants (the relay): SKIP LOCKED needs MySQL 8.0.1 or later.
func (r *AuditOutboxRepository) ProcessDue(ctx context.Context, now time.Time, limit int, backoff func(int) time.Duration, deliver func(context.Context, usecase.AuditOutboxRecord) error) (delivered, failed int, err error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, fmt.Errorf("mysql: begin audit relay tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	rows, err := tx.QueryContext(ctx, `
		SELECT id, tenant_id, audit_id, action, actor_id, actor_type, target_type, target_id, outcome, ip_address, metadata_json, attempts, created_at
		FROM request_audit_outbox
		WHERE delivered_at IS NULL AND next_attempt_at <= ?
		ORDER BY next_attempt_at, id
		LIMIT ?
		FOR UPDATE SKIP LOCKED`, now.UTC(), limit)
	if err != nil {
		return 0, 0, fmt.Errorf("mysql: claim audit outbox: %w", err)
	}
	var due []usecase.AuditOutboxRecord
	for rows.Next() {
		var rec usecase.AuditOutboxRecord
		if err := rows.Scan(&rec.ID, &rec.TenantID, &rec.AuditID, &rec.Action, &rec.ActorID, &rec.ActorType, &rec.TargetType,
			&rec.TargetID, &rec.Outcome, &rec.IPAddress, &rec.MetadataJSON, &rec.Attempts, &rec.CreatedAt); err != nil {
			_ = rows.Close()
			return 0, 0, fmt.Errorf("mysql: scan audit outbox: %w", err)
		}
		due = append(due, rec)
	}
	_ = rows.Close()
	if err := rows.Err(); err != nil {
		return 0, 0, fmt.Errorf("mysql: read audit outbox: %w", err)
	}
	for _, rec := range due {
		if derr := deliver(ctx, rec); derr != nil {
			failed++
			if _, err := tx.ExecContext(ctx, `UPDATE request_audit_outbox SET attempts = attempts + 1, next_attempt_at = ? WHERE id = ?`,
				now.Add(backoff(rec.Attempts+1)).UTC(), rec.ID); err != nil {
				return 0, 0, fmt.Errorf("mysql: record audit failure: %w", err)
			}
			continue
		}
		delivered++
		if _, err := tx.ExecContext(ctx, `UPDATE request_audit_outbox SET delivered_at = ? WHERE id = ?`, now.UTC(), rec.ID); err != nil {
			return 0, 0, fmt.Errorf("mysql: mark audit delivered: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, 0, fmt.Errorf("mysql: commit audit relay: %w", err)
	}
	return delivered, failed, nil
}

func (r *AuditOutboxRepository) PurgeDelivered(ctx context.Context, before time.Time, limit int) (int, error) {
	res, err := r.db.ExecContext(ctx, `DELETE FROM request_audit_outbox WHERE delivered_at IS NOT NULL AND delivered_at < ? LIMIT ?`, before.UTC(), limit)
	if err != nil {
		return 0, fmt.Errorf("mysql: purge audit outbox: %w", err)
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

func (r *AuditOutboxRepository) CountPending(ctx context.Context) (int, error) {
	n := 0
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM request_audit_outbox WHERE delivered_at IS NULL`).Scan(&n); err != nil {
		return 0, fmt.Errorf("mysql: count pending audit: %w", err)
	}
	return n, nil
}
