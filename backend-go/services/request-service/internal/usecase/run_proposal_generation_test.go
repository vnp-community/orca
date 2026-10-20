package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// scriptedEngine plays the openspec engine: one scripted answer per attempt.
type scriptedEngine struct {
	ws         *fakeWorkspace
	afterWrite func(dir string)
	answers    []ProposalResult
	errs       []error
	inputs     []ProposalInput
	// announce is the worktree the engine reports through OnWorkspace before it can fail, as the real one does.
	announce *Workspace
	// reserve runs just before the worktree is announced, as ReservingProposalWorkspace.Ensure does in production.
	reserve func(in ProposalInput)
}

func (e *scriptedEngine) GenerateProposal(_ context.Context, in ProposalInput) (ProposalResult, error) {
	n := len(e.inputs)
	e.inputs = append(e.inputs, in)
	if e.reserve != nil {
		e.reserve(in)
	}
	if e.announce != nil && in.OnWorkspace != nil {
		in.OnWorkspace(*e.announce)
	}
	if n < len(e.errs) && e.errs[n] != nil {
		return ProposalResult{}, e.errs[n]
	}
	a := e.answers[len(e.answers)-1]
	if n < len(e.answers) {
		a = e.answers[n]
	}
	a.ChangeID = in.ChangeID
	if e.ws != nil { // act as the agent: leave the files of the change in the worktree
		dir := "openspec/changes/" + in.ChangeID + "/"
		e.ws.changed = nil
		e.ws.write(dir+"tasks.md", a.TasksMD)
		e.ws.write(dir+"proposal.md", "## Why\nx\n")
		e.ws.write(dir+"specs/ai/spec.md", "### Requirement: x\n")
		if e.afterWrite != nil {
			e.afterWrite(dir)
		}
		a.Workspace = e.ws.ws
	}
	return a, nil
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

type proposalFixtures struct {
	ws        *fakeWorkspace
	changes   *fakeChanges
	revisions *fakeRevisions
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
		Auth: NewPlanActorAuthorizer(allowAll{}, nil, false), Approvals: fakeCanceller{k.w}, Transition: k.w.transitioner(), Tx: k.w, Spawner: k.spawner,
		LeaseOwner: testOwner, Pin: &stubPinner{engine: domain.EngineOpenSpec},
	})
	run := k.startedRun(req.ID)
	if run.Mode != domain.AnalysisModeAgentProposal {
		t.Fatalf("setup: run mode %s", run.Mode)
	}
	ws, changes, revisions := newFakeWorkspace(), newFakeChanges(), &fakeRevisions{}
	eng.ws = ws
	w := NewRunProposalGeneration(RunProposalGenerationDeps{
		Requests: k.w, Solutions: fakeSolutions{k.w}, Registry: registryWith(eng), Policies: fakePolicies{domain.NewPolicyRegistry(domain.PolicyDeps{})},
		Checker: checker, Workspace: ws, Changes: changes, Revisions: revisions, Runs: fakeRuns{k.w}, Tx: k.w, Limits: domain.DefaultPlanLimits, Owner: testOwner,
	})
	k.proposal = &proposalFixtures{ws: ws, changes: changes, revisions: revisions}
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
	w.engineSettings = openspecSettings(k.w.requests[run.RequestID].ProjectID)
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

