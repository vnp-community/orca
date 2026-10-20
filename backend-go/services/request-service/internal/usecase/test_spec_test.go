package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stablyai/orca-go/common/tddagent"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

const (
	tsTenant  = "11111111-1111-1111-1111-111111111111"
	tsRequest = "22222222-2222-2222-2222-222222222222"
	tsProject = "33333333-3333-3333-3333-333333333333"
)

type tsLane struct {
	reqs      *tsRequests
	runs      *tsRuns
	agent     *tsAgent
	resolver  *tsResolver
	actors    *tsActors
	solutions *tsSolutions
	changes   *tsChanges
	enabled   bool
	now       time.Time
}

func newLane() *tsLane {
	l := &tsLane{
		reqs: &tsRequests{reqs: map[string]domain.Request{tsRequest: {
			ID: tsRequest, ProjectID: tsProject, Number: 2, Title: "Thêm api", Status: domain.RequestStatusExecuting,
		}}},
		runs:      &tsRuns{},
		agent:     &tsAgent{cfg: TestSpecConfig{Enabled: true, LLM: "claude-cli", ContextMode: "solution"}, features: []string{"TDD-NOTES"}},
		resolver:  &tsResolver{conn: AnalysisConnection{DevServerID: "ds-1", RepoPath: "/home/dev/notes"}},
		actors:    &tsActors{actor: "user-9"},
		solutions: &tsSolutions{list: []domain.Solution{{ID: "sol-1", Status: domain.SolutionStatusApproved, OptionsJSON: []byte(`{"options":[{"id":"opt-1"}]}`)}}},
		changes:   &tsChanges{},
		enabled:   true,
		now:       time.Date(2026, 10, 19, 4, 0, 0, 0, time.UTC),
	}
	return l
}

func (l *tsLane) start() *StartTestSpecs {
	return &StartTestSpecs{
		Requests: l.reqs, Runs: l.runs, Agent: l.agent, Resolver: l.resolver, Actors: l.actors,
		Context:  &TestSpecContextBuilder{Solutions: l.solutions, Changes: l.changes},
		Settings: TestSpecSettingsFunc(func(context.Context, string) bool { return l.enabled }),
		Now:      func() time.Time { return l.now }, NewID: func() string { return "row-1" },
	}
}

func (l *tsLane) reconcile() *ReconcileTestSpecs {
	return &ReconcileTestSpecs{
		Runs: l.runs, Requests: l.reqs, Start: l.start(), Agent: l.agent, Resolver: l.resolver,
		Timing: TestSpecTiming{Deadline: 30 * time.Minute}, Owner: "t", Now: func() time.Time { return l.now },
	}
}

func tsCtx() context.Context { return tenant.WithTenantID(context.Background(), tsTenant) }

func TestStartsTheLaneWhenTheRequestIsExecuting(t *testing.T) {
	l := newLane()
	l.solutions.list[0].ChosenOption = ptr(0)
	started, err := l.start().Execute(tsCtx(), tsRequest)
	if err != nil || !started {
		t.Fatalf("started %v err %v", started, err)
	}
	row := l.runs.status(tsRequest)
	if row.State != domain.TestSpecRunning || row.TDDRunID != "run-abc" || row.ContextMode != "solution" || row.ContextRef != "REQ-2" || row.StartedBy != "user-9" {
		t.Fatalf("row %+v", row)
	}
	if l.agent.count("config") != 1 || l.agent.count("start") != 1 || l.agent.count("dispatch") != 1 {
		t.Fatalf("calls %v", l.agent.calls)
	}
	var doc map[string]any
	if err := json.Unmarshal(l.agent.lastStart.Document, &doc); err != nil {
		t.Fatal(err)
	}
	req := doc["request"].(map[string]any)
	if doc["id"] != "sol-1" || doc["status"] != "approved" || doc["chosenOption"] != float64(0) || req["ref"] != "REQ-2" {
		t.Fatalf("document %v", doc)
	}
	if l.agent.lastStart.ExecutionID != tsRequest || l.agent.lastStart.StepID != "testspec" || l.agent.lastStart.UserID != "user-9" {
		t.Fatalf("external %+v", l.agent.lastStart)
	}
}

