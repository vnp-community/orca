package usecase

import (
	"context"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

type PinSolutionEngine struct {
	reqRepo RequestRepository
	setRepo EngineSettingsRepository
}

func NewPinSolutionEngine(reqRepo RequestRepository, setRepo EngineSettingsRepository) *PinSolutionEngine {
	return &PinSolutionEngine{reqRepo: reqRepo, setRepo: setRepo}
}

func (uc *PinSolutionEngine) Execute(ctx context.Context, requestID string) (domain.EngineName, error) {
	// Dummy implementation for now
	return domain.EngineNative, nil
}
