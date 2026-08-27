package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

func TestStartAgentSession_ResolvedConnection_SpawnsAndPersists(t *testing.T) {
	ds, err := domain.NewDevServer("ds1", "tenant-1", "10.0.0.5", domain.ConnectionModeRelayWebSocket, "")
	if err != nil {
		t.Fatalf("building dev server: %v", err)
	}
	resolver := &fakeConnectionResolver{byConnectionID: map[string]domain.DevServer{"conn-1": ds}}
	agent := &fakeDevServerAgentClient{spawnAgentResult: SpawnAgentResult{PtyID: "agent-pty-1"}}
	sessions := &fakeAgentSessionRepository{}
	uc := NewStartAgentSession(resolver, agent, sessions, nil, nil)

	ctx := withTenant(context.Background(), "tenant-1")
	session, err := uc.Execute(ctx, StartAgentSessionInput{
		ConnectionID: "conn-1", WorktreeID: "wt-1", UserID: "user-1", Cwd: "/repo", ModelID: "claude", Cols: 80, Rows: 24,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if session.PtyID != "agent-pty-1" {
		t.Errorf("expected PtyID %q, got %q", "agent-pty-1", session.PtyID)
	}
	if session.Status != domain.AgentStatusSpawning {
		t.Errorf("expected status %q, got %q", domain.AgentStatusSpawning, session.Status)
	}
	if len(agent.spawnAgentCalls) != 1 {
		t.Fatalf("expected exactly one SpawnAgent call, got %d", len(agent.spawnAgentCalls))
	}
	if agent.spawnAgentCalls[0].UserID != "user-1" || agent.spawnAgentCalls[0].ModelID != "claude" {
		t.Errorf("unexpected SpawnAgent params: %+v", agent.spawnAgentCalls[0])
	}
	if len(sessions.createCalls) != 1 {
		t.Fatalf("expected exactly one Create call, got %d", len(sessions.createCalls))
	}
}

func TestStartAgentSession_AlreadyRunning_KillsOrphanedAgentAndReturnsTypedError(t *testing.T) {
	ds, err := domain.NewDevServer("ds1", "tenant-1", "10.0.0.5", domain.ConnectionModeRelayWebSocket, "")
	if err != nil {
		t.Fatalf("building dev server: %v", err)
	}
	resolver := &fakeConnectionResolver{byConnectionID: map[string]domain.DevServer{"conn-1": ds}}
	agent := &fakeDevServerAgentClient{spawnAgentResult: SpawnAgentResult{PtyID: "agent-pty-1"}}
	sessions := &fakeAgentSessionRepository{createErr: domain.ErrAgentAlreadyRunning}
	uc := NewStartAgentSession(resolver, agent, sessions, nil, nil)

	ctx := withTenant(context.Background(), "tenant-1")
	_, err = uc.Execute(ctx, StartAgentSessionInput{ConnectionID: "conn-1", WorktreeID: "wt-1", UserID: "user-1"})
	if err == nil {
		t.Fatal("expected an error")
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Code != "INFRA_AGENT_ALREADY_RUNNING" {
		t.Fatalf("expected INFRA_AGENT_ALREADY_RUNNING, got %v", err)
	}
	if len(agent.killAgentCalls) != 1 || agent.killAgentCalls[0] != "agent-pty-1:SIGKILL" {
		t.Errorf("expected the orphaned agent to be killed, got calls: %v", agent.killAgentCalls)
	}
}

func TestStartAgentSession_CredentialInjectionUnavailable(t *testing.T) {
	ds, err := domain.NewDevServer("ds1", "tenant-1", "10.0.0.5", domain.ConnectionModeRelayWebSocket, "")
	if err != nil {
		t.Fatalf("building dev server: %v", err)
	}
	resolver := &fakeConnectionResolver{byConnectionID: map[string]domain.DevServer{"conn-1": ds}}
	agent := &fakeDevServerAgentClient{spawnAgentErr: errors.New("agent.spawn: no plaintext resolvedApiKey was provided for accountId=acc-1")}
	sessions := &fakeAgentSessionRepository{}
	uc := NewStartAgentSession(resolver, agent, sessions, nil, nil)

	ctx := withTenant(context.Background(), "tenant-1")
	_, err = uc.Execute(ctx, StartAgentSessionInput{ConnectionID: "conn-1", WorktreeID: "wt-1", UserID: "user-1", AccountID: "acc-1"})
	if err == nil {
		t.Fatal("expected an error")
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Code != "INFRA_AGENT_CREDENTIAL_INJECTION_UNAVAILABLE" {
		t.Fatalf("expected INFRA_AGENT_CREDENTIAL_INJECTION_UNAVAILABLE, got %v", err)
	}
	if len(sessions.createCalls) != 0 {
		t.Error("expected no persisted session when spawn fails")
	}
}

func TestStartAgentSession_UnresolvedConnection_ReturnsNotFoundError(t *testing.T) {
	resolver := &fakeConnectionResolver{byConnectionID: map[string]domain.DevServer{}}
	agent := &fakeDevServerAgentClient{}
	sessions := &fakeAgentSessionRepository{}
	uc := NewStartAgentSession(resolver, agent, sessions, nil, nil)

	ctx := withTenant(context.Background(), "tenant-1")
	_, err := uc.Execute(ctx, StartAgentSessionInput{ConnectionID: "unknown-conn"})
	if err == nil {
		t.Fatal("expected an error when the connectionId doesn't resolve")
	}
	if len(agent.spawnAgentCalls) != 0 {
		t.Error("expected no agent.spawn call when the connectionId doesn't resolve")
	}
}

// TestStartAgentSession_CallsEnsureHookConsumerAfterResolvingConnection
// guards TASK-AG-03-06's wiring: ensureHookConsumer must fire exactly once,
// synchronously, right after ResolveConnection succeeds — the call site
// ResumeAgentSession's delegated Execute also goes through, so this one
// test covers both entry points.
func TestStartAgentSession_CallsEnsureHookConsumerAfterResolvingConnection(t *testing.T) {
	ds, err := domain.NewDevServer("ds1", "tenant-1", "10.0.0.5", domain.ConnectionModeRelayWebSocket, "")
	if err != nil {
		t.Fatalf("building dev server: %v", err)
	}
	resolver := &fakeConnectionResolver{byConnectionID: map[string]domain.DevServer{"conn-1": ds}}
	agent := &fakeDevServerAgentClient{spawnAgentResult: SpawnAgentResult{PtyID: "agent-pty-1"}}
	sessions := &fakeAgentSessionRepository{}

	var calls int
	var gotTenantID string
	var gotDevServer domain.DevServer
	ensureHook := func(_ context.Context, tenantID string, devServer domain.DevServer) {
		calls++
		gotTenantID = tenantID
		gotDevServer = devServer
	}
	uc := NewStartAgentSession(resolver, agent, sessions, nil, ensureHook)

	ctx := withTenant(context.Background(), "tenant-1")
	if _, err := uc.Execute(ctx, StartAgentSessionInput{ConnectionID: "conn-1", WorktreeID: "wt-1", UserID: "user-1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected ensureHookConsumer to be called exactly once, got %d", calls)
	}
	if gotTenantID != "tenant-1" || gotDevServer.ID != "ds1" {
		t.Errorf("unexpected ensureHookConsumer args: tenantID=%q devServer=%+v", gotTenantID, gotDevServer)
	}
}

// TestStartAgentSession_LaunchesClassifierAfterPersist guards
// TASK-AG-05-06's wiring: a non-nil classifier must be started as a
// goroutine (AgentOutputClassifier.Run) right after Create persists —
// observed here via the fake agent's StreamPty subscribe/unsubscribe pair,
// since Run's first call is exactly that subscription.
func TestStartAgentSession_LaunchesClassifierAfterPersist(t *testing.T) {
	ds, err := domain.NewDevServer("ds1", "tenant-1", "10.0.0.5", domain.ConnectionModeRelayWebSocket, "")
	if err != nil {
		t.Fatalf("building dev server: %v", err)
	}
	resolver := &fakeConnectionResolver{byConnectionID: map[string]domain.DevServer{"conn-1": ds}}
	events := make(chan PtyEvent)
	close(events) // classifier.Run's for-range returns immediately — no exit event needed to observe the subscribe/unsubscribe pair
	agent := &fakeDevServerAgentClient{spawnAgentResult: SpawnAgentResult{PtyID: "agent-pty-1"}, streamPtyEvents: events}
	sessions := &fakeAgentSessionRepository{}
	publisher := &fakeAgentStatusEventPublisher{}
	kill := NewKillAgentSession(sessions, resolver, agent, nil)
	classifier := NewAgentOutputClassifier(sessions, agent, publisher, kill)
	classifier.startupTimeout = time.Hour // don't fire during this test

	uc := NewStartAgentSession(resolver, agent, sessions, classifier, nil)

	ctx := withTenant(context.Background(), "tenant-1")
	if _, err := uc.Execute(ctx, StartAgentSessionInput{ConnectionID: "conn-1", WorktreeID: "wt-1", UserID: "user-1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for {
		agent.mu.Lock()
		unsubscribed := agent.streamPtyUnsubscribed
		agent.mu.Unlock()
		if unsubscribed {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for the classifier goroutine to subscribe to and unsubscribe from StreamPty")
		}
		time.Sleep(5 * time.Millisecond)
	}
}
