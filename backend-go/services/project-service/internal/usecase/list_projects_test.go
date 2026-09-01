package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

func TestListProjects_EmptyPageToken_Succeeds(t *testing.T) {
	repo := newFakeProjectRepository()
	uc := NewListProjects(repo)

	ctx := withTenantAndUser(context.Background(), "tenant-1", "user-1")
	_, err := uc.Execute(ctx, ListProjectsInput{})
	if err != nil {
		t.Fatalf("unexpected error with empty PageToken: %v", err)
	}
}

func TestListProjects_MalformedPageToken_ReturnsInvalidArgument(t *testing.T) {
	repo := newFakeProjectRepository()
	uc := NewListProjects(repo)

	ctx := withTenantAndUser(context.Background(), "tenant-1", "user-1")
	_, err := uc.Execute(ctx, ListProjectsInput{PageToken: "not-a-uuid"})
	assertAppError(t, err, apperrors.KindInvalidArgument, "PROJECT_INVALID_PAGE_TOKEN")
}

func TestListProjects_ValidUUIDPageToken_ReachesRepository(t *testing.T) {
	repo := newFakeProjectRepository()
	uc := NewListProjects(repo)

	ctx := withTenantAndUser(context.Background(), "tenant-1", "user-1")
	// A syntactically valid but nonexistent cursor should reach the
	// repository (fake or real) rather than being rejected by validation —
	// only non-UUID-shaped tokens should be.
	_, err := uc.Execute(ctx, ListProjectsInput{PageToken: "00000000-0000-0000-0000-000000000000"})
	if err != nil {
		t.Fatalf("unexpected error with a well-formed (if nonexistent) cursor: %v", err)
	}
}

// TestListProjects_ScopesToCallersMembership_NotWholeTenant is the direct
// regression test for the "one private default project per user" pass
// (Phase 4b): List used to filter by tenant_id only, so every tenant
// member's project.list call returned every OTHER member's projects too —
// found live while building the default-private-project flow. A project
// the caller has no project_members row for must never come back, even
// though it is in the same tenant.
func TestListProjects_ScopesToCallersMembership_NotWholeTenant(t *testing.T) {
	repo := newFakeProjectRepository()
	repo.projects["mine"] = domain.Project{ID: "mine", TenantID: "tenant-1", Name: "Mine"}
	repo.projects["theirs"] = domain.Project{ID: "theirs", TenantID: "tenant-1", Name: "Theirs"}
	repo.members = []domain.ProjectMember{
		{ProjectID: "mine", UserID: "user-1", Role: domain.ProjectRoleOwner},
		{ProjectID: "theirs", UserID: "user-2", Role: domain.ProjectRoleOwner},
	}
	uc := NewListProjects(repo)

	ctx := withTenantAndUser(context.Background(), "tenant-1", "user-1")
	out, err := uc.Execute(ctx, ListProjectsInput{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.Projects) != 1 || out.Projects[0].ID != "mine" {
		t.Fatalf("want only [mine], got %+v", out.Projects)
	}
}

// TestListProjects_NoUserInContext_ReturnsUnauthenticated guards the new
// tenant.UserID(ctx) requirement this pass added.
func TestListProjects_NoUserInContext_ReturnsUnauthenticated(t *testing.T) {
	repo := newFakeProjectRepository()
	uc := NewListProjects(repo)

	ctx := tenant.WithTenantID(context.Background(), "tenant-1")
	_, err := uc.Execute(ctx, ListProjectsInput{})
	assertAppError(t, err, apperrors.KindUnauthenticated, "PROJECT_NO_USER")
}
