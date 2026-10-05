package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

// pgUniqueViolation is SQLSTATE 23505.
const pgUniqueViolation = "23505"

func (r *Repository) LinkSource(ctx context.Context, src domain.TaskSource) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO task.task_sources (task_id, tenant_id, project_id, provider, ref, url, site_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, src.TaskID, src.TenantID, nullableUUID(src.ProjectID), string(src.Provider), src.Ref, src.URL, src.Site)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
			return domain.ErrSourceAlreadyLinked
		}
		return fmt.Errorf("postgres: insert task source: %w", err)
	}
	return nil
}

func (r *Repository) FindTaskIDBySource(ctx context.Context, tenantID, projectID string, provider domain.SourceProvider, site, ref string) (string, bool, error) {
	var taskID string
	err := r.db.QueryRow(ctx, `
		SELECT task_id::text FROM task.task_sources
		WHERE tenant_id = $1
		  AND COALESCE(project_id, '00000000-0000-0000-0000-000000000000'::uuid) = COALESCE($2::uuid, '00000000-0000-0000-0000-000000000000'::uuid)
		  AND provider = $3 AND ref = $4 AND site_id IN ($5, '')
		ORDER BY (site_id = $5) DESC LIMIT 1
	`, tenantID, nullableUUID(projectID), string(provider), ref, site).Scan(&taskID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("postgres: find task by source: %w", err)
	}
	return taskID, true, nil
}

func (r *Repository) GetSource(ctx context.Context, tenantID, taskID string) (domain.TaskSource, bool, error) {
	src := domain.TaskSource{TenantID: tenantID, TaskID: taskID}
	var provider string
	err := r.db.QueryRow(ctx, `
		SELECT COALESCE(project_id::text, ''), provider, ref, url, site_id
		FROM task.task_sources WHERE tenant_id = $1 AND task_id = $2
	`, tenantID, taskID).Scan(&src.ProjectID, &provider, &src.Ref, &src.URL, &src.Site)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.TaskSource{}, false, nil
	}
	if err != nil {
		return domain.TaskSource{}, false, fmt.Errorf("postgres: get task source: %w", err)
	}
	src.Provider = domain.SourceProvider(provider)
	return src, true, nil
}
