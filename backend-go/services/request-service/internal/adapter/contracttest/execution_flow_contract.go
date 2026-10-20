package contracttest

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

// dbOpener stands in for OpenApproval (which needs policies and directories): it keeps one pending approval per
// subject in the real approvals table, which is the property the execution loop relies on.
type dbOpener struct {
	env ExecutionEnv
	// phase is the real Phase subject validation OpenApproval runs: it reads the tree from task-service as the context's user.
	phase usecase.SubjectArtifacts
}

func (o *dbOpener) Execute(ctx context.Context, in usecase.OpenApprovalInput) (*domain.Approval, error) {
	tenantID, _ := tenant.TenantID(ctx)
	if existing, err := o.env.Approvals.FindPendingBySubject(ctx, tenantID, in.SubjectType, in.SubjectID); err != nil || existing != nil {
		return existing, err
	}
	req, err := o.env.Requests.Get(ctx, in.RequestID)
	if err != nil {
		return nil, err
	}
	if in.SubjectType == domain.SubjectPhase && o.phase != nil {
		if _, _, err := o.phase.Describe(ctx, req, in.SubjectType, in.SubjectID); err != nil {
			return nil, err
		}
	}
	now := usec()
	a := domain.Approval{
		ID: uuid.NewString(), TenantID: tenantID, RequestID: req.ID, SubjectType: in.SubjectType, SubjectID: in.SubjectID, Stage: string(req.Status),
		Status: domain.ApprovalStatusPending, RequestedBy: in.RequestedBy, Version: 1, SubjectDigest: "d", SelfApprovalAllowed: true, CreatedAt: now, UpdatedAt: now,
	}
	if err := o.env.Approvals.Insert(ctx, a); err != nil {
		return nil, err
	}
	return &a, nil
}

type flowRig struct {
	t        *testing.T
	env      ExecutionEnv
	tasks    *FakeTaskService
	tenantID string
	ctx      context.Context
	settings usecase.ExecutionSettings

	report    *usecase.ReportTaskOutcome
	startPh   *usecase.StartPhase
	startExec *usecase.StartExecution
	evaluate  *usecase.EvaluateExecution
	advance   *usecase.AdvanceExecution
	reconcile func(owner string) *usecase.ReconcileExecutingRequests

	deliver atomic.Bool
	pending sync.WaitGroup
	queue   chan func()
}

type allowStart struct{}

func (allowStart) CanStart(context.Context, domain.Request, *domain.Approval) error { return nil }

