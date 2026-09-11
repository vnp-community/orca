package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

// GetByShareToken implements usecase.TaskRepository's public,
// unauthenticated lookup — deliberately no tenant_id filter, mirroring
// internal/adapter/postgres/share_link.go's identical query and its doc
// comment on why (the token itself is the only authorization this path
// uses). MySQL has no RLS at all (dbcapability.Capabilities.SupportsRLS is
// false), so there is no backstop-vs-inert-backstop distinction to make
// here the way the Postgres file's comment makes for that dialect — this
// query's explicit lack of a tenant filter is the ENTIRE enforcement
// surface on both dialects alike.
func (r *Repository) GetByShareToken(ctx context.Context, token string) (domain.Task, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+taskColumns+` FROM tasks WHERE share_token = ?`, token)
	t, err := scanTask(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Task{}, fmt.Errorf("mysql: no task found for share token: %w", err)
		}
		return domain.Task{}, fmt.Errorf("mysql: query task by share token: %w", err)
	}
	return t, nil
}
