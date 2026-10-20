package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// PinExecutionModeIfUnset is versionless, like the real repositories.
func (s *lcStore) PinExecutionModeIfUnset(_ context.Context, id string, mode domain.ExecutionMode) (domain.ExecutionMode, error) {
	r, ok := s.requests[id]
	if !ok {
		return "", domain.ErrRequestNotFound(id)
	}
	if r.ExecutionMode == "" {
		r.ExecutionMode = mode
		s.requests[id] = r
	}
	return r.ExecutionMode, nil
}

type forbiddenActors struct{}

func (forbiddenActors) ActorFor(context.Context, domain.Request, string) (string, error) {
	return "", domain.ErrExecuteForbidden()
}

type staticEngineSettings struct{ s *domain.ProjectEngineSettings }

func (f staticEngineSettings) Get(context.Context, string) (domain.ProjectEngineSettings, bool, error) {
	if f.s == nil {
		return domain.ProjectEngineSettings{}, false, nil
	}
	return *f.s, true, nil
}
func (f staticEngineSettings) Upsert(_ context.Context, s domain.ProjectEngineSettings, _ int64) (domain.ProjectEngineSettings, error) {
	return s, nil
}

func (r *exRig) continuousRequest(typ domain.RequestType, size domain.RequestSize) domain.Request {
	return r.store.seed(func(q *domain.Request) {
		q.Type, q.Size, q.Status, q.ExecutionMode = typ, size, domain.RequestStatusExecuting, domain.ExecutionContinuous
	})
}

func (r *exRig) startedPhases() []domain.PhaseStart {
	rows, _ := r.starts.ListByRequest(lcCtx(), r.store.anyRequestID())
	return rows
}

func (s *lcStore) anyRequestID() string {
	for id := range s.requests {
		return id
	}
	return ""
}

func (r *exRig) finishPhaseTask(req domain.Request, task domain.TaskView) {
	r.t.Helper()
	r.tasks.mu.Lock()
	r.tasks.setStatusLocked(task.ID, domain.TaskStatusReview)
	r.tasks.mu.Unlock()
	if err := r.event(req, r.tasks.get(task.ID), domain.CauseExecutionCompleted, domain.TaskStatusReview, ""); err != nil {
		r.t.Fatalf("report outcome: %v", err)
	}
}

func TestContinuous_ChangeRequest_RunsEveryPhaseAfterOneApproval(t *testing.T) {
	r := newExRig(t)
	req := r.continuousRequest(domain.RequestTypeChangeRequest, domain.RequestSizeM)
	plan := r.plan(req)
	p1, p2, p3 := r.phase(req, plan, "p1"), r.phase(req, plan, "p2"), r.phase(req, plan, "p3")
	t1, t2, t3 := r.leaf(req, p1, "t1", domain.TaskStatusOpen), r.leaf(req, p2, "t2", domain.TaskStatusOpen), r.leaf(req, p3, "t3", domain.TaskStatusOpen)

	started, err := r.startExec.Execute(lcCtx(), req.ID)
	if err != nil || !started {
		t.Fatalf("a continuous run starts by itself: started=%v err=%v", started, err)
	}
	if got := pendingPhaseApprovals(r); len(got) != 0 {
		t.Fatalf("no Phase approval is opened in continuous mode, got %v", got)
	}
	if r.tasks.get(t1.ID).Status != domain.TaskStatusInProgress || r.tasks.get(t2.ID).Status != domain.TaskStatusOpen {
		t.Fatalf("only the first Phase runs: %v", r.tasks.calls)
	}

	r.finishPhaseTask(req, t1)
	if r.tasks.get(t2.ID).Status != domain.TaskStatusInProgress || r.tasks.get(t3.ID).Status != domain.TaskStatusOpen {
		t.Fatalf("finishing Phase 1 starts Phase 2 and not Phase 3: %v", r.tasks.calls)
	}
	r.finishPhaseTask(req, t2)
	if r.tasks.get(t3.ID).Status != domain.TaskStatusInProgress {
		t.Fatalf("finishing Phase 2 starts Phase 3: %v", r.tasks.calls)
	}
	r.finishPhaseTask(req, t3)

	if got := r.reload(req).Status; got != domain.RequestStatusCompleted {
		t.Fatalf("status after the last Phase = %s, want completed", got)
	}
	rows, _ := r.starts.ListByRequest(lcCtx(), req.ID)
	if len(rows) != 3 {
		t.Fatalf("phase_starts = %d rows, want 3", len(rows))
	}
	for i, row := range rows {
		if row.Trigger != domain.PhaseStartAuto || row.StartedBy != "approver-1" {
			t.Errorf("start %d: trigger=%q by=%q, want auto by the plan approver", i, row.Trigger, row.StartedBy)
		}
	}
}

