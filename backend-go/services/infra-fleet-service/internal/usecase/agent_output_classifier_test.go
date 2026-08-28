package usecase

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// fakeAgentStatusEventPublisher implements AgentStatusEventPublisher for tests.
type fakeAgentStatusEventPublisher struct {
	mu sync.Mutex

	statusChangedCalls []domain.AgentStatus
	rateLimitedCalls   int
}

func (f *fakeAgentStatusEventPublisher) PublishStatusChanged(ctx context.Context, tenantID, sessionID string, status domain.AgentStatus) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.statusChangedCalls = append(f.statusChangedCalls, status)
	return nil
}

func (f *fakeAgentStatusEventPublisher) PublishRateLimited(ctx context.Context, tenantID, sessionID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rateLimitedCalls++
	return nil
}

func (f *fakeAgentStatusEventPublisher) snapshotStatusChanged() []domain.AgentStatus {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]domain.AgentStatus(nil), f.statusChangedCalls...)
}

func (f *fakeAgentStatusEventPublisher) snapshotRateLimited() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.rateLimitedCalls
}

func TestAgentOutputClassifier_ClaudeFreshSpawn_NoStatusChangeFromTrack2Bytes(t *testing.T) {
	sessions := &fakeAgentSessionRepository{byID: map[string]domain.AgentSession{
		"sess-1": {ID: "sess-1", TenantID: "tenant-1", PtyID: "pty-1", ModelID: "claude", Status: domain.AgentStatusSpawning, StartedAt: time.Now(), LastActiveAt: time.Now()},
	}}
	events := make(chan PtyEvent, 4)
	agent := &fakeDevServerAgentClient{streamPtyEvents: events}
	publisher := &fakeAgentStatusEventPublisher{}
	kill := NewKillAgentSession(sessions, &fakeConnectionResolver{}, agent, nil)
	classifier := NewAgentOutputClassifier(sessions, agent, publisher, kill)
	classifier.startupTimeout = time.Hour // don't fire during this test

	session := domain.AgentSession{ID: "sess-1", ModelID: "claude", Status: domain.AgentStatusSpawning} // UsesStreamJSON() == true (fresh spawn)
	ds, _ := domain.NewDevServer("ds1", "tenant-1", "10.0.0.5", domain.ConnectionModeRelayWebSocket, "", nil)

	done := make(chan struct{})
	go func() {
		classifier.Run(context.Background(), "tenant-1", session, ds)
		close(done)
	}()

	// TODO: once classifyStreamJSONLine does real parsing, this must be
	// re-verified — right now it's a stub always returning ("", false), so
	// track 1 being "exclusive when active" holds trivially.
	events <- PtyEvent{PtyID: "pty-1", Data: []byte("\x1b]133;C\x07")}
	events <- PtyEvent{PtyID: "pty-1", Data: []byte("\x1b]133;D;0\x07")}
	time.Sleep(50 * time.Millisecond)
	close(events)
	<-done

	if len(publisher.snapshotStatusChanged()) != 0 {
		t.Errorf("expected no status change for a Claude fresh-spawn session fed track-2 bytes, got %v", publisher.snapshotStatusChanged())
	}
}

func TestAgentOutputClassifier_RateLimitString_PublishesRateLimitedNotStatus(t *testing.T) {
	sessions := &fakeAgentSessionRepository{byID: map[string]domain.AgentSession{
		"sess-1": {ID: "sess-1", TenantID: "tenant-1", PtyID: "pty-1", ModelID: "claude", ResumeOfSessionID: "prior", Status: domain.AgentStatusRunning, StartedAt: time.Now(), LastActiveAt: time.Now()},
	}}
	events := make(chan PtyEvent, 4)
	agent := &fakeDevServerAgentClient{streamPtyEvents: events}
	publisher := &fakeAgentStatusEventPublisher{}
	kill := NewKillAgentSession(sessions, &fakeConnectionResolver{}, agent, nil)
	classifier := NewAgentOutputClassifier(sessions, agent, publisher, kill)
	classifier.startupTimeout = time.Hour

	// ResumeOfSessionID != "" => UsesStreamJSON() == false, so this exercises track 2.
	session := domain.AgentSession{ID: "sess-1", ModelID: "claude", ResumeOfSessionID: "prior", Status: domain.AgentStatusRunning}
	ds, _ := domain.NewDevServer("ds1", "tenant-1", "10.0.0.5", domain.ConnectionModeRelayWebSocket, "", nil)

	done := make(chan struct{})
	go func() {
		classifier.Run(context.Background(), "tenant-1", session, ds)
		close(done)
	}()

	events <- PtyEvent{PtyID: "pty-1", Data: []byte("Error: rate limit exceeded")}
	time.Sleep(50 * time.Millisecond)
	close(events)
	<-done

	if publisher.snapshotRateLimited() != 1 {
		t.Errorf("expected exactly one PublishRateLimited call, got %d", publisher.snapshotRateLimited())
	}
	if len(sessions.updateStatusCalls) != 0 {
		t.Errorf("expected UpdateStatus to never be called for a rate-limited chunk, got %v", sessions.updateStatusCalls)
	}
}

