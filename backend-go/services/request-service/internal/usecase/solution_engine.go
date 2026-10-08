package usecase

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

type ProjectRef struct {
	TenantID  string
	ProjectID string
	RepoID    string
}

// PriorArtifact is an earlier Solution of the same Request, shown to the model so a regenerated answer does not repeat a rejected one.
type PriorArtifact struct {
	Kind     string
	Status   string
	Content  string
	Feedback string
}

type AnalysisInput struct {
	Request        domain.Request
	PriorArtifacts []PriorArtifact
	Feedback       string
	RunID          string
	Attempt        int
}

type AnalysisOutput struct {
	OptionsJSON []byte
	Raw         string
	Provenance  json.RawMessage // Use json.RawMessage until domain.Provenance is ready
	Model       string
}

type PlanInput struct {
	Request  domain.Request
	Solution *domain.Solution
	Feedback string
}

type CompletionInput struct {
	Request domain.Request
}

type SolutionEngine interface {
	Name() domain.EngineName
	Preflight(ctx context.Context, p ProjectRef, settings *domain.ProjectEngineSettings) (PreflightReport, error)
	GenerateAnalysis(ctx context.Context, p ProjectRef, in AnalysisInput) (AnalysisOutput, error)
	GeneratePlan(ctx context.Context, p ProjectRef, in PlanInput) (string, error) // PlanRaw
	OnRequestCompleted(ctx context.Context, p ProjectRef, in CompletionInput) error
}

type EngineRegistry interface {
	For(name domain.EngineName) (SolutionEngine, error)
}

type DefaultEngineRegistry struct {
	engines map[domain.EngineName]SolutionEngine
}

func NewDefaultEngineRegistry() *DefaultEngineRegistry {
	return &DefaultEngineRegistry{
		engines: make(map[domain.EngineName]SolutionEngine),
	}
}

func (r *DefaultEngineRegistry) Register(engine SolutionEngine) {
	r.engines[engine.Name()] = engine
}

func (r *DefaultEngineRegistry) For(name domain.EngineName) (SolutionEngine, error) {
	if e, ok := r.engines[name]; ok {
		return e, nil
	}
	return nil, fmt.Errorf("engine not found: %s", name)
}
