package usecase

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/stablyai/orca-go/services/orchestration-service/internal/domain"
)

// synchronousSerializer is the fake HandleSerializer suggested by
// orchestration-service.md §6: Do calls fn immediately, no goroutines
// needed to test usecase business logic. It also records the keys it was
// called with so tests can assert usecases key their serialized work
// correctly.
type synchronousSerializer struct {
	mu   sync.Mutex
	keys []string
}

func (s *synchronousSerializer) Do(_ context.Context, key string, fn func() error) error {
	s.mu.Lock()
	s.keys = append(s.keys, key)
	s.mu.Unlock()
	return fn()
}

func (s *synchronousSerializer) calledKeys() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, len(s.keys))
	copy(out, s.keys)
	return out
}

// fakeOrchestrationTaskRepository is an in-memory OrchestrationTaskRepository.
type fakeOrchestrationTaskRepository struct {
	tasks      map[string]domain.OrchestrationTask
	dispatched map[string]bool
	err        error
	// runs, when set, lets UpdateStatusAndPromote detect run finalization
	// against the same fake coordinator-run state a test also asserts on.
	runs *fakeCoordinatorRunRepository
	// enqueuedEvents records every non-zero-value outbox event
	// UpdateStatusAndPromote was asked to enqueue (BE-SOL-003/TASK-FT-003-01)
	// so tests can assert on it.
	enqueuedEvents []domain.OutboxEvent
}

func newFakeOrchestrationTaskRepository(tasks ...domain.OrchestrationTask) *fakeOrchestrationTaskRepository {
	m := make(map[string]domain.OrchestrationTask, len(tasks))
	for _, t := range tasks {
		m[t.ID] = t
	}
	return &fakeOrchestrationTaskRepository{tasks: m}
}

func (f *fakeOrchestrationTaskRepository) Create(_ context.Context, task domain.OrchestrationTask) (domain.OrchestrationTask, error) {
	if f.err != nil {
		return domain.OrchestrationTask{}, f.err
	}
	f.tasks[task.ID] = task
	return task, nil
}

func (f *fakeOrchestrationTaskRepository) Get(_ context.Context, _, id string) (domain.OrchestrationTask, error) {
	t, ok := f.tasks[id]
	if !ok {
		return domain.OrchestrationTask{}, ErrTaskNotFound
	}
	return t, nil
}

// UpdateStatusAndPromote mirrors the real Postgres transaction's semantics
// in memory: update the task, then promote any pending sibling whose deps
// are now all completed, then detect run finalization the same way the
// postgres adapter's new tail does (TASK-TASKV1-005-07). `runs`, when set,
// is consulted/updated exactly like the real coordinator_runs table so
// tests can assert on RunFinalized without a real database.
func (f *fakeOrchestrationTaskRepository) UpdateStatusAndPromote(_ context.Context, tenantID, taskID string, newStatus domain.TaskStatus, event domain.OutboxEvent) (UpdateStatusAndPromoteResult, error) {
	if f.err != nil {
		return UpdateStatusAndPromoteResult{}, f.err
	}
	if event.ID != "" {
		f.enqueuedEvents = append(f.enqueuedEvents, event)
	}
	task, ok := f.tasks[taskID]
	if !ok {
		return UpdateStatusAndPromoteResult{}, ErrTaskNotFound
	}
	task.Status = newStatus
	f.tasks[taskID] = task

	var promoted []string
	if newStatus == domain.TaskStatusCompleted {
		completed := map[string]struct{}{}
		for _, t := range f.tasks {
			if t.CoordinatorRunID == task.CoordinatorRunID && t.Status == domain.TaskStatusCompleted {
				completed[t.ID] = struct{}{}
			}
		}
		for id, t := range f.tasks {
			if t.CoordinatorRunID == task.CoordinatorRunID && t.Status == domain.TaskStatusPending && t.DepsSatisfied(completed) {
				t.Status = domain.TaskStatusReady
				f.tasks[id] = t
				promoted = append(promoted, id)
			}
		}
	}

	var finalized *RunFinalization
	if (newStatus == domain.TaskStatusCompleted || newStatus == domain.TaskStatusFailed) && f.runs != nil {
		nonTerminal := 0
		anyFailed := false
		for _, t := range f.tasks {
			if t.CoordinatorRunID != task.CoordinatorRunID {
				continue
			}
			if t.Status != domain.TaskStatusCompleted && t.Status != domain.TaskStatusFailed {
				nonTerminal++
			}
			if t.Status == domain.TaskStatusFailed {
				anyFailed = true
			}
		}
		if nonTerminal == 0 {
			run, ok := f.runs.runs[task.CoordinatorRunID]
			if ok && run.Status == domain.RunStatusRunning {
				if anyFailed {
					run, _ = run.Fail("")
				} else {
					run, _ = run.Complete(nil)
				}
				f.runs.runs[task.CoordinatorRunID] = run
				finalized = &RunFinalization{
					CoordinatorRunID: task.CoordinatorRunID,
					OriginTaskID:     run.OriginTaskID,
					Success:          !anyFailed,
				}
			}
		}
	}
	_ = tenantID
	return UpdateStatusAndPromoteResult{Task: task, PromotedIDs: promoted, RunFinalized: finalized}, nil
}

