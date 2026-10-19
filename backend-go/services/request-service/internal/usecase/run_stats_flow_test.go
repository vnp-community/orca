package usecase

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

type recordingObserver struct {
	mu         sync.Mutex
	runs       []string
	preflight  []string
	toolCalls  []domain.RunStats
	violations []string
}

func (r *recordingObserver) RunFinished(engine, kind, result string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.runs = append(r.runs, engine+"/"+kind+"/"+result)
}
func (r *recordingObserver) PreflightFailed(code string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.preflight = append(r.preflight, code)
}
func (r *recordingObserver) KnowledgeToolCalls(s domain.RunStats) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.toolCalls = append(r.toolCalls, s)
}
func (r *recordingObserver) SpecViolation(code string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.violations = append(r.violations, code)
}

func twoCalls() domain.RunStats {
	c := 0.4
	return domain.RunStats{ToolCalls: map[string]int{domain.KnowledgeToolPrefix + "kg_search": 3, "Read": 2}, Usage: &domain.RunUsage{Turns: 4, DurationMS: 1000, CostUSD: &c}}
}

func TestRunStats_ASolutionRunKeepsWhatItsAgentReportedAndTheMetricsHearOfIt(t *testing.T) {
	k := newSolutionAgentKit(t, true)
	obs := &recordingObserver{}
	k.agent.res = func(int, AgentPromptInput) (AgentPromptResult, error) {
		return AgentPromptResult{Stdout: fenced(domainTestdata(t, "solution_options/valid_with_design_references.json")), AppliedAccessMode: "readonly", Stats: twoCalls()}, nil
	}
	run, err := k.start(t)
	if err != nil {
		t.Fatal(err)
	}
	k.build()
	k.runner.WithObserver(obs)
	k.runner.worker(run).(*RunAgentReadonlyAnalysis).WithObserver(obs)
	k.runner.execute(run)
	got := k.w.runs[run.ID]
	if got.Status != domain.RunStatusSucceeded || got.Stats == nil || got.Stats.KnowledgeCalls() != 3 || got.Stats.Usage == nil || got.Stats.Usage.Turns != 4 {
		t.Fatalf("run %s stats %+v", describe(got), got.Stats)
	}
	if len(obs.runs) != 1 || obs.runs[0] != "native/solution/succeeded" || len(obs.toolCalls) != 1 || obs.toolCalls[0].KnowledgeCalls() != 3 {
		t.Fatalf("observer: runs=%v calls=%v", obs.runs, obs.toolCalls)
	}
}

func TestRunStats_ARunThatFailsAfterTheAgentRanStillKeepsItsStatsAndIsCountedAsFailed(t *testing.T) {
	k := newSolutionAgentKit(t, true)
	obs := &recordingObserver{}
	k.agent.res = func(int, AgentPromptInput) (AgentPromptResult, error) {
		return AgentPromptResult{Stdout: "not json at all", AppliedAccessMode: "readonly", Stats: twoCalls()}, nil
	}
	run, err := k.start(t)
	if err != nil {
		t.Fatal(err)
	}
	k.build()
	k.runner.WithObserver(obs)
	k.runner.execute(run)
	got := k.w.runs[run.ID]
	if got.Status != domain.RunStatusFailed {
		t.Fatalf("run %s", describe(got))
	}
	// Two attempts (one corrective retry), so the figures of both are on the run.
	if got.Stats == nil || got.Stats.ToolCalls[domain.KnowledgeToolPrefix+"kg_search"] != 6 || got.Stats.Usage.Turns != 8 {
		t.Fatalf("failed run stats = %+v", got.Stats)
	}
	if got.Stats.Usage.CostUSD == nil || *got.Stats.Usage.CostUSD != 0.8 {
		t.Fatalf("the cost of the two attempts must add up: %+v", got.Stats.Usage)
	}
	if len(obs.runs) != 1 || obs.runs[0] != "native/solution/failed" {
		t.Fatalf("runs = %v", obs.runs)
	}
}

func TestRunStats_ARunWithoutAnAgentStoresNoStats(t *testing.T) {
	k := newSolutionAgentKit(t, true)
	k.agent.res = func(int, AgentPromptInput) (AgentPromptResult, error) {
		return AgentPromptResult{Stdout: fenced(validSolutionReply(t)), AppliedAccessMode: "readonly"}, nil
	}
	run, err := k.start(t)
	if err != nil {
		t.Fatal(err)
	}
	k.runner.execute(run)
	if got := k.w.runs[run.ID]; got.Status != domain.RunStatusSucceeded || got.Stats != nil {
		t.Fatalf("an older agent reports nothing and the run must say so: %s %+v", describe(got), got.Stats)
	}
}

