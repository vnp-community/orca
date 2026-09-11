package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/stablyai/orca-go/common/outbox"
)

// AgentRateLimitedOutboxStore implements common/outbox.Store against
// agent_rate_limited_outbox_events (migrations/mysql/0031).
type AgentRateLimitedOutboxStore struct {
	db *sql.DB
}

func NewAgentRateLimitedOutboxStore(db *sql.DB) *AgentRateLimitedOutboxStore {
	return &AgentRateLimitedOutboxStore{db: db}
}

func (s *AgentRateLimitedOutboxStore) Enqueue(ctx context.Context, rec outbox.Record) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO agent_rate_limited_outbox_events (id, tenant_id, subject, occurred_at, version, payload)
		VALUES (?, ?, ?, ?, 1, ?)
	`, rec.ID, rec.Event.TenantID, rec.Subject, rec.Event.OccurredAt, rec.Event.Payload)
	if err != nil {
		return fmt.Errorf("mysql: insert agent rate-limited outbox event: %w", err)
	}
	return nil
}

func (s *AgentRateLimitedOutboxStore) FetchUnpublished(ctx context.Context, limit int) ([]outbox.Record, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, tenant_id, subject, occurred_at, version, payload
		FROM agent_rate_limited_outbox_events
		WHERE published_at IS NULL
		ORDER BY created_at
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("mysql: query unpublished agent rate-limited outbox events: %w", err)
	}
	defer rows.Close()

	var out []outbox.Record
	for rows.Next() {
		var rec outbox.Record
		if err := rows.Scan(&rec.ID, &rec.Event.TenantID, &rec.Subject, &rec.Event.OccurredAt, &rec.Event.Version, &rec.Event.Payload); err != nil {
			return nil, fmt.Errorf("mysql: scan agent rate-limited outbox row: %w", err)
		}
		rec.Event.ID = rec.ID
		out = append(out, rec)
	}
	return out, rows.Err()
}

// MarkPublished builds a dynamic IN (?,?,...) — MySQL has no equivalent to
// Postgres's `id = ANY($1)` array parameter, same translation
// usage-service's MarkPublished already established. The len(ids) == 0
// guard matters here more than usual: MySQL's `IN ()` is a syntax error,
// unlike Postgres's `= ANY('{}')`.
func (s *AgentRateLimitedOutboxStore) MarkPublished(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	q := fmt.Sprintf(`UPDATE agent_rate_limited_outbox_events SET published_at = CURRENT_TIMESTAMP(6) WHERE id IN (%s)`, placeholders)
	_, err := s.db.ExecContext(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("mysql: mark agent rate-limited outbox events published: %w", err)
	}
	return nil
}
