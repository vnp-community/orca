package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

func TestGetRepo_ReturnsRepoAndOwningProjectsDevServerID(t *testing.T) {
	repoRepo := newFakeRepoRepository()
	repoRepo.repos["r1"] = domain.Repo{ID: "r1", ProjectID: "p1", URL: "/srv/repo", DisplayName: "Repo One"}
	projectRepo := newFakeProjectRepository()
	projectRepo.projects["p1"] = domain.Project{ID: "p1", TenantID: "tenant-1", DevServerID: "ds-1"}
	uc := NewGetRepo(repoRepo, projectRepo)

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	result, err := uc.Execute(ctx, GetRepoInput{RepoID: "r1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Repo.ID != "r1" || result.Repo.URL != "/srv/repo" {
		t.Errorf("unexpected repo: %+v", result.Repo)
	}
	if result.DevServerID != "ds-1" {
		t.Errorf("expected dev_server_id ds-1, got %q", result.DevServerID)
	}
}

func TestGetRepo_RepoNotFound(t *testing.T) {
	uc := NewGetRepo(newFakeRepoRepository(), newFakeProjectRepository())

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	_, err := uc.Execute(ctx, GetRepoInput{RepoID: "missing"})
	assertAppError(t, err, apperrors.KindNotFound, "PROJECT_REPO_NOT_FOUND")
}

func TestGetRepo_RequiresRepoID(t *testing.T) {
	uc := NewGetRepo(newFakeRepoRepository(), newFakeProjectRepository())

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	_, err := uc.Execute(ctx, GetRepoInput{RepoID: ""})
	assertAppError(t, err, apperrors.KindInvalidArgument, "PROJECT_REPO_ID_REQUIRED")
}

func TestGetRepo_RequiresTenant(t *testing.T) {
	uc := NewGetRepo(newFakeRepoRepository(), newFakeProjectRepository())

	_, err := uc.Execute(context.Background(), GetRepoInput{RepoID: "r1"})
	assertAppError(t, err, apperrors.KindUnauthenticated, "PROJECT_NO_TENANT")
}

// TestGetRepo_DoesNotRequireUserInContext confirms this RPC's trust
// boundary: git-gateway-service's outbound calls only forward tenant, never
// the acting user's identity (see GetRepo's own doc comment) — so this must
// succeed on tenant alone, unlike requireProjectAccess/requireRepoAccess-
// gated usecases which fail closed without a user.
func TestGetRepo_DoesNotRequireUserInContext(t *testing.T) {
	repoRepo := newFakeRepoRepository()
	repoRepo.repos["r1"] = domain.Repo{ID: "r1", ProjectID: "p1", URL: "/srv/repo"}
	projectRepo := newFakeProjectRepository()
	projectRepo.projects["p1"] = domain.Project{ID: "p1", TenantID: "tenant-1", DevServerID: "ds-1"}
	uc := NewGetRepo(repoRepo, projectRepo)

	ctx := tenant.WithTenantID(context.Background(), "tenant-1")
	if _, err := uc.Execute(ctx, GetRepoInput{RepoID: "r1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
