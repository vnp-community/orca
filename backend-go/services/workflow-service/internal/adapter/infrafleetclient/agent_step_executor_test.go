package infrafleetclient

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"google.golang.org/grpc"

	"github.com/stablyai/orca-go/common/tenant"
	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	"github.com/stablyai/orca-go/services/workflow-service/internal/adapter/serverresolver"
	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
	"github.com/stablyai/orca-go/services/workflow-service/internal/usecase"
)

// testResolver is the real serverresolver.New with nil project/infra
// clients — safe here because every fixture in this package's tests uses
// only the "connection:"/legacy-ConnectionID/empty Target shapes, none of
// which ever touch those clients (see resolver.Resolve's switch). Testing
// the RPC-calling shapes (server:/project:/fleet:tag:) is
// internal/adapter/serverresolver's own job.
func testResolver() usecase.ServerResolver {
	return serverresolver.New(nil, nil)
}

// fakeInfraFleetClient implements infrafleetv1.InfraFleetServiceClient
// directly — embedding the (nil) interface means any RPC this package's
// executors don't call panics loudly on the zero value rather than silently
// succeeding. "Fake the port, not the transport"
// (specs/backend-go/standards/testing-strategy.md); no bufconn needed since
// this package only ever calls the one Relay method.
type fakeInfraFleetClient struct {
	infrafleetv1.InfraFleetServiceClient
	relayFunc func(ctx context.Context, in *infrafleetv1.RelayRequest, opts ...grpc.CallOption) (*infrafleetv1.RelayResponse, error)
}

func (f *fakeInfraFleetClient) Relay(ctx context.Context, in *infrafleetv1.RelayRequest, opts ...grpc.CallOption) (*infrafleetv1.RelayResponse, error) {
	return f.relayFunc(ctx, in, opts...)
}

func withTenantContext(ctx context.Context, tenantID string) context.Context {
	return tenant.WithTenantID(ctx, tenantID)
}

func TestAgentExecutor_SuccessfulRelayProducesCompletedStepResult(t *testing.T) {
	var gotRequest *infrafleetv1.RelayRequest
	fake := &fakeInfraFleetClient{
		relayFunc: func(_ context.Context, in *infrafleetv1.RelayRequest, _ ...grpc.CallOption) (*infrafleetv1.RelayResponse, error) {
			gotRequest = in
			result, _ := json.Marshal(map[string]any{"exitCode": 0, "stdout": "done"})
			return &infrafleetv1.RelayResponse{ResultJson: string(result)}, nil
		},
	}
	exec := NewAgentExecutor(fake, testResolver())
	ctx := withTenantContext(context.Background(), "tenant-1")

	cfg, _ := json.Marshal(domain.AgentStepConfig{ConnectionID: "conn-1", Prompt: "do the thing", WorktreePath: "/wt"})
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
	if gotRequest.ConnectionId != "conn-1" {
		t.Errorf("expected connectionId conn-1, got %q", gotRequest.ConnectionId)
	}
	if gotRequest.Method != agentExecMethod {
		t.Errorf("expected method %q, got %q", agentExecMethod, gotRequest.Method)
	}
}

func TestAgentExecutor_NonZeroExitCodeProducesFailedStepResult(t *testing.T) {
	fake := &fakeInfraFleetClient{
		relayFunc: func(_ context.Context, _ *infrafleetv1.RelayRequest, _ ...grpc.CallOption) (*infrafleetv1.RelayResponse, error) {
			result, _ := json.Marshal(map[string]any{"exitCode": 1, "stderr": "boom"})
			return &infrafleetv1.RelayResponse{ResultJson: string(result)}, nil
		},
	}
	exec := NewAgentExecutor(fake, testResolver())
	ctx := withTenantContext(context.Background(), "tenant-1")

	cfg, _ := json.Marshal(domain.AgentStepConfig{ConnectionID: "conn-1", Prompt: "do the thing"})
	result, err := exec.Execute(ctx, string(cfg))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != domain.ResultStatusFailed {
		t.Errorf("expected failed status, got %v", result.Status)
	}
}

func TestAgentExecutor_RelayErrorPropagates(t *testing.T) {
	fake := &fakeInfraFleetClient{
		relayFunc: func(_ context.Context, _ *infrafleetv1.RelayRequest, _ ...grpc.CallOption) (*infrafleetv1.RelayResponse, error) {
			return nil, errors.New("dev server unreachable")
		},
	}
	exec := NewAgentExecutor(fake, testResolver())
	ctx := withTenantContext(context.Background(), "tenant-1")

	cfg, _ := json.Marshal(domain.AgentStepConfig{ConnectionID: "conn-1", Prompt: "do the thing"})
	_, err := exec.Execute(ctx, string(cfg))
	if err == nil {
		t.Fatal("expected the relay error to propagate")
	}
}

func TestAgentExecutor_MissingConnectionIDErrorsWithoutCallingRelay(t *testing.T) {
	fake := &fakeInfraFleetClient{
		relayFunc: func(_ context.Context, _ *infrafleetv1.RelayRequest, _ ...grpc.CallOption) (*infrafleetv1.RelayResponse, error) {
			t.Fatal("Relay should not be called without a connectionId")
			return nil, nil
		},
	}
	exec := NewAgentExecutor(fake, testResolver())
	ctx := withTenantContext(context.Background(), "tenant-1")

	cfg, _ := json.Marshal(domain.AgentStepConfig{Prompt: "do the thing"})
	_, err := exec.Execute(ctx, string(cfg))
	if err == nil {
		t.Fatal("expected an error for missing connectionId")
	}
}

func TestAgentExecutor_NoTenantInContextErrorsWithoutCallingRelay(t *testing.T) {
	fake := &fakeInfraFleetClient{
		relayFunc: func(_ context.Context, _ *infrafleetv1.RelayRequest, _ ...grpc.CallOption) (*infrafleetv1.RelayResponse, error) {
			t.Fatal("Relay should not be called without a tenant in context")
			return nil, nil
		},
	}
	exec := NewAgentExecutor(fake, testResolver())

	cfg, _ := json.Marshal(domain.AgentStepConfig{ConnectionID: "conn-1", Prompt: "do the thing"})
	_, err := exec.Execute(context.Background(), string(cfg))
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}
