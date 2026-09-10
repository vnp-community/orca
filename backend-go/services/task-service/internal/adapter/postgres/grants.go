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
	_, err := r.db.Exec(ctx, `
		INSERT INTO task.task_grants (tenant_id, task_id, subject_id, level, apply_tree, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, tenantID, grant.TaskID, grant.SubjectID, level, grant.ApplyTree, grant.ExpiresAt)
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

	rows, err := r.db.Query(ctx, `
		SELECT task_id, subject_id, level, apply_tree, expires_at
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
		if err := rows.Scan(&g.TaskID, &g.SubjectID, &level, &g.ApplyTree, &g.ExpiresAt); err != nil {
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

// Revoke deletes a grant by its (task_id, subject_id, level) composite key
// — see usecase.GrantRepository.Revoke's doc comment for why there's no
// surrogate grant_id to delete by instead. Idempotent by design: a DELETE
// affecting 0 rows (already revoked, or never existed) is not an error.
func (r *Repository) Revoke(ctx context.Context, tenantID, taskID, subjectID string, level domain.GrantLevel) error {
	levelStr, ok := grantLevelToString[level]
	if !ok {
		return fmt.Errorf("postgres: unrecognized grant level %v", level)
	}
	_, err := r.db.Exec(ctx, `
		DELETE FROM task.task_grants WHERE tenant_id = $1 AND task_id = $2 AND subject_id = $3 AND level = $4
	`, tenantID, taskID, subjectID, levelStr)
	if err != nil {
		return fmt.Errorf("postgres: revoke task grant: %w", err)
	}
	return nil
}

// ListByTask returns every grant recorded directly against taskID — the
// public ListGrants RPC's backing query (distinct from
// ListGrantsForAncestors's whole-chain, map-keyed shape).
func (r *Repository) ListByTask(ctx context.Context, tenantID, taskID string) ([]domain.Grant, error) {
	rows, err := r.db.Query(ctx, `
		SELECT task_id, subject_id, level, apply_tree, expires_at FROM task.task_grants
		WHERE tenant_id = $1 AND task_id = $2
	`, tenantID, taskID)
	if err != nil {
		return nil, fmt.Errorf("postgres: query grants for task: %w", err)
	}
	defer rows.Close()
	var out []domain.Grant
	for rows.Next() {
		var g domain.Grant
		var level string
		if err := rows.Scan(&g.TaskID, &g.SubjectID, &level, &g.ApplyTree, &g.ExpiresAt); err != nil {
			return nil, fmt.Errorf("postgres: scan grant row: %w", err)
		}
		g.Level = stringToGrantLevel[level]
		out = append(out, g)
	}
	return out, rows.Err()
}
