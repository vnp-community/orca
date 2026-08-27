package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

func TestRelayStream_RequiresTenantContext(t *testing.T) {
	uc := NewRelayStream(&fakeConnectionResolver{}, &fakeDevServerAgentClient{})
	err := uc.Execute(context.Background(), RelayStreamInput{ConnectionID: "conn-1", Method: "git.execStream"}, func(map[string]any) error { return nil })
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestRelayStream_RequiresConnectionIDAndMethod(t *testing.T) {
	uc := NewRelayStream(&fakeConnectionResolver{}, &fakeDevServerAgentClient{})
	ctx := withTenant(context.Background(), "tenant-1")

	if err := uc.Execute(ctx, RelayStreamInput{Method: "git.execStream"}, func(map[string]any) error { return nil }); err == nil {
		t.Error("expected an error for an empty connectionId")
	}
	if err := uc.Execute(ctx, RelayStreamInput{ConnectionID: "conn-1"}, func(map[string]any) error { return nil }); err == nil {
		t.Error("expected an error for an empty method")
	}
}

func TestRelayStream_UnresolvedConnection_ReturnsNotFoundError(t *testing.T) {
	resolver := &fakeConnectionResolver{byConnectionID: map[string]domain.DevServer{}}
	agent := &fakeDevServerAgentClient{}
	uc := NewRelayStream(resolver, agent)

	ctx := withTenant(context.Background(), "tenant-1")
	err := uc.Execute(ctx, RelayStreamInput{ConnectionID: "unknown-conn", Method: "git.execStream"}, func(map[string]any) error { return nil })
	if err == nil {
		t.Fatal("expected an error when the connectionId doesn't resolve")
	}
	if len(agent.execStreamCalls) != 0 {
		t.Error("expected no relay to the agent when the connectionId doesn't resolve")
	}
}

// TestRelayStream_ForwardsFramesInOrder is the regression guard on Execute's
// core contract: every frame the agent emits reaches sink, in order, and
// Execute returns once the agent closes the channel (its own terminal-frame
// signal, per devserveragent.Client.ExecStream's doc comment).
func TestRelayStream_ForwardsFramesInOrder(t *testing.T) {
	ds, err := domain.NewDevServer("ds1", "tenant-1", "10.0.0.5", domain.ConnectionModeRelayWebSocket, "")
	if err != nil {
		t.Fatalf("building dev server: %v", err)
	}
	resolver := &fakeConnectionResolver{byConnectionID: map[string]domain.DevServer{"conn-1": ds}}
	frames := make(chan map[string]any, 3)
	frames <- map[string]any{"type": "stream.chunk", "line": "Enumerating objects"}
	frames <- map[string]any{"type": "stream.chunk", "line": "Counting objects"}
	frames <- map[string]any{"type": "stream.end", "exitCode": float64(0)}
	close(frames)
	agent := &fakeDevServerAgentClient{execStreamFrames: frames}
	uc := NewRelayStream(resolver, agent)

	var got []map[string]any
	ctx := withTenant(context.Background(), "tenant-1")
	err = uc.Execute(ctx, RelayStreamInput{ConnectionID: "conn-1", Method: "git.execStream"}, func(f map[string]any) error {
		got = append(got, f)
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("want 3 frames delivered in order, got %d: %+v", len(got), got)
	}
	if got[0]["line"] != "Enumerating objects" || got[2]["type"] != "stream.end" {
		t.Errorf("unexpected frame order/content: %+v", got)
	}
	if len(agent.execStreamCalls) != 1 || agent.execStreamCalls[0] != "git.execStream" {
		t.Fatalf("expected exactly one git.execStream relay call, got %v", agent.execStreamCalls)
	}
	if !agent.execStreamUnsubscribed {
		t.Error("expected unsubscribe to be called")
	}
}

func TestRelayStream_AgentFailurePropagates(t *testing.T) {
	ds, err := domain.NewDevServer("ds1", "tenant-1", "10.0.0.5", domain.ConnectionModeRelayWebSocket, "")
	if err != nil {
		t.Fatalf("building dev server: %v", err)
	}
	resolver := &fakeConnectionResolver{byConnectionID: map[string]domain.DevServer{"conn-1": ds}}
	agent := &fakeDevServerAgentClient{execStreamErr: errors.New("devserveragent: not connected")}
	uc := NewRelayStream(resolver, agent)

	ctx := withTenant(context.Background(), "tenant-1")
	err = uc.Execute(ctx, RelayStreamInput{ConnectionID: "conn-1", Method: "git.execStream"}, func(map[string]any) error { return nil })
	if err == nil {
		t.Fatal("expected the agent's error to propagate")
	}
}

// TestRelayStream_SinkErrorAbortsAndUnsubscribes guards that a sink failure
// (e.g. the gRPC stream.Send in adapter/grpc failing because the client
// disconnected) stops Execute promptly and still releases the subscription.
func TestRelayStream_SinkErrorAbortsAndUnsubscribes(t *testing.T) {
	ds, err := domain.NewDevServer("ds1", "tenant-1", "10.0.0.5", domain.ConnectionModeRelayWebSocket, "")
	if err != nil {
		t.Fatalf("building dev server: %v", err)
	}
	resolver := &fakeConnectionResolver{byConnectionID: map[string]domain.DevServer{"conn-1": ds}}
	frames := make(chan map[string]any, 2)
	frames <- map[string]any{"type": "stream.chunk", "line": "first"}
	frames <- map[string]any{"type": "stream.chunk", "line": "second"}
	agent := &fakeDevServerAgentClient{execStreamFrames: frames}
	uc := NewRelayStream(resolver, agent)

	wantErr := errors.New("client disconnected")
	calls := 0
	ctx := withTenant(context.Background(), "tenant-1")
	err = uc.Execute(ctx, RelayStreamInput{ConnectionID: "conn-1", Method: "git.execStream"}, func(map[string]any) error {
		calls++
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("want sink error to propagate unmodified, got %v", err)
	}
	if calls != 1 {
		t.Errorf("expected Execute to stop after the first sink error, got %d calls", calls)
	}
	if !agent.execStreamUnsubscribed {
		t.Error("expected unsubscribe to be called even on a sink error")
	}
}
