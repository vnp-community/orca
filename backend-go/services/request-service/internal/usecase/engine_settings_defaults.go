package usecase

import (
	"context"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// DefaultedEngineSettings answers for a project that has no saved settings with the deployment's defaults, so the engine and
// the agent-written Solution can be on for every project without a row each. A saved row always wins, and a write goes
// to the store untouched (a defaulted project reads as version 0, which is what its first write expects).
type DefaultedEngineSettings struct {
	EngineSettingsRepository
	defaults domain.EngineDefaults
}

func NewDefaultedEngineSettings(inner EngineSettingsRepository, defaults domain.EngineDefaults) EngineSettingsRepository {
	if defaults.Engine == domain.EngineNative && !defaults.SolutionAgent && defaults.ExecutionMode.OrDefault() == domain.ExecutionStepByStep {
		return inner
	}
	return &DefaultedEngineSettings{EngineSettingsRepository: inner, defaults: defaults}
}

func (d *DefaultedEngineSettings) Get(ctx context.Context, projectID string) (domain.ProjectEngineSettings, bool, error) {
	s, found, err := d.EngineSettingsRepository.Get(ctx, projectID)
	if err != nil || found {
		return s, found, err
	}
	return domain.ProjectEngineSettings{ProjectID: projectID, Engine: d.defaults.Engine, SolutionAgent: d.defaults.SolutionAgent, ExecutionMode: d.defaults.ExecutionMode.OrDefault(), PlanAutoGenerate: true, FromDeploymentDefault: true}, true, nil
}
