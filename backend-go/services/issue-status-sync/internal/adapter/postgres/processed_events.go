// Package postgres implements usecase.ProcessedEventStore — this service's
// only database, holding the dedup cache the async consumer needs to
// tolerate JetStream's at-least-once redelivery (08-inter-service-communication.md).
package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stablyai/orca-go/services/issue-status-sync/internal/usecase"
)

// ProcessedEventsStore implements usecase.ProcessedEventStore against
// issuestatussync.processed_events.
type ProcessedEventsStore struct {
	pool *pgxpool.Pool
}

func NewProcessedEventsStore(pool *pgxpool.Pool) *ProcessedEventsStore {
	return &ProcessedEventsStore{pool: pool}
}

var _ usecase.ProcessedEventStore = (*ProcessedEventsStore)(nil)

func (s *ProcessedEventsStore) Seen(ctx context.Context, eventID string) (bool, error) {
	var id string
	err := s.pool.QueryRow(ctx, `SELECT event_id FROM issuestatussync.processed_events WHERE event_id = $1`, eventID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("postgres: query processed events: %w", err)
	}
	return true, nil
}

func (s *ProcessedEventsStore) MarkSeen(ctx context.Context, eventID string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO issuestatussync.processed_events (event_id) VALUES ($1)
		ON CONFLICT (event_id) DO NOTHING
	`, eventID)
	if err != nil {
		return fmt.Errorf("postgres: mark event processed: %w", err)
	}
	return nil
}
