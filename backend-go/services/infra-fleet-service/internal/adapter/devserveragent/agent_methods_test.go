package devserveragent

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/usecase"
)

func TestSpawnAgent_SendsExpectedParamsAndDecodesResult(t *testing.T) {
	agent := &fakeAgent{t: t, requireToken: fakeAgentToken, results: map[string]any{
		"agent.spawn": map[string]any{"ptyId": "agent-pty-1"},
	}}
	host, port := startFakeAgent(t, agent)

	client := newTestClientWithToken(port, fakeAgentToken)
	t.Cleanup(client.Close)

	devServer, err := domain.NewDevServer("ds-agent-1", "tenant-1", host, domain.ConnectionModeRelayWebSocket, "", nil)
	if err != nil {
		t.Fatalf("NewDevServer: %v", err)
	}

	result, err := client.SpawnAgent(context.Background(), devServer, usecase.SpawnAgentInput{
		TaskID: "task-1", UserID: "user-1", ModelID: "claude", AccountID: "acc-1",
		Cwd: "/repo", WorktreePath: "/repo", ResumeID: "resume-1", BranchName: "main",
		Cols: 80, Rows: 24, TrustPreset: "standard",
	})
	if err != nil {
		t.Fatalf("SpawnAgent: %v", err)
	}
	if result.PtyID != "agent-pty-1" {
		t.Errorf("expected PtyID %q, got %q", "agent-pty-1", result.PtyID)
	}

	params := agent.lastParams(t, "agent.spawn")
	if params["taskId"] != "task-1" || params["userId"] != "user-1" || params["model"] != "claude" || params["accountId"] != "acc-1" {
		t.Errorf("unexpected agent.spawn params: %+v", params)
	}
	if params["resumeId"] != "resume-1" || params["worktreePath"] != "/repo" || params["branchName"] != "main" || params["trustPreset"] != "standard" {
		t.Errorf("unexpected agent.spawn params: %+v", params)
	}
}

func TestSpawnAgent_MissingPtyIDInResponse_ReturnsError(t *testing.T) {
	agent := &fakeAgent{t: t, requireToken: fakeAgentToken, results: map[string]any{
		"agent.spawn": map[string]any{},
	}}
	host, port := startFakeAgent(t, agent)

	client := newTestClientWithToken(port, fakeAgentToken)
	t.Cleanup(client.Close)

	devServer, err := domain.NewDevServer("ds-agent-missing", "tenant-1", host, domain.ConnectionModeRelayWebSocket, "", nil)
	if err != nil {
		t.Fatalf("NewDevServer: %v", err)
	}

	if _, err := client.SpawnAgent(context.Background(), devServer, usecase.SpawnAgentInput{TaskID: "task-1"}); err == nil {
		t.Fatal("expected an error for an agent.spawn response missing ptyId")
	}
}

func TestKillAgent_SendsExpectedParamsAndDefaultsSignal(t *testing.T) {
	agent := &fakeAgent{t: t, requireToken: fakeAgentToken, results: map[string]any{
		"agent.kill": map[string]any{},
	}}
	host, port := startFakeAgent(t, agent)

	client := newTestClientWithToken(port, fakeAgentToken)
	t.Cleanup(client.Close)

	devServer, err := domain.NewDevServer("ds-agent-kill", "tenant-1", host, domain.ConnectionModeRelayWebSocket, "", nil)
	if err != nil {
		t.Fatalf("NewDevServer: %v", err)
	}

	if err := client.KillAgent(context.Background(), devServer, "agent-pty-1", ""); err != nil {
		t.Fatalf("KillAgent: %v", err)
	}
	params := agent.lastParams(t, "agent.kill")
	if params["id"] != "agent-pty-1" || params["signal"] != "SIGKILL" {
		t.Errorf("expected default signal SIGKILL, got params=%+v", params)
	}

	if err := client.KillAgent(context.Background(), devServer, "agent-pty-1", "SIGTERM"); err != nil {
		t.Fatalf("KillAgent: %v", err)
	}
	params = agent.lastParams(t, "agent.kill")
	if params["signal"] != "SIGTERM" {
		t.Errorf("expected signal SIGTERM to be forwarded as-is, got params=%+v", params)
	}
}

func TestSendAgentInput_SendsExpectedParams(t *testing.T) {
	agent := &fakeAgent{t: t, requireToken: fakeAgentToken, results: map[string]any{
		"agent.sendInput": map[string]any{},
	}}
	host, port := startFakeAgent(t, agent)

	client := newTestClientWithToken(port, fakeAgentToken)
	t.Cleanup(client.Close)

	devServer, err := domain.NewDevServer("ds-agent-input", "tenant-1", host, domain.ConnectionModeRelayWebSocket, "", nil)
	if err != nil {
		t.Fatalf("NewDevServer: %v", err)
	}

	if err := client.SendAgentInput(context.Background(), devServer, "agent-pty-1", []byte{0x03}); err != nil {
		t.Fatalf("SendAgentInput: %v", err)
	}
	params := agent.lastParams(t, "agent.sendInput")
	if params["id"] != "agent-pty-1" || params["data"] != "\x03" {
		t.Errorf("unexpected agent.sendInput params: %+v", params)
	}
}

func TestStreamAgentHooks_DecodesAndDeliversProviderSession(t *testing.T) {
	hookParams, _ := json.Marshal(map[string]any{
		"worktreeId":      "wt-1",
		"providerSession": map[string]any{"key": "session_id", "id": "provider-sess-1"},
	})
	noSessionParams, _ := json.Marshal(map[string]any{"worktreeId": "wt-2"})

	agent := &fakeAgent{t: t, requireToken: fakeAgentToken, results: map[string]any{}, pushNotifications: []fakeAgentNotification{
		{method: "agent.hook", rawParams: hookParams},
		{method: "agent.hook", rawParams: noSessionParams},
	}}
	host, port := startFakeAgent(t, agent)

	client := newTestClientWithToken(port, fakeAgentToken)
	t.Cleanup(client.Close)

	devServer, err := domain.NewDevServer("ds-agent-hooks", "tenant-1", host, domain.ConnectionModeRelayWebSocket, "", nil)
	if err != nil {
		t.Fatalf("NewDevServer: %v", err)
	}

	// Connect first so the subscription attaches before the fake agent's
	// delayed push fires — mirrors methods_test.go's StreamPty tests.
	if _, err := client.Exec(context.Background(), devServer, "noop", nil); err == nil {
		t.Fatal("expected an unknown-method error from the fake agent (sanity check that the session is live)")
	}

	events, unsubscribe, err := client.StreamAgentHooks(context.Background(), devServer)
	if err != nil {
		t.Fatalf("StreamAgentHooks: %v", err)
	}
	defer unsubscribe()

	var withSession, withoutSession bool
	deadline := time.After(2 * time.Second)
	for !withSession || !withoutSession {
		select {
		case ev, ok := <-events:
			if !ok {
				t.Fatal("events channel closed before both expected notifications arrived")
			}
			switch ev.WorktreeID {
			case "wt-1":
				withSession = true
				if ev.ProviderSessionKey != "session_id" || ev.ProviderSessionID != "provider-sess-1" {
					t.Errorf("unexpected providerSession decoding: %+v", ev)
				}
			case "wt-2":
				withoutSession = true
				if ev.ProviderSessionKey != "" || ev.ProviderSessionID != "" {
					t.Errorf("expected empty providerSession fields for a hook with none, got %+v", ev)
				}
			}
		case <-deadline:
			t.Fatalf("timed out waiting for both notifications (withSession=%v withoutSession=%v)", withSession, withoutSession)
		}
	}
}
