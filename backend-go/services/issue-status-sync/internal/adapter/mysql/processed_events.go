// Package mysql implements issue-status-sync's ProcessedEventStore port
// (defined in internal/usecase) against MySQL/TiDB via database/sql +
// github.com/go-sql-driver/mysql — this service's rollout of the
// multi-dialect pattern established by the usage-service pilot (CR-DB-002/
// CR-DB-003), mirroring internal/adapter/postgres's behavior 1:1 against
// the dialect-safe schema created by migrations/mysql/0001_processed_events.up.sql
// (table `processed_events`, no schema/database prefix). See
// specs/backend-go/crs/v4/multi-database/solutions/BE-DB-SOL-003.md.
package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/stablyai/orca-go/services/issue-status-sync/internal/usecase"
)

// ProcessedEventsStore implements usecase.ProcessedEventStore against
// MySQL/TiDB. No tenant_id column exists on this table in either dialect —
// the dedup cache is keyed purely on event_id — so there is no tenant-
// isolation compensating control needed here (see BE-DB-SOL-003 §4).
type ProcessedEventsStore struct {
	db *sql.DB
}

func New(db *sql.DB) *ProcessedEventsStore {
	return &ProcessedEventsStore{db: db}
}

var _ usecase.ProcessedEventStore = (*ProcessedEventsStore)(nil)

func (s *ProcessedEventsStore) Seen(ctx context.Context, eventID string) (bool, error) {
	var id string
	err := s.db.QueryRowContext(ctx, `SELECT event_id FROM processed_events WHERE event_id = ?`, eventID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("mysql: query processed events: %w", err)
	}
	return true, nil
}

func (s *ProcessedEventsStore) MarkSeen(ctx context.Context, eventID string) error {
	// INSERT IGNORE is MySQL's equivalent of Postgres's
	// ON CONFLICT (event_id) DO NOTHING here — same idempotent-mark
	// semantics, relying on the PRIMARY KEY on event_id.
	_, err := s.db.ExecContext(ctx, `INSERT IGNORE INTO processed_events (event_id) VALUES (?)`, eventID)
	if err != nil {
		return fmt.Errorf("mysql: mark event processed: %w", err)
	}
	return nil
}
