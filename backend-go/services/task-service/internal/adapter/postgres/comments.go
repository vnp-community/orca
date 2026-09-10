package postgres

import (
	"context"
	"fmt"

	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

func (r *Repository) AddComment(ctx context.Context, tenantID string, c domain.TaskComment) (domain.TaskComment, error) {
	row := r.db.QueryRow(ctx, `
		INSERT INTO task.task_comments (id, tenant_id, task_id, author_id, content)
		VALUES (gen_random_uuid(), $1, $2, $3, $4)
		RETURNING id, author_id, content, created_at
	`, tenantID, c.TaskID, c.AuthorID, c.Content)
	var out domain.TaskComment
	out.TaskID = c.TaskID
	if err := row.Scan(&out.ID, &out.AuthorID, &out.Content, &out.CreatedAt); err != nil {
		return domain.TaskComment{}, fmt.Errorf("postgres: insert task comment: %w", err)
	}
	return out, nil
}

// ListComments is a plain tenant+task-scoped SELECT, cursor-paginated
// identically to Repository.List (id-based keyset pagination).
func (r *Repository) ListComments(ctx context.Context, tenantID, taskID, pageToken string, pageSize int32) ([]domain.TaskComment, string, error) {
	if pageSize <= 0 {
		pageSize = 50
	}
	rows, err := r.db.Query(ctx, `
		SELECT id, author_id, content, created_at
		FROM task.task_comments
		WHERE tenant_id = $1 AND task_id = $2 AND ($3 = '' OR id::text > $3)
		ORDER BY created_at, id
		LIMIT $4
	`, tenantID, taskID, pageToken, pageSize)
	if err != nil {
		return nil, "", fmt.Errorf("postgres: query task comments: %w", err)
	}
	defer rows.Close()

	var out []domain.TaskComment
	for rows.Next() {
		var c domain.TaskComment
		c.TaskID = taskID
		if err := rows.Scan(&c.ID, &c.AuthorID, &c.Content, &c.CreatedAt); err != nil {
			return nil, "", fmt.Errorf("postgres: scan task comment row: %w", err)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("postgres: iterate task comment rows: %w", err)
	}
	nextToken := ""
	if len(out) == int(pageSize) {
		nextToken = out[len(out)-1].ID
	}
	return out, nextToken, nil
}
