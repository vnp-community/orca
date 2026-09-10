package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// fakePollerRepository is an in-memory FleetHealthPollerRepository.
type fakePollerRepository struct {
	devServers []domain.DevServer
	listErr    error
}

func (f *fakePollerRepository) ListAllDevServers(ctx context.Context) ([]domain.DevServer, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.devServers, nil
}

// fakeFleetHealthWriter is an in-memory FleetHealthWriter.
type fakeFleetHealthWriter struct {
	written   []domain.DevServerHealth
	upsertErr error
	// previous seeds GetDevServerHealth's return per dev server — set
	// before Execute to simulate "this dev server was already
	// reachable/at some status last poll."
	previous map[string]domain.DevServerHealth
	getErr   error
}

func (f *fakeFleetHealthWriter) UpsertFleetHealth(ctx context.Context, health domain.DevServerHealth) error {
	if f.upsertErr != nil {
		return f.upsertErr
	}
	f.written = append(f.written, health)
	return nil
}

func (f *fakeFleetHealthWriter) GetDevServerHealth(ctx context.Context, devServerID string) (domain.DevServerHealth, bool, error) {
	if f.getErr != nil {
		return domain.DevServerHealth{}, false, f.getErr
	}
	h, ok := f.previous[devServerID]
	return h, ok, nil
}

// fakeOutboxWriter is an in-memory OutboxWriter.
type fakeOutboxWriter struct {
	inserted  []domain.OutboxEvent
	insertErr error
}

func (f *fakeOutboxWriter) InsertOutboxEvent(ctx context.Context, event domain.OutboxEvent) error {
	if f.insertErr != nil {
		return f.insertErr
	}
	f.inserted = append(f.inserted, event)
	return nil
}

// fakePollLock is an in-memory PollLockPort — denyList makes TryLock report
// locked=false for specific devServerIDs (simulating "another replica
// already holds this lock").
type fakePollLock struct {
	denyList map[string]bool
	unlocked []string
}

func (f *fakePollLock) TryLock(ctx context.Context, devServerID string) (bool, func(), error) {
	if f.denyList[devServerID] {
		return false, nil, nil
	}
	return true, func() {
		f.unlocked = append(f.unlocked, devServerID)
	}, nil
}

// fakeHealthEventPublisher/fakeWebhookAlerter record every call for
// assertions on call count.
type statusChangeCall struct {
	devServerID string
	from, to    domain.HealthStatus
}

type fakeHealthEventPublisher struct {
	calls []statusChangeCall
}

func (f *fakeHealthEventPublisher) PublishStatusChange(ctx context.Context, ds domain.DevServer, from, to domain.HealthStatus) {
	f.calls = append(f.calls, statusChangeCall{devServerID: ds.ID, from: from, to: to})
}

type fakeWebhookAlerter struct {
	calls []statusChangeCall
}

func (f *fakeWebhookAlerter) NotifyStatusChange(ctx context.Context, ds domain.DevServer, from, to domain.HealthStatus, sample domain.DevServerHealth) {
	f.calls = append(f.calls, statusChangeCall{devServerID: ds.ID, from: from, to: to})
}

// fakeMetricsCollector is an in-memory MetricsCollector.
type fakeMetricsCollector struct {
	updates []domain.DevServerHealth
}

func (f *fakeMetricsCollector) Update(devServerID, host string, sample domain.DevServerHealth) {
	f.updates = append(f.updates, sample)
}

func healthyExecResult() map[string]any {
	return map[string]any{
		"stdout": "cpu  100 0 50 850 0 0 0 0 0 0\n---\nMem: 1000 300 700 0 0 700\n---\nFilesystem 1024-blocks Used Available Capacity Mounted-on\n/dev/sda1 1000 100 900 10% /\n",
	}
}

func devServerForPollTest(t *testing.T, id string) domain.DevServer {
	t.Helper()
	ds, err := domain.NewDevServer(id, "tenant-1", "10.0.0.1", domain.ConnectionModeDirectWebSocket, "", nil)
	if err != nil {
		t.Fatalf("building dev server: %v", err)
	}
	return ds
}

