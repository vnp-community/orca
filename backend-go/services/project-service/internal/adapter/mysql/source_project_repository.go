package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

const sourceProjectColumns = `id, container_project_id, source_project_id, linked_by, linked_at`

// SourceProjectRepository implements usecase.SourceProjectRepository
// against `source_projects` — mirrors postgres.SourceProjectRepository.
type SourceProjectRepository struct {
	db *sql.DB
}

func NewSourceProjectRepository(db *sql.DB) *SourceProjectRepository {
	return &SourceProjectRepository{db: db}
}

// Link upserts the join row — a re-link bumps linked_at/linked_by, same as
// postgres.SourceProjectRepository.Link's ON CONFLICT DO UPDATE.
func (r *SourceProjectRepository) Link(ctx context.Context, sp domain.SourceProject) (domain.SourceProject, error) {
	now := time.Now().UTC()
	if _, err := r.db.ExecContext(ctx, `
		INSERT INTO source_projects (id, container_project_id, source_project_id, linked_by, linked_at)
		VALUES (?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE linked_by = VALUES(linked_by), linked_at = VALUES(linked_at)
	`, sp.ID, sp.ContainerProjectID, sp.SourceProjectID, sp.LinkedBy, now); err != nil {
		return domain.SourceProject{}, fmt.Errorf("mysql: upsert source project: %w", err)
	}

	row := r.db.QueryRowContext(ctx, `
		SELECT `+sourceProjectColumns+` FROM source_projects
		WHERE container_project_id = ? AND source_project_id = ?
	`, sp.ContainerProjectID, sp.SourceProjectID)
	out, err := scanSourceProject(row)
	if err != nil {
		return domain.SourceProject{}, fmt.Errorf("mysql: read back upserted source project: %w", err)
	}
	return out, nil
}

func (r *SourceProjectRepository) Unlink(ctx context.Context, containerProjectID, sourceProjectID string) error {
	if _, err := r.db.ExecContext(ctx, `
		DELETE FROM source_projects
		WHERE container_project_id = ? AND source_project_id = ?
	`, containerProjectID, sourceProjectID); err != nil {
		return fmt.Errorf("mysql: delete source project: %w", err)
	}
	return nil
}

func (r *SourceProjectRepository) List(ctx context.Context, containerProjectID string) ([]domain.SourceProject, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+sourceProjectColumns+`
		FROM source_projects
		WHERE container_project_id = ?
		ORDER BY linked_at, id
	`, containerProjectID)
	if err != nil {
		return nil, fmt.Errorf("mysql: query source projects: %w", err)
	}
	defer rows.Close()

	var out []domain.SourceProject
	for rows.Next() {
		sp, err := scanSourceProject(rows)
		if err != nil {
			return nil, fmt.Errorf("mysql: scan source project row: %w", err)
		}
		out = append(out, sp)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("mysql: iterate source project rows: %w", err)
	}
	return out, nil
}

func (r *SourceProjectRepository) Get(ctx context.Context, containerProjectID, sourceProjectID string) (domain.SourceProject, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT `+sourceProjectColumns+`
		FROM source_projects
		WHERE container_project_id = ? AND source_project_id = ?
	`, containerProjectID, sourceProjectID)
	out, err := scanSourceProject(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.SourceProject{}, fmt.Errorf("mysql: get source project: %w", domain.ErrSourceProjectNotFound)
		}
		return domain.SourceProject{}, fmt.Errorf("mysql: get source project: %w", err)
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