func TestStartingTwiceStartsOnce(t *testing.T) {
	l := newLane()
	uc := l.start()
	if started, _ := uc.Execute(tsCtx(), tsRequest); !started {
		t.Fatal("first start")
	}
	if started, err := uc.Execute(tsCtx(), tsRequest); started || err != nil {
		t.Fatalf("second start %v %v", started, err)
	}
	if l.agent.count("start") != 1 || len(l.runs.rows) != 1 {
		t.Fatalf("calls %v rows %d", l.agent.calls, len(l.runs.rows))
	}
}

func TestNothingHappensWhenTheLaneIsOffOrTheRequestIsNotExecuting(t *testing.T) {
	l := newLane()
	l.enabled = false
	if started, err := l.start().Execute(tsCtx(), tsRequest); started || err != nil || len(l.runs.rows) != 0 || len(l.agent.calls) != 0 {
		t.Fatalf("off: %v %v rows %d calls %v", started, err, len(l.runs.rows), l.agent.calls)
	}
	l = newLane()
	r := l.reqs.reqs[tsRequest]
	r.Status = domain.RequestStatusAwaitingPlanApproval
	l.reqs.reqs[tsRequest] = r
	if started, _ := l.start().Execute(tsCtx(), tsRequest); started || len(l.runs.rows) != 0 || len(l.agent.calls) != 0 {
		t.Fatalf("not executing: rows %d calls %v", len(l.runs.rows), l.agent.calls)
	}
}

func TestSkippedIsNotAnErrorAndSaysWhy(t *testing.T) {
	cases := []struct {
		name  string
		setup func(*tsLane)
		code  string
	}{
		{"mode none on the dev server", func(l *tsLane) { l.agent.cfg.ContextMode = "none" }, domain.TestSpecModeNone},
		{"disabled on the dev server", func(l *tsLane) { l.agent.cfg.Enabled = false }, domain.TestSpecDisabledOnDev},
		{"no Solution", func(l *tsLane) { l.solutions.list = nil }, domain.TestSpecNoSolution},
		{"plan without an OpenSpec change", func(l *tsLane) { l.agent.cfg.ContextMode = "plan" }, domain.TestSpecNoPlanBundle},
		{"no dev server", func(l *tsLane) { l.resolver.err = ErrNoDevServer }, domain.TestSpecNoDevServer},
		{"no repository path", func(l *tsLane) { l.resolver.conn.RepoPath = "" }, domain.TestSpecNoDevServer},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			l := newLane()
			tc.setup(l)
			if _, err := l.start().Execute(tsCtx(), tsRequest); err != nil {
				t.Fatal(err)
			}
			row := l.runs.status(tsRequest)
			if row.State != domain.TestSpecSkipped || row.ReasonCode != tc.code || row.FinishedAt == nil {
				t.Fatalf("row %+v", row)
			}
			if l.agent.count("start") != 0 {
				t.Fatalf("a skipped run must not start tdd-agent: %v", l.agent.calls)
			}
		})
	}
}

