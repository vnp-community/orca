package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// newTestRunner wires the real runner and workers over the fakes; tests call runner.execute for determinism.
func newTestRunner(w *solWorld, completer ProjectAICompleter, readonly AnalysisWorker) (*AnalysisRunner, *RunSolutionGeneration) {
	settings := AnalysisSettings{LeaseTTL: time.Minute, Heartbeat: time.Hour}
	gen := NewRunSolutionGeneration(w, fakeSolutions{w}, completer, nil, w.newWriter(w.transitioner()), settings)
	return NewAnalysisRunner(fakeRuns{w}, fakeSolutions{w}, w, gen, readonly, settings, testOwner), gen
}

// startRun creates a run through GenerateSolution and returns it without executing the worker.
func startRun(t *testing.T, w *solWorld, typ domain.RequestType, conn AnalysisConnection) (domain.AnalysisRun, domain.Request) {
	t.Helper()
	sp := &capturingSpawner{}
	r := w.seedRequest(typ, domain.RequestStatusAnalyzing)
	if _, err := w.newGenerate(sp, fakeConns{conn: conn}).Execute(w.ctx(), GenerateSolutionInput{RequestID: r.ID}); err != nil {
		t.Fatal(err)
	}
	if len(sp.runs) != 1 {
		t.Fatalf("spawned %d runs", len(sp.runs))
	}
	return sp.runs[0], r
}

func TestRunSolutionGeneration_SuccessWithFencedJSON(t *testing.T) {
	w := newSolWorld()
	run, r := startRun(t, w, domain.RequestTypeChangeRequest, AnalysisConnection{ConnectionID: "c"})
	completer := &scriptedCompleter{replies: []string{fenced(validSolutionReply(t))}}
	runner, _ := newTestRunner(w, completer, nil)
	runner.execute(run)

	got := w.runs[run.ID]
	if got.Status != domain.RunStatusSucceeded || got.RawOutput == nil {
		t.Fatalf("run = %s", describe(got))
	}
	sol := w.solutions[run.SolutionID]
	if sol.Status != domain.SolutionStatusProposed || sol.ChosenOption != nil {
		t.Fatalf("solution = %+v", sol)
	}
	if _, err := domain.ParseSolutionOptions(sol.OptionsJSON); err != nil {
		t.Fatalf("stored options unreadable: %v", err)
	}
	if st := w.requests[r.ID].Status; st != domain.RequestStatusAwaitingAnalysisApproval {
		t.Fatalf("request = %s", st)
	}
	if len(w.opened) != 1 || w.opened[0].SubjectType != domain.SubjectSolution || w.opened[0].SubjectID != sol.ID {
		t.Fatalf("approvals opened = %+v", w.opened)
	}
	wantDigest := must(domain.DigestOptions(sol.OptionsJSON, nil))
	if w.opened[0].Digest != wantDigest {
		t.Fatalf("digest = %s, want %s", w.opened[0].Digest, wantDigest)
	}
	subjects := w.outboxSubjects()
	if !hasString(subjects, domain.SubjectSolutionProposed) || !hasString(subjects, domain.SubjectRequestStatusChanged) {
		t.Fatalf("outbox = %v", subjects)
	}
	for _, e := range w.events {
		if e.Subject == domain.SubjectSolutionProposed {
			var p map[string]any
			_ = json.Unmarshal(e.Payload, &p)
			if _, leaks := p["options"]; leaks || p["option_count"] != float64(2) || p["recommended_option_id"] != "opt-1" || p["request_id"] != r.ID {
				t.Fatalf("payload = %v", p)
			}
		}
	}
}

