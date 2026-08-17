package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

const projectGroupColumns = `id, tenant_id, name, COALESCE(parent_group_id::text, '')`

// ProjectGroupRepository implements usecase.ProjectGroupRepository against
// project.project_groups.
type ProjectGroupRepository struct {
	pool *pgxpool.Pool
}

func NewProjectGroupRepository(pool *pgxpool.Pool) *ProjectGroupRepository {
	return &ProjectGroupRepository{pool: pool}
}

func (r *ProjectGroupRepository) CreateProjectGroup(ctx context.Context, g domain.ProjectGroup) (domain.ProjectGroup, error) {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO project.project_groups (id, tenant_id, name, parent_group_id)
		VALUES ($1, $2, $3, $4)
		RETURNING `+projectGroupColumns,
		g.ID, g.TenantID, g.Name, nullableString(g.ParentGroupID),
	)

	out, err := scanProjectGroup(row)
	if err != nil {
		return domain.ProjectGroup{}, fmt.Errorf("postgres: insert project group: %w", err)
	}
	return out, nil
}

func (r *ProjectGroupRepository) GetProjectGroup(ctx context.Context, tenantID, id string) (domain.ProjectGroup, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT `+projectGroupColumns+`
		FROM project.project_groups
		WHERE tenant_id = $1 AND id = $2
	`, tenantID, id)

	out, err := scanProjectGroup(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ProjectGroup{}, domain.ErrProjectGroupNotFound
	}
	if err != nil {
		return domain.ProjectGroup{}, fmt.Errorf("postgres: query project group: %w", err)
	}
	return out, nil
}

func (r *ProjectGroupRepository) UpdateProjectGroup(ctx context.Context, tenantID, id, name string) (domain.ProjectGroup, error) {
	row := r.pool.QueryRow(ctx, `
		UPDATE project.project_groups SET name = $3
		WHERE tenant_id = $1 AND id = $2
		RETURNING `+projectGroupColumns,
		tenantID, id, name,
	)

	out, err := scanProjectGroup(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ProjectGroup{}, domain.ErrProjectGroupNotFound
	}
	if err != nil {
		return domain.ProjectGroup{}, fmt.Errorf("postgres: update project group: %w", err)
	}
	return out, nil
}

func (r *ProjectGroupRepository) DeleteProjectGroup(ctx context.Context, tenantID, id string) error {
	tag, err := r.pool.Exec(ctx, `
		DELETE FROM project.project_groups WHERE tenant_id = $1 AND id = $2
	`, tenantID, id)
	if err != nil {
		return fmt.Errorf("postgres: delete project group: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrProjectGroupNotFound
	}
	return nil
}

func (r *ProjectGroupRepository) ListProjectGroups(ctx context.Context, tenantID string) ([]domain.ProjectGroup, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+projectGroupColumns+`
		FROM project.project_groups
		WHERE tenant_id = $1
		ORDER BY id
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("postgres: query project groups: %w", err)
	}
	defer rows.Close()

	var out []domain.ProjectGroup
	for rows.Next() {
		g, err := scanProjectGroup(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres: scan project group row: %w", err)
		}
		out = append(out, g)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: iterate project group rows: %w", err)
	}
	return out, nil
}

// scanProjectGroup does NOT convert pgx.ErrNoRows itself — every caller
// already checks errors.Is(err, pgx.ErrNoRows) against the raw scan error
// (matching Repository.scanProject's pattern in repository.go), so
// converting here too would double-wrap and never actually match that
// check.
func scanProjectGroup(row rowScanner) (domain.ProjectGroup, error) {
	var g domain.ProjectGroup
	if err := row.Scan(&g.ID, &g.TenantID, &g.Name, &g.ParentGroupID); err != nil {
		return domain.ProjectGroup{}, err
	}
	return g, nil
}
