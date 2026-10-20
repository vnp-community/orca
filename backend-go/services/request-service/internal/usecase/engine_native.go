package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/stablyai/orca-go/prompts/output_language"
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
	settings  AnalysisSettings
}

// NativeEngineOption configures NewNativeEngine.
type NativeEngineOption func(*nativeEngine)

// WithOutputLanguage sets the language the model writes its options in; without it the prompt has no language instruction.
func WithOutputLanguage(l output_language.Language) NativeEngineOption {
	return func(e *nativeEngine) { e.settings.OutputLanguage = l }
}

func NewNativeEngine(completer AICompleter, planGen PlanGenerator, opts ...NativeEngineOption) SolutionEngine {
	e := &nativeEngine{
		completer: completer,
		planGen:   planGen,
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

func (e *nativeEngine) Name() domain.EngineName {
	return domain.EngineNative
}

func (e *nativeEngine) Preflight(ctx context.Context, p ProjectRef, settings *domain.ProjectEngineSettings) (PreflightReport, error) {
	return PreflightReport{OK: true, CheckedAt: time.Now().UTC()}, nil
}

// GenerateAnalysis asks the model for solution options with the same prompt and validation as the background worker,
// so engine "native" and the direct flow cannot drift apart. One corrective retry, like the worker.
func (e *nativeEngine) GenerateAnalysis(ctx context.Context, p ProjectRef, in AnalysisInput) (AnalysisOutput, error) {
	if e.completer == nil {
		return AnalysisOutput{}, errors.New("native engine has no AI completer")
	}
	minOptions := domain.MinOptionsFor(in.Request.Type)
	prompt := SolutionPromptInput{
		Kind: domain.SolutionKindSolution, Request: in.Request, MinOptions: minOptions, PriorArtifacts: in.PriorArtifacts, Feedback: in.Feedback,
		Language: e.settings.OutputLanguageFor(in.Request),
	}
	var raw string
	for attempt := 0; attempt < 2; attempt++ {
		rendered, perr := RenderSolutionPrompt(prompt)
		if perr != nil {
			return AnalysisOutput{Raw: raw}, perr
		}
		text, err := e.completer.Complete(ctx, rendered)
		if err != nil {
			return AnalysisOutput{Raw: raw}, err
		}
		raw = text
		doc, verr := parseAnalysisOutput(domain.SolutionKindSolution, text, minOptions)
		if verr == nil {
			return AnalysisOutput{OptionsJSON: doc, Raw: raw}, nil
		}
		prompt.RetryNote = verr.Error()
	}
	return AnalysisOutput{Raw: raw}, fmt.Errorf("%w: the model returned no valid options after a retry", domain.ErrSolutionOptionsInvalid)
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
