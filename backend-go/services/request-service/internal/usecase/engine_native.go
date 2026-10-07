package usecase

import (
	"context"
	"time"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

type AICompleter interface {
	Complete(ctx context.Context, prompt string) (string, error)
}

type PlanGenerator interface {
	Generate(ctx context.Context, req domain.Request) (string, error)
}

type nativeEngine struct {
	completer AICompleter
	planGen   PlanGenerator
}

func NewNativeEngine(completer AICompleter, planGen PlanGenerator) SolutionEngine {
	return &nativeEngine{
		completer: completer,
		planGen:   planGen,
	}
}

func (e *nativeEngine) Name() domain.EngineName {
	return domain.EngineNative
}

func (e *nativeEngine) Preflight(ctx context.Context, p ProjectRef, settings *domain.ProjectEngineSettings) (PreflightReport, error) {
	return PreflightReport{OK: true, CheckedAt: time.Now().UTC()}, nil
}

func (e *nativeEngine) GenerateAnalysis(ctx context.Context, p ProjectRef, in AnalysisInput) (AnalysisOutput, error) {
	// Stub implementation to be filled when SOL-007 is fully merged
	return AnalysisOutput{}, nil
}

func (e *nativeEngine) GeneratePlan(ctx context.Context, p ProjectRef, in PlanInput) (string, error) {
	if e.planGen != nil {
		return e.planGen.Generate(ctx, in.Request)
	}
	return "", nil
}

func (e *nativeEngine) OnRequestCompleted(ctx context.Context, p ProjectRef, in CompletionInput) error {
	return nil
}
