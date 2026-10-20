package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// countingPinner records how often the decision tried to pin.
type countingPinner struct {
	inner ExecutionModePinner
	calls int
}

func (p *countingPinner) PinExecutionModeIfUnset(ctx context.Context, id string, mode domain.ExecutionMode) (domain.ExecutionMode, error) {
	p.calls++
	return p.inner.PinExecutionModeIfUnset(ctx, id, mode)
}

func runAllSetup(t *testing.T) (*apprEnv, domain.Request, *domain.Approval, *countingPinner) {
	t.Helper()
	e, r, a := approvedSetup(t)
	p := &countingPinner{inner: e.s}
	e.decide.Pinner = p
	return e, r, a, p
}

func runAllInput(a *domain.Approval, mode string) DecideApprovalInput {
	return DecideApprovalInput{ID: a.ID, Decision: "approve", ExpectedDigest: a.SubjectDigest, ExecutionMode: mode}
}

func decidedPayload(t *testing.T, e *apprEnv) map[string]any {
	t.Helper()
	var found []map[string]any
	for _, ev := range e.s.events {
		if ev.Subject != domain.SubjectApprovalDecided {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal(ev.Payload, &m); err != nil {
			t.Fatal(err)
		}
		found = append(found, m)
	}
	if len(found) != 1 {
		t.Fatalf("want exactly one approval.decided event, got %d", len(found))
	}
	return found[0]
}

func TestDecideApproval_ContinuousPinsTheModeWithTheDecision(t *testing.T) {
	e, r, a, p := runAllSetup(t)
	res, err := e.decide.Execute(userCtx(r.ReporterID, "user"), runAllInput(a, " Continuous "))
	if err != nil {
		t.Fatal(err)
	}
	if res.ExecutionMode != domain.ExecutionContinuous || res.RequestStatus != domain.RequestStatusExecuting {
		t.Fatalf("result = %+v", res)
	}
	got := e.s.requests[r.ID]
	if got.ExecutionMode != domain.ExecutionContinuous || got.Status != domain.RequestStatusExecuting || p.calls != 1 {
		t.Fatalf("request = %+v, pin calls = %d", got, p.calls)
	}
	if m := decidedPayload(t, e); m["execution_mode"] != "continuous" {
		t.Fatalf("payload = %v", m)
	}
}

func TestDecideApproval_NoChoiceBehavesAsBefore(t *testing.T) {
	e, r, a, p := runAllSetup(t)
	res, err := e.decide.Execute(userCtx(r.ReporterID, "user"), runAllInput(a, ""))
	if err != nil {
		t.Fatal(err)
	}
	if res.ExecutionMode != "" || p.calls != 0 || e.s.requests[r.ID].ExecutionMode != "" {
		t.Fatalf("an approval without a choice must not pin: %+v calls=%d", res, p.calls)
	}
	if _, has := decidedPayload(t, e)["execution_mode"]; has {
		t.Fatal("the key must be absent when nothing was chosen")
	}
}

func TestDecideApproval_NoChoiceReportsTheModeAlreadyPinned(t *testing.T) {
	e, r, a, p := runAllSetup(t)
	req := e.s.requests[r.ID]
	req.ExecutionMode = domain.ExecutionStepByStep
	e.s.requests[r.ID] = req
	res, err := e.decide.Execute(userCtx(r.ReporterID, "user"), runAllInput(a, ""))
	if err != nil {
		t.Fatal(err)
	}
	if res.ExecutionMode != domain.ExecutionStepByStep || p.calls != 0 {
		t.Fatalf("the response states the truth and nothing is pinned again: %+v calls=%d", res, p.calls)
	}
	if _, has := decidedPayload(t, e)["execution_mode"]; has {
		t.Fatal("the payload records the choice, not the pinned mode")
	}
}

func TestDecideApproval_RefusedModesLeaveTheApprovalPending(t *testing.T) {
	cases := []struct {
		name     string
		decision string
		mode     string
		code     string
	}{
		{"step_by_step is not a choice", "approve", "step_by_step", "REQUEST_EXECUTION_MODE_INVALID"},
		{"garbage", "approve", "turbo", "REQUEST_EXECUTION_MODE_INVALID"},
		{"mode with reject", "reject", "continuous", "REQUEST_EXECUTION_MODE_INVALID"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e, r, a, p := runAllSetup(t)
			in := runAllInput(a, c.mode)
			in.Decision, in.Comment = c.decision, "no"
			_, err := e.decide.Execute(userCtx(r.ReporterID, "user"), in)
			wantCode(t, err, c.code)
			if e.s.approvals[a.ID].Status != domain.ApprovalStatusPending || p.calls != 0 || len(e.s.events) != 0 {
				t.Fatalf("a refused mode must write nothing: calls=%d events=%d", p.calls, len(e.s.events))
			}
		})
	}
}