func TestAgentOutputClassifier_StartupTimeout_KillsAndMarksError(t *testing.T) {
	sessions := &fakeAgentSessionRepository{byID: map[string]domain.AgentSession{
		"sess-1": {ID: "sess-1", TenantID: "tenant-1", PtyID: "pty-1", ConnectionID: "conn-1", ModelID: "codex", Status: domain.AgentStatusSpawning, StartedAt: time.Now(), LastActiveAt: time.Now()},
	}}
	events := make(chan PtyEvent)
	ds, _ := domain.NewDevServer("ds1", "tenant-1", "10.0.0.5", domain.ConnectionModeRelayWebSocket, "", nil)
	resolver := &fakeConnectionResolver{byConnectionID: map[string]domain.DevServer{"conn-1": ds}}
	agent := &fakeDevServerAgentClient{streamPtyEvents: events}
	publisher := &fakeAgentStatusEventPublisher{}
	kill := NewKillAgentSession(sessions, resolver, agent, nil)
	classifier := NewAgentOutputClassifier(sessions, agent, publisher, kill)
	classifier.startupTimeout = 20 * time.Millisecond // short, for the test

	session := domain.AgentSession{ID: "sess-1", ConnectionID: "conn-1", ModelID: "codex", Status: domain.AgentStatusSpawning}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	go classifier.Run(ctx, "tenant-1", session, ds)

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if len(agent.killAgentCalls) > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if len(agent.killAgentCalls) == 0 {
		t.Fatal("expected KillAgentSession.Execute to be invoked after the startup timeout")
	}
	if len(publisher.snapshotStatusChanged()) == 0 || publisher.snapshotStatusChanged()[len(publisher.snapshotStatusChanged())-1] != domain.AgentStatusError {
		t.Errorf("expected the final published status to be 'error', got %v", publisher.snapshotStatusChanged())
	}
}

func TestAgentOutputClassifier_ExitEvent_MarksStoppedOrError(t *testing.T) {
	for _, tc := range []struct {
		name     string
		exitCode int32
		want     domain.AgentStatus
	}{
		{"clean exit", 0, domain.AgentStatusStopped},
		{"nonzero exit", 1, domain.AgentStatusError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sessions := &fakeAgentSessionRepository{byID: map[string]domain.AgentSession{
				"sess-1": {ID: "sess-1", TenantID: "tenant-1", PtyID: "pty-1", ModelID: "codex", Status: domain.AgentStatusRunning, StartedAt: time.Now(), LastActiveAt: time.Now()},
			}}
			events := make(chan PtyEvent, 1)
			agent := &fakeDevServerAgentClient{streamPtyEvents: events}
			publisher := &fakeAgentStatusEventPublisher{}
			kill := NewKillAgentSession(sessions, &fakeConnectionResolver{}, agent, nil)
			classifier := NewAgentOutputClassifier(sessions, agent, publisher, kill)
			classifier.startupTimeout = time.Hour

			session := domain.AgentSession{ID: "sess-1", ModelID: "codex", Status: domain.AgentStatusRunning}
			ds, _ := domain.NewDevServer("ds1", "tenant-1", "10.0.0.5", domain.ConnectionModeRelayWebSocket, "", nil)

			done := make(chan struct{})
			go func() {
				classifier.Run(context.Background(), "tenant-1", session, ds)
				close(done)
			}()

			events <- PtyEvent{PtyID: "pty-1", Exited: true, ExitCode: tc.exitCode}
			<-done

			if len(sessions.markStoppedWithStatusCall) != 1 || sessions.markStoppedWithStatusCall[0] != tc.want {
				t.Fatalf("expected MarkStoppedWithStatus(%q), got %v", tc.want, sessions.markStoppedWithStatusCall)
			}
		})
	}
}
