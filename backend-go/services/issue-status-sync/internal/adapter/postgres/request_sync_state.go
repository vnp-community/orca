package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stablyai/orca-go/services/issue-status-sync/internal/usecase"
)

// RequestSyncStateStore implements usecase.RequestSyncStateStore against
// issuestatussync.request_sync_state.
type RequestSyncStateStore struct {
	pool *pgxpool.Pool
}

func NewRequestSyncStateStore(pool *pgxpool.Pool) *RequestSyncStateStore {
	return &RequestSyncStateStore{pool: pool}
}

var _ usecase.RequestSyncStateStore = (*RequestSyncStateStore)(nil)

// Advance is one atomic upsert: the WHERE makes a stale or equal version a
// no-op that returns no row, so concurrent callers cannot both win.
func (s *RequestSyncStateStore) Advance(ctx context.Context, tenantID, requestID string, version int64, target string) (bool, error) {
	rows, err := s.pool.Query(ctx, `
		INSERT INTO issuestatussync.request_sync_state (tenant_id, request_id, last_version, last_target)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (tenant_id, request_id) DO UPDATE
		   SET last_version = EXCLUDED.last_version, last_target = EXCLUDED.last_target, updated_at = now()
		 WHERE issuestatussync.request_sync_state.last_version < EXCLUDED.last_version
		RETURNING 1`, tenantID, requestID, version, target)
	if err != nil {
		return false, fmt.Errorf("postgres: advance request sync state: %w", err)
	}
	defer rows.Close()
	applied := rows.Next()
	if err := rows.Err(); err != nil {
		return false, fmt.Errorf("postgres: advance request sync state: %w", err)
	}
	return applied, nil
}
