// Package mysql implements issue-tracking-service's persistence ports
// (usecase.OutboxEnqueuer + common/outbox.Store, usecase.ConnectionRepository)
// against MySQL/TiDB via database/sql + github.com/go-sql-driver/mysql —
// this service's multi-dialect rollout adapter for CR-DB-002/CR-DB-003,
// mirroring internal/adapter/postgres's behavior 1:1 against the
// dialect-safe schema in migrations/mysql/{0001_outbox,0002_connections}.up.sql
// (tables `outbox_events`, `connections`, no schema/database prefix — same
// naming decision as usage-service's pilot adapter, see
// specs/backend-go/crs/v4/multi-database/solutions/BE-DB-SOL-004.md).
package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/stablyai/orca-go/common/outbox"
	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/domain"
)

// Repository implements usecase.OutboxEnqueuer (Enqueue), common/outbox.Store
// (FetchUnpublished/MarkPublished), and usecase.ConnectionRepository (see
// connections.go) against MySQL/TiDB. No RLS equivalent exists in MySQL —
// every query below filters by tenant_id explicitly, which is the ONLY
// tenant-isolation enforcement for this adapter (see BE-DB-SOL-001 §4,
// TASK-BE-DB-003's finding for the pilot: this was already true for the
// Postgres adapter too, since RLS never actually activated there — this
// doesn't lower the bar, it just doesn't add a backstop that was never
// real).
type Repository struct {
	db *sql.DB
}

func New(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// Enqueue durably records event for tenantID. A single INSERT is already
// atomic on its own — see internal/adapter/postgres's package doc comment
// for why this stays a vacuously-single-write "transaction" for this
// service specifically (no other domain state to be atomic with).
func (r *Repository) Enqueue(ctx context.Context, tenantID string, event domain.OutboxEvent) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO outbox_events (id, tenant_id, subject, occurred_at, version, payload)
		VALUES (?, ?, ?, ?, 1, ?)
	`, event.ID, tenantID, event.Subject, event.OccurredAt, event.PayloadJSON)
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

func (r *Repository) MarkPublished(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	args := make([]any, 0, len(ids))
	for _, id := range ids {
		args = append(args, id)
	}
	query := fmt.Sprintf(`UPDATE outbox_events SET published_at = NOW(6) WHERE id IN (%s)`, placeholders)
	_, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("mysql: mark outbox events published: %w", err)
	}
	return nil
}
