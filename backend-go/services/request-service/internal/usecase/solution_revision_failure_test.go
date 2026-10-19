package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

const failedRevisionAsk = "ASK-FAILED shrink the rollout and keep the schema"

// failureMode breaks the next model or agent call and heals it again; lastPrompt reads the newest prompt sent.
type failureMode struct {
	name       string
	code       string
	breakIt    func()
	heal       func()
	lastPrompt func() string
}

// revisionFailureRig runs a change through the real GenerateSolution, runner and worker for one request type.
type revisionFailureRig struct {
	*revisionDriver
	mode failureMode
}

func (r *revisionFailureRig) lastRun() domain.AnalysisRun { return r.sp.runs[len(r.sp.runs)-1] }

func (r *revisionFailureRig) statuses(t *testing.T) map[domain.SolutionStatus]int {
	t.Helper()
	out := map[domain.SolutionStatus]int{}
	for _, s := range r.w.solutions {
		out[s.Status]++
	}
	return out
}

func completeFailureModes(comp *scriptedCompleter, t *testing.T) []failureMode {
	last := func() string { return comp.prompts[len(comp.prompts)-1] }
	good := validSolutionReply(t)
	heal := func() { comp.errs, comp.replies = nil, []string{good} }
	return []failureMode{
		{"model timed out", domain.RunErrAITimeout, func() {
			comp.errs = make([]error, len(comp.prompts)+1)
			comp.errs[len(comp.prompts)] = context.DeadlineExceeded
		}, heal, last},
		{"invalid json", domain.RunErrInvalidOutput, func() { comp.replies = []string{"this is not json"} }, heal, last},
		{"model call failed", domain.RunErrAIFailed, func() {
			comp.errs = make([]error, len(comp.prompts)+1)
			comp.errs[len(comp.prompts)] = errors.New("relay down")
		}, heal, last},
	}
}

func TestRevisionFailure_AiCompleteLeavesTheRequestRecoverable(t *testing.T) {
	comp := &scriptedCompleter{replies: []string{validSolutionReply(t)}}
	for _, mode := range completeFailureModes(comp, t) {
		t.Run(mode.name, func(t *testing.T) {
			comp.errs, comp.prompts, comp.replies = nil, nil, []string{validSolutionReply(t)}
			d, _ := completeDriver(t, comp)
			assertFailedRevisionIsRecoverable(t, &revisionFailureRig{d, mode})
		})
	}
}

func agentFailureModes(k *agentKit, t *testing.T, good string) []failureMode {
	ok := func(int, AgentPromptInput) (AgentPromptResult, error) {
		return AgentPromptResult{Stdout: good, AppliedAccessMode: "readonly"}, nil
	}
	last := func() string { return k.agent.inputs[len(k.agent.inputs)-1].Prompt }
	heal := func() { k.agent.res = ok }
	return []failureMode{
		{"agent timed out", domain.RunErrAnalysisTimeout, func() {
			k.agent.res = func(int, AgentPromptInput) (AgentPromptResult, error) { return AgentPromptResult{TimedOut: true}, nil }
		}, heal, last},
		{"invalid json", domain.RunErrAnalysisInvalid, func() {
			k.agent.res = func(int, AgentPromptInput) (AgentPromptResult, error) {
				return AgentPromptResult{Stdout: "not json", AppliedAccessMode: "readonly"}, nil
			}
		}, heal, last},
		{"READONLY_VIOLATION", domain.RunErrRepoModified, func() {
			k.agent.res = func(int, AgentPromptInput) (AgentPromptResult, error) {
				return AgentPromptResult{Stdout: good, AppliedAccessMode: "readonly", Warnings: []string{"READONLY_VIOLATION"}}, nil
			}
		}, heal, last},
	}
}

func TestRevisionFailure_AgentAnalysisLeavesTheRequestRecoverable(t *testing.T) {
	for _, tc := range []struct {
		typ   domain.RequestType
		reply string
	}{
		{domain.RequestTypeBug, "diagnosis_valid.json"},
		{domain.RequestTypeSpike, "findings_valid.json"},
		{domain.RequestTypeQuestion, "answer_valid.json"},
	} {
		for _, name := range []string{"agent timed out", "invalid json", "READONLY_VIOLATION"} {
			t.Run(string(tc.typ)+"/"+name, func(t *testing.T) {
				k := newAgentKit(t, "")
				doc := "```json\n" + analysisReply(t, tc.reply) + "\n```"
				k.agent.res = func(int, AgentPromptInput) (AgentPromptResult, error) {
					return AgentPromptResult{Stdout: doc, AppliedAccessMode: "readonly"}, nil
				}
				d := agentDriver(t, k, tc.typ, func(sp *capturingSpawner) *GenerateSolution { return k.w.newGenerate(sp, fakeConns{conn: k.conn}) })
				for _, mode := range agentFailureModes(k, t, doc) {
					if mode.name == name {
						assertFailedRevisionIsRecoverable(t, &revisionFailureRig{d, mode})
					}
				}
			})
		}
	}
}

