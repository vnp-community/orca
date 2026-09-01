package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

const repoColumns = `id, project_id, url, display_name, position`

// RepoRepository implements usecase.RepoRepository against project.repos.
// Kept as its own struct (not folded into Repository) — one struct per
// entity/port, matching task-service's postgres package layout
// (edges.go/grants.go/repository.go, each a distinct concern).
type RepoRepository struct {
	pool *pgxpool.Pool
}

func NewRepoRepository(pool *pgxpool.Pool) *RepoRepository {
	return &RepoRepository{pool: pool}
}

// AddRepo assigns the next position atomically within the INSERT itself
// (MAX(position)+1 over the project's existing repos, 0 if none) — see
// usecase.RepoRepository.AddRepo's doc comment. Concurrent AddRepo calls for
// the same project could in principle compute the same MAX and collide on
// position; positions aren't unique (see domain.Repo.Position), so this is a
// display-order tie, not a correctness bug — acceptable for how rarely repos
// are added.
func (r *RepoRepository) AddRepo(ctx context.Context, repo domain.Repo) (domain.Repo, error) {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO project.repos (id, project_id, url, display_name, position)
		SELECT $1, $2, $3, $4, COALESCE(MAX(position) + 1, 0)
		FROM project.repos WHERE project_id = $2
		RETURNING `+repoColumns,
		repo.ID, repo.ProjectID, repo.URL, repo.DisplayName,
	)

	out, err := scanRepo(row)
	if err != nil {
		return domain.Repo{}, fmt.Errorf("postgres: insert repo: %w", err)
	}
	return out, nil
}

func (r *RepoRepository) ListRepos(ctx context.Context, projectID string) ([]domain.Repo, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+repoColumns+`
		FROM project.repos
		WHERE project_id = $1
		ORDER BY position, id
	`, projectID)
	if err != nil {
		return nil, fmt.Errorf("postgres: query repos: %w", err)
	}
	defer rows.Close()

	var out []domain.Repo
	for rows.Next() {
		repo, err := scanRepo(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres: scan repo row: %w", err)
		}
		out = append(out, repo)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: iterate repo rows: %w", err)
	}
	return out, nil
}

// ListReposForTenant returns every repo in the caller's tenant, across every
// project — see usecase.RepoRepository.ListReposForTenant's doc comment. No
// project_id filter: relies on Postgres RLS to scope project.repos (via its
// owning project.projects row) to the caller's tenant, same pattern as
// WorktreeRepository.ListLineage's tenant-wide query.
func (r *RepoRepository) ListReposForTenant(ctx context.Context) ([]domain.Repo, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+repoColumns+`
		FROM project.repos
		ORDER BY project_id, position, id
	`)
	if err != nil {
		return nil, fmt.Errorf("postgres: query repos for tenant: %w", err)
	}
	defer rows.Close()

	var out []domain.Repo
	for rows.Next() {
		repo, err := scanRepo(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres: scan repo row: %w", err)
		}
		out = append(out, repo)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: iterate repo rows: %w", err)
	}
	return out, nil
}

// ReorderRepos rewrites every listed repo's position (0-indexed, by list
// order) in one transaction — the caller (usecase.ReorderRepos) has already
// validated idsInOrder is an exact permutation of the project's repo ids.
func (r *RepoRepository) ReorderRepos(ctx context.Context, projectID string, idsInOrder []string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("postgres: begin reorder repos transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	for i, id := range idsInOrder {
		tag, err := tx.Exec(ctx, `
			UPDATE project.repos SET position = $1 WHERE id = $2 AND project_id = $3
		`, i, id, projectID)
		if err != nil {
			return fmt.Errorf("postgres: update repo position: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return domain.ErrRepoNotFound
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("postgres: commit reorder repos transaction: %w", err)
	}
	return nil
}

// GetRepo implements usecase.RepoRepository.GetRepo — used by
// usecase.RemoveRepo to resolve a repo's owning project_id for the OPA
// owner-only authorization check.
func (r *RepoRepository) GetRepo(ctx context.Context, repoID string) (domain.Repo, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT `+repoColumns+`
		FROM project.repos
		WHERE id = $1
	`, repoID)

	out, err := scanRepo(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Repo{}, domain.ErrRepoNotFound
	}
	if err != nil {
		return domain.Repo{}, fmt.Errorf("postgres: query repo: %w", err)
	}
	return out, nil
}

// Update persists repo's current url/display_name — used by
// usecase.UpdateRepo after it applies the field-mask.
func (r *RepoRepository) Update(ctx context.Context, repo domain.Repo) (domain.Repo, error) {
	row := r.pool.QueryRow(ctx, `
		UPDATE project.repos
		SET url = $2, display_name = $3
		WHERE id = $1
		RETURNING `+repoColumns,
		repo.ID, repo.URL, repo.DisplayName,
	)

	out, err := scanRepo(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Repo{}, domain.ErrRepoNotFound
	}
	if err != nil {
		return domain.Repo{}, fmt.Errorf("postgres: update repo: %w", err)
	}
	return out, nil
}

