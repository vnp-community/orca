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
	"github.com/stablyai/orca-go/services/workflow-service/internal/usecase"
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

// fakeAgentProfileResolver is an in-memory usecase.ProfileResolver.
type fakeAgentProfileResolver struct {
	settings map[string]any
	err      error
	calls    []string // userIDs, in call order
}

func (f *fakeAgentProfileResolver) GetResolvedProfile(ctx context.Context, userID string) (map[string]any, error) {
	f.calls = append(f.calls, userID)
	if f.err != nil {
		return nil, f.err
	}
	return f.settings, nil
}

// fakeAgentProjectContextResolver is an in-memory usecase.ProjectContextResolver.
type fakeAgentProjectContextResolver struct {
	ctx   usecase.ProjectContext
	err   error
	calls []string // projectIDs, in call order
}

func (f *fakeAgentProjectContextResolver) GetProjectContext(ctx context.Context, projectID string) (usecase.ProjectContext, error) {
	f.calls = append(f.calls, projectID)
	if f.err != nil {
		return usecase.ProjectContext{}, f.err
	}
	return f.ctx, nil
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
	exec := NewAgentExecutor(fake, &fakeAgentProfileResolver{}, &fakeAgentProjectContextResolver{})
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
	if gotRequest.Method != agentExecPromptMethod {
		t.Errorf("expected method %q, got %q", agentExecPromptMethod, gotRequest.Method)
	}
}

