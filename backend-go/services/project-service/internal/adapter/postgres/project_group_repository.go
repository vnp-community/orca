package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

const projectGroupColumns = `id, tenant_id, name, COALESCE(parent_group_id::text, ''), COALESCE(project_id::text, '')`

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
	if err := row.Scan(&g.ID, &g.TenantID, &g.Name, &g.ParentGroupID, &g.ProjectID); err != nil {
		return domain.ProjectGroup{}, err
	}
	return g, nil
}

func (r *ProjectGroupRepository) UpsertLeafGroupForProject(ctx context.Context, tenantID, projectID, projectName, targetParentGroupID string) (domain.ProjectGroup, error) {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO project.project_groups (id, tenant_id, name, parent_group_id, project_id)
		VALUES (gen_random_uuid(), $1, $3, $4, $2)
		ON CONFLICT (project_id) WHERE project_id IS NOT NULL
		DO UPDATE SET parent_group_id = EXCLUDED.parent_group_id
		RETURNING `+projectGroupColumns,
		tenantID, projectID, projectName, nullableString(targetParentGroupID),
	)
	out, err := scanProjectGroup(row)
	if err != nil {
		return domain.ProjectGroup{}, fmt.Errorf("postgres: upsert leaf project group: %w", err)
	}
	return out, nil
}

// ImportNested creates one ProjectGroup + one Project + one Repo per
// candidate, atomically — see usecase.ImportNested's doc comment for why
// this is one hand-rolled multi-table transaction rather than composed
// usecase calls (mirrors RepoRepository.ReorderRepos's existing
// "one repository method owns its own multi-row transaction" convention).
// devServerID is stamped onto both the created project and its repo —
// Phase 10 gave repos their own dev-server binding, so the repo needs the
// same value the project row records, not just an inherited one.
//
// Reuses project.repos' `url` column to store the absolute on-disk path — a
// remote-clone-URL-shaped column reused for "this is already a folder on
// the dev server, not something to clone." Flagged here rather than
// silently assumed: if RepoRepository/domain.Repo later gain a distinct
// `path` field, migrate this insert to use it instead.
func (r *ProjectGroupRepository) ImportNested(ctx context.Context, tenantID, createdBy, devServerID, parentGroupID string, candidates []domain.NestedRepoCandidate) ([]domain.ProjectGroup, []domain.Project, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("postgres: begin import nested transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	groups := make([]domain.ProjectGroup, 0, len(candidates))
	projects := make([]domain.Project, 0, len(candidates))

	for _, c := range candidates {
		name := c.SuggestedName
		if name == "" {
			name = c.Path
		}

		var p domain.Project
		if err := tx.QueryRow(ctx, `
			INSERT INTO project.projects (id, tenant_id, name, dev_server_id, description, default_branch, visibility, created_by)
			VALUES (gen_random_uuid(), $1, $2, $3, '', 'main', 'private', $4)
			RETURNING `+projectColumns,
			tenantID, name, nullableString(devServerID), createdBy,
		).Scan(&p.ID, &p.TenantID, &p.Name, &p.DevServerID, &p.Description, &p.DefaultBranch, &p.Visibility, &p.CreatedBy, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, nil, fmt.Errorf("postgres: insert imported project: %w", err)
		}

		if _, err := tx.Exec(ctx, `
			INSERT INTO project.repos (id, project_id, url, display_name, position, dev_server_id)
			VALUES (gen_random_uuid(), $1, $2, $3, 0, $4)
		`, p.ID, c.Path, name, nullableString(devServerID)); err != nil {
			return nil, nil, fmt.Errorf("postgres: insert imported repo: %w", err)
		}

		var g domain.ProjectGroup
		if err := tx.QueryRow(ctx, `
			INSERT INTO project.project_groups (id, tenant_id, name, parent_group_id, project_id)
			VALUES (gen_random_uuid(), $1, $2, $3, $4)
			RETURNING `+projectGroupColumns,
			tenantID, name, nullableString(parentGroupID), p.ID,
		).Scan(&g.ID, &g.TenantID, &g.Name, &g.ParentGroupID, &g.ProjectID); err != nil {
			return nil, nil, fmt.Errorf("postgres: insert imported project group: %w", err)
		}

		projects = append(projects, p)
		groups = append(groups, g)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, nil, fmt.Errorf("postgres: commit import nested transaction: %w", err)
	}
	return groups, projects, nil
}
