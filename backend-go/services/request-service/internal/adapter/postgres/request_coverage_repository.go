package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

type RequestCoverageRepository struct {
	*Repository
}

func NewRequestCoverageRepository(r *Repository) *RequestCoverageRepository {
	return &RequestCoverageRepository{Repository: r}
}

var _ usecase.RequestCoverageRepository = (*RequestCoverageRepository)(nil)

// ReplaceForPlan runs in the caller's transaction (scoped joins it), so a failure after the delete takes the delete back with it.
func (r *RequestCoverageRepository) ReplaceForPlan(ctx context.Context, requestID, planTaskID string, rows []domain.CoverageRow) error {
	return r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		if _, err := db.Exec(ctx, `DELETE FROM request.request_coverage WHERE tenant_id = $1 AND request_id = $2 AND plan_task_id = $3`,
			tenantID, requestID, planTaskID); err != nil {
			return fmt.Errorf("postgres: clear request coverage: %w", err)
		}
		for _, row := range rows {
			if _, err := db.Exec(ctx, `
				INSERT INTO request.request_coverage (id, tenant_id, request_id, plan_task_id, ac_id, task_id, check_id)
				VALUES ($1, $2, $3, $4, $5, $6, $7)`,
				uuid.NewString(), tenantID, requestID, planTaskID, row.ACID, row.TaskID, nullIfEmpty(row.CheckID)); err != nil {
				return fmt.Errorf("postgres: insert request coverage: %w", err)
			}
		}
		return nil
	})
}

func (r *RequestCoverageRepository) ListByRequest(ctx context.Context, requestID string) ([]domain.CoverageRow, error) {
	var out []domain.CoverageRow
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		if _, perr := uuid.Parse(requestID); perr != nil {
			return nil
		}
		rows, err := db.Query(ctx, `SELECT ac_id, task_id, check_id, plan_task_id FROM request.request_coverage
			WHERE tenant_id = $1 AND request_id = $2 ORDER BY ac_id, task_id, check_id NULLS FIRST`, tenantID, requestID)
		if err != nil {
			return fmt.Errorf("postgres: list request coverage: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var c domain.CoverageRow
			var check *string
			if err := rows.Scan(&c.ACID, &c.TaskID, &check, &c.PlanTaskID); err != nil {
				return err
			}
			c.CheckID = derefString(check)
			out = append(out, c)
		}
		return rows.Err()
	})
	return out, err
}
