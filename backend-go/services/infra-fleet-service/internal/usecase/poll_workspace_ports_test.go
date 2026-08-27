package usecase

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// fakeScanResultSource lets tests script ScanWorkspacePorts.Execute's
// answer per tick without a real agent — ScanWorkspacePorts itself isn't an
// interface (it's a concrete *ScanWorkspacePorts), so these tests build one
// backed by a fakeConnectionResolver + fakeDevServerAgentClient whose Exec
// result is swapped between ticks via a small scripted sequence.
type scriptedAgent struct {
	mu      sync.Mutex
	results []map[string]any // consumed one per Exec call; last one repeats
	errs    []error
	calls   int
}

func (f *scriptedAgent) Exec(ctx context.Context, devServer domain.DevServer, method string, params map[string]any) (map[string]any, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	idx := f.calls
	if idx >= len(f.results) {
		idx = len(f.results) - 1
	}
	f.calls++
	var err error
	if idx < len(f.errs) {
		err = f.errs[idx]
	}
	if err != nil {
		return nil, err
	}
	return f.results[idx], nil
}

func (f *scriptedAgent) Health(ctx context.Context, devServer domain.DevServer) (bool, error) {
	return true, nil
}
func (f *scriptedAgent) SpawnPty(ctx context.Context, devServer domain.DevServer, in SpawnPtyInput) (SpawnPtyResult, error) {
	return SpawnPtyResult{}, errors.New("not used")
}
func (f *scriptedAgent) WritePty(ctx context.Context, devServer domain.DevServer, ptyID string, data []byte) error {
	return errors.New("not used")
}
func (f *scriptedAgent) ResizePty(ctx context.Context, devServer domain.DevServer, ptyID string, cols, rows int32) error {
	return errors.New("not used")
}
func (f *scriptedAgent) KillPty(ctx context.Context, devServer domain.DevServer, ptyID string, graceful bool) error {
	return errors.New("not used")
}
func (f *scriptedAgent) SendSignal(ctx context.Context, devServer domain.DevServer, ptyID string, signal string) error {
	return errors.New("not used")
}
func (f *scriptedAgent) StreamPty(ctx context.Context, devServer domain.DevServer, ptyID string) (<-chan PtyEvent, func(), error) {
	return nil, nil, errors.New("not used")
}
func (f *scriptedAgent) AgentStatus(ctx context.Context, devServer domain.DevServer, ptyID string) (AgentStatusResult, error) {
	return AgentStatusResult{}, errors.New("not used")
}
func (f *scriptedAgent) InspectProcess(ctx context.Context, devServer domain.DevServer, ptyID string) (InspectProcessResult, error) {
	return InspectProcessResult{}, errors.New("not used")
}
func (f *scriptedAgent) CancelReconnect(devServerID string) {}
func (f *scriptedAgent) LastHandshakeInfo(devServerID string) (HandshakeInfo, bool) {
	return HandshakeInfo{}, false
}

// fakePortAllocator hands out sequential fake ports — no real net.Listen.
type fakePortAllocator struct {
	mu       sync.Mutex
	next     int
	released []int
}

func (f *fakePortAllocator) Allocate(portForwardID string) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.next++
	return 3000 + f.next, nil
}

func (f *fakePortAllocator) Release(localPort int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.released = append(f.released, localPort)
}

// fakeTunnel/fakeTunnelOpener stand in for sshconn.Tunnel/Connection.Forward.
type fakeTunnel struct {
	mu     sync.Mutex
	closed bool
}

func (t *fakeTunnel) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.closed = true
	return nil
}

func (t *fakeTunnel) isClosed() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.closed
}

type fakeTunnelOpener struct {
	mu      sync.Mutex
	opened  []struct{ local, remote int }
	tunnels []*fakeTunnel
	failAt  map[int]bool // remotePort -> force Forward to fail
}

func (f *fakeTunnelOpener) Forward(localPort, remotePort int) (Tunnel, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failAt[remotePort] {
		return nil, errors.New("fakeTunnelOpener: forced failure")
	}
	f.opened = append(f.opened, struct{ local, remote int }{localPort, remotePort})
	tun := &fakeTunnel{}
	f.tunnels = append(f.tunnels, tun)
	return tun, nil
}

// fakePortForwardRepository is a minimal in-memory PortForwardRepository.
type fakePortForwardRepository struct {
	mu            sync.Mutex
	created       []domain.PortForward
	statusUpdates []struct {
		id     string
		status domain.PortForwardStatus
	}
	createErr error
}