func TestDecideApproval_ModeOnAnApprovalThatDoesNotStartARun(t *testing.T) {
	for _, tc := range []struct {
		name    string
		subject domain.SubjectType
		stage   string
	}{
		{"phase approval", domain.SubjectPhase, string(domain.RequestStatusAwaitingPlanApproval)},
		{"solution approval", domain.SubjectSolution, string(domain.RequestStatusAwaitingPlanApproval)},
		{"plan approval at another stage", domain.SubjectPlan, string(domain.RequestStatusExecuting)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e, r, a, p := runAllSetup(t)
			cur := e.s.approvals[a.ID]
			cur.SubjectType, cur.Stage = tc.subject, tc.stage
			e.s.approvals[a.ID] = cur
			_, err := e.decide.Execute(userCtx(r.ReporterID, "user"), runAllInput(a, "continuous"))
			wantCode(t, err, "REQUEST_EXECUTION_MODE_NOT_APPLICABLE")
			if p.calls != 0 || e.s.approvals[a.ID].Status != domain.ApprovalStatusPending {
				t.Fatal("nothing may be written")
			}
		})
	}
}

func TestDecideApproval_RequestAlreadyPinnedStepByStepRefusesTheDecision(t *testing.T) {
	e, r, a, _ := runAllSetup(t)
	req := e.s.requests[r.ID]
	req.ExecutionMode = domain.ExecutionStepByStep
	e.s.requests[r.ID] = req

	_, err := e.decide.Execute(userCtx(r.ReporterID, "user"), runAllInput(a, "continuous"))
	wantCode(t, err, "REQUEST_EXECUTION_MODE_ALREADY_PINNED")
	got := e.s.requests[r.ID]
	if e.s.approvals[a.ID].Status != domain.ApprovalStatusPending || got.Status != domain.RequestStatusAwaitingPlanApproval ||
		got.ExecutionMode != domain.ExecutionStepByStep || len(e.s.events) != 0 || len(e.art.decided) != 0 {
		t.Fatalf("the refused decision must leave everything as it was: %+v", got)
	}
}

func TestDecideApproval_RequestAlreadyPinnedContinuousIsIdempotent(t *testing.T) {
	e, r, a, _ := runAllSetup(t)
	req := e.s.requests[r.ID]
	req.ExecutionMode = domain.ExecutionContinuous
	e.s.requests[r.ID] = req
	res, err := e.decide.Execute(userCtx(r.ReporterID, "user"), runAllInput(a, "continuous"))
	if err != nil || res.ExecutionMode != domain.ExecutionContinuous || res.RequestStatus != domain.RequestStatusExecuting {
		t.Fatalf("res=%+v err=%v", res, err)
	}
}

// The pin lives in the decision's transaction: when the subject handler fails the column goes back to unset.
func TestDecideApproval_HandlerFailureRollsTheModeBack(t *testing.T) {
	e, r, a, p := runAllSetup(t)
	e.art.decidedEr = errors.New("artifact store down")
	_, err := e.decide.Execute(userCtx(r.ReporterID, "user"), runAllInput(a, "continuous"))
	if err == nil {
		t.Fatal("the handler error must surface")
	}
	if p.calls != 1 {
		t.Fatalf("the pin was attempted once, got %d", p.calls)
	}
	if got := e.s.requests[r.ID]; got.ExecutionMode != "" || got.Status != domain.RequestStatusAwaitingPlanApproval {
		t.Fatalf("pin must roll back with the decision: %+v", got)
	}
	if e.s.approvals[a.ID].Status != domain.ApprovalStatusPending || len(e.s.events) != 0 {
		t.Fatal("approval stays pending and no event is written")
	}
}