// ListReadyUnclaimed returns every 'ready' task with no dispatch context
// recorded via markDispatched (test-only tracking set, mirrors the
// postgres adapter's NOT EXISTS join against dispatch_contexts).
func (f *fakeOrchestrationTaskRepository) ListReadyUnclaimed(_ context.Context) ([]domain.OrchestrationTask, error) {
	if f.err != nil {
		return nil, f.err
	}
	var out []domain.OrchestrationTask
	for _, t := range f.tasks {
		if t.Status == domain.TaskStatusReady && !f.dispatched[t.ID] {
			out = append(out, t)
		}
	}
	return out, nil
}

// ClaimReady mirrors the postgres CAS: succeeds once per ready task, a
// concurrent/second claim on an already-dispatched task loses the race.
func (f *fakeOrchestrationTaskRepository) ClaimReady(_ context.Context, _, taskID string) (domain.OrchestrationTask, bool, error) {
	if f.err != nil {
		return domain.OrchestrationTask{}, false, f.err
	}
	t, ok := f.tasks[taskID]
	if !ok || t.Status != domain.TaskStatusReady {
		return domain.OrchestrationTask{}, false, nil
	}
	t.Status = domain.TaskStatusDispatched
	f.tasks[taskID] = t
	if f.dispatched == nil {
		f.dispatched = map[string]bool{}
	}
	f.dispatched[taskID] = true
	return t, true, nil
}

func (f *fakeOrchestrationTaskRepository) CountNonTerminalByRun(_ context.Context, _, coordinatorRunID string) (int, error) {
	if f.err != nil {
		return 0, f.err
	}
	count := 0
	for _, t := range f.tasks {
		if t.CoordinatorRunID == coordinatorRunID && t.Status != domain.TaskStatusCompleted && t.Status != domain.TaskStatusFailed {
			count++
		}
	}
	return count, nil
}

// fakeDispatchContextRepository is an in-memory DispatchContextRepository.
type fakeDispatchContextRepository struct {
	created []domain.DispatchContext
	err     error

	getLatestReturns domain.DispatchContext
	getLatestErr     error
	getLatestFunc    func(ctx context.Context, tenantID, taskID string) (domain.DispatchContext, error)

	listActiveErr  error
	listActiveFunc func(tenantID, userID string) ([]domain.DispatchContext, error)

	byID              map[string]domain.DispatchContext
	recordFailureErr  error
	recordFailureFunc func(tenantID, dispatchContextID, reason string) (domain.DispatchContext, error)

	heartbeatErr error

	// enqueuedEvents records every non-zero-value outbox event
	// CreateDispatchContext was asked to enqueue (BE-SOL-003/TASK-FT-003-01).
	enqueuedEvents []domain.OutboxEvent

	getDispatchContextErr error
}

