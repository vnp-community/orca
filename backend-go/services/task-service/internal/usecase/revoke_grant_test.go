package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

func TestRevokeGrant_RequiresTenantContext(t *testing.T) {
	uc := NewRevokeGrant(&fakeGrantRepository{})
	err := uc.Execute(context.Background(), RevokeGrantInput{TaskID: "t1", SubjectID: "u1", Level: domain.GrantLevelAdmin})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestRevokeGrant_RemovesTheGrant(t *testing.T) {
	repo := &fakeGrantRepository{grants: []domain.Grant{
		{TaskID: "t1", SubjectID: "u1", Level: domain.GrantLevelAdmin},
	}}
	uc := NewRevokeGrant(repo)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	if err := uc.Execute(ctx, RevokeGrantInput{TaskID: "t1", SubjectID: "u1", Level: domain.GrantLevelAdmin}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repo.grants) != 0 {
		t.Errorf("expected the grant to be removed, got %+v", repo.grants)
	}
}

// TestRevokeGrant_Idempotent_SecondCallNoOps is the task's own named test:
// revoking a grant that no longer exists must not error.
func TestRevokeGrant_Idempotent_SecondCallNoOps(t *testing.T) {
	repo := &fakeGrantRepository{grants: []domain.Grant{
		{TaskID: "t1", SubjectID: "u1", Level: domain.GrantLevelAdmin},
	}}
	uc := NewRevokeGrant(repo)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	if err := uc.Execute(ctx, RevokeGrantInput{TaskID: "t1", SubjectID: "u1", Level: domain.GrantLevelAdmin}); err != nil {
		t.Fatalf("unexpected error on first revoke: %v", err)
	}
	if err := uc.Execute(ctx, RevokeGrantInput{TaskID: "t1", SubjectID: "u1", Level: domain.GrantLevelAdmin}); err != nil {
		t.Fatalf("expected the second revoke to no-op without error, got %v", err)
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
	revokeGrant := NewRevokeGrant(grants)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	if err := revokeGrant.Execute(ctx, RevokeGrantInput{TaskID: "child", SubjectID: "user-1", Level: domain.GrantLevelOwner}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resolvePermission := NewResolvePermission(tasks, grants, &fakeTeamScopeResolver{}, &fakeOPAClient{allow: true})
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
