package postgres

import (
	"context"
	"fmt"

	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

var grantLevelToString = map[domain.GrantLevel]string{
	domain.GrantLevelOwner:   "owner",
	domain.GrantLevelAdmin:   "admin",
	domain.GrantLevelUser:    "user",
	domain.GrantLevelTeam:    "team",
	domain.GrantLevelCompany: "company",
}

var stringToGrantLevel = map[string]domain.GrantLevel{
	"owner":   domain.GrantLevelOwner,
	"admin":   domain.GrantLevelAdmin,
	"user":    domain.GrantLevelUser,
	"team":    domain.GrantLevelTeam,
	"company": domain.GrantLevelCompany,
}

func (r *Repository) Grant(ctx context.Context, tenantID string, grant domain.Grant) error {
	level, ok := grantLevelToString[grant.Level]
	if !ok {
		return fmt.Errorf("postgres: unrecognized grant level %v", grant.Level)
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO task.task_grants (tenant_id, task_id, subject_id, level, apply_tree)
		VALUES ($1, $2, $3, $4, $5)
	`, tenantID, grant.TaskID, grant.SubjectID, level, grant.ApplyTree)
	if err != nil {
		return fmt.Errorf("postgres: insert task grant: %w", err)
	}
	return nil
}

// ListGrantsForAncestors returns every grant recorded against any of
// taskIDs, grouped by task ID — the input domain.ResolveGrant's BFS walk
// consumes. taskIDs is typically the output of GetAncestors, so this is one
// query per ResolvePermission call rather than one per ancestor hop.
func (r *Repository) ListGrantsForAncestors(ctx context.Context, tenantID string, taskIDs []string) (map[string][]domain.Grant, error) {
	out := map[string][]domain.Grant{}
	if len(taskIDs) == 0 {
		return out, nil
	}

	rows, err := r.pool.Query(ctx, `
		SELECT task_id, subject_id, level, apply_tree
		FROM task.task_grants
		WHERE tenant_id = $1 AND task_id = ANY($2)
	`, tenantID, taskIDs)
	if err != nil {
		return nil, fmt.Errorf("postgres: query grants for ancestors: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var g domain.Grant
		var level string
		if err := rows.Scan(&g.TaskID, &g.SubjectID, &level, &g.ApplyTree); err != nil {
			return nil, fmt.Errorf("postgres: scan grant row: %w", err)
		}
		g.Level = stringToGrantLevel[level]
		out[g.TaskID] = append(out[g.TaskID], g)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: iterate grant rows: %w", err)
	}
	return out, nil
}
