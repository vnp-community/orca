package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

func newResumeFixture(prior domain.AgentSession) (*fakeConnectionResolver, *fakeAgentSessionRepository, *fakeDevServerAgentClient) {
	ds, _ := domain.NewDevServer("ds1", "tenant-1", "10.0.0.5", domain.ConnectionModeRelayWebSocket, "")
	resolver := &fakeConnectionResolver{byConnectionID: map[string]domain.DevServer{"conn-1": ds}}
	sessions := &fakeAgentSessionRepository{byID: map[string]domain.AgentSession{prior.ID: prior}}
	agent := &fakeDevServerAgentClient{spawnAgentResult: SpawnAgentResult{PtyID: "agent-pty-2"}}
	return resolver, sessions, agent
}

func TestResumeAgentSession_NoPriorSession_ReturnsNotFound(t *testing.T) {
	resolver, sessions, agent := newResumeFixture(domain.AgentSession{})
	sessions.byID = map[string]domain.AgentSession{} // wipe the fixture's zero-value entry
	uc := NewResumeAgentSession(sessions, resolver, NewStartAgentSession(resolver, agent, sessions, nil, nil))

	ctx := withTenant(context.Background(), "tenant-1")
	_, err := uc.Execute(ctx, ResumeAgentSessionInput{ConnectionID: "conn-1", WorktreeID: "wt-1", UserID: "user-1"})
	if err == nil {
		t.Fatal("expected an error")
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Code != "INFRA_AGENT_SESSION_NOT_FOUND" {
		t.Fatalf("expected INFRA_AGENT_SESSION_NOT_FOUND, got %v", err)
	}
}

func TestResumeAgentSession_Expired_ReturnsExpiredError(t *testing.T) {
	prior := domain.AgentSession{
		ID: "sess-1", TenantID: "tenant-1", WorktreeID: "wt-1", ModelID: "claude",
		ResumeProviderSessionID: "provider-sess-1",
		StartedAt:               time.Now().Add(-10 * 24 * time.Hour),
		LastActiveAt:            time.Now().Add(-8 * 24 * time.Hour), // > 7 days ago
	}
	resolver, sessions, agent := newResumeFixture(prior)
	start := NewStartAgentSession(resolver, agent, sessions, nil, nil)
	uc := NewResumeAgentSession(sessions, resolver, start)

	ctx := withTenant(context.Background(), "tenant-1")
	_, err := uc.Execute(ctx, ResumeAgentSessionInput{ConnectionID: "conn-1", WorktreeID: "wt-1", UserID: "user-1"})
	if err == nil {
		t.Fatal("expected an error")
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Code != "INFRA_AGENT_SESSION_EXPIRED" {
		t.Fatalf("expected INFRA_AGENT_SESSION_EXPIRED, got %v", err)
	}
	if len(agent.spawnAgentCalls) != 0 {
		t.Error("expected start.Execute to never be called for an expired session")
	}
}

func TestResumeAgentSession_NoResumableProviderSessionID(t *testing.T) {
	prior := domain.AgentSession{
		ID: "sess-1", TenantID: "tenant-1", WorktreeID: "wt-1", ModelID: "claude",
		StartedAt: time.Now(), LastActiveAt: time.Now(), // recent, but no ResumeProviderSessionID
	}
	resolver, sessions, agent := newResumeFixture(prior)
	uc := NewResumeAgentSession(sessions, resolver, NewStartAgentSession(resolver, agent, sessions, nil, nil))

	ctx := withTenant(context.Background(), "tenant-1")
	_, err := uc.Execute(ctx, ResumeAgentSessionInput{ConnectionID: "conn-1", WorktreeID: "wt-1", UserID: "user-1"})
	if err == nil {
		t.Fatal("expected an error")
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Code != "INFRA_AGENT_NO_RESUMABLE_SESSION" {
		t.Fatalf("expected INFRA_AGENT_NO_RESUMABLE_SESSION, got %v", err)
	}
}

func TestResumeAgentSession_AgentVersionMismatch(t *testing.T) {
	prior := domain.AgentSession{
		ID: "sess-1", TenantID: "tenant-1", WorktreeID: "wt-1", ModelID: "claude",
		ResumeProviderSessionID: "provider-sess-1", AgentVersion: "1.0.0",
		StartedAt: time.Now(), LastActiveAt: time.Now(),
	}
	ds, _ := domain.NewDevServer("ds1", "tenant-1", "10.0.0.5", domain.ConnectionModeRelayWebSocket, "")
	ds.AgentVersion = "2.0.0"
	resolver := &fakeConnectionResolver{byConnectionID: map[string]domain.DevServer{"conn-1": ds}}
	sessions := &fakeAgentSessionRepository{byID: map[string]domain.AgentSession{prior.ID: prior}}
	agent := &fakeDevServerAgentClient{spawnAgentResult: SpawnAgentResult{PtyID: "agent-pty-2"}}
	uc := NewResumeAgentSession(sessions, resolver, NewStartAgentSession(resolver, agent, sessions, nil, nil))

	ctx := withTenant(context.Background(), "tenant-1")
	_, err := uc.Execute(ctx, ResumeAgentSessionInput{ConnectionID: "conn-1", WorktreeID: "wt-1", UserID: "user-1"})
	if err == nil {
		t.Fatal("expected an error")
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Code != "INFRA_AGENT_VERSION_MISMATCH" {
		t.Fatalf("expected INFRA_AGENT_VERSION_MISMATCH, got %v", err)
	}
}

func TestResumeAgentSession_HappyPath_UsesProviderSessionIDNotRowID(t *testing.T) {
	prior := domain.AgentSession{
		ID: "sess-1", TenantID: "tenant-1", WorktreeID: "wt-1", ModelID: "claude", AccountID: "acc-1",
		ResumeProviderSessionID: "provider-sess-1",
		StartedAt:               time.Now(), LastActiveAt: time.Now(),
	}
	resolver, sessions, agent := newResumeFixture(prior)
	uc := NewResumeAgentSession(sessions, resolver, NewStartAgentSession(resolver, agent, sessions, nil, nil))

	ctx := withTenant(context.Background(), "tenant-1")
	session, err := uc.Execute(ctx, ResumeAgentSessionInput{ConnectionID: "conn-1", WorktreeID: "wt-1", UserID: "user-1", Cwd: "/repo"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if session.PtyID != "agent-pty-2" {
		t.Errorf("expected the resumed session's PtyID to be %q, got %q", "agent-pty-2", session.PtyID)
	}
	if len(agent.spawnAgentCalls) != 1 {
		t.Fatalf("expected exactly one SpawnAgent call, got %d", len(agent.spawnAgentCalls))
	}
	if agent.spawnAgentCalls[0].ResumeID != "provider-sess-1" {
		t.Errorf("expected ResumeID to be the CLI's own provider session id %q, got %q", "provider-sess-1", agent.spawnAgentCalls[0].ResumeID)
	}
}
