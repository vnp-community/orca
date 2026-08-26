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

// TestClientSpawnPty_MissingIDInResponse_ReturnsClearError closes a gap
// between methods.go's original design intent (a malformed pty.create
// response should fail loudly, not surface an empty PtyID several layers
// up) and what the code actually did before this test was added.
func TestClientSpawnPty_MissingIDInResponse_ReturnsClearError(t *testing.T) {
	agent := &fakeAgent{t: t, requireToken: fakeAgentToken, results: map[string]any{
		// No "id" field — a malformed/degenerate agent response.
		"pty.create": map[string]any{"cols": 80, "rows": 24, "cwd": "/work", "shell": "/bin/bash"},
	}}
	host, port := startFakeAgent(t, agent)

	client := New(testConfig(port, fakeAgentToken), slog.Default())
	t.Cleanup(client.Close)

	devServer, err := domain.NewDevServer("ds-missing-id", "tenant-1", host, domain.ConnectionModeRelayWebSocket, "")
	if err != nil {
		t.Fatalf("NewDevServer: %v", err)
	}

	result, err := client.SpawnPty(context.Background(), devServer, usecase.SpawnPtyInput{Cwd: "/repo"})
	if err == nil {
		t.Fatalf("expected an error for a pty.create response missing id, got result=%+v", result)
	}
}