func TestRunStats_AProposalRunKeepsTheStatsOfEveryAttemptAndAFailureKeepsThemToo(t *testing.T) {
	eng := &statsEngine{scriptedEngine: scriptedEngine{answers: []ProposalResult{answer(tasksDoc(twoPhaseBody))}}, stats: twoCalls()}
	k, run, w := proposalKit(t, &eng.scriptedEngine, nil)
	w.registry = registryWith(eng)
	if err := w.Execute(k.w.ctx(), run); err != nil {
		t.Fatal(err)
	}
	got := k.w.runs[run.ID]
	if got.Stats == nil || got.Stats.KnowledgeCalls() != 3 {
		t.Fatalf("success: %+v", got.Stats)
	}

	failing := &statsEngine{scriptedEngine: scriptedEngine{errs: []error{errors.New("agent failed")}, answers: []ProposalResult{{}}}, stats: twoCalls(), failAfterStats: true}
	k2, run2, w2 := proposalKit(t, &failing.scriptedEngine, nil)
	w2.registry = registryWith(failing)
	err := w2.Execute(k2.w.ctx(), run2)
	f, ok := asFailure(err)
	if !ok && err == nil {
		t.Fatal("the run was meant to fail")
	}
	if ok && (f.Stats == nil || f.Stats.KnowledgeCalls() != 3) {
		t.Fatalf("a failure from the engine carries the stats collected before it: %+v", f.Stats)
	}
}

// statsEngine reports stats into the worker the way the real engine does after each agent call.
type statsEngine struct {
	scriptedEngine
	stats          domain.RunStats
	failAfterStats bool
}

func (e *statsEngine) GenerateProposal(ctx context.Context, in ProposalInput) (ProposalResult, error) {
	res, err := e.scriptedEngine.GenerateProposal(ctx, in)
	if in.Stats != nil {
		*in.Stats = in.Stats.Merge(e.stats)
	}
	if e.failAfterStats && err != nil {
		return res, newFailure(in.Run, domain.RunErrSpecAgentFailed, "the agent failed")
	}
	return res, err
}

func TestRunStats_TheWorkerCountsEachStructureProblemOfAnAuthoredChange(t *testing.T) {
	obs := &recordingObserver{}
	eng := &scriptedEngine{answers: []ProposalResult{answer("no plan region at all")}}
	k, run, w := proposalKit(t, eng, nil)
	w.observer = obs
	_ = w.Execute(k.w.ctx(), run)
	if len(obs.violations) == 0 {
		t.Fatalf("a change without a plan region reported no violation")
	}
	for _, code := range obs.violations {
		if code == "" {
			t.Fatal("a violation without a code")
		}
	}
}

func TestRunStats_TheReadinessCheckCountsAFreshBlockerOnceAndACachedVerdictNever(t *testing.T) {
	obs := &recordingObserver{}
	ex := &fakeExec{res: ExecResult{ExitCode: 127}}
	g := NewEngineReadinessGate(worktreeConn, ex, "").WithObserver(obs)
	for i := 0; i < 2; i++ {
		if rep, _ := g.Check(context.Background(), "p", nil); rep.OK {
			t.Fatal("openspec is missing")
		}
	}
	if len(obs.preflight) != 2 || obs.preflight[0] != "REQUEST_ENGINE_OPENSPEC_MISSING" {
		t.Fatalf("a failing verdict is never cached, so each fresh check counts: %v", obs.preflight)
	}
	ok := &fakeExec{res: ExecResult{Stdout: "1.3.0"}}
	obs2 := &recordingObserver{}
	g2 := NewEngineReadinessGate(worktreeConn, ok, "").WithObserver(obs2)
	g2.Check(context.Background(), "p", nil)
	g2.Check(context.Background(), "p", nil)
	if len(obs2.preflight) != 0 {
		t.Fatalf("a clean check counted failures: %v", obs2.preflight)
	}
	none := NewEngineReadinessGate(fakeConns{err: ErrNoDevServer}, ok, "").WithObserver(obs2)
	none.Check(context.Background(), "p", nil)
	if len(obs2.preflight) != 1 || obs2.preflight[0] != "REQUEST_ENGINE_NO_CONNECTION" {
		t.Fatalf("an early blocker is counted too: %v", obs2.preflight)
	}
}
