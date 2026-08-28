package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

func newSwitchFixture(current domain.AgentSession) (*fakeConnectionResolver, *fakeAgentSessionRepository, *fakeDevServerAgentClient) {
	ds, _ := domain.NewDevServer("ds1", "tenant-1", "10.0.0.5", domain.ConnectionModeRelayWebSocket, "", nil)
	resolver := &fakeConnectionResolver{byConnectionID: map[string]domain.DevServer{"conn-1": ds}}
	sessions := &fakeAgentSessionRepository{byID: map[string]domain.AgentSession{current.ID: current}}
	agent := &fakeDevServerAgentClient{spawnAgentResult: SpawnAgentResult{PtyID: "agent-pty-new"}}
	return resolver, sessions, agent
}

func newSwitchUC(resolver *fakeConnectionResolver, sessions *fakeAgentSessionRepository, agent *fakeDevServerAgentClient, resolveClient *fakeAIProviderResolverClient) *SwitchAgentAccount {
	kill := NewKillAgentSession(sessions, resolver, agent, nil)
	start := NewStartAgentSession(resolver, agent, sessions, nil, nil)
	resume := NewResumeAgentSession(sessions, resolver, start)
	return NewSwitchAgentAccount(sessions, kill, resolveClient, start, resume)
}

func TestSwitchAgentAccount_HappyPath_NoResumableSession_StartsWithDifferentAccount(t *testing.T) {
	current := domain.AgentSession{
		ID: "sess-1", TenantID: "tenant-1", WorktreeID: "wt-1", PtyID: "agent-pty-1", ConnectionID: "conn-1",
		ModelID: "claude", AccountID: "acc-old", StartedAt: time.Now(), LastActiveAt: time.Now(),
	}
	resolver, sessions, agent := newSwitchFixture(current)
	resolveClient := &fakeAIProviderResolverClient{accountID: "acc-new"}
	uc := newSwitchUC(resolver, sessions, agent, resolveClient)

	ctx := withTenant(context.Background(), "tenant-1")
	session, err := uc.Execute(ctx, SwitchAgentAccountInput{ConnectionID: "conn-1", WorktreeID: "wt-1", UserID: "user-1", ProjectID: "proj-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if session.AccountID != "acc-new" {
		t.Errorf("expected the new session's AccountID to be %q, got %q", "acc-new", session.AccountID)
	}
	if len(sessions.markStoppedCalls) != 1 {
		t.Errorf("expected the current session to be killed, got calls: %v", sessions.markStoppedCalls)
	}
	if len(resolveClient.calls) != 1 || resolveClient.calls[0].ExcludeAccountID != "acc-old" {
		t.Errorf("expected ResolveProvider called excluding acc-old, got %+v", resolveClient.calls)
	}
}

func TestSwitchAgentAccount_NoAlternateAccount_ReturnsTypedError(t *testing.T) {
	current := domain.AgentSession{
		ID: "sess-1", TenantID: "tenant-1", WorktreeID: "wt-1", PtyID: "agent-pty-1", ConnectionID: "conn-1",
		ModelID: "claude", AccountID: "acc-old", StartedAt: time.Now(), LastActiveAt: time.Now(),
	}
	resolver, sessions, agent := newSwitchFixture(current)
	resolveClient := &fakeAIProviderResolverClient{accountID: "acc-old"} // same account back — no alternate
	uc := newSwitchUC(resolver, sessions, agent, resolveClient)

	ctx := withTenant(context.Background(), "tenant-1")
	_, err := uc.Execute(ctx, SwitchAgentAccountInput{ConnectionID: "conn-1", WorktreeID: "wt-1", UserID: "user-1"})
	if err == nil {
		t.Fatal("expected an error")
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Code != "INFRA_SWITCH_NO_ALTERNATE_ACCOUNT" {
		t.Fatalf("expected INFRA_SWITCH_NO_ALTERNATE_ACCOUNT, got %v", err)
	}
	if len(agent.spawnAgentCalls) != 0 {
		t.Error("expected start/resume to never be called with no alternate account")
	}
}

func TestSwitchAgentAccount_ResumableSession_ResumeSucceeds(t *testing.T) {
	current := domain.AgentSession{
		ID: "sess-1", TenantID: "tenant-1", WorktreeID: "wt-1", PtyID: "agent-pty-1", ConnectionID: "conn-1",
		ModelID: "claude", AccountID: "acc-old", ResumeProviderSessionID: "provider-sess-1",
		StartedAt: time.Now(), LastActiveAt: time.Now(),
	}
	resolver, sessions, agent := newSwitchFixture(current)
	resolveClient := &fakeAIProviderResolverClient{accountID: "acc-new"}
	uc := newSwitchUC(resolver, sessions, agent, resolveClient)

	ctx := withTenant(context.Background(), "tenant-1")
	session, err := uc.Execute(ctx, SwitchAgentAccountInput{ConnectionID: "conn-1", WorktreeID: "wt-1", UserID: "user-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(agent.spawnAgentCalls) != 1 || agent.spawnAgentCalls[0].ResumeID != "provider-sess-1" {
		t.Fatalf("expected a resume call with ResumeID=provider-sess-1, got %+v", agent.spawnAgentCalls)
	}
	if session.PtyID != "agent-pty-new" {
		t.Errorf("unexpected resulting session: %+v", session)
	}
}

func TestSwitchAgentAccount_KillFails_AbortsBeforeResolve(t *testing.T) {
	current := domain.AgentSession{
		ID: "sess-1", TenantID: "tenant-1", WorktreeID: "wt-1", PtyID: "agent-pty-1", ConnectionID: "conn-1",
		ModelID: "claude", AccountID: "acc-old", StartedAt: time.Now(), LastActiveAt: time.Now(),
	}
	resolver, sessions, _ := newSwitchFixture(current)
	agent := &fakeDevServerAgentClient{killAgentErr: nil} // agent call itself can succeed...
	// ...but force the underlying resolve to fail on ConnectionID mismatch to prove ordering: use a resolver
	// that returns not-connected so resolveAgentSession (inside KillAgentSession) fails.
	resolver.byConnectionID = map[string]domain.DevServer{}
	resolveClient := &fakeAIProviderResolverClient{accountID: "acc-new"}
	uc := newSwitchUC(resolver, sessions, agent, resolveClient)

	ctx := withTenant(context.Background(), "tenant-1")
	_, err := uc.Execute(ctx, SwitchAgentAccountInput{ConnectionID: "conn-1", WorktreeID: "wt-1", UserID: "user-1"})
	if err == nil {
		t.Fatal("expected an error when kill fails")
	}
	if len(resolveClient.calls) != 0 {
		t.Error("expected ResolveProvider to never be called when kill fails first")
	}
}

func TestSwitchAgentAccount_InheritsCredentialInjectionBlocker(t *testing.T) {
	current := domain.AgentSession{
		ID: "sess-1", TenantID: "tenant-1", WorktreeID: "wt-1", PtyID: "agent-pty-1", ConnectionID: "conn-1",
		ModelID: "claude", AccountID: "acc-old", StartedAt: time.Now(), LastActiveAt: time.Now(),
	}
	resolver, sessions, agent := newSwitchFixture(current)
	agent.spawnAgentErr = errors.New("agent.spawn: no plaintext resolvedApiKey was provided for accountId=acc-new")
	resolveClient := &fakeAIProviderResolverClient{accountID: "acc-new"}
	uc := newSwitchUC(resolver, sessions, agent, resolveClient)

	ctx := withTenant(context.Background(), "tenant-1")
	_, err := uc.Execute(ctx, SwitchAgentAccountInput{ConnectionID: "conn-1", WorktreeID: "wt-1", UserID: "user-1"})
	if err == nil {
		t.Fatal("expected an error")
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Code != "INFRA_AGENT_CREDENTIAL_INJECTION_UNAVAILABLE" {
		t.Fatalf("expected INFRA_AGENT_CREDENTIAL_INJECTION_UNAVAILABLE (inherited from StartAgentSession), got %v", err)
	}
}
