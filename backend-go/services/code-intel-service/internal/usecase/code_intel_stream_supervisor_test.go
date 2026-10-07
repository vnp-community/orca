package usecase

import (
	"context"
	"sync"
	"testing"
	"time"
)

type mockStreamTargets struct {
	mu      sync.Mutex
	targets []BindingStreamTarget
}

func (m *mockStreamTargets) ListTargets(ctx context.Context) ([]BindingStreamTarget, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.targets, nil
}

func (m *mockStreamTargets) FindTarget(ctx context.Context, tenantID, devServerID, workspaceRoot string) (*BindingStreamTarget, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, t := range m.targets {
		if t.TenantID == tenantID && t.DevServerID == devServerID && t.WorkspaceRoot == workspaceRoot {
			return &t, nil
		}
	}
	return nil, nil
}

type mockStreamListener struct {
	mu           sync.Mutex
	activeListen int
}

func (m *mockStreamListener) ListenStream(ctx context.Context, tenantID, devServerID string, onEvent func(AgentEventPayload)) error {
	m.mu.Lock()
	m.activeListen++
	m.mu.Unlock()

	<-ctx.Done()

	m.mu.Lock()
	m.activeListen--
	m.mu.Unlock()

	return ctx.Err()
}

func TestStreamSupervisor_ReconcilesAndCleansUp(t *testing.T) {
	targets := &mockStreamTargets{
		targets: []BindingStreamTarget{
			{TenantID: "t1", DevServerID: "d1"},
		},
	}
	listener := &mockStreamListener{}
	supervisor := NewCodeIntelStreamSupervisor(targets, listener, nil, 50*time.Millisecond, nil)

	ctx, cancel := context.WithCancel(context.Background())

	go supervisor.Start(ctx)

	// Wait for first reconcile
	time.Sleep(20 * time.Millisecond)

	listener.mu.Lock()
	if listener.activeListen != 1 {
		t.Fatalf("expected 1 active stream, got %d", listener.activeListen)
	}
	listener.mu.Unlock()

	// Cancel context to stop supervisor
	cancel()
	supervisor.StopAll()

	listener.mu.Lock()
	if listener.activeListen != 0 {
		t.Errorf("expected 0 active streams after stop, got %d", listener.activeListen)
	}
	listener.mu.Unlock()
}
