package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

// newTestCreateProject builds a CreateProject with fresh fakes for its four
// new (TASK-PRF-03-04) dependencies — used by tests that don't care about
// dev-server binding at all (DevServerID left empty).
func newTestCreateProject(repo *fakeProjectRepository) (*CreateProject, *fakeRepoRepository, *fakeDevServerLister, *fakeDevServerHealthChecker, *fakeDevServerRelay) {
	repos := newFakeRepoRepository()
	devServers := &fakeDevServerLister{}
	health := &fakeDevServerHealthChecker{}
	relay := &fakeDevServerRelay{}
	return NewCreateProject(repo, repos, devServers, health, relay), repos, devServers, health, relay
}

func TestCreateProject_AppliesDefaultsAndCreatedBy(t *testing.T) {
	repo := newFakeProjectRepository()
	uc, _, _, _, _ := newTestCreateProject(repo)

	ctx := withTenantAndUser(context.Background(), "tenant-1", "user-1")
	got, err := uc.Execute(ctx, CreateProjectInput{Name: "my-project"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.DefaultBranch != domain.DefaultBranch {
		t.Errorf("expected DefaultBranch=%q, got %q", domain.DefaultBranch, got.DefaultBranch)
	}
	if got.Visibility != domain.DefaultVisibility {
		t.Errorf("expected Visibility=%q, got %q", domain.DefaultVisibility, got.Visibility)
	}
	if got.CreatedBy != "user-1" {
		t.Errorf("expected CreatedBy=user-1, got %q", got.CreatedBy)
	}
}

func TestCreateProject_UsesRequestFieldsWhenProvided(t *testing.T) {
	repo := newFakeProjectRepository()
	uc, _, _, _, _ := newTestCreateProject(repo)

	ctx := withTenantAndUser(context.Background(), "tenant-1", "user-1")
	got, err := uc.Execute(ctx, CreateProjectInput{
		Name:          "my-project",
		Description:   "a description",
		DefaultBranch: "develop",
		Visibility:    domain.VisibilityTeam,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Description != "a description" {
		t.Errorf("expected Description to round-trip, got %q", got.Description)
	}
	if got.DefaultBranch != "develop" {
		t.Errorf("expected DefaultBranch=develop, got %q", got.DefaultBranch)
	}
	if got.Visibility != domain.VisibilityTeam {
		t.Errorf("expected Visibility=team, got %q", got.Visibility)
	}
}

func TestCreateProject_RejectsInvalidVisibility(t *testing.T) {
	repo := newFakeProjectRepository()
	uc, _, _, _, _ := newTestCreateProject(repo)

	ctx := withTenantAndUser(context.Background(), "tenant-1", "user-1")
	_, err := uc.Execute(ctx, CreateProjectInput{Name: "my-project", Visibility: "bogus"})
	assertAppError(t, err, apperrors.KindInvalidArgument, "PROJECT_INVALID_VISIBILITY")
}

func TestCreateProject_RequiresTenantContext(t *testing.T) {
	repo := newFakeProjectRepository()
	uc, _, _, _, _ := newTestCreateProject(repo)

	_, err := uc.Execute(context.Background(), CreateProjectInput{Name: "my-project"})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestCreateProject_RequiresUserContext(t *testing.T) {
	repo := newFakeProjectRepository()
	uc, _, _, _, _ := newTestCreateProject(repo)

	ctx := withTenant(context.Background(), "tenant-1")
	_, err := uc.Execute(ctx, CreateProjectInput{Name: "my-project"})
	assertAppError(t, err, apperrors.KindUnauthenticated, "PROJECT_NO_USER")
}

func TestCreateProject_NoDevServerID_UnchangedExistingBehavior(t *testing.T) {
	repo := newFakeProjectRepository()
	uc, _, devServers, health, relay := newTestCreateProject(repo)

	ctx := withTenantAndUser(context.Background(), "tenant-1", "user-1")
	got, err := uc.Execute(ctx, CreateProjectInput{Name: "my-project"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.DevServerID != "" {
		t.Errorf("expected unbound project, got DevServerID=%q", got.DevServerID)
	}
	if devServers.err != nil || health.err != nil {
		t.Fatal("sanity: fakes should not have errors configured")
	}
	if len(health.calls) != 0 {
		t.Errorf("expected no health check calls when DevServerID is empty, got %v", health.calls)
	}
	if len(relay.createConnectionCalls) != 0 {
		t.Errorf("expected no relay calls when DevServerID is empty, got %+v", relay.createConnectionCalls)
	}
}

func TestCreateProject_DevServerIDSet_RepoPathEmpty_InvalidArgument(t *testing.T) {
	repo := newFakeProjectRepository()
	uc, _, devServers, health, relay := newTestCreateProject(repo)
	devServers.exists = true
	health.reachable = true

	ctx := withTenantAndUser(context.Background(), "tenant-1", "user-1")
	_, err := uc.Execute(ctx, CreateProjectInput{Name: "my-project", DevServerID: "ds-1"})
	assertAppError(t, err, apperrors.KindInvalidArgument, "PROJECT_REPO_PATH_REQUIRED")
	if len(relay.createConnectionCalls) != 0 {
		t.Errorf("expected no relay/repo call before repo_path check, got %+v", relay.createConnectionCalls)
	}
}

func TestCreateProject_DevServerDoesNotExist_InvalidArgument(t *testing.T) {
	repo := newFakeProjectRepository()
	uc, _, devServers, health, relay := newTestCreateProject(repo)
	devServers.exists = false

	ctx := withTenantAndUser(context.Background(), "tenant-1", "user-1")
	_, err := uc.Execute(ctx, CreateProjectInput{Name: "my-project", DevServerID: "ds-1", RepoPath: "/repo"})
	assertAppError(t, err, apperrors.KindInvalidArgument, "PROJECT_DEV_SERVER_NOT_FOUND")
	if len(health.calls) != 0 {
		t.Errorf("expected no health check when dev server doesn't exist, got %v", health.calls)
	}
	if len(relay.createConnectionCalls) != 0 {
		t.Errorf("expected no relay call when dev server doesn't exist, got %+v", relay.createConnectionCalls)
	}
}

func TestCreateProject_DevServerUnreachable_FailedPrecondition(t *testing.T) {
	repo := newFakeProjectRepository()
	uc, _, devServers, health, relay := newTestCreateProject(repo)
	devServers.exists = true
	health.reachable = false

	ctx := withTenantAndUser(context.Background(), "tenant-1", "user-1")
	_, err := uc.Execute(ctx, CreateProjectInput{Name: "my-project", DevServerID: "ds-1", RepoPath: "/repo"})
	assertAppError(t, err, apperrors.KindFailedPrecondition, "PROJECT_DEV_SERVER_UNREACHABLE")
	if len(relay.createConnectionCalls) != 0 {
		t.Errorf("expected no relay call when dev server is unreachable, got %+v", relay.createConnectionCalls)
	}
}

func TestCreateProject_RepoPathNotFoundOnDevServer_NoOrphanedProject(t *testing.T) {
	repo := newFakeProjectRepository()
	uc, _, devServers, health, relay := newTestCreateProject(repo)
	devServers.exists = true
	health.reachable = true
	relay.relayResult = []byte(`{"exists":false,"isDir":false}`)

	ctx := withTenantAndUser(context.Background(), "tenant-1", "user-1")
	_, err := uc.Execute(ctx, CreateProjectInput{Name: "my-project", DevServerID: "ds-1", RepoPath: "/missing"})
	assertAppError(t, err, apperrors.KindFailedPrecondition, "PROJECT_REPO_PATH_NOT_FOUND")

	if len(repo.projects) != 0 {
		t.Errorf("expected no project row persisted when repo_path validation fails, got %d", len(repo.projects))
	}
}

func TestCreateProject_DevServerBinding_Success_AttachesRepo(t *testing.T) {
	repo := newFakeProjectRepository()
	uc, repos, devServers, health, relay := newTestCreateProject(repo)
	devServers.exists = true
	health.reachable = true
	relay.relayResult = []byte(`{"exists":true,"isDir":true}`)

	ctx := withTenantAndUser(context.Background(), "tenant-1", "user-1")
	got, err := uc.Execute(ctx, CreateProjectInput{Name: "my-project", DevServerID: "ds-1", RepoPath: "/srv/repo"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.DevServerID != "ds-1" {
		t.Errorf("expected DevServerID=ds-1, got %q", got.DevServerID)
	}
	found := false
	for _, r := range repos.repos {
		if r.ProjectID == got.ID && r.URL == "/srv/repo" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a repo attached with the project's repo_path, got %+v", repos.repos)
	}
}
