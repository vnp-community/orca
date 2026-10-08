package mysql

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

// TryStart uses ON DUPLICATE KEY UPDATE with a no-op instead of INSERT IGNORE, which would also swallow data errors.
func (r *PhaseStartRepository) TryStart(ctx context.Context, s domain.PhaseStart) (bool, error) {
	inserted := false
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		res, err := db.ExecContext(ctx, `INSERT INTO phase_starts (tenant_id, phase_task_id, request_id, started_by) VALUES (?, ?, ?, ?)
			ON DUPLICATE KEY UPDATE tenant_id = tenant_id`, tenantID, s.PhaseTaskID, s.RequestID, s.StartedBy)
		if err != nil {
			return fmt.Errorf("mysql: start phase: %w", err)
		}
		n, err := res.RowsAffected()
		inserted = n == 1
		return err
	})
	return inserted, err
}

func (r *PhaseStartRepository) ListByRequest(ctx context.Context, requestID string) ([]domain.PhaseStart, error) {
	var out []domain.PhaseStart
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		rows, err := db.QueryContext(ctx, `SELECT phase_task_id, request_id, started_by, started_at FROM phase_starts
			WHERE tenant_id = ? AND request_id = ? ORDER BY started_at, phase_task_id`, tenantID, requestID)
		if err != nil {
			return fmt.Errorf("mysql: list phase starts: %w", err)
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