func hasString(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func TestRunSolutionGeneration_InvalidThenValidRetriesOnceWithTheError(t *testing.T) {
	w := newSolWorld()
	run, _ := startRun(t, w, domain.RequestTypeChangeRequest, AnalysisConnection{ConnectionID: "c"})
	completer := &scriptedCompleter{replies: []string{oneOptionReply(), validSolutionReply(t)}}
	runner, _ := newTestRunner(w, completer, nil)
	runner.execute(run)

	if completer.calls() != 2 {
		t.Fatalf("calls = %d", completer.calls())
	}
	if !strings.Contains(completer.prompts[1], "number of options") {
		t.Fatalf("retry prompt lacks the validation error:\n%s", completer.prompts[1])
	}
	got := w.runs[run.ID]
	if got.Status != domain.RunStatusSucceeded || got.Attempt != 2 {
		t.Fatalf("run = %s attempt=%d", describe(got), got.Attempt)
	}
}

func TestRunSolutionGeneration_InvalidTwiceFailsAndLeavesNoDraft(t *testing.T) {
	for name, bad := range map[string]string{
		"one option":          oneOptionReply(),
		"not json":            "xin lỗi, tôi không thể",
		"two recommended":     strings.Replace(validSolutionReply(t), `"recommended": false`, `"recommended": true`, 1),
		"duplicate option id": strings.Replace(validSolutionReply(t), `"id": "opt-2"`, `"id": "opt-1"`, 1),
	} {
		t.Run(name, func(t *testing.T) {
			w := newSolWorld()
			run, r := startRun(t, w, domain.RequestTypeChangeRequest, AnalysisConnection{ConnectionID: "c"})
			completer := &scriptedCompleter{replies: []string{bad}}
			runner, _ := newTestRunner(w, completer, nil)
			runner.execute(run)

			if completer.calls() != 2 {
				t.Fatalf("calls = %d, want exactly one retry", completer.calls())
			}
			got := w.runs[run.ID]
			if got.Status != domain.RunStatusFailed || got.ErrorCode == nil || *got.ErrorCode != domain.RunErrInvalidOutput || got.RawOutput == nil {
				t.Fatalf("run = %s", describe(got))
			}
			if len(w.solutions) != 0 {
				t.Fatalf("draft left behind: %+v", w.solutions)
			}
			if st := w.requests[r.ID].Status; st != domain.RequestStatusAnalyzing {
				t.Fatalf("request moved to %s", st)
			}
			if len(w.opened) != 0 {
				t.Fatal("approval opened for an invalid result")
			}
		})
	}
}

func TestRunSolutionGeneration_AIErrorsFailTheRun(t *testing.T) {
	for name, tc := range map[string]struct {
		err  error
		code string
	}{
		"no dev server": {ErrNoDevServer, domain.RunErrNoConnection},
		"timeout":       {ErrClassifierTimeout, domain.RunErrAITimeout},
		"deadline":      {context.DeadlineExceeded, domain.RunErrAITimeout},
		"other":         {errors.New("relay exploded"), domain.RunErrAIFailed},
	} {
		t.Run(name, func(t *testing.T) {
			w := newSolWorld()
			run, r := startRun(t, w, domain.RequestTypeChangeRequest, AnalysisConnection{ConnectionID: "c"})
			completer := &scriptedCompleter{errs: []error{tc.err}}
			runner, _ := newTestRunner(w, completer, nil)
			runner.execute(run)
			got := w.runs[run.ID]
			if got.Status != domain.RunStatusFailed || got.ErrorCode == nil || *got.ErrorCode != tc.code {
				t.Fatalf("run = %s", describe(got))
			}
			if got.ErrorMessage != nil && strings.Contains(*got.ErrorMessage, "exploded") {
				t.Fatalf("upstream error text leaked into the stored message: %s", *got.ErrorMessage)
			}
			if len(w.solutions) != 0 || w.requests[r.ID].Status != domain.RequestStatusAnalyzing {
				t.Fatalf("draft/status wrong: %d solutions, %s", len(w.solutions), w.requests[r.ID].Status)
			}
		})
	}
}

func TestRunSolutionGeneration_LeaseLostWritesNothing(t *testing.T) {
	w := newSolWorld()
	run, r := startRun(t, w, domain.RequestTypeChangeRequest, AnalysisConnection{ConnectionID: "c"})
	completer := &scriptedCompleter{replies: []string{validSolutionReply(t)}, onCall: func() {
		// Another instance takes the run over while the AI call is in flight.
		cur := w.runs[run.ID]
		other := "other-instance"
		cur.LeaseOwner = &other
		w.runs[run.ID] = cur
	}}
	runner, _ := newTestRunner(w, completer, nil)
	runner.execute(run)

	if st := w.solutions[run.SolutionID].Status; st != domain.SolutionStatusDraft {
		t.Fatalf("solution = %s, must stay draft for the new owner", st)
	}
	if w.runs[run.ID].Status != domain.RunStatusRunning || len(w.opened) != 0 || w.requests[r.ID].Status != domain.RequestStatusAnalyzing {
		t.Fatalf("worker wrote after losing the lease: run=%s opened=%d status=%s", describe(w.runs[run.ID]), len(w.opened), w.requests[r.ID].Status)
	}
	for _, s := range w.outboxSubjects() {
		if s == domain.SubjectSolutionProposed {
			t.Fatal("event emitted after losing the lease")
		}
	}
}

func TestRunSolutionGeneration_WorkerSurvivesTheRPCContext(t *testing.T) {
	w := newSolWorld()
	release := make(chan struct{})
	completer := &scriptedCompleter{replies: []string{validSolutionReply(t)}, block: release}
	runner, _ := newTestRunner(w, completer, nil)
	defer runner.Close()
	uc := w.newGenerate(runner, fakeConns{conn: AnalysisConnection{ConnectionID: "c"}})
	r := w.seedRequest(domain.RequestTypeChangeRequest, domain.RequestStatusAnalyzing)

	rpcCtx, cancel := context.WithCancel(w.ctx())
	res, err := uc.Execute(rpcCtx, GenerateSolutionInput{RequestID: r.ID})
	if err != nil {
		t.Fatal(err)
	}
	cancel() // the RPC ended; the run must go on
	close(release)
	waitFor(t, "solution proposed", func() bool { return w.solutionStatus(res.SolutionID) == domain.SolutionStatusProposed })
}

func TestPersist_RollsBackEverythingWhenTheTransitionFails(t *testing.T) {
	w := newSolWorld()
	run, r := startRun(t, w, domain.RequestTypeChangeRequest, AnalysisConnection{ConnectionID: "c"})
	writer := w.newWriter(failingTransition{err: errBoom})
	err := writer.Persist(w.ctx(), PersistAnalysisInput{Run: run, Doc: []byte(validSolutionReply(t)), Raw: "raw"})
	if !errors.Is(err, errBoom) {
		t.Fatalf("err = %v", err)
	}
	if st := w.solutions[run.SolutionID].Status; st != domain.SolutionStatusDraft {
		t.Fatalf("solution = %s, must stay draft", st)
	}
	if w.runs[run.ID].Status != domain.RunStatusRunning || len(w.opened) != 0 || len(w.events) != 0 {
		t.Fatalf("not rolled back: run=%s opened=%d events=%d", describe(w.runs[run.ID]), len(w.opened), len(w.events))
	}
	if w.requests[r.ID].Status != domain.RequestStatusAnalyzing {
		t.Fatal("request moved")
	}
}

func TestPersist_RollsBackWhenOpeningTheApprovalFails(t *testing.T) {
	w := newSolWorld()
	run, r := startRun(t, w, domain.RequestTypeChangeRequest, AnalysisConnection{ConnectionID: "c"})
	w.failOpen = errBoom
	err := w.newWriter(w.transitioner()).Persist(w.ctx(), PersistAnalysisInput{Run: run, Doc: []byte(validSolutionReply(t))})
	if !errors.Is(err, errBoom) {
		t.Fatalf("err = %v", err)
	}
	if w.solutions[run.SolutionID].Status != domain.SolutionStatusDraft || len(w.events) != 0 || w.requests[r.ID].Status != domain.RequestStatusAnalyzing {
		t.Fatal("approval failure left partial state")
	}
}

func TestPersist_DropsResultWhenTheRequestMovedOn(t *testing.T) {
	w := newSolWorld()
	run, r := startRun(t, w, domain.RequestTypeChangeRequest, AnalysisConnection{ConnectionID: "c"})
	moved := w.requests[r.ID]
	moved.Status = domain.RequestStatusCancelled
	w.requests[r.ID] = moved
	completer := &scriptedCompleter{replies: []string{validSolutionReply(t)}}
	runner, _ := newTestRunner(w, completer, nil)
	runner.execute(run)
	got := w.runs[run.ID]
	if got.Status != domain.RunStatusFailed || *got.ErrorCode != "REQUEST_SOLUTION_REQUEST_MOVED" || len(w.solutions) != 0 || len(w.opened) != 0 {
		t.Fatalf("run=%s solutions=%d opened=%d", describe(got), len(w.solutions), len(w.opened))
	}
}

func TestRecoverInterruptedAnalysisRuns_FailsExpiredRunsAndDropsDrafts(t *testing.T) {
	w := newSolWorld()
	run, r := startRun(t, w, domain.RequestTypeChangeRequest, AnalysisConnection{ConnectionID: "c"})
	live, _ := startRun(t, w, domain.RequestTypeChangeRequest, AnalysisConnection{ConnectionID: "c"})
	past := w.now.Add(-time.Minute)
	cur := w.runs[run.ID]
	cur.LeaseExpiresAt = &past
	w.runs[run.ID] = cur

	rec := NewRecoverInterruptedAnalysisRuns(fakeRuns{w}, fakeSolutions{w}, w, AnalysisSettings{}, "sweeper")
	n, err := rec.RunOnce(context.Background())
	if err != nil || n != 1 {
		t.Fatalf("recovered %d, err %v", n, err)
	}
	got := w.runs[run.ID]
	if got.Status != domain.RunStatusFailed || *got.ErrorCode != domain.RunErrInterrupted {
		t.Fatalf("run = %s", describe(got))
	}
	if _, ok := w.solutions[run.SolutionID]; ok {
		t.Fatal("draft not removed")
	}
	if w.runs[live.ID].Status != domain.RunStatusRunning || w.solutions[live.SolutionID].Status != domain.SolutionStatusDraft {
		t.Fatal("a live run was touched")
	}
	if w.requests[r.ID].Status != domain.RequestStatusAnalyzing {
		t.Fatal("recovery must not move the request")
	}
	// A user can start again: the failed run no longer blocks.
	uc := w.newGenerate(&capturingSpawner{}, fakeConns{conn: AnalysisConnection{ConnectionID: "c"}})
	if res, err := uc.Execute(w.ctx(), GenerateSolutionInput{RequestID: r.ID}); err != nil || !res.Created {
		t.Fatalf("restart blocked: %+v %v", res, err)
	}
}

func TestRecoverLoop_StopsWithContext(t *testing.T) {
	w := newSolWorld()
	rec := NewRecoverInterruptedAnalysisRuns(fakeRuns{w}, fakeSolutions{w}, w, AnalysisSettings{}, "sweeper")
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { rec.RunRecoveryLoop(ctx, 10*time.Millisecond); close(done) }()
	time.Sleep(30 * time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("loop did not stop")
	}
}

func TestRunSolutionGeneration_PromptCarriesPriorArtifactsAndFeedback(t *testing.T) {
	w := newSolWorld()
	r := w.seedRequest(domain.RequestTypeChangeRequest, domain.RequestStatusAnalyzing)
	rejected := domain.Solution{ID: "00000000-0000-0000-0000-0000000000bb", TenantID: "t1", RequestID: r.ID, Kind: domain.SolutionKindSolution, Status: domain.SolutionStatusRejected,
		OptionsJSON: []byte(`{"old":"approach"}`), Version: 1, CreatedAt: time.Now()}
	w.solutions[rejected.ID] = rejected
	sp := &capturingSpawner{}
	if _, err := w.newGenerate(sp, fakeConns{conn: AnalysisConnection{ConnectionID: "c"}}).Execute(w.ctx(), GenerateSolutionInput{RequestID: r.ID}); err != nil {
		t.Fatal(err)
	}
	run := sp.runs[0]
	run.Feedback = "đừng đổi schema"
	completer := &scriptedCompleter{replies: []string{validSolutionReply(t)}}
	runner, _ := newTestRunner(w, completer, nil)
	runner.execute(run)
	p := completer.prompts[0]
	for _, want := range []string{"<prior_artifacts>", `{"old":"approach"}`, "status=rejected", "đừng đổi schema"} {
		if !strings.Contains(p, want) {
			t.Errorf("prompt lacks %q", want)
		}
	}
	_ = tenant.WithTenantID
}