func (f *fakePortForwardRepository) Create(ctx context.Context, pf domain.PortForward) (domain.PortForward, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.createErr != nil {
		return domain.PortForward{}, f.createErr
	}
	f.created = append(f.created, pf)
	return pf, nil
}

func (f *fakePortForwardRepository) UpdateStatus(ctx context.Context, tenantID, id string, status domain.PortForwardStatus) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.statusUpdates = append(f.statusUpdates, struct {
		id     string
		status domain.PortForwardStatus
	}{id, status})
	return nil
}

func (f *fakePortForwardRepository) ListActiveByConnection(ctx context.Context, tenantID, connectionID string) ([]domain.PortForward, error) {
	return nil, nil
}

// fakePortForwardEventPublisher records every Publish call.
type fakePortForwardEventPublisher struct {
	mu     sync.Mutex
	events []struct {
		event string
		pf    domain.PortForward
	}
}

func (f *fakePortForwardEventPublisher) Publish(ctx context.Context, event string, pf domain.PortForward) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.events = append(f.events, struct {
		event string
		pf    domain.PortForward
	}{event, pf})
}

func (f *fakePortForwardEventPublisher) eventKinds() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, len(f.events))
	for i, e := range f.events {
		out[i] = e.event
	}
	return out
}

func waitForCondition(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timed out waiting for condition")
}

func newTestPollWorkspacePorts(t *testing.T, agent *scriptedAgent, alloc *fakePortAllocator, tunnelOpener *fakeTunnelOpener, repo *fakePortForwardRepository, events *fakePortForwardEventPublisher) *PollWorkspacePorts {
	t.Helper()
	ds, err := domain.NewDevServer("ds-1", "tenant-1", "10.0.0.5", domain.ConnectionModeRelaySSH, "ssht-1")
	if err != nil {
		t.Fatalf("NewDevServer: %v", err)
	}
	resolver := &fakeConnectionResolver{byConnectionID: map[string]domain.DevServer{"conn-1": ds}}
	scan := NewScanWorkspacePorts(resolver, agent)
	return NewPollWorkspacePorts(scan, alloc, tunnelOpener, repo, events)
}

func TestPollWorkspacePorts_NewPortOpensTunnelAndPublishes(t *testing.T) {
	agent := &scriptedAgent{results: []map[string]any{
		{"ports": []any{map[string]any{"port": float64(3000), "processName": "node"}}},
	}}
	alloc := &fakePortAllocator{}
	tunnelOpener := &fakeTunnelOpener{failAt: map[int]bool{}}
	repo := &fakePortForwardRepository{}
	events := &fakePortForwardEventPublisher{}
	p := newTestPollWorkspacePorts(t, agent, alloc, tunnelOpener, repo, events)

	ctx, cancel := context.WithCancel(withTenant(context.Background(), "tenant-1"))
	done := make(chan struct{})
	go func() {
		p.Run(ctx, "tenant-1", "conn-1", "wt-1")
		close(done)
	}()

	waitForCondition(t, 3*time.Second, func() bool {
		return len(events.eventKinds()) >= 1
	})
	cancel()
	<-done

	kinds := events.eventKinds()
	if len(kinds) == 0 || kinds[0] != "dev_server.port_opened" {
		t.Fatalf("expected a dev_server.port_opened event, got %v", kinds)
	}
	tunnelOpener.mu.Lock()
	openedCount := len(tunnelOpener.opened)
	tunnelOpener.mu.Unlock()
	if openedCount != 1 {
		t.Errorf("expected exactly one tunnel opened, got %d", openedCount)
	}
}

