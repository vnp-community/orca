package usecase

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// exTasks is an in-memory task-service: tasks in listing order, depends_on edges, and a log of what was asked of it.
type exTasks struct {
	mu          sync.Mutex
	order       []string
	tasks       map[string]domain.TaskView
	deps        map[string][]string // task -> tasks it depends on
	calls       []string
	executed    []string // execution refs
	executedBy  []string // acting user per Execute
	executeErrs map[string][]error
	listErr     error
	listCalls   int
	worktreeSeq int
	// states overrides what ListExecutionStates answers for a task.
	states map[string]ExecutionStateView
	// statesCalls counts ListExecutionStates calls.
	statesCalls int
}

func newExTasks() *exTasks {
	return &exTasks{tasks: map[string]domain.TaskView{}, deps: map[string][]string{}, executeErrs: map[string][]error{}}
}

func (f *exTasks) add(t domain.TaskView) domain.TaskView {
	if t.ID == "" {
		t.ID = uuid.NewString()
	}
	f.order = append(f.order, t.ID)
	f.tasks[t.ID] = t
	return t
}

func (f *exTasks) get(id string) domain.TaskView { return f.tasks[id] }

func (f *exTasks) ListTasks(_ context.Context, q ListTasksQuery) ([]domain.TaskView, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.listCalls++
	for _, id := range q.RequestIDs {
		f.calls = append(f.calls, "list:"+id)
	}
	if f.listErr != nil {
		return nil, f.listErr
	}
	var out []domain.TaskView
	for _, id := range f.order {
		t := f.tasks[id]
		if len(q.RequestIDs) > 0 && !slices.Contains(q.RequestIDs, t.RequestID) {
			continue
		}
		if len(q.TaskTypes) > 0 && !slices.Contains(q.TaskTypes, t.Type) {
			continue
		}
		if q.ParentID != "" && t.ParentID != q.ParentID {
			continue
		}
		out = append(out, t)
	}
	return out, nil
}

func (f *exTasks) GetSubtree(_ context.Context, rootID string) (SubtreeView, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, "subtree:"+rootID)
	if f.listErr != nil {
		return SubtreeView{}, f.listErr
	}
	in := map[string]bool{rootID: true}
	for changed := true; changed; {
		changed = false
		for _, id := range f.order {
			if t := f.tasks[id]; !in[id] && in[t.ParentID] {
				in[id], changed = true, true
			}
		}
	}
	var out SubtreeView
	for _, id := range f.order {
		if in[id] {
			out.Tasks = append(out.Tasks, f.tasks[id])
			for _, d := range f.deps[id] {
				out.DependsOn = append(out.DependsOn, TaskEdge{From: id, To: d})
			}
		}
	}
	return out, nil
}

func (f *exTasks) Execute(ctx context.Context, taskID, ref string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, "execute:"+taskID)
	user, _ := tenant.UserID(ctx)
	f.executedBy = append(f.executedBy, user)
	if errs := f.executeErrs[taskID]; len(errs) > 0 {
		f.executeErrs[taskID] = errs[1:]
		if errs[0] != nil {
			return errs[0]
		}
	}
	t := f.tasks[taskID]
	if domain.IsContainerTaskType(t.Type) {
		panic("Execute called on a container: " + taskID)
	}
	if t.Status != domain.TaskStatusOpen {
		return domain.ErrTaskAlreadyRunning
	}
	if t.WorktreeID == "" {
		f.worktreeSeq++
		t.WorktreeID = fmt.Sprintf("wt-%d", f.worktreeSeq)
	}
	t.Status = domain.TaskStatusInProgress
	f.tasks[taskID] = t
	f.executed = append(f.executed, ref)
	return nil
}

func (f *exTasks) SetWorktree(_ context.Context, taskID, worktreeID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, "worktree:"+taskID+":"+worktreeID)
	t := f.tasks[taskID]
	t.WorktreeID = worktreeID
	f.tasks[taskID] = t
	return nil
}

func (f *exTasks) SetStatus(_ context.Context, taskID, status string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, "status:"+taskID+":"+status)
	f.setStatusLocked(taskID, status)
	return nil
}

