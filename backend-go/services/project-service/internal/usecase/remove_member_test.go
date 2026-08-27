package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

func TestRemoveMember_RejectsWhenWouldBeOwnerless(t *testing.T) {
	repo := newFakeProjectRepository()
	repo.projects["p1"] = domain.Project{ID: "p1", TenantID: "tenant-1", Name: "proj", CreatedBy: "owner-1"}
	repo.members = append(repo.members, domain.ProjectMember{ProjectID: "p1", UserID: "owner-1", Role: domain.ProjectRoleOwner})
	repo.countOwners = 1
	uc := NewRemoveMember(repo, &fakeOPAClient{decide: projectRegoDecide})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "owner-1")
	err := uc.Execute(ctx, RemoveMemberInput{ProjectID: "p1", UserID: "owner-1"})
	assertAppError(t, err, apperrors.KindFailedPrecondition, "PROJECT_WOULD_BE_OWNERLESS")
	if repo.removeMemberCalled {
		t.Error("expected RemoveMember to never be called when the guard fires")
	}
}

func TestRemoveMember_AllowsWhenNotLastOwner(t *testing.T) {
	repo := newFakeProjectRepository()
	repo.projects["p1"] = domain.Project{ID: "p1", TenantID: "tenant-1", Name: "proj", CreatedBy: "owner-1"}
	repo.members = append(repo.members,
		domain.ProjectMember{ProjectID: "p1", UserID: "owner-1", Role: domain.ProjectRoleOwner},
		domain.ProjectMember{ProjectID: "p1", UserID: "owner-2", Role: domain.ProjectRoleOwner},
	)
	repo.countOwners = 2
	uc := NewRemoveMember(repo, &fakeOPAClient{decide: projectRegoDecide})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "owner-1")
	if err := uc.Execute(ctx, RemoveMemberInput{ProjectID: "p1", UserID: "owner-2"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !repo.removeMemberCalled {
		t.Error("expected RemoveMember to be called")
	}
}

func TestRemoveMember_DeniesNonOwnerActor(t *testing.T) {
	repo := newFakeProjectRepository()
	repo.projects["p1"] = domain.Project{ID: "p1", TenantID: "tenant-1", Name: "proj", CreatedBy: "owner-1"}
	repo.members = append(repo.members, domain.ProjectMember{ProjectID: "p1", UserID: "member-1", Role: domain.ProjectRoleMember})
	uc := NewRemoveMember(repo, &fakeOPAClient{decide: projectRegoDecide})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "member-1")
	err := uc.Execute(ctx, RemoveMemberInput{ProjectID: "p1", UserID: "member-1"})
	assertAppError(t, err, apperrors.KindPermissionDenied, "PROJECT_NOT_AUTHORIZED")
	if repo.removeMemberCalled {
		t.Error("expected RemoveMember to never be called for a denied caller")
	}
}

func TestRemoveMember_MembershipNotFound(t *testing.T) {
	repo := newFakeProjectRepository()
	repo.projects["p1"] = domain.Project{ID: "p1", TenantID: "tenant-1", Name: "proj", CreatedBy: "owner-1"}
	repo.members = append(repo.members, domain.ProjectMember{ProjectID: "p1", UserID: "owner-1", Role: domain.ProjectRoleOwner})
	uc := NewRemoveMember(repo, &fakeOPAClient{decide: projectRegoDecide})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "owner-1")
	err := uc.Execute(ctx, RemoveMemberInput{ProjectID: "p1", UserID: "ghost-1"})
	assertAppError(t, err, apperrors.KindNotFound, "PROJECT_MEMBERSHIP_NOT_FOUND")
}
