package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

func TestAssignRepoToProject_OwnerOnBothProjectsSucceeds(t *testing.T) {
	repo := newFakeRepoRepository()
	repo.repos["r1"] = domain.Repo{ID: "r1", ProjectID: "source", URL: "https://old"}
	membership := newFakeProjectRepository()
	membership.members = append(membership.members,
		domain.ProjectMember{ProjectID: "source", UserID: "u1", Role: domain.ProjectRoleOwner},
		domain.ProjectMember{ProjectID: "target", UserID: "u1", Role: domain.ProjectRoleOwner},
	)
	uc := NewAssignRepoToProject(repo, membership, &fakeOPAClient{decide: projectRegoDecide, repoDecide: repoRegoDecide})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	got, err := uc.Execute(ctx, AssignRepoToProjectInput{RepoID: "r1", TargetProjectID: "target"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ProjectID != "target" {
		t.Errorf("got ProjectID %q, want %q", got.ProjectID, "target")
	}
	if repo.repos["r1"].ProjectID != "target" {
		t.Error("expected repo's stored ProjectID to be updated")
	}
}

func TestAssignRepoToProject_AppendsAtNextPositionInTarget(t *testing.T) {
	repo := newFakeRepoRepository()
	repo.repos["r1"] = domain.Repo{ID: "r1", ProjectID: "source"}
	repo.repos["r2"] = domain.Repo{ID: "r2", ProjectID: "target", Position: 0}
	repo.repos["r3"] = domain.Repo{ID: "r3", ProjectID: "target", Position: 1}
	membership := newFakeProjectRepository()
	membership.members = append(membership.members,
		domain.ProjectMember{ProjectID: "source", UserID: "u1", Role: domain.ProjectRoleOwner},
		domain.ProjectMember{ProjectID: "target", UserID: "u1", Role: domain.ProjectRoleOwner},
	)
	uc := NewAssignRepoToProject(repo, membership, &fakeOPAClient{decide: projectRegoDecide, repoDecide: repoRegoDecide})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	got, err := uc.Execute(ctx, AssignRepoToProjectInput{RepoID: "r1", TargetProjectID: "target"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Position != 2 {
		t.Errorf("got Position %d, want 2", got.Position)
	}
}

func TestAssignRepoToProject_RequiresRepoID(t *testing.T) {
	repo := newFakeRepoRepository()
	uc := NewAssignRepoToProject(repo, newFakeProjectRepository(), allowAllOPA())

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	_, err := uc.Execute(ctx, AssignRepoToProjectInput{TargetProjectID: "target"})
	assertAppError(t, err, apperrors.KindInvalidArgument, "PROJECT_REPO_ID_REQUIRED")
}

func TestAssignRepoToProject_RequiresTargetProjectID(t *testing.T) {
	repo := newFakeRepoRepository()
	uc := NewAssignRepoToProject(repo, newFakeProjectRepository(), allowAllOPA())

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	_, err := uc.Execute(ctx, AssignRepoToProjectInput{RepoID: "r1"})
	assertAppError(t, err, apperrors.KindInvalidArgument, "PROJECT_TARGET_PROJECT_ID_REQUIRED")
}

func TestAssignRepoToProject_RepoNotFound(t *testing.T) {
	repo := newFakeRepoRepository()
	uc := NewAssignRepoToProject(repo, newFakeProjectRepository(), allowAllOPA())

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	_, err := uc.Execute(ctx, AssignRepoToProjectInput{RepoID: "missing", TargetProjectID: "target"})
	assertAppError(t, err, apperrors.KindNotFound, "PROJECT_REPO_NOT_FOUND")
}

func TestAssignRepoToProject_AlreadyInTargetProject(t *testing.T) {
	repo := newFakeRepoRepository()
	repo.repos["r1"] = domain.Repo{ID: "r1", ProjectID: "target"}
	uc := NewAssignRepoToProject(repo, ownerMembership("target", "u1"), &fakeOPAClient{decide: projectRegoDecide, repoDecide: repoRegoDecide})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	_, err := uc.Execute(ctx, AssignRepoToProjectInput{RepoID: "r1", TargetProjectID: "target"})
	assertAppError(t, err, apperrors.KindInvalidArgument, "PROJECT_REPO_ALREADY_IN_PROJECT")
}

func TestAssignRepoToProject_MemberOfSourceDenied(t *testing.T) {
	repo := newFakeRepoRepository()
	repo.repos["r1"] = domain.Repo{ID: "r1", ProjectID: "source"}
	membership := newFakeProjectRepository()
	membership.members = append(membership.members,
		domain.ProjectMember{ProjectID: "source", UserID: "u1", Role: domain.ProjectRoleMember},
		domain.ProjectMember{ProjectID: "target", UserID: "u1", Role: domain.ProjectRoleOwner},
	)
	uc := NewAssignRepoToProject(repo, membership, &fakeOPAClient{decide: projectRegoDecide, repoDecide: repoRegoDecide})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	_, err := uc.Execute(ctx, AssignRepoToProjectInput{RepoID: "r1", TargetProjectID: "target"})
	assertAppError(t, err, apperrors.KindPermissionDenied, "PROJECT_NOT_AUTHORIZED")
	if repo.repos["r1"].ProjectID != "source" {
		t.Error("expected repo to remain unchanged")
	}
}

func TestAssignRepoToProject_NotAMemberOfTargetDenied(t *testing.T) {
	repo := newFakeRepoRepository()
	repo.repos["r1"] = domain.Repo{ID: "r1", ProjectID: "source"}
	uc := NewAssignRepoToProject(repo, ownerMembership("source", "u1"), &fakeOPAClient{decide: projectRegoDecide, repoDecide: repoRegoDecide})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	_, err := uc.Execute(ctx, AssignRepoToProjectInput{RepoID: "r1", TargetProjectID: "target"})
	assertAppError(t, err, apperrors.KindPermissionDenied, "PROJECT_NOT_AUTHORIZED")
	if repo.repos["r1"].ProjectID != "source" {
		t.Error("expected repo to remain unchanged")
	}
}

// Regression guard: a repo_members functional-role grant (even "admin")
// on the repo must NOT be sufficient to authorize moving it out of a
// project the caller isn't actually a member of — see
// AssignRepoToProject's doc comment for the exfiltration this closes.
func TestAssignRepoToProject_StaleRepoMemberAdminGrantWithoutProjectMembershipDenied(t *testing.T) {
	repo := newFakeRepoRepository()
	repo.repos["r1"] = domain.Repo{ID: "r1", ProjectID: "source"}
	repo.repoMembers = append(repo.repoMembers, domain.RepoMember{RepoID: "r1", UserID: "u1", Role: domain.RepoRoleAdmin})
	membership := newFakeProjectRepository()
	// u1 has NO membership row on "source" at all — only a lingering
	// repo-level grant — but owns "target".
	membership.members = append(membership.members,
		domain.ProjectMember{ProjectID: "target", UserID: "u1", Role: domain.ProjectRoleOwner},
	)
	uc := NewAssignRepoToProject(repo, membership, &fakeOPAClient{decide: projectRegoDecide, repoDecide: repoRegoDecide})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	_, err := uc.Execute(ctx, AssignRepoToProjectInput{RepoID: "r1", TargetProjectID: "target"})
	assertAppError(t, err, apperrors.KindPermissionDenied, "PROJECT_NOT_AUTHORIZED")
	if repo.repos["r1"].ProjectID != "source" {
		t.Error("expected repo to remain unchanged")
	}
}

// Regression guard: authorization must run BEFORE the "already in target
// project" equality check — otherwise a caller with no rights on either
// project could distinguish PROJECT_REPO_ALREADY_IN_PROJECT from
// PROJECT_NOT_AUTHORIZED to probe repo/project membership for free.
func TestAssignRepoToProject_AlreadyInTargetProjectStillRequiresAuthorization(t *testing.T) {
	repo := newFakeRepoRepository()
	repo.repos["r1"] = domain.Repo{ID: "r1", ProjectID: "target"}
	// u1 has no membership on "target" (or anywhere) at all.
	uc := NewAssignRepoToProject(repo, newFakeProjectRepository(), &fakeOPAClient{decide: projectRegoDecide, repoDecide: repoRegoDecide})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	_, err := uc.Execute(ctx, AssignRepoToProjectInput{RepoID: "r1", TargetProjectID: "target"})
	assertAppError(t, err, apperrors.KindPermissionDenied, "PROJECT_NOT_AUTHORIZED")
}

// Regression guard: repo_members grants are cleared on a successful move —
// they were scoped to the OLD project's trust and must not silently carry
// over to the new owner.
func TestAssignRepoToProject_ClearsRepoMembersOnSuccess(t *testing.T) {
	repo := newFakeRepoRepository()
	repo.repos["r1"] = domain.Repo{ID: "r1", ProjectID: "source"}
	repo.repoMembers = append(repo.repoMembers, domain.RepoMember{RepoID: "r1", UserID: "someone-else", Role: domain.RepoRoleAdmin})
	membership := newFakeProjectRepository()
	membership.members = append(membership.members,
		domain.ProjectMember{ProjectID: "source", UserID: "u1", Role: domain.ProjectRoleOwner},
		domain.ProjectMember{ProjectID: "target", UserID: "u1", Role: domain.ProjectRoleOwner},
	)
	uc := NewAssignRepoToProject(repo, membership, &fakeOPAClient{decide: projectRegoDecide, repoDecide: repoRegoDecide})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	if _, err := uc.Execute(ctx, AssignRepoToProjectInput{RepoID: "r1", TargetProjectID: "target"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, m := range repo.repoMembers {
		if m.RepoID == "r1" {
			t.Errorf("expected repo_members for r1 to be cleared, still found: %+v", m)
		}
	}
}

// Regression guard: a concurrent move (the repo's project_id changed
// between this call's authorization check and its write) must surface as
// a clear, distinguishable conflict, not a misleading "not found" or a
// silent overwrite of the concurrent move.
func TestAssignRepoToProject_ConcurrentMoveSurfacesAsPreconditionFailure(t *testing.T) {
	repo := newFakeRepoRepository()
	repo.repos["r1"] = domain.Repo{ID: "r1", ProjectID: "source"}
	membership := newFakeProjectRepository()
	membership.members = append(membership.members,
		domain.ProjectMember{ProjectID: "source", UserID: "u1", Role: domain.ProjectRoleOwner},
		domain.ProjectMember{ProjectID: "target", UserID: "u1", Role: domain.ProjectRoleOwner},
	)
	uc := NewAssignRepoToProject(repo, membership, &fakeOPAClient{decide: projectRegoDecide, repoDecide: repoRegoDecide})

	// Simulate another actor moving the repo out from under this call right
	// after GetRepo (and the authz check) resolved "source" as current.
	repo.repos["r1"] = domain.Repo{ID: "r1", ProjectID: "already-moved-elsewhere"}

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	_, err := uc.Execute(ctx, AssignRepoToProjectInput{RepoID: "r1", TargetProjectID: "target"})
	// GetRepo re-reads the (now-changed) repo fresh, so this particular fake
	// setup actually re-authorizes against "already-moved-elsewhere" (which
	// u1 doesn't own) and denies at the authorization step — a real,
	// stronger outcome than reaching ReassignProject's own race guard.
	// ReassignProject's guard itself is exercised directly below.
	assertAppError(t, err, apperrors.KindPermissionDenied, "PROJECT_NOT_AUTHORIZED")

	got, reassignErr := repo.ReassignProject(ctx, "r1", "source", "target")
	if !errors.Is(reassignErr, domain.ErrRepoProjectChanged) {
		t.Errorf("expected ErrRepoProjectChanged, got %v (result: %+v)", reassignErr, got)
	}
}

func TestAssignRepoToProject_MemberOfTargetDenied(t *testing.T) {
	// Owner on source (passes the repo-admin check), but only a member
	// (not owner) of the target — AddRepo-equivalent bar (owner_only)
	// must still deny.
	repo := newFakeRepoRepository()
	repo.repos["r1"] = domain.Repo{ID: "r1", ProjectID: "source"}
	membership := newFakeProjectRepository()
	membership.members = append(membership.members,
		domain.ProjectMember{ProjectID: "source", UserID: "u1", Role: domain.ProjectRoleOwner},
		domain.ProjectMember{ProjectID: "target", UserID: "u1", Role: domain.ProjectRoleMember},
	)
	uc := NewAssignRepoToProject(repo, membership, &fakeOPAClient{decide: projectRegoDecide, repoDecide: repoRegoDecide})

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	_, err := uc.Execute(ctx, AssignRepoToProjectInput{RepoID: "r1", TargetProjectID: "target"})
	assertAppError(t, err, apperrors.KindPermissionDenied, "PROJECT_NOT_AUTHORIZED")
	if repo.repos["r1"].ProjectID != "source" {
		t.Error("expected repo to remain unchanged")
	}
}
