package usecase

import (
	"context"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

type SetProjectEngineSettingsInput struct {
	ProjectID       string
	Engine          string
	MinVersion      string
	ExpectedVersion int64
}

type ManageProjectEngineSettings struct {
	repo EngineSettingsRepository
}

func NewManageProjectEngineSettings(repo EngineSettingsRepository) *ManageProjectEngineSettings {
	return &ManageProjectEngineSettings{repo: repo}
}

func (uc *ManageProjectEngineSettings) GetProjectEngineSettings(ctx context.Context, projectID string) (domain.ProjectEngineSettings, error) {
	settings, ok, err := uc.repo.Get(ctx, projectID)
	if err != nil {
		return domain.ProjectEngineSettings{}, err
	}
	if !ok {
		return domain.ProjectEngineSettings{
			ProjectID: projectID,
			Engine:    domain.EngineNative,
			Version:   0,
		}, nil
	}
	return settings, nil
}

func (uc *ManageProjectEngineSettings) SetProjectEngineSettings(ctx context.Context, in SetProjectEngineSettingsInput) (domain.ProjectEngineSettings, []string, error) {
	// auth checks etc.
	// return settings, warnings, err
	s := domain.ProjectEngineSettings{
		ProjectID:  in.ProjectID,
		Engine:     domain.EngineName(in.Engine),
		MinVersion: in.MinVersion,
	}
	res, err := uc.repo.Upsert(ctx, s, in.ExpectedVersion)
	return res, []string{"preflight_not_run"}, err
}
