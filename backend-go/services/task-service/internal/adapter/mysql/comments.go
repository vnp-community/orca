package mysql

import (
	"context"
	"fmt"

	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

// AddComment inserts a task_comments row. Unlike
// internal/adapter/postgres's `INSERT ... VALUES (gen_random_uuid(), ...)
// RETURNING id, ..., created_at`, MySQL has neither RETURNING nor a
// gen_random_uuid() equivalent usable inline — id is generated in Go
// (newUUID, shared with grants.go's identical need) and created_at is read
// back with a follow-up SELECT inside the same call, since the caller
// (usecase.AddComment) needs the DB-assigned timestamp in its response, not
// just the id.
func (r *Repository) AddComment(ctx context.Context, tenantID string, c domain.TaskComment) (domain.TaskComment, error) {
	id := newUUID()
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO task_comments (id, tenant_id, task_id, author_id, content)
		VALUES (?, ?, ?, ?, ?)
	`, id, tenantID, c.TaskID, c.AuthorID, c.Content)
	if err != nil {
		return domain.TaskComment{}, fmt.Errorf("mysql: insert task comment: %w", err)
	}

	row := r.db.QueryRowContext(ctx, `SELECT id, author_id, content, created_at FROM task_comments WHERE tenant_id = ? AND id = ?`, tenantID, id)
	var out domain.TaskComment
	out.TaskID = c.TaskID
	if err := row.Scan(&out.ID, &out.AuthorID, &out.Content, &out.CreatedAt); err != nil {
		return domain.TaskComment{}, fmt.Errorf("mysql: read back inserted task comment: %w", err)
	}
	return out, nil
}

func (r *Repository) ListComments(ctx context.Context, tenantID, taskID, pageToken string, pageSize int32) ([]domain.TaskComment, string, error) {
	if pageSize <= 0 {
		pageSize = 50
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, author_id, content, created_at
		FROM task_comments
		WHERE tenant_id = ? AND task_id = ? AND (? = '' OR id > ?)
		ORDER BY created_at, id
		LIMIT ?
	`, tenantID, taskID, pageToken, pageToken, pageSize)
	if err != nil {
		return nil, "", fmt.Errorf("mysql: query task comments: %w", err)
	}
	defer rows.Close()

	var out []domain.TaskComment
	for rows.Next() {
		var c domain.TaskComment
		c.TaskID = taskID
		if err := rows.Scan(&c.ID, &c.AuthorID, &c.Content, &c.CreatedAt); err != nil {
			return nil, "", fmt.Errorf("mysql: scan task comment row: %w", err)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("mysql: iterate task comment rows: %w", err)
	}
	nextToken := ""
	if len(out) == int(pageSize) {
		nextToken = out[len(out)-1].ID
	}
	return out, nextToken, nil
}
