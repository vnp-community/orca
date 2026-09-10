package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

func newListGrantsForTest(tasks *fakeTaskRepository, grants *fakeGrantRepository, opaAllow bool) *ListGrants {
	resolvePermission := NewResolvePermission(tasks, grants, &fakeTeamScopeResolver{}, &fakeOPAClient{allow: opaAllow}, nil)
	return NewListGrants(grants, resolvePermission)
}

func TestListGrants_RequiresTenantContext(t *testing.T) {
	tasks := newFakeTaskRepository()
	tasks.tasks["t1"] = domain.Task{ID: "t1", TenantID: "tenant-1", OwnerID: "user-1"}
	uc := newListGrantsForTest(tasks, &fakeGrantRepository{}, true)
	if _, err := uc.Execute(context.Background(), "t1"); err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
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

// TestListGrants_ReturnsAllGrantsOnTheTask is the task's own named test: a
// task with 3 grants (owner/admin/team) returns all 3, expires_at populated
// correctly for the one with an expiry set.
func TestListGrants_ReturnsAllGrantsOnTheTask(t *testing.T) {
	tasks := newFakeTaskRepository()
	tasks.tasks["t1"] = domain.Task{ID: "t1", TenantID: "tenant-1", OwnerID: "user-1"}
	expiresAt := time.Now().Add(time.Hour)
	grants := &fakeGrantRepository{grants: []domain.Grant{
		{TaskID: "t1", SubjectID: "user-1", Level: domain.GrantLevelOwner},
		{TaskID: "t1", SubjectID: "user-2", Level: domain.GrantLevelAdmin, ExpiresAt: &expiresAt},
		{TaskID: "t1", SubjectID: "team-a", Level: domain.GrantLevelTeam},
		{TaskID: "t2", SubjectID: "user-3", Level: domain.GrantLevelOwner}, // a different task — must NOT appear
	}}
	uc := newListGrantsForTest(tasks, grants, true)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	got, err := uc.Execute(ctx, "t1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected exactly 3 grants for t1, got %d: %+v", len(got), got)
	}
	var foundExpiring bool
	for _, g := range got {
		if g.SubjectID == "user-2" {
			foundExpiring = true
			if g.ExpiresAt == nil || !g.ExpiresAt.Equal(expiresAt) {
				t.Errorf("expected ExpiresAt to match, got %+v", g.ExpiresAt)
			}
		}
	}
	if !foundExpiring {
		t.Error("expected the admin grant with an expiry to be present")
	}
}

func TestListGrants_EmptyForATaskWithNoGrants(t *testing.T) {
	tasks := newFakeTaskRepository()
	tasks.tasks["t1"] = domain.Task{ID: "t1", TenantID: "tenant-1", OwnerID: "user-1"}
	uc := newListGrantsForTest(tasks, &fakeGrantRepository{}, true)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	got, err := uc.Execute(ctx, "t1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected no grants, got %+v", got)
	}
}
