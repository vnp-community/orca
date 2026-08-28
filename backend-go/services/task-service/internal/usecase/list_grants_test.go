package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

func newListGrantsForTest(tasks *fakeTaskRepository, grants *fakeGrantRepository, opaAllow bool) *ListGrants {
	resolvePermission := NewResolvePermission(tasks, grants, &fakeTeamScopeResolver{}, &fakeOPAClient{allow: opaAllow})
	return NewListGrants(grants, resolvePermission)
}

func TestListGrants_DeniesWhenCallerHasNoManageAccess(t *testing.T) {
	tasks := newFakeTaskRepository()
	tasks.tasks["t1"] = domain.Task{ID: "t1", TenantID: "tenant-1", OwnerID: "someone-else"}
	grants := &fakeGrantRepository{grants: []domain.Grant{{ID: "g1", TaskID: "t1", SubjectID: "u1", Level: domain.GrantLevelUser}}}
	uc := newListGrantsForTest(tasks, grants, true)
	ctx := withIdentity(context.Background(), "tenant-1", "attacker")

	if _, err := uc.Execute(ctx, "t1"); err == nil {
		t.Fatal("expected PermissionDenied for a caller with no manage access")
	}
}

// TestListGrants_ReturnsOnlyTargetTaskGrants_NotAncestorChain is the
// information-leak regression guard: ListGrants must return ONLY the
// target task's own grants, never an ancestor's.
func TestListGrants_ReturnsOnlyTargetTaskGrants_NotAncestorChain(t *testing.T) {
	tasks := newFakeTaskRepository()
	root, _ := domain.NewTask("root", "tenant-1", "root", domain.StatusOpen, "", "")
	root.OwnerID = "user-1"
	child, _ := domain.NewTask("child", "tenant-1", "child", domain.StatusOpen, "root", "")
	tasks.tasks["root"] = root
	tasks.tasks["child"] = child
	grants := &fakeGrantRepository{grants: []domain.Grant{
		// Inherited (ApplyTree=true) grant on root gives user-1 manage
		// access to child too — this is what lets the ListGrants call
		// below succeed; the assertion is that despite this inherited
		// access being resolved FROM root, the returned grant LIST is
		// still scoped to child only.
		{ID: "g-root", TaskID: "root", SubjectID: "user-1", Level: domain.GrantLevelAdmin, ApplyTree: true},
		{ID: "g-child", TaskID: "child", SubjectID: "someone-else", Level: domain.GrantLevelUser},
	}}
	uc := newListGrantsForTest(tasks, grants, true)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	got, err := uc.Execute(ctx, "child")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].ID != "g-child" {
		t.Errorf("expected only child's own grant, got %+v", got)
	}
}
