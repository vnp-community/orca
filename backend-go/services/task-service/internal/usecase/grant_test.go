package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

// newGrantForTest wires a real ResolvePermission (not a fake) so Grant's
// manage-access check is genuinely exercised end to end — tasks/grants are
// the same fakes the rest of this package's tests use.
func newGrantForTest(tasks *fakeTaskRepository, grants GrantRepository, opaAllow bool) *Grant {
	resolvePermission := NewResolvePermission(tasks, grants, &fakeTeamScopeResolver{}, &fakeOPAClient{allow: opaAllow})
	return NewGrant(grants, resolvePermission)
}

func TestGrant_RequiresTenantContext(t *testing.T) {
	uc := newGrantForTest(newFakeTaskRepository(), &fakeGrantRepository{}, true)
	err := uc.Execute(context.Background(), GrantInput{TaskID: "t1", SubjectID: "u1", Level: domain.GrantLevelAdmin})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

// TestGrant_PersistsAValidGrant: the caller is the task's owner (via
// CreateTask's OwnerID + ResolvePermission's owner-intrinsic short-circuit)
// so the manage check passes and the grant is persisted.
func TestGrant_PersistsAValidGrant(t *testing.T) {
	tasks := newFakeTaskRepository()
	tasks.tasks["t1"] = domain.Task{ID: "t1", TenantID: "tenant-1", OwnerID: "user-1"}
	repo := &fakeGrantRepository{}
	uc := newGrantForTest(tasks, repo, true)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	err := uc.Execute(ctx, GrantInput{TaskID: "t1", SubjectID: "u1", Level: domain.GrantLevelAdmin, ApplyTree: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repo.grants) != 1 || repo.grants[0].Level != domain.GrantLevelAdmin {
		t.Errorf("unexpected grants: %+v", repo.grants)
	}
}

// TestGrant_DeniesWhenCallerHasNoManageAccessToTarget is TASK-TG-03-01's
// core regression guard: a caller with no ownership and no grant on the
// target task must be denied — not silently allowed to write an arbitrary
// grant (the live privilege-escalation gap this task closes).
func TestGrant_DeniesWhenCallerHasNoManageAccessToTarget(t *testing.T) {
	tasks := newFakeTaskRepository()
	tasks.tasks["t1"] = domain.Task{ID: "t1", TenantID: "tenant-1", OwnerID: "someone-else"}
	repo := &fakeGrantRepository{}
	uc := newGrantForTest(tasks, repo, true) // even a permissive OPA can't rescue a caller with NO resolved grant level at all
	ctx := withIdentity(context.Background(), "tenant-1", "attacker")

	err := uc.Execute(ctx, GrantInput{TaskID: "t1", SubjectID: "attacker", Level: domain.GrantLevelOwner, ApplyTree: true})
	if err == nil {
		t.Fatal("expected PermissionDenied for a caller with no manage access to the target task")
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Kind != apperrors.KindPermissionDenied {
		t.Fatalf("expected KindPermissionDenied, got %v", err)
	}
	if len(repo.grants) != 0 {
		t.Errorf("expected NO grant to be persisted for a denied caller, got %+v", repo.grants)
	}
}

// TestGrant_DeniesWhenOPARefusesManageAction: the caller resolves a real
// (non-owner) grant level, but OPA's policy doesn't authorize "manage" for
// it — e.g. a "user"-level grantee should not be able to grant others
// access even though they have SOME grant.
func TestGrant_DeniesWhenOPARefusesManageAction(t *testing.T) {
	tasks := newFakeTaskRepository()
	tasks.tasks["t1"] = domain.Task{ID: "t1", TenantID: "tenant-1"}
	grants := &fakeGrantRepository{grants: []domain.Grant{
		{TaskID: "t1", SubjectID: "user-1", Level: domain.GrantLevelUser, ApplyTree: false},
	}}
	uc := newGrantForTest(tasks, grants, false) // OPA denies "manage" for a mere "user"-level grant
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	err := uc.Execute(ctx, GrantInput{TaskID: "t1", SubjectID: "user-1", Level: domain.GrantLevelOwner})
	if err == nil {
		t.Fatal("expected an error when OPA denies the manage action")
	}
}

func TestGrant_RejectsAnUnrecognizedLevel(t *testing.T) {
	uc := newGrantForTest(newFakeTaskRepository(), &fakeGrantRepository{}, true)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	err := uc.Execute(ctx, GrantInput{TaskID: "t1", SubjectID: "u1", Level: domain.GrantLevel(99)})
	if err == nil {
		t.Fatal("expected an error for an unrecognized grant level")
	}
}

func TestGrant_RejectsAnEmptySubject(t *testing.T) {
	uc := newGrantForTest(newFakeTaskRepository(), &fakeGrantRepository{}, true)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	err := uc.Execute(ctx, GrantInput{TaskID: "t1", SubjectID: "", Level: domain.GrantLevelUser})
	if err == nil {
		t.Fatal("expected an error for an empty subject id")
	}
}