// TestClientWriteResizeKillPty_SendsExpectedParams verifies not just that
// the right method name resolves (the fake agent replies per-method
// regardless of params) but that the actual request params reaching the
// wire use the field names methods.go's doc comments claim are confirmed
// against agent-rpc-dispatch.ts ({id,data}/{id,cols,rows}/{id,graceful}).
func TestClientWriteResizeKillPty_SendsExpectedParams(t *testing.T) {
	agent := &fakeAgent{t: t, requireToken: fakeAgentToken, results: map[string]any{
		"pty.write":   map[string]any{},
		"pty.resize":  map[string]any{},
		"pty.destroy": map[string]any{},
	}}
	host, port := startFakeAgent(t, agent)

	client := New(testConfig(port, fakeAgentToken), slog.Default())
	t.Cleanup(client.Close)

	devServer, err := domain.NewDevServer("ds-params", "tenant-1", host, domain.ConnectionModeRelayWebSocket, "")
	if err != nil {
		t.Fatalf("NewDevServer: %v", err)
	}

	if err := client.WritePty(context.Background(), devServer, "pty-9", []byte("ls\n")); err != nil {
		t.Fatalf("WritePty: %v", err)
	}
	writeParams := agent.lastParams(t, "pty.write")
	if writeParams["id"] != "pty-9" || writeParams["data"] != "ls\n" {
		t.Errorf("pty.write params = %+v, want id=pty-9 data=%q", writeParams, "ls\n")
	}

	if err := client.ResizePty(context.Background(), devServer, "pty-9", 100, 40); err != nil {
		t.Fatalf("ResizePty: %v", err)
	}
	resizeParams := agent.lastParams(t, "pty.resize")
	if resizeParams["id"] != "pty-9" || resizeParams["cols"] != float64(100) || resizeParams["rows"] != float64(40) {
		t.Errorf("pty.resize params = %+v, want id=pty-9 cols=100 rows=40", resizeParams)
	}

	if err := client.KillPty(context.Background(), devServer, "pty-9", true); err != nil {
		t.Fatalf("KillPty: %v", err)
	}
	killParams := agent.lastParams(t, "pty.destroy")
	if killParams["id"] != "pty-9" || killParams["graceful"] != true {
		t.Errorf("pty.destroy params = %+v, want id=pty-9 graceful=true", killParams)
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

// TestClientSendSignal_SendsExpectedParams verifies pty.sendSignal's real
// {id, signal} param shape (confirmed against
// agent/src/relay/pty-agent-bridge.ts's handlePtySendSignal doc comment) —
// the primitive that replaces the former WritePty(0x03) Ctrl-C workaround
// for StopTerminalProcess (TASK-183 follow-up).
func TestClientSendSignal_SendsExpectedParams(t *testing.T) {
	agent := &fakeAgent{t: t, requireToken: fakeAgentToken, results: map[string]any{
		"pty.sendSignal": map[string]any{"ok": true},
	}}
	host, port := startFakeAgent(t, agent)

	client := New(testConfig(port, fakeAgentToken), slog.Default())
	t.Cleanup(client.Close)

	devServer, err := domain.NewDevServer("ds-signal", "tenant-1", host, domain.ConnectionModeRelayWebSocket, "")
	if err != nil {
		t.Fatalf("NewDevServer: %v", err)
	}

	if err := client.SendSignal(context.Background(), devServer, "pty-9", "SIGINT"); err != nil {
		t.Fatalf("SendSignal: %v", err)
	}
	params := agent.lastParams(t, "pty.sendSignal")
	if params["id"] != "pty-9" || params["signal"] != "SIGINT" {
		t.Errorf("pty.sendSignal params = %+v, want id=pty-9 signal=SIGINT", params)
	}
}

// TestClientSendSignal_RejectsUnknownSignal_WithoutACall proves an
// unsupported signal never reaches the wire — allowedSignals mirrors the
// agent's own ALLOWED_SIGNALS set, so this fails fast client-side instead of
// round-tripping to a -32602 from the agent.
func TestClientSendSignal_RejectsUnknownSignal_WithoutACall(t *testing.T) {
	agent := &fakeAgent{t: t, requireToken: fakeAgentToken, results: map[string]any{}}
	host, port := startFakeAgent(t, agent)

	client := New(testConfig(port, fakeAgentToken), slog.Default())
	t.Cleanup(client.Close)

	devServer, err := domain.NewDevServer("ds-signal-bad", "tenant-1", host, domain.ConnectionModeRelayWebSocket, "")
	if err != nil {
		t.Fatalf("NewDevServer: %v", err)
	}

	if err := client.SendSignal(context.Background(), devServer, "pty-9", "SIGWHATEVER"); err == nil {
		t.Fatal("expected an error for a signal outside the agent's allowed set")
	}
}

func TestClientAgentStatusAndInspectProcess_FromListProcesses(t *testing.T) {
	agent := &fakeAgent{t: t, requireToken: fakeAgentToken, results: map[string]any{
		"pty.listProcesses": []any{
			map[string]any{"id": "pty-1", "cwd": "/work", "title": "claude", "pid": 4242},
			map[string]any{"id": "pty-2", "cwd": "/other", "title": "bash", "pid": 4243},
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
	if !inspect.Known || inspect.Command != "claude" || inspect.Cwd != "/work" || inspect.Pid != 4242 {
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

// TestClientStreamPty_TwoConcurrentSubscriptions_EachGetsOwnEvents proves
// the demux is genuinely per-subscriber, not just "not obviously wrong" —
// two StreamPty calls for two different ptyIds live on the same session at
// once, and each must see only its own pty's notifications, never the
// other's.
func TestClientStreamPty_TwoConcurrentSubscriptions_EachGetsOwnEvents(t *testing.T) {
	agent := &fakeAgent{t: t, requireToken: fakeAgentToken, results: map[string]any{}, pushNotifications: []fakeAgentNotification{
		{method: "pty.data", ptyID: "pty-a", data: "from-a-1"},
		{method: "pty.data", ptyID: "pty-b", data: "from-b-1"},
		{method: "pty.data", ptyID: "pty-a", data: "from-a-2"},
		{method: "pty.exit", ptyID: "pty-b", exitCode: 7},
		{method: "pty.exit", ptyID: "pty-a", exitCode: 0},
	}}
	host, port := startFakeAgent(t, agent)

	client := New(testConfig(port, fakeAgentToken), slog.Default())
	t.Cleanup(client.Close)

	devServer, err := domain.NewDevServer("ds-5", "tenant-1", host, domain.ConnectionModeRelayWebSocket, "")
	if err != nil {
		t.Fatalf("NewDevServer: %v", err)
	}

	// Connect first so both subscriptions attach before the fake agent's
	// delayed push fires (mirrors the single-subscriber test above).
	if _, err := client.Exec(context.Background(), devServer, "noop", nil); err == nil {
		t.Fatal("expected an unknown-method error from the fake agent (sanity check that the session is live)")
	}

	eventsA, unsubA, err := client.StreamPty(context.Background(), devServer, "pty-a")
	if err != nil {
		t.Fatalf("StreamPty(pty-a): %v", err)
	}
	defer unsubA()
	eventsB, unsubB, err := client.StreamPty(context.Background(), devServer, "pty-b")
	if err != nil {
		t.Fatalf("StreamPty(pty-b): %v", err)
	}
	defer unsubB()

	var gotAExit, gotBExit bool
	deadline := time.After(2 * time.Second)
	for !gotAExit || !gotBExit {
		select {
		case ev, ok := <-eventsA:
			if !ok {
				t.Fatal("pty-a events channel closed before its exit event arrived")
			}
			if ev.PtyID != "pty-a" {
				t.Fatalf("pty-a subscription leaked a foreign event: %+v", ev)
			}
			if ev.Exited {
				gotAExit = true
			}
		case ev, ok := <-eventsB:
			if !ok {
				t.Fatal("pty-b events channel closed before its exit event arrived")
			}
			if ev.PtyID != "pty-b" {
				t.Fatalf("pty-b subscription leaked a foreign event: %+v", ev)
			}
			if ev.Exited {
				gotBExit = true
			}
		case <-deadline:
			t.Fatalf("timed out waiting for both subscriptions' exit events (gotAExit=%v gotBExit=%v)", gotAExit, gotBExit)
		}
	}
}

// TestClientStreamPty_ContextCancellationClosesOutputChannel verifies the
// forwarding goroutine started by StreamPty exits and closes its output
// channel when ctx is cancelled, even if the caller never calls unsubscribe
// — so a cancelled-context caller isn't left blocked reading from a channel
// that will never close or receive again.
func TestClientStreamPty_ContextCancellationClosesOutputChannel(t *testing.T) {
	agent := &fakeAgent{t: t, requireToken: fakeAgentToken, results: map[string]any{}}
	host, port := startFakeAgent(t, agent)

	client := New(testConfig(port, fakeAgentToken), slog.Default())
	t.Cleanup(client.Close)

	devServer, err := domain.NewDevServer("ds-6", "tenant-1", host, domain.ConnectionModeRelayWebSocket, "")
	if err != nil {
		t.Fatalf("NewDevServer: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	events, unsubscribe, err := client.StreamPty(ctx, devServer, "pty-cancel")
	if err != nil {
		t.Fatalf("StreamPty: %v", err)
	}
	t.Cleanup(unsubscribe)

	cancel()

	select {
	case _, ok := <-events:
		if ok {
			t.Fatal("expected the events channel to close (no events were ever pushed), got a value instead")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for events channel to close after ctx cancellation")
	}
}
