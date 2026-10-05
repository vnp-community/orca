package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	drivermysql "github.com/go-sql-driver/mysql"

	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

// mysqlDuplicateEntry is ER_DUP_ENTRY.
const mysqlDuplicateEntry = 1062

func (r *Repository) LinkSource(ctx context.Context, src domain.TaskSource) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO task_sources (task_id, tenant_id, project_id, provider, ref, url, site_id)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, src.TaskID, src.TenantID, nullableUUID(src.ProjectID), string(src.Provider), src.Ref, src.URL, src.Site)
	if err != nil {
		var myErr *drivermysql.MySQLError
		if errors.As(err, &myErr) && myErr.Number == mysqlDuplicateEntry {
			return domain.ErrSourceAlreadyLinked
		}
		return fmt.Errorf("mysql: insert task source: %w", err)
	}
	return nil
}

func (r *Repository) FindTaskIDBySource(ctx context.Context, tenantID, projectID string, provider domain.SourceProvider, site, ref string) (string, bool, error) {
	var taskID string
	err := r.db.QueryRowContext(ctx, `
		SELECT task_id FROM task_sources
		WHERE tenant_id = ? AND project_key = ? AND provider = ? AND ref = ? AND site_id IN (?, '')
		ORDER BY (site_id = ?) DESC LIMIT 1
	`, tenantID, projectID, string(provider), ref, site, site).Scan(&taskID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("mysql: find task by source: %w", err)
	}
	return taskID, true, nil
}

func (r *Repository) GetSource(ctx context.Context, tenantID, taskID string) (domain.TaskSource, bool, error) {
	src := domain.TaskSource{TenantID: tenantID, TaskID: taskID}
	var provider string
	var projectID sql.NullString
	err := r.db.QueryRowContext(ctx, `
		SELECT project_id, provider, ref, url, site_id FROM task_sources WHERE tenant_id = ? AND task_id = ?
	`, tenantID, taskID).Scan(&projectID, &provider, &src.Ref, &src.URL, &src.Site)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.TaskSource{}, false, nil
	}
	if err != nil {
		return domain.TaskSource{}, false, fmt.Errorf("mysql: get task source: %w", err)
	}
	src.ProjectID = projectID.String
	src.Provider = domain.SourceProvider(provider)
	return src, true, nil
}