func TestContinuous_StepByStepStaysTheDefault(t *testing.T) {
	r := newExRig(t)
	req := r.request(domain.RequestTypeChangeRequest, domain.RequestSizeM, domain.RequestStatusExecuting)
	ph := r.phase(req, r.plan(req), "p1")
	task := r.leaf(req, ph, "t", domain.TaskStatusOpen)
	if started, err := r.startExec.Execute(lcCtx(), req.ID); err != nil || started {
		t.Fatalf("an unpinned Request without a mode setting waits for a person: %v %v", started, err)
	}
	if got := pendingPhaseApprovals(r); len(got) != 1 || got[0] != ph.ID {
		t.Fatalf("the Phase approval still opens: %v", got)
	}
	if r.tasks.get(task.ID).Status != domain.TaskStatusOpen || len(r.starts.rows) != 0 {
		t.Fatal("nothing starts without a person")
	}
}

func TestContinuous_SizeLBugStartsItsPhasesWithoutAButton(t *testing.T) {
	r := newExRig(t)
	req := r.continuousRequest(domain.RequestTypeBug, domain.RequestSizeL)
	plan := r.plan(req)
	p1, p2 := r.phase(req, plan, "p1"), r.phase(req, plan, "p2")
	t1, t2 := r.leaf(req, p1, "t1", domain.TaskStatusOpen), r.leaf(req, p2, "t2", domain.TaskStatusOpen)
	if started, err := r.startExec.Execute(lcCtx(), req.ID); err != nil || !started {
		t.Fatalf("got %v %v", started, err)
	}
	r.finishPhaseTask(req, t1)
	if r.tasks.get(t2.ID).Status != domain.TaskStatusInProgress {
		t.Fatalf("second Phase runs after the first: %v", r.tasks.calls)
	}
}

func TestContinuous_RespectsDependsOnWhenTheListOrderDiffers(t *testing.T) {
	r := newExRig(t)
	req := r.continuousRequest(domain.RequestTypeChangeRequest, domain.RequestSizeM)
	plan := r.plan(req)
	dependent := r.phase(req, plan, "tests, after the module")
	module := r.phase(req, plan, "the module")
	r.dependsOn(dependent, module)
	tDep := r.leaf(req, dependent, "t-tests", domain.TaskStatusBlocked)
	tMod := r.leaf(req, module, "t-module", domain.TaskStatusOpen)
	if _, err := r.startExec.Execute(lcCtx(), req.ID); err != nil {
		t.Fatal(err)
	}
	if r.tasks.get(tMod.ID).Status != domain.TaskStatusInProgress {
		t.Fatalf("the Phase nothing waits on starts first: %v", r.tasks.calls)
	}
	if r.tasks.get(tDep.ID).Status == domain.TaskStatusInProgress {
		t.Fatal("the dependent Phase must wait")
	}
	r.finishPhaseTask(req, tMod)
	rows, _ := r.starts.ListByRequest(lcCtx(), req.ID)
	if len(rows) != 2 || rows[1].PhaseTaskID != dependent.ID {
		t.Fatalf("the dependent Phase is released once its predecessor is done: %+v", rows)
	}
}