func (f *fakeDispatchContextRepository) CreateDispatchContext(_ context.Context, tenantID, userID, worktreeID, handle, coordinatorRunID, orchestrationTaskID string, event domain.OutboxEvent) (domain.DispatchContext, error) {
	if f.err != nil {
		return domain.DispatchContext{}, f.err
	}
	if event.ID != "" {
		f.enqueuedEvents = append(f.enqueuedEvents, event)
	}
	dc := domain.DispatchContext{
		ID:                  "dc-" + handle,
		TenantID:            tenantID,
		UserID:              userID,
		WorktreeID:          worktreeID,
		Handle:              handle,
		CoordinatorRunID:    coordinatorRunID,
		OrchestrationTaskID: orchestrationTaskID,
		Status:              domain.DispatchStatusPending,
	}
	f.created = append(f.created, dc)
	return dc, nil
}

// GetDispatchContext looks up byID first (tests that populate it directly),
// falling back to created — mirroring RecordFailure/RecordHeartbeat's own
// fallback for the same reason (a test that only drives
// CreateDispatchContext never populates byID directly).
func (f *fakeDispatchContextRepository) GetDispatchContext(_ context.Context, _, dispatchContextID string) (domain.DispatchContext, error) {
	if f.getDispatchContextErr != nil {
		return domain.DispatchContext{}, f.getDispatchContextErr
	}
	if dc, ok := f.byID[dispatchContextID]; ok {
		return dc, nil
	}
	for _, c := range f.created {
		if c.ID == dispatchContextID {
			return c, nil
		}
	}
	return domain.DispatchContext{}, ErrDispatchContextNotFound
}

func (f *fakeDispatchContextRepository) GetLatestForTask(ctx context.Context, tenantID, taskID string) (domain.DispatchContext, error) {
	if f.getLatestFunc != nil {
		return f.getLatestFunc(ctx, tenantID, taskID)
	}
	if f.getLatestErr != nil {
		return domain.DispatchContext{}, f.getLatestErr
	}
	return f.getLatestReturns, nil
}

func (f *fakeDispatchContextRepository) ListActiveDispatchContextsForUser(_ context.Context, tenantID, userID string) ([]domain.DispatchContext, error) {
	if f.listActiveErr != nil {
		return nil, f.listActiveErr
	}
	if f.listActiveFunc != nil {
		return f.listActiveFunc(tenantID, userID)
	}
	var out []domain.DispatchContext
	for _, dc := range f.created {
		if dc.TenantID == tenantID && dc.UserID == userID {
			out = append(out, dc)
		}
	}
	return out, nil
}

func (f *fakeDispatchContextRepository) RecordDispatchFailure(_ context.Context, tenantID, dispatchContextID, reason string) (domain.DispatchContext, error) {
	if f.recordFailureErr != nil {
		return domain.DispatchContext{}, f.recordFailureErr
	}
	if f.recordFailureFunc != nil {
		return f.recordFailureFunc(tenantID, dispatchContextID, reason)
	}
	dc, ok := f.byID[dispatchContextID]
	if !ok {
		// Same created-fallback RecordHeartbeat uses below: a test that
		// drives CreateDispatchContext then RecordDispatchFailure in the
		// same flow (e.g. TickDispatch's tests) never populates byID
		// directly.
		for _, c := range f.created {
			if c.ID == dispatchContextID {
				dc = c
				ok = true
				break
			}
		}
	}
	if !ok {
		return domain.DispatchContext{}, ErrDispatchContextNotFound
	}
	updated := dc.RecordFailure(reason)
	if f.byID == nil {
		f.byID = map[string]domain.DispatchContext{}
	}
	f.byID[dispatchContextID] = updated
	return updated, nil
}

// RecordHeartbeat updates last_heartbeat_at on the fake's byID-tracked
// dispatch context, seeding a zero-value entry from `created` if the
// caller never populated byID directly (tests using CreateDispatchContext
// then RecordHeartbeat in the same flow rely on this).
func (f *fakeDispatchContextRepository) RecordHeartbeat(_ context.Context, tenantID, dispatchContextID string) (domain.DispatchContext, error) {
	if f.heartbeatErr != nil {
		return domain.DispatchContext{}, f.heartbeatErr
	}
	if f.byID == nil {
		f.byID = map[string]domain.DispatchContext{}
	}
	dc, ok := f.byID[dispatchContextID]
	if !ok {
		for _, c := range f.created {
			if c.ID == dispatchContextID {
				dc = c
				ok = true
				break
			}
		}
	}
	if !ok {
		return domain.DispatchContext{}, ErrDispatchContextNotFound
	}
	dc.LastHeartbeatAt = time.Now()
	f.byID[dispatchContextID] = dc
	_ = tenantID
	return dc, nil
}

