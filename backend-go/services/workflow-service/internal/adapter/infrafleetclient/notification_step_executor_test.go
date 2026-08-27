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

func TestNotificationExecutor_SuccessfulRelayProducesCompletedStepResult(t *testing.T) {
	var gotRequest *infrafleetv1.RelayRequest
	fake := &fakeInfraFleetClient{
		relayFunc: func(_ context.Context, in *infrafleetv1.RelayRequest, _ ...grpc.CallOption) (*infrafleetv1.RelayResponse, error) {
			gotRequest = in
			return &infrafleetv1.RelayResponse{ResultJson: `{}`}, nil
		},
	}
	exec := NewNotificationExecutor(fake)
	ctx := withTenantContext(context.Background(), "tenant-1")

	cfg, _ := json.Marshal(domain.NotificationStepConfig{ConnectionID: "conn-1", Channel: "#builds", Message: "workflow finished"})
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
	if gotRequest.Method != notificationSendMethod {
		t.Errorf("expected method %q, got %q", notificationSendMethod, gotRequest.Method)
	}
}

func TestNotificationExecutor_RelayResultErrorProducesFailedStepResult(t *testing.T) {
	fake := &fakeInfraFleetClient{
		relayFunc: func(_ context.Context, _ *infrafleetv1.RelayRequest, _ ...grpc.CallOption) (*infrafleetv1.RelayResponse, error) {
			result, _ := json.Marshal(map[string]any{"error": "channel not found"})
			return &infrafleetv1.RelayResponse{ResultJson: string(result)}, nil
		},
	}
	exec := NewNotificationExecutor(fake)
	ctx := withTenantContext(context.Background(), "tenant-1")

	cfg, _ := json.Marshal(domain.NotificationStepConfig{ConnectionID: "conn-1", Channel: "#nope", Message: "hi"})
	result, err := exec.Execute(ctx, string(cfg))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != domain.ResultStatusFailed {
		t.Errorf("expected failed status, got %v", result.Status)
	}
}

func TestNotificationExecutor_RelayErrorPropagates(t *testing.T) {
	fake := &fakeInfraFleetClient{
		relayFunc: func(_ context.Context, _ *infrafleetv1.RelayRequest, _ ...grpc.CallOption) (*infrafleetv1.RelayResponse, error) {
			return nil, errors.New("dev server unreachable")
		},
	}
	exec := NewNotificationExecutor(fake)
	ctx := withTenantContext(context.Background(), "tenant-1")

	cfg, _ := json.Marshal(domain.NotificationStepConfig{ConnectionID: "conn-1", Channel: "#builds", Message: "hi"})
	_, err := exec.Execute(ctx, string(cfg))
	if err == nil {
		t.Fatal("expected the relay error to propagate")
	}
}
