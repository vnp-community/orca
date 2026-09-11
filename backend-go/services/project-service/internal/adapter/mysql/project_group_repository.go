package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

const projectGroupColumns = `id, tenant_id, name, parent_group_id, project_id`

// ProjectGroupRepository implements usecase.ProjectGroupRepository against
// `project_groups` — mirrors postgres.ProjectGroupRepository.
type ProjectGroupRepository struct {
	db *sql.DB
}

func NewProjectGroupRepository(db *sql.DB) *ProjectGroupRepository {
	return &ProjectGroupRepository{db: db}
}

func (r *ProjectGroupRepository) CreateProjectGroup(ctx context.Context, g domain.ProjectGroup) (domain.ProjectGroup, error) {
	if _, err := r.db.ExecContext(ctx, `
		INSERT INTO project_groups (id, tenant_id, name, parent_group_id)
		VALUES (?, ?, ?, ?)
	`, g.ID, g.TenantID, g.Name, nullableString(g.ParentGroupID)); err != nil {
		return domain.ProjectGroup{}, fmt.Errorf("mysql: insert project group: %w", err)
	}
	// project_id is always empty on create (only UpsertLeafGroupForProject
	// sets it) — echoing the input directly avoids an unnecessary re-SELECT,
	// same reasoning as repository.go's Create.
	return g, nil
}

func (r *ProjectGroupRepository) GetProjectGroup(ctx context.Context, tenantID, id string) (domain.ProjectGroup, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT `+projectGroupColumns+`
		FROM project_groups
		WHERE tenant_id = ? AND id = ?
	`, tenantID, id)

	out, err := scanProjectGroup(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ProjectGroup{}, domain.ErrProjectGroupNotFound
	}
	if err != nil {
		return domain.ProjectGroup{}, fmt.Errorf("mysql: query project group: %w", err)
	}
	return out, nil
}

// UpdateProjectGroup checks existence separately rather than trusting
// RowsAffected — see repository.go's UpdateDevServerID doc comment (a
// rename to the SAME name would otherwise report RowsAffected()==0 even
// though the row exists).
func (r *ProjectGroupRepository) UpdateProjectGroup(ctx context.Context, tenantID, id, name string) (domain.ProjectGroup, error) {
	if _, err := r.GetProjectGroup(ctx, tenantID, id); err != nil {
		return domain.ProjectGroup{}, err
	}
	if _, err := r.db.ExecContext(ctx, `
		UPDATE project_groups SET name = ? WHERE tenant_id = ? AND id = ?
	`, name, tenantID, id); err != nil {
		return domain.ProjectGroup{}, fmt.Errorf("mysql: update project group: %w", err)
	}
	return r.GetProjectGroup(ctx, tenantID, id)
}

func (r *ProjectGroupRepository) DeleteProjectGroup(ctx context.Context, tenantID, id string) error {
	res, err := r.db.ExecContext(ctx, `
		DELETE FROM project_groups WHERE tenant_id = ? AND id = ?
	`, tenantID, id)
	if err != nil {
		return fmt.Errorf("mysql: delete project group: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("mysql: rows affected: %w", err)
	}
	if n == 0 {
		return domain.ErrProjectGroupNotFound
	}
	return nil
}

func (r *ProjectGroupRepository) ListProjectGroups(ctx context.Context, tenantID string) ([]domain.ProjectGroup, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+projectGroupColumns+`
		FROM project_groups
		WHERE tenant_id = ?
		ORDER BY id
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("mysql: query project groups: %w", err)
	}
	defer rows.Close()

	var out []domain.ProjectGroup
	for rows.Next() {
		g, err := scanProjectGroup(rows)
		if err != nil {
			return nil, fmt.Errorf("mysql: scan project group row: %w", err)
		}
		out = append(out, g)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("mysql: iterate project group rows: %w", err)
	}
	return out, nil
}

func scanProjectGroup(row rowScanner) (domain.ProjectGroup, error) {
	var g domain.ProjectGroup
	var parentGroupID, projectID sql.NullString
	if err := row.Scan(&g.ID, &g.TenantID, &g.Name, &parentGroupID, &projectID); err != nil {
		return domain.ProjectGroup{}, err
	}
	g.ParentGroupID = parentGroupID.String
	g.ProjectID = projectID.String
	return g, nil
}

