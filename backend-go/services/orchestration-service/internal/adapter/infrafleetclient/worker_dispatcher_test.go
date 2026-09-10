package infrafleetclient

import (
	"context"
	"testing"

	"google.golang.org/grpc"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	"github.com/stablyai/orca-go/services/orchestration-service/internal/domain"
)

// fakeInfraFleetServiceClient implements infrafleetv1.InfraFleetServiceClient
// directly (no bufconn/wire-level gRPC) — mirrors task-service's
// simple_executor_test.go fake of the same name/shape.
type fakeInfraFleetServiceClient struct {
	infrafleetv1.InfraFleetServiceClient // embed: panics on any unimplemented method, intentional for these tests

	relayResp   *infrafleetv1.RelayResponse
	relayErr    error
	gotRelay    *infrafleetv1.RelayRequest
	relayCalled bool
}

func (f *fakeInfraFleetServiceClient) Relay(_ context.Context, in *infrafleetv1.RelayRequest, _ ...grpc.CallOption) (*infrafleetv1.RelayResponse, error) {
	f.relayCalled = true
	f.gotRelay = in
	if f.relayErr != nil {
		return nil, f.relayErr
	}
	return f.relayResp, nil
}

func TestWorkerDispatcher_Dispatch_HappyPath(t *testing.T) {
	fake := &fakeInfraFleetServiceClient{
		relayResp: &infrafleetv1.RelayResponse{ResultJson: `{"stdout":"ok","exitCode":0,"timedOut":false}`},
	}
	d := NewWorkerDispatcher(fake)

	task := domain.OrchestrationTask{
		ID:        "task-1",
		TaskTitle: "do the thing",
		Spec:      []byte(`{"connectionId":"conn-1","worktreePath":"/wt"}`),
	}
	if err := d.Dispatch(context.Background(), "tenant-1", task, "worker:task-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !fake.relayCalled {
		t.Fatal("expected Relay to be called")
	}
	if fake.gotRelay.ConnectionId != "conn-1" || fake.gotRelay.Method != "agent.execPrompt" {
		t.Errorf("unexpected relay request: %+v", fake.gotRelay)
	}
}

func TestWorkerDispatcher_Dispatch_MissingConnectionID(t *testing.T) {
	fake := &fakeInfraFleetServiceClient{}
	d := NewWorkerDispatcher(fake)

	task := domain.OrchestrationTask{ID: "task-1", TaskTitle: "x", Spec: []byte(`{"worktreePath":"/wt"}`)}
	if err := d.Dispatch(context.Background(), "tenant-1", task, "worker:task-1"); err == nil {
		t.Fatal("expected an error for a missing connectionId")
	}
	if fake.relayCalled {
		t.Error("expected Relay NOT to be called when connectionId is missing")
	}
}

func TestWorkerDispatcher_Dispatch_NonZeroExitCode(t *testing.T) {
	fake := &fakeInfraFleetServiceClient{
		relayResp: &infrafleetv1.RelayResponse{ResultJson: `{"stdout":"","stderr":"boom","exitCode":1,"timedOut":false}`},
	}
	d := NewWorkerDispatcher(fake)

	task := domain.OrchestrationTask{ID: "task-1", TaskTitle: "x", Spec: []byte(`{"connectionId":"conn-1"}`)}
	err := d.Dispatch(context.Background(), "tenant-1", task, "worker:task-1")
	if err == nil {
		t.Fatal("expected an error for a non-zero exit code")
	}
}

func TestWorkerDispatcher_Dispatch_TimedOut(t *testing.T) {
	fake := &fakeInfraFleetServiceClient{
		relayResp: &infrafleetv1.RelayResponse{ResultJson: `{"exitCode":0,"timedOut":true}`},
	}
	d := NewWorkerDispatcher(fake)

	task := domain.OrchestrationTask{ID: "task-1", TaskTitle: "x", Spec: []byte(`{"connectionId":"conn-1"}`)}
	if err := d.Dispatch(context.Background(), "tenant-1", task, "worker:task-1"); err == nil {
		t.Fatal("expected an error for a timed-out dispatch")
	}
}
