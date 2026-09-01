package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

func TestStopTerminalProcess_RequiresTenantContext(t *testing.T) {
	uc := NewStopTerminalProcess(&fakeTerminalSessionRepository{}, &fakeConnectionResolver{}, &fakeDevServerRepository{}, &fakeDevServerAgentClient{})
	if err := uc.Execute(context.Background(), "pty-1"); err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestStopTerminalProcess_UnknownPty_ReturnsNotFoundError(t *testing.T) {
	uc := NewStopTerminalProcess(&fakeTerminalSessionRepository{}, &fakeConnectionResolver{}, &fakeDevServerRepository{}, &fakeDevServerAgentClient{})
	ctx := withTenant(context.Background(), "tenant-1")
	if err := uc.Execute(ctx, "pty-unknown"); err == nil {
		t.Fatal("expected an error for a pty with no terminal session row")
	}
}

// TestStopTerminalProcess_SendsRealSIGINT_NotWritePtyCtrlC is the regression
// test for TASK-183's resolved deviation: StopTerminalProcess used to send
// Ctrl-C (0x03) via WritePty; it now calls the real pty.sendSignal primitive
// (agent/src/relay/pty-agent-bridge.ts's handlePtySendSignal, confirmed via
// agent-rpc-dispatch.ts's 'pty.sendSignal' case) with signal=SIGINT.
func TestStopTerminalProcess_SendsRealSIGINT_NotWritePtyCtrlC(t *testing.T) {
	ds, err := domain.NewDevServer("ds1", "tenant-1", "10.0.0.5", domain.ConnectionModeRelayWebSocket, "")
	if err != nil {
		t.Fatalf("building dev server: %v", err)
	}
	resolver := &fakeConnectionResolver{byConnectionID: map[string]domain.DevServer{"conn-1": ds}}
	sessions := &fakeTerminalSessionRepository{byPtyID: map[string]domain.TerminalSession{
		"pty-1": {PtyID: "pty-1", TenantID: "tenant-1", ConnectionID: "conn-1"},
	}}
	agent := &fakeDevServerAgentClient{}
	uc := NewStopTerminalProcess(sessions, resolver, &fakeDevServerRepository{}, agent)

	ctx := withTenant(context.Background(), "tenant-1")
	if err := uc.Execute(ctx, "pty-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(agent.writePtyCalls) != 0 {
		t.Errorf("expected NO WritePty calls (the old Ctrl-C-byte workaround), got %v", agent.writePtyCalls)
	}
	if len(agent.sendSignalCalls) != 1 || agent.sendSignalCalls[0] != "pty-1:SIGINT" {
		t.Errorf("expected exactly one SendSignal(pty-1, SIGINT) call, got %v", agent.sendSignalCalls)
	}
}

func TestStopTerminalProcess_AgentFailurePropagates(t *testing.T) {
	ds, err := domain.NewDevServer("ds1", "tenant-1", "10.0.0.5", domain.ConnectionModeRelayWebSocket, "")
	if err != nil {
		t.Fatalf("building dev server: %v", err)
	}
	resolver := &fakeConnectionResolver{byConnectionID: map[string]domain.DevServer{"conn-1": ds}}
	sessions := &fakeTerminalSessionRepository{byPtyID: map[string]domain.TerminalSession{
		"pty-1": {PtyID: "pty-1", TenantID: "tenant-1", ConnectionID: "conn-1"},
	}}
	agent := &fakeDevServerAgentClient{sendSignalErr: errors.New("agent unreachable")}
	uc := NewStopTerminalProcess(sessions, resolver, &fakeDevServerRepository{}, agent)

	ctx := withTenant(context.Background(), "tenant-1")
	if err := uc.Execute(ctx, "pty-1"); err == nil {
		t.Fatal("expected the agent's SendSignal error to propagate")
	}
}
