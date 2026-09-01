// Package postgres implements project-service's ProjectRepository/
// RepoRepository/WorktreeRepository/ProjectGroupRepository ports (defined in
// internal/usecase) against this service's own PostgreSQL database — see
// specs/backend-go/architecture/05-data-architecture.md's
// database-per-service rule: this is the ONLY package in project-service
// that knows SQL exists.
package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

// projectColumns is the column list shared by every SELECT/RETURNING
// against project.projects — kept as one constant so Create/Get/List/
// UpdateDevServerID/UpdateProject/scanProject can't drift out of sync with
// each other. Both nullable UUID columns are cast to ::text before
// COALESCE — COALESCE(uuid_col, ”) fails at parse time (Postgres unifies
// the branch types to uuid and tries to parse ” as one), a latent bug this
// change also fixes on dev_server_id, not just the new created_by column.
const projectColumns = `id, tenant_id, name, COALESCE(dev_server_id::text, ''), description, default_branch, visibility, COALESCE(created_by::text, ''), created_at, updated_at`

// Repository implements usecase.ProjectRepository against Postgres via pgx
// — hand-written SQL (see architecture/04-tech-stack.md: sqlc codegen is the
// eventual target, this scaffold hand-writes the equivalent queries
// directly to avoid a build-time dependency on the sqlc binary, matching
// usage-service's reference scaffold).
type Repository struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Create(ctx context.Context, p domain.Project) (domain.Project, error) {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO project.projects (id, tenant_id, name, dev_server_id, description, default_branch, visibility, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING `+projectColumns,
		p.ID, p.TenantID, p.Name, nullableString(p.DevServerID), p.Description, p.DefaultBranch, p.Visibility, nullableString(p.CreatedBy),
	)

	out, err := scanProject(row)
	if err != nil {
		return domain.Project{}, fmt.Errorf("postgres: insert project: %w", err)
	}
	return out, nil
}

func (r *Repository) Get(ctx context.Context, tenantID, id string) (domain.Project, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT `+projectColumns+`
		FROM project.projects
		WHERE tenant_id = $1 AND id = $2
	`, tenantID, id)

	out, err := scanProject(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Project{}, domain.ErrProjectNotFound
	}
	if err != nil {
		return domain.Project{}, fmt.Errorf("postgres: query project: %w", err)
	}
	return out, nil
}

// List scopes to userID's own project_members rows, not just tenantID —
// see usecase.ProjectRepository.List's doc comment (found live: a bare
// tenant_id filter leaked every tenant member's projects to every other
// member — no RLS guard catches this either, since project_members' own
// tenant isolation is transitive through projects, not membership-aware).
func (r *Repository) List(ctx context.Context, tenantID, userID, pageToken string, pageSize int32) ([]domain.Project, string, error) {
	var rows pgx.Rows
	var err error
	if pageToken == "" {
		// AIP-158: an empty/absent page_token means "from the beginning" —
		// no cursor comparison at all. See specs/backend-go/bugs/missing-v2/BUG-004:
		// binding "" into `id > $2` (id is UUID) previously errored on
		// every first-page call.
		rows, err = r.pool.Query(ctx, `
			SELECT `+projectColumns+`
			FROM project.projects
			JOIN project.project_members ON project_members.project_id = projects.id
			WHERE projects.tenant_id = $1 AND project_members.user_id = $2
			ORDER BY projects.id
			LIMIT $3
		`, tenantID, userID, pageSize)
	} else {
		rows, err = r.pool.Query(ctx, `
			SELECT `+projectColumns+`
			FROM project.projects
			JOIN project.project_members ON project_members.project_id = projects.id
			WHERE projects.tenant_id = $1 AND project_members.user_id = $2 AND projects.id > $3
			ORDER BY projects.id
			LIMIT $4
		`, tenantID, userID, pageToken, pageSize)
	}
	if err != nil {
		return nil, "", fmt.Errorf("postgres: query projects: %w", err)
	}
	defer rows.Close()

	var out []domain.Project
	for rows.Next() {
		p, err := scanProject(rows)
		if err != nil {
			return nil, "", fmt.Errorf("postgres: scan project row: %w", err)
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("postgres: iterate project rows: %w", err)
	}

	next := ""
	if int32(len(out)) == pageSize && len(out) > 0 {
		next = out[len(out)-1].ID
	}
	return out, next, nil
}

func (r *Repository) AddMember(ctx context.Context, m domain.ProjectMember) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO project.project_members (project_id, user_id, role)
		VALUES ($1, $2, $3)
		ON CONFLICT (project_id, user_id) DO UPDATE SET role = EXCLUDED.role
	`, m.ProjectID, m.UserID, string(m.Role))
	if err != nil {
		return fmt.Errorf("postgres: insert project member: %w", err)
	}
	return nil
}

// UpdateDevServerID is the ONLY write path for dev_server_id — called after
// usecase.RebindDevServer's active-execution guard has already passed. See
// project-service.md §3: "UpdateProject's field mask rejects dev_server_id
// so there is exactly one code path for rebinding, not two that can drift."
func (r *Repository) UpdateDevServerID(ctx context.Context, tenantID, projectID, devServerID string) (domain.Project, error) {
	row := r.pool.QueryRow(ctx, `
		UPDATE project.projects
		SET dev_server_id = $3, updated_at = now()
		WHERE tenant_id = $1 AND id = $2
		RETURNING `+projectColumns,
		tenantID, projectID, nullableString(devServerID),
	)

	out, err := scanProject(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Project{}, domain.ErrProjectNotFound
	}
	if err != nil {
		return domain.Project{}, fmt.Errorf("postgres: update dev_server_id: %w", err)
	}
	return out, nil
}

// UpdateProject applies patch's non-empty fields via COALESCE(NULLIF($n,
// ”), column) — an empty string argument leaves the column unchanged, per
// domain.ProjectUpdatePatch's "" = no-change semantics. Never references
// dev_server_id.
func (r *Repository) UpdateProject(ctx context.Context, tenantID, projectID string, patch domain.ProjectUpdatePatch) (domain.Project, error) {
	row := r.pool.QueryRow(ctx, `
		UPDATE project.projects
		SET name           = COALESCE(NULLIF($3, ''), name),
		    description    = COALESCE(NULLIF($4, ''), description),
		    default_branch = COALESCE(NULLIF($5, ''), default_branch),
		    visibility     = COALESCE(NULLIF($6, ''), visibility),
		    updated_at     = now()
		WHERE tenant_id = $1 AND id = $2
		RETURNING `+projectColumns,
		tenantID, projectID, patch.Name, patch.Description, patch.DefaultBranch, patch.Visibility,
	)

	out, err := scanProject(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Project{}, domain.ErrProjectNotFound
	}
	if err != nil {
		return domain.Project{}, fmt.Errorf("postgres: update project: %w", err)
	}
	return out, nil
}

// DeleteProject hard-deletes a project row. project_members/repos/worktrees
// cascade via ON DELETE CASCADE FKs (migrations/0001/0003/0004) — see
// usecase.DeleteProject's doc comment for why a single DELETE here is
// sufficient.
func (r *Repository) DeleteProject(ctx context.Context, tenantID, projectID string) error {
	tag, err := r.pool.Exec(ctx, `
		DELETE FROM project.projects WHERE tenant_id = $1 AND id = $2
	`, tenantID, projectID)
	if err != nil {
		return fmt.Errorf("postgres: delete project: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrProjectNotFound
	}
	return nil
}

// GetMembership implements usecase.ProjectRepository.GetMembership (and,
// structurally, usecase.MembershipRepository — see that interface's doc
// comment) against project.project_members.
func (r *Repository) GetMembership(ctx context.Context, projectID, userID string) (domain.ProjectMember, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT project_id, user_id, role
		FROM project.project_members
		WHERE project_id = $1 AND user_id = $2
	`, projectID, userID)

	var m domain.ProjectMember
	var role string
	if err := row.Scan(&m.ProjectID, &m.UserID, &role); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ProjectMember{}, domain.ErrMembershipNotFound
		}
		return domain.ProjectMember{}, fmt.Errorf("postgres: query project membership: %w", err)
	}
	m.Role = domain.ProjectRole(role)
	return m, nil
}

