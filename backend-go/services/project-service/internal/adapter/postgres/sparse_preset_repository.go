package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

const sparsePresetColumns = `id, repo_id, name, directories, created_at, updated_at`

// SparsePresetRepository implements usecase.SparsePresetRepository against
// project.sparse_presets. Kept as its own struct — one struct per entity/
// port, matching SourceProjectRepository/RepoRepository's own convention in
// this package.
type SparsePresetRepository struct {
	pool *pgxpool.Pool
}

func NewSparsePresetRepository(pool *pgxpool.Pool) *SparsePresetRepository {
	return &SparsePresetRepository{pool: pool}
}

func scanSparsePreset(row rowScanner) (domain.SparsePreset, error) {
	var p domain.SparsePreset
	if err := row.Scan(&p.ID, &p.RepoID, &p.Name, &p.Directories, &p.CreatedAt, &p.UpdatedAt); err != nil {
		return domain.SparsePreset{}, err
	}
	return p, nil
}

func (r *SparsePresetRepository) ListSparsePresets(ctx context.Context, repoID string) ([]domain.SparsePreset, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+sparsePresetColumns+`
		FROM project.sparse_presets
		WHERE repo_id = $1
		ORDER BY name
	`, repoID)
	if err != nil {
		return nil, fmt.Errorf("postgres: query sparse presets: %w", err)
	}
	defer rows.Close()

	var out []domain.SparsePreset
	for rows.Next() {
		p, err := scanSparsePreset(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres: scan sparse preset row: %w", err)
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: iterate sparse preset rows: %w", err)
	}
	return out, nil
}

func (r *SparsePresetRepository) GetSparsePreset(ctx context.Context, repoID, presetID string) (domain.SparsePreset, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT `+sparsePresetColumns+`
		FROM project.sparse_presets
		WHERE repo_id = $1 AND id = $2
	`, repoID, presetID)

	p, err := scanSparsePreset(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.SparsePreset{}, domain.ErrSparsePresetNotFound
	}
	if err != nil {
		return domain.SparsePreset{}, fmt.Errorf("postgres: query sparse preset: %w", err)
	}
	return p, nil
}

// SaveSparsePreset upserts by id — the usecase layer has already resolved
// whether this is a fresh preset or an update (and which id/created_at to
// use), so a plain ON CONFLICT DO UPDATE covers both cases here.
func (r *SparsePresetRepository) SaveSparsePreset(ctx context.Context, preset domain.SparsePreset) (domain.SparsePreset, error) {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO project.sparse_presets (id, repo_id, name, directories, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			directories = EXCLUDED.directories,
			updated_at = EXCLUDED.updated_at
		RETURNING `+sparsePresetColumns,
		preset.ID, preset.RepoID, preset.Name, preset.Directories, preset.CreatedAt, preset.UpdatedAt,
	)

	saved, err := scanSparsePreset(row)
	if err != nil {
		return domain.SparsePreset{}, fmt.Errorf("postgres: upsert sparse preset: %w", err)
	}
	return saved, nil
}

func (r *SparsePresetRepository) RemoveSparsePreset(ctx context.Context, repoID, presetID string) error {
	tag, err := r.pool.Exec(ctx, `
		DELETE FROM project.sparse_presets WHERE repo_id = $1 AND id = $2
	`, repoID, presetID)
	if err != nil {
		return fmt.Errorf("postgres: delete sparse preset: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrSparsePresetNotFound
	}
	return nil
}
