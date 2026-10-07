package usecase

import (
	"context"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

type openspecEngine struct {
	gate *EngineReadinessGate
	ws   ProposalWorkspace
}

func NewOpenSpecEngine(gate *EngineReadinessGate, ws ProposalWorkspace) SolutionEngine {
	return &openspecEngine{gate: gate, ws: ws}
}

func (e *openspecEngine) Name() domain.EngineName {
	return domain.EngineOpenSpec
}

func (e *openspecEngine) Preflight(ctx context.Context, p ProjectRef, settings *domain.ProjectEngineSettings) (PreflightReport, error) {
	return e.gate.Check(ctx, p.ProjectID, settings)
}

func (e *openspecEngine) GenerateAnalysis(ctx context.Context, p ProjectRef, in AnalysisInput) (AnalysisOutput, error) {
	return AnalysisOutput{}, nil // Stub implementation
}

func (e *openspecEngine) GeneratePlan(ctx context.Context, p ProjectRef, in PlanInput) (string, error) {
	return "", nil
}

func (e *openspecEngine) OnRequestCompleted(ctx context.Context, p ProjectRef, in CompletionInput) error {
	return nil
}
