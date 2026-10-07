package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/code-intel-service/internal/domain"
)

func TestSnapshotInvalidator_Idempotent(t *testing.T) {
	store := newFakeStore()
	probeGW := newMockGateway()
	probe := NewHeadProbe(probeGW, 15*time.Second)
	reader := NewCachedViewReader(&mockViewReaderSpy{}, store, probe, nil, time.Minute, time.Minute)

	invalidator := NewSnapshotInvalidator(store, probe, reader, nil)

	// Populate probe and store
	probe.Observe("t1", "b1", "commit-1", nil, 0)
	_ = store.Put(context.Background(), domain.Snapshot{
		Key: domain.SnapshotKey{Tenant: "t1", Binding: "b1", View: domain.ViewKindStructure},
	})

	// Call 1
	err1 := invalidator.InvalidateBinding(context.Background(), "t1", "b1", "index_changed")
	if err1 != nil {
		t.Fatalf("unexpected error on first call: %v", err1)
	}

	// Call 2 (idempotent, no error)
	err2 := invalidator.InvalidateBinding(context.Background(), "t1", "b1", "index_changed")
	if err2 != nil {
		t.Fatalf("unexpected error on second call: %v", err2)
	}

	// Verify store is empty for t1:b1
	if len(store.snapshots) != 0 {
		t.Errorf("expected snapshots to be deleted, got %d", len(store.snapshots))
	}
}

func TestSnapshotJanitor_StopsOnContextDone(t *testing.T) {
	store := newFakeStore()
	janitor := NewSnapshotJanitor(store, 10*time.Millisecond, 500, nil)

	ctx, cancel := context.WithCancel(context.Background())

	stopped := make(chan struct{})
	go func() {
		janitor.Start(ctx)
		close(stopped)
	}()

	time.Sleep(25 * time.Millisecond)
	cancel()

	select {
	case <-stopped:
		// Succeeded in stopping
	case <-time.After(500 * time.Millisecond):
		t.Fatal("janitor failed to stop after context cancellation")
	}
}