func newFlowRig(t *testing.T, env ExecutionEnv) *flowRig {
	t.Helper()
	r := &flowRig{t: t, env: env, tenantID: newTenant(), tasks: StartFakeTaskService(t), settings: usecase.DefaultExecutionSettings(), queue: make(chan func(), 1024)}
	r.settings.ReconcileQuiet = 200 * time.Millisecond
	r.ctx = tenOf(r.tenantID)
	r.deliver.Store(true)

	client := r.tasks.Client
	transition := usecase.NewTransitionRequest(env.Requests, env.TxScope, env.Outbox)
	guard := &usecase.TaskExecutionGuard{Tasks: client}
	returner := usecase.NewReturnRequestToBacklog(env.Requests, transition, env.Returns, noopCanceller{}, guard, env.Tx, env.Outbox)
	policies := domain.NewPolicyRegistry(domain.PolicyDeps{
		Checks:    &usecase.CheckReaderFromRepository{Checks: env.Checks},
		Approvals: &usecase.ApprovalLookupFromGateReader{Approvals: env.GateApprovals},
	})
	spawn := usecase.NewSpawnChildRequest(env.Requests, env.Links, usecase.NewIdempotentChildCreator(env.Requests, env.Idempotency, env.Outbox), env.Tx)
	opener := &dbOpener{env: env, phase: &usecase.PhaseArtifacts{Tasks: client}}
	access := &usecase.ExecutionAccess{Tasks: client}
	advance := &usecase.AdvanceExecution{
		Requests: env.Requests, Tasks: client, Outcomes: env.Outcomes, PhaseStarts: env.PhaseStarts, Policies: policies, Approvals: opener, Settings: r.settings,
	}
	r.advance = advance
	r.evaluate = &usecase.EvaluateExecution{
		Tasks: client, Outcomes: env.Outcomes, PhaseStarts: env.PhaseStarts, Checks: env.Checks, Approvals: env.GateApprovals, Open: opener, Advance: advance,
		Actors: &usecase.ApprovalExecutionActors{Approvals: env.GateApprovals, PhaseStarts: env.PhaseStarts, Access: access}, Policies: policies, Transition: transition,
		Returner: returner, FollowUps: &usecase.SpawnFollowUps{Spawn: spawn}, Tx: env.Tx, Outbox: env.Outbox, Settings: r.settings,
	}
	r.report = &usecase.ReportTaskOutcome{
		Requests: env.Requests, Tasks: client, Outcomes: env.Outcomes, Processed: env.Processed, Evaluate: r.evaluate, Tx: env.Tx, Outbox: env.Outbox, Settings: r.settings,
	}
	r.startPh = &usecase.StartPhase{
		Requests: env.Requests, Tasks: client, PhaseStarts: env.PhaseStarts, Approvals: env.GateApprovals, Authorizer: allowStart{}, Advance: advance, Access: access, Tx: env.Tx, Outbox: env.Outbox,
	}
	r.startExec = &usecase.StartExecution{Requests: env.Requests, Evaluate: r.evaluate}
	r.reconcile = func(owner string) *usecase.ReconcileExecutingRequests {
		return &usecase.ReconcileExecutingRequests{Scanner: env.Scanner, Leases: env.Leases, Requests: env.Requests, Evaluate: r.evaluate, Settings: r.settings, Owner: owner}
	}

	// Events travel through a queue, like NATS: delivering one inside the RPC that caused it would deadlock on the request lock.
	go func() {
		for fn := range r.queue {
			fn()
		}
	}()
	t.Cleanup(func() { close(r.queue) })
	r.tasks.OnChange = func(c StatusChange) {
		if !r.deliver.Load() {
			return
		}
		r.pending.Add(1)
		r.queue <- func() {
			defer r.pending.Done()
			err := r.report.Execute(r.ctx, usecase.ReportTaskOutcomeInput{
				EventID: uuid.NewString(), RequestID: c.RequestID, TaskID: c.TaskID, TaskType: c.TaskType, ParentID: c.ParentID, PreviousStatus: c.Previous,
				NewStatus: c.New, Cause: c.Cause, ExecutionLinkID: c.LinkID, ErrorMessage: c.Error, OccurredAt: time.Now().UTC(),
			})
			if err != nil {
				t.Errorf("ReportTaskOutcome(%s %s->%s): %v", c.Cause, c.Previous, c.New, err)
			}
		}
	}
	return r
}

// settle waits until every queued event, and the events its handling caused, has been processed.
func (r *flowRig) settle() { r.pending.Wait() }

func (r *flowRig) request(typ domain.RequestType, size domain.RequestSize) domain.Request {
	return seedExecuting(r.t, r.env, r.ctx, func(q *domain.Request) {
		q.Type, q.Size = typ, size
		q.UpdatedAt = usec().Add(-time.Hour) // quiet, so the reconcile scan may pick it up
	})
}

func (r *flowRig) reload(req domain.Request) domain.Request {
	r.t.Helper()
	got, err := r.env.Requests.Get(r.ctx, req.ID)
	if err != nil {
		r.t.Fatal(err)
	}
	return got
}

// approve records an approved Approval decided by decider.
func (r *flowRig) approve(req domain.Request, st domain.SubjectType, subjectID, decider string) {
	r.t.Helper()
	now := usec()
	a := domain.Approval{
		ID: uuid.NewString(), TenantID: r.tenantID, RequestID: req.ID, SubjectType: st, SubjectID: subjectID, Stage: string(domain.RequestStatusExecuting),
		Status: domain.ApprovalStatusPending, RequestedBy: uuid.NewString(), Version: 1, SubjectDigest: "d", SelfApprovalAllowed: true, CreatedAt: now, UpdatedAt: now,
	}
	if err := r.env.Approvals.Insert(r.ctx, a); err != nil {
		r.t.Fatal(err)
	}
	r.decide(a, decider)
}

func (r *flowRig) decide(a domain.Approval, decider string) {
	r.t.Helper()
	version := a.Version
	if err := a.Approve(decider, "ok", usec()); err != nil {
		r.t.Fatal(err)
	}
	if ok, err := r.env.Approvals.UpdateDecision(r.ctx, a, version); err != nil || !ok {
		r.t.Fatalf("decide: %v %v", ok, err)
	}
}

