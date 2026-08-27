// Package postgres implements issue-tracking-service's outbox persistence
// — this service's first (and, deliberately, only) database, added
// purely to host the transactional-outbox table (Epic G,
// docs/execution-plan.md). LinkIssue's own domain fact — that an issue is
// linked to a task — still lives nowhere as queryable state in this
// service: Jira/Linear remain the systems of record for issue data itself
// (design doc §2/§5), and that has not changed. What's new is durability
// of the EVENT between "LinkIssue accepted the call" and "the event
// reached NATS" — see internal/usecase/link_issue.go's and
// internal/usecase/ports.go's OutboxEnqueuer doc comments for the full
// reasoning on why this is a vacuously-single-write "transaction" rather
// than the usual domain-write-plus-outbox-row pair.
package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stablyai/orca-go/common/outbox"
	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/domain"
)

// Repository implements both usecase.OutboxEnqueuer (Enqueue) and
// common/outbox.Store (FetchUnpublished/MarkPublished) against this
// service's own issuetracking.outbox_events table.
type Repository struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// Enqueue durably records event for tenantID. A single INSERT is already
// atomic on its own — there is no separate domain-state write to wrap it
// in an explicit transaction with, unlike a typical outbox producer (see
// this package's doc comment).
func (r *Repository) Enqueue(ctx context.Context, tenantID string, event domain.OutboxEvent) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO issuetracking.outbox_events (id, tenant_id, subject, occurred_at, version, payload)
		VALUES ($1, $2, $3, $4, 1, $5)
	`, event.ID, tenantID, event.Subject, event.OccurredAt, event.PayloadJSON)
	if err != nil {
		return fmt.Errorf("postgres: insert outbox event: %w", err)
	}
	return nil
}

// FetchUnpublished and MarkPublished implement common/outbox.Store — see
// cmd/server/main.go for where the relay is wired.
func (r *Repository) FetchUnpublished(ctx context.Context, limit int) ([]outbox.Record, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, tenant_id, subject, occurred_at, version, payload
		FROM issuetracking.outbox_events
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
	_, err := r.pool.Exec(ctx, `UPDATE issuetracking.outbox_events SET published_at = now() WHERE id = ANY($1)`, ids)
	if err != nil {
		return fmt.Errorf("postgres: mark outbox events published: %w", err)
	}
	return nil
}
