package devserveragent

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/usecase"
)

func TestClientSpawnPtySucceedsAgainstFakeAgent(t *testing.T) {
	agent := &fakeAgent{t: t, requireToken: fakeAgentToken, results: map[string]any{
		"pty.create": map[string]any{"id": "pty-abc", "cols": 80, "rows": 24, "cwd": "/work", "shell": "/bin/bash"},
	}}
	host, port := startFakeAgent(t, agent)

	client := New(testConfig(port, fakeAgentToken), slog.Default())
	t.Cleanup(client.Close)

	devServer, err := domain.NewDevServer("ds-1", "tenant-1", host, domain.ConnectionModeRelayWebSocket, "")
	if err != nil {
		t.Fatalf("NewDevServer: %v", err)
	}

	result, err := client.SpawnPty(context.Background(), devServer, usecase.SpawnPtyInput{Cwd: "/repo", Cols: 80, Rows: 24})
	if err != nil {
		t.Fatalf("SpawnPty: %v", err)
	}
	if result.PtyID != "pty-abc" || result.Cwd != "/work" || result.Shell != "/bin/bash" {
		t.Errorf("unexpected SpawnPtyResult: %+v", result)
	}
}

func TestClientWriteResizeKillPty_CallExpectedMethods(t *testing.T) {
	agent := &fakeAgent{t: t, requireToken: fakeAgentToken, results: map[string]any{
		"pty.write":   map[string]any{},
		"pty.resize":  map[string]any{},
		"pty.destroy": map[string]any{},
	}}
	host, port := startFakeAgent(t, agent)

	client := New(testConfig(port, fakeAgentToken), slog.Default())
	t.Cleanup(client.Close)

	devServer, err := domain.NewDevServer("ds-2", "tenant-1", host, domain.ConnectionModeRelayWebSocket, "")
	if err != nil {
		t.Fatalf("NewDevServer: %v", err)
	}

	if err := client.WritePty(context.Background(), devServer, "pty-1", []byte("ls\n")); err != nil {
		t.Errorf("WritePty: %v", err)
	}
	if err := client.ResizePty(context.Background(), devServer, "pty-1", 100, 40); err != nil {
		t.Errorf("ResizePty: %v", err)
	}
	if err := client.KillPty(context.Background(), devServer, "pty-1", true); err != nil {
		t.Errorf("KillPty: %v", err)
	}
}

func TestClientAgentStatusAndInspectProcess_FromListProcesses(t *testing.T) {
	agent := &fakeAgent{t: t, requireToken: fakeAgentToken, results: map[string]any{
		"pty.listProcesses": []any{
			map[string]any{"id": "pty-1", "cwd": "/work", "title": "claude"},
			map[string]any{"id": "pty-2", "cwd": "/other", "title": "bash"},
		},
	}}
	host, port := startFakeAgent(t, agent)

	client := New(testConfig(port, fakeAgentToken), slog.Default())
	t.Cleanup(client.Close)

	devServer, err := domain.NewDevServer("ds-3", "tenant-1", host, domain.ConnectionModeRelayWebSocket, "")
	if err != nil {
		t.Fatalf("NewDevServer: %v", err)
	}

	status, err := client.AgentStatus(context.Background(), devServer, "pty-1")
	if err != nil {
		t.Fatalf("AgentStatus: %v", err)
	}
	if !status.AgentRunning || status.AgentKind != "claude" {
		t.Errorf("expected AgentRunning=true AgentKind=claude for pty-1, got %+v", status)
	}

	status2, err := client.AgentStatus(context.Background(), devServer, "pty-2")
	if err != nil {
		t.Fatalf("AgentStatus: %v", err)
	}
	if status2.AgentRunning {
		t.Errorf("expected AgentRunning=false for a plain shell, got %+v", status2)
	}

	inspect, err := client.InspectProcess(context.Background(), devServer, "pty-1")
	if err != nil {
		t.Fatalf("InspectProcess: %v", err)
	}
	if !inspect.Known || inspect.Command != "claude" || inspect.Cwd != "/work" {
		t.Errorf("unexpected InspectProcessResult: %+v", inspect)
	}

	inspectUnknown, err := client.InspectProcess(context.Background(), devServer, "pty-unknown")
	if err != nil {
		t.Fatalf("InspectProcess: %v", err)
	}
	if inspectUnknown.Known {
		t.Errorf("expected Known=false for an id absent from pty.listProcesses, got %+v", inspectUnknown)
	}
}

func TestClientStreamPty_RoutesDataAndExitNotifications(t *testing.T) {
	agent := &fakeAgent{t: t, requireToken: fakeAgentToken, results: map[string]any{}, pushNotifications: []fakeAgentNotification{
		{method: "pty.data", ptyID: "pty-1", data: "hello"},
		{method: "pty.data", ptyID: "pty-other", data: "not for us"},
		{method: "pty.exit", ptyID: "pty-1", exitCode: 5},
	}}
	host, port := startFakeAgent(t, agent)

	client := New(testConfig(port, fakeAgentToken), slog.Default())
	t.Cleanup(client.Close)

	devServer, err := domain.NewDevServer("ds-4", "tenant-1", host, domain.ConnectionModeRelayWebSocket, "")
	if err != nil {
		t.Fatalf("NewDevServer: %v", err)
	}

	// Connect first so subscribePty attaches to a live, already-handshaked
	// session before the fake agent's delayed push fires.
	if _, err := client.Exec(context.Background(), devServer, "noop", nil); err == nil {
		t.Fatal("expected an unknown-method error from the fake agent (sanity check that the session is live)")
	}

	events, unsubscribe, err := client.StreamPty(context.Background(), devServer, "pty-1")
	if err != nil {
		t.Fatalf("StreamPty: %v", err)
	}
	defer unsubscribe()

	var gotData, gotExit bool
	deadline := time.After(2 * time.Second)
	for !gotData || !gotExit {
		select {
		case ev, ok := <-events:
			if !ok {
				t.Fatal("events channel closed before both expected events arrived")
			}
			if ev.PtyID != "pty-1" {
				t.Fatalf("expected events only for pty-1 (subscription should be scoped), got %+v", ev)
			}
			if ev.Exited {
				gotExit = true
				if ev.ExitCode != 5 {
					t.Errorf("expected ExitCode=5, got %d", ev.ExitCode)
				}
			} else {
				gotData = true
				if string(ev.Data) != "hello" {
					t.Errorf("expected Data=%q, got %q", "hello", ev.Data)
				}
			}
		case <-deadline:
			t.Fatalf("timed out waiting for events (gotData=%v gotExit=%v)", gotData, gotExit)
		}
	}
}
