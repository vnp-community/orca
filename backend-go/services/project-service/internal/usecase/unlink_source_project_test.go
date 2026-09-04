package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

func TestUnlinkSourceProject_OwnerCanUnlink(t *testing.T) {
	repo := newFakeProjectRepository()
	repo.projects["container"] = domain.Project{ID: "container", TenantID: "tenant-1"}
	repo.members = append(repo.members, domain.ProjectMember{ProjectID: "container", UserID: "owner-1", Role: domain.ProjectRoleOwner})
	sourceProjects := newFakeSourceProjectRepository()
	sourceProjects.links[sourceProjectKey("container", "source")] = domain.SourceProject{ContainerProjectID: "container", SourceProjectID: "source"}
	uc := NewUnlinkSourceProject(sourceProjects, repo, &fakeOPAClient{decide: projectRegoDecide})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "owner-1")
	if err := uc.Execute(ctx, UnlinkSourceProjectInput{ContainerProjectID: "container", SourceProjectID: "source"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sourceProjects.links) != 0 {
		t.Error("expected link removed")
	}
}

func TestUnlinkSourceProject_MemberDenied(t *testing.T) {
	repo := newFakeProjectRepository()
	repo.projects["container"] = domain.Project{ID: "container", TenantID: "tenant-1"}
	repo.members = append(repo.members, domain.ProjectMember{ProjectID: "container", UserID: "member-1", Role: domain.ProjectRoleMember})
	sourceProjects := newFakeSourceProjectRepository()
	sourceProjects.links[sourceProjectKey("container", "source")] = domain.SourceProject{ContainerProjectID: "container", SourceProjectID: "source"}
	uc := NewUnlinkSourceProject(sourceProjects, repo, &fakeOPAClient{decide: projectRegoDecide})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "member-1")
	err := uc.Execute(ctx, UnlinkSourceProjectInput{ContainerProjectID: "container", SourceProjectID: "source"})
	assertAppError(t, err, apperrors.KindPermissionDenied, "PROJECT_NOT_AUTHORIZED")
	if len(sourceProjects.links) != 1 {
		t.Error("expected link NOT removed")
	}
}

// TestUnlinkSourceProject_IdempotentNoOp matches the legacy TS reference's
// exact behavior: unlinking an absent link is a success no-op, not an
// error.
func TestUnlinkSourceProject_IdempotentNoOp(t *testing.T) {
	repo := newFakeProjectRepository()
	repo.projects["container"] = domain.Project{ID: "container", TenantID: "tenant-1"}
	repo.members = append(repo.members, domain.ProjectMember{ProjectID: "container", UserID: "owner-1", Role: domain.ProjectRoleOwner})
	sourceProjects := newFakeSourceProjectRepository()
	uc := NewUnlinkSourceProject(sourceProjects, repo, &fakeOPAClient{decide: projectRegoDecide})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "owner-1")
	if err := uc.Execute(ctx, UnlinkSourceProjectInput{ContainerProjectID: "container", SourceProjectID: "never-linked"}); err != nil {
		t.Fatalf("expected idempotent no-op, got error: %v", err)
	}
}
