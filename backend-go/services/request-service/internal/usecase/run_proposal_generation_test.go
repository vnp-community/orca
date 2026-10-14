package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// scriptedEngine plays the openspec engine: one scripted answer per attempt.
type scriptedEngine struct {
	answers []ProposalResult
	errs    []error
	inputs  []ProposalInput
}

func (e *scriptedEngine) GenerateProposal(_ context.Context, in ProposalInput) (ProposalResult, error) {
	n := len(e.inputs)
	e.inputs = append(e.inputs, in)
	if n < len(e.errs) && e.errs[n] != nil {
		return ProposalResult{}, e.errs[n]
	}
	if n < len(e.answers) {
		return e.answers[n], nil
	}
	return e.answers[len(e.answers)-1], nil
}

func (e *scriptedEngine) Name() domain.EngineName { return domain.EngineOpenSpec }
func (e *scriptedEngine) Preflight(context.Context, ProjectRef, *domain.ProjectEngineSettings) (PreflightReport, error) {
	return PreflightReport{OK: true}, nil
}
func (e *scriptedEngine) GenerateAnalysis(context.Context, ProjectRef, AnalysisInput) (AnalysisOutput, error) {
	return AnalysisOutput{}, ErrEngineStepNotSupported
}
func (e *scriptedEngine) GeneratePlan(context.Context, ProjectRef, PlanInput) (string, error) {
	return "", ErrEngineStepNotSupported
}
func (e *scriptedEngine) OnRequestCompleted(context.Context, ProjectRef, CompletionInput) error {
	return nil
}

func registryWith(e SolutionEngine) *DefaultEngineRegistry {
	r := NewDefaultEngineRegistry()
	r.Register(e)
	return r
}

func answer(md string) ProposalResult {
	return ProposalResult{ChangeID: "req-1", Ref: "REQ-1", TasksMD: md, Workspace: Workspace{WorktreeID: "wt-1", Path: "/srv/wt"}}
}

func tasksDoc(body string) string {
	return "<!-- orca:begin plan request=REQ-1 schema=1 -->\n" + body + "<!-- orca:end plan -->\n"
}

const twoPhaseBody = "## PH-1 Nền\n- [ ] T1.1 Dựng client [type=feature] [h=2] [ac=AC-1]\n  Serves: FR-1\n- [ ] T1.2 Thêm retry [depends=T1.1]\n## PH-2 Dịch vụ\n- [ ] T2.1 Nối service [depends=T1.2]\n"

type stubChecker struct {
	findings [][]BundleFinding
	calls    int
	err      error
}

func (c *stubChecker) Check(context.Context, BundleCheckInput) ([]BundleFinding, error) {
	n := c.calls
	c.calls++
	if c.err != nil {
		return nil, c.err
	}
	if n < len(c.findings) {
		return c.findings[n], nil
	}
	return nil, nil
}

func proposalKit(t *testing.T, eng *scriptedEngine, checker BundleChecker) (*planKit, domain.AnalysisRun, *RunProposalGeneration) {
	t.Helper()
	k := newPlanKit(fakeConns{conn: AnalysisConnection{ConnectionID: "c", RepoPath: "/srv/repo"}})
	req := k.w.seedRequest(domain.RequestTypeChangeRequest, domain.RequestStatusPlanning)
	r := k.w.requests[req.ID]
	r.Size, r.Number = domain.RequestSizeM, 1
	k.w.requests[req.ID] = r
	k.w.seedApprovedSolution(req.ID, domain.SolutionKindSolution)
	k.generate = NewGeneratePlan(GeneratePlanDeps{
		Requests: k.w, Solutions: fakeSolutions{k.w}, Runs: fakeRuns{k.w}, Conns: fakeConns{conn: AnalysisConnection{ConnectionID: "c", RepoPath: "/srv/repo"}},
		Auth: NewPlanActorAuthorizer(allowAll{}), Approvals: fakeCanceller{k.w}, Transition: k.w.transitioner(), Tx: k.w, Spawner: k.spawner,
		LeaseOwner: testOwner, Pin: &stubPinner{engine: domain.EngineOpenSpec},
	})
	run := k.startedRun(req.ID)
	if run.Mode != domain.AnalysisModeAgentProposal {
		t.Fatalf("setup: run mode %s", run.Mode)
	}
	w := NewRunProposalGeneration(RunProposalGenerationDeps{
		Requests: k.w, Solutions: fakeSolutions{k.w}, Registry: registryWith(eng), Policies: fakePolicies{domain.NewPolicyRegistry(domain.PolicyDeps{})},
		Checker: checker, Runs: fakeRuns{k.w}, Tx: k.w, Limits: domain.DefaultPlanLimits, Owner: testOwner,
	})
	return k, run, w
}

