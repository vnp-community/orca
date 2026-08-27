package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

func TestAddMember_OwnerCanAddAnotherMember(t *testing.T) {
	repo := newFakeProjectRepository()
	repo.projects["p1"] = domain.Project{ID: "p1", TenantID: "tenant-1", Name: "proj", CreatedBy: "owner-1"}
	repo.members = append(repo.members, domain.ProjectMember{ProjectID: "p1", UserID: "owner-1", Role: domain.ProjectRoleOwner})
	uc := NewAddMember(repo, &fakeOPAClient{decide: projectRegoDecide})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "owner-1")
	_, err := uc.Execute(ctx, AddMemberInput{ProjectID: "p1", UserID: "u2", Role: domain.ProjectRoleMember})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAddMember_MemberDeniedFromAddingAnotherMember(t *testing.T) {
	repo := newFakeProjectRepository()
	repo.projects["p1"] = domain.Project{ID: "p1", TenantID: "tenant-1", Name: "proj", CreatedBy: "owner-1"}
	repo.members = append(repo.members, domain.ProjectMember{ProjectID: "p1", UserID: "member-1", Role: domain.ProjectRoleMember})
	uc := NewAddMember(repo, &fakeOPAClient{decide: projectRegoDecide})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "member-1")
	_, err := uc.Execute(ctx, AddMemberInput{ProjectID: "p1", UserID: "u2", Role: domain.ProjectRoleMember})
	assertAppError(t, err, apperrors.KindPermissionDenied, "PROJECT_NOT_AUTHORIZED")
	if len(repo.members) != 1 {
		t.Errorf("expected no member added, got %+v", repo.members)
	}
}

func TestAddMember_CreatorBootstrapSelfAddBypassesOwnerCheck(t *testing.T) {
	repo := newFakeProjectRepository()
	repo.projects["p1"] = domain.Project{ID: "p1", TenantID: "tenant-1", Name: "proj", CreatedBy: "creator-1"}
	// No membership row exists yet — the creator has no project role to
	// present to OPA, matching the exact bootstrap gap this exception
	// closes. opa.allow stays false to prove the bypass path, not OPA,
	// authorized this call.
	opa := &fakeOPAClient{allow: false}
	uc := NewAddMember(repo, opa)

	ctx := withTenantAndUser(context.Background(), "tenant-1", "creator-1")
	member, err := uc.Execute(ctx, AddMemberInput{ProjectID: "p1", UserID: "creator-1", Role: domain.ProjectRoleOwner})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if member.Role != domain.ProjectRoleOwner {
		t.Errorf("expected owner role, got %v", member.Role)
	}
	if len(opa.calls) != 0 {
		t.Errorf("expected the bootstrap bypass to skip the OPA call entirely, got %+v", opa.calls)
	}
}

func TestAddMember_SelfAddByNonCreatorStillRequiresOwnerCheck(t *testing.T) {
	repo := newFakeProjectRepository()
	repo.projects["p1"] = domain.Project{ID: "p1", TenantID: "tenant-1", Name: "proj", CreatedBy: "creator-1"}
	// u2 is NOT the recorded creator and has no membership — self-adding
	// must not bypass authorization just because in.UserID == actorID.
	uc := NewAddMember(repo, &fakeOPAClient{decide: projectRegoDecide})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u2")
	_, err := uc.Execute(ctx, AddMemberInput{ProjectID: "p1", UserID: "u2", Role: domain.ProjectRoleOwner})
	assertAppError(t, err, apperrors.KindPermissionDenied, "PROJECT_NOT_AUTHORIZED")
}

func TestAddMember_GlobalAdminAllowedRegardlessOfProjectRole(t *testing.T) {
	repo := newFakeProjectRepository()
	repo.projects["p1"] = domain.Project{ID: "p1", TenantID: "tenant-1", Name: "proj", CreatedBy: "owner-1"}
	uc := NewAddMember(repo, &fakeOPAClient{decide: func(callerProjectRole, callerGlobalRole, action string) bool { return true }})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "admin-1")
	if _, err := uc.Execute(ctx, AddMemberInput{ProjectID: "p1", UserID: "u2", Role: domain.ProjectRoleMember}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAddMember_FailsClosedOnPolicyEvalError(t *testing.T) {
	repo := newFakeProjectRepository()
	repo.projects["p1"] = domain.Project{ID: "p1", TenantID: "tenant-1", Name: "proj", CreatedBy: "owner-1"}
	repo.members = append(repo.members, domain.ProjectMember{ProjectID: "p1", UserID: "owner-1", Role: domain.ProjectRoleOwner})
	uc := NewAddMember(repo, &fakeOPAClient{err: errors.New("opa unreachable")})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "owner-1")
	// owner-1 has a membership row but isn't the recorded creator adding
	// THEMSELVES as owner in this call — u2 is a different user, so the
	// bootstrap exception doesn't apply and the OPA error must surface.
	_, err := uc.Execute(ctx, AddMemberInput{ProjectID: "p1", UserID: "u2", Role: domain.ProjectRoleMember})
	assertAppError(t, err, apperrors.KindInternal, "PROJECT_POLICY_EVAL_FAILED")
}

func TestAddMember_RejectsInvalidMember(t *testing.T) {
	repo := newFakeProjectRepository()
	uc := NewAddMember(repo, allowAllOPA())

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	_, err := uc.Execute(ctx, AddMemberInput{ProjectID: "p1", UserID: "", Role: domain.ProjectRoleMember})
	assertAppError(t, err, apperrors.KindInvalidArgument, "PROJECT_MEMBER_INVALID")
}

func TestAddMember_RequiresTenantContext(t *testing.T) {
	repo := newFakeProjectRepository()
	uc := NewAddMember(repo, allowAllOPA())

	_, err := uc.Execute(context.Background(), AddMemberInput{ProjectID: "p1", UserID: "u2", Role: domain.ProjectRoleMember})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestAddMember_RequiresUserContext(t *testing.T) {
	repo := newFakeProjectRepository()
	uc := NewAddMember(repo, allowAllOPA())

	ctx := withTenant(context.Background(), "tenant-1")
	_, err := uc.Execute(ctx, AddMemberInput{ProjectID: "p1", UserID: "u2", Role: domain.ProjectRoleMember})
	assertAppError(t, err, apperrors.KindUnauthenticated, "PROJECT_NO_USER")
}
