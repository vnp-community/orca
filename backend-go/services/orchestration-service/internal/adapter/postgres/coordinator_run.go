package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/stablyai/orca-go/services/orchestration-service/internal/domain"
	"github.com/stablyai/orca-go/services/orchestration-service/internal/usecase"
)

// ---- CoordinatorRunRepository -------------------------------------------
//
// Method names are disambiguated (CreateCoordinatorRun, not Create) since
// this same *Repository already implements OrchestrationTaskRepository's
// bare Create/Get — see CoordinatorRunRepository's own doc comment.

func (r *Repository) CreateCoordinatorRun(ctx context.Context, run domain.CoordinatorRun) (domain.CoordinatorRun, error) {
	id := run.ID
	if id == "" {
		id = uuid.NewString()
	}
	spec := run.Spec
	if spec == nil {
		spec = json.RawMessage(`{}`)
	}

	row := r.pool.QueryRow(ctx, `
		INSERT INTO orchestration.coordinator_runs (id, tenant_id, origin_task_id, spec, status, coordinator_handle, poll_interval_ms)
		VALUES ($1, $2, $3, $4, 'idle', $5, $6)
		RETURNING created_at
	`, id, run.TenantID, run.OriginTaskID, spec, run.CoordinatorHandle, run.PollIntervalMs)

	var createdAt time.Time
	if err := row.Scan(&createdAt); err != nil {
		return domain.CoordinatorRun{}, fmt.Errorf("postgres: insert coordinator run: %w", err)
	}

	run.ID = id
	run.Status = domain.RunStatusIdle
	run.Spec = spec
	run.CreatedAt = createdAt
	return run, nil
}

func (r *Repository) GetCoordinatorRun(ctx context.Context, tenantID, id string) (domain.CoordinatorRun, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, tenant_id, origin_task_id, spec, status, coordinator_handle, poll_interval_ms, created_at, completed_at
		FROM orchestration.coordinator_runs
		WHERE id = $1 AND tenant_id = $2
	`, id, tenantID)
	run, err := scanCoordinatorRun(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.CoordinatorRun{}, usecase.ErrCoordinatorRunNotFound
	}
	if err != nil {
		return domain.CoordinatorRun{}, fmt.Errorf("postgres: query coordinator run: %w", err)
	}
	return run, nil
}

// UpdateCoordinatorRunStatus writes status and (when non-nil) completedAt —
// see CoordinatorRunRepository.UpdateCoordinatorRunStatus's doc comment for
// why the caller decides completedAt explicitly rather than this method
// inferring it from status.
func (r *Repository) UpdateCoordinatorRunStatus(ctx context.Context, tenantID, id string, status domain.RunStatus, completedAt *time.Time) (domain.CoordinatorRun, error) {
	tag, err := r.pool.Exec(ctx, `
		UPDATE orchestration.coordinator_runs SET status = $1, completed_at = $2
		WHERE id = $3 AND tenant_id = $4
	`, string(status), completedAt, id, tenantID)
	if err != nil {
		return domain.CoordinatorRun{}, fmt.Errorf("postgres: update coordinator run status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.CoordinatorRun{}, usecase.ErrCoordinatorRunNotFound
	}
	return r.GetCoordinatorRun(ctx, tenantID, id)
}

// RecordHeartbeat updates heartbeat_at only — never touches status, per
// CoordinatorRunRepository's own doc comment.
func (r *Repository) RecordHeartbeat(ctx context.Context, tenantID, id string, at time.Time) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE orchestration.coordinator_runs SET heartbeat_at = $1
		WHERE id = $2 AND tenant_id = $3
	`, at, id, tenantID)
	if err != nil {
		return fmt.Errorf("postgres: record coordinator run heartbeat: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return usecase.ErrCoordinatorRunNotFound
	}
	return nil
}

// ListRunningForAdvance is a FOR UPDATE SKIP LOCKED batch fetch of
// RunStatusRunning rows — used exclusively by TASK-TG-004-03's tick loop.
// SKIP LOCKED lets multiple orchestration-service instances each grab a
// disjoint batch concurrently instead of blocking on each other's row
// locks — the whole point of a tick-loop-safe batch claim.
func (r *Repository) ListRunningForAdvance(ctx context.Context, limit int) ([]domain.CoordinatorRun, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id, tenant_id, origin_task_id, spec, status, coordinator_handle, poll_interval_ms, created_at, completed_at
		FROM orchestration.coordinator_runs
		WHERE status = 'running'
		ORDER BY id
		FOR UPDATE SKIP LOCKED
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("postgres: query running coordinator runs for advance: %w", err)
	}
	defer rows.Close()

	var out []domain.CoordinatorRun
	for rows.Next() {
		run, err := scanCoordinatorRun(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres: scan coordinator run row: %w", err)
		}
		out = append(out, run)
	}
	return out, rows.Err()
}

func scanCoordinatorRun(row interface{ Scan(dest ...any) error }) (domain.CoordinatorRun, error) {
	var run domain.CoordinatorRun
	var completedAt *time.Time
	var status string
	err := row.Scan(
		&run.ID, &run.TenantID, &run.OriginTaskID, &run.Spec, &status,
		&run.CoordinatorHandle, &run.PollIntervalMs, &run.CreatedAt, &completedAt,
	)
	if err != nil {
		return domain.CoordinatorRun{}, err
	}
	run.Status = domain.RunStatus(status)
	if completedAt != nil {
		run.CompletedAt = *completedAt
	}
	return run, nil
}
