package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
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
	var s domain.ProjectEngineSettings
	found := false
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		if notUUID(projectID) {
			return nil
		}
		var engine, mode string
		var minVer *string
		err := db.QueryRow(ctx, `
			SELECT tenant_id::text, project_id::text, solution_engine, openspec_min_version, solution_agent, execution_mode, plan_auto_generate, updated_by::text, updated_at, version
			FROM request.project_engine_settings
			WHERE tenant_id = $1 AND project_id = $2`, tenantID, projectID).Scan(
			&s.TenantID, &s.ProjectID, &engine, &minVer, &s.SolutionAgent, &mode, &s.PlanAutoGenerate, &s.UpdatedBy, &s.UpdatedAt, &s.Version,
		)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("postgres: get engine settings: %w", err)
		}
		s.Engine = domain.EngineName(engine)
		s.ExecutionMode = domain.ExecutionMode(mode)
		if minVer != nil {
			s.MinVersion = *minVer
		}
		found = true
		return nil
	})
	return s, found, err
}

// Upsert inserts when expectedVersion is 0 and otherwise updates the row at that version; a lost race on either path is a
// version conflict, never a silent overwrite.
func (r *ProjectEngineSettingsRepository) Upsert(ctx context.Context, s domain.ProjectEngineSettings, expectedVersion int64) (domain.ProjectEngineSettings, error) {
	var minVer any
	if s.MinVersion != "" {
		minVer = s.MinVersion
	}
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		if s.TenantID != tenantID {
			return domain.ErrRequestTenantRequired()
		}
		if expectedVersion == 0 {
			tag, err := db.Exec(ctx, `
				INSERT INTO request.project_engine_settings (tenant_id, project_id, solution_engine, openspec_min_version, solution_agent, execution_mode, plan_auto_generate, updated_by, updated_at, version)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 1)
				ON CONFLICT DO NOTHING`, tenantID, s.ProjectID, string(s.Engine), minVer, s.SolutionAgent, string(s.ExecutionMode.OrDefault()), s.PlanAutoGenerate, s.UpdatedBy, s.UpdatedAt)
			if err != nil {
				return fmt.Errorf("postgres: insert engine settings: %w", err)
			}
			if tag.RowsAffected() == 0 {
				return domain.ErrEngineSettingsVersionConflict
			}
			s.Version = 1
			return nil
		}
		tag, err := db.Exec(ctx, `
			UPDATE request.project_engine_settings
			SET solution_engine = $4, openspec_min_version = $5, solution_agent = $6, execution_mode = $7, plan_auto_generate = $8, updated_by = $9, updated_at = $10, version = version + 1
			WHERE tenant_id = $1 AND project_id = $2 AND version = $3`,
			tenantID, s.ProjectID, expectedVersion, string(s.Engine), minVer, s.SolutionAgent, string(s.ExecutionMode.OrDefault()), s.PlanAutoGenerate, s.UpdatedBy, s.UpdatedAt)
		if err != nil {
			return fmt.Errorf("postgres: update engine settings: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return domain.ErrEngineSettingsVersionConflict
		}
		s.Version = expectedVersion + 1
		return nil
	})
	return s, err
}
