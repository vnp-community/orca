package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

func TestRelay_RequiresTenantContext(t *testing.T) {
	uc := NewRelay(&fakeConnectionResolver{}, &fakeDevServerAgentClient{})
	_, err := uc.Execute(context.Background(), RelayInput{ConnectionID: "conn-1", Method: "git.status"})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestRelay_RequiresConnectionIDAndMethod(t *testing.T) {
	uc := NewRelay(&fakeConnectionResolver{}, &fakeDevServerAgentClient{})
	ctx := withTenant(context.Background(), "tenant-1")

	if _, err := uc.Execute(ctx, RelayInput{Method: "git.status"}); err == nil {
		t.Error("expected an error for an empty connectionId")
	}
	if _, err := uc.Execute(ctx, RelayInput{ConnectionID: "conn-1"}); err == nil {
		t.Error("expected an error for an empty method")
	}
}

// Unresolved connectionId is a real error for Relay — unlike ScanWorkspacePorts's
// "execute locally" fallback, Relay has no local path: a caller reaching this
// RPC has already decided it needs a dev server.
func TestRelay_UnresolvedConnection_ReturnsNotFoundError(t *testing.T) {
	resolver := &fakeConnectionResolver{byConnectionID: map[string]domain.DevServer{}}
	agent := &fakeDevServerAgentClient{}
	uc := NewRelay(resolver, agent)

	ctx := withTenant(context.Background(), "tenant-1")
	_, err := uc.Execute(ctx, RelayInput{ConnectionID: "unknown-conn", Method: "git.status"})
	if err == nil {
		t.Fatal("expected an error when the connectionId doesn't resolve")
	}
	if len(agent.execCalls) != 0 {
		t.Error("expected no relay to the agent when the connectionId doesn't resolve")
	}
}

func TestRelay_ResolvedConnection_ExecutesOnAgent(t *testing.T) {
	ds, err := domain.NewDevServer("ds1", "tenant-1", "10.0.0.5", domain.ConnectionModeRelayWebSocket, "")
	if err != nil {
		t.Fatalf("building dev server: %v", err)
	}
	resolver := &fakeConnectionResolver{byConnectionID: map[string]domain.DevServer{"conn-1": ds}}
	agent := &fakeDevServerAgentClient{execResult: map[string]any{"branch": "main"}}
	uc := NewRelay(resolver, agent)

	ctx := withTenant(context.Background(), "tenant-1")
	result, err := uc.Execute(ctx, RelayInput{ConnectionID: "conn-1", Method: "git.status", Params: map[string]any{"repoPath": "/repo"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(agent.execCalls) != 1 || agent.execCalls[0] != "git.status" {
		t.Fatalf("expected exactly one git.status relay call, got %v", agent.execCalls)
	}
	if result["branch"] != "main" {
		t.Errorf("expected agent result to pass through, got %+v", result)
	}
}

func TestRelay_AgentFailurePropagates(t *testing.T) {
	ds, err := domain.NewDevServer("ds1", "tenant-1", "10.0.0.5", domain.ConnectionModeRelayWebSocket, "")
	if err != nil {
		t.Fatalf("building dev server: %v", err)
	}
	resolver := &fakeConnectionResolver{byConnectionID: map[string]domain.DevServer{"conn-1": ds}}
	agent := &fakeDevServerAgentClient{execErr: errors.New("devserveragent: not connected")}
	uc := NewRelay(resolver, agent)

	ctx := withTenant(context.Background(), "tenant-1")
	_, err = uc.Execute(ctx, RelayInput{ConnectionID: "conn-1", Method: "git.status"})
	if err == nil {
		t.Fatal("expected the agent's error to propagate")
	}
}