func TestContinuous_DuplicateDeliveryStartsEachPhaseOnce(t *testing.T) {
	r := newExRig(t)
	req := r.continuousRequest(domain.RequestTypeChangeRequest, domain.RequestSizeM)
	plan := r.plan(req)
	r.leaf(req, r.phase(req, plan, "p1"), "t1", domain.TaskStatusOpen)
	r.leaf(req, r.phase(req, plan, "p2"), "t2", domain.TaskStatusOpen)
	for i := 0; i < 4; i++ {
		if _, err := r.startExec.Execute(lcCtx(), req.ID); err != nil {
			t.Fatal(err)
		}
		if err := r.evaluate.Run(lcCtx(), r.reload(req)); err != nil {
			t.Fatal(err)
		}
	}
	rows, _ := r.starts.ListByRequest(lcCtx(), req.ID)
	if len(rows) != 1 || r.tasks.count("execute:") != 1 {
		t.Fatalf("one Phase running, repeated evaluation changes nothing: starts=%d calls=%v", len(rows), r.tasks.calls)
	}
}

func TestContinuous_APendingPhaseApprovalPausesTheRun(t *testing.T) {
	r := newExRig(t)
	req := r.continuousRequest(domain.RequestTypeChangeRequest, domain.RequestSizeM)
	plan := r.plan(req)
	p1, p2 := r.phase(req, plan, "p1"), r.phase(req, plan, "p2")
	t1, t2 := r.leaf(req, p1, "t1", domain.TaskStatusOpen), r.leaf(req, p2, "t2", domain.TaskStatusOpen)
	if _, err := r.startExec.Execute(lcCtx(), req.ID); err != nil {
		t.Fatal(err)
	}
	// A drift review opens an Approval on the Phase that ran; a person has to look before anything else starts.
	r.gates.add(domain.Approval{RequestID: req.ID, SubjectType: domain.SubjectPhase, SubjectID: p1.ID, Status: domain.ApprovalStatusPending})
	r.finishPhaseTask(req, t1)
	if r.tasks.get(t2.ID).Status == domain.TaskStatusInProgress {
		t.Fatalf("the next Phase must wait for the pending Phase approval: %v", r.tasks.calls)
	}
}

func TestContinuous_ATaskThatRunsOutOfAttemptsStopsTheRun(t *testing.T) {
	r := newExRig(t)
	req := r.continuousRequest(domain.RequestTypeChangeRequest, domain.RequestSizeM)
	plan := r.plan(req)
	p1, p2 := r.phase(req, plan, "p1"), r.phase(req, plan, "p2")
	t1, t2 := r.leaf(req, p1, "t1", domain.TaskStatusOpen), r.leaf(req, p2, "t2", domain.TaskStatusOpen)
	if _, err := r.startExec.Execute(lcCtx(), req.ID); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < r.settings.MaxTaskAttempts; i++ {
		r.finishRun(req, r.tasks.get(t1.ID), false, "TIMED_OUT")
	}
	got := r.reload(req)
	if got.Status != domain.RequestStatusRequestBacklog {
		t.Fatalf("status = %s, want request_backlog", got.Status)
	}
	if r.tasks.get(t2.ID).Status != domain.TaskStatusOpen {
		t.Fatalf("no later Phase may start after the run stopped: %v", r.tasks.calls)
	}
}

func TestContinuous_AnActorWhoMayNotExecuteSendsTheRequestBack(t *testing.T) {
	r := newExRig(t)
	r.evaluate.Actors = forbiddenActors{}
	req := r.continuousRequest(domain.RequestTypeChangeRequest, domain.RequestSizeM)
	r.leaf(req, r.phase(req, r.plan(req), "p1"), "t1", domain.TaskStatusOpen)
	if _, err := r.startExec.Execute(lcCtx(), req.ID); err != nil {
		t.Fatalf("the Request goes back instead of failing the delivery: %v", err)
	}
	got := r.reload(req)
	if got.Status != domain.RequestStatusRequestBacklog || got.ReturnedCategory != domain.ReturnCategoryBlockedDependency {
		t.Fatalf("status=%s category=%s, want request_backlog / blocked_dependency", got.Status, got.ReturnedCategory)
	}
}

