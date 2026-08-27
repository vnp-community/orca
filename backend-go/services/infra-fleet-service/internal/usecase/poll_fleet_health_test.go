package usecase

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// fakeFleetHealthWriter is an in-memory FleetHealthWriter.
type fakeFleetHealthWriter struct {
	mu sync.Mutex

	upserted   []domain.DevServerHealth
	upsertErr  error
	previous   map[string]domain.DevServerHealth
	getPrevErr error
}

func (f *fakeFleetHealthWriter) UpsertFleetHealth(ctx context.Context, sample domain.DevServerHealth) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.upsertErr != nil {
		return f.upsertErr
	}
	f.upserted = append(f.upserted, sample)
	return nil
}

func (f *fakeFleetHealthWriter) GetPrevious(ctx context.Context, devServerID string) (domain.DevServerHealth, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.getPrevErr != nil {
		return domain.DevServerHealth{}, false, f.getPrevErr
	}
	sample, ok := f.previous[devServerID]
	return sample, ok, nil
}

// fakePollLock is an in-memory PollLockPort — denyList makes TryLock report
// locked=false for specific devServerIDs (simulating "another replica
// already holds this lock").
type fakePollLock struct {
	mu       sync.Mutex
	denyList map[string]bool
	unlocked []string
}

func (f *fakePollLock) TryLock(ctx context.Context, devServerID string) (bool, func(), error) {
	f.mu.Lock()
	deny := f.denyList[devServerID]
	f.mu.Unlock()
	if deny {
		return false, nil, nil
	}
	return true, func() {
		f.mu.Lock()
		f.unlocked = append(f.unlocked, devServerID)
		f.mu.Unlock()
	}, nil
}

// fakeHealthEventPublisher/fakeWebhookAlerter record every call for
// assertions on call count.
type statusChangeCall struct {
	devServerID string
	from, to    domain.HealthStatus
}

type fakeHealthEventPublisher struct {
	mu    sync.Mutex
	calls []statusChangeCall
}

func (f *fakeHealthEventPublisher) PublishStatusChange(ctx context.Context, ds domain.DevServer, from, to domain.HealthStatus) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, statusChangeCall{devServerID: ds.ID, from: from, to: to})
}

type fakeWebhookAlerter struct {
	mu    sync.Mutex
	calls []statusChangeCall
}

func (f *fakeWebhookAlerter) NotifyStatusChange(ctx context.Context, ds domain.DevServer, from, to domain.HealthStatus, sample domain.DevServerHealth) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, statusChangeCall{devServerID: ds.ID, from: from, to: to})
}

func healthyExecResult() map[string]any {
	return map[string]any{
		"stdout": "cpu  100 0 50 850 0 0 0 0 0 0\n---\nMem: 1000 300 700 0 0 700\n---\nFilesystem 1024-blocks Used Available Capacity Mounted-on\n/dev/sda1 1000 100 900 10% /\n",
	}
}

func TestPollFleetHealth_HealthyServerWritesOneUpsert(t *testing.T) {
	ds, _ := domain.NewDevServer("ds1", "t1", "10.0.0.1", domain.ConnectionModeRelayWebSocket, "")
	devRepo := &fakeDevServerRepository{byID: map[string]domain.DevServer{"ds1": ds}}
	health := &fakeFleetHealthWriter{previous: map[string]domain.DevServerHealth{}}
	agent := &fakeDevServerAgentClient{healthy: true, execResult: healthyExecResult()}
	lock := &fakePollLock{}
	events := &fakeHealthEventPublisher{}
	webhook := &fakeWebhookAlerter{}

	uc := NewPollFleetHealth(devRepo, health, agent, lock, events, webhook, discardLogger())
	uc.pollOnce(context.Background())

	if len(health.upserted) != 1 {
		t.Fatalf("expected exactly 1 UpsertFleetHealth call, got %d", len(health.upserted))
	}
	if health.upserted[0].Status != domain.HealthStatusHealthy {
		t.Errorf("expected status=healthy, got %q", health.upserted[0].Status)
	}
	if len(events.calls) != 0 || len(webhook.calls) != 0 {
		t.Errorf("expected no status-change calls on a first-ever sample, got events=%d webhook=%d", len(events.calls), len(webhook.calls))
	}
}

