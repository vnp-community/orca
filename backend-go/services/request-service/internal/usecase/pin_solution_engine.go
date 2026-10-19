package usecase

import (
	"context"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// EnginePinner decides, once per Request, which engine produces its Solution and Plan.
type EnginePinner interface {
	Execute(ctx context.Context, requestID string) (domain.EngineName, error)
}

// PinSolutionEngine fixes the engine of a Request at its first generation, so a later change of the project setting
// never switches a Request half-way: the Plan must come from the engine that produced the Solution.
type PinSolutionEngine struct {
	reqRepo RequestRepository
	setRepo EngineSettingsRepository
}

var _ EnginePinner = (*PinSolutionEngine)(nil)

func NewPinSolutionEngine(reqRepo RequestRepository, setRepo EngineSettingsRepository) *PinSolutionEngine {
	return &PinSolutionEngine{reqRepo: reqRepo, setRepo: setRepo}
}

func (uc *PinSolutionEngine) Execute(ctx context.Context, requestID string) (domain.EngineName, error) {
	req, err := uc.reqRepo.Get(ctx, requestID)
	if err != nil {
		return "", err
	}
	if req.SolutionEngine != nil {
		return *req.SolutionEngine, nil
	}
	var settings *domain.ProjectEngineSettings
	s, found, err := uc.setRepo.Get(ctx, req.ProjectID)
	if err != nil {
		return "", err
	}
	if found {
		settings = &s
	}
	engine := domain.EffectiveEngine(nil, settings, domain.OpenSpecProfileFor(req.Type))
	// Atomic and versionless: the first writer wins and every caller gets the same answer.
	return uc.reqRepo.PinSolutionEngineIfUnset(ctx, req.ID, engine)
}
