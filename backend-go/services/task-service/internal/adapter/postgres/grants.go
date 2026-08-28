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

// Grant inserts a task_grants row and returns its generated id — needed by
// RevokeGrant callers (TASK-TG-03-04's GrantResponse.id).
func (r *Repository) Grant(ctx context.Context, tenantID string, grant domain.Grant) (string, error) {
	level, ok := grantLevelToString[grant.Level]
	if !ok {
		return "", fmt.Errorf("postgres: unrecognized grant level %v", grant.Level)
	}
	row := r.db.QueryRow(ctx, `
		INSERT INTO task.task_grants (tenant_id, task_id, subject_id, level, apply_tree, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id
	`, tenantID, grant.TaskID, grant.SubjectID, level, grant.ApplyTree, grant.ExpiresAt)
	var id string
	if err := row.Scan(&id); err != nil {
		return "", fmt.Errorf("postgres: insert task grant: %w", err)
	}
	return id, nil
}

// Revoke deletes a task_grants row by id — a nonexistent grant_id is a
// real NOT_FOUND error, never a silent no-op.
func (r *Repository) Revoke(ctx context.Context, tenantID, grantID string) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM task.task_grants WHERE tenant_id = $1 AND id = $2`, tenantID, grantID)
	if err != nil {
		return fmt.Errorf("postgres: revoke task grant: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("postgres: grant %s not found", grantID)
	}
	return nil
}

// ListGrantsForTask returns only the grants recorded directly against
// taskID — NOT the ancestor chain, since leaking an ancestor's grant
// details to a caller without visibility into that ancestor would be a
// real information leak (see usecase.ListGrants's doc comment).
func (r *Repository) ListGrantsForTask(ctx context.Context, tenantID, taskID string) ([]domain.Grant, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, task_id, subject_id, level, apply_tree, expires_at
		FROM task.task_grants
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
		if err := rows.Scan(&g.ID, &g.TaskID, &g.SubjectID, &level, &g.ApplyTree, &g.ExpiresAt); err != nil {
			return nil, fmt.Errorf("postgres: scan grant row: %w", err)
		}
		g.Level = stringToGrantLevel[level]
		out = append(out, g)
	}
	return out, rows.Err()
}

// ListGrantsForAncestors returns every grant recorded against any of
// taskIDs, grouped by task ID — the input domain.ResolveGrant's BFS walk
// consumes. taskIDs is typically the output of GetAncestors, so this is one
// query per ResolvePermission call rather than one per ancestor hop.
// Expired rows are excluded at the SQL layer too — defense-in-depth
// alongside domain.ResolveGrant's own expiry filter (TASK-TG-03-06).
func (r *Repository) ListGrantsForAncestors(ctx context.Context, tenantID string, taskIDs []string) (map[string][]domain.Grant, error) {
	out := map[string][]domain.Grant{}
	if len(taskIDs) == 0 {
		return out, nil
	}

	rows, err := r.db.Query(ctx, `
		SELECT task_id, subject_id, level, apply_tree, expires_at
		FROM task.task_grants
		WHERE tenant_id = $1 AND task_id = ANY($2) AND (expires_at IS NULL OR expires_at > now())
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