// UpsertLeafGroupForProject finds-or-creates projectID's own leaf group —
// Postgres uses `ON CONFLICT (project_id) WHERE project_id IS NOT NULL`;
// the plain UNIQUE index migrations/mysql/0008 creates on project_id has
// the same effective scope (MySQL already only constrains non-NULL values
// in a UNIQUE index — see that migration's comment), so a plain
// `ON DUPLICATE KEY UPDATE` triggers in exactly the same cases. No
// RETURNING: re-SELECT by project_id afterward, since this method can't
// otherwise tell whether the insert or the update branch ran.
func (r *ProjectGroupRepository) UpsertLeafGroupForProject(ctx context.Context, tenantID, projectID, projectName, targetParentGroupID string) (domain.ProjectGroup, error) {
	newID := uuid.NewString()
	if _, err := r.db.ExecContext(ctx, `
		INSERT INTO project_groups (id, tenant_id, name, parent_group_id, project_id)
		VALUES (?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE parent_group_id = VALUES(parent_group_id)
	`, newID, tenantID, projectName, nullableString(targetParentGroupID), projectID); err != nil {
		return domain.ProjectGroup{}, fmt.Errorf("mysql: upsert leaf project group: %w", err)
	}

	row := r.db.QueryRowContext(ctx, `SELECT `+projectGroupColumns+` FROM project_groups WHERE project_id = ?`, projectID)
	out, err := scanProjectGroup(row)
	if err != nil {
		return domain.ProjectGroup{}, fmt.Errorf("mysql: read back upserted leaf project group: %w", err)
	}
	return out, nil
}

// ImportNested creates one ProjectGroup + one Project + one Repo per
// candidate, atomically — mirrors postgres.ProjectGroupRepository.
// ImportNested's shape and the same `url`-column-reuse decision (see that
// method's doc comment).
//
// Deviation from the Postgres source (documented, not silent): ids are
// generated in Go via uuid.NewString() rather than a DB-side
// gen_random_uuid()/UUID() call — this service already generates every
// other id in Go (see usecase/*.go's uuid.NewString() calls, confirmed via
// grep before writing this file), so this keeps id-generation in ONE place
// across both dialects rather than only 3 Postgres callsites relying on a
// DB function MySQL's UUID() wouldn't even produce in the same format.
//
// Also NOT reproduced here: the Postgres source's ImportNested has a
// pre-existing bug, found while translating this method (flagged, not
// fixed — out of this rollout's scope, same "flag don't fix" posture as
// TASK-BE-DB-009's issue-tracking-service UUID finding) —
// internal/adapter/postgres/project_group_repository.go's ImportNested
// scans its inline `INSERT ... RETURNING` (12-column projectColumns) into
// only 10 Scan destinations (missing IssueStatusSyncEnabled and
// MobileEmulatorAgentID), which pgx will error on at the column-count
// mismatch — every ImportNested call against Postgres today likely fails
// at that Scan. This MySQL version avoids the bug entirely by constructing
// the Project result from known Go values instead of a hand-rolled partial
// Scan.
func (r *ProjectGroupRepository) ImportNested(ctx context.Context, tenantID, createdBy, devServerID, parentGroupID string, candidates []domain.NestedRepoCandidate) ([]domain.ProjectGroup, []domain.Project, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("mysql: begin import nested transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	groups := make([]domain.ProjectGroup, 0, len(candidates))
	projects := make([]domain.Project, 0, len(candidates))
	now := time.Now().UTC()

	for _, c := range candidates {
		name := c.SuggestedName
		if name == "" {
			name = c.Path
		}

		p := domain.Project{
			ID: uuid.NewString(), TenantID: tenantID, Name: name, DevServerID: devServerID,
			Description: "", DefaultBranch: "main", Visibility: domain.VisibilityPrivate, CreatedBy: createdBy,
			CreatedAt: now, UpdatedAt: now, IssueStatusSyncEnabled: true,
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO projects (id, tenant_id, name, dev_server_id, description, default_branch, visibility, created_by, issue_status_sync_enabled, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, p.ID, p.TenantID, p.Name, nullableString(p.DevServerID), p.Description, p.DefaultBranch, p.Visibility, nullableString(p.CreatedBy), p.IssueStatusSyncEnabled, now, now); err != nil {
			return nil, nil, fmt.Errorf("mysql: insert imported project: %w", err)
		}

		if _, err := tx.ExecContext(ctx, `
			INSERT INTO repos (id, project_id, url, display_name, position, dev_server_id, created_at)
			VALUES (?, ?, ?, ?, 0, ?, ?)
		`, uuid.NewString(), p.ID, c.Path, name, nullableString(devServerID), now); err != nil {
			return nil, nil, fmt.Errorf("mysql: insert imported repo: %w", err)
		}

		g := domain.ProjectGroup{ID: uuid.NewString(), TenantID: tenantID, Name: name, ParentGroupID: parentGroupID, ProjectID: p.ID}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO project_groups (id, tenant_id, name, parent_group_id, project_id)
			VALUES (?, ?, ?, ?, ?)
		`, g.ID, g.TenantID, g.Name, nullableString(g.ParentGroupID), g.ProjectID); err != nil {
			return nil, nil, fmt.Errorf("mysql: insert imported project group: %w", err)
		}

		projects = append(projects, p)
		groups = append(groups, g)
	}

	if err := tx.Commit(); err != nil {
		return nil, nil, fmt.Errorf("mysql: commit import nested transaction: %w", err)
	}
	return groups, projects, nil
}
