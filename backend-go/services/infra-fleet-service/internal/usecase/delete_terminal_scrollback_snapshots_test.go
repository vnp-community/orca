package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

func TestDeleteTerminalScrollbackSnapshots_DeletesOnlyTargetWorktree(t *testing.T) {
	now := time.Now()
	repo := &fakeTerminalScrollbackSnapshotRepository{
		byKey: map[string]domain.TerminalScrollbackSnapshot{
			scrollbackKey("tenant-1", "wt-1", "pane-a"): {TenantID: "tenant-1", WorktreeID: "wt-1", PaneKey: "pane-a", UpdatedAt: now},
			scrollbackKey("tenant-1", "wt-1", "pane-b"): {TenantID: "tenant-1", WorktreeID: "wt-1", PaneKey: "pane-b", UpdatedAt: now},
			scrollbackKey("tenant-1", "wt-2", "pane-a"): {TenantID: "tenant-1", WorktreeID: "wt-2", PaneKey: "pane-a", UpdatedAt: now},
		},
	}
	uc := NewDeleteTerminalScrollbackSnapshots(repo)

	ctx := withTenant(context.Background(), "tenant-1")
	if err := uc.Execute(ctx, "wt-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(repo.deleteByWorktreeCalls) != 1 {
		t.Fatalf("expected exactly one DeleteByWorktree call, got %d", len(repo.deleteByWorktreeCalls))
	}
	call := repo.deleteByWorktreeCalls[0]
	if call.tenantID != "tenant-1" || call.worktreeID != "wt-1" {
		t.Errorf("unexpected call args: %+v", call)
	}

	if _, ok := repo.byKey[scrollbackKey("tenant-1", "wt-2", "pane-a")]; !ok {
		t.Error("expected wt-2's snapshot to survive deletion of wt-1")
	}
	if _, ok := repo.byKey[scrollbackKey("tenant-1", "wt-1", "pane-a")]; ok {
		t.Error("expected wt-1/pane-a to be deleted")
	}
}