func TestPollFleetHealth_StatusTransitionTriggersExactlyOneEventAndWebhookCall(t *testing.T) {
	ds, _ := domain.NewDevServer("ds1", "t1", "10.0.0.1", domain.ConnectionModeRelayWebSocket, "")
	devRepo := &fakeDevServerRepository{byID: map[string]domain.DevServer{"ds1": ds}}
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

	uc := NewPollFleetHealth(devRepo, health, agent, lock, events, webhook, discardLogger())
	uc.pollOnce(context.Background())

	if len(health.upserted) != 1 || health.upserted[0].Status != domain.HealthStatusDegraded {
		t.Fatalf("expected 1 upsert with status=degraded, got %+v", health.upserted)
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
	ds, _ := domain.NewDevServer("ds1", "t1", "10.0.0.1", domain.ConnectionModeRelayWebSocket, "")
	devRepo := &fakeDevServerRepository{byID: map[string]domain.DevServer{"ds1": ds}}
	health := &fakeFleetHealthWriter{previous: map[string]domain.DevServerHealth{
		"ds1": {DevServerID: "ds1", Status: domain.HealthStatusHealthy},
	}}
	agent := &fakeDevServerAgentClient{healthy: true, execResult: healthyExecResult()}
	lock := &fakePollLock{}
	events := &fakeHealthEventPublisher{}
	webhook := &fakeWebhookAlerter{}

	uc := NewPollFleetHealth(devRepo, health, agent, lock, events, webhook, discardLogger())
	uc.pollOnce(context.Background())

	if len(events.calls) != 0 || len(webhook.calls) != 0 {
		t.Errorf("expected no status-change calls when status is unchanged, got events=%d webhook=%d", len(events.calls), len(webhook.calls))
	}
}

func TestPollFleetHealth_LockedFalseSkipsAgentCallsEntirely(t *testing.T) {
	ds, _ := domain.NewDevServer("ds1", "t1", "10.0.0.1", domain.ConnectionModeRelayWebSocket, "")
	devRepo := &fakeDevServerRepository{byID: map[string]domain.DevServer{"ds1": ds}}
	health := &fakeFleetHealthWriter{previous: map[string]domain.DevServerHealth{}}
	agent := &fakeDevServerAgentClient{healthy: true, execResult: healthyExecResult()}
	lock := &fakePollLock{denyList: map[string]bool{"ds1": true}}
	events := &fakeHealthEventPublisher{}
	webhook := &fakeWebhookAlerter{}

	uc := NewPollFleetHealth(devRepo, health, agent, lock, events, webhook, discardLogger())
	uc.pollOnce(context.Background())

	if len(health.upserted) != 0 {
		t.Errorf("expected zero UpsertFleetHealth calls when TryLock returns locked=false, got %d", len(health.upserted))
	}
	agent.mu.Lock()
	execCalls := len(agent.execCalls)
	agent.mu.Unlock()
	if execCalls != 0 {
		t.Errorf("expected zero agent.Exec calls when TryLock returns locked=false, got %d", execCalls)
	}
}

func TestPollFleetHealth_UnreachableServerRecordsUnreachableStatus(t *testing.T) {
	ds, _ := domain.NewDevServer("ds1", "t1", "10.0.0.1", domain.ConnectionModeRelayWebSocket, "")
	devRepo := &fakeDevServerRepository{byID: map[string]domain.DevServer{"ds1": ds}}
	health := &fakeFleetHealthWriter{previous: map[string]domain.DevServerHealth{}}
	agent := &fakeDevServerAgentClient{healthy: false, healthErr: errors.New("connection refused")}
	lock := &fakePollLock{}
	events := &fakeHealthEventPublisher{}
	webhook := &fakeWebhookAlerter{}

	uc := NewPollFleetHealth(devRepo, health, agent, lock, events, webhook, discardLogger())
	uc.pollOnce(context.Background())

	if len(health.upserted) != 1 || health.upserted[0].Status != domain.HealthStatusUnreachable {
		t.Fatalf("expected 1 upsert with status=unreachable, got %+v", health.upserted)
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