// fakeGateRepository is an in-memory GateRepository.
type fakeGateRepository struct {
	gates map[string]domain.DecisionGate
	err   error
	// enqueuedEvents records every non-zero-value outbox event CreateGate
	// was asked to enqueue (BE-SOL-003/TASK-FT-003-02).
	enqueuedEvents []domain.OutboxEvent
}

func newFakeGateRepository(gates ...domain.DecisionGate) *fakeGateRepository {
	m := make(map[string]domain.DecisionGate, len(gates))
	for _, g := range gates {
		m[g.ID] = g
	}
	return &fakeGateRepository{gates: m}
}

func (f *fakeGateRepository) CreateGate(_ context.Context, tenantID, dispatchContextID, question string, options []string, event domain.OutboxEvent) (domain.DecisionGate, error) {
	if f.err != nil {
		return domain.DecisionGate{}, f.err
	}
	if event.ID != "" {
		f.enqueuedEvents = append(f.enqueuedEvents, event)
	}
	g := domain.DecisionGate{
		ID:                  "gate-" + dispatchContextID,
		TenantID:            tenantID,
		OrchestrationTaskID: "task-for-" + dispatchContextID,
		DispatchContextID:   dispatchContextID,
		Question:            question,
		Options:             options,
		Status:              domain.GateStatusPending,
	}
	f.gates[g.ID] = g
	return g, nil
}

func (f *fakeGateRepository) ResolveGate(_ context.Context, _, gateID, resolution string) (domain.DecisionGate, []string, error) {
	if f.err != nil {
		return domain.DecisionGate{}, nil, f.err
	}
	g, ok := f.gates[gateID]
	if !ok {
		return domain.DecisionGate{}, nil, ErrGateNotFound
	}
	resolved, err := g.Resolve(resolution)
	if err != nil {
		return domain.DecisionGate{}, nil, ErrGateNotPending
	}
	f.gates[gateID] = resolved
	return resolved, []string{resolved.OrchestrationTaskID}, nil
}

func (f *fakeGateRepository) ListPending(_ context.Context, tenantID string) ([]domain.DecisionGate, error) {
	if f.err != nil {
		return nil, f.err
	}
	var out []domain.DecisionGate
	for _, g := range f.gates {
		if g.TenantID == tenantID && g.Status == domain.GateStatusPending {
			out = append(out, g)
		}
	}
	return out, nil
}

// fakeCoordinatorRunRepository is an in-memory CoordinatorRunRepository.
type fakeCoordinatorRunRepository struct {
	mu   sync.Mutex
	runs map[string]domain.CoordinatorRun

	createErr       error
	createWithTasks func(tenantID string, run domain.CoordinatorRun) (domain.CoordinatorRun, error)

	markReportedCalls []string
	markReportedErr   error
}

func newFakeCoordinatorRunRepository(runs ...domain.CoordinatorRun) *fakeCoordinatorRunRepository {
	m := make(map[string]domain.CoordinatorRun, len(runs))
	for _, r := range runs {
		m[r.ID] = r
	}
	return &fakeCoordinatorRunRepository{runs: m}
}

func (f *fakeCoordinatorRunRepository) CreateWithTasks(_ context.Context, tenantID string, run domain.CoordinatorRun) (domain.CoordinatorRun, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.createErr != nil {
		return domain.CoordinatorRun{}, f.createErr
	}
	if f.createWithTasks != nil {
		created, err := f.createWithTasks(tenantID, run)
		if err != nil {
			return domain.CoordinatorRun{}, err
		}
		if f.runs == nil {
			f.runs = map[string]domain.CoordinatorRun{}
		}
		f.runs[created.ID] = created
		return created, nil
	}
	if run.ID == "" {
		run.ID = "run-" + run.CoordinatorHandle
	}
	run.TenantID = tenantID
	if f.runs == nil {
		f.runs = map[string]domain.CoordinatorRun{}
	}
	f.runs[run.ID] = run
	return run, nil
}