// setStatusLocked also unblocks dependents and derives container statuses, as task-service does.
func (f *exTasks) setStatusLocked(taskID, status string) {
	t := f.tasks[taskID]
	t.Status = status
	f.tasks[taskID] = t
	if status == domain.TaskStatusDone {
		for _, id := range f.order {
			d := f.tasks[id]
			if d.Status != domain.TaskStatusBlocked || !slices.Contains(f.deps[id], taskID) {
				continue
			}
			ready := true
			for _, dep := range f.deps[id] {
				ready = ready && f.tasks[dep].Status == domain.TaskStatusDone
			}
			if ready {
				d.Status = domain.TaskStatusOpen
				f.tasks[id] = d
			}
		}
	}
	for t.ParentID != "" {
		parent, ok := f.tasks[t.ParentID]
		if !ok {
			break
		}
		done, any := true, false
		for _, id := range f.order {
			if c := f.tasks[id]; c.ParentID == parent.ID {
				any = true
				done = done && (c.Status == domain.TaskStatusDone || c.Status == domain.TaskStatusCancelled)
			}
		}
		if any && done {
			parent.Status = domain.TaskStatusDone
		} else if parent.Status == domain.TaskStatusDone {
			parent.Status = domain.TaskStatusOpen
		}
		f.tasks[parent.ID] = parent
		t = parent
	}
}

func (f *exTasks) ListExecutionStates(_ context.Context, ids []string) (map[string]ExecutionStateView, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, "states")
	f.statesCalls++
	if f.listErr != nil {
		return nil, f.listErr
	}
	out := map[string]ExecutionStateView{}
	for _, id := range ids {
		if override, ok := f.states[id]; ok {
			out[id] = override
			continue
		}
		st := ExecutionStateView{}
		for _, d := range f.deps[id] {
			if f.tasks[d].Status != domain.TaskStatusDone {
				st.BlockedByTaskIDs = append(st.BlockedByTaskIDs, d)
			}
		}
		out[id] = st
	}
	return out, nil
}

func (f *exTasks) count(prefix string) int {
	n := 0
	for _, c := range f.calls {
		if len(c) >= len(prefix) && c[:len(prefix)] == prefix {
			n++
		}
	}
	return n
}

// exOutcomes mirrors the database rules: event_id is unique and a container completes once.
type exOutcomes struct {
	mu   sync.Mutex
	rows []domain.TaskRunOutcome
}

func (f *exOutcomes) Insert(_ context.Context, o domain.TaskRunOutcome) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, r := range f.rows {
		if r.EventID == o.EventID || (o.Outcome.CompletesContainer() && r.Outcome.CompletesContainer() && r.TaskID == o.TaskID) {
			return false, nil
		}
	}
	f.rows = append(f.rows, o)
	return true, nil
}

func (f *exOutcomes) CountFailed(_ context.Context, taskID string) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, r := range f.rows {
		if r.TaskID == taskID && r.Outcome == domain.OutcomeFailed && r.Cause != domain.CauseDispatchError {
			n++
		}
	}
	return n, nil
}

func (f *exOutcomes) LatestFailed(_ context.Context, ids []string) (map[string]domain.TaskRunOutcome, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := map[string]domain.TaskRunOutcome{}
	for _, r := range f.rows {
		if r.Outcome == domain.OutcomeFailed && slices.Contains(ids, r.TaskID) {
			out[r.TaskID] = r
		}
	}
	return out, nil
}

func (f *exOutcomes) LastEventAt(context.Context, string) (time.Time, error) { return time.Time{}, nil }

func (f *exOutcomes) Exists(_ context.Context, taskID string, outcome domain.Outcome) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, r := range f.rows {
		if r.TaskID == taskID && r.Outcome == outcome {
			return true, nil
		}
	}
	return false, nil
}

func (f *exOutcomes) DispatchRetrySince(_ context.Context, taskID string) (time.Time, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var lastStart, first time.Time
	for _, r := range f.rows {
		if r.TaskID == taskID && r.Outcome == domain.OutcomeStarted && r.OccurredAt.After(lastStart) {
			lastStart = r.OccurredAt
		}
	}
	for _, r := range f.rows {
		if r.TaskID == taskID && r.Cause == domain.CauseDispatchError && r.OccurredAt.After(lastStart) && (first.IsZero() || r.OccurredAt.Before(first)) {
			first = r.OccurredAt
		}
	}
	return first, !first.IsZero(), nil
}