func (r *flowRig) approvals(req domain.Request, st domain.SubjectType, subjectID string) []domain.Approval {
	r.t.Helper()
	all, err := r.env.GateApprovals.ListGateApprovals(r.ctx, r.tenantID, []string{req.ID})
	if err != nil {
		r.t.Fatal(err)
	}
	var out []domain.Approval
	for _, a := range all {
		if a.SubjectType == st && a.SubjectID == subjectID {
			out = append(out, a)
		}
	}
	return out
}

func (r *flowRig) outboxCount(subject string) int {
	return countSubject(r.env.OutboxRows(r.t, r.tenantID), subject)
}

func actingAs(ctx context.Context, user string) context.Context { return tenant.WithUserID(ctx, user) }

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// RunExecutionFlowContract runs the execution loop end to end: real repositories, the real task-service client against
// an in-memory task-service over bufconn, and the real use cases. Events reach ReportTaskOutcome through a queue.
func RunExecutionFlowContract(t *testing.T, newEnv func(t *testing.T) ExecutionEnv) {
	env := newEnv(t)
	scenarios := []struct {
		name string
		fn   func(t *testing.T, env ExecutionEnv)
	}{
		{"ExecutionFlow_ChangeRequest_ToCompleted", flowChangeRequestToCompleted},
		{"ExecutionFlow_Continuous_OneApprovalRunsEveryPhase", flowContinuousChangeRequest},
		{"ExecutionFlow_Continuous_FailureStopsTheRun", flowContinuousFailureStops},
		{"ExecutionFlow_RunAll_OneDecisionPinsAndRunsEveryPhase", flowRunAllFromTheDecision},
		{"ExecutionFlow_TaskFailsTwice_ToBacklog", flowTaskFailsTwice},
		{"ExecutionFlow_LostEvents_ReconcileFixes", flowLostEvents},
		{"ExecutionFlow_ReleasedTask_ReconcileRedispatches", flowReleasedTask},
		{"ExecutionFlow_TwoReplicasReconcileOnce", flowTwoReplicas},
		{"ExecutionFlow_DuplicateEventsAreHarmless", flowDuplicateEvents},
		{"ExecutionFlow_DispatchOutageThenRecovery", flowDispatchOutage},
		{"ExecutionFlow_ApproverAndStarterAreNotTheCommitter", flowApproverIsNotTheCommitter},
		{"ExecutionFlow_BaseRef_ReachesTaskServiceOnBothDispatchPaths", flowBaseRefOnBothPaths},
	}
	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) { s.fn(t, env) })
	}
}

