package usecase

import (
	"context"

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
	updateStatusCalls []updateStatusCall
}

// updateStatusCall records one UpdateStatus invocation — used by
// execute_task_test.go to assert ExecuteTask marks a task in_progress
// before dispatching, without needing a full call-recorder abstraction.
type updateStatusCall struct {
	tenantID string
	id       string
	status   string
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
func (f *fakeTaskRepository) UpdateStatus(ctx context.Context, tenantID, id, status string) error {
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

var errNotFound = &notFoundError{}

type notFoundError struct{}

func (*notFoundError) Error() string { return "not found" }

type fakeEdgeRepository struct {
	edges   []domain.TaskEdge
	addErr  error
	listErr error
}

func (f *fakeEdgeRepository) Add(ctx context.Context, tenantID string, edge domain.TaskEdge) error {
	if f.addErr != nil {
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
	grants   []domain.Grant
	grantErr error
	listErr  error
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

type fakeExecutor struct {
	ref    string
	err    error
	called bool
}

func (f *fakeExecutor) Execute(ctx context.Context, tenantID, taskID, requestID string) (string, error) {
	f.called = true
	if f.err != nil {
		return "", f.err
	}
	return f.ref, nil
}