func (f *exOutcomes) LatestDispatchError(_ context.Context, taskID string) (domain.TaskRunOutcome, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var best domain.TaskRunOutcome
	found := false
	for _, r := range f.rows {
		if r.TaskID == taskID && r.Cause == domain.CauseDispatchError && (!found || r.OccurredAt.After(best.OccurredAt)) {
			best, found = r, true
		}
	}
	return best, found, nil
}

func (f *exOutcomes) count(o domain.Outcome) int {
	n := 0
	for _, r := range f.rows {
		if r.Outcome == o {
			n++
		}
	}
	return n
}

type exPhaseStarts struct {
	mu   sync.Mutex
	rows []domain.PhaseStart
}

func (f *exPhaseStarts) TryStart(_ context.Context, s domain.PhaseStart) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, r := range f.rows {
		if r.PhaseTaskID == s.PhaseTaskID {
			return false, nil
		}
	}
	f.rows = append(f.rows, s)
	return true, nil
}

func (f *exPhaseStarts) ListByRequest(_ context.Context, requestID string) ([]domain.PhaseStart, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []domain.PhaseStart
	for _, r := range f.rows {
		if r.RequestID == requestID {
			out = append(out, r)
		}
	}
	return out, nil
}

type exChecks struct {
	mu   sync.Mutex
	rows []domain.RequestCheck
	seq  int
}

func (f *exChecks) Append(_ context.Context, c domain.RequestCheck) (domain.RequestCheck, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.seq++
	c.CreatedAt = time.Date(2026, 10, 1, 0, 0, f.seq, 0, time.UTC)
	f.rows = append(f.rows, c)
	return c, nil
}

func (f *exChecks) ListByRequest(_ context.Context, requestID string) ([]domain.RequestCheck, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []domain.RequestCheck
	for _, r := range f.rows {
		if r.RequestID == requestID {
			out = append(out, r)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}

func (f *exChecks) Latest(ctx context.Context, requestID string, kind domain.CheckKind) (domain.RequestCheck, bool, error) {
	all, _ := f.ListByRequest(ctx, requestID)
	c, ok := domain.LatestCheck(all, kind)
	return c, ok, nil
}

type exGates struct {
	mu        sync.Mutex
	approvals []domain.Approval
	calls     int
}

func (f *exGates) add(a domain.Approval) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if a.ID == "" {
		a.ID = uuid.NewString()
	}
	if a.CreatedAt.IsZero() {
		a.CreatedAt = time.Date(2026, 10, 1, 0, len(f.approvals), 0, 0, time.UTC)
	}
	f.approvals = append(f.approvals, a)
}

func (f *exGates) ListGateApprovals(_ context.Context, _ string, requestIDs []string) ([]domain.Approval, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	var out []domain.Approval
	for _, a := range f.approvals {
		if slices.Contains(requestIDs, a.RequestID) {
			out = append(out, a)
		}
	}
	return out, nil
}

// exOpener records OpenApproval calls and keeps one pending row per subject, as the real use case does.
type exOpener struct {
	gates *exGates
	calls []OpenApprovalInput
	err   error
}

func (f *exOpener) Execute(_ context.Context, in OpenApprovalInput) (*domain.Approval, error) {
	f.calls = append(f.calls, in)
	if f.err != nil {
		return nil, f.err
	}
	for _, a := range f.gates.approvals {
		if a.SubjectType == in.SubjectType && a.SubjectID == in.SubjectID && a.Status == domain.ApprovalStatusPending {
			return &a, nil
		}
	}
	a := domain.Approval{RequestID: in.RequestID, SubjectType: in.SubjectType, SubjectID: in.SubjectID, Status: domain.ApprovalStatusPending, RequestedBy: in.RequestedBy}
	f.gates.add(a)
	return &a, nil
}

type exProcessed struct {
	mu   sync.Mutex
	seen map[string]bool
}

func (f *exProcessed) MarkProcessed(_ context.Context, id, _ string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.seen == nil {
		f.seen = map[string]bool{}
	}
	already := f.seen[id]
	f.seen[id] = true
	return already, nil
}
func (f *exProcessed) Prune(context.Context, time.Time) (int64, error) { return 0, nil }

type exActors struct{ id string }

func (f exActors) ActorFor(context.Context, domain.Request, string) (string, error) { return f.id, nil }

type exFollowUps struct {
	calls [][]domain.FollowUp
	err   error
}

func (f *exFollowUps) ExecuteFollowUps(_ context.Context, _ domain.Request, fus []domain.FollowUp) error {
	f.calls = append(f.calls, fus)
	return f.err
}

type exScanner struct{ refs []ExecutingRef }

func (f *exScanner) ListQuietExecuting(context.Context, time.Duration, int) ([]ExecutingRef, error) {
	return f.refs, nil
}

type exLeases struct {
	mu    sync.Mutex
	held  map[string]string
	claim int
}

func (f *exLeases) Claim(_ context.Context, requestID, owner string, _, _ time.Duration) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.claim++
	if f.held == nil {
		f.held = map[string]string{}
	}
	if _, taken := f.held[requestID]; taken {
		return false, nil
	}
	f.held[requestID] = owner
	return true, nil
}

