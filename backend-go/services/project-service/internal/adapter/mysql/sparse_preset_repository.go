package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

const sparsePresetColumns = `id, repo_id, name, directories, created_at, updated_at`

// SparsePresetRepository implements usecase.SparsePresetRepository against
// `sparse_presets` — mirrors postgres.SparsePresetRepository.
type SparsePresetRepository struct {
	db *sql.DB
}

func NewSparsePresetRepository(db *sql.DB) *SparsePresetRepository {
	return &SparsePresetRepository{db: db}
}

// scanSparsePreset unmarshals `directories` from its JSON-array column —
// the MySQL translation of Postgres's native TEXT[] (pgx marshals []string
// <-> text[] automatically; MySQL has no array type, see
// migrations/mysql/0016_sparse_presets.up.sql's comment).
func scanSparsePreset(row rowScanner) (domain.SparsePreset, error) {
	var p domain.SparsePreset
	var directoriesJSON []byte
	if err := row.Scan(&p.ID, &p.RepoID, &p.Name, &directoriesJSON, &p.CreatedAt, &p.UpdatedAt); err != nil {
		return domain.SparsePreset{}, err
	}
	if len(directoriesJSON) > 0 {
		if err := json.Unmarshal(directoriesJSON, &p.Directories); err != nil {
			return domain.SparsePreset{}, fmt.Errorf("mysql: unmarshal sparse preset directories: %w", err)
		}
	}
	return p, nil
}

func (r *SparsePresetRepository) ListSparsePresets(ctx context.Context, repoID string) ([]domain.SparsePreset, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+sparsePresetColumns+`
		FROM sparse_presets
		WHERE repo_id = ?
		ORDER BY name
	`, repoID)
	if err != nil {
		return nil, fmt.Errorf("mysql: query sparse presets: %w", err)
	}
	defer rows.Close()

	var out []domain.SparsePreset
	for rows.Next() {
		p, err := scanSparsePreset(rows)
		if err != nil {
			return nil, fmt.Errorf("mysql: scan sparse preset row: %w", err)
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("mysql: iterate sparse preset rows: %w", err)
	}
	return out, nil
}

func (r *SparsePresetRepository) GetSparsePreset(ctx context.Context, repoID, presetID string) (domain.SparsePreset, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT `+sparsePresetColumns+`
		FROM sparse_presets
		WHERE repo_id = ? AND id = ?
	`, repoID, presetID)

	p, err := scanSparsePreset(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.SparsePreset{}, domain.ErrSparsePresetNotFound
	}
	if err != nil {
		return domain.SparsePreset{}, fmt.Errorf("mysql: query sparse preset: %w", err)
	}
	return p, nil
}

// SaveSparsePreset upserts by id — same convention as
// postgres.SparsePresetRepository.SaveSparsePreset: the usecase layer has
// already decided fresh-vs-update and which id/created_at to use.
func (r *SparsePresetRepository) SaveSparsePreset(ctx context.Context, preset domain.SparsePreset) (domain.SparsePreset, error) {
	directoriesJSON, err := json.Marshal(preset.Directories)
	if err != nil {
		return domain.SparsePreset{}, fmt.Errorf("mysql: marshal sparse preset directories: %w", err)
	}

	if _, err := r.db.ExecContext(ctx, `
		INSERT INTO sparse_presets (id, repo_id, name, directories, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			name = VALUES(name),
			directories = VALUES(directories),
			updated_at = VALUES(updated_at)
	`, preset.ID, preset.RepoID, preset.Name, directoriesJSON, preset.CreatedAt, preset.UpdatedAt); err != nil {
		return domain.SparsePreset{}, fmt.Errorf("mysql: upsert sparse preset: %w", err)
	}

	row := r.db.QueryRowContext(ctx, `SELECT `+sparsePresetColumns+` FROM sparse_presets WHERE id = ?`, preset.ID)
	saved, err := scanSparsePreset(row)
	if err != nil {
		return domain.SparsePreset{}, fmt.Errorf("mysql: read back upserted sparse preset: %w", err)
	}
	return saved, nil
}

func (r *SparsePresetRepository) RemoveSparsePreset(ctx context.Context, repoID, presetID string) error {
	res, err := r.db.ExecContext(ctx, `
		DELETE FROM sparse_presets WHERE repo_id = ? AND id = ?
	`, repoID, presetID)
	if err != nil {
		return fmt.Errorf("mysql: delete sparse preset: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("mysql: rows affected: %w", err)
	}
	if n == 0 {
		return domain.ErrSparsePresetNotFound
	}
	return nil
}
