package postgres

import (
	"context"
	"fmt"

	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

// RecentCompletedTasks implements usecase.VelocityResolver directly against
// this service's own tasks table — unlike ProjectContextResolver/
// TechStackDetector (which cross a service boundary), "which tasks in this
// project did we recently finish" is task-service's own data, so no client
// adapter is needed; Repository satisfies the port the same way it already
// satisfies TaskRepository/EdgeRepository/GrantRepository/CommentRepository.
func (r *Repository) RecentCompletedTasks(ctx context.Context, tenantID, projectID string, n int) ([]domain.Task, error) {
	if n <= 0 {
		n = 10
	}
	rows, err := r.db.Query(ctx, `
		SELECT `+taskColumns+`
		FROM task.tasks
		WHERE tenant_id = $1 AND project_id = $2 AND status = 'done'
		ORDER BY updated_at DESC
		LIMIT $3
	`, tenantID, projectID, n)
	if err != nil {
		return nil, fmt.Errorf("postgres: query recent completed tasks: %w", err)
	}
	defer rows.Close()

	var out []domain.Task
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres: scan recent completed task row: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
