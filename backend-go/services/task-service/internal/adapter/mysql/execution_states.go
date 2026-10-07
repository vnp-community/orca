package mysql

import (
	"context"
	"fmt"
	"strings"
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

	placeholders := strings.Repeat("?,", len(taskIDs)-1) + "?"
	args := make([]any, 0, len(taskIDs)+1)
	args = append(args, tenantID)
	for _, id := range taskIDs {
		args = append(args, id)
	}

	q1 := fmt.Sprintf(`
		SELECT task_id, engine, status_mirror, started_at, completed_at
		FROM (
			SELECT task_id, engine, status_mirror, started_at, completed_at,
			       ROW_NUMBER() OVER (PARTITION BY task_id ORDER BY started_at DESC, id DESC) rn
			FROM execution_links
			WHERE tenant_id = ? AND task_id IN (%s)
		) x
		WHERE rn = 1
	`, placeholders)

	rows1, err := r.db.QueryContext(ctx, q1, args...)
	if err != nil {
		return nil, fmt.Errorf("mysql: list execution states last links: %w", err)
	}
	defer rows1.Close()

	for rows1.Next() {
		var taskID, engine, statusMirror string
		var startedAt time.Time
		var completedAt *time.Time
		if err := rows1.Scan(&taskID, &engine, &statusMirror, &startedAt, &completedAt); err != nil {
			return nil, fmt.Errorf("mysql: scan last link: %w", err)
		}
		if st, ok := stateMap[taskID]; ok {
			st.LastEngine = engine
			st.LastLinkStatus = statusMirror
			st.LastStartedAt = startedAt
			st.LastCompletedAt = completedAt
		}
	}
	if err := rows1.Err(); err != nil {
		return nil, fmt.Errorf("mysql: rows last link: %w", err)
	}

	q2 := fmt.Sprintf(`
		SELECT task_id, SUM(status_mirror = 'failed')
		FROM execution_links
		WHERE tenant_id = ? AND task_id IN (%s)
		GROUP BY task_id
	`, placeholders)

	rows2, err := r.db.QueryContext(ctx, q2, args...)
	if err != nil {
		return nil, fmt.Errorf("mysql: list execution states failed counts: %w", err)
	}
	defer rows2.Close()

	for rows2.Next() {
		var taskID string
		var failedCount int
		if err := rows2.Scan(&taskID, &failedCount); err != nil {
			return nil, fmt.Errorf("mysql: scan failed count: %w", err)
		}
		if st, ok := stateMap[taskID]; ok {
			st.FailedAttempts = failedCount
		}
	}
	if err := rows2.Err(); err != nil {
		return nil, fmt.Errorf("mysql: rows failed count: %w", err)
	}

	q3 := fmt.Sprintf(`
		SELECT e.from_task_id, e.to_task_id
		FROM task_edges e
		JOIN tasks t ON t.id = e.to_task_id
		WHERE e.tenant_id = ? AND e.edge_type = 'depends_on'
		  AND e.from_task_id IN (%s)
		  AND t.status NOT IN ('done','cancelled')
	`, placeholders)

	rows3, err := r.db.QueryContext(ctx, q3, args...)
	if err != nil {
		return nil, fmt.Errorf("mysql: list execution states blocked by: %w", err)
	}
	defer rows3.Close()

	for rows3.Next() {
		var fromTaskID, toTaskID string
		if err := rows3.Scan(&fromTaskID, &toTaskID); err != nil {
			return nil, fmt.Errorf("mysql: scan blocked by: %w", err)
		}
		if st, ok := stateMap[fromTaskID]; ok {
			st.BlockedByTaskIDs = append(st.BlockedByTaskIDs, toTaskID)
		}
	}
	if err := rows3.Err(); err != nil {
		return nil, fmt.Errorf("mysql: rows blocked by: %w", err)
	}

	var result []domain.ExecutionState
	for _, id := range taskIDs {
		if st, ok := stateMap[id]; ok {
			result = append(result, *st)
		}
	}

	return result, nil
}