func TestRunProposalGeneration_StoresTheProposalReadFromTasksMD(t *testing.T) {
	eng := &scriptedEngine{answers: []ProposalResult{answer(tasksDoc(twoPhaseBody))}}
	k, run, w := proposalKit(t, eng, nil)
	if err := w.Execute(k.w.ctx(), run); err != nil {
		t.Fatalf("execute: %v", err)
	}
	got := k.w.runs[run.ID]
	if got.Status != domain.RunStatusSucceeded || got.RawOutput == nil {
		t.Fatalf("run = %s", describe(got))
	}
	p, err := DecodePlanProposal(*got.RawOutput)
	if err != nil || len(p.Phases) != 2 || len(p.Phases[0].Tasks) != 2 || p.Phases[1].Tasks[0].Title != "Nối service" {
		t.Fatalf("stored proposal: %+v %v", p, err)
	}
	if p.Phases[0].Tasks[1].DependsOnIndices[0] != 0 || p.Phases[1].DependsOnPhaseIndices[0] != 0 {
		t.Fatalf("dependencies were lost: %+v", p)
	}
	if !strings.Contains(p.Phases[0].Tasks[0].Description, "Serves: FR-1") {
		t.Fatalf("the task contract must reach the description: %q", p.Phases[0].Tasks[0].Description)
	}
	if eng.inputs[0].Analysis == "" || !strings.Contains(eng.inputs[0].Analysis, "Use a cache") || eng.inputs[0].Shape != domain.ShapePlanPhases {
		t.Fatalf("engine input = %+v", eng.inputs[0])
	}
}

func TestRunProposalGeneration_RetriesOnceInTheSameWorktreeWithTheViolations(t *testing.T) {
	bad := tasksDoc("## PH-1 Nền\n- [ ] T1.1 Việc\nghi chú lạc\n")
	eng := &scriptedEngine{answers: []ProposalResult{answer(bad), answer(tasksDoc(twoPhaseBody))}}
	k, run, w := proposalKit(t, eng, nil)
	if err := w.Execute(k.w.ctx(), run); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(eng.inputs) != 2 {
		t.Fatalf("attempts = %d", len(eng.inputs))
	}
	if eng.inputs[0].Workspace != nil || eng.inputs[1].Workspace == nil || eng.inputs[1].Workspace.WorktreeID != "wt-1" {
		t.Fatalf("the retry must reuse the first attempt's worktree: %+v / %+v", eng.inputs[0].Workspace, eng.inputs[1].Workspace)
	}
	if !strings.Contains(eng.inputs[1].RetryNote, "TASKSMD_UNKNOWN_LINE") || !strings.Contains(eng.inputs[1].RetryNote, "line 4") {
		t.Fatalf("retry note = %q", eng.inputs[1].RetryNote)
	}
	if k.w.runs[run.ID].Status != domain.RunStatusSucceeded {
		t.Fatalf("run = %s", describe(k.w.runs[run.ID]))
	}
}

