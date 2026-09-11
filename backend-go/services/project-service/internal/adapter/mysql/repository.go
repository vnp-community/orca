// Package mysql implements project-service's ProjectRepository/
// RepoRepository/WorktreeRepository/ProjectGroupRepository/
// SparsePresetRepository/SourceProjectRepository/HostSetupRepository/
// FolderWorkspaceRepository ports (defined in internal/usecase) against this
// service's own MySQL/TiDB database — the dialect-2 counterpart to
// internal/adapter/postgres, wired via cmd/server/main.go's
// `switch caps.Dialect` (BE-DB-SOL-001 §3). See
// specs/backend-go/crs/v4/multi-database/solutions/BE-DB-SOL-016-project-service-mysql-tidb-adapter.md
// for the full translation writeup.
package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"

	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

// projectColumns is the column list shared by every SELECT against
// projects — kept as one constant so Create/Get/List/UpdateDevServerID/
// UpdateProject/scanProject can't drift out of sync, mirroring
// internal/adapter/postgres/repository.go's identical convention.
const projectColumns = `id, tenant_id, name, dev_server_id, description, default_branch, visibility, created_by, created_at, updated_at, issue_status_sync_enabled, mobile_emulator_agent_id`

// Repository implements usecase.ProjectRepository against MySQL via
// database/sql + github.com/go-sql-driver/mysql.
type Repository struct {
	db *sql.DB
}

func New(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, p domain.Project) (domain.Project, error) {
	now := time.Now().UTC()
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO projects (id, tenant_id, name, dev_server_id, description, default_branch, visibility, created_by, issue_status_sync_enabled, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, p.ID, p.TenantID, p.Name, nullableString(p.DevServerID), p.Description, p.DefaultBranch, p.Visibility, nullableString(p.CreatedBy), p.IssueStatusSyncEnabled, now, now)
	if err != nil {
		return domain.Project{}, fmt.Errorf("mysql: insert project: %w", err)
	}
	// No RETURNING in MySQL — every value returned is either a caller-
	// supplied input or the `now` this method just computed itself, so
	// constructing the result in Go avoids an unnecessary read-after-write
	// round trip (unlike UPDATE-with-a-patch methods below, which must
	// re-SELECT since they don't know the pre-existing column values).
	p.CreatedAt, p.UpdatedAt = now, now
	return p, nil
}

func (r *Repository) Get(ctx context.Context, tenantID, id string) (domain.Project, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT `+projectColumns+`
		FROM projects
		WHERE tenant_id = ? AND id = ?
	`, tenantID, id)

	out, err := scanProject(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Project{}, domain.ErrProjectNotFound
	}
	if err != nil {
		return domain.Project{}, fmt.Errorf("mysql: query project: %w", err)
	}
	return out, nil
}

// List scopes to userID's own project_members rows, not just tenantID — see
// usecase.ProjectRepository.List's doc comment (mirrors
// postgres.Repository.List exactly).
func (r *Repository) List(ctx context.Context, tenantID, userID, pageToken string, pageSize int32) ([]domain.Project, string, error) {
	var rows *sql.Rows
	var err error
	if pageToken == "" {
		rows, err = r.db.QueryContext(ctx, `
			SELECT `+projectColumns+`
			FROM projects
			JOIN project_members ON project_members.project_id = projects.id
			WHERE projects.tenant_id = ? AND project_members.user_id = ?
			ORDER BY projects.id
			LIMIT ?
		`, tenantID, userID, pageSize)
	} else {
		rows, err = r.db.QueryContext(ctx, `
			SELECT `+projectColumns+`
			FROM projects
			JOIN project_members ON project_members.project_id = projects.id
			WHERE projects.tenant_id = ? AND project_members.user_id = ? AND projects.id > ?
			ORDER BY projects.id
			LIMIT ?
		`, tenantID, userID, pageToken, pageSize)
	}
	if err != nil {
		return nil, "", fmt.Errorf("mysql: query projects: %w", err)
	}
	defer rows.Close()

	var out []domain.Project
	for rows.Next() {
		p, err := scanProject(rows)
		if err != nil {
			return nil, "", fmt.Errorf("mysql: scan project row: %w", err)
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("mysql: iterate project rows: %w", err)
	}

	next := ""
	if int32(len(out)) == pageSize && len(out) > 0 {
		next = out[len(out)-1].ID
	}
	return out, next, nil
}

// ListForMember is List's membership-scoped counterpart — see
// usecase.ProjectRepository.ListForMember's doc comment.
func (r *Repository) ListForMember(ctx context.Context, tenantID, userID, pageToken string, pageSize int32) ([]domain.Project, string, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+projectColumns+`
		FROM projects p
		JOIN project_members m ON m.project_id = p.id
		WHERE p.tenant_id = ? AND m.user_id = ? AND p.id > ?
		ORDER BY p.id
		LIMIT ?
	`, tenantID, userID, pageToken, pageSize)
	if err != nil {
		return nil, "", fmt.Errorf("mysql: query member projects: %w", err)
	}
	defer rows.Close()

	var out []domain.Project
	for rows.Next() {
		p, err := scanProject(rows)
		if err != nil {
			return nil, "", fmt.Errorf("mysql: scan project row: %w", err)
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("mysql: iterate member project rows: %w", err)
	}

	next := ""
	if int32(len(out)) == pageSize && len(out) > 0 {
		next = out[len(out)-1].ID
	}
	return out, next, nil
}

func (r *Repository) AddMember(ctx context.Context, m domain.ProjectMember) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO project_members (project_id, user_id, role)
		VALUES (?, ?, ?)
		ON DUPLICATE KEY UPDATE role = VALUES(role)
	`, m.ProjectID, m.UserID, string(m.Role))
	if err != nil {
		return fmt.Errorf("mysql: insert project member: %w", err)
	}
	return nil
}