func flowChangeRequestToCompleted(t *testing.T, env ExecutionEnv) {
	r := newFlowRig(t, env)
	req := r.request(domain.RequestTypeChangeRequest, domain.RequestSizeM)
	plan := r.tasks.Add(req.ID, "plan", "plan", "", "open")
	p1 := r.tasks.Add(req.ID, "phase", "phase 1", plan, "open")
	p2 := r.tasks.Add(req.ID, "phase", "phase 2", plan, "open")
	t1 := r.tasks.Add(req.ID, "task", "t1", p1, "open")
	t2 := r.tasks.Add(req.ID, "task", "t2", p1, "blocked")
	t3 := r.tasks.Add(req.ID, "task", "t3", p2, "open")
	r.tasks.DependsOn(t2, t1)
	planApprover, starter := uuid.NewString(), uuid.NewString()
	// Like task-service, show the Plan tree only to the two people who act on it: a background context sees nothing.
	r.tasks.AllowTreeReaders(planApprover, starter)
	r.approve(req, domain.SubjectPlan, plan, planApprover)
	r.approve(req, domain.SubjectPhase, p1, planApprover)

	if res, err := r.startPh.Execute(actingAs(r.ctx, starter), usecase.StartPhaseInput{RequestID: req.ID, PhaseTaskID: p1}); err != nil || res.AlreadyStarted || len(res.Dispatched) != 1 || res.Dispatched[0] != t1 {
		t.Fatalf("StartPhase: %+v %v", res, err)
	}
	r.settle()
	if r.tasks.Task(t1).Status != "in_progress" || r.tasks.Task(t2).Status != "blocked" {
		t.Fatalf("t1 runs, t2 waits for it: %s %s", r.tasks.Task(t1).Status, r.tasks.Task(t2).Status)
	}
	if got := r.tasks.ExecutedBy(); len(got) != 1 || got[0] != starter {
		t.Fatalf("task-service must see the person who started the phase: %v", got)
	}

	r.tasks.RunSucceeded(t1)
	r.settle()
	if r.tasks.Task(t1).Status != "done" {
		t.Fatalf("t1 in review is completed automatically, got %s", r.tasks.Task(t1).Status)
	}
	second := r.tasks.Task(t2)
	if second.Status != "in_progress" {
		t.Fatalf("the unblocked second task runs: %s", second.Status)
	}
	if second.WorktreeId == "" || second.WorktreeId != r.tasks.Task(t1).WorktreeId {
		t.Fatalf("the second task of the Plan runs in the same worktree as the first: %q vs %q", second.WorktreeId, r.tasks.Task(t1).WorktreeId)
	}

	r.tasks.RunSucceeded(t2)
	r.settle()
	if r.tasks.Task(p1).Status != "done" {
		t.Fatalf("phase 1 derived done, got %s", r.tasks.Task(p1).Status)
	}
	if n := r.outboxCount(domain.SubjectPhaseCompleted); n != 1 {
		t.Fatalf("phase.completed once, got %d", n)
	}
	pending := r.approvals(req, domain.SubjectPhase, p2)
	if len(pending) != 1 || pending[0].Status != domain.ApprovalStatusPending {
		t.Fatalf("one pending approval for phase 2: %+v", pending)
	}
	if got := r.reload(req); got.Status != domain.RequestStatusExecuting {
		t.Fatalf("the request waits in executing for phase 2, got %s", got.Status)
	}
	if r.tasks.Task(t3).Status != "open" {
		t.Fatal("phase 2 must not run before it is approved and started")
	}

	r.decide(pending[0], planApprover)
	if _, err := r.startPh.Execute(actingAs(r.ctx, starter), usecase.StartPhaseInput{RequestID: req.ID, PhaseTaskID: p2}); err != nil {
		t.Fatalf("StartPhase 2: %v", err)
	}
	r.settle()
	r.tasks.RunSucceeded(t3)
	r.settle()

	if got := r.reload(req); got.Status != domain.RequestStatusCompleted {
		t.Fatalf("every phase done completes the request, got %s", got.Status)
	}
	if r.outboxCount(domain.SubjectRequestCompleted) != 1 || r.outboxCount(domain.SubjectPhaseCompleted) != 2 || r.outboxCount(domain.SubjectPhaseStarted) != 2 {
		t.Fatalf("events: completed=%d phase.completed=%d phase.started=%d", r.outboxCount(domain.SubjectRequestCompleted), r.outboxCount(domain.SubjectPhaseCompleted), r.outboxCount(domain.SubjectPhaseStarted))
	}
	if ok, _ := env.Outcomes.Exists(r.ctx, plan, domain.OutcomePlanDone); !ok {
		t.Fatal("plan_done must be recorded")
	}
	for _, ref := range r.tasks.Executed() {
		if !strings.HasPrefix(ref, "req:"+req.ID+":") || !strings.HasSuffix(ref, ":1") {
			t.Errorf("execution reference %q should be req:<request>:<task>:<attempt>", ref)
		}
	}
}

func flowTaskFailsTwice(t *testing.T, env ExecutionEnv) {
	r := newFlowRig(t, env)
	req := r.request(domain.RequestTypeBug, domain.RequestSizeS)
	plan := r.tasks.Add(req.ID, "plan", "plan", "", "open")
	task := r.tasks.Add(req.ID, "task", "flaky build", plan, "open")
	r.approve(req, domain.SubjectPlan, plan, uuid.NewString())

	if started, err := r.startExec.Execute(r.ctx, req.ID); err != nil || !started {
		t.Fatalf("start: %v %v", started, err)
	}
	r.settle()
	r.tasks.RunFailed(task, "compile error 1")
	r.settle()
	if r.tasks.Task(task).Status != "in_progress" {
		t.Fatalf("attempt 1 failed: the task runs again, got %s", r.tasks.Task(task).Status)
	}
	if got := r.reload(req); got.Status != domain.RequestStatusExecuting {
		t.Fatalf("one failure is not enough to give up: %s", got.Status)
	}
	r.tasks.RunFailed(task, "compile error 2")
	r.settle()
	got := r.reload(req)
	if got.Status != domain.RequestStatusRequestBacklog || got.ReturnedFromStage != domain.ReturnStageTask || got.ReturnedCategory != domain.ReturnCategoryOther ||
		!strings.Contains(got.ReturnReason, "compile error 2") || !strings.Contains(got.ReturnReason, "flaky build") {
		t.Fatalf("out of attempts the request returns to the backlog with the last error: %+v", got)
	}
	refs := r.tasks.Executed()
	if len(refs) != 2 || !strings.HasSuffix(refs[0], ":1") || !strings.HasSuffix(refs[1], ":2") {
		t.Fatalf("two attempts, numbered: %v", refs)
	}
	if n, _ := env.Outcomes.CountFailed(r.ctx, task); n != 2 {
		t.Fatalf("two failures recorded, got %d", n)
	}
}

