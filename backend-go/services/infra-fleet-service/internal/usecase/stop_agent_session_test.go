package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

func TestStopAgentSession_SendsGracefulInterrupt_NotShellSignal(t *testing.T) {
	ds, err := domain.NewDevServer("ds1", "tenant-1", "10.0.0.5", domain.ConnectionModeRelayWebSocket, "")
	if err != nil {
		t.Fatalf("building dev server: %v", err)
	}
	resolver := &fakeConnectionResolver{byConnectionID: map[string]domain.DevServer{"conn-1": ds}}
	agent := &fakeDevServerAgentClient{}
	sessions := &fakeAgentSessionRepository{byID: map[string]domain.AgentSession{
		"sess-1": {ID: "sess-1", TenantID: "tenant-1", PtyID: "agent-pty-1", ConnectionID: "conn-1", StartedAt: time.Now(), LastActiveAt: time.Now()},
	}}
	uc := NewStopAgentSession(sessions, resolver, agent)

	ctx := withTenant(context.Background(), "tenant-1")
	if err := uc.Execute(ctx, "sess-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(agent.sendAgentInputCalls) != 1 || agent.sendAgentInputCalls[0] != "agent-pty-1:\x03" {
		t.Fatalf("expected SendAgentInput({0x03}) against agent-pty-1, got %v", agent.sendAgentInputCalls)
	}
	if len(agent.sendSignalCalls) != 0 || len(agent.killPtyCalls) != 0 {
		t.Error("expected the shell-PTY methods (SendSignal/KillPty) to never be called for an agent session")
	}
}

func TestStopAgentSession_UnknownSession_ReturnsNotFound(t *testing.T) {
	uc := NewStopAgentSession(&fakeAgentSessionRepository{}, &fakeConnectionResolver{}, &fakeDevServerAgentClient{})
	ctx := withTenant(context.Background(), "tenant-1")
	if err := uc.Execute(ctx, "missing"); err == nil {
		t.Fatal("expected an error for an unknown session")
	}
}
