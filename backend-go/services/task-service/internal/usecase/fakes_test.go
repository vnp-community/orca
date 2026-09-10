package usecase

import (
	"context"
	"sort"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

// The fakes in this file back every usecase test in this package — the
// "test against fakes, not a real database" pattern from
// specs/backend-go/standards/testing-strategy.md's unit-test section
// (mirrors usage-service/internal/usecase/record_usage_session_test.go).

func withIdentity(ctx context.Context, tenantID, userID string) context.Context {
	ctx = tenant.WithTenantID(ctx, tenantID)
	return tenant.WithUserID(ctx, userID)
}

type fakeTaskRepository struct {
	tasks             map[string]domain.Task
	createErr         error
	updateStatusErr   error
	hasActiveErr      error
	listErr           error
	updateErr         error
	deleteErr         error
	updateStatusCalls []updateStatusCall
}

// updateStatusCall records one UpdateStatus invocation — used by
// execute_task_test.go to assert ExecuteTask marks a task in_progress
// before dispatching, without needing a full call-recorder abstraction.
type updateStatusCall struct {
	tenantID string
	id       string
	status   domain.Status
}

func newFakeTaskRepository() *fakeTaskRepository {
	return &fakeTaskRepository{tasks: map[string]domain.Task{}}
}

func (f *fakeTaskRepository) Create(ctx context.Context, task domain.Task) (domain.Task, error) {
	if f.createErr != nil {
		return domain.Task{}, f.createErr
	}
	f.tasks[task.ID] = task
	return task, nil
}

func (f *fakeTaskRepository) Get(ctx context.Context, tenantID, id string) (domain.Task, error) {
	t, ok := f.tasks[id]
	if !ok || t.TenantID != tenantID {
		return domain.Task{}, errNotFound
	}
	return t, nil
}

func (f *fakeTaskRepository) GetAncestors(ctx context.Context, tenantID, id string, maxDepth int) ([]domain.Task, error) {
	var chain []domain.Task
	current, ok := f.tasks[id]
	if !ok || current.TenantID != tenantID {
		return nil, errNotFound
	}
	for i := 0; ; i++ {
		if maxDepth > 0 && i >= maxDepth {
			break
		}
		chain = append(chain, current)
		if current.ParentID == "" {
			break
		}
		parent, ok := f.tasks[current.ParentID]
		if !ok {
			break
		}
		current = parent
	}
	return chain, nil
}

// UpdateStatus mirrors adapter/postgres's real semantics (mutates the
// stored task's status) but, unlike the real repository, does not error
// when the task is missing from the fake's map — several existing tests in
// this package exercise ExecuteTask without first seeding a task via
// Create, and this fake is a permissive test double, not a fidelity
// replica of Postgres's not-found behavior.
func (f *fakeTaskRepository) UpdateStatus(ctx context.Context, tenantID, id string, status domain.Status) error {
	f.updateStatusCalls = append(f.updateStatusCalls, updateStatusCall{tenantID: tenantID, id: id, status: status})
	if f.updateStatusErr != nil {
		return f.updateStatusErr
	}
	if t, ok := f.tasks[id]; ok && t.TenantID == tenantID {
		t.Status = status
		f.tasks[id] = t
	}
	return nil
}

// RecalculateAncestorProgress recomputes done_subtasks/total_subtasks for
// every task in the fake's map whose id is an ancestor of taskID, walking
// ParentID the same way GetAncestors does — a plain in-memory equivalent of
// the real WITH RECURSIVE UPDATE, close enough to exercise
// RecalculateProgress's usecase-layer wiring without a database.
func (f *fakeTaskRepository) RecalculateAncestorProgress(ctx context.Context, tenantID, taskID string) error {
	current, ok := f.tasks[taskID]
	if !ok || current.TenantID != tenantID {
		return nil
	}
	id := current.ParentID
	for id != "" {
		parent, ok := f.tasks[id]
		if !ok {
			break
		}
		done, total := 0, 0
		for _, t := range f.tasks {
			if t.ParentID == id {
				total++
				if t.Status == domain.StatusDone {
					done++
				}
			}
		}
		parent.DoneSubtasks, parent.TotalSubtasks = done, total
		f.tasks[id] = parent
		id = parent.ParentID
	}
	return nil
}

// GetSubtree walks the fake's tasks map by ParentID (the in-memory inverse
// of RecalculateAncestorProgress's ancestor walk above) — close enough to
// exercise GetSubtree's usecase-layer wiring without a database.
func (f *fakeTaskRepository) GetSubtree(ctx context.Context, tenantID, id string) ([]domain.Task, error) {
	root, ok := f.tasks[id]
	if !ok || root.TenantID != tenantID {
		return nil, errNotFound
	}
	out := []domain.Task{root}
	queue := []string{id}
	for len(queue) > 0 {
		parentID := queue[0]
		queue = queue[1:]
		for _, t := range f.tasks {
			if t.ParentID == parentID && t.TenantID == tenantID {
				out = append(out, t)
				queue = append(queue, t.ID)
			}
		}
	}
	return out, nil
}

// ListChildren scans the fake's tasks map for direct children of taskID —
// real enough to exercise AIDecompose's context-bundle wiring without a
// database.
func (f *fakeTaskRepository) ListChildren(ctx context.Context, tenantID, taskID string) ([]domain.Task, error) {
	var out []domain.Task
	for _, t := range f.tasks {
		if t.TenantID == tenantID && t.ParentID == taskID {
			out = append(out, t)
		}
	}
	return out, nil
}

// GetByShareToken scans the fake's tasks map for a matching ShareToken —
// deliberately NOT tenant-filtered, matching the real repository's
// unauthenticated-lookup contract (TASK-TG-003-05).
func (f *fakeTaskRepository) GetByShareToken(ctx context.Context, token string) (domain.Task, error) {
	for _, t := range f.tasks {
		if t.ShareToken != nil && *t.ShareToken == token {
			return t, nil
		}
	}
	return domain.Task{}, errNotFound
}

// HasActiveExecutions scans the fake's tasks map — real enough to exercise
// usecase.HasActiveExecutions's tenant/project filtering without a database.
func (f *fakeTaskRepository) HasActiveExecutions(ctx context.Context, tenantID, projectID string) (bool, error) {
	if f.hasActiveErr != nil {
		return false, f.hasActiveErr
	}
	for _, t := range f.tasks {
		if t.TenantID == tenantID && t.ProjectID == projectID && t.Status == domain.StatusInProgress {
			return true, nil
		}
	}
	return false, nil
}

// List returns tasks for tenantID (optionally filtered by projectID),
// sorted by ID for deterministic test assertions — real enough to exercise
// ListTasks's filtering without a database. Pagination (pageToken/pageSize)
// is intentionally not simulated here (no test in this package needs it
// yet); every match is returned with an empty next-page token.
func (f *fakeTaskRepository) List(ctx context.Context, tenantID, projectID, pageToken string, pageSize int32) ([]domain.Task, string, error) {
	if f.listErr != nil {
		return nil, "", f.listErr
	}
	ids := make([]string, 0, len(f.tasks))
	for id := range f.tasks {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	var out []domain.Task
	for _, id := range ids {
		t := f.tasks[id]
		if t.TenantID != tenantID {
			continue
		}
		if projectID != "" && t.ProjectID != projectID {
			continue
		}
		out = append(out, t)
	}
	return out, "", nil
}

func (f *fakeTaskRepository) Update(ctx context.Context, tenantID string, task domain.Task) error {
	if f.updateErr != nil {
		return f.updateErr
	}
	existing, ok := f.tasks[task.ID]
	if !ok || existing.TenantID != tenantID {
		return errNotFound
	}
	f.tasks[task.ID] = task
	return nil
}

func (f *fakeTaskRepository) Delete(ctx context.Context, tenantID, id string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	existing, ok := f.tasks[id]
	if !ok || existing.TenantID != tenantID {
		return errNotFound
	}
	delete(f.tasks, id)
	return nil
}

var errNotFound = &notFoundError{}

type notFoundError struct{}

func (*notFoundError) Error() string { return "not found" }

type fakeEdgeRepository struct {
	edges   []domain.TaskEdge
	addErr  error
	listErr error
	// addErrAfterCalls, when > 0, lets the first N Add calls succeed before
	// addErr starts firing — used by ai_apply_test.go's mid-loop-failure
	// case to simulate AIApply's documented non-transactional gap (one
	// proposal committed, a later one failing) without a real database.
	addErrAfterCalls int
	addCalls         int
}

func (f *fakeEdgeRepository) Add(ctx context.Context, tenantID string, edge domain.TaskEdge) error {
	f.addCalls++
	if f.addErr != nil && f.addCalls > f.addErrAfterCalls {
		return f.addErr
	}
	f.edges = append(f.edges, edge)
	return nil
}

func (f *fakeEdgeRepository) ListByKind(ctx context.Context, tenantID string, kind domain.EdgeKind) ([]domain.TaskEdge, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	var out []domain.TaskEdge
	for _, e := range f.edges {
		if e.Kind == kind {
			out = append(out, e)
		}
	}
	return out, nil
}

func (f *fakeEdgeRepository) ListFrom(ctx context.Context, tenantID, fromTaskID string, kind domain.EdgeKind) ([]domain.TaskEdge, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	var out []domain.TaskEdge
	for _, e := range f.edges {
		if e.Kind == kind && e.FromTaskID == fromTaskID {
			out = append(out, e)
		}
	}
	return out, nil
}

type fakeGrantRepository struct {
	grants    []domain.Grant
	grantErr  error
	listErr   error
	revokeErr error
}

func (f *fakeGrantRepository) Grant(ctx context.Context, tenantID string, grant domain.Grant) error {
	if f.grantErr != nil {
		return f.grantErr
	}
	f.grants = append(f.grants, grant)
	return nil
}

func (f *fakeGrantRepository) ListGrantsForAncestors(ctx context.Context, tenantID string, taskIDs []string) (map[string][]domain.Grant, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	ids := map[string]bool{}
	for _, id := range taskIDs {
		ids[id] = true
	}
	out := map[string][]domain.Grant{}
	for _, g := range f.grants {
		if ids[g.TaskID] {
			out[g.TaskID] = append(out[g.TaskID], g)
		}
	}
	return out, nil
}

// Revoke removes the first grant matching (taskID, subjectID, level) —
// idempotent, mirrors the real postgres.Repository.Revoke's
// DELETE-affecting-0-rows-is-fine semantics.
func (f *fakeGrantRepository) Revoke(ctx context.Context, tenantID, taskID, subjectID string, level domain.GrantLevel) error {
	if f.revokeErr != nil {
		return f.revokeErr
	}
	for i, g := range f.grants {
		if g.TaskID == taskID && g.SubjectID == subjectID && g.Level == level {
			f.grants = append(f.grants[:i], f.grants[i+1:]...)
			break
		}
	}
	return nil
}

// ListByTask returns every grant recorded directly against taskID — no
// tenant filtering in this fake (the real repository filters by tenant_id;
// this in-memory fake's tests don't currently need multi-tenant fixtures
// for grants, matching ListGrantsForAncestors's own fake above).
func (f *fakeGrantRepository) ListByTask(ctx context.Context, tenantID, taskID string) ([]domain.Grant, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	var out []domain.Grant
	for _, g := range f.grants {
		if g.TaskID == taskID {
			out = append(out, g)
		}
	}
	return out, nil
}

type fakeTeamScopeResolver struct {
	teams []string
	err   error
}

func (f *fakeTeamScopeResolver) ResolveTeams(ctx context.Context, tenantID, userID string) ([]string, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.teams, nil
}

// fakeOPAClient backs ResolvePermission's tests without loading the real
// Rego bundle — allow/decisionErr let a test force either branch of
// Execute's fail-closed check; called records whether Decision was ever
// invoked, so a not-found test can assert OPA was never even reached.
type fakeOPAClient struct {
	allow       bool
	decisionErr error
	called      bool
}

func (f *fakeOPAClient) Decision(ctx context.Context, level domain.GrantLevel, action, tenantID string) (bool, error) {
	f.called = true
	if f.decisionErr != nil {
		return false, f.decisionErr
	}
	return f.allow, nil
}

// fakeTxRunner is an in-memory TxRunner mirroring
// credential-broker-service/internal/usecase/fakes_test.go's fakeTxRunner
// exactly: it runs fn against the same fakeTaskRepository/fakeEdgeRepository
// instances the test already holds a handle to, but snapshots their state
// first and restores it if fn returns an error. This models real Postgres
// ROLLBACK semantics (internal/adapter/postgres.Repository.RunInTx, via
// pgx.BeginFunc) closely enough to prove atomicity in tests without a real
// database — see ai_apply_test.go's rollback assertion, the reason this
// fake exists rather than a bare pass-through.
type fakeTxRunner struct {
	tasks *fakeTaskRepository
	edges *fakeEdgeRepository
}

func newFakeTxRunner(tasks *fakeTaskRepository, edges *fakeEdgeRepository) *fakeTxRunner {
	return &fakeTxRunner{tasks: tasks, edges: edges}
}

func (f *fakeTxRunner) RunInTx(ctx context.Context, fn func(ctx context.Context, tasks TaskRepository, edges EdgeRepository) error) error {
	savedTasks := make(map[string]domain.Task, len(f.tasks.tasks))
	for k, v := range f.tasks.tasks {
		savedTasks[k] = v
	}
	savedEdges := append([]domain.TaskEdge(nil), f.edges.edges...)
	savedAddCalls := f.edges.addCalls

	if err := fn(ctx, f.tasks, f.edges); err != nil {
		// Rollback: undo whatever fn did to either fake before returning its
		// error, exactly like a real ROLLBACK undoes every statement in the
		// transaction.
		f.tasks.tasks = savedTasks
		f.edges.edges = savedEdges
		f.edges.addCalls = savedAddCalls
		return err
	}
	return nil
}

type fakeExecutor struct {
	ref       string
	err       error
	called    bool
	gotPrompt string
}

func (f *fakeExecutor) Execute(ctx context.Context, tenantID, taskID, requestID, prompt string) (string, error) {
	f.called = true
	f.gotPrompt = prompt
	if f.err != nil {
		return "", f.err
	}
	return f.ref, nil
}
