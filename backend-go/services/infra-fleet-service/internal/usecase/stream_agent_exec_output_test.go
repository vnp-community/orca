package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

func TestStreamAgentExecOutput_RequiresTenantContext(t *testing.T) {
	uc := NewStreamAgentExecOutput(&fakeConnectionResolver{}, &fakeDevServerRepository{}, &fakeDevServerAgentClient{})
	_, _, err := uc.Execute(context.Background(), StreamAgentExecOutputInput{ConnectionID: "conn-1", StepID: "step-1"})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestStreamAgentExecOutput_RequiresConnectionIDAndStepID(t *testing.T) {
	uc := NewStreamAgentExecOutput(&fakeConnectionResolver{}, &fakeDevServerRepository{}, &fakeDevServerAgentClient{})
	ctx := withTenant(context.Background(), "tenant-1")

	if _, _, err := uc.Execute(ctx, StreamAgentExecOutputInput{StepID: "step-1"}); err == nil {
		t.Error("expected an error when neither connectionId nor devServerId is given")
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
	uc := NewStreamAgentExecOutput(resolver, &fakeDevServerRepository{}, agent)

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
	uc := NewStreamAgentExecOutput(resolver, &fakeDevServerRepository{}, agent)

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
	uc := NewStreamAgentExecOutput(resolver, &fakeDevServerRepository{}, agent)

	ctx := withTenant(context.Background(), "tenant-1")
	_, _, err = uc.Execute(ctx, StreamAgentExecOutputInput{ConnectionID: "conn-1", StepID: "step-1"})
	if err == nil {
		t.Fatal("expected the agent's error to propagate")
	}
}

// tenantScopedDevServers mirrors the real repository's tenant-scoped Get, which fakeDevServerRepository does not.
type tenantScopedDevServers struct {
	fakeDevServerRepository
	owner string
}

func (r *tenantScopedDevServers) Get(ctx context.Context, tenantID, id string) (domain.DevServer, error) {
	ds, ok := r.byID[id]
	if !ok || r.owner != tenantID {
		return domain.DevServer{}, domain.ErrDevServerNotFound
	}
	return ds, nil
}

// A project bound to a dev server without an infra.connections row has no connectionId: the stream must
// follow the dev server id, the same way Relay does through RelayByDevServer.
func TestStreamAgentExecOutput_DevServerFallback_SubscribesWithoutConnection(t *testing.T) {
	ds, err := domain.NewDevServer("ds1", "tenant-1", "10.0.0.5", domain.ConnectionModeDirectWebSocket, "", nil)
	if err != nil {
		t.Fatalf("building dev server: %v", err)
	}
	repo := &tenantScopedDevServers{owner: "tenant-1"}
	repo.byID = map[string]domain.DevServer{ds.ID: ds}
	events := make(chan ExecOutputEvent, 1)
	events <- ExecOutputEvent{StepID: "step-1", Stream: "stdout", Data: "hi"}
	agent := &fakeDevServerAgentClient{isConnected: true, streamExecOutputEvents: events}
	resolver := &fakeConnectionResolver{byConnectionID: map[string]domain.DevServer{}}
	uc := NewStreamAgentExecOutput(resolver, repo, agent)

	out, unsubscribe, err := uc.Execute(withTenant(context.Background(), "tenant-1"), StreamAgentExecOutputInput{DevServerID: ds.ID, StepID: "step-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer unsubscribe()
	if len(agent.streamExecOutputCalls) != 1 || agent.streamExecOutputCalls[0] != "step-1" {
		t.Fatalf("expected one subscribe for step-1, got %v", agent.streamExecOutputCalls)
	}
	if ev := <-out; ev.Data != "hi" {
		t.Errorf("event not passed through: %+v", ev)
	}
}

// Another tenant's dev server id must look like an unknown one, never subscribe.
func TestStreamAgentExecOutput_DevServerFallback_OtherTenantIsNotFound(t *testing.T) {
	ds, err := domain.NewDevServer("ds1", "tenant-1", "10.0.0.5", domain.ConnectionModeDirectWebSocket, "", nil)
	if err != nil {
		t.Fatalf("building dev server: %v", err)
	}
	repo := &tenantScopedDevServers{owner: "tenant-1"}
	repo.byID = map[string]domain.DevServer{ds.ID: ds}
	agent := &fakeDevServerAgentClient{isConnected: true}
	uc := NewStreamAgentExecOutput(&fakeConnectionResolver{}, repo, agent)

	_, _, err = uc.Execute(withTenant(context.Background(), "tenant-2"), StreamAgentExecOutputInput{DevServerID: ds.ID, StepID: "step-1"})
	if err == nil {
		t.Fatal("expected not-found for a dev server owned by another tenant")
	}
	if len(agent.streamExecOutputCalls) != 0 {
		t.Error("must not subscribe to another tenant's dev server")
	}
}

func TestStreamAgentExecOutput_DevServerFallback_NotConnected(t *testing.T) {
	ds, err := domain.NewDevServer("ds1", "tenant-1", "10.0.0.5", domain.ConnectionModeDirectWebSocket, "", nil)
	if err != nil {
		t.Fatalf("building dev server: %v", err)
	}
	repo := &tenantScopedDevServers{owner: "tenant-1"}
	repo.byID = map[string]domain.DevServer{ds.ID: ds}
	agent := &fakeDevServerAgentClient{isConnected: false}
	uc := NewStreamAgentExecOutput(&fakeConnectionResolver{}, repo, agent)

	_, _, err = uc.Execute(withTenant(context.Background(), "tenant-1"), StreamAgentExecOutputInput{DevServerID: ds.ID, StepID: "step-1"})
	if err == nil {
		t.Fatal("expected an error when the dev server has no live agent session")
	}
	if len(agent.streamExecOutputCalls) != 0 {
		t.Error("must not subscribe without a live session")
	}
}

// connectionId keeps priority: the existing path is unchanged when both are sent.
func TestStreamAgentExecOutput_ConnectionIDWinsOverDevServerID(t *testing.T) {
	ds, err := domain.NewDevServer("ds1", "tenant-1", "10.0.0.5", domain.ConnectionModeRelayWebSocket, "", nil)
	if err != nil {
		t.Fatalf("building dev server: %v", err)
	}
	resolver := &fakeConnectionResolver{byConnectionID: map[string]domain.DevServer{"conn-1": ds}}
	events := make(chan ExecOutputEvent)
	agent := &fakeDevServerAgentClient{streamExecOutputEvents: events}
	// The repository holds nothing: reaching it would return a zero dev server and fail the assertions below.
	uc := NewStreamAgentExecOutput(resolver, &tenantScopedDevServers{owner: "tenant-1"}, agent)

	_, unsubscribe, err := uc.Execute(withTenant(context.Background(), "tenant-1"), StreamAgentExecOutputInput{ConnectionID: "conn-1", DevServerID: "other", StepID: "step-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer unsubscribe()
	if len(agent.streamExecOutputCalls) != 1 {
		t.Fatalf("expected the connection path to subscribe, got %v", agent.streamExecOutputCalls)
	}
}
