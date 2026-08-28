package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

func TestSpawnTerminalSession_RequiresTenantContext(t *testing.T) {
	uc := NewSpawnTerminalSession(&fakeConnectionResolver{}, &fakeDevServerAgentClient{}, &fakeTerminalSessionRepository{}, false)
	_, err := uc.Execute(context.Background(), SpawnTerminalSessionInput{ConnectionID: "conn-1"})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestSpawnTerminalSession_HostLocal_RejectedInServerDeploymentMode(t *testing.T) {
	uc := NewSpawnTerminalSession(&fakeConnectionResolver{}, &fakeDevServerAgentClient{}, &fakeTerminalSessionRepository{}, true)
	ctx := withTenant(context.Background(), "tenant-1")
	_, err := uc.Execute(ctx, SpawnTerminalSessionInput{})
	if err == nil {
		t.Fatal("expected an error for a host-local spawn in server-deployment mode")
	}
}

func TestSpawnTerminalSession_HostLocal_UnimplementedOutsideServerDeploymentMode(t *testing.T) {
	uc := NewSpawnTerminalSession(&fakeConnectionResolver{}, &fakeDevServerAgentClient{}, &fakeTerminalSessionRepository{}, false)
	ctx := withTenant(context.Background(), "tenant-1")
	_, err := uc.Execute(ctx, SpawnTerminalSessionInput{})
	if err == nil {
		t.Fatal("expected an error for a host-local spawn — no local pty adapter exists in this service")
	}
}

func TestSpawnTerminalSession_UnresolvedConnection_ReturnsNotFoundError(t *testing.T) {
	resolver := &fakeConnectionResolver{byConnectionID: map[string]domain.DevServer{}}
	agent := &fakeDevServerAgentClient{}
	sessions := &fakeTerminalSessionRepository{}
	uc := NewSpawnTerminalSession(resolver, agent, sessions, false)

	ctx := withTenant(context.Background(), "tenant-1")
	_, err := uc.Execute(ctx, SpawnTerminalSessionInput{ConnectionID: "unknown-conn"})
	if err == nil {
		t.Fatal("expected an error when the connectionId doesn't resolve")
	}
	if len(agent.spawnPtyCalls) != 0 {
		t.Error("expected no pty.create call when the connectionId doesn't resolve")
	}
}

func TestSpawnTerminalSession_ResolvedConnection_SpawnsAndPersists(t *testing.T) {
	ds, err := domain.NewDevServer("ds1", "tenant-1", "10.0.0.5", domain.ConnectionModeRelayWebSocket, "", nil)
	if err != nil {
		t.Fatalf("building dev server: %v", err)
	}
	resolver := &fakeConnectionResolver{byConnectionID: map[string]domain.DevServer{"conn-1": ds}}
	agent := &fakeDevServerAgentClient{spawnPtyResult: SpawnPtyResult{PtyID: "pty-abc", Cwd: "/work", Cols: 80, Rows: 24, Shell: "/bin/bash"}}
	sessions := &fakeTerminalSessionRepository{}
	uc := NewSpawnTerminalSession(resolver, agent, sessions, false)

	ctx := withTenant(context.Background(), "tenant-1")
	session, err := uc.Execute(ctx, SpawnTerminalSessionInput{ConnectionID: "conn-1", Cwd: "/repo", Cols: 80, Rows: 24})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if session.PtyID != "pty-abc" {
		t.Errorf("expected PtyID %q, got %q", "pty-abc", session.PtyID)
	}
	if session.TenantID != "tenant-1" {
		t.Errorf("expected TenantID %q, got %q", "tenant-1", session.TenantID)
	}
	if session.ConnectionID != "conn-1" {
		t.Errorf("expected ConnectionID %q, got %q", "conn-1", session.ConnectionID)
	}
	if session.Cwd != "/work" {
		t.Errorf("expected agent's effective Cwd %q to win, got %q", "/work", session.Cwd)
	}
	if len(sessions.createCalls) != 1 {
		t.Fatalf("expected exactly one Create call, got %d", len(sessions.createCalls))
	}
}

// TestSpawnTerminalSession_ShellIntegration_ReachesSpawnPtyInputUnmodified is
// TASK-TM-04-06's regression guard: coordination decides whether (the
// boolean), execution decides how — this usecase must forward
// ShellIntegration unexamined.
func TestSpawnTerminalSession_ShellIntegration_ReachesSpawnPtyInputUnmodified(t *testing.T) {
	ds, err := domain.NewDevServer("ds1", "tenant-1", "10.0.0.5", domain.ConnectionModeRelayWebSocket, "", nil)
	if err != nil {
		t.Fatalf("building dev server: %v", err)
	}
	resolver := &fakeConnectionResolver{byConnectionID: map[string]domain.DevServer{"conn-1": ds}}
	agent := &fakeDevServerAgentClient{spawnPtyResult: SpawnPtyResult{PtyID: "pty-abc"}}
	sessions := &fakeTerminalSessionRepository{}
	uc := NewSpawnTerminalSession(resolver, agent, sessions, false)

	ctx := withTenant(context.Background(), "tenant-1")
	if _, err := uc.Execute(ctx, SpawnTerminalSessionInput{ConnectionID: "conn-1", ShellIntegration: true}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(agent.spawnPtyCalls) != 1 {
		t.Fatalf("expected exactly one SpawnPty call, got %d", len(agent.spawnPtyCalls))
	}
	if !agent.spawnPtyCalls[0].ShellIntegration {
		t.Error("expected ShellIntegration=true to reach SpawnPtyInput unmodified")
	}
}

// TestSpawnTerminalSession_ShellIntegration_DefaultsFalse confirms existing
// callers that never set ShellIntegration keep seeing false — no behavior
// change for callers unaware of BR-TM-13.
func TestSpawnTerminalSession_ShellIntegration_DefaultsFalse(t *testing.T) {
	ds, err := domain.NewDevServer("ds1", "tenant-1", "10.0.0.5", domain.ConnectionModeRelayWebSocket, "", nil)
	if err != nil {
		t.Fatalf("building dev server: %v", err)
	}
	resolver := &fakeConnectionResolver{byConnectionID: map[string]domain.DevServer{"conn-1": ds}}
	agent := &fakeDevServerAgentClient{spawnPtyResult: SpawnPtyResult{PtyID: "pty-abc"}}
	sessions := &fakeTerminalSessionRepository{}
	uc := NewSpawnTerminalSession(resolver, agent, sessions, false)

	ctx := withTenant(context.Background(), "tenant-1")
	if _, err := uc.Execute(ctx, SpawnTerminalSessionInput{ConnectionID: "conn-1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(agent.spawnPtyCalls) != 1 {
		t.Fatalf("expected exactly one SpawnPty call, got %d", len(agent.spawnPtyCalls))
	}
	if agent.spawnPtyCalls[0].ShellIntegration {
		t.Error("expected ShellIntegration to default to false when unset")
	}
}

func TestSpawnTerminalSession_AgentFailurePropagates(t *testing.T) {
	ds, err := domain.NewDevServer("ds1", "tenant-1", "10.0.0.5", domain.ConnectionModeRelayWebSocket, "", nil)
	if err != nil {
		t.Fatalf("building dev server: %v", err)
	}
	resolver := &fakeConnectionResolver{byConnectionID: map[string]domain.DevServer{"conn-1": ds}}
	agent := &fakeDevServerAgentClient{spawnPtyErr: errors.New("devserveragent: not connected")}
	sessions := &fakeTerminalSessionRepository{}
	uc := NewSpawnTerminalSession(resolver, agent, sessions, false)

	ctx := withTenant(context.Background(), "tenant-1")
	_, err = uc.Execute(ctx, SpawnTerminalSessionInput{ConnectionID: "conn-1"})
	if err == nil {
		t.Fatal("expected the agent's error to propagate")
	}
	if len(sessions.createCalls) != 0 {
		t.Error("expected no persisted session when the agent call fails")
	}
}