func TestWhatTddAgentAnswersBecomesTheStateOfTheRow(t *testing.T) {
	cases := []struct {
		name   string
		setup  func(*tsLane)
		status domain.TestSpecStatus
		code   string
	}{
		{"llm not configured", func(l *tsLane) {
			l.agent.startErr = &tddagent.ToolError{Code: "LLM_NOT_CONFIGURED", Message: "set [testspec] llm"}
		}, domain.TestSpecBlocked, domain.TestSpecLLMMissing},
		{"write disabled", func(l *tsLane) {
			l.agent.startErr = &tddagent.ToolError{Code: "WRITE_DISABLED", Message: "set [mcp] write = true"}
		}, domain.TestSpecBlocked, domain.TestSpecWriteDisabled},
		{"old agent without the tool", func(l *tsLane) { l.agent.cfgErr = &tddagent.BridgeError{Code: "MCP_TOOL_NOT_ALLOWED", Message: "no"} }, domain.TestSpecBlocked, domain.TestSpecToolMissing},
		{"tdd-agent not installed", func(l *tsLane) {
			l.agent.cfgErr = &tddagent.BridgeError{Code: "MCP_SERVER_NOT_INSTALLED", Message: "no"}
		}, domain.TestSpecBlocked, domain.TestSpecToolMissing},
		{"context refused", func(l *tsLane) {
			l.agent.startErr = &tddagent.ToolError{Code: "TESTSPEC_CONTEXT_INVALID", Message: "bad"}
		}, domain.TestSpecBlocked, "TESTSPEC_CONTEXT_INVALID"},
		{"another tool error", func(l *tsLane) { l.agent.startErr = &tddagent.ToolError{Code: "BOOM", Message: "x"} }, domain.TestSpecFailed, "BOOM"},
		{"nobody may execute", func(l *tsLane) { l.actors.err = errors.New("not allowed") }, domain.TestSpecBlocked, domain.TestSpecActorNotAllowed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			l := newLane()
			tc.setup(l)
			if _, err := l.start().Execute(tsCtx(), tsRequest); err != nil {
				t.Fatal(err)
			}
			row := l.runs.status(tsRequest)
			if row.State != tc.status || row.ReasonCode != tc.code {
				t.Fatalf("row %+v", row)
			}
		})
	}
}

func TestADevServerThatCannotBeReachedKeepsTheRowPendingForTheReconcilePass(t *testing.T) {
	l := newLane()
	l.agent.cfgErr = errUnreachable
	if _, err := l.start().Execute(tsCtx(), tsRequest); err != nil {
		t.Fatal(err)
	}
	row := l.runs.status(tsRequest)
	if row.State != domain.TestSpecPending || row.ReasonCode != domain.TestSpecUnreachable || row.FinishedAt != nil {
		t.Fatalf("row %+v", row)
	}
	l.agent.cfgErr = nil // the dev server came back
	if n, err := l.reconcile().Execute(tsCtx()); err != nil || n != 1 {
		t.Fatalf("reconcile %d %v", n, err)
	}
	if row = l.runs.status(tsRequest); row.State != domain.TestSpecRunning || row.ReasonCode != "" {
		t.Fatalf("after reconcile %+v", row)
	}
}

func TestThePlanModeSendsOnlyTheFilesTddAgentReads(t *testing.T) {
	l := newLane()
	l.agent.cfg.ContextMode = "plan"
	l.changes.found = true
	l.changes.change = domain.OpenSpecChange{Bundle: map[string]string{
		"tasks.md": "t", "proposal.md": "p", "design.md": "d", "specs/healthz/spec.md": "s",
		"notes/secret.txt": "x", "specs/a/b/spec.md": "x", "run.sh": "x",
	}}
	if _, err := l.start().Execute(tsCtx(), tsRequest); err != nil {
		t.Fatal(err)
	}
	in := l.agent.lastStart
	if in.Kind != "plan" || in.Ref != "REQ-2" || len(in.Files) != 4 || in.Files["specs/healthz/spec.md"] != "s" || in.Files["run.sh"] != "" {
		t.Fatalf("input %+v", in)
	}
}

func TestAPlanWithoutTasksOrOverTheLimitIsSkippedNotCut(t *testing.T) {
	l := newLane()
	l.agent.cfg.ContextMode = "plan"
	l.changes.found = true
	l.changes.change = domain.OpenSpecChange{Bundle: map[string]string{"proposal.md": "p"}}
	l.start().Execute(tsCtx(), tsRequest)
	if row := l.runs.status(tsRequest); row.ReasonCode != domain.TestSpecNoPlanBundle {
		t.Fatalf("no tasks.md: %+v", row)
	}
	l = newLane()
	l.agent.cfg.ContextMode = "plan"
	l.changes.found = true
	l.changes.change = domain.OpenSpecChange{Bundle: map[string]string{"tasks.md": strings.Repeat("x", testSpecMaxFileBytes+1)}}
	l.start().Execute(tsCtx(), tsRequest)
	if row := l.runs.status(tsRequest); row.ReasonCode != domain.TestSpecTooLarge || l.agent.count("start") != 0 {
		t.Fatalf("too large: %+v", row)
	}
}

