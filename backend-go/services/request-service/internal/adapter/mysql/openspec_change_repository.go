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
	var c domain.OpenSpecChange
	found := false
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		var err error
		c, found, err = getOpenSpecChange(ctx, db, tenantID, requestID)
		return err
	})
	return c, found, err
}

// Tenant comes from the scoped context, never a constant, so one tenant cannot read or overwrite another's change.
func getOpenSpecChange(ctx context.Context, db dbExecer, tenantID, requestID string) (domain.OpenSpecChange, bool, error) {
	var c domain.OpenSpecChange
	var status, syncState string
	var commit sql.NullString
	err := db.QueryRowContext(ctx, `
		SELECT id, request_id, status, tasks_sync_state, commit_sha
		FROM openspec_changes
		WHERE tenant_id = ? AND request_id = ?`, tenantID, requestID).Scan(
		&c.ID, &c.RequestID, &status, &syncState, &commit,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return c, false, nil
	}
	if err != nil {
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
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		now := time.Now()
		if _, err := db.ExecContext(ctx, `
			INSERT INTO openspec_changes (
				id, tenant_id, request_id, change_id, branch, status, tasks_sync_state,
				created_at, updated_at, version
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 1)
			ON DUPLICATE KEY UPDATE updated_at = VALUES(updated_at)`,
			c.ID, tenantID, c.RequestID, "change-id", nil, string(c.Status), string(c.SyncState), now, now,
		); err != nil {
			return fmt.Errorf("mysql upsert openspec change: %w", err)
		}
		fetched, _, err := getOpenSpecChange(ctx, db, tenantID, c.RequestID)
		if err != nil {
			return err
		}
		c = fetched
		return nil
	})
	return c, err
}

func (r *OpenSpecChangeRepository) UpdateSync(ctx context.Context, id string, state domain.SyncState, digest string, at *time.Time, expectedVersion int64) error {
	return nil
}

func (r *OpenSpecChangeRepository) ListPendingSync(ctx context.Context, limit int) ([]domain.OpenSpecChange, error) {
	return nil, nil
}