// UpdateDevServerID is the ONLY write path for dev_server_id — see
// postgres.Repository.UpdateDevServerID's identical doc comment. No
// RETURNING in MySQL: UPDATE then re-SELECT by (tenant_id, id) — this also
// sidesteps MySQL's RowsAffected()-counts-CHANGED-not-MATCHED-rows pitfall
// (BE-DB-SOL-005 §3.1 finding): a no-op rebind to the same dev_server_id
// would report RowsAffected()==0 even though the row exists, which would
// wrongly look like "not found" if RowsAffected were used for that check.
func (r *Repository) UpdateDevServerID(ctx context.Context, tenantID, projectID, devServerID string) (domain.Project, error) {
	if _, err := r.db.ExecContext(ctx, `
		UPDATE projects SET dev_server_id = ?, updated_at = ? WHERE tenant_id = ? AND id = ?
	`, nullableString(devServerID), time.Now().UTC(), tenantID, projectID); err != nil {
		return domain.Project{}, fmt.Errorf("mysql: update dev_server_id: %w", err)
	}
	return r.Get(ctx, tenantID, projectID)
}

// UpdateProject applies patch's non-empty fields via
// COALESCE(NULLIF(?, ”), column) — identical semantics to
// postgres.Repository.UpdateProject, MySQL supports both functions
// natively, no dialect-specific rewrite needed beyond `?` placeholders and
// dropping the `::uuid` cast (CHAR(36) needs no cast). See
// UpdateDevServerID's doc comment for why this re-SELECTs rather than
// trusting RowsAffected().
func (r *Repository) UpdateProject(ctx context.Context, tenantID, projectID string, patch domain.ProjectUpdatePatch) (domain.Project, error) {
	if _, err := r.db.ExecContext(ctx, `
		UPDATE projects
		SET name                       = COALESCE(NULLIF(?, ''), name),
		    description                = COALESCE(NULLIF(?, ''), description),
		    default_branch             = COALESCE(NULLIF(?, ''), default_branch),
		    visibility                 = COALESCE(NULLIF(?, ''), visibility),
		    issue_status_sync_enabled  = COALESCE(?, issue_status_sync_enabled),
		    mobile_emulator_agent_id   = COALESCE(NULLIF(?, ''), mobile_emulator_agent_id),
		    updated_at                 = ?
		WHERE tenant_id = ? AND id = ?
	`, patch.Name, patch.Description, patch.DefaultBranch, patch.Visibility, patch.IssueStatusSyncEnabled, patch.MobileEmulatorAgentID, time.Now().UTC(), tenantID, projectID); err != nil {
		return domain.Project{}, fmt.Errorf("mysql: update project: %w", err)
	}
	return r.Get(ctx, tenantID, projectID)
}

// DeleteProject hard-deletes a project row — repos/worktrees/project_members
// cascade via ON DELETE CASCADE FKs (migrations/mysql/0001,0003,0004).
// DELETE's RowsAffected() counts matched (not changed) rows on every
// dialect, so — unlike UPDATE above — it's safe to use directly for
// not-found detection here.
func (r *Repository) DeleteProject(ctx context.Context, tenantID, projectID string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM projects WHERE tenant_id = ? AND id = ?`, tenantID, projectID)
	if err != nil {
		return fmt.Errorf("mysql: delete project: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("mysql: rows affected: %w", err)
	}
	if n == 0 {
		return domain.ErrProjectNotFound
	}
	return nil
}

func (r *Repository) GetMembership(ctx context.Context, projectID, userID string) (domain.ProjectMember, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT project_id, user_id, role
		FROM project_members
		WHERE project_id = ? AND user_id = ?
	`, projectID, userID)

	var m domain.ProjectMember
	var role string
	if err := row.Scan(&m.ProjectID, &m.UserID, &role); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.ProjectMember{}, domain.ErrMembershipNotFound
		}
		return domain.ProjectMember{}, fmt.Errorf("mysql: query project membership: %w", err)
	}
	m.Role = domain.ProjectRole(role)
	return m, nil
}