func flowLostEvents(t *testing.T, env ExecutionEnv) {
	r := newFlowRig(t, env)
	req := r.request(domain.RequestTypeTask, domain.RequestSizeS)
	plan := r.tasks.Add(req.ID, "plan", "plan", "", "open")
	task := r.tasks.Add(req.ID, "task", "t", plan, "open")
	r.approve(req, domain.SubjectTaskList, plan, uuid.NewString())

	r.deliver.Store(false) // the consumer is down: nothing reaches request-service
	if _, err := r.startExec.Execute(r.ctx, req.ID); err != nil {
		t.Fatal(err)
	}
	r.tasks.RunSucceeded(task)
	if got := r.reload(req); got.Status != domain.RequestStatusExecuting || r.tasks.Task(task).Status != "review" {
		t.Fatalf("with the events lost the request is stuck: %s / %s", got.Status, r.tasks.Task(task).Status)
	}
	// An immediate pass finds the request busy only if it was touched recently; the seeded request is old, so one pass is enough.
	r.deliver.Store(true)
	if n, err := r.reconcile("replica-1").Execute(context.Background()); err != nil || n < 1 {
		t.Fatalf("reconcile: %d %v", n, err)
	}
	r.settle()
	if r.tasks.Task(task).Status != "done" {
		t.Fatalf("reconcile completes the task in review: %s", r.tasks.Task(task).Status)
	}
	if got := r.reload(req); got.Status != domain.RequestStatusCompleted {
		t.Fatalf("reconcile brings the request to its true state within one pass, got %s", got.Status)
	}
}

func flowReleasedTask(t *testing.T, env ExecutionEnv) {
	r := newFlowRig(t, env)
	req := r.request(domain.RequestTypeTask, domain.RequestSizeS)
	plan := r.tasks.Add(req.ID, "plan", "plan", "", "open")
	task := r.tasks.Add(req.ID, "task", "t", plan, "open")
	r.approve(req, domain.SubjectTaskList, plan, uuid.NewString())
	r.deliver.Store(false)
	if _, err := r.startExec.Execute(r.ctx, req.ID); err != nil {
		t.Fatal(err)
	}
	r.tasks.ReleaseUnlinkedSilently(task) // the bulk release emits no event
	if len(r.tasks.Executed()) != 1 || r.tasks.Task(task).Status != "open" {
		t.Fatal("setup")
	}
	r.deliver.Store(true)
	if _, err := r.reconcile("replica-1").Execute(context.Background()); err != nil {
		t.Fatal(err)
	}
	r.settle()
	if len(r.tasks.Executed()) != 2 || r.tasks.Task(task).Status != "in_progress" {
		t.Fatalf("reconcile dispatches the released task again: %v / %s", r.tasks.Executed(), r.tasks.Task(task).Status)
	}
}

func flowTwoReplicas(t *testing.T, env ExecutionEnv) {
	r := newFlowRig(t, env)
	var reqs []domain.Request
	for i := 0; i < 4; i++ {
		req := r.request(domain.RequestTypeTask, domain.RequestSizeS)
		plan := r.tasks.Add(req.ID, "plan", "plan", "", "open")
		r.tasks.Add(req.ID, "task", "t", plan, "open")
		r.approve(req, domain.SubjectTaskList, plan, uuid.NewString())
		reqs = append(reqs, req)
	}
	r.deliver.Store(false)
	var wg sync.WaitGroup
	var handled atomic.Int32
	for _, owner := range []string{"replica-a", "replica-b", "replica-c"} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			n, err := r.reconcile(owner).Execute(context.Background())
			if err != nil {
				t.Errorf("%s: %v", owner, err)
			}
			handled.Add(int32(n))
		}()
	}
	wg.Wait()
	if got := len(r.tasks.Executed()); got != 4 {
		t.Fatalf("each of the 4 tasks is dispatched exactly once however many replicas reconcile: %d", got)
	}
	if handled.Load() < 4 {
		t.Fatalf("every request is handled by some replica, got %d", handled.Load())
	}
	_ = reqs
}

