package mysql

import (
	"context"
	"fmt"
	"strings"

	"github.com/stablyai/orca-go/services/task-service/internal/domain"
	"github.com/stablyai/orca-go/services/task-service/internal/usecase"
)

const defaultListPageSize = 50

// inPlaceholders returns "?,?,..." for n values.
func inPlaceholders(n int) string {
	return strings.TrimSuffix(strings.Repeat("?,", n), ",")
}

// buildListQuery assembles the filtered, id-ordered page query with '?' placeholders;
// ids are CHAR(36) strings so IN lists bind as plain strings.
func buildListQuery(tenantID string, f usecase.ListFilter) (string, []any) {
	pageSize := f.PageSize
	if pageSize <= 0 {
		pageSize = defaultListPageSize
	}
	args := []any{tenantID}
	conds := []string{"tenant_id = ?"}
	if f.ProjectID != "" {
		conds = append(conds, "project_id = ?")
		args = append(args, f.ProjectID)
	}
	if len(f.TaskTypes) > 0 {
		conds = append(conds, "task_type IN ("+inPlaceholders(len(f.TaskTypes))+")")
		for _, t := range f.TaskTypes {
			args = append(args, t)
		}
	}
	if len(f.RequestIDs) > 0 {
		conds = append(conds, "request_id IN ("+inPlaceholders(len(f.RequestIDs))+")")
		for _, id := range f.RequestIDs {
			args = append(args, id)
		}
	}
	if f.ParentID != "" {
		conds = append(conds, "parent_id = ?")
		args = append(args, f.ParentID)
	}
	if f.PageToken != "" {
		conds = append(conds, "id > ?")
		args = append(args, f.PageToken)
	}
	args = append(args, pageSize)
	q := "SELECT " + taskColumns + " FROM tasks WHERE " + strings.Join(conds, " AND ") + " ORDER BY id LIMIT ?"
	return q, args
}

// List returns one id-ordered page of tasks matching f; the next token is the last id of a full page.
func (r *Repository) List(ctx context.Context, tenantID string, f usecase.ListFilter) ([]domain.Task, string, error) {
	q, args := buildListQuery(tenantID, f)
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, "", fmt.Errorf("mysql: query tasks: %w", err)
	}
	defer rows.Close()

	var out []domain.Task
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, "", fmt.Errorf("mysql: scan task row: %w", err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("mysql: iterate task rows: %w", err)
	}
	pageSize := f.PageSize
	if pageSize <= 0 {
		pageSize = defaultListPageSize
	}
	nextToken := ""
	if len(out) == int(pageSize) {
		nextToken = out[len(out)-1].ID
	}
	return out, nextToken, nil
}
