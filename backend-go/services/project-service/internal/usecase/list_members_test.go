package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

func TestListMembers_AnyMemberCanList(t *testing.T) {
	repo := newFakeProjectRepository()
	repo.projects["p1"] = domain.Project{ID: "p1", TenantID: "tenant-1", Name: "proj", CreatedBy: "owner-1"}
	repo.members = append(repo.members,
		domain.ProjectMember{ProjectID: "p1", UserID: "owner-1", Role: domain.ProjectRoleOwner},
		domain.ProjectMember{ProjectID: "p1", UserID: "member-1", Role: domain.ProjectRoleMember},
	)
	uc := NewListMembers(repo, &fakeOPAClient{decide: projectRegoDecide})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "member-1")
	members, err := uc.Execute(ctx, ListMembersInput{ProjectID: "p1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(members) != 2 {
		t.Errorf("expected 2 members, got %d: %+v", len(members), members)
	}
}

func TestListMembers_DeniesNonMember(t *testing.T) {
	repo := newFakeProjectRepository()
	repo.projects["p1"] = domain.Project{ID: "p1", TenantID: "tenant-1", Name: "proj", CreatedBy: "owner-1"}
	uc := NewListMembers(repo, &fakeOPAClient{decide: projectRegoDecide})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "stranger-1")
	_, err := uc.Execute(ctx, ListMembersInput{ProjectID: "p1"})
	if err == nil {
		t.Fatal("expected an error for a non-member caller")
	}
}