func TestRunProposalGeneration_CommitsTheChangeAndRecordsWhatItWasBuiltFrom(t *testing.T) {
	eng := &scriptedEngine{answers: []ProposalResult{func() ProposalResult {
		a := answer(tasksDoc(twoPhaseBody))
		a.SkillHash = "tdd-knowledge=1,tdd-orca-task=4"
		a.KnowledgeRequested = true
		a.Report = AgentReport{Present: true, Knowledge: KnowledgeReport{Available: true, Snapshot: "snap-000001", Read: []string{"FR-SYS-07", "ADR-12"}, Gaps: []string{"quota"}, Conflicts: []string{"ADR-12"}}}
		return a
	}()}}
	k, run, w := proposalKit(t, eng, nil)
	if err := w.Execute(k.w.ctx(), run); err != nil {
		t.Fatalf("execute: %v", err)
	}
	fx := k.proposal
	if len(fx.ws.commits) != 1 || !strings.HasPrefix(fx.ws.commits[0], "proposal: REQ-1 ") {
		t.Fatalf("commits = %v", fx.ws.commits)
	}
	prefix := "openspec/changes/" + eng.inputs[0].ChangeID + "/"
	for _, p := range fx.ws.committed[0] {
		if !strings.HasPrefix(p, prefix) {
			t.Fatalf("only paths of the change are committed, got %s", p)
		}
	}
	change, found, _ := fx.changes.GetByRequest(context.Background(), run.RequestID)
	if !found || change.Status != domain.ChangeStatusReady || change.SyncState != domain.SyncStateInSync || change.Commit != "sha-1" || change.Branch != "request/42-proposal" {
		t.Fatalf("%+v", change)
	}
	if change.ChangeID != eng.inputs[0].ChangeID || !domain.ValidChangeID(change.ChangeID) || change.WorkspaceID != "wt-1" {
		t.Fatalf("%+v", change)
	}
	if change.TasksMDDigest != domain.PlanRegionDigest(tasksDoc(twoPhaseBody)) || change.SkillHash != "tdd-knowledge=1,tdd-orca-task=4" {
		t.Fatalf("%+v", change)
	}
	if !change.KGAvailable || change.KGSnapshot != "snap-000001" || len(change.KGReadIDs) != 2 || len(change.KGGaps) != 1 || len(change.KGConflicts) != 1 {
		t.Fatalf("what the agent said it read must be kept: %+v", change)
	}
	if len(change.Bundle) != 3 || change.Bundle["tasks.md"] != tasksDoc(twoPhaseBody) || change.Bundle["proposal.md"] == "" {
		t.Fatalf("bundle paths are relative to the change: %v", change.Bundle)
	}
}

func TestRunProposalGeneration_FillsEveryTaskPromptAndKeepsTheCriteria(t *testing.T) {
	eng := &scriptedEngine{answers: []ProposalResult{answer(tasksDoc(twoPhaseBody))}}
	k, run, w := proposalKit(t, eng, nil)
	if err := w.Execute(k.w.ctx(), run); err != nil {
		t.Fatal(err)
	}
	p, err := DecodePlanProposal(*k.w.runs[run.ID].RawOutput)
	if err != nil {
		t.Fatal(err)
	}
	first := p.Phases[0].Tasks[0]
	if !strings.Contains(first.PromptTemplate, "task T1.1 of the OpenSpec change "+eng.inputs[0].ChangeID) || !strings.Contains(first.PromptTemplate, "git show request/42-proposal:") {
		t.Fatalf("the prompt must name the task, the change and the branch: %q", first.PromptTemplate)
	}
	if len(first.Satisfies) != 1 || first.Satisfies[0] != "AC-1" || !strings.Contains(first.PromptTemplate, "acceptance criteria AC-1") {
		t.Fatalf("ac= must survive storage and reach the prompt: %+v", first)
	}
	if !strings.Contains(p.Phases[1].Tasks[0].PromptTemplate, "task T2.1") {
		t.Fatal("ids are positional per phase")
	}
}

