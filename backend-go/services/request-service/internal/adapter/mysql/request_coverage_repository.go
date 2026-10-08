package mysql

import (
	"context"
	"database/sql"
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
		if _, err := db.ExecContext(ctx, `DELETE FROM request_coverage WHERE tenant_id = ? AND request_id = ? AND plan_task_id = ?`,
			tenantID, requestID, planTaskID); err != nil {
			return fmt.Errorf("mysql: clear request coverage: %w", err)
		}
		for _, row := range rows {
			if _, err := db.ExecContext(ctx, `
				INSERT INTO request_coverage (id, tenant_id, request_id, plan_task_id, ac_id, task_id, check_id) VALUES (?,?,?,?,?,?,?)`,
				uuid.NewString(), tenantID, requestID, planTaskID, row.ACID, row.TaskID, nullIfEmpty(row.CheckID)); err != nil {
				return fmt.Errorf("mysql: insert request coverage: %w", err)
			}
		}
		return nil
	})
}

func (r *RequestCoverageRepository) ListByRequest(ctx context.Context, requestID string) ([]domain.CoverageRow, error) {
	var out []domain.CoverageRow
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		rows, err := db.QueryContext(ctx, `SELECT ac_id, task_id, check_id, plan_task_id FROM request_coverage
			WHERE tenant_id = ? AND request_id = ? ORDER BY ac_id, task_id, check_id IS NOT NULL, check_id`, tenantID, requestID)
		if err != nil {
			return fmt.Errorf("mysql: list request coverage: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var c domain.CoverageRow
			var check sql.NullString
			if err := rows.Scan(&c.ACID, &c.TaskID, &check, &c.PlanTaskID); err != nil {
				return err
			}
			c.CheckID = check.String
			out = append(out, c)
		}
		return rows.Err()
	})
	return out, err
}
