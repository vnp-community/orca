package usecase

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// RunPlanGeneration is the background worker of GeneratePlan: prompt, one AI call (plus one corrective retry),
// validate, and store the proposal on the run. It writes nothing to task-service; a person reviews the proposal
// and CommitPlan writes it.
type RunPlanGeneration struct {
	requests  RequestRepository
	solutions SolutionStore
	completer ProjectAICompleter
	projects  ProjectContextReader
	policies  PlanPolicies
	runs      AnalysisRunStore
	tx        TxRunner
	limits    domain.PlanLimits
	settings  AnalysisSettings
	owner     string
	now       func() time.Time
}

var _ AnalysisWorker = (*RunPlanGeneration)(nil)

type RunPlanGenerationDeps struct {
	Requests  RequestRepository
	Solutions SolutionStore
	Completer ProjectAICompleter
	Projects  ProjectContextReader
	Policies  PlanPolicies
	Runs      AnalysisRunStore
	Tx        TxRunner
	Limits    domain.PlanLimits
	Settings  AnalysisSettings
	Owner     string
}

func NewRunPlanGeneration(d RunPlanGenerationDeps) *RunPlanGeneration {
	return &RunPlanGeneration{requests: d.Requests, solutions: d.Solutions, completer: d.Completer, projects: d.Projects, policies: d.Policies,
		runs: d.Runs, tx: d.Tx, limits: d.Limits, settings: d.Settings.withDefaults(), owner: d.Owner, now: func() time.Time { return time.Now().UTC() }}
}

// Run codes beyond the shared REQUEST_AI_* ones.
const (
	planRunMoved = "REQUEST_PLAN_REQUEST_MOVED"
)

func (w *RunPlanGeneration) Execute(ctx context.Context, run domain.AnalysisRun) error {
	req, err := w.requests.Get(ctx, run.RequestID)
	if err != nil {
		return err
	}
	if req.Status != domain.RequestStatusPlanning {
		return newFailure(run, planRunMoved, "request left planning while the Plan was being written")
	}
	flow, err := domain.FlowFor(req.Type)
	if err != nil {
		return err
	}
	shape := domain.PlanShapeFor(flow, req.Size)
	if shape == domain.ShapeNone {
		return newFailure(run, "REQUEST_PLAN_NOT_APPLICABLE", "this request type has no Plan")
	}

	in := PlanPromptInput{Request: req, Shape: shape, Limits: w.limits, Feedback: run.Feedback, Language: w.settings.OutputLanguageFor(req)}
	in.ProjectName, in.RepoURL = projectFacts(ctx, w.projects, req.ProjectID)
	prior, err := priorPlanText(ctx, w.runs, req.ID)
	if err != nil {
		return err
	}
	in.PriorPlan = prior
	if sol, ok := latestApprovedAnalysis(ctx, w.solutions, req.ID); ok {
		in.Analysis = approvedAnalysisText(sol)
	}
	policy := w.policies.PolicyFor(req.Type)

	var lastRaw string
	for {
		prompt, perr := RenderPlanPrompt(in)
		if perr != nil {
			return promptBuildFailure(run, perr)
		}
		text, err := w.complete(ctx, req.ProjectID, prompt)
		if err != nil {
			f := planCompletionFailure(run, err)
			f.Raw = lastRaw
			return f
		}
		lastRaw = text
		proposal, verr := ParsePlanOutput(text)
		if verr == nil {
			verr = domain.ValidatePlanProposal(req.Type, shape, w.limits, proposal)
		}
		if verr == nil {
			verr = policy.PlanPreconditions(ctx, req, proposal)
		}
		if verr == nil {
			return w.finish(ctx, run, proposal)
		}
		if !planErrorIsModelFault(verr) {
			f := newFailure(run, errorCode(verr), verr.Error())
			f.Raw = lastRaw
			return f
		}
		if rerr := run.RecordAttempt(); rerr != nil {
			f := newFailure(run, "REQUEST_PLAN_AI_INVALID_JSON", "the model did not return a valid Plan after a retry: "+verr.Error())
			f.Raw = lastRaw
			return f
		}
		in.RetryNote = verr.Error()
	}
}

func (w *RunPlanGeneration) complete(ctx context.Context, projectID, prompt string) (string, error) {
	cctx, cancel := context.WithTimeout(ctx, w.settings.AICompleteTimeout)
	defer cancel()
	return w.completer.Complete(cctx, projectID, prompt)
}

// finish stores the redacted proposal as the run's result; losing the lease means another instance owns the run.
func (w *RunPlanGeneration) finish(ctx context.Context, run domain.AnalysisRun, p domain.PlanProposal) error {
	encoded, err := EncodePlanProposal(p)
	if err != nil {
		return err
	}
	return finishPlanRun(ctx, w.tx, w.runs, w.owner, w.now(), run, encoded)
}

// finishPlanRun records the encoded proposal on the run in one transaction; both plan workers end here.
func finishPlanRun(ctx context.Context, tx TxRunner, runs AnalysisRunStore, owner string, at time.Time, run domain.AnalysisRun, encoded string) error {
	return tx.InTx(ctx, func(ctx context.Context) error {
		done := run
		done.Succeed(at)
		done.RawOutput = &encoded
		ok, err := runs.FinishOwned(ctx, done, owner)
		if err != nil {
			return err
		}
		if !ok {
			return ErrLeaseLost
		}
		return nil
	})
}

// planErrorIsModelFault says whether another AI attempt could fix the problem. A missing baseline check is the
// project's state, not the model's output, so it fails the run at once.
func planErrorIsModelFault(err error) bool {
	code := errorCode(err)
	if code == "" {
		return true // a parse error from ParsePlanOutput
	}
	return len(code) >= len("REQUEST_PLAN_") && code[:len("REQUEST_PLAN_")] == "REQUEST_PLAN_"
}

func planCompletionFailure(run domain.AnalysisRun, err error) *AnalysisFailure {
	var ai *AIUnavailableError
	if errors.As(err, &ai) {
		return newFailure(run, ai.Code, ai.Detail)
	}
	switch {
	case errors.Is(err, ErrNoDevServer):
		return newFailure(run, "REQUEST_PLAN_AI_UNAVAILABLE", "no dev server could run the AI call")
	case errors.Is(err, ErrClassifierTimeout), errors.Is(err, context.DeadlineExceeded):
		return newFailure(run, "REQUEST_AI_COMPLETE_TIMEOUT", "the AI call timed out")
	}
	slog.Warn("plan ai.complete failed", slog.String("run_id", run.ID), slog.Any("error", err))
	return newFailure(run, "REQUEST_PLAN_AI_UNAVAILABLE", "the AI call failed")
}

// latestApprovedAnalysis is the newest approved Solution or diagnosis of the Request.
func latestApprovedAnalysis(ctx context.Context, store SolutionStore, requestID string) (domain.Solution, bool) {
	all, err := store.ListByRequest(ctx, SolutionListFilter{RequestID: requestID, Status: domain.SolutionStatusApproved})
	if err != nil {
		return domain.Solution{}, false
	}
	for i := len(all) - 1; i >= 0; i-- {
		if all[i].Kind == domain.SolutionKindSolution || all[i].Kind == domain.SolutionKindDiagnosis {
			return all[i], true
		}
	}
	return domain.Solution{}, false
}