func TestRunProposalGeneration_ARegenerateKeepsTheChangeIDAndTheRow(t *testing.T) {
	eng := &scriptedEngine{answers: []ProposalResult{answer(tasksDoc(twoPhaseBody))}}
	k, run, w := proposalKit(t, eng, nil)
	if err := w.Execute(k.w.ctx(), run); err != nil {
		t.Fatal(err)
	}
	first, _, _ := k.proposal.changes.GetByRequest(context.Background(), run.RequestID)
	r := k.w.requests[run.RequestID]
	r.Title = "Hoàn toàn khác" // the title is edited between generations
	k.w.requests[run.RequestID] = r
	run2 := run
	run2.ID = "run-2"
	k.w.runs[run2.ID] = run2
	if err := w.Execute(k.w.ctx(), run2); err != nil {
		t.Fatal(err)
	}
	second, _, _ := k.proposal.changes.GetByRequest(context.Background(), run.RequestID)
	if eng.inputs[1].ChangeID != eng.inputs[0].ChangeID || second.ChangeID != first.ChangeID || second.ID != first.ID || second.Version != first.Version+1 {
		t.Fatalf("the directory, branch and row are fixed at the first generation: %+v / %+v", first, second)
	}
}

func TestRunProposalGeneration_ACommitThatFailsRecordsNothing(t *testing.T) {
	eng := &scriptedEngine{answers: []ProposalResult{answer(tasksDoc(twoPhaseBody))}}
	k, run, w := proposalKit(t, eng, nil)
	k.proposal.ws.commitErr = errors.New("git refused")
	f, ok := asFailure(w.Execute(k.w.ctx(), run))
	if !ok || f.Code != domain.RunErrSpecAgentFailed || strings.Contains(f.Message, "git refused") {
		t.Fatalf("%+v", f)
	}
	if _, found, _ := k.proposal.changes.GetByRequest(context.Background(), run.RequestID); found {
		t.Fatal("a change that is not on its branch must not be recorded as ready")
	}
	if k.w.runs[run.ID].RawOutput != nil {
		t.Fatal("and the run must not carry a proposal")
	}
}

func TestRunProposalGeneration_AStoreFailureRollsTheRunBack(t *testing.T) {
	eng := &scriptedEngine{answers: []ProposalResult{answer(tasksDoc(twoPhaseBody))}}
	k, run, w := proposalKit(t, eng, nil)
	k.proposal.changes.err = errors.New("db down")
	if err := w.Execute(k.w.ctx(), run); err == nil {
		t.Fatal("must fail")
	}
	if k.w.runs[run.ID].Status == domain.RunStatusSucceeded {
		t.Fatal("the run must not succeed when its change could not be recorded")
	}
}

func TestRunProposalGeneration_OversizeFilesAreKeptAsMarkers(t *testing.T) {
	eng := &scriptedEngine{answers: []ProposalResult{answer(tasksDoc(twoPhaseBody))}}
	k, run, w := proposalKit(t, eng, nil)
	big := strings.Repeat("x", MaxProposalReadBytes)
	eng.afterWrite = func(dir string) {
		for i := 0; i < 6; i++ {
			k.proposal.ws.write(fmt.Sprintf("%sspecs/cap%d/spec.md", dir, i), big)
		}
	}
	if err := w.Execute(k.w.ctx(), run); err != nil {
		t.Fatal(err)
	}
	change, _, _ := k.proposal.changes.GetByRequest(context.Background(), run.RequestID)
	omitted, whole := 0, 0
	total := 0
	for _, v := range change.Bundle {
		total += len(v)
		if strings.HasPrefix(v, "<omitted size=") {
			omitted++
		} else {
			whole++
		}
	}
	if omitted == 0 || total > maxBundleBytes+4096 {
		t.Fatalf("the stored bundle stays under its budget: omitted=%d total=%d", omitted, total)
	}
	if len(change.Bundle) != 9 || change.Bundle["tasks.md"] == "" {
		t.Fatalf("every file is listed, the small ones whole: %d files", len(change.Bundle))
	}
}