func TestRunProposalGeneration_GivesUpAfterTheRetryAndKeepsTheRawFile(t *testing.T) {
	bad := tasksDoc("## PH-1 Nền\nghi chú lạc\n")
	eng := &scriptedEngine{answers: []ProposalResult{answer(bad)}}
	k, run, w := proposalKit(t, eng, nil)
	err := w.Execute(k.w.ctx(), run)
	f, ok := asFailure(err)
	if !ok || f.Code != domain.RunErrSpecInvalidOutput || !strings.Contains(f.Raw, "TASKSMD_UNKNOWN_LINE") || len(eng.inputs) != 2 {
		t.Fatalf("want one retry then an invalid-output failure, got %v attempts=%d", err, len(eng.inputs))
	}
	if k.w.runs[run.ID].Status == domain.RunStatusSucceeded {
		t.Fatal("a failed run must not read as succeeded")
	}
}

func TestRunProposalGeneration_ModelFaultsInValidationAreRetried(t *testing.T) {
	// A phased Request whose tasks.md has no phases is the agent's mistake, fixed on the second attempt.
	flat := tasksDoc("## TASKS\n- [ ] T1.1 Việc\n")
	eng := &scriptedEngine{answers: []ProposalResult{answer(flat), answer(tasksDoc(twoPhaseBody))}}
	k, run, w := proposalKit(t, eng, nil)
	if err := w.Execute(k.w.ctx(), run); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(eng.inputs) != 2 || !strings.Contains(eng.inputs[1].RetryNote, "REQUEST_PLAN_") && !strings.Contains(eng.inputs[1].RetryNote, "phase") {
		t.Fatalf("attempts=%d note=%q", len(eng.inputs), eng.inputs[len(eng.inputs)-1].RetryNote)
	}
}

func TestRunProposalGeneration_BundleFindingsAreRetriedThenFatal(t *testing.T) {
	eng := &scriptedEngine{answers: []ProposalResult{answer(tasksDoc(twoPhaseBody))}}
	chk := &stubChecker{findings: [][]BundleFinding{{{Code: "AC_WITHOUT_SCENARIO", Message: "AC-2 has no scenario"}}, nil}}
	k, run, w := proposalKit(t, eng, chk)
	if err := w.Execute(k.w.ctx(), run); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(eng.inputs) != 2 || !strings.Contains(eng.inputs[1].RetryNote, "AC_WITHOUT_SCENARIO: AC-2 has no scenario") || chk.calls != 2 {
		t.Fatalf("attempts=%d checks=%d note=%q", len(eng.inputs), chk.calls, eng.inputs[len(eng.inputs)-1].RetryNote)
	}

	eng = &scriptedEngine{answers: []ProposalResult{answer(tasksDoc(twoPhaseBody))}}
	chk = &stubChecker{findings: [][]BundleFinding{{{Code: "X", Message: "still bad"}}, {{Code: "X", Message: "still bad"}}}}
	k, run, w = proposalKit(t, eng, chk)
	f, ok := asFailure(w.Execute(k.w.ctx(), run))
	if !ok || f.Code != domain.RunErrSpecInvalidOutput || !strings.Contains(f.Raw, "still bad") {
		t.Fatalf("%+v", f)
	}
}

func TestRunProposalGeneration_ACheckerThatCannotRunCostsAnAttemptNotTheRun(t *testing.T) {
	eng := &scriptedEngine{answers: []ProposalResult{answer(tasksDoc(twoPhaseBody))}}
	k, run, w := proposalKit(t, eng, &stubChecker{err: errors.New("openspec not responding")})
	f, ok := asFailure(w.Execute(k.w.ctx(), run))
	if !ok || f.Code != domain.RunErrSpecInvalidOutput || !strings.Contains(f.Raw, "could not be checked") {
		t.Fatalf("%+v", f)
	}
}

