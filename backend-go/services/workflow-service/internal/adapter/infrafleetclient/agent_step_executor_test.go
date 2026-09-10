package infrafleetclient

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"google.golang.org/grpc"

	"github.com/stablyai/orca-go/common/tenant"
	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
)

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
	exec := NewAgentExecutor(fake, newPassthroughServerResolver(), newNoopProviderResolver())
	ctx := withTenantContext(context.Background(), "tenant-1")

	cfg, _ := json.Marshal(domain.AgentStepConfig{ConnectionID: "server:conn-1", Prompt: "do the thing", WorktreePath: "/wt"})
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

func TestAgentExecutor_RelayUsesExecPromptMethodAndParamsShape(t *testing.T) {
	var gotRequest *infrafleetv1.RelayRequest
	fake := &fakeInfraFleetClient{
		relayFunc: func(_ context.Context, in *infrafleetv1.RelayRequest, _ ...grpc.CallOption) (*infrafleetv1.RelayResponse, error) {
			gotRequest = in
			result, _ := json.Marshal(map[string]any{"exitCode": 0, "stdout": "done"})
			return &infrafleetv1.RelayResponse{ResultJson: string(result)}, nil
		},
	}
	exec := NewAgentExecutor(fake, newPassthroughServerResolver(), newNoopProviderResolver())
	ctx := withTenantContext(context.Background(), "tenant-1")

	cfg, _ := json.Marshal(domain.AgentStepConfig{
		ConnectionID: "server:conn-1",
		Prompt:       "do the thing",
		WorktreePath: "/wt",
		TrustPreset:  "yolo",
	})
	if _, err := exec.Execute(ctx, string(cfg)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotRequest == nil {
		t.Fatal("expected Relay to be called")
	}

	// This is the exact class of bug that shipped undetected before: a
	// compiling struct literal that silently didn't match the real
	// handler's contract. Assert both the method name and the params JSON
	// shape explicitly so a regression here fails loudly.
	if gotRequest.Method != "agent.execPrompt" {
		t.Errorf("expected method %q, got %q", "agent.execPrompt", gotRequest.Method)
	}

	var params map[string]any
	if err := json.Unmarshal([]byte(gotRequest.ParamsJson), &params); err != nil {
		t.Fatalf("failed to unmarshal params_json: %v", err)
	}
	if params["prompt"] != "do the thing" {
		t.Errorf("expected prompt %q, got %v", "do the thing", params["prompt"])
	}
	if params["worktreePath"] != "/wt" {
		t.Errorf("expected worktreePath %q, got %v", "/wt", params["worktreePath"])
	}
	if params["trustPreset"] != "yolo" {
		t.Errorf("expected trustPreset %q, got %v", "yolo", params["trustPreset"])
	}
	// Model/AccountID/StepID/Env are new, still-unpopulated fields — they
	// must marshal as absent (omitempty), not as present-but-empty, since
	// agent.execPrompt's real handler treats absence as "use the default"
	// rather than an explicit empty override.
	for _, absentKey := range []string{"model", "accountId", "stepId", "env"} {
		if _, present := params[absentKey]; present {
			t.Errorf("expected %q to be absent from params_json when unset, got %v", absentKey, params[absentKey])
		}
	}
}

func TestAgentExecutor_NonZeroExitCodeProducesFailedStepResult(t *testing.T) {
	fake := &fakeInfraFleetClient{
		relayFunc: func(_ context.Context, _ *infrafleetv1.RelayRequest, _ ...grpc.CallOption) (*infrafleetv1.RelayResponse, error) {
			result, _ := json.Marshal(map[string]any{"exitCode": 1, "stderr": "boom"})
			return &infrafleetv1.RelayResponse{ResultJson: string(result)}, nil
		},
	}
	exec := NewAgentExecutor(fake, newPassthroughServerResolver(), newNoopProviderResolver())
	ctx := withTenantContext(context.Background(), "tenant-1")

	cfg, _ := json.Marshal(domain.AgentStepConfig{ConnectionID: "server:conn-1", Prompt: "do the thing"})
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
	exec := NewAgentExecutor(fake, newPassthroughServerResolver(), newNoopProviderResolver())
	ctx := withTenantContext(context.Background(), "tenant-1")

	cfg, _ := json.Marshal(domain.AgentStepConfig{ConnectionID: "server:conn-1", Prompt: "do the thing"})
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
	exec := NewAgentExecutor(fake, newPassthroughServerResolver(), newNoopProviderResolver())
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
	exec := NewAgentExecutor(fake, newPassthroughServerResolver(), newNoopProviderResolver())

	cfg, _ := json.Marshal(domain.AgentStepConfig{ConnectionID: "server:conn-1", Prompt: "do the thing"})
	_, err := exec.Execute(context.Background(), string(cfg))
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

// TestAgentExecutor_MalformedTargetSpecSurfacesAsStepFailureNotPanic covers
// TASK-WF-002-03's test plan item: a malformed ConnectionID must be a clear
// step failure, never a panic or an opaque relay error.
func TestAgentExecutor_MalformedTargetSpecSurfacesAsStepFailureNotPanic(t *testing.T) {
	fake := &fakeInfraFleetClient{
		relayFunc: func(_ context.Context, _ *infrafleetv1.RelayRequest, _ ...grpc.CallOption) (*infrafleetv1.RelayResponse, error) {
			t.Fatal("Relay should not be called for an unparseable target spec")
			return nil, nil
		},
	}
	exec := NewAgentExecutor(fake, newPassthroughServerResolver(), newNoopProviderResolver())
	ctx := withTenantContext(context.Background(), "tenant-1")

	cfg, _ := json.Marshal(domain.AgentStepConfig{ConnectionID: "not-a-valid-target-spec", Prompt: "do the thing"})
	_, err := exec.Execute(ctx, string(cfg))
	if !errors.Is(err, domain.ErrUnknownTargetKind) {
		t.Fatalf("expected ErrUnknownTargetKind wrapped in the returned error, got %v", err)
	}
}

// TestAgentExecutor_ResolvesTargetKindProject covers a 2-different-target
// dispatch scenario end to end: a "project:<id>" spec resolves through
// ServerResolver's ProjectClient to that project's bound dev server, and
// the resolved id (not the raw spec string) is what reaches Relay.
func TestAgentExecutor_ResolvesTargetKindProject(t *testing.T) {
	var gotRequest *infrafleetv1.RelayRequest
	fake := &fakeInfraFleetClient{
		relayFunc: func(_ context.Context, in *infrafleetv1.RelayRequest, _ ...grpc.CallOption) (*infrafleetv1.RelayResponse, error) {
			gotRequest = in
			result, _ := json.Marshal(map[string]any{"exitCode": 0})
			return &infrafleetv1.RelayResponse{ResultJson: string(result)}, nil
		},
	}
	serverResolver := usecaseServerResolverWithProject("ds-bound-to-project")
	exec := NewAgentExecutor(fake, serverResolver, newNoopProviderResolver())
	ctx := withTenantContext(context.Background(), "tenant-1")

	cfg, _ := json.Marshal(domain.AgentStepConfig{ConnectionID: "project:proj-1", Prompt: "do the thing"})
	if _, err := exec.Execute(ctx, string(cfg)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotRequest.ConnectionId != "ds-bound-to-project" {
		t.Errorf("expected the resolved dev server id, got %q", gotRequest.ConnectionId)
	}
}

// TestAgentExecutor_ResolvesTargetKindFleetTag mirrors the project-target
// test above for "fleet:tag:<tag>" — resolved via InfraFleetPicker.
func TestAgentExecutor_ResolvesTargetKindFleetTag(t *testing.T) {
	var gotRequest *infrafleetv1.RelayRequest
	fake := &fakeInfraFleetClient{
		relayFunc: func(_ context.Context, in *infrafleetv1.RelayRequest, _ ...grpc.CallOption) (*infrafleetv1.RelayResponse, error) {
			gotRequest = in
			result, _ := json.Marshal(map[string]any{"exitCode": 0})
			return &infrafleetv1.RelayResponse{ResultJson: string(result)}, nil
		},
	}
	serverResolver := usecaseServerResolverWithFleetTag("conn-picked-by-tag")
	exec := NewAgentExecutor(fake, serverResolver, newNoopProviderResolver())
	ctx := withTenantContext(context.Background(), "tenant-1")

	cfg, _ := json.Marshal(domain.AgentStepConfig{ConnectionID: "fleet:tag:gpu-fleet", Prompt: "do the thing"})
	if _, err := exec.Execute(ctx, string(cfg)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotRequest.ConnectionId != "conn-picked-by-tag" {
		t.Errorf("expected the tag-picked connection id, got %q", gotRequest.ConnectionId)
	}
}

// TestAgentExecutor_ExplicitProviderPinReachesRelayParams confirms the
// resolved provider's AccountID/Model flow all the way into the relay
// call's params — closing the loop TASK-WF-001-01 opened (widened
// agentExecParams) and TASK-WF-002-02 built (ProviderResolver).
func TestAgentExecutor_ExplicitProviderPinReachesRelayParams(t *testing.T) {
	var gotRequest *infrafleetv1.RelayRequest
	fake := &fakeInfraFleetClient{
		relayFunc: func(_ context.Context, in *infrafleetv1.RelayRequest, _ ...grpc.CallOption) (*infrafleetv1.RelayResponse, error) {
			gotRequest = in
			result, _ := json.Marshal(map[string]any{"exitCode": 0})
			return &infrafleetv1.RelayResponse{ResultJson: string(result)}, nil
		},
	}
	providerResolver := usecaseProviderResolverWithActiveAccount()
	exec := NewAgentExecutor(fake, newPassthroughServerResolver(), providerResolver)
	ctx := withTenantContext(context.Background(), "tenant-1")

	cfg, _ := json.Marshal(domain.AgentStepConfig{
		ConnectionID: "server:conn-1",
		Prompt:       "do the thing",
		Provider:     &domain.ProviderPin{AccountID: "acct-pinned", Model: "claude-3-opus"},
	})
	if _, err := exec.Execute(ctx, string(cfg)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var params map[string]any
	if err := json.Unmarshal([]byte(gotRequest.ParamsJson), &params); err != nil {
		t.Fatalf("failed to unmarshal params_json: %v", err)
	}
	if params["accountId"] != "acct-pinned" {
		t.Errorf("expected accountId acct-pinned, got %v", params["accountId"])
	}
	if params["model"] != "claude-3-opus" {
		t.Errorf("expected model claude-3-opus, got %v", params["model"])
	}
}
