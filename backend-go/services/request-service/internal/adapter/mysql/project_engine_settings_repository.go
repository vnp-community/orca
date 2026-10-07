package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

var _ usecase.EngineSettingsRepository = (*ProjectEngineSettingsRepository)(nil)

type ProjectEngineSettingsRepository struct {
	*Repository
}

func NewProjectEngineSettingsRepository(r *Repository) *ProjectEngineSettingsRepository {
	return &ProjectEngineSettingsRepository{Repository: r}
}

func (r *ProjectEngineSettingsRepository) Get(ctx context.Context, projectID string) (domain.ProjectEngineSettings, bool, error) {
	query := `
		SELECT tenant_id, project_id, solution_engine, openspec_min_version, updated_by, updated_at, version
		FROM project_engine_settings
		WHERE project_id = ?
	`
	var s domain.ProjectEngineSettings
	var engine string
	var minVer sql.NullString
	err := r.exec(ctx).QueryRowContext(ctx, query, projectID).Scan(
		&s.TenantID, &s.ProjectID, &engine, &minVer, &s.UpdatedBy, &s.UpdatedAt, &s.Version,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return s, false, nil
		}
		return s, false, fmt.Errorf("mysql get engine settings: %w", err)
	}
	s.Engine = domain.EngineName(engine)
	if minVer.Valid {
		s.MinVersion = minVer.String
	}
	return s, true, nil
}

func (r *ProjectEngineSettingsRepository) Upsert(ctx context.Context, s domain.ProjectEngineSettings, expectedVersion int64) (domain.ProjectEngineSettings, error) {
	var minVer interface{}
	if s.MinVersion != "" {
		minVer = s.MinVersion
	}

	if expectedVersion == 0 {
		query := `
			INSERT IGNORE INTO project_engine_settings (
				tenant_id, project_id, solution_engine, openspec_min_version, updated_by, updated_at, version
			) VALUES (?, ?, ?, ?, ?, ?, 1)
		`
		res, err := r.exec(ctx).ExecContext(ctx, query,
			s.TenantID, s.ProjectID, string(s.Engine), minVer, s.UpdatedBy, s.UpdatedAt,
		)
		if err != nil {
			return s, fmt.Errorf("mysql insert engine settings: %w", err)
		}
		rows, _ := res.RowsAffected()
		if rows == 0 {
			return s, domain.ErrEngineSettingsVersionConflict
		}
		s.Version = 1
		return s, nil
	}

	query := `
		UPDATE project_engine_settings
		SET solution_engine = ?, openspec_min_version = ?, updated_by = ?, updated_at = ?, version = version + 1
		WHERE tenant_id = ? AND project_id = ? AND version = ?
	`
	res, err := r.exec(ctx).ExecContext(ctx, query,
		string(s.Engine), minVer, s.UpdatedBy, s.UpdatedAt, s.TenantID, s.ProjectID, expectedVersion,
	)
	if err != nil {
		return s, fmt.Errorf("mysql update engine settings: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return s, domain.ErrEngineSettingsVersionConflict
	}
	s.Version = expectedVersion + 1
	return s, nil
}
