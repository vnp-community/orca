package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

// ShareLinkStore implements usecase.ShareLinkRepository against
// task_share_links — kept as its own type for the same method-name-collision
// reason internal/adapter/postgres/share_links.go's doc comment gives
// (Create/Revoke collide with Repository's TaskRepository.Create/
// GrantRepository.Revoke on one struct).
type ShareLinkStore struct {
	db dbtx
}

func NewShareLinkStore(pool *sql.DB) *ShareLinkStore {
	return &ShareLinkStore{db: pool}
}

// Create inserts a new task_share_links row and returns its generated id —
// generated in Go (newUUID), not read back via RETURNING (MySQL has none).
func (s *ShareLinkStore) Create(ctx context.Context, tenantID, taskID, tokenHash, createdBy string) (string, error) {
	id := newUUID()
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO task_share_links (id, tenant_id, task_id, token_hash, created_by)
		VALUES (?, ?, ?, ?, ?)
	`, id, tenantID, taskID, tokenHash, createdBy)
	if err != nil {
		return "", fmt.Errorf("mysql: insert share link: %w", err)
	}
	return id, nil
}

func (s *ShareLinkStore) ResolveActive(ctx context.Context, tenantID, tokenHash string) (string, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT task_id FROM task_share_links
		WHERE tenant_id = ? AND token_hash = ? AND revoked_at IS NULL AND (expires_at IS NULL OR expires_at > NOW(6))
	`, tenantID, tokenHash)
	var taskID string
	if err := row.Scan(&taskID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", fmt.Errorf("mysql: active share link: %w", domain.ErrShareLinkNotFound)
		}
		return "", fmt.Errorf("mysql: resolve active share link: %w", err)
	}
	return taskID, nil
}

func (s *ShareLinkStore) Revoke(ctx context.Context, tenantID, linkID string) error {
	res, err := s.db.ExecContext(ctx, `
		UPDATE task_share_links SET revoked_at = NOW(6)
		WHERE tenant_id = ? AND id = ? AND revoked_at IS NULL
	`, tenantID, linkID)
	if err != nil {
		return fmt.Errorf("mysql: revoke share link: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("mysql: revoke share link rows affected: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("mysql: share link %s not found or already revoked: %w", linkID, domain.ErrShareLinkNotFound)
	}
	return nil
}

func (s *ShareLinkStore) TaskIDFor(ctx context.Context, tenantID, linkID string) (string, error) {
	row := s.db.QueryRowContext(ctx, `SELECT task_id FROM task_share_links WHERE tenant_id = ? AND id = ?`, tenantID, linkID)
	var taskID string
	if err := row.Scan(&taskID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", fmt.Errorf("mysql: share link %s: %w", linkID, domain.ErrShareLinkNotFound)
		}
		return "", fmt.Errorf("mysql: query share link task id: %w", err)
	}
	return taskID, nil
}
