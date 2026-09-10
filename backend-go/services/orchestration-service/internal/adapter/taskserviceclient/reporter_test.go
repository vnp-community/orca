package taskserviceclient

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	taskv1 "github.com/stablyai/orca-go/proto/gen/go/orca/task/v1"
)

// fakeTaskServiceClient implements taskv1.TaskServiceClient directly (no
// bufconn/wire-level gRPC) — mirrors infrafleetclient's fake shape.
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

func TestReporter_ReportResult_MapsFieldsCorrectly(t *testing.T) {
	fake := &fakeTaskServiceClient{resp: &emptypb.Empty{}}
	r := NewReporter(fake)

	if err := r.ReportResult(context.Background(), "task-1", "run-1", true, ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fake.got.TaskId != "task-1" || fake.got.ExecutionRef != "run-1" || !fake.got.Success || fake.got.ErrorMessage != "" || fake.got.Engine != "orchestration" {
		t.Errorf("unexpected request: %+v", fake.got)
	}

	if err := r.ReportResult(context.Background(), "task-2", "run-2", false, "boom"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fake.got.Success || fake.got.ErrorMessage != "boom" {
		t.Errorf("expected failure fields to round-trip, got %+v", fake.got)
	}
}

func TestReporter_ReportResult_SurfacesGRPCErrorWrapped(t *testing.T) {
	fake := &fakeTaskServiceClient{err: errors.New("unavailable")}
	r := NewReporter(fake)

	err := r.ReportResult(context.Background(), "task-1", "run-1", true, "")
	if err == nil {
		t.Fatal("expected an error to surface")
	}
	if !errors.Is(err, fake.err) {
		t.Errorf("expected the underlying gRPC error to be wrapped (errors.Is), got %v", err)
	}
}