func TestTheSolutionThatIsApprovedWinsOverANewerOpenOne(t *testing.T) {
	list := []domain.Solution{
		{ID: "newer", Status: domain.SolutionStatusProposed, Seq: 5},
		{ID: "approved", Status: domain.SolutionStatusApproved, Seq: 2},
		{ID: "old", Status: domain.SolutionStatusSuperseded, Seq: 9},
	}
	if got, _ := pickSolution(list); got.ID != "approved" {
		t.Fatalf("picked %s", got.ID)
	}
	if got, _ := pickSolution(list[:1]); got.ID != "newer" {
		t.Fatalf("an unapproved Solution is still sent: %s", got.ID)
	}
	if _, ok := pickSolution(nil); ok {
		t.Fatal("no Solution")
	}
}

type tsRedactor struct{}

func (tsRedactor) Apply(_ context.Context, s string) string {
	return strings.ReplaceAll(s, "sk-secret", "[REDACTED]")
}

func TestEverythingSentIsRedactedFirst(t *testing.T) {
	l := newLane()
	l.solutions.list[0].OptionsJSON = []byte(`{"options":[{"id":"opt-1","approach":"use sk-secret"}]}`)
	uc := l.start()
	uc.Context.Redactor = tsRedactor{}
	if _, err := uc.Execute(tsCtx(), tsRequest); err != nil {
		t.Fatal(err)
	}
	if got := string(l.agent.lastStart.Document); strings.Contains(got, "sk-secret") || !strings.Contains(got, "[REDACTED]") {
		t.Fatalf("document %s", got)
	}
}

func runningLane(t *testing.T) *tsLane {
	t.Helper()
	l := newLane()
	if _, err := l.start().Execute(tsCtx(), tsRequest); err != nil {
		t.Fatal(err)
	}
	if l.runs.status(tsRequest).State != domain.TestSpecRunning {
		t.Fatal("not running")
	}
	l.agent.calls = nil
	return l
}

func TestReconcileCompletesARunAndKeepsTheSummary(t *testing.T) {
	l := runningLane(t)
	l.agent.runState = TestSpecRunState{Status: tddagent.RunCompleted}
	l.agent.summary = json.RawMessage(`{"feature":"TDD-NOTES","counts":{"TestCase":16}}`)
	if n, err := l.reconcile().Execute(tsCtx()); err != nil || n != 1 {
		t.Fatalf("reconcile %d %v", n, err)
	}
	row := l.runs.status(tsRequest)
	if row.State != domain.TestSpecCompleted || row.Feature != "TDD-NOTES" || row.FinishedAt == nil || !strings.Contains(string(row.Summary), `"TestCase":16`) {
		t.Fatalf("row %+v", row)
	}
	if n, _ := l.reconcile().Execute(tsCtx()); n != 0 {
		t.Fatalf("a finished run is not looked at again (%d)", n)
	}
}

