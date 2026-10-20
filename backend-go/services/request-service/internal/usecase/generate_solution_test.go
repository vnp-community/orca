package usecase

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

func TestGenerateSolution_ReturnsRunAndSpawnsWorker(t *testing.T) {
	w := newSolWorld()
	sp := &capturingSpawner{}
	uc := w.newGenerate(sp, fakeConns{conn: AnalysisConnection{ConnectionID: "c1"}})
	r := w.seedRequest(domain.RequestTypeChangeRequest, domain.RequestStatusAnalyzing)

	res, err := uc.Execute(w.ctx(), GenerateSolutionInput{RequestID: r.ID})
	if err != nil || !res.Created || res.RunID == "" || res.SolutionID == "" {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	run := w.runs[res.RunID]
	if run.Mode != domain.AnalysisModeComplete || run.Kind != domain.RunKindSolution || run.Status != domain.RunStatusRunning || run.SolutionID != res.SolutionID {
		t.Fatalf("run = %+v", run)
	}
	draft := w.solutions[res.SolutionID]
	if draft.Status != domain.SolutionStatusDraft || draft.GenerationRunID != res.RunID || draft.Kind != domain.SolutionKindSolution {
		t.Fatalf("draft = %+v", draft)
	}
	if len(sp.runs) != 1 || sp.runs[0].ID != res.RunID {
		t.Fatalf("worker not spawned once: %+v", sp.runs)
	}
	if got := w.requests[r.ID]; got.Status != domain.RequestStatusAnalyzing {
		t.Fatalf("request status = %s", got.Status)
	}
}

func TestGenerateSolution_NoConnectionCreatesNothing(t *testing.T) {
	w := newSolWorld()
	r := w.seedRequest(domain.RequestTypeChangeRequest, domain.RequestStatusAnalyzing)
	uc := w.newGenerate(&capturingSpawner{}, fakeConns{err: ErrNoDevServer})
	_, err := uc.Execute(w.ctx(), GenerateSolutionInput{RequestID: r.ID})
	requireCode(t, err, "REQUEST_SOLUTION_NO_CONNECTION")
	if len(w.runs) != 0 || len(w.solutions) != 0 {
		t.Fatalf("rows left behind: %d runs, %d solutions", len(w.runs), len(w.solutions))
	}
}

func TestGenerateSolution_RejectsWrongStateTypeAndInput(t *testing.T) {
	w := newSolWorld()
	uc := w.newGenerate(&capturingSpawner{}, fakeConns{conn: AnalysisConnection{ConnectionID: "c"}})
	cases := []struct {
		name string
		typ  domain.RequestType
		st   domain.RequestStatus
		in   GenerateSolutionInput
		code string
	}{
		{"new request", domain.RequestTypeChangeRequest, domain.RequestStatusNew, GenerateSolutionInput{}, "REQUEST_SOLUTION_WRONG_STATE"},
		{"awaiting without feedback", domain.RequestTypeChangeRequest, domain.RequestStatusAwaitingAnalysisApproval, GenerateSolutionInput{}, "REQUEST_SOLUTION_WRONG_STATE"},
		{"executing", domain.RequestTypeChangeRequest, domain.RequestStatusExecuting, GenerateSolutionInput{Feedback: "x"}, "REQUEST_SOLUTION_WRONG_STATE"},
		{"type without analysis", domain.RequestTypeTask, domain.RequestStatusAnalyzing, GenerateSolutionInput{}, "REQUEST_SOLUTION_KIND_NOT_ALLOWED"},
		{"agent mode for a solution", domain.RequestTypeChangeRequest, domain.RequestStatusAnalyzing, GenerateSolutionInput{Mode: domain.AnalysisModeAgentReadonly}, "REQUEST_SOLUTION_MODE_NOT_ALLOWED"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := w.seedRequest(tc.typ, tc.st)
			tc.in.RequestID = r.ID
			_, err := uc.Execute(w.ctx(), tc.in)
			requireCode(t, err, tc.code)
		})
	}
	if len(w.runs) != 0 {
		t.Fatalf("rejected calls created %d runs", len(w.runs))
	}

	_, err := uc.Execute(w.ctx(), GenerateSolutionInput{RequestID: "missing"})
	requireCode(t, err, "REQUEST_SOLUTION_REQUEST_NOT_FOUND")
	r := w.seedRequest(domain.RequestTypeChangeRequest, domain.RequestStatusAwaitingAnalysisApproval)
	long := make([]rune, 2001)
	for i := range long {
		long[i] = 'ạ'
	}
	_, err = uc.Execute(w.ctx(), GenerateSolutionInput{RequestID: r.ID, Feedback: string(long)})
	requireCode(t, err, "REQUEST_SOLUTION_FEEDBACK_TOO_LONG")
	_, err = uc.Execute(tenant.WithTenantID(t.Context(), "t1"), GenerateSolutionInput{RequestID: r.ID, Feedback: "x"})
	requireCode(t, err, "REQUEST_REPORTER_REQUIRED")
	_, err = uc.Execute(t.Context(), GenerateSolutionInput{RequestID: r.ID})
	requireCode(t, err, "REQUEST_TENANT_REQUIRED")
}

