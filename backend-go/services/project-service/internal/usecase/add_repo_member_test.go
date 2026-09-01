package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

func TestAddRepoMember_ProjectOwnerCanGrant(t *testing.T) {
	repo := newFakeRepoRepository()
	repo.repos["r1"] = domain.Repo{ID: "r1", ProjectID: "p1", URL: "u1"}
	uc := NewAddRepoMember(repo, ownerMembership("p1", "owner-1"), &fakeOPAClient{repoDecide: repoRegoDecide})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "owner-1")
	member, err := uc.Execute(ctx, AddRepoMemberInput{RepoID: "r1", UserID: "dev-1", Role: domain.RepoRoleDeveloper})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if member.Role != domain.RepoRoleDeveloper || member.UserID != "dev-1" {
		t.Errorf("unexpected member: %+v", member)
	}
	if len(repo.repoMembers) != 1 {
		t.Errorf("expected one repo_members row, got %+v", repo.repoMembers)
	}
}

func TestAddRepoMember_ExistingRepoAdminCanGrant(t *testing.T) {
	repo := newFakeRepoRepository()
	repo.repos["r1"] = domain.Repo{ID: "r1", ProjectID: "p1", URL: "u1"}
	repo.repoMembers = append(repo.repoMembers, domain.RepoMember{RepoID: "r1", UserID: "admin-1", Role: domain.RepoRoleAdmin})
	membership := newFakeProjectRepository()
	membership.members = append(membership.members, domain.ProjectMember{ProjectID: "p1", UserID: "admin-1", Role: domain.ProjectRoleMember})
	uc := NewAddRepoMember(repo, membership, &fakeOPAClient{repoDecide: repoRegoDecide})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "admin-1")
	if _, err := uc.Execute(ctx, AddRepoMemberInput{RepoID: "r1", UserID: "dev-2", Role: domain.RepoRoleDeveloper}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAddRepoMember_PlainMemberWithNoRepoGrantDenied(t *testing.T) {
	repo := newFakeRepoRepository()
	repo.repos["r1"] = domain.Repo{ID: "r1", ProjectID: "p1", URL: "u1"}
	membership := newFakeProjectRepository()
	membership.members = append(membership.members, domain.ProjectMember{ProjectID: "p1", UserID: "member-1", Role: domain.ProjectRoleMember})
	uc := NewAddRepoMember(repo, membership, &fakeOPAClient{repoDecide: repoRegoDecide})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "member-1")
	_, err := uc.Execute(ctx, AddRepoMemberInput{RepoID: "r1", UserID: "dev-1", Role: domain.RepoRoleDeveloper})
	assertAppError(t, err, apperrors.KindPermissionDenied, "PROJECT_NOT_AUTHORIZED")
	if len(repo.repoMembers) != 0 {
		t.Errorf("expected no grant added, got %+v", repo.repoMembers)
	}
}

// DeveloperOrLeadCannotGrant: only repo-admin (or project owner) can grant —
// repoActionAdminOnly, not repo_lead_or_admin.
func TestAddRepoMember_RepoLeadCannotGrant(t *testing.T) {
	repo := newFakeRepoRepository()
	repo.repos["r1"] = domain.Repo{ID: "r1", ProjectID: "p1", URL: "u1"}
	repo.repoMembers = append(repo.repoMembers, domain.RepoMember{RepoID: "r1", UserID: "lead-1", Role: domain.RepoRoleLead})
	membership := newFakeProjectRepository()
	membership.members = append(membership.members, domain.ProjectMember{ProjectID: "p1", UserID: "lead-1", Role: domain.ProjectRoleMember})
	uc := NewAddRepoMember(repo, membership, &fakeOPAClient{repoDecide: repoRegoDecide})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "lead-1")
	_, err := uc.Execute(ctx, AddRepoMemberInput{RepoID: "r1", UserID: "dev-1", Role: domain.RepoRoleDeveloper})
	assertAppError(t, err, apperrors.KindPermissionDenied, "PROJECT_NOT_AUTHORIZED")
}

func TestAddRepoMember_RepoNotFound(t *testing.T) {
	repo := newFakeRepoRepository()
	uc := NewAddRepoMember(repo, newFakeProjectRepository(), allowAllOPA())

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	_, err := uc.Execute(ctx, AddRepoMemberInput{RepoID: "missing", UserID: "dev-1", Role: domain.RepoRoleDeveloper})
	assertAppError(t, err, apperrors.KindNotFound, "PROJECT_REPO_NOT_FOUND")
}

func TestAddRepoMember_RejectsInvalidRole(t *testing.T) {
	repo := newFakeRepoRepository()
	repo.repos["r1"] = domain.Repo{ID: "r1", ProjectID: "p1", URL: "u1"}
	uc := NewAddRepoMember(repo, newFakeProjectRepository(), allowAllOPA())

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	_, err := uc.Execute(ctx, AddRepoMemberInput{RepoID: "r1", UserID: "dev-1", Role: ""})
	assertAppError(t, err, apperrors.KindInvalidArgument, "PROJECT_REPO_MEMBER_INVALID")
}

func TestAddRepoMember_GlobalAdminAllowedRegardlessOfRole(t *testing.T) {
	repo := newFakeRepoRepository()
	repo.repos["r1"] = domain.Repo{ID: "r1", ProjectID: "p1", URL: "u1"}
	opa := &fakeOPAClient{repoDecide: func(callerProjectRole, callerRepoRole, callerGlobalRole, action string) bool { return true }}
	uc := NewAddRepoMember(repo, newFakeProjectRepository(), opa)

	ctx := withTenantAndUser(context.Background(), "tenant-1", "admin-1")
	if _, err := uc.Execute(ctx, AddRepoMemberInput{RepoID: "r1", UserID: "dev-1", Role: domain.RepoRoleDeveloper}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAddRepoMember_FailsClosedOnPolicyEvalError(t *testing.T) {
	repo := newFakeRepoRepository()
	repo.repos["r1"] = domain.Repo{ID: "r1", ProjectID: "p1", URL: "u1"}
	uc := NewAddRepoMember(repo, ownerMembership("p1", "owner-1"), &fakeOPAClient{err: errors.New("opa unreachable")})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "owner-1")
	_, err := uc.Execute(ctx, AddRepoMemberInput{RepoID: "r1", UserID: "dev-1", Role: domain.RepoRoleDeveloper})
	assertAppError(t, err, apperrors.KindInternal, "PROJECT_POLICY_EVAL_FAILED")
}
