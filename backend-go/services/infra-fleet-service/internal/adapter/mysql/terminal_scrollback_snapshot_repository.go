package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// TerminalScrollbackSnapshotStore implements
// usecase.TerminalScrollbackSnapshotRepository against
// terminal_scrollback_snapshots (migrations/mysql/0022). `rows` is
// backtick-quoted throughout — MySQL 8.0.19+ reserves it as a keyword (see
// that migration's comment).
type TerminalScrollbackSnapshotStore struct {
	db *sql.DB
}

func NewTerminalScrollbackSnapshotStore(db *sql.DB) *TerminalScrollbackSnapshotStore {
	return &TerminalScrollbackSnapshotStore{db: db}
}

func (s *TerminalScrollbackSnapshotStore) Upsert(ctx context.Context, snap domain.TerminalScrollbackSnapshot) error {
	// id has no MySQL-side DEFAULT (unlike Postgres's gen_random_uuid()) and
	// domain.TerminalScrollbackSnapshot carries no ID field for a caller to
	// set — generated fresh on every call; ON DUPLICATE KEY UPDATE below
	// never touches id, so an existing row keeps its original one.
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO terminal_scrollback_snapshots
			(id, tenant_id, worktree_id, pane_key, cols, `+"`rows`"+`, data_gzip, uncompressed_bytes, last_title, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			cols = VALUES(cols), `+"`rows`"+` = VALUES(`+"`rows`"+`), data_gzip = VALUES(data_gzip),
			uncompressed_bytes = VALUES(uncompressed_bytes), last_title = VALUES(last_title),
			updated_at = VALUES(updated_at)
	`, uuid.NewString(), snap.TenantID, snap.WorktreeID, snap.PaneKey, snap.Cols, snap.Rows,
		snap.DataGzip, snap.UncompressedBytes, snap.LastTitle, snap.UpdatedAt)
	if err != nil {
		return fmt.Errorf("mysql: upsert terminal scrollback snapshot: %w", err)
	}
	return nil
}

func (s *TerminalScrollbackSnapshotStore) Get(ctx context.Context, tenantID, worktreeID, paneKey string) (bool, domain.TerminalScrollbackSnapshot, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT tenant_id, worktree_id, pane_key, cols, `+"`rows`"+`, data_gzip, uncompressed_bytes, last_title, updated_at
		FROM terminal_scrollback_snapshots
		WHERE tenant_id = ? AND worktree_id = ? AND pane_key = ?
	`, tenantID, worktreeID, paneKey)

	var snap domain.TerminalScrollbackSnapshot
	err := row.Scan(&snap.TenantID, &snap.WorktreeID, &snap.PaneKey, &snap.Cols, &snap.Rows,
		&snap.DataGzip, &snap.UncompressedBytes, &snap.LastTitle, &snap.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return false, domain.TerminalScrollbackSnapshot{}, nil
	}
	if err != nil {
		return false, domain.TerminalScrollbackSnapshot{}, fmt.Errorf("mysql: query terminal scrollback snapshot: %w", err)
	}
	return true, snap, nil
}

// SumUncompressedBytes excludes excludePaneKey (the row Upsert is about to
// replace) so two saves to the same pane never double-count toward BR-TM-10's cap.
func (s *TerminalScrollbackSnapshotStore) SumUncompressedBytes(ctx context.Context, tenantID, worktreeID, excludePaneKey string) (int64, error) {
	var total int64
	err := s.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(uncompressed_bytes), 0) FROM terminal_scrollback_snapshots
		WHERE tenant_id = ? AND worktree_id = ? AND pane_key != ?
	`, tenantID, worktreeID, excludePaneKey).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("mysql: sum terminal scrollback snapshot bytes: %w", err)
	}
	return total, nil
}

func (s *TerminalScrollbackSnapshotStore) DeleteByWorktree(ctx context.Context, tenantID, worktreeID string) error {
	_, err := s.db.ExecContext(ctx, `
		DELETE FROM terminal_scrollback_snapshots WHERE tenant_id = ? AND worktree_id = ?
	`, tenantID, worktreeID)
	if err != nil {
		return fmt.Errorf("mysql: delete terminal scrollback snapshots by worktree: %w", err)
	}
	return nil
}

func (s *TerminalScrollbackSnapshotStore) DeleteExpired(ctx context.Context, olderThan time.Time) (int, error) {
	res, err := s.db.ExecContext(ctx, `
		DELETE FROM terminal_scrollback_snapshots WHERE updated_at < ?
	`, olderThan)
	if err != nil {
		return 0, fmt.Errorf("mysql: delete expired terminal scrollback snapshots: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("mysql: rows affected: %w", err)
	}
	return int(n), nil
}
