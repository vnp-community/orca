package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

func newRevokeGrantForTest(tasks *fakeTaskRepository, grants *fakeGrantRepository, events EventPublisher, opaAllow bool) *RevokeGrant {
	resolvePermission := NewResolvePermission(tasks, grants, &fakeTeamScopeResolver{}, &fakeOPAClient{allow: opaAllow})
	return NewRevokeGrant(grants, resolvePermission, events)
}

func TestRevokeGrant_RequiresTenantContext(t *testing.T) {
	uc := newRevokeGrantForTest(newFakeTaskRepository(), &fakeGrantRepository{}, &fakeEventPublisher{}, true)
	err := uc.Execute(context.Background(), RevokeGrantInput{TaskID: "t1", GrantID: "g1"})
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

	err := uc.Execute(ctx, RevokeGrantInput{TaskID: "t1", GrantID: "g1"})
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

// TestRevokeGrant_NonexistentGrantID_ReturnsNotFound: revoking a
// nonexistent grant ID is NOT_FOUND, not a silent no-op.
func TestRevokeGrant_NonexistentGrantID_ReturnsNotFound(t *testing.T) {
	tasks := newFakeTaskRepository()
	tasks.tasks["t1"] = domain.Task{ID: "t1", TenantID: "tenant-1", OwnerID: "user-1"}
	grants := &fakeGrantRepository{}
	uc := newRevokeGrantForTest(tasks, grants, &fakeEventPublisher{}, true)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	err := uc.Execute(ctx, RevokeGrantInput{TaskID: "t1", GrantID: "does-not-exist"})
	if err == nil {
		t.Fatal("expected an error for a nonexistent grant id")
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Kind != apperrors.KindNotFound {
		t.Fatalf("expected KindNotFound, got %v", err)
	}
}

func TestRevokeGrant_SuccessfulRevoke_RemovesGrantAndPublishesEvent(t *testing.T) {
	tasks := newFakeTaskRepository()
	tasks.tasks["t1"] = domain.Task{ID: "t1", TenantID: "tenant-1", OwnerID: "user-1"}
	grants := &fakeGrantRepository{grants: []domain.Grant{{ID: "g1", TaskID: "t1", SubjectID: "u1", Level: domain.GrantLevelUser}}}
	events := &fakeEventPublisher{}
	uc := newRevokeGrantForTest(tasks, grants, events, true)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	if err := uc.Execute(ctx, RevokeGrantInput{TaskID: "t1", GrantID: "g1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(grants.grants) != 0 {
		t.Errorf("expected the grant to be removed, got %+v", grants.grants)
	}
	if len(events.events) != 1 || events.events[0].eventType != "task.grant_revoked" {
		t.Errorf("expected 1 task.grant_revoked event, got %+v", events.events)
	}
}
