package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

var errFakeAgentExec = errors.New("fake agent exec failure")

func TestKillWorkspacePort_ResolveThenDispatch(t *testing.T) {
	t.Run("no connectionId: honest not-implemented, no agent call", func(t *testing.T) {
		resolver := &fakeConnectionResolver{}
		agent := &fakeDevServerAgentClient{}
		uc := NewKillWorkspacePort(resolver, agent)

		ok, reason, err := uc.Execute(withTenant(context.Background(), "t1"), KillWorkspacePortInput{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ok || reason == "" {
			t.Errorf("expected ok=false with a reason, got ok=%v reason=%q", ok, reason)
		}
		if agent.execCalled {
			t.Error("expected no agent call when connectionId is empty")
		}
	})

	t.Run("connectionId resolves but not connected: same as no connectionId, no agent call", func(t *testing.T) {
		resolver := &fakeConnectionResolver{}
		agent := &fakeDevServerAgentClient{}
		uc := NewKillWorkspacePort(resolver, agent)

		ok, _, err := uc.Execute(withTenant(context.Background(), "t1"), KillWorkspacePortInput{ConnectionID: "c1"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ok {
			t.Error("expected ok=false when not connected")
		}
		if agent.execCalled {
			t.Error("expected no agent call when not connected")
		}
	})

	t.Run("connectionId resolves and connected: relays to agent ports.kill", func(t *testing.T) {
		resolver := &fakeConnectionResolver{byConnectionID: map[string]domain.DevServer{"c1": {ID: "ds1", TenantID: "t1"}}}
		agent := &fakeDevServerAgentClient{execResult: map[string]any{"ok": true}}
		uc := NewKillWorkspacePort(resolver, agent)

		ok, _, err := uc.Execute(withTenant(context.Background(), "t1"), KillWorkspacePortInput{
			ConnectionID: "c1", WorktreeID: "wt1", PID: 123, Port: 8080,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !ok {
			t.Error("expected ok=true")
		}
		if agent.lastMethod != "ports.kill" {
			t.Errorf("expected agent method ports.kill, got %q", agent.lastMethod)
		}
	})

	t.Run("agent exec error is propagated, never swallowed into ok:false", func(t *testing.T) {
		resolver := &fakeConnectionResolver{byConnectionID: map[string]domain.DevServer{"c1": {ID: "ds1", TenantID: "t1"}}}
		agent := &fakeDevServerAgentClient{execErr: errFakeAgentExec}
		uc := NewKillWorkspacePort(resolver, agent)

		_, _, err := uc.Execute(withTenant(context.Background(), "t1"), KillWorkspacePortInput{ConnectionID: "c1"})
		if err == nil {
			t.Fatal("expected error to propagate, not be swallowed into ok:false")
		}
	})
}

func TestKillWorkspacePort_RequiresTenantContext(t *testing.T) {
	uc := NewKillWorkspacePort(&fakeConnectionResolver{}, &fakeDevServerAgentClient{})
	_, _, err := uc.Execute(context.Background(), KillWorkspacePortInput{})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}