func TestRunProposalGeneration_EngineFailuresPassThroughUntouched(t *testing.T) {
	boom := newFailure(domain.AnalysisRun{}, domain.RunErrSpecOutOfScope, "touched backend/main.go")
	eng := &scriptedEngine{errs: []error{boom}}
	k, run, w := proposalKit(t, eng, nil)
	if err := w.Execute(k.w.ctx(), run); err != boom {
		t.Fatalf("got %v", err)
	}
	if len(eng.inputs) != 1 {
		t.Fatalf("an engine failure is not retried: %d", len(eng.inputs))
	}
}

func TestRunProposalGeneration_RequestThatLeftPlanningIsNotWritten(t *testing.T) {
	eng := &scriptedEngine{answers: []ProposalResult{answer(tasksDoc(twoPhaseBody))}}
	k, run, w := proposalKit(t, eng, nil)
	r := k.w.requests[run.RequestID]
	r.Status = domain.RequestStatusCancelled
	k.w.requests[run.RequestID] = r
	f, ok := asFailure(w.Execute(k.w.ctx(), run))
	if !ok || f.Code != planRunMoved || len(eng.inputs) != 0 {
		t.Fatalf("%+v attempts=%d", f, len(eng.inputs))
	}
}

func TestRunProposalGeneration_UsesProjectSettingsForTheReadinessMinimum(t *testing.T) {
	eng := &scriptedEngine{answers: []ProposalResult{answer(tasksDoc(twoPhaseBody))}}
	k, run, w := proposalKit(t, eng, nil)
	w.settings = openspecSettings(k.w.requests[run.RequestID].ProjectID)
	if err := w.Execute(k.w.ctx(), run); err != nil {
		t.Fatal(err)
	}
	if eng.inputs[0].Settings == nil || eng.inputs[0].Settings.Engine != domain.EngineOpenSpec {
		t.Fatalf("the engine needs the project's setting for the readiness check: %+v", eng.inputs[0].Settings)
	}
}

func TestRunProposalGeneration_FinishLosesTheLeaseQuietly(t *testing.T) {
	eng := &scriptedEngine{answers: []ProposalResult{answer(tasksDoc(twoPhaseBody))}}
	k, run, w := proposalKit(t, eng, nil)
	w.owner = "someone-else"
	if err := w.Execute(k.w.ctx(), run); !errors.Is(err, ErrLeaseLost) {
		t.Fatalf("got %v", err)
	}
}

func TestRunProposalGeneration_ARegistryWithoutTheEngineFailsClosed(t *testing.T) {
	eng := &scriptedEngine{answers: []ProposalResult{answer(tasksDoc(twoPhaseBody))}}
	k, run, w := proposalKit(t, eng, nil)
	for name, reg := range map[string]EngineRegistry{"nil registry": nil, "no openspec in it": NewDefaultEngineRegistry(), "wrong kind of engine": registryWith(nativeOnly{})} {
		w.registry = reg
		f, ok := asFailure(w.Execute(k.w.ctx(), run))
		if !ok || f.Code != RunErrEngineUnavailable {
			t.Fatalf("%s: %+v", name, f)
		}
	}
	if len(eng.inputs) != 0 {
		t.Fatal("nothing may run without the engine")
	}
}

// nativeOnly claims to be the openspec engine but has no way to author a change.
type nativeOnly struct{}

func (nativeOnly) Name() domain.EngineName { return domain.EngineOpenSpec }
func (nativeOnly) Preflight(context.Context, ProjectRef, *domain.ProjectEngineSettings) (PreflightReport, error) {
	return PreflightReport{OK: true}, nil
}
func (nativeOnly) GenerateAnalysis(context.Context, ProjectRef, AnalysisInput) (AnalysisOutput, error) {
	return AnalysisOutput{}, ErrEngineStepNotSupported
}
func (nativeOnly) GeneratePlan(context.Context, ProjectRef, PlanInput) (string, error) {
	return "", ErrEngineStepNotSupported
}
func (nativeOnly) OnRequestCompleted(context.Context, ProjectRef, CompletionInput) error { return nil }
