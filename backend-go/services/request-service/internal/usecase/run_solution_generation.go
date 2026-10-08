package usecase

import (
	"context"
	"errors"
	"log/slog"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// RunSolutionGeneration is the ai.complete worker: prompt, one call (plus one corrective retry), validate, commit.
// It also serves the agent kinds when the caller asked for analysis_mode=COMPLETE.
type RunSolutionGeneration struct {
	requests  RequestRepository
	solutions SolutionStore
	completer ProjectAICompleter
	projects  ProjectContextReader
	writer    *AnalysisResultWriter
	settings  AnalysisSettings
	coverage  SolutionArtifacts
}

// WithCoverage makes a solution document that leaves an active acceptance criterion uncovered count as invalid,
// so the model gets the corrective retry instead of the reviewer getting an incomplete proposal.
func (g *RunSolutionGeneration) WithCoverage(a SolutionArtifacts) *RunSolutionGeneration {
	g.coverage = a
	return g
}

var _ AnalysisWorker = (*RunSolutionGeneration)(nil)

func NewRunSolutionGeneration(requests RequestRepository, solutions SolutionStore, completer ProjectAICompleter, projects ProjectContextReader,
	writer *AnalysisResultWriter, settings AnalysisSettings) *RunSolutionGeneration {
	return &RunSolutionGeneration{requests: requests, solutions: solutions, completer: completer, projects: projects, writer: writer, settings: settings.withDefaults()}
}

func (g *RunSolutionGeneration) Execute(ctx context.Context, run domain.AnalysisRun) error {
	req, err := g.requests.Get(ctx, run.RequestID)
	if err != nil {
		return err
	}
	kind := domain.SolutionKind(run.Kind)
	in := SolutionPromptInput{
		Kind: kind, Request: req, MinOptions: domain.MinOptionsFor(req.Type), Feedback: run.Feedback,
		PriorArtifacts: priorArtifacts(ctx, g.solutions, req.ID, kind, run.SolutionID),
	}
	in.ProjectName, in.RepoURL = projectFacts(ctx, g.projects, req.ProjectID)

	var lastRaw string
	for {
		text, err := g.complete(ctx, req.ProjectID, BuildSolutionPrompt(in))
		if err != nil {
			f := completionFailure(run, err)
			f.Raw = lastRaw
			return f
		}
		lastRaw = text
		doc, verr := parseAnalysisOutput(kind, text, in.MinOptions)
		if verr == nil && g.coverage != nil && kind == domain.SolutionKindSolution {
			verr = g.coverage.CheckCoverage(req, doc)
		}
		if verr == nil {
			return g.writer.Persist(ctx, PersistAnalysisInput{Run: run, Doc: doc, Raw: text})
		}
		if rerr := run.RecordAttempt(); rerr != nil {
			f := newFailure(run, domain.RunErrInvalidOutput, "the model did not return a valid document after a retry: "+verr.Error())
			f.Raw = lastRaw
			return f
		}
		in.RetryNote = verr.Error()
	}
}

func (g *RunSolutionGeneration) complete(ctx context.Context, projectID, prompt string) (string, error) {
	cctx, cancel := context.WithTimeout(ctx, g.settings.AICompleteTimeout)
	defer cancel()
	return g.completer.Complete(cctx, projectID, prompt)
}

func completionFailure(run domain.AnalysisRun, err error) *AnalysisFailure {
	switch {
	case errors.Is(err, ErrNoDevServer):
		return newFailure(run, domain.RunErrNoConnection, "no dev server could run the AI call")
	case errors.Is(err, ErrClassifierTimeout), errors.Is(err, context.DeadlineExceeded):
		return newFailure(run, domain.RunErrAITimeout, "the AI call timed out")
	}
	slog.Warn("ai.complete failed", slog.String("run_id", run.ID), slog.Any("error", err))
	return newFailure(run, domain.RunErrAIFailed, "the AI call failed")
}

// parseAnalysisOutput turns model text into the stored document: extract, validate against the kind's schema, redact.
func parseAnalysisOutput(kind domain.SolutionKind, text string, minOptions int) ([]byte, error) {
	obj, err := ExtractJSONObject(text)
	if err != nil {
		return nil, err
	}
	doc, err := ValidateByKind(kind, obj, minOptions)
	if err != nil {
		return nil, err
	}
	redacted, _, err := RedactDocument(doc)
	return redacted, err
}

// priorArtifacts are the superseded and rejected attempts shown to the model; the draft being filled is excluded.
func priorArtifacts(ctx context.Context, store SolutionStore, requestID string, kind domain.SolutionKind, exceptID string) []PriorArtifact {
	all, err := store.ListByRequest(ctx, SolutionListFilter{RequestID: requestID})
	if err != nil {
		return nil // best effort: a missing history only costs quality
	}
	var out []PriorArtifact
	for _, s := range all {
		if s.ID == exceptID || (s.Status != domain.SolutionStatusSuperseded && s.Status != domain.SolutionStatusRejected) {
			continue
		}
		out = append(out, PriorArtifact{Kind: string(s.Kind), Status: string(s.Status), Content: string(s.OptionsJSON)})
	}
	return out
}

func projectFacts(ctx context.Context, projects ProjectContextReader, projectID string) (name, repoURL string) {
	if projects == nil {
		return "", ""
	}
	pc, err := projects.Read(ctx, projectID)
	if err != nil {
		return "", ""
	}
	return pc.Name, pc.RepoURL
}