func TestDecideApproval_DoubleClickPinsOnceAndReportsThePinnedMode(t *testing.T) {
	e, r, a, p := runAllSetup(t)
	ctx := userCtx(r.ReporterID, "user")
	if _, err := e.decide.Execute(ctx, runAllInput(a, "continuous")); err != nil {
		t.Fatal(err)
	}
	res, err := e.decide.Execute(ctx, runAllInput(a, "continuous"))
	if err != nil {
		t.Fatal(err)
	}
	if res.ExecutionMode != domain.ExecutionContinuous || p.calls != 1 {
		t.Fatalf("second call: %+v, pin calls = %d", res, p.calls)
	}
	decidedPayload(t, e) // exactly one event
}

func TestDecideApproval_ModeWithoutAPinnerIsAnExplicitError(t *testing.T) {
	e, r, a, _ := runAllSetup(t)
	e.decide.Pinner = nil
	_, err := e.decide.Execute(userCtx(r.ReporterID, "user"), runAllInput(a, "continuous"))
	wantCode(t, err, "REQUEST_EXECUTION_MODE_PINNER_MISSING")
	if e.s.approvals[a.ID].Status != domain.ApprovalStatusPending {
		t.Fatal("the decision must not be recorded")
	}
	if _, err := e.decide.Execute(userCtx(r.ReporterID, "user"), runAllInput(a, "")); err != nil {
		t.Fatalf("without a choice no pinner is needed: %v", err)
	}
}

func TestDecideApproval_ConcurrentDecisionsOneWins(t *testing.T) {
	e, r, a, _ := runAllSetup(t)
	admin1, admin2 := userCtx(uuid.NewString(), "admin"), userCtx(uuid.NewString(), "admin")
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i, c := range []struct {
		ctx  context.Context
		mode string
	}{{admin1, "continuous"}, {admin2, ""}} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = e.decide.Execute(c.ctx, runAllInput(a, c.mode))
		}()
	}
	wg.Wait()
	wins := 0
	for _, err := range errs {
		if err == nil {
			wins++
			continue
		}
		wantCode(t, err, "REQUEST_APPROVAL_ALREADY_DECIDED")
	}
	if wins != 1 {
		t.Fatalf("exactly one decision may win, errors: %v", errs)
	}
	chosen := decidedPayload(t, e)["execution_mode"] == "continuous"
	if (e.s.requests[r.ID].ExecutionMode == domain.ExecutionContinuous) != chosen {
		t.Fatalf("pinned mode %q must match what the winner chose (%v)", e.s.requests[r.ID].ExecutionMode, chosen)
	}
}

// A project that is step by step still needs the approver to be able to execute when they chose to run everything.
func TestDecideApproval_PrecheckSeesTheRequestedMode(t *testing.T) {
	e, r, a, p := runAllSetup(t)
	tasks := newExTasks()
	tasks.add(domain.TaskView{Type: domain.TaskTypePlan, Status: domain.TaskStatusOpen, RequestID: r.ID, Title: "plan"})
	tasks.executors = map[string]bool{"committer": true}
	settings := &domain.ProjectEngineSettings{ExecutionMode: domain.ExecutionStepByStep}
	e.decide.StartPrecheck = &ExecutionStartPrecheck{
		Requests: e.s, Access: &ExecutionAccess{Tasks: tasks}, Authorizer: e.decide.Authorizer,
		Modes: &ExecutionModeResolver{Settings: staticEngineSettings{settings}},
	}
	ctx := userCtx(r.ReporterID, "user")

	_, err := e.decide.Execute(ctx, runAllInput(a, "continuous"))
	wantCode(t, err, "REQUEST_EXECUTION_ACTOR_NOT_ALLOWED")
	if p.calls != 0 || e.s.approvals[a.ID].Status != domain.ApprovalStatusPending {
		t.Fatal("the precheck refuses before anything is written")
	}
	if _, err := e.decide.Execute(ctx, runAllInput(a, "")); err != nil {
		t.Fatalf("no choice in a step by step project is not checked: %v", err)
	}
}
