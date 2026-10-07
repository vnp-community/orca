package usecase

import (
	"context"
	"sync"
	"testing"
	"time"

	codeintelv1 "github.com/stablyai/orca-go/proto/gen/go/orca/codeintel/v1"
)

type mockPushBroadcaster struct {
	mu     sync.Mutex
	pushes []*codeintelv1.CodeIntelPush
}

func (m *mockPushBroadcaster) Publish(push *codeintelv1.CodeIntelPush) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pushes = append(m.pushes, push)
}

type mockQualitySink struct {
	mu           sync.Mutex
	finishedRuns []string
}

func (m *mockQualitySink) OnQualityFinished(ctx context.Context, tenantID, devServerID, workspaceRoot, runID, payloadJSON string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.finishedRuns = append(m.finishedRuns, runID)
	return nil
}

type mockProcessedStore struct {
	mu   sync.Mutex
	seen map[string]bool
}

func (m *mockProcessedStore) MarkProcessed(ctx context.Context, tenantID, eventID string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := tenantID + ":" + eventID
	if m.seen[key] {
		return false, nil
	}
	if m.seen == nil {
		m.seen = make(map[string]bool)
	}
	m.seen[key] = true
	return true, nil
}

func TestAgentEventHandler_IndexChanged_PushesAndDeduplicates(t *testing.T) {
	store := newFakeStore()
	probeGW := newMockGateway()
	probe := NewHeadProbe(probeGW, 15*time.Second)
	invalidator := NewSnapshotInvalidator(store, probe, nil, nil)
	broadcaster := &mockPushBroadcaster{}
	processed := &mockProcessedStore{}
	qualitySink := &mockQualitySink{}

	handler := NewAgentCodeIntelEventHandler(
		nil,
		invalidator,
		nil, // no coalescer in unit test -> direct flush
		broadcaster,
		nil,
		processed,
		nil,
		qualitySink,
		nil,
		nil,
	)

	payload := AgentEventPayload{
		Kind:          "index_changed",
		WorkspaceRoot: "/workspace/repo-1",
		Tool:          "gitnexus",
		Commit:        "commit-1",
		IndexedAt:     "2026-10-06T10:00:00Z",
	}

	// First execution -> succeeds and pushes
	err := handler.HandleEvent(context.Background(), "tenant-1", "dev-1", payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	broadcaster.mu.Lock()
	if len(broadcaster.pushes) != 1 {
		t.Fatalf("expected 1 push, got %d", len(broadcaster.pushes))
	}
	p := broadcaster.pushes[0]
	if p.Kind != "changed" || p.Reason != "index_changed" {
		t.Errorf("expected changed/index_changed, got %s/%s", p.Kind, p.Reason)
	}
	broadcaster.mu.Unlock()

	// Second execution with identical payload -> deduplicated by processedStore
	err = handler.HandleEvent(context.Background(), "tenant-1", "dev-1", payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	broadcaster.mu.Lock()
	if len(broadcaster.pushes) != 1 {
		t.Errorf("expected push count to stay 1 due to deduplication, got %d", len(broadcaster.pushes))
	}
	broadcaster.mu.Unlock()
}

func TestAgentEventHandler_ReindexProgress_TerminalStateFlushes(t *testing.T) {
	broadcaster := &mockPushBroadcaster{}
	handler := NewAgentCodeIntelEventHandler(
		nil, nil, nil, broadcaster, nil, nil, nil, nil, nil, nil,
	)

	pct := int32(50)
	// 1. Running progress
	_ = handler.HandleEvent(context.Background(), "t1", "d1", AgentEventPayload{
		Kind:          "reindex_progress",
		WorkspaceRoot: "/workspace/repo",
		JobID:         "job-1",
		Stage:         "indexing",
		Percent:       &pct,
		State:         "running",
	})

	broadcaster.mu.Lock()
	if len(broadcaster.pushes) != 1 || broadcaster.pushes[0].Kind != "reindex_progress" {
		t.Errorf("expected reindex_progress push")
	}
	broadcaster.mu.Unlock()

	// 2. Terminal state (succeeded)
	_ = handler.HandleEvent(context.Background(), "t1", "d1", AgentEventPayload{
		Kind:          "reindex_progress",
		WorkspaceRoot: "/workspace/repo",
		JobID:         "job-1",
		Stage:         "done",
		State:         "succeeded",
		Commit:        "commit-final",
	})

	broadcaster.mu.Lock()
	// Should have received reindex_progress + changed (terminal)
	if len(broadcaster.pushes) != 3 {
		t.Fatalf("expected 3 pushes total, got %d", len(broadcaster.pushes))
	}
	terminalPush := broadcaster.pushes[2]
	if terminalPush.Kind != "changed" || terminalPush.Reason != "reindex_finished" {
		t.Errorf("expected terminal push changed/reindex_finished, got %s/%s", terminalPush.Kind, terminalPush.Reason)
	}
	broadcaster.mu.Unlock()
}

func TestAgentEventHandler_QualityFinished_NotifiesSink(t *testing.T) {
	broadcaster := &mockPushBroadcaster{}
	qualitySink := &mockQualitySink{}

	handler := NewAgentCodeIntelEventHandler(
		nil, nil, nil, broadcaster, nil, nil, nil, qualitySink, nil, nil,
	)

	err := handler.HandleEvent(context.Background(), "t1", "d1", AgentEventPayload{
		Kind:          "quality_finished",
		WorkspaceRoot: "/workspace/repo",
		JobID:         "run-xyz",
		PayloadJSON:   `{"status":"PASSED"}`,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	qualitySink.mu.Lock()
	if len(qualitySink.finishedRuns) != 1 || qualitySink.finishedRuns[0] != "run-xyz" {
		t.Errorf("expected quality sink notified of run-xyz, got %+v", qualitySink.finishedRuns)
	}
	qualitySink.mu.Unlock()
}