func (f *exLeases) Release(_ context.Context, requestID, owner string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.held[requestID] == owner {
		delete(f.held, requestID)
	}
	return nil
}

// exRig wires the real execution use cases over the in-memory fakes above and the real transition and return use cases.
type exRig struct {
	t         *testing.T
	store     *lcStore
	tasks     *exTasks
	outcomes  *exOutcomes
	starts    *exPhaseStarts
	checks    *exChecks
	gates     *exGates
	opener    *exOpener
	processed *exProcessed
	follow    *exFollowUps
	guard     *lcGuard
	settings  ExecutionSettings
	clock     time.Time

	advance   *AdvanceExecution
	evaluate  *EvaluateExecution
	report    *ReportTaskOutcome
	startPh   *StartPhase
	startExec *StartExecution
	scanner   *exScanner
	leases    *exLeases
	reconcile *ReconcileExecutingRequests
}

type exAuthorizer struct{ err error }

func (a exAuthorizer) CanStart(context.Context, domain.Request, *domain.Approval) error { return a.err }

func newExRig(t *testing.T, mod ...func(*ExecutionSettings)) *exRig {
	t.Helper()
	r := &exRig{
		t: t, store: newLcStore(), tasks: newExTasks(), outcomes: &exOutcomes{}, starts: &exPhaseStarts{}, checks: &exChecks{}, gates: &exGates{},
		processed: &exProcessed{}, follow: &exFollowUps{}, guard: &lcGuard{}, settings: DefaultExecutionSettings(),
		clock: time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC), scanner: &exScanner{}, leases: &exLeases{},
	}
	for _, m := range mod {
		m(&r.settings)
	}
	r.opener = &exOpener{gates: r.gates}
	policies := domain.NewPolicyRegistry(domain.PolicyDeps{
		Checks:    &CheckReaderFromRepository{Checks: r.checks},
		Approvals: &ApprovalLookupFromGateReader{Approvals: r.gates},
	})
	transition := newLcTransition(r.store)
	returner := NewReturnRequestToBacklog(r.store, transition, lcHistory{r.store}, &lcCanceller{}, r.guard, r.store, r.store)
	now := func() time.Time { return r.clock }
	r.advance = &AdvanceExecution{
		Requests: r.store, Tasks: r.tasks, Outcomes: r.outcomes, PhaseStarts: r.starts, Policies: policies, Approvals: r.opener, Settings: r.settings, Now: now,
	}
	r.evaluate = &EvaluateExecution{
		Tasks: r.tasks, Outcomes: r.outcomes, PhaseStarts: r.starts, Checks: r.checks, Approvals: r.gates, Open: r.opener, Advance: r.advance,
		Actors: exActors{id: "approver-1"}, Policies: policies, Transition: transition, Returner: returner, FollowUps: r.follow,
		Tx: r.store, Outbox: r.store, Settings: r.settings, Now: now,
	}
	r.report = &ReportTaskOutcome{
		Requests: r.store, Tasks: r.tasks, Outcomes: r.outcomes, Processed: r.processed, Evaluate: r.evaluate, Tx: r.store, Outbox: r.store, Settings: r.settings,
	}
	r.startPh = &StartPhase{
		Requests: r.store, Tasks: r.tasks, PhaseStarts: r.starts, Approvals: r.gates, Authorizer: exAuthorizer{}, Advance: r.advance, Tx: r.store, Outbox: r.store,
	}
	r.startExec = &StartExecution{Requests: r.store, Evaluate: r.evaluate}
	r.reconcile = &ReconcileExecutingRequests{Scanner: r.scanner, Leases: r.leases, Requests: r.store, Evaluate: r.evaluate, Settings: r.settings, Owner: "test"}
	return r
}