func TestPollFleetHealth_HealthyServerWritesOneUpsert(t *testing.T) {
	ds, _ := domain.NewDevServer("ds1", "t1", "10.0.0.1", domain.ConnectionModeRelayWebSocket, "", nil)
	repo := &fakePollerRepository{devServers: []domain.DevServer{ds}}
	health := &fakeFleetHealthWriter{previous: map[string]domain.DevServerHealth{}}
	agent := &fakeDevServerAgentClient{healthy: true, execResult: healthyExecResult()}
	lock := &fakePollLock{}
	events := &fakeHealthEventPublisher{}
	webhook := &fakeWebhookAlerter{}

	uc := NewPollFleetHealth(repo, health, nil, agent, lock, nil, nil, events, webhook, nil, discardLogger())
	if err := uc.Execute(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(health.written) != 1 {
		t.Fatalf("expected exactly 1 UpsertFleetHealth call, got %d", len(health.written))
	}
	if health.written[0].Status != domain.HealthStatusHealthy {
		t.Errorf("expected status=healthy, got %q", health.written[0].Status)
	}
	if len(events.calls) != 0 || len(webhook.calls) != 0 {
		t.Errorf("expected no status-change calls on a first-ever sample, got events=%d webhook=%d", len(events.calls), len(webhook.calls))
	}
}

func TestPollFleetHealth_UpdatesMetricsCollectorWhenPresent(t *testing.T) {
	ds, _ := domain.NewDevServer("ds1", "t1", "10.0.0.1", domain.ConnectionModeRelayWebSocket, "", nil)
	repo := &fakePollerRepository{devServers: []domain.DevServer{ds}}
	health := &fakeFleetHealthWriter{previous: map[string]domain.DevServerHealth{}}
	agent := &fakeDevServerAgentClient{healthy: true, execResult: healthyExecResult()}
	lock := &fakePollLock{}
	events := &fakeHealthEventPublisher{}
	webhook := &fakeWebhookAlerter{}
	collector := &fakeMetricsCollector{}

	uc := NewPollFleetHealth(repo, health, nil, agent, lock, nil, nil, events, webhook, collector, discardLogger())
	if err := uc.Execute(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(collector.updates) != 1 || collector.updates[0].Status != domain.HealthStatusHealthy {
		t.Errorf("expected exactly 1 collector.Update call with status=healthy, got %+v", collector.updates)
	}
}

// TestPollFleetHealth_NilCollectorNeverPanics covers the every-other-test's
// implicit nil-collector coverage explicitly — PollFleetHealth must work
// fine before TASK-FLEET-03-08's metrics wiring lands anywhere that
// constructs it without one.
func TestPollFleetHealth_NilCollectorNeverPanics(t *testing.T) {
	ds, _ := domain.NewDevServer("ds1", "t1", "10.0.0.1", domain.ConnectionModeRelayWebSocket, "", nil)
	repo := &fakePollerRepository{devServers: []domain.DevServer{ds}}
	health := &fakeFleetHealthWriter{previous: map[string]domain.DevServerHealth{}}
	agent := &fakeDevServerAgentClient{healthy: true, execResult: healthyExecResult()}
	uc := NewPollFleetHealth(repo, health, nil, agent, &fakePollLock{}, nil, nil, &fakeHealthEventPublisher{}, &fakeWebhookAlerter{}, nil, discardLogger())
	if err := uc.Execute(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPollFleetHealth_StatusTransitionTriggersExactlyOneEventAndWebhookCall(t *testing.T) {
	ds, _ := domain.NewDevServer("ds1", "t1", "10.0.0.1", domain.ConnectionModeRelayWebSocket, "", nil)
	repo := &fakePollerRepository{devServers: []domain.DevServer{ds}}
	health := &fakeFleetHealthWriter{previous: map[string]domain.DevServerHealth{
		"ds1": {DevServerID: "ds1", Status: domain.HealthStatusHealthy},
	}}
	// Degraded via high CPU (jiffies chosen so busy/total > 80%).
	agent := &fakeDevServerAgentClient{healthy: true, execResult: map[string]any{
		"stdout": "cpu  900 0 0 100 0 0 0 0 0 0\n---\nMem: 1000 300 700 0 0 700\n---\nFilesystem 1024-blocks Used Available Capacity Mounted-on\n/dev/sda1 1000 100 900 10% /\n",
	}}
	lock := &fakePollLock{}
	events := &fakeHealthEventPublisher{}
	webhook := &fakeWebhookAlerter{}

	uc := NewPollFleetHealth(repo, health, nil, agent, lock, nil, nil, events, webhook, nil, discardLogger())
	if err := uc.Execute(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(health.written) != 1 || health.written[0].Status != domain.HealthStatusDegraded {
		t.Fatalf("expected 1 upsert with status=degraded, got %+v", health.written)
	}
	if len(events.calls) != 1 {
		t.Fatalf("expected exactly 1 PublishStatusChange call, got %d", len(events.calls))
	}
	if events.calls[0].from != domain.HealthStatusHealthy || events.calls[0].to != domain.HealthStatusDegraded {
		t.Errorf("unexpected status-change call: %+v", events.calls[0])
	}
	if len(webhook.calls) != 1 {
		t.Fatalf("expected exactly 1 NotifyStatusChange call, got %d", len(webhook.calls))
	}
}

func TestPollFleetHealth_NoTransitionTriggersNoEventOrWebhookCall(t *testing.T) {
	ds, _ := domain.NewDevServer("ds1", "t1", "10.0.0.1", domain.ConnectionModeRelayWebSocket, "", nil)
	repo := &fakePollerRepository{devServers: []domain.DevServer{ds}}
	health := &fakeFleetHealthWriter{previous: map[string]domain.DevServerHealth{
		"ds1": {DevServerID: "ds1", Status: domain.HealthStatusHealthy},
	}}
	agent := &fakeDevServerAgentClient{healthy: true, execResult: healthyExecResult()}
	lock := &fakePollLock{}
	events := &fakeHealthEventPublisher{}
	webhook := &fakeWebhookAlerter{}

	uc := NewPollFleetHealth(repo, health, nil, agent, lock, nil, nil, events, webhook, nil, discardLogger())
	if err := uc.Execute(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(events.calls) != 0 || len(webhook.calls) != 0 {
		t.Errorf("expected no status-change calls when status is unchanged, got events=%d webhook=%d", len(events.calls), len(webhook.calls))
	}
}

func TestPollFleetHealth_LockedFalseSkipsAgentCallsEntirely(t *testing.T) {
	ds, _ := domain.NewDevServer("ds1", "t1", "10.0.0.1", domain.ConnectionModeRelayWebSocket, "", nil)
	repo := &fakePollerRepository{devServers: []domain.DevServer{ds}}
	health := &fakeFleetHealthWriter{previous: map[string]domain.DevServerHealth{}}
	agent := &fakeDevServerAgentClient{healthy: true, execResult: healthyExecResult()}
	lock := &fakePollLock{denyList: map[string]bool{"ds1": true}}
	events := &fakeHealthEventPublisher{}
	webhook := &fakeWebhookAlerter{}

	uc := NewPollFleetHealth(repo, health, nil, agent, lock, nil, nil, events, webhook, nil, discardLogger())
	if err := uc.Execute(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(health.written) != 0 {
		t.Errorf("expected zero UpsertFleetHealth calls when TryLock returns locked=false, got %d", len(health.written))
	}
	agent.mu.Lock()
	execCalls := len(agent.execCalls)
	agent.mu.Unlock()
	if execCalls != 0 {
		t.Errorf("expected zero agent.Exec calls when TryLock returns locked=false, got %d", execCalls)
	}
}

func TestPollFleetHealth_UnreachableServerRecordsUnreachableStatus(t *testing.T) {
	ds, _ := domain.NewDevServer("ds1", "t1", "10.0.0.1", domain.ConnectionModeRelayWebSocket, "", nil)
	repo := &fakePollerRepository{devServers: []domain.DevServer{ds}}
	health := &fakeFleetHealthWriter{previous: map[string]domain.DevServerHealth{}}
	agent := &fakeDevServerAgentClient{healthy: false, healthErr: errors.New("connection refused")}
	lock := &fakePollLock{}
	events := &fakeHealthEventPublisher{}
	webhook := &fakeWebhookAlerter{}

	uc := NewPollFleetHealth(repo, health, nil, agent, lock, nil, nil, events, webhook, nil, discardLogger())
	if err := uc.Execute(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(health.written) != 1 || health.written[0].Status != domain.HealthStatusUnreachable {
		t.Fatalf("expected 1 upsert with status=unreachable, got %+v", health.written)
	}
	agent.mu.Lock()
	execCalls := len(agent.execCalls)
	agent.mu.Unlock()
	if execCalls != 0 {
		t.Errorf("expected agent.Exec to be skipped when unreachable, got %d calls", execCalls)
	}
}

func TestParseFleetMetrics(t *testing.T) {
	tests := []struct {
		name     string
		stdout   string
		wantCPU  float64
		wantRAM  float64
		wantDisk float64
	}{
		{
			name:     "well-formed output",
			stdout:   "cpu  100 0 50 850 0 0 0 0 0 0\n---\nMem: 1000 300 700 0 0 700\n---\nFilesystem 1024-blocks Used Available Capacity Mounted-on\n/dev/sda1 1000 100 900 10% /\n",
			wantCPU:  15, // (100+50) busy out of 1000 total = 15%
			wantRAM:  30, // 300/1000
			wantDisk: 10,
		},
		{
			name:     "malformed stat line degrades to zero, not a panic",
			stdout:   "not-a-valid-stat-line\n---\nMem: 1000 300 700 0 0 700\n---\nFilesystem 1024-blocks Used Available Capacity Mounted-on\n/dev/sda1 1000 100 900 10% /\n",
			wantCPU:  0,
			wantRAM:  30,
			wantDisk: 10,
		},
		{
			name:     "malformed free output degrades to zero",
			stdout:   "cpu  100 0 50 850 0 0 0 0 0 0\n---\ngarbage\n---\nFilesystem 1024-blocks Used Available Capacity Mounted-on\n/dev/sda1 1000 100 900 10% /\n",
			wantCPU:  15,
			wantRAM:  0,
			wantDisk: 10,
		},
		{
			name:     "malformed df output degrades to zero",
			stdout:   "cpu  100 0 50 850 0 0 0 0 0 0\n---\nMem: 1000 300 700 0 0 700\n---\ngarbage\n",
			wantCPU:  15,
			wantRAM:  30,
			wantDisk: 0,
		},
		{
			name:     "empty stdout never panics",
			stdout:   "",
			wantCPU:  0,
			wantRAM:  0,
			wantDisk: 0,
		},
		{
			name:     "missing stdout key never panics",
			stdout:   "",
			wantCPU:  0,
			wantRAM:  0,
			wantDisk: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cpu, ram, disk := parseFleetMetrics(map[string]any{"stdout": tt.stdout})
			if cpu != tt.wantCPU {
				t.Errorf("cpu = %v, want %v", cpu, tt.wantCPU)
			}
			if ram != tt.wantRAM {
				t.Errorf("ram = %v, want %v", ram, tt.wantRAM)
			}
			if disk != tt.wantDisk {
				t.Errorf("disk = %v, want %v", disk, tt.wantDisk)
			}
		})
	}
}

func TestParseFleetMetrics_MissingStdoutKeyNeverPanics(t *testing.T) {
	cpu, ram, disk := parseFleetMetrics(map[string]any{})
	if cpu != 0 || ram != 0 || disk != 0 {
		t.Errorf("expected all zeros for a result with no stdout key, got cpu=%v ram=%v disk=%v", cpu, ram, disk)
	}
}

// TestPollFleetHealth_WritesReachableSampleForEachDevServer is the live-bug
// regression: infra.fleet_health had zero rows for every dev server,
// forever, because nothing ever wrote to it — DevServerReachability.IsReachable
// (GetFleetHealth's real caller) always fell through to "no sample yet,
// treat as not reachable" even for a genuinely connected agent.
func TestPollFleetHealth_WritesReachableSampleForEachDevServer(t *testing.T) {
	ds1 := devServerForPollTest(t, "ds-1")
	ds2 := devServerForPollTest(t, "ds-2")
	repo := &fakePollerRepository{devServers: []domain.DevServer{ds1, ds2}}
	writer := &fakeFleetHealthWriter{}
	agent := &fakeDevServerAgentClient{healthy: true}

	uc := NewPollFleetHealth(repo, writer, nil, agent, nil, nil, nil, &fakeHealthEventPublisher{}, &fakeWebhookAlerter{}, nil, slog.Default())
	if err := uc.Execute(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(writer.written) != 2 {
		t.Fatalf("want 2 samples written, got %d", len(writer.written))
	}
	for _, sample := range writer.written {
		if !sample.Reachable {
			t.Errorf("want Reachable=true for %s, got false", sample.DevServerID)
		}
	}
}

func TestPollFleetHealth_RecordsUnreachableWhenHealthCheckFails(t *testing.T) {
	ds := devServerForPollTest(t, "ds-1")
	repo := &fakePollerRepository{devServers: []domain.DevServer{ds}}
	writer := &fakeFleetHealthWriter{}
	agent := &fakeDevServerAgentClient{healthErr: errors.New("dial failed")}

	uc := NewPollFleetHealth(repo, writer, nil, agent, nil, nil, nil, &fakeHealthEventPublisher{}, &fakeWebhookAlerter{}, nil, slog.Default())
	if err := uc.Execute(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(writer.written) != 1 {
		t.Fatalf("want 1 sample written, got %d", len(writer.written))
	}
	if writer.written[0].Reachable {
		t.Error("want Reachable=false when the health check errors")
	}
}

func TestPollFleetHealth_OneDevServerWriteFailureDoesNotStopTheRest(t *testing.T) {
	ds1 := devServerForPollTest(t, "ds-1")
	ds2 := devServerForPollTest(t, "ds-2")
	repo := &fakePollerRepository{devServers: []domain.DevServer{ds1, ds2}}
	writer := &fakeFleetHealthWriter{upsertErr: errors.New("db down")}
	agent := &fakeDevServerAgentClient{healthy: true}

	uc := NewPollFleetHealth(repo, writer, nil, agent, nil, nil, nil, &fakeHealthEventPublisher{}, &fakeWebhookAlerter{}, nil, slog.Default())
	if err := uc.Execute(context.Background()); err != nil {
		t.Fatalf("a per-dev-server write failure must not fail the whole poll: %v", err)
	}
}

func TestPollFleetHealth_ListFailurePropagates(t *testing.T) {
	repo := &fakePollerRepository{listErr: errors.New("db down")}
	writer := &fakeFleetHealthWriter{}
	agent := &fakeDevServerAgentClient{healthy: true}

	uc := NewPollFleetHealth(repo, writer, nil, agent, nil, nil, nil, &fakeHealthEventPublisher{}, &fakeWebhookAlerter{}, nil, slog.Default())
	if err := uc.Execute(context.Background()); err == nil {
		t.Fatal("expected the list failure to propagate")
	}
}

// TestPollFleetHealth_ReachableToUnreachableTransition_EnqueuesOneAlert is
// the admin-alerting regression: a dev server that WAS reachable and just
// went unreachable must enqueue exactly one outbox event (see
// alertDevServerDisconnected's doc comment on why not-repeated).
func TestPollFleetHealth_ReachableToUnreachableTransition_EnqueuesOneAlert(t *testing.T) {
	ds := devServerForPollTest(t, "ds-1")
	repo := &fakePollerRepository{devServers: []domain.DevServer{ds}}
	writer := &fakeFleetHealthWriter{
		previous: map[string]domain.DevServerHealth{"ds-1": {DevServerID: "ds-1", Reachable: true}},
	}
	outboxW := &fakeOutboxWriter{}
	agent := &fakeDevServerAgentClient{healthErr: errors.New("dial failed")}

	uc := NewPollFleetHealth(repo, writer, outboxW, agent, nil, nil, nil, &fakeHealthEventPublisher{}, &fakeWebhookAlerter{}, nil, slog.Default())
	if err := uc.Execute(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(outboxW.inserted) != 1 {
		t.Fatalf("want 1 outbox event enqueued, got %d", len(outboxW.inserted))
	}
	event := outboxW.inserted[0]
	if event.Subject != domain.DevServerDisconnectedSubject {
		t.Errorf("want subject %q, got %q", domain.DevServerDisconnectedSubject, event.Subject)
	}
	if event.TenantID != ds.TenantID {
		t.Errorf("want tenantId %q, got %q", ds.TenantID, event.TenantID)
	}
	var payload domain.DevServerDisconnectedPayload
	if err := json.Unmarshal(event.PayloadJSON, &payload); err != nil {
		t.Fatalf("unmarshaling payload: %v", err)
	}
	if payload.DevServerID != "ds-1" || payload.Host != ds.Host {
		t.Errorf("unexpected payload: %+v", payload)
	}
}

// TestPollFleetHealth_RepeatedUnreachableSamples_DoNotReAlert proves the
// edge-triggered rule: a dev server already recorded unreachable last poll
// must not enqueue another alert just for staying unreachable.
func TestPollFleetHealth_RepeatedUnreachableSamples_DoNotReAlert(t *testing.T) {
	ds := devServerForPollTest(t, "ds-1")
	repo := &fakePollerRepository{devServers: []domain.DevServer{ds}}
	writer := &fakeFleetHealthWriter{
		previous: map[string]domain.DevServerHealth{"ds-1": {DevServerID: "ds-1", Reachable: false}},
	}
	outboxW := &fakeOutboxWriter{}
	agent := &fakeDevServerAgentClient{healthErr: errors.New("still down")}

	uc := NewPollFleetHealth(repo, writer, outboxW, agent, nil, nil, nil, &fakeHealthEventPublisher{}, &fakeWebhookAlerter{}, nil, slog.Default())
	if err := uc.Execute(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(outboxW.inserted) != 0 {
		t.Fatalf("want no re-alert on a repeated unreachable sample, got %d", len(outboxW.inserted))
	}
}

// TestPollFleetHealth_UnreachableToReachable_DoesNotAlert proves a recovery
// edge (false -> true) never fires the disconnect alert.
func TestPollFleetHealth_UnreachableToReachable_DoesNotAlert(t *testing.T) {
	ds := devServerForPollTest(t, "ds-1")
	repo := &fakePollerRepository{devServers: []domain.DevServer{ds}}
	writer := &fakeFleetHealthWriter{
		previous: map[string]domain.DevServerHealth{"ds-1": {DevServerID: "ds-1", Reachable: false}},
	}
	outboxW := &fakeOutboxWriter{}
	agent := &fakeDevServerAgentClient{healthy: true}

	uc := NewPollFleetHealth(repo, writer, outboxW, agent, nil, nil, nil, &fakeHealthEventPublisher{}, &fakeWebhookAlerter{}, nil, slog.Default())
	if err := uc.Execute(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(outboxW.inserted) != 0 {
		t.Fatalf("want no alert on a recovery edge, got %d", len(outboxW.inserted))
	}
}

// TestPollFleetHealth_FirstEverPoll_NoPreviousSample_DoesNotAlert proves a
// dev server's very first poll (no row exists yet, found=false) never
// alerts even if it comes back unreachable — there is no "was reachable"
// edge to have transitioned from.
func TestPollFleetHealth_FirstEverPoll_NoPreviousSample_DoesNotAlert(t *testing.T) {
	ds := devServerForPollTest(t, "ds-1")
	repo := &fakePollerRepository{devServers: []domain.DevServer{ds}}
	writer := &fakeFleetHealthWriter{} // previous is nil — GetDevServerHealth returns found=false
	outboxW := &fakeOutboxWriter{}
	agent := &fakeDevServerAgentClient{healthErr: errors.New("dial failed")}

	uc := NewPollFleetHealth(repo, writer, outboxW, agent, nil, nil, nil, &fakeHealthEventPublisher{}, &fakeWebhookAlerter{}, nil, slog.Default())
	if err := uc.Execute(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(outboxW.inserted) != 0 {
		t.Fatalf("want no alert on a dev server's first-ever poll, got %d", len(outboxW.inserted))
	}
}

// TestPollFleetHealth_NilOutboxWriter_StillWritesSampleWithoutPanicking
// proves outbox is genuinely optional (main.go leaves it nil when NATS is
// unreachable at startup) — the WARN log still fires, just no enqueue.
func TestPollFleetHealth_NilOutboxWriter_StillWritesSampleWithoutPanicking(t *testing.T) {
	ds := devServerForPollTest(t, "ds-1")
	repo := &fakePollerRepository{devServers: []domain.DevServer{ds}}
	writer := &fakeFleetHealthWriter{
		previous: map[string]domain.DevServerHealth{"ds-1": {DevServerID: "ds-1", Reachable: true}},
	}
	agent := &fakeDevServerAgentClient{healthErr: errors.New("dial failed")}

	uc := NewPollFleetHealth(repo, writer, nil, agent, nil, nil, nil, &fakeHealthEventPublisher{}, &fakeWebhookAlerter{}, nil, slog.Default())
	if err := uc.Execute(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(writer.written) != 1 {
		t.Fatalf("want the sample still written despite outbox being nil, got %d", len(writer.written))
	}
}

// TestPollFleetHealth_ReachableToUnreachableTransition_MarksActiveConnectionDegraded
// is TASK-BE-STORAGE-009's usecase-level wiring check: a dev server's
// reachable=true -> false edge must transition its active Connection
// established -> degraded via the domain state machine (BE-SOL-STORAGE-003
// §2), not leave it dangling at "established" while the fleet health
// sample already says unreachable.
func TestPollFleetHealth_ReachableToUnreachableTransition_MarksActiveConnectionDegraded(t *testing.T) {
	ds := devServerForPollTest(t, "ds-1")
	repo := &fakePollerRepository{devServers: []domain.DevServer{ds}}
	writer := &fakeFleetHealthWriter{
		previous: map[string]domain.DevServerHealth{"ds-1": {DevServerID: "ds-1", Reachable: true}},
	}
	agent := &fakeDevServerAgentClient{healthErr: errors.New("dial failed")}
	conns := &fakeConnectionRepository{
		found:      true,
		activeConn: domain.Connection{ID: "conn-1", TenantID: ds.TenantID, DevServerID: ds.ID, Status: domain.ConnectionStatusEstablished, GracePeriodSeconds: 300},
	}

	uc := NewPollFleetHealth(repo, writer, nil, agent, nil, conns, nil, &fakeHealthEventPublisher{}, &fakeWebhookAlerter{}, nil, slog.Default())
	if err := uc.Execute(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(conns.updated) != 1 {
		t.Fatalf("want 1 connection status update, got %d", len(conns.updated))
	}
	got := conns.updated[0]
	if got.Status != domain.ConnectionStatusDegraded {
		t.Errorf("got status %q, want %q", got.Status, domain.ConnectionStatusDegraded)
	}
	if got.DegradedSince == nil {
		t.Error("expected DegradedSince to be set")
	}
}

// TestPollFleetHealth_UnreachableToReachable_ReestablishesActiveConnection
// is the recovery-side counterpart: an agent coming back within its grace
// period must return its degraded Connection to established, REUSING the
// same connectionId (never creating a new one) — BE-SOL-STORAGE-003 §2.
func TestPollFleetHealth_UnreachableToReachable_ReestablishesActiveConnection(t *testing.T) {
	ds := devServerForPollTest(t, "ds-1")
	repo := &fakePollerRepository{devServers: []domain.DevServer{ds}}
	writer := &fakeFleetHealthWriter{
		previous: map[string]domain.DevServerHealth{"ds-1": {DevServerID: "ds-1", Reachable: false}},
	}
	agent := &fakeDevServerAgentClient{healthy: true}
	degradedAt := time.Now().Add(-30 * time.Second)
	conns := &fakeConnectionRepository{
		found: true,
		activeConn: domain.Connection{
			ID: "conn-1", TenantID: ds.TenantID, DevServerID: ds.ID,
			Status: domain.ConnectionStatusDegraded, DegradedSince: &degradedAt, GracePeriodSeconds: 300,
		},
	}

	uc := NewPollFleetHealth(repo, writer, nil, agent, nil, conns, nil, &fakeHealthEventPublisher{}, &fakeWebhookAlerter{}, nil, slog.Default())
	if err := uc.Execute(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(conns.updated) != 1 {
		t.Fatalf("want 1 connection status update, got %d", len(conns.updated))
	}
	got := conns.updated[0]
	if got.Status != domain.ConnectionStatusEstablished {
		t.Errorf("got status %q, want %q", got.Status, domain.ConnectionStatusEstablished)
	}
	if got.ID != "conn-1" {
		t.Errorf("expected the same connection id to be reused, got %q", got.ID)
	}
	if got.DegradedSince != nil {
		t.Error("expected DegradedSince to be cleared")
	}
}

// TestMarkDegraded_DoesNotCloseTerminalSessions is TASK-BE-STORAGE-010's
// core regression: a reachable -> unreachable edge marks the active
// connection degraded (TASK-BE-STORAGE-009), but must NOT touch
// terminal_sessions at all — closed_at is only set once the connection
// actually reaches 'closed' (BE-SOL-STORAGE-003 §3). A wired-but-unused
// TerminalSessionRepository proves this, not merely "nil repository never
// crashes."
func TestMarkDegraded_DoesNotCloseTerminalSessions(t *testing.T) {
	ds := devServerForPollTest(t, "ds-1")
	repo := &fakePollerRepository{devServers: []domain.DevServer{ds}}
	writer := &fakeFleetHealthWriter{
		previous: map[string]domain.DevServerHealth{"ds-1": {DevServerID: "ds-1", Reachable: true}},
	}
	agent := &fakeDevServerAgentClient{healthErr: errors.New("dial failed")}
	conns := &fakeConnectionRepository{
		found:      true,
		activeConn: domain.Connection{ID: "conn-1", TenantID: ds.TenantID, DevServerID: ds.ID, Status: domain.ConnectionStatusEstablished, GracePeriodSeconds: 300},
	}
	sessions := &fakeTerminalSessionRepository{
		byPtyID: map[string]domain.TerminalSession{
			"pty-1": {PtyID: "pty-1", TenantID: ds.TenantID, ConnectionID: "conn-1"},
			"pty-2": {PtyID: "pty-2", TenantID: ds.TenantID, ConnectionID: "conn-1"},
		},
	}

	uc := NewPollFleetHealth(repo, writer, nil, agent, nil, conns, sessions, &fakeHealthEventPublisher{}, &fakeWebhookAlerter{}, nil, slog.Default())
	if err := uc.Execute(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := conns.activeConn.Status; got != domain.ConnectionStatusDegraded {
		t.Fatalf("want connection status %q, got %q", domain.ConnectionStatusDegraded, got)
	}
	if len(sessions.closeAllCalls) != 0 {
		t.Fatalf("want CloseAllForConnection never called on a degraded transition, got %d calls", len(sessions.closeAllCalls))
	}
	for ptyID, s := range sessions.byPtyID {
		if s.ClosedAt != nil {
			t.Errorf("want terminal session %q still open after degraded transition, got closed_at=%v", ptyID, *s.ClosedAt)
		}
	}
}

// TestCloseAfterGracePeriodExpiry_ClosesAllTerminalSessionsForConnection is
// the other half of TASK-BE-STORAGE-010: once the grace period has actually
// expired and reestablishConnection closes the connection instead of
// reestablishing it, every open terminal_sessions row for that connection
// must be closed too (BE-SOL-STORAGE-003 §3's "(b) connections.status
// chuyển sang closed" rule).
func TestCloseAfterGracePeriodExpiry_ClosesAllTerminalSessionsForConnection(t *testing.T) {
	ds := devServerForPollTest(t, "ds-1")
	repo := &fakePollerRepository{devServers: []domain.DevServer{ds}}
	writer := &fakeFleetHealthWriter{
		previous: map[string]domain.DevServerHealth{"ds-1": {DevServerID: "ds-1", Reachable: false}},
	}
	agent := &fakeDevServerAgentClient{healthy: true}
	degradedAt := time.Now().Add(-10 * time.Minute) // well past the 300s grace period
	conns := &fakeConnectionRepository{
		found: true,
		activeConn: domain.Connection{
			ID: "conn-1", TenantID: ds.TenantID, DevServerID: ds.ID,
			Status: domain.ConnectionStatusDegraded, DegradedSince: &degradedAt, GracePeriodSeconds: 300,
		},
	}
	sessions := &fakeTerminalSessionRepository{
		byPtyID: map[string]domain.TerminalSession{
			"pty-1": {PtyID: "pty-1", TenantID: ds.TenantID, ConnectionID: "conn-1"},
			"pty-2": {PtyID: "pty-2", TenantID: ds.TenantID, ConnectionID: "conn-1"},
			// a session on a DIFFERENT connection must be left untouched.
			"pty-other": {PtyID: "pty-other", TenantID: ds.TenantID, ConnectionID: "conn-other"},
		},
	}

	uc := NewPollFleetHealth(repo, writer, nil, agent, nil, conns, sessions, &fakeHealthEventPublisher{}, &fakeWebhookAlerter{}, nil, slog.Default())
	if err := uc.Execute(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := conns.activeConn.Status; got != domain.ConnectionStatusClosed {
		t.Fatalf("want connection status %q, got %q", domain.ConnectionStatusClosed, got)
	}
	if len(sessions.closeAllCalls) != 1 || sessions.closeAllCalls[0] != "conn-1" {
		t.Fatalf("want CloseAllForConnection called once with %q, got %v", "conn-1", sessions.closeAllCalls)
	}
	if s := sessions.byPtyID["pty-1"]; s.ClosedAt == nil {
		t.Error("want pty-1 closed after grace period expiry")
	}
	if s := sessions.byPtyID["pty-2"]; s.ClosedAt == nil {
		t.Error("want pty-2 closed after grace period expiry")
	}
	if s := sessions.byPtyID["pty-other"]; s.ClosedAt != nil {
		t.Error("want pty-other (a different connection) left untouched")
	}
}

// TestDegradedConnectionDoesNotTripCircuitBreaker (BE-SOL-STORAGE-003 §6,
// TASK-BE-STORAGE-009's test list) intentionally has no implementation
// here: the failure_count/circuit_broken concept it needs to assert against
// lives on orchestration-service's DispatchContext, not on anything
// infra-fleet-service owns (see BE-SOL-STORAGE-003 §4's "FailDispatch"
// classification table) — it depends on TASK-BE-STORAGE-011's dispatch
// classification landing there first, exactly as this task's own doc
// anticipates ("có thể viết trước dạng pending/skip nếu làm task này
// trước"). What this task DOES verify at the usecase level is the two
// tests immediately above: a reachable<->unreachable edge really does
// drive Connection.MarkDegraded/Reestablish, not an inline status
// assignment — the precondition TestDegradedConnectionDoesNotTripCircuitBreaker
// will need once orchestration-service's side exists.