// TestAgentExecutor_MethodIsAlwaysAgentExecPrompt is the hard regression
// test for the stale "agent.exec" bug this task fixes — asserted
// unconditionally, both with and without a UserID in the step config.
func TestAgentExecutor_MethodIsAlwaysAgentExecPrompt(t *testing.T) {
	for _, cfg := range []domain.AgentStepConfig{
		{ConnectionID: "conn-1", Prompt: "p"},
		{ConnectionID: "conn-1", Prompt: "p", UserID: "user-1"},
	} {
		var gotMethod string
		fake := &fakeInfraFleetClient{
			relayFunc: func(_ context.Context, in *infrafleetv1.RelayRequest, _ ...grpc.CallOption) (*infrafleetv1.RelayResponse, error) {
				gotMethod = in.Method
				result, _ := json.Marshal(map[string]any{"exitCode": 0})
				return &infrafleetv1.RelayResponse{ResultJson: string(result)}, nil
			},
		}
		exec := NewAgentExecutor(fake, &fakeAgentProfileResolver{settings: map[string]any{}}, &fakeAgentProjectContextResolver{})
		ctx := withTenantContext(context.Background(), "tenant-1")

		cfgJSON, _ := json.Marshal(cfg)
		if _, err := exec.Execute(ctx, string(cfgJSON)); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if gotMethod != agentExecPromptMethod {
			t.Errorf("expected method %q, got %q", agentExecPromptMethod, gotMethod)
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
	exec := NewAgentExecutor(fake, &fakeAgentProfileResolver{}, &fakeAgentProjectContextResolver{})
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
	exec := NewAgentExecutor(fake, &fakeAgentProfileResolver{}, &fakeAgentProjectContextResolver{})
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
	exec := NewAgentExecutor(fake, &fakeAgentProfileResolver{}, &fakeAgentProjectContextResolver{})
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
	exec := NewAgentExecutor(fake, &fakeAgentProfileResolver{}, &fakeAgentProjectContextResolver{})

	cfg, _ := json.Marshal(domain.AgentStepConfig{ConnectionID: "conn-1", Prompt: "do the thing"})
	_, err := exec.Execute(context.Background(), string(cfg))
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

// TestAgentExecutor_EmptyUserID_LegacyPassthrough asserts that when
// cfg.UserID is empty, the profile-aware path is skipped entirely — no
// ProfileResolver call, params.Env/Model stay unset.
func TestAgentExecutor_EmptyUserID_LegacyPassthrough(t *testing.T) {
	var gotParams agentExecPromptParams
	fake := &fakeInfraFleetClient{
		relayFunc: func(_ context.Context, in *infrafleetv1.RelayRequest, _ ...grpc.CallOption) (*infrafleetv1.RelayResponse, error) {
			_ = json.Unmarshal([]byte(in.ParamsJson), &gotParams)
			result, _ := json.Marshal(map[string]any{"exitCode": 0})
			return &infrafleetv1.RelayResponse{ResultJson: string(result)}, nil
		},
	}
	profiles := &fakeAgentProfileResolver{}
	exec := NewAgentExecutor(fake, profiles, &fakeAgentProjectContextResolver{})
	ctx := withTenantContext(context.Background(), "tenant-1")

	cfg, _ := json.Marshal(domain.AgentStepConfig{ConnectionID: "conn-1", Prompt: "do the thing"})
	if _, err := exec.Execute(ctx, string(cfg)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(profiles.calls) != 0 {
		t.Errorf("expected no ProfileResolver call for an empty UserID, got %v", profiles.calls)
	}
	if len(gotParams.Env) != 0 {
		t.Errorf("expected no env for legacy passthrough, got %+v", gotParams.Env)
	}
	if gotParams.Model != "" {
		t.Errorf("expected no model for legacy passthrough, got %q", gotParams.Model)
	}
}

// TestAgentExecutor_UserIDSet_ResolvesProfileAndPopulatesEnv asserts the
// profile-aware path: ProfileResolver called with the right userID, env
// populated, params.Model equals the raw resolved model string (not a
// binary name).
func TestAgentExecutor_UserIDSet_ResolvesProfileAndPopulatesEnv(t *testing.T) {
	var gotParams agentExecPromptParams
	fake := &fakeInfraFleetClient{
		relayFunc: func(_ context.Context, in *infrafleetv1.RelayRequest, _ ...grpc.CallOption) (*infrafleetv1.RelayResponse, error) {
			_ = json.Unmarshal([]byte(in.ParamsJson), &gotParams)
			result, _ := json.Marshal(map[string]any{"exitCode": 0})
			return &infrafleetv1.RelayResponse{ResultJson: string(result)}, nil
		},
	}
	profiles := &fakeAgentProfileResolver{settings: map[string]any{
		"agent": map[string]any{"preferredModel": "claude-opus-4-5"},
	}}
	exec := NewAgentExecutor(fake, profiles, &fakeAgentProjectContextResolver{})
	ctx := withTenantContext(context.Background(), "tenant-1")

	cfg, _ := json.Marshal(domain.AgentStepConfig{ConnectionID: "conn-1", Prompt: "do the thing", UserID: "user-42"})
	if _, err := exec.Execute(ctx, string(cfg)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(profiles.calls) != 1 || profiles.calls[0] != "user-42" {
		t.Errorf("expected ProfileResolver called with user-42, got %v", profiles.calls)
	}
	if gotParams.Model != "claude-opus-4-5" {
		t.Errorf("expected raw resolved model string, got %q", gotParams.Model)
	}
	if gotParams.Env == nil {
		t.Error("expected env to be populated")
	}
}

// TestAgentExecutor_ProjectContextResolverFailure_BestEffort asserts that a
// ProjectContextResolver failure never blocks the agent spawn — the relay
// call still happens, with InitFile empty.
func TestAgentExecutor_ProjectContextResolverFailure_BestEffort(t *testing.T) {
	var gotParams agentExecPromptParams
	var relayCalled bool
	fake := &fakeInfraFleetClient{
		relayFunc: func(_ context.Context, in *infrafleetv1.RelayRequest, _ ...grpc.CallOption) (*infrafleetv1.RelayResponse, error) {
			relayCalled = true
			_ = json.Unmarshal([]byte(in.ParamsJson), &gotParams)
			result, _ := json.Marshal(map[string]any{"exitCode": 0})
			return &infrafleetv1.RelayResponse{ResultJson: string(result)}, nil
		},
	}
	profiles := &fakeAgentProfileResolver{settings: map[string]any{}}
	projects := &fakeAgentProjectContextResolver{err: errors.New("project-service unreachable")}
	exec := NewAgentExecutor(fake, profiles, projects)
	ctx := withTenantContext(context.Background(), "tenant-1")

	cfg, _ := json.Marshal(domain.AgentStepConfig{ConnectionID: "conn-1", Prompt: "do the thing", UserID: "user-1", ProjectID: "proj-1"})
	if _, err := exec.Execute(ctx, string(cfg)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !relayCalled {
		t.Fatal("expected the relay/spawn call to still happen despite the project-context failure")
	}
	if gotParams.InitFile != "" {
		t.Errorf("expected empty InitFile on project-context failure, got %q", gotParams.InitFile)
	}
}
