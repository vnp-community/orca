package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

const repoColumns = `id, project_id, url, display_name, position, dev_server_id, hook_settings`

// RepoRepository implements usecase.RepoRepository against `repos` —
// mirrors postgres.RepoRepository's one-struct-per-entity layout.
type RepoRepository struct {
	db *sql.DB
}

func NewRepoRepository(db *sql.DB) *RepoRepository {
	return &RepoRepository{db: db}
}

// AddRepo assigns the next position atomically within the INSERT itself —
// same MAX(position)+1-or-0 subquery as postgres.RepoRepository.AddRepo,
// `INSERT ... SELECT` is standard SQL, works identically on MySQL. No
// RETURNING: re-SELECT by id afterward (id is caller-supplied, always
// known).
func (r *RepoRepository) AddRepo(ctx context.Context, repo domain.Repo) (domain.Repo, error) {
	if _, err := r.db.ExecContext(ctx, `
		INSERT INTO repos (id, project_id, url, display_name, position, dev_server_id)
		SELECT ?, ?, ?, ?, COALESCE(MAX(position) + 1, 0), ?
		FROM repos WHERE project_id = ?
	`, repo.ID, repo.ProjectID, repo.URL, repo.DisplayName, nullableString(repo.DevServerID), repo.ProjectID); err != nil {
		return domain.Repo{}, fmt.Errorf("mysql: insert repo: %w", err)
	}
	return r.GetRepo(ctx, repo.ID)
}

func (r *RepoRepository) ListRepos(ctx context.Context, projectID string) ([]domain.Repo, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+repoColumns+`
		FROM repos
		WHERE project_id = ?
		ORDER BY position, id
	`, projectID)
	if err != nil {
		return nil, fmt.Errorf("mysql: query repos: %w", err)
	}
	defer rows.Close()

	var out []domain.Repo
	for rows.Next() {
		repo, err := scanRepo(rows)
		if err != nil {
			return nil, fmt.Errorf("mysql: scan repo row: %w", err)
		}
		out = append(out, repo)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("mysql: iterate repo rows: %w", err)
	}
	return out, nil
}

// ListReposForTenant returns every repo across every project — unlike
// postgres.RepoRepository.ListReposForTenant, this has no RLS to lean on at
// all (MySQL has none, see 0001_init.up.sql's comment); the query was
// already relying purely on this being an intentionally tenant-wide read
// (see usecase.RepoRepository.ListReposForTenant's doc comment), not a
// scoping gap this migration introduces.
func (r *RepoRepository) ListReposForTenant(ctx context.Context) ([]domain.Repo, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+repoColumns+`
		FROM repos
		ORDER BY project_id, position, id
	`)
	if err != nil {
		return nil, fmt.Errorf("mysql: query repos for tenant: %w", err)
	}
	defer rows.Close()

	var out []domain.Repo
	for rows.Next() {
		repo, err := scanRepo(rows)
		if err != nil {
			return nil, fmt.Errorf("mysql: scan repo row: %w", err)
		}
		out = append(out, repo)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("mysql: iterate repo rows: %w", err)
	}
	return out, nil
}

// ReorderRepos rewrites every listed repo's position in one transaction —
// DELETE-style RowsAffected (0 == not found) is safe here since this is a
// bare `SET position = ?`, not a patch that could no-op to the same value
// on a legitimate call... actually it CAN no-op (re-submitting the same
// order), so — same pitfall as UPDATE elsewhere in this package — this
// checks existence via a separate lookup instead of trusting RowsAffected.
func (r *RepoRepository) ReorderRepos(ctx context.Context, projectID string, idsInOrder []string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("mysql: begin reorder repos transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	for i, id := range idsInOrder {
		var exists bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM repos WHERE id = ? AND project_id = ?)`, id, projectID).Scan(&exists); err != nil {
			return fmt.Errorf("mysql: check repo existence: %w", err)
		}
		if !exists {
			return domain.ErrRepoNotFound
		}
		if _, err := tx.ExecContext(ctx, `UPDATE repos SET position = ? WHERE id = ? AND project_id = ?`, i, id, projectID); err != nil {
			return fmt.Errorf("mysql: update repo position: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("mysql: commit reorder repos transaction: %w", err)
	}
	return nil
}

func (r *RepoRepository) GetRepo(ctx context.Context, repoID string) (domain.Repo, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT `+repoColumns+`
		FROM repos
		WHERE id = ?
	`, repoID)

	out, err := scanRepo(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Repo{}, domain.ErrRepoNotFound
	}
	if err != nil {
		return domain.Repo{}, fmt.Errorf("mysql: query repo: %w", err)
	}
	return out, nil
}