// assertFailedRevisionIsRecoverable drives: proposal -> revision that fails -> Regenerate without text -> proposal again.
// The first proposal is superseded when the revision starts and cannot come back, so the Request must stay actionable
// and the ask that failed must not be lost.
func assertFailedRevisionIsRecoverable(t *testing.T, r *revisionFailureRig) {
	t.Helper()
	r.round("")
	first := r.w.requests[r.reqID]
	if first.Status != domain.RequestStatusAwaitingAnalysisApproval || len(r.w.opened) != 1 {
		t.Fatalf("setup: request=%s approvals=%d", first.Status, len(r.w.opened))
	}

	r.mode.breakIt()
	if _, err := r.gen.Execute(r.w.ctx(), GenerateSolutionInput{RequestID: r.reqID, Feedback: failedRevisionAsk}); err != nil {
		t.Fatal(err)
	}
	failing := r.lastRun()
	r.exec(failing)

	got := r.w.runs[failing.ID]
	if got.Status != domain.RunStatusFailed || got.ErrorCode == nil || *got.ErrorCode != r.mode.code {
		t.Fatalf("revision run = %s, want failed %s", describe(got), r.mode.code)
	}
	if st := r.w.requests[r.reqID].Status; st != domain.RequestStatusAnalyzing {
		t.Fatalf("request = %s after the failed revision, want analyzing", st)
	}
	if n := r.statuses(t); n[domain.SolutionStatusSuperseded] != 1 || len(r.w.solutions) != 1 {
		t.Fatalf("solutions after the failure = %v (the old proposal stays superseded, no draft is left)", n)
	}
	if len(r.w.opened) != 1 || len(r.w.cancelled) != 1 {
		t.Fatalf("approvals opened=%d cancelled=%d: the failed round must not open another", len(r.w.opened), len(r.w.cancelled))
	}
	if got.Feedback != failedRevisionAsk {
		t.Fatal("the failed run must keep the ask it was started with")
	}

	// Regenerate with no text: the failed ask is carried forward.
	r.mode.heal()
	if _, err := r.gen.Execute(r.w.ctx(), GenerateSolutionInput{RequestID: r.reqID}); err != nil {
		t.Fatalf("regenerate after the failure: %v", err)
	}
	retry := r.lastRun()
	if retry.ID == failing.ID || retry.Feedback != failedRevisionAsk {
		t.Fatalf("retry run %s carries feedback %q, want the failed ask", retry.ID, retry.Feedback)
	}
	r.exec(retry)
	if run := r.w.runs[retry.ID]; run.Status != domain.RunStatusSucceeded {
		t.Fatalf("retry run = %s", describe(run))
	}
	prompt := r.mode.lastPrompt()
	if !strings.Contains(blockOf(t, prompt, "feedback"), failedRevisionAsk) {
		t.Error("the retry prompt lost the failed ask")
	}
	if prior := priorBlock(t, prompt); !strings.Contains(prior, "status=superseded") {
		t.Errorf("the superseded first proposal is missing from prior_artifacts:\n%s", prior)
	}
	if st := r.w.requests[r.reqID].Status; st != domain.RequestStatusAwaitingAnalysisApproval || len(r.w.opened) != 2 {
		t.Fatalf("after the retry: request=%s approvals opened=%d", st, len(r.w.opened))
	}
	if n := r.statuses(t); n[domain.SolutionStatusSuperseded] != 1 || n[domain.SolutionStatusProposed] != 1 {
		t.Fatalf("solutions after the retry = %v", n)
	}
}