func TestGenerateSolution_ForbiddenForOutsiders(t *testing.T) {
	w := newSolWorld()
	r := w.seedRequest(domain.RequestTypeChangeRequest, domain.RequestStatusAnalyzing)
	uc := w.newGenerate(&capturingSpawner{}, fakeConns{conn: AnalysisConnection{ConnectionID: "c"}})
	uc.auth = ReporterOrAdminAuthorizer{}
	_, err := uc.Execute(w.ctx(), GenerateSolutionInput{RequestID: r.ID})
	requireCode(t, err, "REQUEST_SOLUTION_FORBIDDEN")
	if _, err := uc.Execute(tenant.WithUserID(lcCtx(), r.ReporterID), GenerateSolutionInput{RequestID: r.ID}); err != nil {
		t.Fatalf("reporter must be allowed: %v", err)
	}
}

func TestGenerateSolution_ConcurrentCallsShareOneRun(t *testing.T) {
	w := newSolWorld()
	sp := &capturingSpawner{}
	uc := w.newGenerate(sp, fakeConns{conn: AnalysisConnection{ConnectionID: "c"}})
	r := w.seedRequest(domain.RequestTypeChangeRequest, domain.RequestStatusAnalyzing)

	var wg sync.WaitGroup
	ids := make([]string, 8)
	for i := range ids {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, err := uc.Execute(w.ctx(), GenerateSolutionInput{RequestID: r.ID})
			if err != nil {
				t.Error(err)
			}
			ids[i] = res.RunID
		}()
	}
	wg.Wait()
	for _, id := range ids {
		if id != ids[0] {
			t.Fatalf("run ids differ: %v", ids)
		}
	}
	if len(w.runs) != 1 || len(sp.runs) != 1 {
		t.Fatalf("want one run and one worker, got %d runs, %d workers", len(w.runs), len(sp.runs))
	}
}

func TestGenerateSolution_IdempotencyKeyReturnsEarlierRun(t *testing.T) {
	w := newSolWorld()
	sp := &capturingSpawner{}
	uc := w.newGenerate(sp, fakeConns{conn: AnalysisConnection{ConnectionID: "c"}})
	r := w.seedRequest(domain.RequestTypeChangeRequest, domain.RequestStatusAnalyzing)
	first := must(uc.Execute(w.ctx(), GenerateSolutionInput{RequestID: r.ID, IdempotencyKey: "k1"}))
	// Even after the first run ended, the same key still answers with that run.
	run := w.runs[first.RunID]
	run.Fail("X", "x", time.Now())
	w.runs[first.RunID] = run
	second := must(uc.Execute(w.ctx(), GenerateSolutionInput{RequestID: r.ID, IdempotencyKey: "k1"}))
	if second.RunID != first.RunID || second.Created || len(sp.runs) != 1 {
		t.Fatalf("first=%+v second=%+v spawned=%d", first, second, len(sp.runs))
	}
}