// Update persists repo's current url/display_name/hook_settings — see
// UpdateDevServerID's doc comment (repository.go) for why this re-SELECTs
// rather than trusting RowsAffected after the UPDATE.
func (r *RepoRepository) Update(ctx context.Context, repo domain.Repo) (domain.Repo, error) {
	if _, err := existingRepoOrNotFound(ctx, r.db, repo.ID); err != nil {
		return domain.Repo{}, err
	}
	if _, err := r.db.ExecContext(ctx, `
		UPDATE repos SET url = ?, display_name = ?, hook_settings = ? WHERE id = ?
	`, repo.URL, repo.DisplayName, nullableString(repo.HookSettings), repo.ID); err != nil {
		return domain.Repo{}, fmt.Errorf("mysql: update repo: %w", err)
	}
	return r.GetRepo(ctx, repo.ID)
}

// UpdateDevServerID is the ONLY write path for a repo's dev_server_id.
func (r *RepoRepository) UpdateDevServerID(ctx context.Context, repoID, devServerID string) (domain.Repo, error) {
	if _, err := existingRepoOrNotFound(ctx, r.db, repoID); err != nil {
		return domain.Repo{}, err
	}
	if _, err := r.db.ExecContext(ctx, `UPDATE repos SET dev_server_id = ? WHERE id = ?`, nullableString(devServerID), repoID); err != nil {
		return domain.Repo{}, fmt.Errorf("mysql: update repo dev_server_id: %w", err)
	}
	return r.GetRepo(ctx, repoID)
}

// existingRepoOrNotFound is Update/UpdateDevServerID's not-found guard,
// since a value-preserving UPDATE would otherwise report RowsAffected()==0
// on MySQL even though the row exists (see repository.go's
// UpdateDevServerID doc comment) — check existence up front instead of
// relying on the UPDATE's own result.
func existingRepoOrNotFound(ctx context.Context, db *sql.DB, repoID string) (bool, error) {
	var exists bool
	if err := db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM repos WHERE id = ?)`, repoID).Scan(&exists); err != nil {
		return false, fmt.Errorf("mysql: check repo existence: %w", err)
	}
	if !exists {
		return false, domain.ErrRepoNotFound
	}
	return true, nil
}

// ReassignProject moves repo to a different project — mirrors
// postgres.RepoRepository.ReassignProject's TOCTOU guard and repo_members
// clearing exactly, translated to database/sql transactions.
func (r *RepoRepository) ReassignProject(ctx context.Context, repoID, fromProjectID, targetProjectID string) (domain.Repo, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.Repo{}, fmt.Errorf("mysql: begin reassign repo project transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.ExecContext(ctx, `
		UPDATE repos
		SET project_id = ?,
		    position = COALESCE((SELECT x.m FROM (SELECT MAX(position) + 1 AS m FROM repos WHERE project_id = ?) x), 0)
		WHERE id = ? AND project_id = ?
	`, targetProjectID, targetProjectID, repoID, fromProjectID)
	if err != nil {
		return domain.Repo{}, fmt.Errorf("mysql: reassign repo project: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return domain.Repo{}, fmt.Errorf("mysql: rows affected: %w", err)
	}
	if n == 0 {
		var exists bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM repos WHERE id = ?)`, repoID).Scan(&exists); err != nil {
			return domain.Repo{}, fmt.Errorf("mysql: check repo existence after failed reassign: %w", err)
		}
		if exists {
			return domain.Repo{}, domain.ErrRepoProjectChanged
		}
		return domain.Repo{}, domain.ErrRepoNotFound
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM repo_members WHERE repo_id = ?`, repoID); err != nil {
		return domain.Repo{}, fmt.Errorf("mysql: clear repo members on reassign: %w", err)
	}

	row := tx.QueryRowContext(ctx, `SELECT `+repoColumns+` FROM repos WHERE id = ?`, repoID)
	out, err := scanRepo(row)
	if err != nil {
		return domain.Repo{}, fmt.Errorf("mysql: scan reassigned repo: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return domain.Repo{}, fmt.Errorf("mysql: commit reassign repo project transaction: %w", err)
	}
	return out, nil
}

func (r *RepoRepository) RemoveRepo(ctx context.Context, repoID string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM repos WHERE id = ?`, repoID)
	if err != nil {
		return fmt.Errorf("mysql: delete repo: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("mysql: rows affected: %w", err)
	}
	if n == 0 {
		return domain.ErrRepoNotFound
	}
	return nil
}

