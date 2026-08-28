package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

func TestExpireTerminalScrollbackSnapshots_DeletesOnlyRowsOlderThanTTL(t *testing.T) {
	now := time.Date(2026, 8, 27, 0, 0, 0, 0, time.UTC)
	old := now.Add(-domain.ScrollbackSnapshotTTL).Add(-24 * time.Hour)   // 31 days old — should be deleted
	fresh := now.Add(-domain.ScrollbackSnapshotTTL).Add(24 * time.Hour) // 29 days old — should survive

	repo := &fakeTerminalScrollbackSnapshotRepository{
		byKey: map[string]domain.TerminalScrollbackSnapshot{
			scrollbackKey("tenant-1", "wt-1", "pane-old"):   {TenantID: "tenant-1", WorktreeID: "wt-1", PaneKey: "pane-old", UpdatedAt: old},
			scrollbackKey("tenant-1", "wt-1", "pane-fresh"): {TenantID: "tenant-1", WorktreeID: "wt-1", PaneKey: "pane-fresh", UpdatedAt: fresh},
		},
	}
	uc := NewExpireTerminalScrollbackSnapshots(repo, fakeClock{now: now})

	deleted, err := uc.Execute(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deleted != 1 {
		t.Errorf("expected 1 deleted row, got %d", deleted)
	}
	if _, ok := repo.byKey[scrollbackKey("tenant-1", "wt-1", "pane-old")]; ok {
		t.Error("expected the 31-day-old row to be deleted")
	}
	if _, ok := repo.byKey[scrollbackKey("tenant-1", "wt-1", "pane-fresh")]; !ok {
		t.Error("expected the 29-day-old row to survive")
	}

	if len(repo.deleteExpiredCalls) != 1 {
		t.Fatalf("expected exactly one DeleteExpired call, got %d", len(repo.deleteExpiredCalls))
	}
	wantCutoff := now.Add(-domain.ScrollbackSnapshotTTL)
	if !repo.deleteExpiredCalls[0].Equal(wantCutoff) {
		t.Errorf("expected DeleteExpired called with cutoff %v, got %v", wantCutoff, repo.deleteExpiredCalls[0])
	}
}