func TestGenerateSolution_RegenerateWithFeedback(t *testing.T) {
	w := newSolWorld()
	sp := &capturingSpawner{}
	uc := w.newGenerate(sp, fakeConns{conn: AnalysisConnection{ConnectionID: "c"}})
	r := w.seedRequest(domain.RequestTypeChangeRequest, domain.RequestStatusAwaitingAnalysisApproval)
	old := domain.Solution{ID: "00000000-0000-0000-0000-0000000000aa", TenantID: "t1", RequestID: r.ID, Kind: domain.SolutionKindSolution, Status: domain.SolutionStatusProposed, Version: 1, CreatedAt: time.Now()}
	w.solutions[old.ID] = old

	res, err := uc.Execute(w.ctx(), GenerateSolutionInput{RequestID: r.ID, Feedback: "thử phương án không đổi schema"})
	if err != nil || !res.Created {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	if got := w.solutions[old.ID].Status; got != domain.SolutionStatusSuperseded {
		t.Fatalf("old solution = %s", got)
	}
	if len(w.cancelled) != 1 || w.cancelled[0] != r.ID+":revision_requested" {
		t.Fatalf("cancelled = %v", w.cancelled)
	}
	if got := w.requests[r.ID].Status; got != domain.RequestStatusAnalyzing {
		t.Fatalf("request = %s, want analyzing (analysis_revision)", got)
	}
	if w.runs[res.RunID].Feedback != "thử phương án không đổi schema" {
		t.Fatalf("feedback not stored on the run: %+v", w.runs[res.RunID])
	}
	if w.solutions[res.SolutionID].Status != domain.SolutionStatusDraft {
		t.Fatalf("new draft missing")
	}
}

func TestGenerateSolution_RegenerateRollsBackWhenCancelFails(t *testing.T) {
	w := newSolWorld()
	w.failCanc = errors.New("approvals down")
	uc := w.newGenerate(&capturingSpawner{}, fakeConns{conn: AnalysisConnection{ConnectionID: "c"}})
	r := w.seedRequest(domain.RequestTypeChangeRequest, domain.RequestStatusAwaitingAnalysisApproval)
	_, err := uc.Execute(w.ctx(), GenerateSolutionInput{RequestID: r.ID, Feedback: "again"})
	if err == nil {
		t.Fatal("want error")
	}
	if len(w.runs) != 0 || len(w.solutions) != 0 || w.requests[r.ID].Status != domain.RequestStatusAwaitingAnalysisApproval {
		t.Fatalf("partial write: runs=%d solutions=%d status=%s", len(w.runs), len(w.solutions), w.requests[r.ID].Status)
	}
}

func TestGenerateSolution_AgentKindsUseReadonlyMode(t *testing.T) {
	cases := []struct {
		typ  domain.RequestType
		kind domain.RunKind
	}{
		{domain.RequestTypeBug, domain.RunKindDiagnosis},
		{domain.RequestTypeSpike, domain.RunKindFindings},
		{domain.RequestTypeQuestion, domain.RunKindAnswer},
		{domain.RequestTypeHotfix, domain.RunKindDiagnosis},
	}
	for _, tc := range cases {
		t.Run(string(tc.typ), func(t *testing.T) {
			w := newSolWorld()
			uc := w.newGenerate(&capturingSpawner{}, fakeConns{conn: AnalysisConnection{ConnectionID: "c", RepoPath: "/repo"}})
			r := w.seedRequest(tc.typ, domain.RequestStatusAnalyzing)
			res := must(uc.Execute(w.ctx(), GenerateSolutionInput{RequestID: r.ID}))
			if run := w.runs[res.RunID]; run.Mode != domain.AnalysisModeAgentReadonly || run.Kind != tc.kind {
				t.Fatalf("run = %+v", run)
			}
			// analysis_mode=COMPLETE overrides the registry for the agent kinds.
			w2 := newSolWorld()
			uc2 := w2.newGenerate(&capturingSpawner{}, fakeConns{conn: AnalysisConnection{ConnectionID: "c"}})
			r2 := w2.seedRequest(tc.typ, domain.RequestStatusAnalyzing)
			res2 := must(uc2.Execute(w2.ctx(), GenerateSolutionInput{RequestID: r2.ID, Mode: domain.AnalysisModeComplete}))
			if w2.runs[res2.RunID].Mode != domain.AnalysisModeComplete {
				t.Fatalf("override ignored: %+v", w2.runs[res2.RunID])
			}
		})
	}
}

func TestGenerateSolution_AgentModeNeedsRepoPathAndConnection(t *testing.T) {
	w := newSolWorld()
	r := w.seedRequest(domain.RequestTypeQuestion, domain.RequestStatusAnalyzing)
	_, err := w.newGenerate(&capturingSpawner{}, fakeConns{conn: AnalysisConnection{ConnectionID: "c"}}).Execute(w.ctx(), GenerateSolutionInput{RequestID: r.ID})
	requireCode(t, err, "REQUEST_ANALYSIS_NO_REPO_PATH")
	_, err = w.newGenerate(&capturingSpawner{}, fakeConns{err: ErrNoDevServer}).Execute(w.ctx(), GenerateSolutionInput{RequestID: r.ID})
	requireCode(t, err, "REQUEST_ANALYSIS_NO_CONNECTION")
	if len(w.runs) != 0 {
		t.Fatal("no run may be created")
	}
}

// The dev-server fallback of the connection resolver has a repo path and a dev server but no connection id or worktree.
func TestGenerateSolution_AgentModeAcceptsADevServerConnectionWithARepoPath(t *testing.T) {
	for _, typ := range []domain.RequestType{domain.RequestTypeBug, domain.RequestTypeHotfix, domain.RequestTypeSecurity, domain.RequestTypePerformance, domain.RequestTypeSpike, domain.RequestTypeQuestion} {
		t.Run(string(typ), func(t *testing.T) {
			w := newSolWorld()
			r := w.seedRequest(typ, domain.RequestStatusAnalyzing)
			conn := AnalysisConnection{DevServerID: "ds-1", RepoPath: "/srv/repo"}
			res, err := w.newGenerate(&capturingSpawner{}, fakeConns{conn: conn}).Execute(w.ctx(), GenerateSolutionInput{RequestID: r.ID})
			if err != nil || w.runs[res.RunID].Mode != domain.AnalysisModeAgentReadonly {
				t.Fatalf("res %+v err %v run %+v", res, err, w.runs[res.RunID])
			}
		})
	}
}

// A dev server whose default repo is a remote URL has nowhere to run the agent: refuse with the structured code, create nothing.
func TestGenerateSolution_AgentModeRefusesADevServerConnectionWithoutARepoPath(t *testing.T) {
	w := newSolWorld()
	r := w.seedRequest(domain.RequestTypeBug, domain.RequestStatusAnalyzing)
	_, err := w.newGenerate(&capturingSpawner{}, fakeConns{conn: AnalysisConnection{DevServerID: "ds-1"}}).Execute(w.ctx(), GenerateSolutionInput{RequestID: r.ID})
	requireCode(t, err, "REQUEST_ANALYSIS_NO_REPO_PATH")
	if len(w.runs) != 0 || len(w.solutions) != 0 {
		t.Fatalf("no row may be created: %d runs, %d drafts", len(w.runs), len(w.solutions))
	}
	// Complete mode never reads the repo, so the same connection still serves it.
	res, err := w.newGenerate(&capturingSpawner{}, fakeConns{conn: AnalysisConnection{DevServerID: "ds-1"}}).Execute(w.ctx(), GenerateSolutionInput{RequestID: r.ID, Mode: domain.AnalysisModeComplete})
	if err != nil || w.runs[res.RunID].Mode != domain.AnalysisModeComplete {
		t.Fatalf("complete mode: %+v err %v", res, err)
	}
}

func TestGenerateSolution_BusyProjectLeavesNoRows(t *testing.T) {
	w := newSolWorld()
	uc := w.newGenerate(&capturingSpawner{}, fakeConns{conn: AnalysisConnection{ConnectionID: "c", RepoPath: "/repo"}})
	project := "11111111-1111-1111-1111-111111111111"
	var third domain.Request
	for i := 0; i < 3; i++ {
		r := w.seedRequest(domain.RequestTypeQuestion, domain.RequestStatusAnalyzing)
		r.ProjectID = project
		w.requests[r.ID] = r
		third = r
		_, err := uc.Execute(w.ctx(), GenerateSolutionInput{RequestID: r.ID})
		if i < 2 && err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
		if i == 2 {
			requireCode(t, err, "REQUEST_ANALYSIS_BUSY")
		}
	}
	if len(w.runs) != 2 || len(w.solutions) != 2 {
		t.Fatalf("want 2 runs and 2 drafts, got %d and %d", len(w.runs), len(w.solutions))
	}
	for _, r := range w.runs {
		if r.RequestID == third.ID {
			t.Fatal("busy call left a run behind")
		}
	}
}

func TestGenerateSolution_ReturnsBeforeSlowAIFinishes(t *testing.T) {
	w := newSolWorld()
	block := make(chan struct{})
	completer := &scriptedCompleter{replies: []string{validSolutionReply(t)}, block: block}
	runner, _ := newTestRunner(w, completer, nil)
	defer runner.Close()
	uc := w.newGenerate(runner, fakeConns{conn: AnalysisConnection{ConnectionID: "c"}})
	r := w.seedRequest(domain.RequestTypeChangeRequest, domain.RequestStatusAnalyzing)

	start := time.Now()
	res, err := uc.Execute(w.ctx(), GenerateSolutionInput{RequestID: r.ID})
	elapsed := time.Since(start)
	if err != nil || elapsed > time.Second {
		t.Fatalf("Execute took %v (err=%v) while the AI call blocks", elapsed, err)
	}
	close(block)
	waitFor(t, "solution proposed", func() bool { return w.solutionStatus(res.SolutionID) == domain.SolutionStatusProposed })
}

func (w *solWorld) solutionStatus(id string) domain.SolutionStatus {
	w.lcStore.mu.Lock()
	defer w.lcStore.mu.Unlock()
	return w.solutions[id].Status
}

var _ = context.Background

func TestGenerateSolution_SystemStartNeedsNoCallerAndDefersTheSpawn(t *testing.T) {
	w := newSolWorld()
	sp := &capturingSpawner{}
	uc := w.newGenerate(sp, fakeConns{conn: AnalysisConnection{ConnectionID: "c1"}})
	r := w.seedRequest(domain.RequestTypeChangeRequest, domain.RequestStatusAnalyzing)
	noUser := lcCtx() // a consumer has a tenant but no user

	if _, err := uc.Execute(noUser, GenerateSolutionInput{RequestID: r.ID}); err == nil {
		t.Fatal("the public path still needs a caller")
	}
	res, after, err := uc.PrepareForSystem(noUser, GenerateSolutionInput{RequestID: r.ID, Feedback: "clarification:c-1", IdempotencyKey: "clr-c-1"})
	if err != nil || !res.Created || after == nil {
		t.Fatalf("res=%+v after=%v err=%v", res, after != nil, err)
	}
	if len(sp.runs) != 0 {
		t.Fatal("the worker must wait for the caller's commit")
	}
	if w.runs[res.RunID].ActorID != r.ReporterID {
		t.Fatalf("the run is attributed to the reporter, got %q", w.runs[res.RunID].ActorID)
	}
	after()
	if len(sp.runs) != 1 || sp.runs[0].ID != res.RunID {
		t.Fatalf("spawned = %+v", sp.runs)
	}
	again, after2, err := uc.PrepareForSystem(noUser, GenerateSolutionInput{RequestID: r.ID, IdempotencyKey: "clr-c-1"})
	if err != nil || again.Created || after2 != nil || again.RunID != res.RunID {
		t.Fatalf("the same key must hand back the same run without a second spawn: %+v", again)
	}
}

// userRequiringConns mimics AIConnectionResolver: it fails without a user in ctx, as project-service does for a
// tenant-only background call (the cause of SOLUTION-NOT-STARTED: the resume consumer's ctx has only the tenant).
type userRequiringConns struct{}

func (userRequiringConns) ResolveForProject(ctx context.Context, _ string) (AnalysisConnection, error) {
	if u, _ := tenant.UserID(ctx); u == "" {
		return AnalysisConnection{}, domain.ErrRequestReporterRequired()
	}
	return AnalysisConnection{DevServerID: "ds-1"}, nil
}

func TestGenerateSolution_SystemStartActsAsReporterForConnectionLookup(t *testing.T) {
	w := newSolWorld()
	sp := &capturingSpawner{}
	uc := w.newGenerate(sp, userRequiringConns{})
	r := w.seedRequest(domain.RequestTypeChangeRequest, domain.RequestStatusAnalyzing)

	// Same ctx the clarification resume consumer builds: tenant only.
	res, after, err := uc.PrepareForSystem(lcCtx(), GenerateSolutionInput{RequestID: r.ID, Feedback: "clarification:c1", IdempotencyKey: "clr-c1"})
	if err != nil || !res.Created || after == nil {
		t.Fatalf("res=%+v after=%v err=%v", res, after != nil, err)
	}
	if run := w.runs[res.RunID]; run.ActorID != r.ReporterID {
		t.Errorf("run actor = %q, want the reporter %q", run.ActorID, r.ReporterID)
	}
	after()
	if len(sp.runs) != 1 {
		t.Fatalf("worker not spawned: %+v", sp.runs)
	}

	// A user-initiated start without a user still fails: only the system path borrows the reporter.
	if _, err := w.newGenerate(&capturingSpawner{}, userRequiringConns{}).Execute(lcCtx(), GenerateSolutionInput{RequestID: r.ID}); err == nil {
		t.Fatal("a caller-less user start must be rejected")
	}
}

// reporterOnlyConns accepts only the reporter's identity, like a dev server bound to the reporter.
type reporterOnlyConns struct{ reporter string }

func (c reporterOnlyConns) ResolveForProject(ctx context.Context, _ string) (AnalysisConnection, error) {
	if u, _ := tenant.UserID(ctx); u != c.reporter {
		return AnalysisConnection{}, ErrNoDevServer
	}
	return AnalysisConnection{DevServerID: "ds-1"}, nil
}

func TestGenerateSolution_CallerWhoIsNotTheReporterResolvesTheDevServerAsTheReporter(t *testing.T) {
	w := newSolWorld()
	r := w.seedRequest(domain.RequestTypeChangeRequest, domain.RequestStatusAnalyzing)
	approver := "approver-1"
	if approver == r.ReporterID {
		t.Fatal("test setup: the caller must differ from the reporter")
	}
	uc := w.newGenerate(&capturingSpawner{}, reporterOnlyConns{reporter: r.ReporterID})
	uc.auth = allowAllAuthorizer{}

	res, err := uc.Execute(tenant.WithRole(tenant.WithUserID(lcCtx(), approver), "member"), GenerateSolutionInput{RequestID: r.ID})
	if err != nil || !res.Created {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	if run := w.runs[res.RunID]; run.ActorID != approver {
		t.Errorf("the run stays attributed to the caller %q, got %q", approver, run.ActorID)
	}
}

type allowAllAuthorizer struct{}

func (allowAllAuthorizer) AuthorizeGenerate(context.Context, domain.Request) error   { return nil }
func (allowAllAuthorizer) AuthorizeChoose(context.Context, domain.Request) error     { return nil }
func (allowAllAuthorizer) AuthorizeRevise(context.Context, domain.Request) error     { return nil }
func (allowAllAuthorizer) AuthorizeRevisePlan(context.Context, domain.Request) error { return nil }