func scanRepo(row rowScanner) (domain.Repo, error) {
	var repo domain.Repo
	var devServerID, hookSettings sql.NullString
	if err := row.Scan(
		&repo.ID, &repo.ProjectID, &repo.URL, &repo.DisplayName, &repo.Position, &devServerID, &hookSettings,
	); err != nil {
		return domain.Repo{}, err
	}
	repo.DevServerID = devServerID.String
	repo.HookSettings = hookSettings.String
	return repo, nil
}

// ── repo_members (functional-role tier) ─────────────────────────────────

func (r *RepoRepository) AddRepoMember(ctx context.Context, m domain.RepoMember) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO repo_members (repo_id, user_id, functional_role)
		VALUES (?, ?, ?)
		ON DUPLICATE KEY UPDATE functional_role = VALUES(functional_role)
	`, m.RepoID, m.UserID, string(m.Role))
	if err != nil {
		return fmt.Errorf("mysql: insert repo member: %w", err)
	}
	return nil
}

func (r *RepoRepository) GetRepoMembership(ctx context.Context, repoID, userID string) (domain.RepoMember, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT repo_id, user_id, functional_role
		FROM repo_members
		WHERE repo_id = ? AND user_id = ?
	`, repoID, userID)

	var m domain.RepoMember
	var role string
	if err := row.Scan(&m.RepoID, &m.UserID, &role); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.RepoMember{}, domain.ErrRepoMembershipNotFound
		}
		return domain.RepoMember{}, fmt.Errorf("mysql: query repo membership: %w", err)
	}
	m.Role = domain.RepoRole(role)
	return m, nil
}

func (r *RepoRepository) ListRepoMembers(ctx context.Context, repoID string) ([]domain.RepoMember, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT repo_id, user_id, functional_role
		FROM repo_members
		WHERE repo_id = ?
		ORDER BY user_id
	`, repoID)
	if err != nil {
		return nil, fmt.Errorf("mysql: query repo members: %w", err)
	}
	defer rows.Close()

	var out []domain.RepoMember
	for rows.Next() {
		var m domain.RepoMember
		var role string
		if err := rows.Scan(&m.RepoID, &m.UserID, &role); err != nil {
			return nil, fmt.Errorf("mysql: scan repo member row: %w", err)
		}
		m.Role = domain.RepoRole(role)
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("mysql: iterate repo member rows: %w", err)
	}
	return out, nil
}

func (r *RepoRepository) RemoveRepoMember(ctx context.Context, repoID, userID string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM repo_members WHERE repo_id = ? AND user_id = ?`, repoID, userID)
	if err != nil {
		return fmt.Errorf("mysql: delete repo member: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("mysql: rows affected: %w", err)
	}
	if n == 0 {
		return domain.ErrRepoMembershipNotFound
	}
	return nil
}

// UpdateRepoMemberRole checks existence separately rather than trusting
// RowsAffected — see repository.go's UpdateDevServerID doc comment.
func (r *RepoRepository) UpdateRepoMemberRole(ctx context.Context, repoID, userID string, role domain.RepoRole) (domain.RepoMember, error) {
	if _, err := r.GetRepoMembership(ctx, repoID, userID); err != nil {
		return domain.RepoMember{}, err
	}
	if _, err := r.db.ExecContext(ctx, `
		UPDATE repo_members SET functional_role = ? WHERE repo_id = ? AND user_id = ?
	`, string(role), repoID, userID); err != nil {
		return domain.RepoMember{}, fmt.Errorf("mysql: update repo member role: %w", err)
	}
	return domain.RepoMember{RepoID: repoID, UserID: userID, Role: role}, nil
}

// ListRepoIDsWithMembership backs usecase.ListRepos' non-owner visibility
// filter.
func (r *RepoRepository) ListRepoIDsWithMembership(ctx context.Context, projectID, userID string) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT rm.repo_id
		FROM repo_members rm
		JOIN repos r ON r.id = rm.repo_id
		WHERE r.project_id = ? AND rm.user_id = ?
	`, projectID, userID)
	if err != nil {
		return nil, fmt.Errorf("mysql: query repo ids with membership: %w", err)
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("mysql: scan repo id: %w", err)
		}
		out = append(out, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("mysql: iterate repo id rows: %w", err)
	}
	return out, nil
}
