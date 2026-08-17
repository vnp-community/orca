package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

// fakeProjectRepository is an in-memory ProjectRepository — the "test
// against fakes, not a real database" pattern from
// specs/backend-go/standards/testing-strategy.md's unit-test section.
type fakeProjectRepository struct {
	projects map[string]domain.Project
	members  []domain.ProjectMember

	createErr        error
	updateDevServErr error
}

func newFakeProjectRepository() *fakeProjectRepository {
	return &fakeProjectRepository{projects: map[string]domain.Project{}}
}

func (f *fakeProjectRepository) Create(ctx context.Context, p domain.Project) (domain.Project, error) {
	if f.createErr != nil {
		return domain.Project{}, f.createErr
	}
	f.projects[p.ID] = p
	return p, nil
}

func (f *fakeProjectRepository) Get(ctx context.Context, tenantID, id string) (domain.Project, error) {
	p, ok := f.projects[id]
	if !ok || p.TenantID != tenantID {
		return domain.Project{}, domain.ErrProjectNotFound
	}
	return p, nil
}

func (f *fakeProjectRepository) List(ctx context.Context, tenantID, pageToken string, pageSize int32) ([]domain.Project, string, error) {
	var out []domain.Project
	for _, p := range f.projects {
		if p.TenantID == tenantID {
			out = append(out, p)
		}
	}
	return out, "", nil
}

func (f *fakeProjectRepository) AddMember(ctx context.Context, m domain.ProjectMember) error {
	f.members = append(f.members, m)
	return nil
}

func (f *fakeProjectRepository) UpdateDevServerID(ctx context.Context, tenantID, projectID, devServerID string) (domain.Project, error) {
	if f.updateDevServErr != nil {
		return domain.Project{}, f.updateDevServErr
	}
	p, ok := f.projects[projectID]
	if !ok || p.TenantID != tenantID {
		return domain.Project{}, domain.ErrProjectNotFound
	}
	p.DevServerID = devServerID
	f.projects[projectID] = p
	return p, nil
}

// fakeExecutionChecker implements both WorkflowExecutionChecker and
// TaskExecutionChecker — the two ports have identical shape but are kept
// distinct types in usecase/ports.go so a fake for one doesn't couple to the
// other service's contract.
type fakeExecutionChecker struct {
	active bool
	err    error
}

func (f *fakeExecutionChecker) HasActiveExecutions(ctx context.Context, projectID string) (bool, error) {
	return f.active, f.err
}

func withTenant(ctx context.Context, tenantID string) context.Context {
	return tenant.WithTenantID(ctx, tenantID)
}

func TestRebindDevServer_AllowedWhenNoActiveExecutions(t *testing.T) {
	repo := newFakeProjectRepository()
	repo.projects["p1"] = domain.Project{ID: "p1", TenantID: "tenant-1", Name: "proj", DevServerID: "dev-1"}

	uc := NewRebindDevServer(repo, &fakeExecutionChecker{active: false}, &fakeExecutionChecker{active: false})

	ctx := withTenant(context.Background(), "tenant-1")
	got, err := uc.Execute(ctx, RebindDevServerInput{ProjectID: "p1", NewDevServerID: "dev-2"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.DevServerID != "dev-2" {
		t.Errorf("expected DevServerID=dev-2, got %q", got.DevServerID)
	}
}

func TestRebindDevServer_RejectedWhenWorkflowHasActiveExecutions(t *testing.T) {
	repo := newFakeProjectRepository()
	repo.projects["p1"] = domain.Project{ID: "p1", TenantID: "tenant-1", Name: "proj", DevServerID: "dev-1"}

	uc := NewRebindDevServer(repo, &fakeExecutionChecker{active: true}, &fakeExecutionChecker{active: false})

	ctx := withTenant(context.Background(), "tenant-1")
	_, err := uc.Execute(ctx, RebindDevServerInput{ProjectID: "p1", NewDevServerID: "dev-2"})
	assertFailedPrecondition(t, err)

	if repo.projects["p1"].DevServerID != "dev-1" {
		t.Errorf("expected dev_server_id to remain unchanged, got %q", repo.projects["p1"].DevServerID)
	}
}

func TestRebindDevServer_RejectedWhenTaskHasActiveExecutions(t *testing.T) {
	repo := newFakeProjectRepository()
	repo.projects["p1"] = domain.Project{ID: "p1", TenantID: "tenant-1", Name: "proj", DevServerID: "dev-1"}

	uc := NewRebindDevServer(repo, &fakeExecutionChecker{active: false}, &fakeExecutionChecker{active: true})

	ctx := withTenant(context.Background(), "tenant-1")
	_, err := uc.Execute(ctx, RebindDevServerInput{ProjectID: "p1", NewDevServerID: "dev-2"})
	assertFailedPrecondition(t, err)

	if repo.projects["p1"].DevServerID != "dev-1" {
		t.Errorf("expected dev_server_id to remain unchanged, got %q", repo.projects["p1"].DevServerID)
	}
}

func TestRebindDevServer_FailsClosedOnCheckerError(t *testing.T) {
	repo := newFakeProjectRepository()
	repo.projects["p1"] = domain.Project{ID: "p1", TenantID: "tenant-1", Name: "proj", DevServerID: "dev-1"}

	uc := NewRebindDevServer(repo, &fakeExecutionChecker{err: errors.New("workflow-service unreachable")}, &fakeExecutionChecker{active: false})

	ctx := withTenant(context.Background(), "tenant-1")
	_, err := uc.Execute(ctx, RebindDevServerInput{ProjectID: "p1", NewDevServerID: "dev-2"})
	assertFailedPrecondition(t, err)
}

func TestRebindDevServer_RequiresTenantContext(t *testing.T) {
	repo := newFakeProjectRepository()
	uc := NewRebindDevServer(repo, &fakeExecutionChecker{}, &fakeExecutionChecker{})

	_, err := uc.Execute(context.Background(), RebindDevServerInput{ProjectID: "p1", NewDevServerID: "dev-2"})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestRebindDevServer_RejectsEmptyNewDevServerID(t *testing.T) {
	repo := newFakeProjectRepository()
	uc := NewRebindDevServer(repo, &fakeExecutionChecker{}, &fakeExecutionChecker{})

	ctx := withTenant(context.Background(), "tenant-1")
	_, err := uc.Execute(ctx, RebindDevServerInput{ProjectID: "p1", NewDevServerID: ""})
	if err == nil {
		t.Fatal("expected an error for empty new_dev_server_id")
	}
}

func assertFailedPrecondition(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected an error")
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) {
		t.Fatalf("expected an *apperrors.AppError, got %T: %v", err, err)
	}
	if ae.Kind != apperrors.KindFailedPrecondition {
		t.Errorf("expected KindFailedPrecondition, got %v", ae.Kind)
	}
	if ae.Code != "PROJECT_HAS_ACTIVE_WORKFLOWS" {
		t.Errorf("expected code PROJECT_HAS_ACTIVE_WORKFLOWS, got %q", ae.Code)
	}
}
