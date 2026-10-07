package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

var _ usecase.OpenSpecChangeRepository = (*OpenSpecChangeRepository)(nil)

type OpenSpecChangeRepository struct {
	*Repository
}

func NewOpenSpecChangeRepository(r *Repository) *OpenSpecChangeRepository {
	return &OpenSpecChangeRepository{Repository: r}
}

func (r *OpenSpecChangeRepository) GetByRequest(ctx context.Context, requestID string) (domain.OpenSpecChange, bool, error) {
	query := `
		SELECT id, request_id, status, tasks_sync_state, commit_sha
		FROM openspec_changes
		WHERE request_id = ?
	`
	var c domain.OpenSpecChange
	var status string
	var syncState string
	var commit sql.NullString
	err := r.exec(ctx).QueryRowContext(ctx, query, requestID).Scan(
		&c.ID, &c.RequestID, &status, &syncState, &commit,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return c, false, nil
		}
		return c, false, fmt.Errorf("mysql get openspec change: %w", err)
	}
	c.Status = domain.ChangeStatus(status)
	c.SyncState = domain.SyncState(syncState)
	if commit.Valid {
		c.Commit = commit.String
	}
	return c, true, nil
}

func (r *OpenSpecChangeRepository) Upsert(ctx context.Context, c domain.OpenSpecChange) (domain.OpenSpecChange, error) {
	query := `
		INSERT INTO openspec_changes (
			id, tenant_id, request_id, change_id, branch, status, tasks_sync_state,
			created_at, updated_at, version
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 1)
		ON DUPLICATE KEY UPDATE updated_at = VALUES(updated_at)
	`
	now := time.Now()
	_, err := r.exec(ctx).ExecContext(ctx, query,
		c.ID, "00000000-0000-0000-0000-000000000000", c.RequestID, "change-id", nil, string(c.Status), string(c.SyncState), now, now,
	)
	if err != nil {
		return c, fmt.Errorf("mysql upsert openspec change: %w", err)
	}
	
	// Refetch to get actual state
	fetched, _, err := r.GetByRequest(ctx, c.RequestID)
	if err != nil {
		return c, err
	}
	return fetched, nil
}

func (r *OpenSpecChangeRepository) UpdateSync(ctx context.Context, id string, state domain.SyncState, digest string, at *time.Time, expectedVersion int64) error {
	return nil
}

func (r *OpenSpecChangeRepository) ListPendingSync(ctx context.Context, limit int) ([]domain.OpenSpecChange, error) {
	return nil, nil
}