func flowDuplicateEvents(t *testing.T, env ExecutionEnv) {
	r := newFlowRig(t, env)
	req := r.request(domain.RequestTypeTask, domain.RequestSizeS)
	plan := r.tasks.Add(req.ID, "plan", "plan", "", "open")
	task := r.tasks.Add(req.ID, "task", "t", plan, "open")
	r.approve(req, domain.SubjectTaskList, plan, uuid.NewString())
	if _, err := r.startExec.Execute(r.ctx, req.ID); err != nil {
		t.Fatal(err)
	}
	r.settle()
	in := usecase.ReportTaskOutcomeInput{
		EventID: uuid.NewString(), RequestID: req.ID, TaskID: task, TaskType: "task", ParentID: plan, Cause: domain.CauseExecutionFailed, NewStatus: "open", ErrorMessage: "once",
	}
	r.tasks.RunFailed(task, "x") // moves the task back to open and reports a different event
	r.settle()
	before := len(r.tasks.Executed())
	for i := 0; i < 5; i++ {
		if err := r.report.Execute(r.ctx, in); err != nil {
			t.Fatal(err)
		}
	}
	r.settle()
	if n, _ := env.Outcomes.CountFailed(r.ctx, task); n != 2 {
		t.Fatalf("the repeated event counts once on top of the first failure, got %d", n)
	}
	if after := len(r.tasks.Executed()); after > before+1 {
		t.Fatalf("redelivery dispatched more than once: %d -> %d", before, after)
	}
}

func flowDispatchOutage(t *testing.T, env ExecutionEnv) {
	r := newFlowRig(t, env)
	req := r.request(domain.RequestTypeTask, domain.RequestSizeS)
	plan := r.tasks.Add(req.ID, "plan", "plan", "", "open")
	task := r.tasks.Add(req.ID, "task", "t", plan, "open")
	r.approve(req, domain.SubjectTaskList, plan, uuid.NewString())
	noConn := errNoConnection()
	r.tasks.FailExecuteWith(task, noConn)

	if _, err := r.startExec.Execute(r.ctx, req.ID); err != nil {
		t.Fatalf("a transient dispatch failure is not an error: %v", err)
	}
	if n, _ := env.Outcomes.CountFailed(r.ctx, task); n != 0 || len(r.tasks.Executed()) != 0 {
		t.Fatalf("no attempt used, nothing ran: failed=%d executed=%d", n, len(r.tasks.Executed()))
	}
	since, ok, _ := env.Outcomes.DispatchRetrySince(r.ctx, task)
	if !ok || time.Since(since) > time.Minute {
		t.Fatalf("the outage streak is recorded: %v %v", since, ok)
	}
	// The next reconcile pass dispatches it once the connection is back.
	waitFor(t, "the request to become quiet", func() bool {
		refs, _ := env.Scanner.ListQuietExecuting(context.Background(), r.settings.ReconcileQuiet, 100)
		for _, ref := range refs {
			if ref.RequestID == req.ID {
				return true
			}
		}
		return false
	})
	if _, err := r.reconcile("replica-1").Execute(context.Background()); err != nil {
		t.Fatal(err)
	}
	r.settle()
	if r.tasks.Task(task).Status != "in_progress" {
		t.Fatalf("recovered: %s", r.tasks.Task(task).Status)
	}
}

