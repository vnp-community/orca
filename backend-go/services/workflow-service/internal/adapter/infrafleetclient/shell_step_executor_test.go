package infrafleetclient

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"google.golang.org/grpc"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
)

func TestShellExecutor_SuccessfulRelayProducesCompletedStepResult(t *testing.T) {
	var gotRequest *infrafleetv1.RelayRequest
	fake := &fakeInfraFleetClient{
		relayFunc: func(_ context.Context, in *infrafleetv1.RelayRequest, _ ...grpc.CallOption) (*infrafleetv1.RelayResponse, error) {
			gotRequest = in
			result, _ := json.Marshal(map[string]any{"exitCode": 0, "stdout": "hi\n"})
			return &infrafleetv1.RelayResponse{ResultJson: string(result)}, nil
		},
	}
	exec := NewShellExecutor(fake)
	ctx := withTenantContext(context.Background(), "tenant-1")

	cfg, _ := json.Marshal(domain.ShellStepConfig{ConnectionID: "conn-1", Script: "echo hi"})
	result, err := exec.Execute(ctx, string(cfg))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != domain.ResultStatusCompleted {
		t.Errorf("expected completed status, got %v", result.Status)
	}
	if gotRequest == nil {
		t.Fatal("expected Relay to be called")
	}
	if gotRequest.Method != shellExecMethod {
		t.Errorf("expected method %q, got %q", shellExecMethod, gotRequest.Method)
	}
}

func TestShellExecutor_NonZeroExitCodeProducesFailedStepResult(t *testing.T) {
	fake := &fakeInfraFleetClient{
		relayFunc: func(_ context.Context, _ *infrafleetv1.RelayRequest, _ ...grpc.CallOption) (*infrafleetv1.RelayResponse, error) {
			result, _ := json.Marshal(map[string]any{"exitCode": 127, "stderr": "command not found"})
			return &infrafleetv1.RelayResponse{ResultJson: string(result)}, nil
		},
	}
	exec := NewShellExecutor(fake)
	ctx := withTenantContext(context.Background(), "tenant-1")

	cfg, _ := json.Marshal(domain.ShellStepConfig{ConnectionID: "conn-1", Script: "not-a-command"})
	result, err := exec.Execute(ctx, string(cfg))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != domain.ResultStatusFailed {
		t.Errorf("expected failed status, got %v", result.Status)
	}
}

func TestShellExecutor_RelayErrorPropagates(t *testing.T) {
	fake := &fakeInfraFleetClient{
		relayFunc: func(_ context.Context, _ *infrafleetv1.RelayRequest, _ ...grpc.CallOption) (*infrafleetv1.RelayResponse, error) {
			return nil, errors.New("dev server unreachable")
		},
	}
	exec := NewShellExecutor(fake)
	ctx := withTenantContext(context.Background(), "tenant-1")

	cfg, _ := json.Marshal(domain.ShellStepConfig{ConnectionID: "conn-1", Script: "echo hi"})
	_, err := exec.Execute(ctx, string(cfg))
	if err == nil {
		t.Fatal("expected the relay error to propagate")
	}
}
