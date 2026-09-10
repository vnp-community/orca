package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

func newRevokeGrantForTest(tasks *fakeTaskRepository, grants *fakeGrantRepository, events EventPublisher, opaAllow bool) *RevokeGrant {
	resolvePermission := NewResolvePermission(tasks, grants, &fakeTeamScopeResolver{}, &fakeOPAClient{allow: opaAllow}, nil)
	return NewRevokeGrant(grants, resolvePermission, events)
}

func TestRevokeGrant_RequiresTenantContext(t *testing.T) {
	uc := newRevokeGrantForTest(newFakeTaskRepository(), &fakeGrantRepository{}, &fakeEventPublisher{}, true)
	err := uc.Execute(context.Background(), RevokeGrantInput{TaskID: "t1", SubjectID: "u1", Level: domain.GrantLevelAdmin})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

// TestRevokeGrant_DeniesWhenCallerHasNoManageAccess is the manage-gate
// regression guard: a caller without access to the target task must be
// denied, never allowed to revoke a grant.
func TestRevokeGrant_DeniesWhenCallerHasNoManageAccess(t *testing.T) {
	tasks := newFakeTaskRepository()
	tasks.tasks["t1"] = domain.Task{ID: "t1", TenantID: "tenant-1", OwnerID: "someone-else"}
	grants := &fakeGrantRepository{grants: []domain.Grant{{ID: "g1", TaskID: "t1", SubjectID: "u1", Level: domain.GrantLevelUser}}}
	uc := newRevokeGrantForTest(tasks, grants, &fakeEventPublisher{}, true)
	ctx := withIdentity(context.Background(), "tenant-1", "attacker")

	err := uc.Execute(ctx, RevokeGrantInput{TaskID: "t1", SubjectID: "u1", Level: domain.GrantLevelUser})
	if err == nil {
		t.Fatal("expected PermissionDenied for a caller with no manage access")
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Kind != apperrors.KindPermissionDenied {
		t.Fatalf("expected KindPermissionDenied, got %v", err)
	}
	if len(grants.grants) != 1 {
		t.Errorf("expected the grant to survive a denied revoke, got %+v", grants.grants)
	}
}

// TestRevokeGrant_NonexistentGrant_IsIdempotent: revoking a
// (task_id, subject_id, level) tuple that doesn't (or no longer) exist is
// NOT an error — see GrantRepository.Revoke's doc comment.
func TestRevokeGrant_NonexistentGrant_IsIdempotent(t *testing.T) {
	tasks := newFakeTaskRepository()
	tasks.tasks["t1"] = domain.Task{ID: "t1", TenantID: "tenant-1", OwnerID: "user-1"}
	grants := &fakeGrantRepository{}
	uc := newRevokeGrantForTest(tasks, grants, &fakeEventPublisher{}, true)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	if err := uc.Execute(ctx, RevokeGrantInput{TaskID: "t1", SubjectID: "does-not-exist", Level: domain.GrantLevelUser}); err != nil {
		t.Fatalf("expected a no-op, not an error, for a nonexistent grant: %v", err)
	}
}

func TestRevokeGrant_SuccessfulRevoke_RemovesGrantAndPublishesEvent(t *testing.T) {
	tasks := newFakeTaskRepository()
	tasks.tasks["t1"] = domain.Task{ID: "t1", TenantID: "tenant-1", OwnerID: "user-1"}
	grants := &fakeGrantRepository{grants: []domain.Grant{{ID: "g1", TaskID: "t1", SubjectID: "u1", Level: domain.GrantLevelUser}}}
	events := &fakeEventPublisher{}
	uc := newRevokeGrantForTest(tasks, grants, events, true)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	if err := uc.Execute(ctx, RevokeGrantInput{TaskID: "t1", SubjectID: "u1", Level: domain.GrantLevelUser}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(grants.grants) != 0 {
		t.Errorf("expected the grant to be removed, got %+v", grants.grants)
	}
	if len(events.events) != 1 || events.events[0].eventType != "task.grant_revoked" {
		t.Errorf("expected 1 task.grant_revoked event, got %+v", events.events)
	}
}

// TestRevokeGrant_OnlyRemovesTheOneRow proves revoking a caller's direct
// grant doesn't disturb an ancestor's inherited (ApplyTree=true) grant that
// still covers a DIFFERENT caller — revoke only removes the ONE matching
// (task_id, subject_id, level) row, not the whole resolution chain.
func TestRevokeGrant_OnlyRemovesTheOneRow(t *testing.T) {
	tasks := newFakeTaskRepository()
	setupChain(t, tasks, "tenant-1", "root", "child")
	grants := &fakeGrantRepository{grants: []domain.Grant{
		{TaskID: "child", SubjectID: "user-1", Level: domain.GrantLevelOwner, ApplyTree: false},
		{TaskID: "root", SubjectID: "user-2", Level: domain.GrantLevelAdmin, ApplyTree: true},
	}}
	// user-1 revokes their own direct grant — manage access resolves via
	// that same direct Owner grant.
	revokeGrant := newRevokeGrantForTest(tasks, grants, &fakeEventPublisher{}, true)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	if err := revokeGrant.Execute(ctx, RevokeGrantInput{TaskID: "child", SubjectID: "user-1", Level: domain.GrantLevelOwner}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resolvePermission := NewResolvePermission(tasks, grants, &fakeTeamScopeResolver{}, &fakeOPAClient{allow: true}, nil)
	// user-1's own grant is gone: no grant matches them anymore.
	if _, err := resolvePermission.Execute(ctx, ResolvePermissionInput{TaskID: "child", UserID: "user-1", Action: "read"}); err == nil {
		t.Error("expected user-1 to lose access after their own grant was revoked")
	}
	// user-2's inherited root grant is untouched.
	level, err := resolvePermission.Execute(withIdentity(context.Background(), "tenant-1", "user-2"), ResolvePermissionInput{TaskID: "child", UserID: "user-2", Action: "read"})
	if err != nil {
		t.Fatalf("expected user-2 to still resolve via the untouched ancestor grant, got %v", err)
	}
	if level != domain.GrantLevelAdmin {
		t.Errorf("expected GrantLevelAdmin, got %v", level)
	}
}
