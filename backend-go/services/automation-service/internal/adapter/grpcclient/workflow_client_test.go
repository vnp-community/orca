package grpcclient

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/stablyai/orca-go/common/grpcmw"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/automation-service/internal/domain"
	"github.com/stablyai/orca-go/services/automation-service/internal/usecase"

	workflowv1 "github.com/stablyai/orca-go/proto/gen/go/orca/workflow/v1"
)

// fakeWorkflowServiceClient embeds the generated client interface and
// overrides only ExecuteAdHocStep — the embed-and-override pattern this
// repo's other grpcclient fakes use (e.g. git-gateway-service's
// fakeGitGatewayServiceClient) — so a proto addition never breaks this
// test file.
type fakeWorkflowServiceClient struct {
	workflowv1.WorkflowServiceClient
	executeAdHocStepFunc func(ctx context.Context, in *workflowv1.ExecuteAdHocStepRequest, opts ...grpc.CallOption) (*workflowv1.ExecuteAdHocStepResponse, error)
}

func (f *fakeWorkflowServiceClient) ExecuteAdHocStep(ctx context.Context, in *workflowv1.ExecuteAdHocStepRequest, opts ...grpc.CallOption) (*workflowv1.ExecuteAdHocStepResponse, error) {
	return f.executeAdHocStepFunc(ctx, in, opts...)
}

// TestWorkflowClient_ExecuteAdHocStep_ForwardsTenantIDAsOutgoingMetadata is
// the regression guard for the bug TASK-220's E2E test found: workflow-
// service's own tenant.RequireTenantID(ctx) is fed only by its inbound
// grpcmw.TenantExtractionInterceptor reading incoming gRPC metadata (never
// by the ExecuteAdHocStepRequest.TenantId wire field, which no
// workflow-service handler reads) — so a real RunNow dispatch to a real
// workflow-service always failed closed with WORKFLOW_NO_TENANT until
// WorkflowClient started forwarding tenant ID via outgoing metadata too,
// mirroring workflow-service's own internal/adapter/infrafleetclient
// withTenantMetadata precedent for its downstream hop.
func TestWorkflowClient_ExecuteAdHocStep_ForwardsTenantIDAsOutgoingMetadata(t *testing.T) {
	var gotMD metadata.MD
	fake := &fakeWorkflowServiceClient{
		executeAdHocStepFunc: func(ctx context.Context, in *workflowv1.ExecuteAdHocStepRequest, _ ...grpc.CallOption) (*workflowv1.ExecuteAdHocStepResponse, error) {
			gotMD, _ = metadata.FromOutgoingContext(ctx)
			return &workflowv1.ExecuteAdHocStepResponse{Result: &workflowv1.StepResult{Status: "succeeded"}}, nil
		},
	}
	client := &WorkflowClient{client: fake}
	ctx := tenant.WithTenantID(context.Background(), "tenant-1")

	if _, err := client.ExecuteAdHocStep(ctx, usecase.ExecuteAdHocStepInput{
		TenantID:       "tenant-1",
		StepType:       domain.StepTypeShell,
		StepConfigJSON: `{}`,
		RequestID:      "req-1",
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := gotMD.Get(grpcmw.MetadataTenantID)
	if len(got) != 1 || got[0] != "tenant-1" {
		t.Fatalf("expected outgoing metadata %q=%q, got %v", grpcmw.MetadataTenantID, "tenant-1", got)
	}
}

// TestWorkflowClient_ExecuteAdHocStep_NoTenantInContext_ReturnsErrorWithoutCallingClient
// locks in fail-closed behavior: a caller that forgot to put a tenant ID on
// ctx never reaches the wire at all.
func TestWorkflowClient_ExecuteAdHocStep_NoTenantInContext_ReturnsErrorWithoutCallingClient(t *testing.T) {
	called := false
	fake := &fakeWorkflowServiceClient{
		executeAdHocStepFunc: func(ctx context.Context, in *workflowv1.ExecuteAdHocStepRequest, _ ...grpc.CallOption) (*workflowv1.ExecuteAdHocStepResponse, error) {
			called = true
			return &workflowv1.ExecuteAdHocStepResponse{}, nil
		},
	}
	client := &WorkflowClient{client: fake}

	_, err := client.ExecuteAdHocStep(context.Background(), usecase.ExecuteAdHocStepInput{StepType: domain.StepTypeShell})
	if err == nil {
		t.Fatal("expected an error when no tenant is present on ctx")
	}
	if called {
		t.Error("expected the underlying gRPC client to never be called without a tenant in context")
	}
}

// TestWorkflowClient_ExecuteAdHocStep_PropagatesClientError confirms a real
// RPC-level error still surfaces to the caller once tenant forwarding
// succeeds.
func TestWorkflowClient_ExecuteAdHocStep_PropagatesClientError(t *testing.T) {
	wantErr := errors.New("boom")
	fake := &fakeWorkflowServiceClient{
		executeAdHocStepFunc: func(ctx context.Context, in *workflowv1.ExecuteAdHocStepRequest, _ ...grpc.CallOption) (*workflowv1.ExecuteAdHocStepResponse, error) {
			return nil, wantErr
		},
	}
	client := &WorkflowClient{client: fake}
	ctx := tenant.WithTenantID(context.Background(), "tenant-1")

	_, err := client.ExecuteAdHocStep(ctx, usecase.ExecuteAdHocStepInput{StepType: domain.StepTypeShell})
	if err == nil {
		t.Fatal("expected the underlying client error to propagate")
	}
}
