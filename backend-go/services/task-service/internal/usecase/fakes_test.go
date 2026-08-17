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
	tasks     map[string]domain.Task
	createErr error
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
