package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// TerminalScrollbackSnapshotStore implements usecase.TerminalScrollbackSnapshotRepository
// against infra.terminal_scrollback_snapshots (migrations/0007).
type TerminalScrollbackSnapshotStore struct {
	pool *pgxpool.Pool
}

func NewTerminalScrollbackSnapshotStore(pool *pgxpool.Pool) *TerminalScrollbackSnapshotStore {
	return &TerminalScrollbackSnapshotStore{pool: pool}
}

func (s *TerminalScrollbackSnapshotStore) Upsert(ctx context.Context, snap domain.TerminalScrollbackSnapshot) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO infra.terminal_scrollback_snapshots
			(tenant_id, worktree_id, pane_key, cols, rows, data_gzip, uncompressed_bytes, last_title, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (tenant_id, worktree_id, pane_key) DO UPDATE SET
			cols = EXCLUDED.cols, rows = EXCLUDED.rows, data_gzip = EXCLUDED.data_gzip,
			uncompressed_bytes = EXCLUDED.uncompressed_bytes, last_title = EXCLUDED.last_title,
			updated_at = EXCLUDED.updated_at
	`, snap.TenantID, snap.WorktreeID, snap.PaneKey, snap.Cols, snap.Rows,
		snap.DataGzip, snap.UncompressedBytes, snap.LastTitle, snap.UpdatedAt)
	if err != nil {
		return fmt.Errorf("postgres: upsert terminal scrollback snapshot: %w", err)
	}
	return nil
}

func (s *TerminalScrollbackSnapshotStore) Get(ctx context.Context, tenantID, worktreeID, paneKey string) (bool, domain.TerminalScrollbackSnapshot, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT tenant_id, worktree_id, pane_key, cols, rows, data_gzip, uncompressed_bytes, last_title, updated_at
		FROM infra.terminal_scrollback_snapshots
		WHERE tenant_id = $1 AND worktree_id = $2 AND pane_key = $3
	`, tenantID, worktreeID, paneKey)

	var snap domain.TerminalScrollbackSnapshot
	err := row.Scan(&snap.TenantID, &snap.WorktreeID, &snap.PaneKey, &snap.Cols, &snap.Rows,
		&snap.DataGzip, &snap.UncompressedBytes, &snap.LastTitle, &snap.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, domain.TerminalScrollbackSnapshot{}, nil
	}
	if err != nil {
		return false, domain.TerminalScrollbackSnapshot{}, fmt.Errorf("postgres: query terminal scrollback snapshot: %w", err)
	}
	return true, snap, nil
}

// SumUncompressedBytes excludes excludePaneKey (the row Upsert is about to
// replace) so two saves to the same pane never double-count toward BR-TM-10's cap.
func (s *TerminalScrollbackSnapshotStore) SumUncompressedBytes(ctx context.Context, tenantID, worktreeID, excludePaneKey string) (int64, error) {
	var total int64
	err := s.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(uncompressed_bytes), 0) FROM infra.terminal_scrollback_snapshots
		WHERE tenant_id = $1 AND worktree_id = $2 AND pane_key != $3
	`, tenantID, worktreeID, excludePaneKey).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("postgres: sum terminal scrollback snapshot bytes: %w", err)
	}
	return total, nil
}

func (s *TerminalScrollbackSnapshotStore) DeleteByWorktree(ctx context.Context, tenantID, worktreeID string) error {
	_, err := s.pool.Exec(ctx, `
		DELETE FROM infra.terminal_scrollback_snapshots WHERE tenant_id = $1 AND worktree_id = $2
	`, tenantID, worktreeID)
	if err != nil {
		return fmt.Errorf("postgres: delete terminal scrollback snapshots by worktree: %w", err)
	}
	return nil
}

func (s *TerminalScrollbackSnapshotStore) DeleteExpired(ctx context.Context, olderThan time.Time) (int, error) {
	tag, err := s.pool.Exec(ctx, `
		DELETE FROM infra.terminal_scrollback_snapshots WHERE updated_at < $1
	`, olderThan)
	if err != nil {
		return 0, fmt.Errorf("postgres: delete expired terminal scrollback snapshots: %w", err)
	}
	return int(tag.RowsAffected()), nil
}
