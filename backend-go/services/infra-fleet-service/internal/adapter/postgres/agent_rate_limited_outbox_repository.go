package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stablyai/orca-go/common/outbox"
)

// AgentRateLimitedOutboxStore implements common/outbox.Store against
// infra.agent_rate_limited_outbox_events (TASK-AG-05-05's migration).
type AgentRateLimitedOutboxStore struct {
	pool *pgxpool.Pool
}

func NewAgentRateLimitedOutboxStore(pool *pgxpool.Pool) *AgentRateLimitedOutboxStore {
	return &AgentRateLimitedOutboxStore{pool: pool}
}

func (s *AgentRateLimitedOutboxStore) Enqueue(ctx context.Context, rec outbox.Record) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO infra.agent_rate_limited_outbox_events (id, tenant_id, subject, occurred_at, version, payload)
		VALUES ($1, $2, $3, $4, 1, $5)
	`, rec.ID, rec.Event.TenantID, rec.Subject, rec.Event.OccurredAt, rec.Event.Payload)
	if err != nil {
		return fmt.Errorf("postgres: insert agent rate-limited outbox event: %w", err)
	}
	return nil
}

func (s *AgentRateLimitedOutboxStore) FetchUnpublished(ctx context.Context, limit int) ([]outbox.Record, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, tenant_id, subject, occurred_at, version, payload
		FROM infra.agent_rate_limited_outbox_events
		WHERE published_at IS NULL
		ORDER BY created_at
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("postgres: query unpublished agent rate-limited outbox events: %w", err)
	}
	defer rows.Close()

	var out []outbox.Record
	for rows.Next() {
		var rec outbox.Record
		if err := rows.Scan(&rec.ID, &rec.Event.TenantID, &rec.Subject, &rec.Event.OccurredAt, &rec.Event.Version, &rec.Event.Payload); err != nil {
			return nil, fmt.Errorf("postgres: scan agent rate-limited outbox row: %w", err)
		}
		rec.Event.ID = rec.ID
		out = append(out, rec)
	}
	return out, rows.Err()
}

func (s *AgentRateLimitedOutboxStore) MarkPublished(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := s.pool.Exec(ctx, `UPDATE infra.agent_rate_limited_outbox_events SET published_at = now() WHERE id = ANY($1)`, ids)
	if err != nil {
		return fmt.Errorf("postgres: mark agent rate-limited outbox events published: %w", err)
	}
	return nil
}