func (r *Repository) ListMembers(ctx context.Context, projectID string) ([]domain.ProjectMember, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT project_id, user_id, role
		FROM project.project_members
		WHERE project_id = $1
		ORDER BY user_id
	`, projectID)
	if err != nil {
		return nil, fmt.Errorf("postgres: query project members: %w", err)
	}
	defer rows.Close()

	var out []domain.ProjectMember
	for rows.Next() {
		var m domain.ProjectMember
		var role string
		if err := rows.Scan(&m.ProjectID, &m.UserID, &role); err != nil {
			return nil, fmt.Errorf("postgres: scan project member row: %w", err)
		}
		m.Role = domain.ProjectRole(role)
		out = append(out, m)
	}
	return out, rows.Err()
}

func (r *Repository) RemoveMember(ctx context.Context, projectID, userID string) error {
	tag, err := r.pool.Exec(ctx, `
		DELETE FROM project.project_members WHERE project_id = $1 AND user_id = $2
	`, projectID, userID)
	if err != nil {
		return fmt.Errorf("postgres: delete project member: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrMembershipNotFound
	}
	return nil
}

func (r *Repository) UpdateMemberRole(ctx context.Context, projectID, userID string, role domain.ProjectRole) (domain.ProjectMember, error) {
	tag, err := r.pool.Exec(ctx, `
		UPDATE project.project_members SET role = $3
		WHERE project_id = $1 AND user_id = $2
	`, projectID, userID, string(role))
	if err != nil {
		return domain.ProjectMember{}, fmt.Errorf("postgres: update project member role: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ProjectMember{}, domain.ErrMembershipNotFound
	}
	return domain.ProjectMember{ProjectID: projectID, UserID: userID, Role: role}, nil
}

// CountOwners is the read RemoveMember/UpdateMemberRole use to enforce the
// "≥1 owner" invariant before mutating — see usecase.AssertNotLastOwnerRemoval.
func (r *Repository) CountOwners(ctx context.Context, projectID string) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM project.project_members
		WHERE project_id = $1 AND role = $2
	`, projectID, string(domain.ProjectRoleOwner)).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("postgres: count project owners: %w", err)
	}
	return count, nil
}

// rowScanner is satisfied by both pgx.Rows and pgx.Row, letting one scan
// helper serve both a single-row QueryRow and a multi-row Query loop.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanProject(row rowScanner) (domain.Project, error) {
	var p domain.Project
	if err := row.Scan(
		&p.ID, &p.TenantID, &p.Name, &p.DevServerID, &p.Description, &p.DefaultBranch, &p.Visibility, &p.CreatedBy,
		&p.CreatedAt, &p.UpdatedAt,
	); err != nil {
		return domain.Project{}, err
	}
	return p, nil
}

func nullableString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