// After a failed revision the reporter can also send a new ask from analyzing: it replaces the failed one.
func TestRevisionFailure_NewAskFromAnalyzingRunsAndReachesThePrompt(t *testing.T) {
	comp := &scriptedCompleter{replies: []string{validSolutionReply(t)}}
	d, w := completeDriver(t, comp)
	d.round("")
	comp.errs = []error{nil, errors.New("relay down")}
	if _, err := d.gen.Execute(w.ctx(), GenerateSolutionInput{RequestID: d.reqID, Feedback: failedRevisionAsk}); err != nil {
		t.Fatal(err)
	}
	d.exec(d.sp.runs[len(d.sp.runs)-1])
	if w.requests[d.reqID].Status != domain.RequestStatusAnalyzing {
		t.Fatal("setup: the revision should have failed")
	}

	comp.errs = nil
	d.round("ASK-NEW take the other option")
	prompt := comp.prompts[len(comp.prompts)-1]
	if !strings.Contains(blockOf(t, prompt, "feedback"), "ASK-NEW") || strings.Contains(prompt, failedRevisionAsk) {
		t.Fatalf("the new ask must replace the failed one:\n%s", prompt)
	}
	if w.requests[d.reqID].Status != domain.RequestStatusAwaitingAnalysisApproval {
		t.Fatal("request did not reach the approval gate")
	}
}

// A first attempt that failed has no ask to carry: Regenerate stays a plain regenerate.
func TestRevisionFailure_PlainRegenerateOfAFailedFirstAttemptCarriesNothing(t *testing.T) {
	comp := &scriptedCompleter{replies: []string{"not json"}}
	d, w := completeDriver(t, comp)
	if _, err := d.gen.Execute(w.ctx(), GenerateSolutionInput{RequestID: d.reqID}); err != nil {
		t.Fatal(err)
	}
	d.exec(d.sp.runs[0])
	comp.replies = []string{validSolutionReply(t)}
	d.round("")
	if got := w.runs[d.sp.runs[len(d.sp.runs)-1].ID].Feedback; got != "" {
		t.Fatalf("feedback = %q", got)
	}
	if strings.Contains(comp.prompts[len(comp.prompts)-1], "<feedback>") {
		t.Fatal("no feedback block expected")
	}
}

// Once a retry succeeded, the next failure must not resurrect an ask that was already answered.
func TestRevisionFailure_AnsweredAskIsNotInheritedAgain(t *testing.T) {
	comp := &scriptedCompleter{replies: []string{validSolutionReply(t)}}
	d, w := completeDriver(t, comp)
	d.round("")
	d.round("ASK-ANSWERED")
	runs := len(d.sp.runs)
	// A later plain regenerate would need the Request in analyzing; the latest run succeeded, so nothing is inherited.
	inherited, err := inheritedFeedback(w.ctx(), fakeRuns{w}, d.reqID, domain.SolutionKindSolution)
	if err != nil || inherited != "" {
		t.Fatalf("inherited %q, %v", inherited, err)
	}
	if len(d.sp.runs) != runs {
		t.Fatal("reading must not start runs")
	}
}

// --- concurrent and repeated change requests ---

func TestRevision_SecondChangeRequestWhileRunningIsRefused(t *testing.T) {
	comp := &scriptedCompleter{replies: []string{validSolutionReply(t)}}
	d, w := completeDriver(t, comp)
	d.round("")

	first, err := d.gen.Execute(w.ctx(), GenerateSolutionInput{RequestID: d.reqID, Feedback: "ASK-A", IdempotencyKey: "k-a"})
	if err != nil || !first.Created {
		t.Fatalf("first: %+v %v", first, err)
	}
	_, err = d.gen.Execute(w.ctx(), GenerateSolutionInput{RequestID: d.reqID, Feedback: "ASK-B", IdempotencyKey: "k-b"})
	requireCode(t, err, "REQUEST_SOLUTION_WRONG_STATE")
	_, err = d.gen.Execute(w.ctx(), GenerateSolutionInput{RequestID: d.reqID, Feedback: "ASK-C"})
	requireCode(t, err, "REQUEST_SOLUTION_WRONG_STATE")

	if len(d.sp.runs) != 2 {
		t.Fatalf("spawned %d runs, want the first attempt and one revision", len(d.sp.runs))
	}
	running := 0
	for _, run := range w.runs {
		if run.Status == domain.RunStatusRunning {
			running++
			if run.Feedback != "ASK-A" {
				t.Fatalf("the surviving run carries %q", run.Feedback)
			}
		}
	}
	if running != 1 {
		t.Fatalf("running runs = %d", running)
	}
}

