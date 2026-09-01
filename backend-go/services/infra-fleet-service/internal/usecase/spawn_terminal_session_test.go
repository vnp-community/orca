package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

func TestSpawnTerminalSession_RequiresTenantContext(t *testing.T) {
	uc := NewSpawnTerminalSession(&fakeConnectionResolver{}, &fakeDevServerRepository{}, &fakeDevServerAgentClient{}, &fakeTerminalSessionRepository{}, false)
	_, err := uc.Execute(context.Background(), SpawnTerminalSessionInput{ConnectionID: "conn-1"})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestSpawnTerminalSession_HostLocal_RejectedInServerDeploymentMode(t *testing.T) {
	uc := NewSpawnTerminalSession(&fakeConnectionResolver{}, &fakeDevServerRepository{}, &fakeDevServerAgentClient{}, &fakeTerminalSessionRepository{}, true)
	ctx := withTenant(context.Background(), "tenant-1")
	_, err := uc.Execute(ctx, SpawnTerminalSessionInput{})
	if err == nil {
		t.Fatal("expected an error for a host-local spawn in server-deployment mode")
	}
}

func TestSpawnTerminalSession_HostLocal_UnimplementedOutsideServerDeploymentMode(t *testing.T) {
	uc := NewSpawnTerminalSession(&fakeConnectionResolver{}, &fakeDevServerRepository{}, &fakeDevServerAgentClient{}, &fakeTerminalSessionRepository{}, false)
	ctx := withTenant(context.Background(), "tenant-1")
	_, err := uc.Execute(ctx, SpawnTerminalSessionInput{})
	if err == nil {
		t.Fatal("expected an error for a host-local spawn — no local pty adapter exists in this service")
	}
}

func TestSpawnTerminalSession_UnresolvedConnection_ReturnsNotFoundError(t *testing.T) {
	resolver := &fakeConnectionResolver{byConnectionID: map[string]domain.DevServer{}}
	devServers := &fakeDevServerRepository{getErr: errors.New("not found")}
	agent := &fakeDevServerAgentClient{}
	sessions := &fakeTerminalSessionRepository{}
	uc := NewSpawnTerminalSession(resolver, devServers, agent, sessions, false)

	ctx := withTenant(context.Background(), "tenant-1")
	_, err := uc.Execute(ctx, SpawnTerminalSessionInput{ConnectionID: "unknown-conn"})
	if err == nil {
		t.Fatal("expected an error when the connectionId doesn't resolve as a connection or a devServerId")
	}
	if len(agent.spawnPtyCalls) != 0 {
		t.Error("expected no pty.create call when the connectionId doesn't resolve")
	}
}

func TestSpawnTerminalSession_ResolvedConnection_SpawnsAndPersists(t *testing.T) {
	ds, err := domain.NewDevServer("ds1", "tenant-1", "10.0.0.5", domain.ConnectionModeRelayWebSocket, "")
	if err != nil {
		t.Fatalf("building dev server: %v", err)
	}
	resolver := &fakeConnectionResolver{byConnectionID: map[string]domain.DevServer{"conn-1": ds}}
	devServers := &fakeDevServerRepository{}
	agent := &fakeDevServerAgentClient{spawnPtyResult: SpawnPtyResult{PtyID: "pty-abc", Cwd: "/work", Cols: 80, Rows: 24, Shell: "/bin/bash"}}
	sessions := &fakeTerminalSessionRepository{}
	uc := NewSpawnTerminalSession(resolver, devServers, agent, sessions, false)

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

func TestSpawnTerminalSession_AgentFailurePropagates(t *testing.T) {
	ds, err := domain.NewDevServer("ds1", "tenant-1", "10.0.0.5", domain.ConnectionModeRelayWebSocket, "")
	if err != nil {
		t.Fatalf("building dev server: %v", err)
	}
	resolver := &fakeConnectionResolver{byConnectionID: map[string]domain.DevServer{"conn-1": ds}}
	devServers := &fakeDevServerRepository{}
	agent := &fakeDevServerAgentClient{spawnPtyErr: errors.New("devserveragent: not connected")}
	sessions := &fakeTerminalSessionRepository{}
	uc := NewSpawnTerminalSession(resolver, devServers, agent, sessions, false)

	ctx := withTenant(context.Background(), "tenant-1")
	_, err = uc.Execute(ctx, SpawnTerminalSessionInput{ConnectionID: "conn-1"})
	if err == nil {
		t.Fatal("expected the agent's error to propagate")
	}
	if len(sessions.createCalls) != 0 {
		t.Error("expected no persisted session when the agent call fails")
	}
}

// TestSpawnTerminalSession_ConnectionIDIsActuallyADevServerID_SpawnsAndPersists
// is the "chicken-and-egg" regression this fallback exists to close: a
// pre-project ephemeral terminal (CLI install, agent-skill setup) has no
// infra.connections row to resolve — ResolveConnection correctly reports
// connected=false — but the caller's ConnectionID is genuinely a live,
// connected devServerId, and the spawn must still succeed against it,
// exactly like RelayByDevServer already does for Relay. Found live
// 2026-08-30 on the real onboarding CLI-install terminal.
func TestSpawnTerminalSession_ConnectionIDIsActuallyADevServerID_SpawnsAndPersists(t *testing.T) {
	ds, err := domain.NewDevServer("ds-1", "tenant-1", "10.0.0.5", domain.ConnectionModeDirectWebSocket, "")
	if err != nil {
		t.Fatalf("building dev server: %v", err)
	}
	resolver := &fakeConnectionResolver{byConnectionID: map[string]domain.DevServer{}}
	devServers := &fakeDevServerRepository{byID: map[string]domain.DevServer{"ds-1": ds}}
	agent := &fakeDevServerAgentClient{
		isConnected:    true,
		spawnPtyResult: SpawnPtyResult{PtyID: "pty-xyz", Cwd: "/home/orca", Cols: 80, Rows: 24},
	}
	sessions := &fakeTerminalSessionRepository{}
	uc := NewSpawnTerminalSession(resolver, devServers, agent, sessions, false)

	ctx := withTenant(context.Background(), "tenant-1")
	session, err := uc.Execute(ctx, SpawnTerminalSessionInput{ConnectionID: "ds-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if session.PtyID != "pty-xyz" {
		t.Errorf("expected PtyID %q, got %q", "pty-xyz", session.PtyID)
	}
	if len(sessions.createCalls) != 1 {
		t.Fatalf("expected exactly one Create call, got %d", len(sessions.createCalls))
	}
}

func TestSpawnTerminalSession_ConnectionIDIsADevServerIDButNotConnected_ReturnsFailedPrecondition(t *testing.T) {
	ds, err := domain.NewDevServer("ds-1", "tenant-1", "10.0.0.5", domain.ConnectionModeDirectWebSocket, "")
	if err != nil {
		t.Fatalf("building dev server: %v", err)
	}
	resolver := &fakeConnectionResolver{byConnectionID: map[string]domain.DevServer{}}
	devServers := &fakeDevServerRepository{byID: map[string]domain.DevServer{"ds-1": ds}}
	agent := &fakeDevServerAgentClient{isConnected: false}
	sessions := &fakeTerminalSessionRepository{}
	uc := NewSpawnTerminalSession(resolver, devServers, agent, sessions, false)

	ctx := withTenant(context.Background(), "tenant-1")
	_, err = uc.Execute(ctx, SpawnTerminalSessionInput{ConnectionID: "ds-1"})
	if err == nil {
		t.Fatal("expected an error when the dev server has no live agent connection")
	}
	if len(agent.spawnPtyCalls) != 0 {
		t.Error("expected no pty.create call when the dev server isn't connected")
	}
	if len(sessions.createCalls) != 0 {
		t.Error("expected no persisted session when the dev server isn't connected")
	}
}
