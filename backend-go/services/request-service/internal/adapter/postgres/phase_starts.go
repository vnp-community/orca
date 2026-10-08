package postgres

import (
	"context"
	"fmt"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

type PhaseStartRepository struct {
	*Repository
}

func NewPhaseStartRepository(r *Repository) *PhaseStartRepository {
	return &PhaseStartRepository{Repository: r}
}

var _ usecase.PhaseStartRepository = (*PhaseStartRepository)(nil)

func (r *PhaseStartRepository) TryStart(ctx context.Context, s domain.PhaseStart) (bool, error) {
	inserted := false
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		tag, err := db.Exec(ctx, `INSERT INTO request.phase_starts (tenant_id, phase_task_id, request_id, started_by)
			VALUES ($1, $2, $3, $4) ON CONFLICT (tenant_id, phase_task_id) DO NOTHING`, tenantID, s.PhaseTaskID, s.RequestID, s.StartedBy)
		if err != nil {
			return fmt.Errorf("postgres: start phase: %w", err)
		}
		inserted = tag.RowsAffected() == 1
		return nil
	})
	return inserted, err
}

func (r *PhaseStartRepository) ListByRequest(ctx context.Context, requestID string) ([]domain.PhaseStart, error) {
	var out []domain.PhaseStart
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		rows, err := db.Query(ctx, `SELECT phase_task_id, request_id, started_by, started_at FROM request.phase_starts
			WHERE tenant_id = $1 AND request_id = $2::uuid ORDER BY started_at, phase_task_id`, tenantID, requestID)
		if err != nil {
			return fmt.Errorf("postgres: list phase starts: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			s := domain.PhaseStart{TenantID: tenantID}
			if err := rows.Scan(&s.PhaseTaskID, &s.RequestID, &s.StartedBy, &s.StartedAt); err != nil {
				return err
			}
			s.StartedAt = s.StartedAt.UTC()
			out = append(out, s)
		}
		return rows.Err()
	})
	return out, err
}
