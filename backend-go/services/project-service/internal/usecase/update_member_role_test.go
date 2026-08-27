package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

func TestUpdateMemberRole_RejectsWhenWouldBeOwnerless(t *testing.T) {
	repo := newFakeProjectRepository()
	repo.projects["p1"] = domain.Project{ID: "p1", TenantID: "tenant-1", Name: "proj", CreatedBy: "owner-1"}
	repo.members = append(repo.members, domain.ProjectMember{ProjectID: "p1", UserID: "owner-1", Role: domain.ProjectRoleOwner})
	repo.countOwners = 1
	uc := NewUpdateMemberRole(repo, &fakeOPAClient{decide: projectRegoDecide})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "owner-1")
	_, err := uc.Execute(ctx, UpdateMemberRoleInput{ProjectID: "p1", UserID: "owner-1", Role: domain.ProjectRoleMember})
	assertAppError(t, err, apperrors.KindFailedPrecondition, "PROJECT_WOULD_BE_OWNERLESS")
	if repo.updateMemberRoleCalled {
		t.Error("expected UpdateMemberRole to never be called when the guard fires")
	}
}

func TestUpdateMemberRole_AllowsWhenNotLastOwner(t *testing.T) {
	repo := newFakeProjectRepository()
	repo.projects["p1"] = domain.Project{ID: "p1", TenantID: "tenant-1", Name: "proj", CreatedBy: "owner-1"}
	repo.members = append(repo.members,
		domain.ProjectMember{ProjectID: "p1", UserID: "owner-1", Role: domain.ProjectRoleOwner},
		domain.ProjectMember{ProjectID: "p1", UserID: "owner-2", Role: domain.ProjectRoleOwner},
	)
	repo.countOwners = 2
	uc := NewUpdateMemberRole(repo, &fakeOPAClient{decide: projectRegoDecide})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "owner-1")
	member, err := uc.Execute(ctx, UpdateMemberRoleInput{ProjectID: "p1", UserID: "owner-2", Role: domain.ProjectRoleMember})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if member.Role != domain.ProjectRoleMember {
		t.Errorf("expected role=member, got %v", member.Role)
	}
	if !repo.updateMemberRoleCalled {
		t.Error("expected UpdateMemberRole to be called")
	}
}

func TestUpdateMemberRole_DeniesNonOwnerActor(t *testing.T) {
	repo := newFakeProjectRepository()
	repo.projects["p1"] = domain.Project{ID: "p1", TenantID: "tenant-1", Name: "proj", CreatedBy: "owner-1"}
	repo.members = append(repo.members, domain.ProjectMember{ProjectID: "p1", UserID: "member-1", Role: domain.ProjectRoleMember})
	uc := NewUpdateMemberRole(repo, &fakeOPAClient{decide: projectRegoDecide})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "member-1")
	_, err := uc.Execute(ctx, UpdateMemberRoleInput{ProjectID: "p1", UserID: "member-1", Role: domain.ProjectRoleOwner})
	assertAppError(t, err, apperrors.KindPermissionDenied, "PROJECT_NOT_AUTHORIZED")
	if repo.updateMemberRoleCalled {
		t.Error("expected UpdateMemberRole to never be called for a denied caller")
	}
}

func TestUpdateMemberRole_MembershipNotFound(t *testing.T) {
	repo := newFakeProjectRepository()
	repo.projects["p1"] = domain.Project{ID: "p1", TenantID: "tenant-1", Name: "proj", CreatedBy: "owner-1"}
	repo.members = append(repo.members, domain.ProjectMember{ProjectID: "p1", UserID: "owner-1", Role: domain.ProjectRoleOwner})
	uc := NewUpdateMemberRole(repo, &fakeOPAClient{decide: projectRegoDecide})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "owner-1")
	_, err := uc.Execute(ctx, UpdateMemberRoleInput{ProjectID: "p1", UserID: "ghost-1", Role: domain.ProjectRoleMember})
	assertAppError(t, err, apperrors.KindNotFound, "PROJECT_MEMBERSHIP_NOT_FOUND")
}

func TestUpdateMemberRole_PromotionNeverBlockedByGuard(t *testing.T) {
	repo := newFakeProjectRepository()
	repo.projects["p1"] = domain.Project{ID: "p1", TenantID: "tenant-1", Name: "proj", CreatedBy: "owner-1"}
	repo.members = append(repo.members,
		domain.ProjectMember{ProjectID: "p1", UserID: "owner-1", Role: domain.ProjectRoleOwner},
		domain.ProjectMember{ProjectID: "p1", UserID: "member-1", Role: domain.ProjectRoleMember},
	)
	// currentOwnerCount deliberately left at the fake's zero value — a
	// promotion (member -> owner) must never consult CountOwners's value at
	// all, per AssertNotLastOwnerRemoval's targetRoleAfter == owner branch.
	uc := NewUpdateMemberRole(repo, &fakeOPAClient{decide: projectRegoDecide})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "owner-1")
	member, err := uc.Execute(ctx, UpdateMemberRoleInput{ProjectID: "p1", UserID: "member-1", Role: domain.ProjectRoleOwner})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if member.Role != domain.ProjectRoleOwner {
		t.Errorf("expected role=owner, got %v", member.Role)
	}
}

func TestUpdateMemberRole_RejectsInvalidRole(t *testing.T) {
	repo := newFakeProjectRepository()
	repo.projects["p1"] = domain.Project{ID: "p1", TenantID: "tenant-1", Name: "proj", CreatedBy: "owner-1"}
	repo.members = append(repo.members, domain.ProjectMember{ProjectID: "p1", UserID: "owner-1", Role: domain.ProjectRoleOwner})
	uc := NewUpdateMemberRole(repo, &fakeOPAClient{decide: projectRegoDecide})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "owner-1")
	_, err := uc.Execute(ctx, UpdateMemberRoleInput{ProjectID: "p1", UserID: "member-1", Role: domain.ProjectRole("")})
	assertAppError(t, err, apperrors.KindInvalidArgument, "PROJECT_INVALID_ROLE")
}
