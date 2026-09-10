package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

func TestStreamAgentExecOutput_RequiresTenantContext(t *testing.T) {
	uc := NewStreamAgentExecOutput(&fakeConnectionResolver{}, &fakeDevServerAgentClient{})
	_, _, err := uc.Execute(context.Background(), StreamAgentExecOutputInput{ConnectionID: "conn-1", StepID: "step-1"})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestStreamAgentExecOutput_RequiresConnectionIDAndStepID(t *testing.T) {
	uc := NewStreamAgentExecOutput(&fakeConnectionResolver{}, &fakeDevServerAgentClient{})
	ctx := withTenant(context.Background(), "tenant-1")

	if _, _, err := uc.Execute(ctx, StreamAgentExecOutputInput{StepID: "step-1"}); err == nil {
		t.Error("expected an error for an empty connectionId")
	}
	if _, _, err := uc.Execute(ctx, StreamAgentExecOutputInput{ConnectionID: "conn-1"}); err == nil {
		t.Error("expected an error for an empty stepId")
	}
}

// Unresolved connectionId is a real error here too, same posture as Relay's
// own TestRelay_UnresolvedConnection_ReturnsNotFoundError.
func TestStreamAgentExecOutput_UnresolvedConnection_ReturnsNotFoundError(t *testing.T) {
	resolver := &fakeConnectionResolver{byConnectionID: map[string]domain.DevServer{}}
	agent := &fakeDevServerAgentClient{}
	uc := NewStreamAgentExecOutput(resolver, agent)

	ctx := withTenant(context.Background(), "tenant-1")
	_, _, err := uc.Execute(ctx, StreamAgentExecOutputInput{ConnectionID: "unknown-conn", StepID: "step-1"})
	if err == nil {
		t.Fatal("expected an error when the connectionId doesn't resolve")
	}
	if len(agent.streamExecOutputCalls) != 0 {
		t.Error("expected no subscribe call to the agent when the connectionId doesn't resolve")
	}
}

func TestStreamAgentExecOutput_ResolvedConnection_SubscribesOnAgent(t *testing.T) {
	ds, err := domain.NewDevServer("ds1", "tenant-1", "10.0.0.5", domain.ConnectionModeRelayWebSocket, "", nil)
	if err != nil {
		t.Fatalf("building dev server: %v", err)
	}
	resolver := &fakeConnectionResolver{byConnectionID: map[string]domain.DevServer{"conn-1": ds}}
	events := make(chan ExecOutputEvent, 1)
	events <- ExecOutputEvent{StepID: "step-1", Stream: "stdout", Data: "hi"}
	agent := &fakeDevServerAgentClient{streamExecOutputEvents: events}
	uc := NewStreamAgentExecOutput(resolver, agent)

	ctx := withTenant(context.Background(), "tenant-1")
	out, unsubscribe, err := uc.Execute(ctx, StreamAgentExecOutputInput{ConnectionID: "conn-1", StepID: "step-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(agent.streamExecOutputCalls) != 1 || agent.streamExecOutputCalls[0] != "step-1" {
		t.Fatalf("expected exactly one StreamExecOutput call for step-1, got %v", agent.streamExecOutputCalls)
	}
	select {
	case ev := <-out:
		if ev.Data != "hi" {
			t.Errorf("expected event to pass through unchanged, got %+v", ev)
		}
	default:
		t.Fatal("expected the fake's buffered event to be immediately readable")
	}
	unsubscribe()
	if !agent.streamExecOutputUnsubscribed {
		t.Error("expected unsubscribe to propagate to the agent client")
	}
}

func TestStreamAgentExecOutput_AgentFailurePropagates(t *testing.T) {
	ds, err := domain.NewDevServer("ds1", "tenant-1", "10.0.0.5", domain.ConnectionModeRelayWebSocket, "", nil)
	if err != nil {
		t.Fatalf("building dev server: %v", err)
	}
	resolver := &fakeConnectionResolver{byConnectionID: map[string]domain.DevServer{"conn-1": ds}}
	agent := &fakeDevServerAgentClient{streamExecOutputErr: errors.New("devserveragent: not connected")}
	uc := NewStreamAgentExecOutput(resolver, agent)

	ctx := withTenant(context.Background(), "tenant-1")
	_, _, err = uc.Execute(ctx, StreamAgentExecOutputInput{ConnectionID: "conn-1", StepID: "step-1"})
	if err == nil {
		t.Fatal("expected the agent's error to propagate")
	}
}
