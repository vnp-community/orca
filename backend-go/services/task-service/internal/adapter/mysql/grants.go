package mysql

import (
	"context"
	"fmt"
	"strings"

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

// Grant inserts a task_grants row and returns its generated id. Unlike
// internal/adapter/postgres's INSERT ... RETURNING id, MySQL has no
// RETURNING — id is generated here in Go (uuid.NewString would be the
// obvious choice, but task_grants.id has no application code reading it
// back except THIS call's own return value, so a plain UUID generated
// inline keeps this method self-contained without a new domain-layer
// dependency).
func (r *Repository) Grant(ctx context.Context, tenantID string, grant domain.Grant) (string, error) {
	level, ok := grantLevelToString[grant.Level]
	if !ok {
		return "", fmt.Errorf("mysql: unrecognized grant level %v", grant.Level)
	}
	id := newUUID()
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO task_grants (id, tenant_id, task_id, subject_id, level, apply_tree, expires_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, id, tenantID, grant.TaskID, grant.SubjectID, level, grant.ApplyTree, grant.ExpiresAt)
	if err != nil {
		return "", fmt.Errorf("mysql: insert task grant: %w", err)
	}
	return id, nil
}

func (r *Repository) Revoke(ctx context.Context, tenantID, taskID, subjectID string, level domain.GrantLevel) error {
	levelStr, ok := grantLevelToString[level]
	if !ok {
		return fmt.Errorf("mysql: unrecognized grant level %v", level)
	}
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM task_grants WHERE tenant_id = ? AND task_id = ? AND subject_id = ? AND level = ?
	`, tenantID, taskID, subjectID, levelStr)
	if err != nil {
		return fmt.Errorf("mysql: revoke task grant: %w", err)
	}
	return nil
}

func (r *Repository) ListGrantsForTask(ctx context.Context, tenantID, taskID string) ([]domain.Grant, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, task_id, subject_id, level, apply_tree, expires_at
		FROM task_grants
		WHERE tenant_id = ? AND task_id = ?
	`, tenantID, taskID)
	if err != nil {
		return nil, fmt.Errorf("mysql: query grants for task: %w", err)
	}
	defer rows.Close()

	var out []domain.Grant
	for rows.Next() {
		g, err := scanGrant(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// ListGrantsForAncestors returns every grant recorded against any of
// taskIDs, grouped by task ID. Unlike Postgres's `task_id = ANY(?)`, MySQL
// needs a dynamically-built `IN (?,...)` — same translation this service's
// MarkPublished-shaped methods elsewhere in the rollout need (see
// BE-DB-SOL-002 §3's `id = ANY($1)` note); guarded for len(taskIDs) == 0
// the same way (MySQL's `IN ()` is a syntax error, unlike Postgres's
// `= ANY('{}')`).
func (r *Repository) ListGrantsForAncestors(ctx context.Context, tenantID string, taskIDs []string) (map[string][]domain.Grant, error) {
	out := map[string][]domain.Grant{}
	if len(taskIDs) == 0 {
		return out, nil
	}

	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(taskIDs)), ",")
	args := make([]any, 0, len(taskIDs)+1)
	args = append(args, tenantID)
	for _, id := range taskIDs {
		args = append(args, id)
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT task_id, id, subject_id, level, apply_tree, expires_at
		FROM task_grants
		WHERE tenant_id = ? AND task_id IN (`+placeholders+`) AND (expires_at IS NULL OR expires_at > NOW(6))
	`, args...)
	if err != nil {
		return nil, fmt.Errorf("mysql: query grants for ancestors: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var g domain.Grant
		var level string
		if err := rows.Scan(&g.TaskID, &g.ID, &g.SubjectID, &level, &g.ApplyTree, &g.ExpiresAt); err != nil {
			return nil, fmt.Errorf("mysql: scan grant row: %w", err)
		}
		g.Level = stringToGrantLevel[level]
		out[g.TaskID] = append(out[g.TaskID], g)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("mysql: iterate grant rows: %w", err)
	}
	return out, nil
}

func scanGrant(rows interface{ Scan(dest ...any) error }) (domain.Grant, error) {
	var g domain.Grant
	var level string
	if err := rows.Scan(&g.ID, &g.TaskID, &g.SubjectID, &level, &g.ApplyTree, &g.ExpiresAt); err != nil {
		return domain.Grant{}, fmt.Errorf("mysql: scan grant row: %w", err)
	}
	g.Level = stringToGrantLevel[level]
	return g, nil
}
