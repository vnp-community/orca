package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

func (r *Repository) ListExecutionStates(ctx context.Context, tenantID string, taskIDs []string) ([]domain.ExecutionState, error) {
	if len(taskIDs) == 0 {
		return []domain.ExecutionState{}, nil
	}

	stateMap := make(map[string]*domain.ExecutionState, len(taskIDs))
	for _, id := range taskIDs {
		stateMap[id] = &domain.ExecutionState{
			TaskID:           id,
			BlockedByTaskIDs: []string{},
		}
	}

	q1 := `
		SELECT DISTINCT ON (task_id) task_id, engine, status_mirror, started_at, completed_at
		FROM task.execution_links
		WHERE tenant_id = $1 AND task_id = ANY($2::uuid[])
		ORDER BY task_id, started_at DESC, id DESC
	`
	rows1, err := r.db.Query(ctx, q1, tenantID, taskIDs)
	if err != nil {
		return nil, fmt.Errorf("postgres: list execution states last links: %w", err)
	}
	defer rows1.Close()

	for rows1.Next() {
		var taskID, engine, statusMirror string
		var startedAt time.Time
		var completedAt *time.Time
		if err := rows1.Scan(&taskID, &engine, &statusMirror, &startedAt, &completedAt); err != nil {
			return nil, fmt.Errorf("postgres: scan last link: %w", err)
		}
		if st, ok := stateMap[taskID]; ok {
			st.LastEngine = engine
			st.LastLinkStatus = statusMirror
			st.LastStartedAt = startedAt
			st.LastCompletedAt = completedAt
		}
	}
	if err := rows1.Err(); err != nil {
		return nil, fmt.Errorf("postgres: rows last link: %w", err)
	}

	q2 := `
		SELECT task_id, count(*) FILTER (WHERE status_mirror = 'failed')
		FROM task.execution_links
		WHERE tenant_id = $1 AND task_id = ANY($2::uuid[])
		GROUP BY task_id
	`
	rows2, err := r.db.Query(ctx, q2, tenantID, taskIDs)
	if err != nil {
		return nil, fmt.Errorf("postgres: list execution states failed counts: %w", err)
	}
	defer rows2.Close()

	for rows2.Next() {
		var taskID string
		var failedCount int
		if err := rows2.Scan(&taskID, &failedCount); err != nil {
			return nil, fmt.Errorf("postgres: scan failed count: %w", err)
		}
		if st, ok := stateMap[taskID]; ok {
			st.FailedAttempts = failedCount
		}
	}
	if err := rows2.Err(); err != nil {
		return nil, fmt.Errorf("postgres: rows failed count: %w", err)
	}

	q3 := `
		SELECT e.from_task_id, e.to_task_id
		FROM task.task_edges e
		JOIN task.tasks t ON t.id = e.to_task_id
		WHERE e.tenant_id = $1 AND e.edge_type = 'depends_on'
		  AND e.from_task_id = ANY($2::uuid[])
		  AND t.status NOT IN ('done','cancelled')
	`
	rows3, err := r.db.Query(ctx, q3, tenantID, taskIDs)
	if err != nil {
		return nil, fmt.Errorf("postgres: list execution states blocked by: %w", err)
	}
	defer rows3.Close()

	for rows3.Next() {
		var fromTaskID, toTaskID string
		if err := rows3.Scan(&fromTaskID, &toTaskID); err != nil {
			return nil, fmt.Errorf("postgres: scan blocked by: %w", err)
		}
		if st, ok := stateMap[fromTaskID]; ok {
			st.BlockedByTaskIDs = append(st.BlockedByTaskIDs, toTaskID)
		}
	}
	if err := rows3.Err(); err != nil {
		return nil, fmt.Errorf("postgres: rows blocked by: %w", err)
	}

	var result []domain.ExecutionState
	for _, id := range taskIDs {
		if st, ok := stateMap[id]; ok {
			result = append(result, *st)
		}
	}

	return result, nil
}
