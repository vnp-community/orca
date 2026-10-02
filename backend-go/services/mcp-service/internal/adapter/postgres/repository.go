// Package postgres implements mcp-service's repository ports against its
// own PostgreSQL database. It is the only package that knows SQL exists.
package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stablyai/orca-go/common/outbox"
	"github.com/stablyai/orca-go/services/mcp-service/internal/domain"
)

// Repository implements usecase.TenantSettingsRepository, usecase.OutboxWriter
// and common/outbox.Store. The pool's role must be neither superuser nor
// BYPASSRLS, otherwise RLS silently does nothing.
type Repository struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

const tenantSettingsColumns = `tenant_id::text, enabled, dcr_enabled, max_token_days, approval_ttl_seconds,
	kill_switch_active, kill_switch_reason, kill_switch_at, COALESCE(updated_by::text, ''), updated_at`

func scanTenantSettings(row pgx.Row) (domain.TenantSettings, error) {
	var s domain.TenantSettings
	err := row.Scan(&s.TenantID, &s.Enabled, &s.DCREnabled, &s.MaxTokenDays, &s.ApprovalTTLSeconds,
		&s.KillSwitch.Active, &s.KillSwitch.Reason, &s.KillSwitch.At, &s.UpdatedBy, &s.UpdatedAt)
	return s, err
}

func (r *Repository) GetOrCreateTenantSettings(ctx context.Context, d domain.TenantSettings) (domain.TenantSettings, error) {
	var out domain.TenantSettings
	err := r.withTenantTx(ctx, d.TenantID, func(tx pgx.Tx) error {
		// DO NOTHING keeps an existing row untouched; a concurrent first
		// insert blocks until it commits, so the SELECT below always sees it.
		if _, err := tx.Exec(ctx, `
			INSERT INTO mcp.tenant_settings (tenant_id, enabled, dcr_enabled, max_token_days, approval_ttl_seconds)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (tenant_id) DO NOTHING
		`, d.TenantID, d.Enabled, d.DCREnabled, d.MaxTokenDays, d.ApprovalTTLSeconds); err != nil {
			return fmt.Errorf("postgres: insert default tenant settings: %w", err)
		}
		s, err := scanTenantSettings(tx.QueryRow(ctx,
			`SELECT `+tenantSettingsColumns+` FROM mcp.tenant_settings WHERE tenant_id = $1`, d.TenantID))
		if err != nil {
			return fmt.Errorf("postgres: select tenant settings: %w", err)
		}
		out = s
		return nil
	})
	return out, err
}

func (r *Repository) UpdateTenantSettings(ctx context.Context, s domain.TenantSettings) (domain.TenantSettings, error) {
	var out domain.TenantSettings
	err := r.withTenantTx(ctx, s.TenantID, func(tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			INSERT INTO mcp.tenant_settings (tenant_id, enabled, dcr_enabled, max_token_days, approval_ttl_seconds, updated_by)
			VALUES ($1, $2, $3, $4, $5, NULLIF($6, '')::uuid)
			ON CONFLICT (tenant_id) DO UPDATE SET
				enabled = EXCLUDED.enabled,
				dcr_enabled = EXCLUDED.dcr_enabled,
				max_token_days = EXCLUDED.max_token_days,
				approval_ttl_seconds = EXCLUDED.approval_ttl_seconds,
				updated_by = EXCLUDED.updated_by,
				updated_at = now()
			RETURNING `+tenantSettingsColumns,
			s.TenantID, s.Enabled, s.DCREnabled, s.MaxTokenDays, s.ApprovalTTLSeconds, s.UpdatedBy)
		got, err := scanTenantSettings(row)
		if err != nil {
			return fmt.Errorf("postgres: upsert tenant settings: %w", err)
		}
		out = got
		return nil
	})
	return out, err
}

func (r *Repository) EnqueueOutbox(ctx context.Context, tenantID string, rec domain.OutboxRecord) error {
	return r.withTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO mcp.outbox_events (id, tenant_id, subject, occurred_at, version, payload)
			VALUES ($1, $2, $3, $4, $5, $6)
		`, rec.ID, tenantID, rec.Subject, rec.OccurredAt, rec.Version, rec.PayloadJSON)
		if err != nil {
			return fmt.Errorf("postgres: insert outbox event: %w", err)
		}
		return nil
	})
}

// FetchUnpublished and MarkPublished implement common/outbox.Store.
func (r *Repository) FetchUnpublished(ctx context.Context, limit int) ([]outbox.Record, error) {
	var out []outbox.Record
	err := r.withRelayTx(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id::text, tenant_id::text, subject, occurred_at, version, payload
			FROM mcp.outbox_events
			WHERE published_at IS NULL
			ORDER BY created_at
			LIMIT $1
		`, limit)
		if err != nil {
			return fmt.Errorf("postgres: query unpublished outbox events: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var rec outbox.Record
			if err := rows.Scan(&rec.ID, &rec.Event.TenantID, &rec.Subject, &rec.Event.OccurredAt, &rec.Event.Version, &rec.Event.Payload); err != nil {
				return fmt.Errorf("postgres: scan outbox event row: %w", err)
			}
			rec.Event.ID = rec.ID
			out = append(out, rec)
		}
		return rows.Err()
	})
	return out, err
}

func (r *Repository) MarkPublished(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	return r.withRelayTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `UPDATE mcp.outbox_events SET published_at = now() WHERE id = ANY($1::uuid[])`, ids); err != nil {
			return fmt.Errorf("postgres: mark outbox events published: %w", err)
		}
		return nil
	})
}