func TestRevision_DoubleClickWithTheSameKeyIsOneRun(t *testing.T) {
	comp := &scriptedCompleter{replies: []string{validSolutionReply(t)}}
	d, w := completeDriver(t, comp)
	d.round("")

	in := GenerateSolutionInput{RequestID: d.reqID, Feedback: "ASK-A", IdempotencyKey: "click-1"}
	first, err := d.gen.Execute(w.ctx(), in)
	if err != nil || !first.Created {
		t.Fatalf("first: %+v %v", first, err)
	}
	second, err := d.gen.Execute(w.ctx(), in)
	if err != nil || second.Created || second.RunID != first.RunID {
		t.Fatalf("second click: %+v %v", second, err)
	}
	if len(d.sp.runs) != 2 || len(w.cancelled) != 1 {
		t.Fatalf("spawned %d runs, cancelled %d approvals", len(d.sp.runs), len(w.cancelled))
	}

	// The same click again after the revision finished still answers with that run and starts nothing.
	d.exec(d.sp.runs[1])
	third, err := d.gen.Execute(w.ctx(), in)
	if err != nil || third.Created || third.RunID != first.RunID || len(d.sp.runs) != 2 {
		t.Fatalf("late click: %+v %v runs=%d", third, err, len(d.sp.runs))
	}
}

// A plain Regenerate that meets a running run keeps its old quiet behaviour: it returns that run.
func TestRevision_PlainRegenerateWhileRunningReturnsTheRunningRun(t *testing.T) {
	comp := &scriptedCompleter{replies: []string{validSolutionReply(t)}}
	d, w := completeDriver(t, comp)
	d.round("")
	rev, err := d.gen.Execute(w.ctx(), GenerateSolutionInput{RequestID: d.reqID, Feedback: "ASK-A"})
	if err != nil {
		t.Fatal(err)
	}
	again, err := d.gen.Execute(w.ctx(), GenerateSolutionInput{RequestID: d.reqID})
	if err != nil || again.Created || again.RunID != rev.RunID {
		t.Fatalf("plain regenerate: %+v %v", again, err)
	}
}

// The system path (a clarification was answered) keeps skipping quietly when a run is already going.
func TestRevision_SystemStartKeepsQuietReuse(t *testing.T) {
	comp := &scriptedCompleter{replies: []string{validSolutionReply(t)}}
	d, w := completeDriver(t, comp)
	d.round("")
	rev, err := d.gen.Execute(w.ctx(), GenerateSolutionInput{RequestID: d.reqID, Feedback: "ASK-A"})
	if err != nil {
		t.Fatal(err)
	}
	res, err := d.gen.ExecuteForSystem(w.ctx(), GenerateSolutionInput{RequestID: d.reqID, Feedback: "answers"})
	if err != nil || res.Created || res.RunID != rev.RunID {
		t.Fatalf("system start: %+v %v", res, err)
	}
}

// Real clocks differ per platform; ordering of recent runs must not depend on identical timestamps.
func TestRevisionFailure_RunsAreOrderedByStartTime(t *testing.T) {
	comp := &scriptedCompleter{replies: []string{validSolutionReply(t)}}
	d, w := completeDriver(t, comp)
	base := time.Now().UTC()
	step := 0
	d.gen.now = func() time.Time { step++; return base.Add(time.Duration(step) * time.Second) }
	d.round("")
	comp.errs = []error{nil, errors.New("relay down")}
	if _, err := d.gen.Execute(w.ctx(), GenerateSolutionInput{RequestID: d.reqID, Feedback: failedRevisionAsk}); err != nil {
		t.Fatal(err)
	}
	d.exec(d.sp.runs[1])
	got, err := inheritedFeedback(w.ctx(), fakeRuns{w}, d.reqID, domain.SolutionKindSolution)
	if err != nil || got != failedRevisionAsk {
		t.Fatalf("inherited %q, %v", got, err)
	}
}

type failingRunList struct {
	AnalysisRunStore
}

func (failingRunList) ListRecent(context.Context, string, int) ([]domain.AnalysisRun, error) {
	return nil, errors.New("runs unavailable")
}

// A history that cannot be read stops the start instead of silently dropping the failed ask.
func TestRevisionFailure_UnreadableRunHistoryFailsTheStart(t *testing.T) {
	w := newSolWorld()
	sp := &capturingSpawner{}
	r := w.seedRequest(domain.RequestTypeChangeRequest, domain.RequestStatusAnalyzing)
	gen := w.newGenerate(sp, fakeConns{conn: AnalysisConnection{ConnectionID: "c"}})
	gen.runs = failingRunList{fakeRuns{w}}
	if _, err := gen.Execute(w.ctx(), GenerateSolutionInput{RequestID: r.ID}); err == nil || !strings.Contains(err.Error(), "runs unavailable") {
		t.Fatalf("err = %v", err)
	}
	if len(sp.runs) != 0 || len(w.runs) != 0 {
		t.Fatal("nothing may start when the history is unreadable")
	}
}
