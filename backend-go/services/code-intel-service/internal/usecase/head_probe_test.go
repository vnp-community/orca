package usecase

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/code-intel-service/internal/domain"
)

func TestHeadProbe_ConcurrentSingleflight(t *testing.T) {
	mockGW := newMockGateway()
	mockGW.statusRes = RawCodeIntelResult{
		HeadCommit: "commit-abc",
		Data:       json.RawMessage(`{"indexes":{"gitnexus":{"headCommit":"commit-abc"}}}`),
	}
	probe := NewHeadProbe(mockGW, 15*time.Second)

	target := AgentTarget{
		TenantID:      "tenant-1",
		WorkspaceRoot: "/workspace/repo-1",
	}

	var wg sync.WaitGroup
	concurrent := 20
	wg.Add(concurrent)

	for i := 0; i < concurrent; i++ {
		go func() {
			defer wg.Done()
			entry, err := probe.Get(context.Background(), target, "repo-1")
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if entry.HeadCommit != "commit-abc" {
				t.Errorf("expected head commit 'commit-abc', got %s", entry.HeadCommit)
			}
		}()
	}

	wg.Wait()

	calls := mockGW.callCount("status")
	if calls != 1 {
		t.Fatalf("expected exactly 1 call to gateway.Status via singleflight, got %d", calls)
	}
}

func TestHeadProbe_ObserveAndInvalidate(t *testing.T) {
	mockGW := newMockGateway()
	probe := NewHeadProbe(mockGW, 15*time.Second)

	target := AgentTarget{TenantID: "tenant-1", WorkspaceRoot: "/workspace/repo-1"}

	// Observe manual entry
	probe.Observe("tenant-1", "repo-1", "commit-observed", nil, 0)

	entry, err := probe.Get(context.Background(), target, "repo-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entry.HeadCommit != "commit-observed" {
		t.Errorf("expected observed commit, got %s", entry.HeadCommit)
	}
	if mockGW.callCount("status") != 0 {
		t.Errorf("expected 0 gateway calls after observe")
	}

	// Invalidate
	probe.Invalidate("tenant-1", "repo-1")

	mockGW.statusRes = RawCodeIntelResult{HeadCommit: "commit-fresh"}
	entry2, err := probe.Get(context.Background(), target, "repo-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entry2.HeadCommit != "commit-fresh" {
		t.Errorf("expected fresh commit after invalidate, got %s", entry2.HeadCommit)
	}
	if mockGW.callCount("status") != 1 {
		t.Errorf("expected 1 gateway call after invalidate")
	}
}

func TestStaleness_Evaluate(t *testing.T) {
	eval := StalenessEvaluator{}
	t0 := time.Now()
	t1 := t0.Add(-1 * time.Hour)

	// Case 1: Fresh matching commits, no pending changes -> stale: false, miss: false
	stale, miss := eval.Evaluate(
		[]domain.ToolIndexStatus{{Tool: "gitnexus", IndexedCommit: "c1", Version: "1.0", IndexedAt: t0}},
		"c1",
		"c1",
		[]domain.ToolIndexStatus{{Tool: "gitnexus", IndexedCommit: "c1", Version: "1.0", IndexedAt: t0}},
		0,
	)
	if stale || miss {
		t.Errorf("expected fresh, got stale=%v, miss=%v", stale, miss)
	}

	// Case 2: Commit mismatch between head and index -> stale: true
	stale, miss = eval.Evaluate(
		[]domain.ToolIndexStatus{{Tool: "gitnexus", IndexedCommit: "c0", Version: "1.0", IndexedAt: t0}},
		"c1",
		"c1",
		[]domain.ToolIndexStatus{{Tool: "gitnexus", IndexedCommit: "c0", Version: "1.0", IndexedAt: t0}},
		0,
	)
	if !stale || miss {
		t.Errorf("expected stale due to commit mismatch, got stale=%v, miss=%v", stale, miss)
	}

	// Case 3: Pending changes > 0 -> stale: true
	stale, miss = eval.Evaluate(
		[]domain.ToolIndexStatus{{Tool: "gitnexus", IndexedCommit: "c1", Version: "1.0", IndexedAt: t0}},
		"c1",
		"c1",
		[]domain.ToolIndexStatus{{Tool: "gitnexus", IndexedCommit: "c1", Version: "1.0", IndexedAt: t0}},
		2,
	)
	if !stale || miss {
		t.Errorf("expected stale due to pending changes, got stale=%v, miss=%v", stale, miss)
	}

	// Case 4: Tool version changed -> miss: true (Branch B)
	stale, miss = eval.Evaluate(
		[]domain.ToolIndexStatus{{Tool: "gitnexus", IndexedCommit: "c1", Version: "2.0", IndexedAt: t0}},
		"c1",
		"c1",
		[]domain.ToolIndexStatus{{Tool: "gitnexus", IndexedCommit: "c1", Version: "1.0", IndexedAt: t0}},
		0,
	)
	if !miss {
		t.Errorf("expected miss due to tool version upgrade, got miss=%v", miss)
	}

	// Case 5: IndexedAt changed -> miss: true (Branch B)
	stale, miss = eval.Evaluate(
		[]domain.ToolIndexStatus{{Tool: "gitnexus", IndexedCommit: "c1", Version: "1.0", IndexedAt: t0}},
		"c1",
		"c1",
		[]domain.ToolIndexStatus{{Tool: "gitnexus", IndexedCommit: "c1", Version: "1.0", IndexedAt: t1}},
		0,
	)
	if !miss {
		t.Errorf("expected miss due to indexedAt change, got miss=%v", miss)
	}
}