// flowApproverIsNotTheCommitter is the multi-person case: one person commits the Plan (and so owns the tree in
// task-service), another approves it, a third starts the first Phase, a fourth the second. Nobody is given tree
// access by the test; the execution loop has to obtain what task-service demands.
func flowApproverIsNotTheCommitter(t *testing.T, env ExecutionEnv) {
	r := newFlowRig(t, env)
	req := r.request(domain.RequestTypeChangeRequest, domain.RequestSizeM)
	plan := r.tasks.Add(req.ID, "plan", "plan", "", "open")
	p1 := r.tasks.Add(req.ID, "phase", "phase 1", plan, "open")
	p2 := r.tasks.Add(req.ID, "phase", "phase 2", plan, "open")
	t1 := r.tasks.Add(req.ID, "task", "t1", p1, "open")
	t2 := r.tasks.Add(req.ID, "task", "t2", p2, "open")
	r.tasks.DependsOn(p2, p1)
	committer, planApprover, starter1, starter2, outsider := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	r.tasks.OwnTree(plan, committer)
	r.approve(req, domain.SubjectPlan, plan, planApprover)
	r.approve(req, domain.SubjectPhase, p1, planApprover)

	if res, err := r.startPh.Execute(actingAs(r.ctx, starter1), usecase.StartPhaseInput{RequestID: req.ID, PhaseTaskID: p1}); err != nil || len(res.Dispatched) != 1 {
		t.Fatalf("a Phase started by someone who did not commit the Plan must run: %+v %v", res, err)
	}
	r.settle()
	if r.tasks.Task(t1).Status != "in_progress" {
		t.Fatalf("t1 runs, got %s", r.tasks.Task(t1).Status)
	}

	r.tasks.RunSucceeded(t1)
	r.settle()
	if r.tasks.Task(t1).Status != "done" || r.tasks.Task(p1).Status != "done" {
		t.Fatalf("the finished task is completed and its phase derived done: %s %s", r.tasks.Task(t1).Status, r.tasks.Task(p1).Status)
	}
	pending := r.approvals(req, domain.SubjectPhase, p2)
	if len(pending) != 1 || pending[0].Status != domain.ApprovalStatusPending {
		t.Fatalf("the next Phase's gate opens although nobody involved committed the Plan: %+v", pending)
	}
	r.decide(pending[0], planApprover)
	if res, err := r.startPh.Execute(actingAs(r.ctx, starter2), usecase.StartPhaseInput{RequestID: req.ID, PhaseTaskID: p2}); err != nil || len(res.Dispatched) != 1 {
		t.Fatalf("StartPhase 2: %+v %v", res, err)
	}
	r.settle()
	r.tasks.RunSucceeded(t2)
	r.settle()
	if got := r.reload(req); got.Status != domain.RequestStatusCompleted {
		t.Fatalf("every phase done completes the request, got %s", got.Status)
	}

	if got := r.tasks.ExecutedBy(); len(got) != 2 || got[0] != starter1 || got[1] != starter2 {
		t.Fatalf("task-service sees each Phase's starter: %v", got)
	}
	granted := strings.Join(r.tasks.Grants(plan), " ")
	for _, who := range []string{starter1, starter2} {
		if !strings.Contains(granted, who+":GRANT_LEVEL_USER") {
			t.Errorf("%s needs a user-level grant on the Plan, got [%s]", who, granted)
		}
	}
	if strings.Contains(granted, "OWNER") || strings.Contains(granted, "ADMIN") || strings.Contains(granted, outsider) {
		t.Errorf("only execute-level grants for people the Request authorized, got [%s]", granted)
	}
	// Nobody else gains access: an unrelated user is still refused by task-service.
	if ok, err := r.tasks.Client.CanExecute(r.ctx, plan, outsider); err != nil || ok {
		t.Errorf("an outsider must not be able to execute: %v %v", ok, err)
	}
}

// pinContinuous gives the Request the mode StartExecution would pin for a project on continuous.
func (r *flowRig) pinContinuous(req domain.Request) {
	r.t.Helper()
	pinner, ok := r.env.Requests.(usecase.ExecutionModePinner)
	if !ok {
		r.t.Fatal("the request repository must pin execution modes")
	}
	if got, err := pinner.PinExecutionModeIfUnset(r.ctx, req.ID, domain.ExecutionContinuous); err != nil || got != domain.ExecutionContinuous {
		r.t.Fatalf("pin: %q %v", got, err)
	}
}

