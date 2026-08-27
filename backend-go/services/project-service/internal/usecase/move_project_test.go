package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

func TestMoveProject_RejectsNonexistentTargetGroup(t *testing.T) {
	repo := newFakeProjectRepository()
	repo.projects["p1"] = domain.Project{ID: "p1", TenantID: "tenant-1", Name: "proj", CreatedBy: "owner-1"}
	repo.members = append(repo.members, domain.ProjectMember{ProjectID: "p1", UserID: "owner-1", Role: domain.ProjectRoleOwner})
	groupRepo := newFakeProjectGroupRepository()
	uc := NewMoveProject(repo, groupRepo, &fakeOPAClient{decide: projectRegoDecide})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "owner-1")
	_, err := uc.Execute(ctx, "tenant-1", MoveProjectInput{ProjectID: "p1", TargetParentGroupID: "missing"})
	assertAppError(t, err, apperrors.KindNotFound, "PROJECT_GROUP_NOT_FOUND")
	if groupRepo.upsertLeafCalled {
		t.Error("expected UpsertLeafGroupForProject to never be called for a nonexistent target group")
	}
}

func TestMoveProject_RejectsOtherTenantsTargetGroup(t *testing.T) {
	repo := newFakeProjectRepository()
	repo.projects["p1"] = domain.Project{ID: "p1", TenantID: "tenant-1", Name: "proj", CreatedBy: "owner-1"}
	repo.members = append(repo.members, domain.ProjectMember{ProjectID: "p1", UserID: "owner-1", Role: domain.ProjectRoleOwner})
	groupRepo := newFakeProjectGroupRepository()
	// group-x belongs to a different tenant — GetProjectGroup (scoped by
	// tenantID) must not find it for tenant-1's caller.
	groupRepo.groups["group-x"] = domain.ProjectGroup{ID: "group-x", TenantID: "tenant-2", Name: "other tenant's group"}
	uc := NewMoveProject(repo, groupRepo, &fakeOPAClient{decide: projectRegoDecide})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "owner-1")
	_, err := uc.Execute(ctx, "tenant-1", MoveProjectInput{ProjectID: "p1", TargetParentGroupID: "group-x"})
	assertAppError(t, err, apperrors.KindNotFound, "PROJECT_GROUP_NOT_FOUND")
	if groupRepo.upsertLeafCalled {
		t.Error("expected UpsertLeafGroupForProject to never be called for another tenant's group")
	}
}

func TestMoveProject_CreatesLeafGroupWhenNoneExists(t *testing.T) {
	repo := newFakeProjectRepository()
	repo.projects["p1"] = domain.Project{ID: "p1", TenantID: "tenant-1", Name: "proj", CreatedBy: "owner-1"}
	repo.members = append(repo.members, domain.ProjectMember{ProjectID: "p1", UserID: "owner-1", Role: domain.ProjectRoleOwner})
	groupRepo := newFakeProjectGroupRepository()
	groupRepo.groups["parent-1"] = domain.ProjectGroup{ID: "parent-1", TenantID: "tenant-1", Name: "parent"}
	uc := NewMoveProject(repo, groupRepo, &fakeOPAClient{decide: projectRegoDecide})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "owner-1")
	group, err := uc.Execute(ctx, "tenant-1", MoveProjectInput{ProjectID: "p1", TargetParentGroupID: "parent-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !groupRepo.upsertLeafCalled {
		t.Error("expected UpsertLeafGroupForProject to be called")
	}
	if group.ParentGroupID != "parent-1" {
		t.Errorf("expected ParentGroupID=parent-1, got %q", group.ParentGroupID)
	}
	if group.ProjectID != "p1" {
		t.Errorf("expected ProjectID=p1, got %q", group.ProjectID)
	}
}

func TestMoveProject_DeniesNonOwnerActor(t *testing.T) {
	repo := newFakeProjectRepository()
	repo.projects["p1"] = domain.Project{ID: "p1", TenantID: "tenant-1", Name: "proj", CreatedBy: "owner-1"}
	repo.members = append(repo.members, domain.ProjectMember{ProjectID: "p1", UserID: "member-1", Role: domain.ProjectRoleMember})
	groupRepo := newFakeProjectGroupRepository()
	uc := NewMoveProject(repo, groupRepo, &fakeOPAClient{decide: projectRegoDecide})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "member-1")
	_, err := uc.Execute(ctx, "tenant-1", MoveProjectInput{ProjectID: "p1"})
	assertAppError(t, err, apperrors.KindPermissionDenied, "PROJECT_NOT_AUTHORIZED")
	if groupRepo.upsertLeafCalled {
		t.Error("expected UpsertLeafGroupForProject to never be called for a denied caller")
	}
}
