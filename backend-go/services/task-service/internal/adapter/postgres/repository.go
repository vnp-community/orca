// Package postgres implements task-service's TaskRepository, EdgeRepository,
// and GrantRepository ports (defined in internal/usecase) against this
// service's own PostgreSQL database — see
// specs/backend-go/architecture/05-data-architecture.md's
// database-per-service rule: this is the ONLY package in task-service that
// knows SQL exists.
//
// Hand-written SQL via pgx, not sqlc codegen — task-service.md §6 names
// sqlc (with hand-written recursive CTEs) as the chosen approach for this
// service specifically, but this scaffold hand-writes the equivalent
// queries directly to avoid an extra build-time toolchain dependency, same
// posture as usage-service's adapter/postgres. Add a sqlc.yaml + regenerate
// once the query set stabilizes — see this service's README.
package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

// Repository implements usecase.TaskRepository, usecase.EdgeRepository, and
// usecase.GrantRepository against Postgres via pgx — one struct, one pool,
// same shape as usage-service/internal/adapter/postgres.Repository.
type Repository struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Create(ctx context.Context, task domain.Task) (domain.Task, error) {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO task.tasks (id, tenant_id, title, status, parent_id)
		VALUES ($1, $2, $3, $4, $5)
	`, task.ID, task.TenantID, task.Title, task.Status, nullableUUID(task.ParentID))
	if err != nil {
		return domain.Task{}, fmt.Errorf("postgres: insert task: %w", err)
	}
	return task, nil
}

func (r *Repository) Get(ctx context.Context, tenantID, id string) (domain.Task, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, tenant_id, title, status, COALESCE(parent_id::text, '')
		FROM task.tasks
		WHERE tenant_id = $1 AND id = $2
	`, tenantID, id)

	var t domain.Task
	if err := row.Scan(&t.ID, &t.TenantID, &t.Title, &t.Status, &t.ParentID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Task{}, fmt.Errorf("postgres: task %s not found: %w", id, err)
		}
		return domain.Task{}, fmt.Errorf("postgres: query task: %w", err)
	}
	return t, nil
}

// GetAncestors walks tasks.parent_id from id up to the root via one
// WITH RECURSIVE query, per task-service.md §6's query-shape note and §8's
// max-depth guard (bounded by the depth column in the recursive term, not
// re-queried per hop). The first row is id itself; the last is the root.
func (r *Repository) GetAncestors(ctx context.Context, tenantID, id string, maxDepth int) ([]domain.Task, error) {
	if maxDepth <= 0 {
		maxDepth = domain.DefaultMaxAncestorDepth
	}

	rows, err := r.pool.Query(ctx, `
		WITH RECURSIVE ancestors AS (
			SELECT id, tenant_id, title, status, parent_id, 0 AS depth
			FROM task.tasks
			WHERE tenant_id = $1 AND id = $2

			UNION ALL

			SELECT t.id, t.tenant_id, t.title, t.status, t.parent_id, a.depth + 1
			FROM task.tasks t
			JOIN ancestors a ON t.id = a.parent_id
			WHERE a.depth + 1 < $3
		)
		SELECT id, tenant_id, title, status, COALESCE(parent_id::text, '')
		FROM ancestors
		ORDER BY depth
	`, tenantID, id, maxDepth)
	if err != nil {
		return nil, fmt.Errorf("postgres: query ancestors: %w", err)
	}
	defer rows.Close()

	var out []domain.Task
	for rows.Next() {
		var t domain.Task
		if err := rows.Scan(&t.ID, &t.TenantID, &t.Title, &t.Status, &t.ParentID); err != nil {
			return nil, fmt.Errorf("postgres: scan ancestor row: %w", err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: iterate ancestor rows: %w", err)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("postgres: task %s not found while resolving ancestors", id)
	}
	return out, nil
}

func nullableUUID(id string) any {
	if id == "" {
		return nil
	}
	return id
}
