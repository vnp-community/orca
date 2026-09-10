package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

// GetByShareToken implements usecase.TaskRepository's public, unauthenticated
// lookup (TASK-TG-003-05, SECURITY REVIEW REQUIRED before merge) — the
// query deliberately has NO tenant_id filter, since the token itself is the
// only authorization this path uses. Confirmed against this codebase's real
// RLS posture rather than assuming: app.tenant_id is never SET anywhere in
// task-service today (grep across internal/ finds zero SET LOCAL/set_config
// calls), so task.tasks's RLS policy (`tenant_id =
// current_setting('app.tenant_id', true)::uuid`) is already inert in this
// deployment — every existing query in this file relies solely on its own
// explicit tenant_id filter, per this file's own header comment ("RLS is
// the secondary backstop"). This method's missing filter is therefore
// consistent with, not a deviation from, how RLS actually behaves here
// today — flagged explicitly for the security reviewer to confirm this
// holds in the real production database (a role that owns the table
// bypasses RLS by default; a non-owner role would need FORCE ROW LEVEL
// SECURITY to actually be restricted), not assumed to generalize silently.
func (r *Repository) GetByShareToken(ctx context.Context, token string) (domain.Task, error) {
	row := r.db.QueryRow(ctx, `
		SELECT `+taskColumns+`
		FROM task.tasks
		WHERE share_token = $1
	`, token)

	t, err := scanTask(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Task{}, fmt.Errorf("postgres: no task found for share token: %w", err)
		}
		return domain.Task{}, fmt.Errorf("postgres: query task by share token: %w", err)
	}
	return t, nil
}
