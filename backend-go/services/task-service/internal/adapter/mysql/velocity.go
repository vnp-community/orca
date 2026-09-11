package mysql

import (
	"context"
	"fmt"

	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

// RecentCompletedTasks implements usecase.VelocityResolver — see
// internal/adapter/postgres/velocity.go's doc comment for why this is
// task-service's own data (no cross-service client adapter needed).
func (r *Repository) RecentCompletedTasks(ctx context.Context, tenantID, projectID string, n int) ([]domain.Task, error) {
	if n <= 0 {
		n = 10
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+taskColumns+`
		FROM tasks
		WHERE tenant_id = ? AND project_id = ? AND status = 'done'
		ORDER BY updated_at DESC
		LIMIT ?
	`, tenantID, projectID, n)
	if err != nil {
		return nil, fmt.Errorf("mysql: query recent completed tasks: %w", err)
	}
	defer rows.Close()

	var out []domain.Task
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, fmt.Errorf("mysql: scan recent completed task row: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
