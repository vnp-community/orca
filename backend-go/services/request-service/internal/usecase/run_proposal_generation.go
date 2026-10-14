package usecase

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// ProposalEngine is the part of the openspec engine the worker drives: one authoring attempt in a (possibly reused) worktree.
type ProposalEngine interface {
	GenerateProposal(ctx context.Context, in ProposalInput) (ProposalResult, error)
}

// BundleChecker looks at the finished change for faults a parse of tasks.md cannot see (T2.5). Nil means no extra check.
type BundleChecker interface {
	Check(ctx context.Context, in BundleCheckInput) ([]BundleFinding, error)
}

type BundleCheckInput struct {
	Request   domain.Request
	Result    ProposalResult
	Proposal  domain.PlanProposal
	Workspace ProposalWorkspace
}

// BundleFinding is one fault in the change, worded so the agent can fix it on a retry.
type BundleFinding struct {
	Code    string
	Message string
}

// RunProposalGeneration is the worker of agent_proposal plan runs: an agent writes an OpenSpec change, and the Plan is read
// from its tasks.md. It writes nothing to task-service; a person reviews the proposal and CommitPlan writes it.
type RunProposalGeneration struct {
	requests  RequestRepository
	solutions SolutionStore
	registry  EngineRegistry
	projects  ProjectContextReader
	policies  PlanPolicies
	settings  EngineSettingsRepository
	checker   BundleChecker
	workspace ProposalWorkspace
	runs      AnalysisRunStore
	tx        TxRunner
	limits    domain.PlanLimits
	owner     string
	now       func() time.Time
}

var _ AnalysisWorker = (*RunProposalGeneration)(nil)

type RunProposalGenerationDeps struct {
	Requests  RequestRepository
	Solutions SolutionStore
	// Registry holds the openspec engine; it must also implement ProposalEngine.
	Registry EngineRegistry
	Projects ProjectContextReader
	Policies PlanPolicies
	// EngineSettings may be nil; the readiness check then uses the server's default minimum version.
	EngineSettings EngineSettingsRepository
	Checker        BundleChecker
	Workspace      ProposalWorkspace
	Runs           AnalysisRunStore
	Tx             TxRunner
	Limits         domain.PlanLimits
	Owner          string
}

func NewRunProposalGeneration(d RunProposalGenerationDeps) *RunProposalGeneration {
	return &RunProposalGeneration{requests: d.Requests, solutions: d.Solutions, registry: d.Registry, projects: d.Projects, policies: d.Policies,
		settings: d.EngineSettings, checker: d.Checker, workspace: d.Workspace, runs: d.Runs, tx: d.Tx, limits: d.Limits, owner: d.Owner, now: func() time.Time { return time.Now().UTC() }}
}

func (w *RunProposalGeneration) Execute(ctx context.Context, run domain.AnalysisRun) error {
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

	in := ProposalInput{Run: run, Request: req, Shape: shape, Limits: w.limits, Feedback: run.Feedback, Settings: w.engineSettings(ctx, req.ProjectID)}
	in.ProjectName, in.RepoURL = projectFacts(ctx, w.projects, req.ProjectID)
	if sol, ok := latestApprovedAnalysis(ctx, w.solutions, req.ID); ok {
		in.Analysis = approvedAnalysisText(sol)
	}
	policy := w.policies.PolicyFor(req.Type)

	engine, err := w.proposalEngine()
	if err != nil {
		return newFailure(run, RunErrEngineUnavailable, err.Error())
	}
	for {
		res, err := engine.GenerateProposal(ctx, in)
		if err != nil {
			return err
		}
		in.Workspace = &res.Workspace // the retry fixes the files in place
		proposal, problems, fatal := w.assemble(ctx, req, shape, policy, res)
		if fatal != nil {
			f := newFailure(run, errorCode(fatal), fatal.Error())
			f.Raw = res.TasksMD
			return f
		}
		if len(problems) == 0 {
			return w.finish(ctx, run, proposal)
		}
		if rerr := run.RecordAttempt(); rerr != nil {
			f := newFailure(run, domain.RunErrSpecInvalidOutput, "the agent did not leave a valid change after a retry: "+problems[0])
			f.Raw = strings.Join(problems, "\n")
			return f
		}
		in.RetryNote = strings.Join(problems, "\n")
	}
}

// proposalEngine finds the openspec engine in the registry; a server that did not register one cannot run these runs.
func (w *RunProposalGeneration) proposalEngine() (ProposalEngine, error) {
	if w.registry == nil {
		return nil, fmt.Errorf("this server has no engine registry")
	}
	e, err := w.registry.For(domain.EngineOpenSpec)
	if err != nil {
		return nil, fmt.Errorf("this server has no worker for the openspec engine")
	}
	pe, ok := e.(ProposalEngine)
	if !ok {
		return nil, fmt.Errorf("the registered openspec engine cannot author a change")
	}
	return pe, nil
}

func (w *RunProposalGeneration) engineSettings(ctx context.Context, projectID string) *domain.ProjectEngineSettings {
	if w.settings == nil {
		return nil
	}
	s, found, err := w.settings.Get(ctx, projectID)
	if err != nil || !found {
		return nil
	}
	return &s
}

// assemble turns the change into a validated PlanProposal, or returns what is wrong in words the agent can act on.
// fatal is a problem a retry cannot fix (the project's state, not the agent's output).
func (w *RunProposalGeneration) assemble(ctx context.Context, req domain.Request, shape domain.PlanShape, policy domain.TypePolicy, res ProposalResult) (proposal domain.PlanProposal, problems []string, fatal error) {
	proposal, violations := domain.ParsePlanRegion(res.TasksMD, res.Ref)
	if len(violations) > 0 {
		return domain.PlanProposal{}, violationLines(violations), nil
	}
	proposal.Title = req.Title
	proposal.Summary = "OpenSpec change " + res.ChangeID
	proposal = domain.NormalizePlanProposal(proposal)
	if err := domain.ValidatePlanProposal(req.Type, shape, w.limits, proposal); err != nil {
		if !planErrorIsModelFault(err) {
			return domain.PlanProposal{}, nil, err
		}
		return domain.PlanProposal{}, []string{err.Error()}, nil
	}
	if err := policy.PlanPreconditions(ctx, req, proposal); err != nil {
		if !planErrorIsModelFault(err) {
			return domain.PlanProposal{}, nil, err
		}
		return domain.PlanProposal{}, []string{err.Error()}, nil
	}
	if w.checker != nil {
		findings, err := w.checker.Check(ctx, BundleCheckInput{Request: req, Result: res, Proposal: proposal, Workspace: w.workspace})
		if err != nil {
			return domain.PlanProposal{}, []string{"the change could not be checked: " + err.Error()}, nil
		}
		if len(findings) > 0 {
			out := make([]string, len(findings))
			for i, f := range findings {
				out[i] = f.Code + ": " + f.Message
			}
			return domain.PlanProposal{}, out, nil
		}
	}
	return proposal, nil, nil
}

func violationLines(v []domain.Violation) []string {
	out := make([]string, 0, len(v))
	for _, x := range v {
		out = append(out, fmt.Sprintf("tasks.md line %d %s: %s", x.Line, x.Code, x.Message))
		if len(out) == 20 {
			break
		}
	}
	return out
}

func (w *RunProposalGeneration) finish(ctx context.Context, run domain.AnalysisRun, p domain.PlanProposal) error {
	encoded, err := EncodePlanProposal(p)
	if err != nil {
		return err
	}
	return finishPlanRun(ctx, w.tx, w.runs, w.owner, w.now(), run, encoded)
}