func TestContinuous_StepByStepForbiddenActorKeepsRetrying(t *testing.T) {
	r := newExRig(t)
	r.evaluate.Actors = forbiddenActors{}
	req := r.request(domain.RequestTypeChangeRequest, domain.RequestSizeM, domain.RequestStatusExecuting)
	ph := r.phase(req, r.plan(req), "p1")
	r.leaf(req, ph, "t1", domain.TaskStatusOpen)
	r.starts.rows = append(r.starts.rows, domain.PhaseStart{RequestID: req.ID, PhaseTaskID: ph.ID, StartedBy: "someone"})
	if err := r.evaluate.Run(lcCtx(), req); err == nil {
		t.Fatal("the old mode still reports the error so the reconcile loop retries")
	}
	if got := r.reload(req).Status; got != domain.RequestStatusExecuting {
		t.Fatalf("a step_by_step Request is not sent back automatically, status = %s", got)
	}
}

func TestPinExecutionMode_ProjectSettingIsFrozenWhenTheRequestStarts(t *testing.T) {
	r := newExRig(t)
	settings := &domain.ProjectEngineSettings{ExecutionMode: domain.ExecutionContinuous}
	r.startExec.Pinner = r.store
	r.startExec.Modes = &ExecutionModeResolver{Settings: staticEngineSettings{settings}}
	req := r.request(domain.RequestTypeChangeRequest, domain.RequestSizeM, domain.RequestStatusExecuting)
	plan := r.plan(req)
	t1 := r.leaf(req, r.phase(req, plan, "p1"), "t1", domain.TaskStatusOpen)
	r.leaf(req, r.phase(req, plan, "p2"), "t2", domain.TaskStatusOpen)

	if started, err := r.startExec.Execute(lcCtx(), req.ID); err != nil || !started {
		t.Fatalf("a project on continuous starts its Request by itself: %v %v", started, err)
	}
	if got := r.reload(req).ExecutionMode; got != domain.ExecutionContinuous {
		t.Fatalf("pinned mode = %q, want continuous", got)
	}
	settings.ExecutionMode = domain.ExecutionStepByStep // an administrator changes the project mid-run
	r.finishPhaseTask(req, t1)
	if len(r.starts.rows) != 2 {
		t.Fatalf("the running Request keeps its pinned mode and starts Phase 2, starts=%d", len(r.starts.rows))
	}
}

func TestPinExecutionMode_NoResolverLeavesTheRequestStepByStep(t *testing.T) {
	r := newExRig(t)
	req := r.request(domain.RequestTypeChangeRequest, domain.RequestSizeM, domain.RequestStatusExecuting)
	r.leaf(req, r.phase(req, r.plan(req), "p1"), "t1", domain.TaskStatusOpen)
	if _, err := r.startExec.Execute(lcCtx(), req.ID); err != nil {
		t.Fatal(err)
	}
	if got := r.reload(req).ExecutionMode; got != "" {
		t.Fatalf("no resolver wired: the mode stays unpinned, got %q", got)
	}
}

func TestStartPhase_ContinuousNeedsNoPhaseApproval(t *testing.T) {
	r := newExRig(t)
	req := r.continuousRequest(domain.RequestTypeChangeRequest, domain.RequestSizeM)
	ph := r.phase(req, r.plan(req), "p1")
	task := r.leaf(req, ph, "t", domain.TaskStatusOpen)
	if _, err := r.startPh.Execute(r.ctx(), StartPhaseInput{RequestID: req.ID, PhaseTaskID: ph.ID}); err != nil {
		t.Fatalf("a person may still press Start without an approval in continuous mode: %v", err)
	}
	if r.tasks.get(task.ID).Status != domain.TaskStatusInProgress {
		t.Fatalf("the Phase runs: %v", r.tasks.calls)
	}
	if rows := r.starts.rows; len(rows) != 1 || rows[0].Trigger != domain.PhaseStartManual {
		t.Fatalf("a person's click is recorded as manual: %+v", rows)
	}
}

