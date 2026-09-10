package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

// TestGetSubtree_ExcludesMidTreeTaskButKeepsIndependentlyGrantedChild locks
// in GetSubtree's per-node access filter: a mid-tree task the caller has no
// grant on is excluded, but its own child — independently granted directly
// to the caller — is NOT excluded just because its parent was.
func TestGetSubtree_ExcludesMidTreeTaskButKeepsIndependentlyGrantedChild(t *testing.T) {
	tenantID := "tenant-1"
	userID := "user-1"

	root, _ := domain.NewTask("root", tenantID, "root", domain.StatusOpen, "", "")
	mid, _ := domain.NewTask("mid", tenantID, "mid", domain.StatusOpen, "root", "")
	leaf, _ := domain.NewTask("leaf", tenantID, "leaf", domain.StatusOpen, "mid", "")

	tasks := newFakeTaskRepository()
	tasks.tasks[root.ID] = root
	tasks.tasks[mid.ID] = mid
	tasks.tasks[leaf.ID] = leaf

	grants := &fakeGrantRepository{grants: []domain.Grant{
		{TaskID: "root", SubjectID: userID, Level: domain.GrantLevelOwner, ApplyTree: false}, // not inherited
		{TaskID: "leaf", SubjectID: userID, Level: domain.GrantLevelUser, ApplyTree: false},  // direct grant on leaf itself
	}}
	teams := &fakeTeamScopeResolver{}

	uc := NewGetSubtree(tasks, grants, teams)
	ctx := withIdentity(context.Background(), tenantID, userID)

	result, err := uc.Execute(ctx, GetSubtreeInput{RootID: "root"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	gotIDs := map[string]bool{}
	for _, task := range result.Tasks {
		gotIDs[task.ID] = true
	}
	if !gotIDs["root"] {
		t.Errorf("expected root to be visible (direct owner grant on itself)")
	}
	if gotIDs["mid"] {
		t.Errorf("expected mid to be EXCLUDED (no grant on it, root's grant is not apply_tree)")
	}
	if !gotIDs["leaf"] {
		t.Errorf("expected leaf to be visible (independent direct grant), even though its parent mid was excluded")
	}
}

func TestGetSubtree_NotFound(t *testing.T) {
	tasks := newFakeTaskRepository()
	grants := &fakeGrantRepository{}
	teams := &fakeTeamScopeResolver{}
	uc := NewGetSubtree(tasks, grants, teams)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	if _, err := uc.Execute(ctx, GetSubtreeInput{RootID: "missing"}); err == nil {
		t.Fatal("expected an error for a missing root task")
	}
}
