package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/stablyai/orca-go/services/issue-status-sync/internal/usecase"
)

// RequestSyncStateStore implements usecase.RequestSyncStateStore against
// MySQL/TiDB table request_sync_state.
type RequestSyncStateStore struct {
	db *sql.DB
}

func NewRequestSyncStateStore(db *sql.DB) *RequestSyncStateStore {
	return &RequestSyncStateStore{db: db}
}

var _ usecase.RequestSyncStateStore = (*RequestSyncStateStore)(nil)

// Advance locks the row inside a transaction instead of reading ROW_COUNT()
// of an upsert, whose value depends on client flags and unchanged rows.
func (s *RequestSyncStateStore) Advance(ctx context.Context, tenantID, requestID string, version int64, target string) (bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("mysql: begin advance request sync state: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var last int64
	err = tx.QueryRowContext(ctx,
		`SELECT last_version FROM request_sync_state WHERE tenant_id = ? AND request_id = ? FOR UPDATE`,
		tenantID, requestID).Scan(&last)
	applied := false
	switch {
	case errors.Is(err, sql.ErrNoRows):
		// Two first-time writers can both miss the row; the loser hits a duplicate key and retries as an update.
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO request_sync_state (tenant_id, request_id, last_version, last_target) VALUES (?, ?, ?, ?)`,
			tenantID, requestID, version, target); err != nil {
			if isDuplicateKey(err) {
				_ = tx.Rollback()
				return s.Advance(ctx, tenantID, requestID, version, target)
			}
			return false, fmt.Errorf("mysql: insert request sync state: %w", err)
		}
		applied = true
	case err != nil:
		return false, fmt.Errorf("mysql: read request sync state: %w", err)
	case last < version:
		if _, err := tx.ExecContext(ctx,
			`UPDATE request_sync_state SET last_version = ?, last_target = ?, updated_at = CURRENT_TIMESTAMP(6) WHERE tenant_id = ? AND request_id = ?`,
			version, target, tenantID, requestID); err != nil {
			return false, fmt.Errorf("mysql: update request sync state: %w", err)
		}
		applied = true
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("mysql: commit request sync state: %w", err)
	}
	return applied, nil
}
