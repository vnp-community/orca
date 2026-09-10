package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ShareLinkStore implements usecase.ShareLinkRepository against
// task.task_share_links (TASK-TG-03-05's migration) — kept as its own type
// rather than added to Repository directly: the port's method names
// (Create/Revoke) collide with Repository's existing TaskRepository.Create
// and GrantRepository.Revoke methods, and Go doesn't support per-interface
// method overloading on one struct.
type ShareLinkStore struct {
	db dbtx
}

func NewShareLinkStore(pool *pgxpool.Pool) *ShareLinkStore {
	return &ShareLinkStore{db: pool}
}

// Create inserts a new task_share_links row and returns its generated id.
// Only tokenHash (SHA-256 of the plaintext, never the plaintext itself) is
// persisted — see usecase.CreatePublicLink's doc comment.
func (s *ShareLinkStore) Create(ctx context.Context, tenantID, taskID, tokenHash, createdBy string) (string, error) {
	row := s.db.QueryRow(ctx, `
		INSERT INTO task.task_share_links (tenant_id, task_id, token_hash, created_by)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`, tenantID, taskID, tokenHash, createdBy)
	var id string
	if err := row.Scan(&id); err != nil {
		return "", fmt.Errorf("postgres: insert share link: %w", err)
	}
	return id, nil
}

// ResolveActive looks up an active (not revoked, not expired) share link by
// its token hash — the one code path in this service reachable without an
// authenticated identity.
func (s *ShareLinkStore) ResolveActive(ctx context.Context, tenantID, tokenHash string) (string, error) {
	row := s.db.QueryRow(ctx, `
		SELECT task_id FROM task.task_share_links
		WHERE tenant_id = $1 AND token_hash = $2 AND revoked_at IS NULL AND (expires_at IS NULL OR expires_at > now())
	`, tenantID, tokenHash)
	var taskID string
	if err := row.Scan(&taskID); err != nil {
		return "", fmt.Errorf("postgres: resolve active share link: %w", err)
	}
	return taskID, nil
}

// Revoke soft-deletes a share link (revoked_at set) so an audit trail
// survives, matching 07-security-architecture.md's append-only posture for
// security-relevant state changes.
func (s *ShareLinkStore) Revoke(ctx context.Context, tenantID, linkID string) error {
	tag, err := s.db.Exec(ctx, `
		UPDATE task.task_share_links SET revoked_at = now()
		WHERE tenant_id = $1 AND id = $2 AND revoked_at IS NULL
	`, tenantID, linkID)
	if err != nil {
		return fmt.Errorf("postgres: revoke share link: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("postgres: share link %s not found or already revoked", linkID)
	}
	return nil
}

// TaskIDFor returns the task_id a share link (by id) refers to — used by
// RevokePublicLink to resolve which task's 'manage' access to check before
// actually revoking.
func (s *ShareLinkStore) TaskIDFor(ctx context.Context, tenantID, linkID string) (string, error) {
	row := s.db.QueryRow(ctx, `SELECT task_id FROM task.task_share_links WHERE tenant_id = $1 AND id = $2`, tenantID, linkID)
	var taskID string
	if err := row.Scan(&taskID); err != nil {
		return "", fmt.Errorf("postgres: query share link task id: %w", err)
	}
	return taskID, nil
}
