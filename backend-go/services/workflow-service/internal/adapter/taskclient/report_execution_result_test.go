package taskclient

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	taskv1 "github.com/stablyai/orca-go/proto/gen/go/orca/task/v1"
	"github.com/stablyai/orca-go/services/workflow-service/internal/usecase"
)

// fakeTaskServiceClient implements taskv1.TaskServiceClient directly (no
// bufconn/wire-level gRPC) — same fake-the-generated-client-port
// convention used throughout this codebase's adapter tests.
type fakeTaskServiceClient struct {
	taskv1.TaskServiceClient // embed: panics on any unimplemented method, intentional for these tests

	resp *emptypb.Empty
	err  error
	got  *taskv1.ReportTaskExecutionResultRequest
}

func (f *fakeTaskServiceClient) ReportTaskExecutionResult(_ context.Context, in *taskv1.ReportTaskExecutionResultRequest, _ ...grpc.CallOption) (*emptypb.Empty, error) {
	f.got = in
	if f.err != nil {
		return nil, f.err
	}
	return f.resp, nil
}

func TestClient_ReportTaskExecutionResult_MapsFieldsCorrectly(t *testing.T) {
	fake := &fakeTaskServiceClient{resp: &emptypb.Empty{}}
	c := New(fake)

	err := c.ReportTaskExecutionResult(context.Background(), usecase.ReportTaskExecutionResultInput{
		TaskID: "task-1", ExecutionRef: "exec-1", Success: true, ActualHours: 0, Engine: "workflow",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fake.got.TaskId != "task-1" || fake.got.ExecutionRef != "exec-1" || !fake.got.Success || fake.got.Engine != "workflow" {
		t.Errorf("unexpected request: %+v", fake.got)
	}
}

func TestClient_ReportTaskExecutionResult_SurfacesGRPCErrorWrapped(t *testing.T) {
	fake := &fakeTaskServiceClient{err: errors.New("unavailable")}
	c := New(fake)

	err := c.ReportTaskExecutionResult(context.Background(), usecase.ReportTaskExecutionResultInput{TaskID: "task-1", ExecutionRef: "exec-1", Engine: "workflow"})
	if err == nil {
		t.Fatal("expected an error to surface")
	}
	if !errors.Is(err, fake.err) {
		t.Errorf("expected the underlying gRPC error to be wrapped (errors.Is), got %v", err)
	}
}