func (r *Repository) ListMembers(ctx context.Context, projectID string) ([]domain.ProjectMember, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT project_id, user_id, role
		FROM project_members
		WHERE project_id = ?
		ORDER BY user_id
	`, projectID)
	if err != nil {
		return nil, fmt.Errorf("mysql: query project members: %w", err)
	}
	defer rows.Close()

	var out []domain.ProjectMember
	for rows.Next() {
		var m domain.ProjectMember
		var role string
		if err := rows.Scan(&m.ProjectID, &m.UserID, &role); err != nil {
			return nil, fmt.Errorf("mysql: scan project member row: %w", err)
		}
		m.Role = domain.ProjectRole(role)
		out = append(out, m)
	}
	return out, rows.Err()
}

func (r *Repository) RemoveMember(ctx context.Context, projectID, userID string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM project_members WHERE project_id = ? AND user_id = ?`, projectID, userID)
	if err != nil {
		return fmt.Errorf("mysql: delete project member: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("mysql: rows affected: %w", err)
	}
	if n == 0 {
		return domain.ErrMembershipNotFound
	}
	return nil
}

// UpdateMemberRole re-SELECTs after the UPDATE, unlike
// postgres.Repository.UpdateMemberRole (which trusts RETURNING/RowsAffected
// together) — see UpdateDevServerID's doc comment for why RowsAffected
// alone can't distinguish "not found" from "found, but role unchanged" on
// MySQL.
func (r *Repository) UpdateMemberRole(ctx context.Context, projectID, userID string, role domain.ProjectRole) (domain.ProjectMember, error) {
	if _, err := r.db.ExecContext(ctx, `
		UPDATE project_members SET role = ? WHERE project_id = ? AND user_id = ?
	`, string(role), projectID, userID); err != nil {
		return domain.ProjectMember{}, fmt.Errorf("mysql: update project member role: %w", err)
	}
	return r.GetMembership(ctx, projectID, userID)
}

// CountOwners is the read RemoveMember/UpdateMemberRole use to enforce the
// "≥1 owner" invariant before mutating.
func (r *Repository) CountOwners(ctx context.Context, projectID string) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM project_members
		WHERE project_id = ? AND role = ?
	`, projectID, string(domain.ProjectRoleOwner)).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("mysql: count project owners: %w", err)
	}
	return count, nil
}

// rowScanner is satisfied by both *sql.Rows and *sql.Row, letting one scan
// helper serve both a single-row QueryRowContext and a multi-row
// QueryContext loop — mirrors postgres.rowScanner's identical purpose.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanProject(row rowScanner) (domain.Project, error) {
	var p domain.Project
	var devServerID, createdBy, mobileEmulatorAgentID sql.NullString
	if err := row.Scan(
		&p.ID, &p.TenantID, &p.Name, &devServerID, &p.Description, &p.DefaultBranch, &p.Visibility, &createdBy,
		&p.CreatedAt, &p.UpdatedAt, &p.IssueStatusSyncEnabled, &mobileEmulatorAgentID,
	); err != nil {
		return domain.Project{}, err
	}
	p.DevServerID = devServerID.String
	p.CreatedBy = createdBy.String
	p.MobileEmulatorAgentID = mobileEmulatorAgentID.String
	return p, nil
}

// nullableString returns nil for an empty string so it binds SQL NULL —
// the MySQL-side inverse of scanProject's sql.NullString reads, mirrors
// postgres.nullableString's identical purpose (empty string == "unset" is
// this codebase's domain-layer convention, not a real value to persist).
func nullableString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// isMySQLDuplicateEntry/isMySQLForeignKeyViolation classify a MySQL driver
// error by its error code — the MySQL/go-sql-driver counterpart to
// postgres's pgUniqueViolationCode/pgForeignKeyViolationCode SQLSTATE
// checks (folder_workspace_repository.go). 1062 = ER_DUP_ENTRY, 1452 =
// ER_NO_REFERENCED_ROW_2.
func isMySQLDuplicateEntry(err error) bool {
	var mysqlErr *mysqldriver.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1062
}

func isMySQLForeignKeyViolation(err error) bool {
	var mysqlErr *mysqldriver.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1452
}