func (r *exRig) ctx() context.Context { return tenant.WithUserID(lcCtx(), "starter-1") }

func (r *exRig) request(typ domain.RequestType, size domain.RequestSize, status domain.RequestStatus) domain.Request {
	return r.store.seed(func(q *domain.Request) { q.Type, q.Size, q.Status = typ, size, status })
}

func (r *exRig) plan(req domain.Request) domain.TaskView {
	return r.tasks.add(domain.TaskView{Type: domain.TaskTypePlan, Status: domain.TaskStatusOpen, RequestID: req.ID, Title: "plan"})
}

func (r *exRig) phase(req domain.Request, plan domain.TaskView, title string) domain.TaskView {
	return r.tasks.add(domain.TaskView{Type: domain.TaskTypePhase, Status: domain.TaskStatusOpen, RequestID: req.ID, ParentID: plan.ID, Title: title})
}

func (r *exRig) leaf(req domain.Request, parent domain.TaskView, title, status string, labels ...string) domain.TaskView {
	return r.tasks.add(domain.TaskView{Type: "task", Status: status, RequestID: req.ID, ParentID: parent.ID, Title: title, Labels: labels})
}

func (r *exRig) dependsOn(task, on domain.TaskView) {
	r.tasks.deps[task.ID] = append(r.tasks.deps[task.ID], on.ID)
}

// event feeds one task-service status event through ReportTaskOutcome.
func (r *exRig) event(req domain.Request, task domain.TaskView, cause, newStatus, errMsg string) error {
	return r.report.Execute(lcCtx(), ReportTaskOutcomeInput{
		EventID: uuid.NewString(), RequestID: req.ID, TaskID: task.ID, TaskType: task.Type, ParentID: task.ParentID,
		Cause: cause, NewStatus: newStatus, ErrorMessage: errMsg, OccurredAt: r.clock,
	})
}

// finishRun mimics a task-service run ending: success moves the task to review, failure puts it back to open.
func (r *exRig) finishRun(req domain.Request, task domain.TaskView, ok bool, errMsg string) {
	r.t.Helper()
	cause, status := domain.CauseExecutionCompleted, domain.TaskStatusReview
	if !ok {
		cause, status = domain.CauseExecutionFailed, domain.TaskStatusOpen
	}
	r.tasks.mu.Lock()
	t := r.tasks.tasks[task.ID]
	t.Status = status
	r.tasks.tasks[task.ID] = t
	r.tasks.mu.Unlock()
	if err := r.event(req, r.tasks.get(task.ID), cause, status, errMsg); err != nil {
		r.t.Fatalf("report outcome: %v", err)
	}
}

func (r *exRig) reload(req domain.Request) domain.Request {
	got, err := r.store.Get(lcCtx(), req.ID)
	if err != nil {
		r.t.Fatal(err)
	}
	return got
}

func (r *exRig) approved(req domain.Request, st domain.SubjectType, subjectID string) {
	by := "approver-1"
	r.gates.add(domain.Approval{RequestID: req.ID, SubjectType: st, SubjectID: subjectID, Status: domain.ApprovalStatusApproved, DecidedBy: &by})
}

type stubDecider struct{ err error }

func (s stubDecider) CanDecide(context.Context, domain.Request, domain.Approval) error { return s.err }

func lcCtxWithUser(user, role string) context.Context {
	ctx := tenant.WithUserID(lcCtx(), user)
	if role != "" {
		ctx = tenant.WithRole(ctx, role)
	}
	return ctx
}

func withStatus(t domain.TaskView, status string) domain.TaskView {
	t.Status = status
	return t
}