func TestReconcileMapsEveryEndOfARun(t *testing.T) {
	cases := []struct {
		name   string
		state  TestSpecRunState
		err    error
		status domain.TestSpecStatus
		code   string
	}{
		{"failed", TestSpecRunState{Status: tddagent.RunFailed, Blockers: []string{"TESTSPEC_FAILED:LLM_GROUPS_FAILED"}}, nil, domain.TestSpecFailed, "TESTSPEC_FAILED"},
		{"blocked for the llm", TestSpecRunState{Status: tddagent.RunBlocked, Blockers: []string{"LLM_NOT_CONFIGURED"}}, nil, domain.TestSpecBlocked, domain.TestSpecLLMMissing},
		{"blocked by a document", TestSpecRunState{Status: tddagent.RunBlocked, Blockers: []string{"TESTSPEC_CONTEXT_MISSING"}}, nil, domain.TestSpecBlocked, "TESTSPEC_CONTEXT_MISSING"},
		{"blocked for another reason", TestSpecRunState{Status: tddagent.RunBlocked, Blockers: []string{"TESTSPEC_FAILED:RUN_FAILED"}}, nil, domain.TestSpecFailed, "TESTSPEC_FAILED"},
		{"lost on the dev server", TestSpecRunState{}, &tddagent.ToolError{Code: "RUN_NOT_FOUND", Message: "no"}, domain.TestSpecFailed, domain.TestSpecRunLost},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			l := runningLane(t)
			l.agent.runState, l.agent.runErr = tc.state, tc.err
			if _, err := l.reconcile().Execute(tsCtx()); err != nil {
				t.Fatal(err)
			}
			if row := l.runs.status(tsRequest); row.State != tc.status || row.ReasonCode != tc.code || row.FinishedAt == nil {
				t.Fatalf("row %+v", row)
			}
		})
	}
}

func TestReconcileLeavesARunAloneWhileItGoesOnOrTheDevServerIsAway(t *testing.T) {
	for name, setup := range map[string]func(*tsLane){
		"still running": func(l *tsLane) { l.agent.runState = TestSpecRunState{Status: tddagent.RunRunning, ProcessAlive: true} },
		"blocked but a process is alive": func(l *tsLane) {
			l.agent.runState = TestSpecRunState{Status: tddagent.RunBlocked, ProcessAlive: true}
		},
		"unreachable": func(l *tsLane) { l.agent.runErr = errUnreachable },
	} {
		t.Run(name, func(t *testing.T) {
			l := runningLane(t)
			setup(l)
			l.now = l.now.Add(time.Minute)
			if _, err := l.reconcile().Execute(tsCtx()); err != nil {
				t.Fatal(err)
			}
			row := l.runs.status(tsRequest)
			if row.State != domain.TestSpecRunning || row.FinishedAt != nil || !row.UpdatedAt.Equal(l.now) {
				t.Fatalf("row %+v", row)
			}
		})
	}
}

func TestARunThatNeverFinishesIsEndedAtTheDeadlineAndCancelled(t *testing.T) {
	l := runningLane(t)
	l.agent.runState = TestSpecRunState{Status: tddagent.RunRunning, ProcessAlive: true}
	l.now = l.now.Add(31 * time.Minute)
	if _, err := l.reconcile().Execute(tsCtx()); err != nil {
		t.Fatal(err)
	}
	if row := l.runs.status(tsRequest); row.State != domain.TestSpecFailed || row.ReasonCode != domain.TestSpecDeadline || l.agent.count("cancel") != 1 {
		t.Fatalf("row %+v calls %v", row, l.agent.calls)
	}
}

func TestReconcileStartsTheLaneForAnExecutingRequestWhoseEventWasLost(t *testing.T) {
	l := newLane()
	l.runs.missing = []TestSpecRef{{TenantID: tsTenant, RequestID: tsRequest}}
	if n, err := l.reconcile().Execute(tsCtx()); err != nil || n < 1 {
		t.Fatalf("reconcile %d %v", n, err)
	}
	if l.runs.status(tsRequest).State != domain.TestSpecRunning || l.agent.count("start") != 1 {
		t.Fatalf("row %+v calls %v", l.runs.status(tsRequest), l.agent.calls)
	}
}

func TestTwoReplicasDoNotLookAtTheSameRowTogether(t *testing.T) {
	l := runningLane(t)
	l.agent.runState = TestSpecRunState{Status: tddagent.RunRunning, ProcessAlive: true}
	row := l.runs.status(tsRequest)
	l.runs.claimed = map[string]bool{row.ID: true} // another replica holds the lease
	if n, _ := l.reconcile().Execute(tsCtx()); n != 0 || l.agent.count("run") != 0 {
		t.Fatalf("handled %d, run calls %d", n, l.agent.count("run"))
	}
}

