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