func (f *fakeCoordinatorRunRepository) GetRun(_ context.Context, _, id string) (domain.CoordinatorRun, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.runs[id]
	if !ok {
		return domain.CoordinatorRun{}, ErrRunNotFound
	}
	return r, nil
}

func (f *fakeCoordinatorRunRepository) Complete(_ context.Context, _, id string, result json.RawMessage) (domain.CoordinatorRun, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.runs[id]
	if !ok {
		return domain.CoordinatorRun{}, ErrRunNotFound
	}
	completed, err := r.Complete(result)
	if err != nil {
		return domain.CoordinatorRun{}, ErrRunNotFound
	}
	f.runs[id] = completed
	return completed, nil
}

func (f *fakeCoordinatorRunRepository) Fail(_ context.Context, _, id, errMsg string) (domain.CoordinatorRun, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.runs[id]
	if !ok {
		return domain.CoordinatorRun{}, ErrRunNotFound
	}
	failed, err := r.Fail(errMsg)
	if err != nil {
		return domain.CoordinatorRun{}, ErrRunNotFound
	}
	f.runs[id] = failed
	return failed, nil
}

func (f *fakeCoordinatorRunRepository) ListRunning(_ context.Context) ([]domain.CoordinatorRun, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []domain.CoordinatorRun
	for _, r := range f.runs {
		if r.Status == domain.RunStatusRunning {
			out = append(out, r)
		}
	}
	return out, nil
}

func (f *fakeCoordinatorRunRepository) MarkReported(_ context.Context, _, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.markReportedErr != nil {
		return f.markReportedErr
	}
	f.markReportedCalls = append(f.markReportedCalls, id)
	r, ok := f.runs[id]
	if ok {
		r.ReportedAt = time.Now()
		f.runs[id] = r
	}
	return nil
}

func (f *fakeCoordinatorRunRepository) ListUnreportedTerminal(_ context.Context) ([]domain.CoordinatorRun, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []domain.CoordinatorRun
	for _, r := range f.runs {
		if (r.Status == domain.RunStatusCompleted || r.Status == domain.RunStatusFailed) && r.ReportedAt.IsZero() {
			out = append(out, r)
		}
	}
	return out, nil
}

// fakeTaskServiceReporter is an in-memory TaskServiceReporter — records
// every call so tests can assert ReportResult was called exactly once with
// the expected arguments.
type fakeTaskServiceReporter struct {
	mu    sync.Mutex
	calls []reportResultCall
	err   error
}

type reportResultCall struct {
	taskID, coordinatorRunID string
	success                  bool
	errMsg                   string
}

func (f *fakeTaskServiceReporter) ReportResult(_ context.Context, taskID, coordinatorRunID string, success bool, errMsg string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	f.calls = append(f.calls, reportResultCall{taskID: taskID, coordinatorRunID: coordinatorRunID, success: success, errMsg: errMsg})
	return nil
}

func (f *fakeTaskServiceReporter) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

// fakeWorkerDispatcher is an in-memory WorkerDispatcher — records every
// dispatched task id, optionally failing dispatch for specific task ids.
type fakeWorkerDispatcher struct {
	mu          sync.Mutex
	dispatched  []string
	failTaskIDs map[string]error
	err         error
}

func (f *fakeWorkerDispatcher) Dispatch(_ context.Context, _ string, task domain.OrchestrationTask, _ string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failTaskIDs != nil {
		if err, ok := f.failTaskIDs[task.ID]; ok {
			return err
		}
	}
	if f.err != nil {
		return f.err
	}
	f.dispatched = append(f.dispatched, task.ID)
	return nil
}

func (f *fakeWorkerDispatcher) dispatchedIDs() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, len(f.dispatched))
	copy(out, f.dispatched)
	return out
}