func TestRestartReplacesTheAttemptAndCancelsTheOldRun(t *testing.T) {
	l := runningLane(t)
	ids := []string{"row-2"}
	uc := l.start()
	uc.NewID = func() string { id := ids[0]; return id }
	got, err := uc.Restart(tenant.WithUserID(tsCtx(), "user-9"), tsRequest)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "row-2" || got.Attempt != 2 || got.State != domain.TestSpecRunning || l.agent.count("cancel") != 1 {
		t.Fatalf("got %+v calls %v", got, l.agent.calls)
	}
	all, _ := l.runs.List(tsCtx(), tsRequest)
	if len(all) != 2 || all[1].SupersededAt == nil {
		t.Fatalf("history %+v", all)
	}
}

func TestRestartIsRefusedWhenTheRequestCannotHaveTestSpecs(t *testing.T) {
	l := newLane()
	r := l.reqs.reqs[tsRequest]
	r.Status = domain.RequestStatusAnalyzing
	l.reqs.reqs[tsRequest] = r
	if _, err := l.start().Restart(tsCtx(), tsRequest); err == nil || !strings.Contains(err.Error(), "REQUEST_TESTSPEC_WRONG_STATUS") && !strings.Contains(err.Error(), "executing") {
		t.Fatalf("err %v", err)
	}
	l = newLane()
	l.enabled = false
	if _, err := l.start().Restart(tsCtx(), tsRequest); err == nil {
		t.Fatal("off: want an error")
	}
}

func TestReadsShowTheCurrentAttemptAndTheEarlierOnes(t *testing.T) {
	l := runningLane(t)
	uc := l.start()
	uc.NewID = func() string { return "row-2" }
	if _, err := uc.Restart(tsCtx(), tsRequest); err != nil {
		t.Fatal(err)
	}
	view, err := (&GetTestSpecs{Requests: l.reqs, Runs: l.runs, Settings: TestSpecSettingsFunc(func(context.Context, string) bool { return true })}).Execute(tsCtx(), tsRequest)
	if err != nil || !view.Enabled || view.Current == nil || view.Current.Attempt != 2 || len(view.History) != 1 || view.History[0].Attempt != 1 {
		t.Fatalf("view %+v %v", view, err)
	}
}

func TestADocumentIsReadOnlyFromACompletedRun(t *testing.T) {
	l := runningLane(t)
	uc := &GetTestSpecDocument{Requests: l.reqs, Runs: l.runs, Agent: l.agent, Resolver: l.resolver}
	if _, err := uc.Execute(tsCtx(), tsRequest, "cases", 0); err == nil {
		t.Fatal("a run that is still going has no documents")
	}
	row := l.runs.status(tsRequest)
	row.State, row.Feature = domain.TestSpecCompleted, "TDD-NOTES"
	_ = l.runs.Update(tsCtx(), row)
	got, err := uc.Execute(tenant.WithUserID(tsCtx(), "user-9"), tsRequest, "cases", 100) // a screen call always carries its user
	if err != nil || got.Text != "TDD-NOTES:cases" || got.Length != 103 {
		t.Fatalf("got %+v %v", got, err)
	}
	if _, err := uc.Execute(tsCtx(), tsRequest, "slides", 0); err == nil {
		t.Fatal("unknown document")
	}
}

// The lane has no handle on the Request but Get: by construction it cannot change its status, which is what ORC-TS-ADR-03 asks.
func TestTheLaneNeverChangesTheRequest(t *testing.T) {
	l := newLane()
	before := l.reqs.reqs[tsRequest]
	l.agent.startErr = &tddagent.ToolError{Code: "BOOM", Message: "x"}
	_, _ = l.start().Execute(tsCtx(), tsRequest)
	if after := l.reqs.reqs[tsRequest]; after.Status != before.Status || after.Version != before.Version || after.UpdatedAt != before.UpdatedAt {
		t.Fatalf("the Request changed: %+v", after)
	}
}

func ptr[T any](v T) *T { return &v }