func TestPollWorkspacePorts_PortDisappearing_TearsDownAndPublishesClosed(t *testing.T) {
	agent := &scriptedAgent{results: []map[string]any{
		{"ports": []any{map[string]any{"port": float64(3000), "processName": "node"}}},
		{"ports": []any{}}, // port no longer detected
		{"ports": []any{}},
	}}
	alloc := &fakePortAllocator{}
	tunnelOpener := &fakeTunnelOpener{failAt: map[int]bool{}}
	repo := &fakePortForwardRepository{}
	events := &fakePortForwardEventPublisher{}
	p := newTestPollWorkspacePorts(t, agent, alloc, tunnelOpener, repo, events)

	ctx, cancel := context.WithCancel(withTenant(context.Background(), "tenant-1"))
	done := make(chan struct{})
	go func() {
		p.Run(ctx, "tenant-1", "conn-1", "wt-1")
		close(done)
	}()

	waitForCondition(t, 5*time.Second, func() bool {
		kinds := events.eventKinds()
		return len(kinds) >= 2
	})
	cancel()
	<-done

	kinds := events.eventKinds()
	if len(kinds) < 2 || kinds[0] != "dev_server.port_opened" || kinds[1] != "dev_server.port_closed" {
		t.Fatalf("expected [opened, closed], got %v", kinds)
	}

	tunnelOpener.mu.Lock()
	tun := tunnelOpener.tunnels[0]
	tunnelOpener.mu.Unlock()
	if !tun.isClosed() {
		t.Error("expected the tunnel to be closed when the port stopped appearing")
	}

	repo.mu.Lock()
	defer repo.mu.Unlock()
	found := false
	for _, u := range repo.statusUpdates {
		if u.status == domain.PortForwardStatusClosed {
			found = true
		}
	}
	if !found {
		t.Error("expected UpdateStatus(closed) to be called")
	}
}

func TestPollWorkspacePorts_TransientScanError_LeavesExistingTunnelsUntouched(t *testing.T) {
	agent := &scriptedAgent{
		results: []map[string]any{
			{"ports": []any{map[string]any{"port": float64(3000), "processName": "node"}}},
			nil, // will error via errs[1]
			{"ports": []any{map[string]any{"port": float64(3000), "processName": "node"}}},
		},
		errs: []error{nil, errors.New("transient agent error")},
	}
	alloc := &fakePortAllocator{}
	tunnelOpener := &fakeTunnelOpener{failAt: map[int]bool{}}
	repo := &fakePortForwardRepository{}
	events := &fakePortForwardEventPublisher{}
	p := newTestPollWorkspacePorts(t, agent, alloc, tunnelOpener, repo, events)

	ctx, cancel := context.WithCancel(withTenant(context.Background(), "tenant-1"))
	done := make(chan struct{})
	go func() {
		p.Run(ctx, "tenant-1", "conn-1", "wt-1")
		close(done)
	}()

	// Let several ticks elapse (including the errored one) — the port
	// should never be reported closed despite the transient scan failure.
	// Checked BEFORE cancel(): Run's own deferred cleanup closes every still-
	// open tunnel on ctx cancellation (by design, tested separately below),
	// which would make a post-cancel check of isClosed() meaningless here.
	time.Sleep(2500 * time.Millisecond)

	kinds := events.eventKinds()
	for _, k := range kinds {
		if k == "dev_server.port_closed" {
			t.Errorf("expected no port_closed event from a transient scan error, got %v", kinds)
		}
	}
	tunnelOpener.mu.Lock()
	openedCount := len(tunnelOpener.opened)
	tun := tunnelOpener.tunnels[0]
	tunnelOpener.mu.Unlock()
	if openedCount != 1 {
		t.Errorf("expected exactly one tunnel ever opened (no duplicate on re-detection), got %d", openedCount)
	}
	if tun.isClosed() {
		t.Error("expected the tunnel to remain open across a transient scan error")
	}

	cancel()
	<-done
}

func TestPollWorkspacePorts_ContextCancellation_TearsDownEveryOpenTunnel(t *testing.T) {
	agent := &scriptedAgent{results: []map[string]any{
		{"ports": []any{
			map[string]any{"port": float64(3000), "processName": "node"},
			map[string]any{"port": float64(4000), "processName": "python"},
		}},
	}}
	alloc := &fakePortAllocator{}
	tunnelOpener := &fakeTunnelOpener{failAt: map[int]bool{}}
	repo := &fakePortForwardRepository{}
	events := &fakePortForwardEventPublisher{}
	p := newTestPollWorkspacePorts(t, agent, alloc, tunnelOpener, repo, events)

	ctx, cancel := context.WithCancel(withTenant(context.Background(), "tenant-1"))
	done := make(chan struct{})
	go func() {
		p.Run(ctx, "tenant-1", "conn-1", "wt-1")
		close(done)
	}()

	waitForCondition(t, 3*time.Second, func() bool {
		tunnelOpener.mu.Lock()
		defer tunnelOpener.mu.Unlock()
		return len(tunnelOpener.tunnels) >= 2
	})
	cancel()
	<-done

	tunnelOpener.mu.Lock()
	defer tunnelOpener.mu.Unlock()
	for i, tun := range tunnelOpener.tunnels {
		if !tun.isClosed() {
			t.Errorf("expected tunnel #%d to be closed on ctx cancellation", i)
		}
	}
}
