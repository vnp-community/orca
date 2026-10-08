package postgres

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/stablyai/orca-go/services/task-service/internal/domain"
	"github.com/stablyai/orca-go/services/task-service/internal/usecase"
)

const defaultListPageSize = 50

// buildListQuery assembles the filtered, id-ordered page query. Conditions use native
// uuid/text[] comparisons so idx_tasks_request and the primary key stay usable.
func buildListQuery(tenantID string, f usecase.ListFilter) (string, []any) {
	pageSize := f.PageSize
	if pageSize <= 0 {
		pageSize = defaultListPageSize
	}
	args := []any{tenantID}
	conds := []string{"tenant_id = $1"}
	add := func(cond string, v any) {
		args = append(args, v)
		conds = append(conds, strings.Replace(cond, "$n", "$"+strconv.Itoa(len(args)), 1))
	}
	if f.ProjectID != "" {
		add("project_id = $n::uuid", f.ProjectID)
	}
	if len(f.TaskTypes) > 0 {
		add("task_type = ANY($n::text[])", f.TaskTypes)
	}
	if len(f.RequestIDs) > 0 {
		add("request_id = ANY($n::uuid[])", f.RequestIDs)
	}
	if f.ParentID != "" {
		add("parent_id = $n::uuid", f.ParentID)
	}
	if f.PageToken != "" {
		add("id > $n::uuid", f.PageToken)
	}
	args = append(args, pageSize)
	q := "SELECT " + taskColumns + " FROM task.tasks WHERE " + strings.Join(conds, " AND ") +
		" ORDER BY id LIMIT $" + strconv.Itoa(len(args))
	return q, args
}

// List returns one id-ordered page of tasks matching f; the next token is the last id of a full page.
func (r *Repository) List(ctx context.Context, tenantID string, f usecase.ListFilter) ([]domain.Task, string, error) {
	q, args := buildListQuery(tenantID, f)
	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, "", fmt.Errorf("postgres: query tasks: %w", err)
	}
	defer rows.Close()

	var out []domain.Task
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, "", fmt.Errorf("postgres: scan task row: %w", err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("postgres: iterate task rows: %w", err)
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
