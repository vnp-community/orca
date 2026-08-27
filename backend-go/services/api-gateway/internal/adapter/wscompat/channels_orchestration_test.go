package wscompat

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc"

	orchestrationv1 "github.com/stablyai/orca-go/proto/gen/go/orca/orchestration/v1"
)

type fakeOrchestrationClient struct {
	orchestrationv1.OrchestrationServiceClient

	getDispatchContextForTaskFunc func(ctx context.Context, in *orchestrationv1.GetDispatchContextForTaskRequest) (*orchestrationv1.GetDispatchContextForTaskResponse, error)
}

func (f *fakeOrchestrationClient) GetDispatchContextForTask(ctx context.Context, in *orchestrationv1.GetDispatchContextForTaskRequest, _ ...grpc.CallOption) (*orchestrationv1.GetDispatchContextForTaskResponse, error) {
	return f.getDispatchContextForTaskFunc(ctx, in)
}

func TestDispatchShowChannel_ReturnsAssigneeHandle(t *testing.T) {
	fake := &fakeOrchestrationClient{
		getDispatchContextForTaskFunc: func(ctx context.Context, in *orchestrationv1.GetDispatchContextForTaskRequest) (*orchestrationv1.GetDispatchContextForTaskResponse, error) {
			if in.GetOrchestrationTaskId() != "task-1" {
				t.Fatalf("want task-1, got %q", in.GetOrchestrationTaskId())
			}
			return &orchestrationv1.GetDispatchContextForTaskResponse{
				Dispatch: &orchestrationv1.DispatchContext{Id: "dc-1", Handle: "terminal-3", OrchestrationTaskId: "task-1"},
			}, nil
		},
	}
	r := NewRegistry()
	registerOrchestrationChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1", UserID: "u1"}, "orchestration.dispatchShow", argsJSON(t, map[string]any{"task": "task-1"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("want map result, got %T", result)
	}
	dv, ok := out["dispatch"].(dispatchView)
	if !ok {
		t.Fatalf("want dispatch to be a dispatchView, got %T", out["dispatch"])
	}
	if dv.AssigneeHandle != "terminal-3" {
		t.Errorf("want dispatch.assignee_handle == terminal-3 (from DispatchContext.handle), got %q — regression guard for the wire-naming translation", dv.AssigneeHandle)
	}
}

func TestDispatchShowChannel_NoDispatchYet_ReturnsNilDispatch(t *testing.T) {
	fake := &fakeOrchestrationClient{
		getDispatchContextForTaskFunc: func(ctx context.Context, in *orchestrationv1.GetDispatchContextForTaskRequest) (*orchestrationv1.GetDispatchContextForTaskResponse, error) {
			return &orchestrationv1.GetDispatchContextForTaskResponse{}, nil // unset Dispatch
		},
	}
	r := NewRegistry()
	registerOrchestrationChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "orchestration.dispatchShow", argsJSON(t, map[string]any{"task": "task-none"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out, ok := result.(map[string]any)
	if !ok || out["dispatch"] != nil {
		t.Fatalf("want {dispatch: nil}, got %+v", result)
	}
}

func TestDispatchShowChannel_PropagatesError(t *testing.T) {
	wantErr := errors.New("orchestration-service unavailable")
	fake := &fakeOrchestrationClient{
		getDispatchContextForTaskFunc: func(ctx context.Context, in *orchestrationv1.GetDispatchContextForTaskRequest) (*orchestrationv1.GetDispatchContextForTaskResponse, error) {
			return nil, wantErr
		},
	}
	r := NewRegistry()
	registerOrchestrationChannels(r, fake)

	_, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "orchestration.dispatchShow", argsJSON(t, map[string]any{"task": "task-1"}))
	if !errors.Is(err, wantErr) {
		t.Fatalf("want %v, got %v", wantErr, err)
	}
}