// A change that is written again leaves the files the agent did not rewrite on its branch. The bundle the approval is bound to and
// the reader sees must be the whole change, not the paths this run touched: the pilot's second run stored two of four specs.
func TestRunProposalGeneration_TheBundleIsTheWholeChangeEvenWhenTheRunTouchedOnlyPartOfIt(t *testing.T) {
	eng := &scriptedEngine{answers: []ProposalResult{answer(tasksDoc(twoPhaseBody))}}
	k, run, w := proposalKit(t, eng, nil)
	ws := k.proposal.ws
	eng.afterWrite = func(dir string) {
		ws.mu.Lock()
		defer ws.mu.Unlock()
		ws.files[dir+"specs/ai-client/spec.md"] = []byte("### Requirement: kept from the first run\n") // on the branch, not changed now
		ws.files[dir+"design.md"] = []byte("## Context\nkept\n")
	}
	if err := w.Execute(k.w.ctx(), run); err != nil {
		t.Fatal(err)
	}
	change, _, _ := k.proposal.changes.GetByRequest(context.Background(), run.RequestID)
	for _, want := range []string{"tasks.md", "proposal.md", "specs/ai/spec.md", "specs/ai-client/spec.md", "design.md"} {
		if change.Bundle[want] == "" {
			t.Errorf("%s is missing from the stored bundle: %v", want, change.Bundle)
		}
	}
	if len(ws.committed) != 1 || len(ws.committed[0]) != 3 {
		t.Fatalf("only what changed is committed: %v", ws.committed)
	}
}

func TestOpenSpecChange_BundleDigestDependsOnContentNotOrder(t *testing.T) {
	c := domain.OpenSpecChange{Bundle: map[string]string{"tasks.md": "a", "design.md": "b"}}
	same := domain.OpenSpecChange{Bundle: map[string]string{"design.md": "b", "tasks.md": "a"}}
	other := domain.OpenSpecChange{Bundle: map[string]string{"tasks.md": "a", "design.md": "B"}}
	if c.BundleDigest() != same.BundleDigest() || c.BundleDigest() == other.BundleDigest() {
		t.Fatal("the digest depends on content, not on map order")
	}
	if (domain.OpenSpecChange{}).BundleDigest() == c.BundleDigest() {
		t.Fatal("an empty bundle differs from a real one")
	}
}

func TestRunProposalGeneration_TheStoredProposalGoesThroughCommitPlanWithOrderAndDependencies(t *testing.T) {
	eng := &scriptedEngine{answers: []ProposalResult{answer(tasksDoc(twoPhaseBody))}}
	k, run, w := proposalKit(t, eng, nil)
	if err := w.Execute(k.w.ctx(), run); err != nil {
		t.Fatal(err)
	}
	proposal, err := DecodePlanProposal(*k.w.runs[run.ID].RawOutput)
	if err != nil {
		t.Fatal(err)
	}
	res, err := k.commit.Execute(k.w.ctx(), CommitPlanInput{RequestID: run.RequestID, Proposal: proposal})
	if err != nil || res.PlanTaskID == "" || len(res.PhaseTaskIDs) != 2 || len(res.TaskIDs) != 3 {
		t.Fatalf("commit: %+v %v", res, err)
	}
	sent := k.writer.calls[0].Proposal
	if len(sent.Phases) != 2 || sent.Phases[0].Title != "Nền" || sent.Phases[1].Tasks[0].Title != "Nối service" {
		t.Fatalf("phase and task order must follow tasks.md: %+v", sent)
	}
	if got := sent.Phases[0].Tasks[1].DependsOnIndices; len(got) != 1 || got[0] != 0 {
		t.Fatalf("T1.2 depends on T1.1: %v", got)
	}
	if got := sent.Phases[1].DependsOnPhaseIndices; len(got) != 1 || got[0] != 0 {
		t.Fatalf("PH-2 starts after PH-1: %v", got)
	}
	if !strings.Contains(sent.Phases[0].Tasks[0].Description, "Serves: FR-1") || sent.Phases[0].Tasks[0].PromptTemplate == "" {
		t.Fatalf("the task contract and the standing prompt must reach task-service: %+v", sent.Phases[0].Tasks[0])
	}
}
