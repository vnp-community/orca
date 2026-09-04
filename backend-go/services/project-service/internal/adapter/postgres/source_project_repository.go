package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

const sourceProjectColumns = `id, container_project_id, source_project_id, linked_by, linked_at`

// SourceProjectRepository implements usecase.SourceProjectRepository
// against project.source_projects. Kept as its own struct — one struct per
// entity/port, matching RepoRepository/RepoMembershipRepository's own
// convention in this package.
type SourceProjectRepository struct {
	pool *pgxpool.Pool
}

func NewSourceProjectRepository(pool *pgxpool.Pool) *SourceProjectRepository {
	return &SourceProjectRepository{pool: pool}
}

// Link upserts the join row — a re-link (already-linked pair) bumps
// linked_at and linked_by rather than erroring, matching the legacy
// reference's INSERT ... ON CONFLICT DO UPDATE.
func (r *SourceProjectRepository) Link(ctx context.Context, sp domain.SourceProject) (domain.SourceProject, error) {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO project.source_projects (id, container_project_id, source_project_id, linked_by, linked_at)
		VALUES ($1, $2, $3, $4, now())
		ON CONFLICT (container_project_id, source_project_id)
		DO UPDATE SET linked_by = excluded.linked_by, linked_at = excluded.linked_at
		RETURNING `+sourceProjectColumns,
		sp.ID, sp.ContainerProjectID, sp.SourceProjectID, sp.LinkedBy,
	)
	out, err := scanSourceProject(row)
	if err != nil {
		return domain.SourceProject{}, fmt.Errorf("postgres: upsert source project: %w", err)
	}
	return out, nil
}

func (r *SourceProjectRepository) Unlink(ctx context.Context, containerProjectID, sourceProjectID string) error {
	if _, err := r.pool.Exec(ctx, `
		DELETE FROM project.source_projects
		WHERE container_project_id = $1 AND source_project_id = $2
	`, containerProjectID, sourceProjectID); err != nil {
		return fmt.Errorf("postgres: delete source project: %w", err)
	}
	return nil
}

func (r *SourceProjectRepository) List(ctx context.Context, containerProjectID string) ([]domain.SourceProject, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+sourceProjectColumns+`
		FROM project.source_projects
		WHERE container_project_id = $1
		ORDER BY linked_at, id
	`, containerProjectID)
	if err != nil {
		return nil, fmt.Errorf("postgres: query source projects: %w", err)
	}
	defer rows.Close()

	var out []domain.SourceProject
	for rows.Next() {
		sp, err := scanSourceProject(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres: scan source project row: %w", err)
		}
		out = append(out, sp)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: iterate source project rows: %w", err)
	}
	return out, nil
}

func (r *SourceProjectRepository) Get(ctx context.Context, containerProjectID, sourceProjectID string) (domain.SourceProject, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT `+sourceProjectColumns+`
		FROM project.source_projects
		WHERE container_project_id = $1 AND source_project_id = $2
	`, containerProjectID, sourceProjectID)
	out, err := scanSourceProject(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.SourceProject{}, fmt.Errorf("postgres: get source project: %w", domain.ErrSourceProjectNotFound)
		}
		return domain.SourceProject{}, fmt.Errorf("postgres: get source project: %w", err)
	}
	return out, nil
}

func scanSourceProject(row rowScanner) (domain.SourceProject, error) {
	var sp domain.SourceProject
	if err := row.Scan(&sp.ID, &sp.ContainerProjectID, &sp.SourceProjectID, &sp.LinkedBy, &sp.LinkedAt); err != nil {
		return domain.SourceProject{}, err
	}
	return sp, nil
}