func TestStartPhase_InternalIsRecordedAsAuto(t *testing.T) {
	r := newExRig(t)
	req := r.request(domain.RequestTypeBug, domain.RequestSizeL, domain.RequestStatusExecuting)
	ph := r.phase(req, r.plan(req), "p1")
	r.leaf(req, ph, "t", domain.TaskStatusOpen)
	if _, err := r.startPh.Execute(lcCtx(), StartPhaseInput{RequestID: req.ID, PhaseTaskID: ph.ID, Internal: true, ActorID: "approver-1"}); err != nil {
		t.Fatal(err)
	}
	if rows := r.starts.rows; len(rows) != 1 || rows[0].Trigger != domain.PhaseStartAuto {
		t.Fatalf("an internal start is auto: %+v", rows)
	}
}

// The same run as above, but the mode comes from the decision that approves the plan instead of being seeded: one
// approval with "continuous" pins the mode, and the Phases then follow without a Phase approval or a Start button.
func TestContinuous_ApprovingThePlanWithTheChoiceRunsEveryPhase(t *testing.T) {
	r := newExRig(t)
	req := r.store.seed(func(q *domain.Request) {
		q.Type, q.Size, q.Status = domain.RequestTypeChangeRequest, domain.RequestSizeM, domain.RequestStatusAwaitingPlanApproval
	})
	plan := r.plan(req)
	p1, p2 := r.phase(req, plan, "p1"), r.phase(req, plan, "p2")
	t1, t2 := r.leaf(req, p1, "t1", domain.TaskStatusOpen), r.leaf(req, p2, "t2", domain.TaskStatusOpen)

	s := &apprStore{lcStore: r.store, approvals: map[string]domain.Approval{}, approvers: map[string][]domain.Principal{}}
	locker := apprLocker{s}
	registry := NewSubjectHandlerRegistry()
	registry.Register(domain.SubjectPlan, &TransitionSubjectHandler{Artifacts: &fakeArtifacts{}, Transition: newLcTransition(r.store), Requests: locker})
	decide := &DecideApproval{Repo: apprRepo{s}, Tx: s, Locker: locker, Registry: registry, Outbox: s, Authorizer: anyApprover{}, Pinner: s}
	gate := domain.Approval{
		ID: "gate-1", TenantID: "t1", RequestID: req.ID, SubjectType: domain.SubjectPlan, SubjectID: plan.ID, Stage: string(domain.RequestStatusAwaitingPlanApproval),
		Status: domain.ApprovalStatusPending, Version: 1, SubjectDigest: "d",
	}
	s.approvals[gate.ID] = gate

	res, err := decide.Execute(lcCtxWithUser("approver-1", "admin"), DecideApprovalInput{ID: gate.ID, Decision: "approve", ExpectedDigest: "d", ExecutionMode: "continuous"})
	if err != nil {
		t.Fatal(err)
	}
	if res.RequestStatus != domain.RequestStatusExecuting || res.ExecutionMode != domain.ExecutionContinuous {
		t.Fatalf("decision result = %+v", res)
	}

	if started, err := r.startExec.Execute(lcCtx(), req.ID); err != nil || !started {
		t.Fatalf("the run starts by itself: started=%v err=%v", started, err)
	}
	if got := pendingPhaseApprovals(r); len(got) != 0 {
		t.Fatalf("no Phase approval is opened, got %v", got)
	}
	r.finishPhaseTask(req, t1)
	if r.tasks.get(t2.ID).Status != domain.TaskStatusInProgress {
		t.Fatalf("the second Phase follows the first: %v", r.tasks.calls)
	}
	r.finishPhaseTask(req, t2)
	got := r.reload(req)
	if got.Status != domain.RequestStatusCompleted || got.ExecutionMode != domain.ExecutionContinuous {
		t.Fatalf("status=%s mode=%q, want completed / continuous", got.Status, got.ExecutionMode)
	}
	rows, _ := r.starts.ListByRequest(lcCtx(), req.ID)
	if len(rows) != 2 {
		t.Fatalf("phase_starts = %d rows, want 2", len(rows))
	}
	for i, row := range rows {
		if row.Trigger != domain.PhaseStartAuto {
			t.Errorf("start %d: trigger=%q, want auto", i, row.Trigger)
		}
	}
}

// anyApprover lets the decision through: who may approve is covered by the authorizer's own tests.
type anyApprover struct{}

func (anyApprover) CanDecide(context.Context, domain.Request, domain.Approval) error { return nil }