// flowContinuousChangeRequest is the case of request #1242 in miniature: one approval of the plan, no Phase approval and no
// Start button, and every Phase still runs after the one before it.
func flowContinuousChangeRequest(t *testing.T, env ExecutionEnv) {
	r := newFlowRig(t, env)
	req := r.request(domain.RequestTypeChangeRequest, domain.RequestSizeM)
	plan := r.tasks.Add(req.ID, "plan", "plan", "", "open")
	p1 := r.tasks.Add(req.ID, "phase", "phase 1", plan, "open")
	p2 := r.tasks.Add(req.ID, "phase", "phase 2", plan, "open")
	p3 := r.tasks.Add(req.ID, "phase", "phase 3", plan, "open")
	t1 := r.tasks.Add(req.ID, "task", "t1", p1, "open")
	t2 := r.tasks.Add(req.ID, "task", "t2", p2, "open")
	t3 := r.tasks.Add(req.ID, "task", "t3", p3, "open")
	r.tasks.DependsOn(p3, p2)
	planApprover := uuid.NewString()
	r.tasks.AllowTreeReaders(planApprover)
	r.approve(req, domain.SubjectPlan, plan, planApprover)
	r.pinContinuous(req)

	started, err := r.startExec.Execute(r.ctx, req.ID)
	if err != nil || !started {
		t.Fatalf("StartExecution: started=%v %v", started, err)
	}
	r.settle()
	if r.tasks.Task(t1).Status != "in_progress" || r.tasks.Task(t2).Status != "open" || r.tasks.Task(t3).Status != "open" {
		t.Fatalf("only phase 1 runs: %s %s %s", r.tasks.Task(t1).Status, r.tasks.Task(t2).Status, r.tasks.Task(t3).Status)
	}
	for _, p := range []string{p1, p2, p3} {
		if got := r.approvals(req, domain.SubjectPhase, p); len(got) != 0 {
			t.Fatalf("no Phase approval is opened in continuous mode: %+v", got)
		}
	}
	r.tasks.RunSucceeded(t1)
	r.settle()
	if r.tasks.Task(t2).Status != "in_progress" || r.tasks.Task(t3).Status != "open" {
		t.Fatalf("finishing phase 1 starts phase 2 alone: %s %s", r.tasks.Task(t2).Status, r.tasks.Task(t3).Status)
	}
	r.tasks.RunSucceeded(t2)
	r.settle()
	if r.tasks.Task(t3).Status != "in_progress" {
		t.Fatalf("finishing phase 2 starts phase 3: %s", r.tasks.Task(t3).Status)
	}
	r.tasks.RunSucceeded(t3)
	r.settle()

	if got := r.reload(req); got.Status != domain.RequestStatusCompleted || got.ExecutionMode != domain.ExecutionContinuous {
		t.Fatalf("status=%s mode=%q, want completed / continuous", got.Status, got.ExecutionMode)
	}
	starts, err := env.PhaseStarts.ListByRequest(r.ctx, req.ID)
	if err != nil || len(starts) != 3 {
		t.Fatalf("three starts: %d %v", len(starts), err)
	}
	for _, s := range starts {
		if s.Trigger != domain.PhaseStartAuto || s.StartedBy != planApprover {
			t.Fatalf("each Phase starts by itself as the plan approver: %+v", s)
		}
	}
	if got := r.tasks.ExecutedBy(); len(got) != 3 || got[0] != planApprover || got[1] != planApprover || got[2] != planApprover {
		t.Fatalf("task-service sees the plan approver for every task: %v", got)
	}
	if r.outboxCount(domain.SubjectPhaseStarted) != 3 || r.outboxCount(domain.SubjectPhaseCompleted) != 3 || r.outboxCount(domain.SubjectRequestCompleted) != 1 {
		t.Fatalf("events: started=%d completed=%d request.completed=%d", r.outboxCount(domain.SubjectPhaseStarted), r.outboxCount(domain.SubjectPhaseCompleted), r.outboxCount(domain.SubjectRequestCompleted))
	}
}

func flowContinuousFailureStops(t *testing.T, env ExecutionEnv) {
	r := newFlowRig(t, env)
	req := r.request(domain.RequestTypeChangeRequest, domain.RequestSizeM)
	plan := r.tasks.Add(req.ID, "plan", "plan", "", "open")
	p1 := r.tasks.Add(req.ID, "phase", "phase 1", plan, "open")
	p2 := r.tasks.Add(req.ID, "phase", "phase 2", plan, "open")
	t1 := r.tasks.Add(req.ID, "task", "t1", p1, "open")
	t2 := r.tasks.Add(req.ID, "task", "t2", p2, "open")
	planApprover := uuid.NewString()
	r.tasks.AllowTreeReaders(planApprover)
	r.approve(req, domain.SubjectPlan, plan, planApprover)
	r.pinContinuous(req)
	if _, err := r.startExec.Execute(r.ctx, req.ID); err != nil {
		t.Fatal(err)
	}
	r.settle()
	for i := 0; i < r.settings.MaxTaskAttempts; i++ {
		r.tasks.RunFailed(t1, "execution failed: retryable TIMED_OUT")
		r.settle()
	}
	got := r.reload(req)
	if got.Status != domain.RequestStatusRequestBacklog || got.ReturnedCategory != domain.ReturnCategoryOther {
		t.Fatalf("status=%s category=%s, want request_backlog after the task ran out of attempts", got.Status, got.ReturnedCategory)
	}
	if r.tasks.Task(t2).Status != "open" {
		t.Fatalf("phase 2 must not start after the run stopped: %s", r.tasks.Task(t2).Status)
	}
	if starts, _ := env.PhaseStarts.ListByRequest(r.ctx, req.ID); len(starts) != 1 {
		t.Fatalf("only phase 1 was ever started: %d", len(starts))
	}
}