func (r *RepoRepository) RemoveRepo(ctx context.Context, repoID string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM project.repos WHERE id = $1`, repoID)
	if err != nil {
		return fmt.Errorf("postgres: delete repo: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrRepoNotFound
	}
	return nil
}

// scanRepo does NOT convert pgx.ErrNoRows itself — see scanProjectGroup's
// identical doc comment (repository.go's scanProject sets this pattern:
// callers check errors.Is(err, pgx.ErrNoRows) against the raw scan error).
func scanRepo(row rowScanner) (domain.Repo, error) {
	var r domain.Repo
	if err := row.Scan(&r.ID, &r.ProjectID, &r.URL, &r.DisplayName, &r.Position); err != nil {
		return domain.Repo{}, err
	}
	return r, nil
}

// ── repo_members (functional-role tier) ─────────────────────────────────
// Mirrors project_members' own postgres methods above one tier down —
// repo_id/user_id/functional_role instead of project_id/user_id/role.

func (r *RepoRepository) AddRepoMember(ctx context.Context, m domain.RepoMember) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO project.repo_members (repo_id, user_id, functional_role)
		VALUES ($1, $2, $3)
		ON CONFLICT (repo_id, user_id) DO UPDATE SET functional_role = EXCLUDED.functional_role
	`, m.RepoID, m.UserID, string(m.Role))
	if err != nil {
		return fmt.Errorf("postgres: insert repo member: %w", err)
	}
	return nil
}

func (r *RepoRepository) GetRepoMembership(ctx context.Context, repoID, userID string) (domain.RepoMember, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT repo_id, user_id, functional_role
		FROM project.repo_members
		WHERE repo_id = $1 AND user_id = $2
	`, repoID, userID)

	var m domain.RepoMember
	var role string
	if err := row.Scan(&m.RepoID, &m.UserID, &role); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.RepoMember{}, domain.ErrRepoMembershipNotFound
		}
		return domain.RepoMember{}, fmt.Errorf("postgres: query repo membership: %w", err)
	}
	m.Role = domain.RepoRole(role)
	return m, nil
}

func (r *RepoRepository) ListRepoMembers(ctx context.Context, repoID string) ([]domain.RepoMember, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT repo_id, user_id, functional_role
		FROM project.repo_members
		WHERE repo_id = $1
		ORDER BY user_id
	`, repoID)
	if err != nil {
		return nil, fmt.Errorf("postgres: query repo members: %w", err)
	}
	defer rows.Close()

	var out []domain.RepoMember
	for rows.Next() {
		var m domain.RepoMember
		var role string
		if err := rows.Scan(&m.RepoID, &m.UserID, &role); err != nil {
			return nil, fmt.Errorf("postgres: scan repo member row: %w", err)
		}
		m.Role = domain.RepoRole(role)
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: iterate repo member rows: %w", err)
	}
	return out, nil
}

func (r *RepoRepository) RemoveRepoMember(ctx context.Context, repoID, userID string) error {
	tag, err := r.pool.Exec(ctx, `
		DELETE FROM project.repo_members WHERE repo_id = $1 AND user_id = $2
	`, repoID, userID)
	if err != nil {
		return fmt.Errorf("postgres: delete repo member: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrRepoMembershipNotFound
	}
	return nil
}

func (r *RepoRepository) UpdateRepoMemberRole(ctx context.Context, repoID, userID string, role domain.RepoRole) (domain.RepoMember, error) {
	tag, err := r.pool.Exec(ctx, `
		UPDATE project.repo_members SET functional_role = $3
		WHERE repo_id = $1 AND user_id = $2
	`, repoID, userID, string(role))
	if err != nil {
		return domain.RepoMember{}, fmt.Errorf("postgres: update repo member role: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.RepoMember{}, domain.ErrRepoMembershipNotFound
	}
	return domain.RepoMember{RepoID: repoID, UserID: userID, Role: role}, nil
}

// ListRepoIDsWithMembership backs usecase.ListRepos' non-owner visibility
// filter — joins through project.repos rather than trusting a caller-
// supplied repo id list, so it can't return an id from a different project.
func (r *RepoRepository) ListRepoIDsWithMembership(ctx context.Context, projectID, userID string) ([]string, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT rm.repo_id
		FROM project.repo_members rm
		JOIN project.repos r ON r.id = rm.repo_id
		WHERE r.project_id = $1 AND rm.user_id = $2
	`, projectID, userID)
	if err != nil {
		return nil, fmt.Errorf("postgres: query repo ids with membership: %w", err)
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("postgres: scan repo id: %w", err)
		}
		out = append(out, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: iterate repo id rows: %w", err)
	}
	return out, nil
}
