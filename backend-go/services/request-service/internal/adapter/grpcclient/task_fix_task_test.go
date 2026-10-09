package grpcclient

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/grpcmw"
	taskv1 "github.com/stablyai/orca-go/proto/gen/go/orca/task/v1"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// createTaskServer adds the plain CreateTask call the hotfix fix task uses to the shared fake.
type createTaskServer struct {
	fakeTaskServer
	creates   []*taskv1.CreateTaskRequest
	createErr error
	noID      bool
}

func (f *createTaskServer) CreateTask(ctx context.Context, req *taskv1.CreateTaskRequest) (*taskv1.CreateTaskResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record(ctx)
	f.creates = append(f.creates, req)
	if f.createErr != nil {
		return nil, f.createErr
	}
	if f.noID {
		return &taskv1.CreateTaskResponse{Task: &taskv1.Task{}}, nil
	}
	return &taskv1.CreateTaskResponse{Task: &taskv1.Task{Id: "fix-1", Title: req.GetTitle()}}, nil
}

func startCreateTaskServer(t *testing.T, f *createTaskServer) *TaskClient {
	t.Helper()
	return startTaskServerFor(t, f)
}

func fixTaskInput() usecase.PlanTreeInput {
	return usecase.PlanTreeInput{
		Request:   domain.Request{ID: "req-1", Number: 7, ProjectID: "proj-1", Title: "Checkout crashes"},
		Shape:     domain.ShapeSingleTask,
		Proposal:  domain.PlanProposal{Title: "Hotfix: Checkout crashes", Summary: "nil pointer", Tasks: []domain.TaskProposal{{Title: "Hotfix: Checkout crashes", Description: "fix it", TaskType: "bug"}}},
		CreatorID: "user-1",
	}
}

func TestTaskClient_CreateFixTask_SendsOneParentlessTaskLinkedToTheRequest(t *testing.T) {
	f := &createTaskServer{}
	c := startCreateTaskServer(t, f)
	id, err := c.CreateFixTask(ctxWith("user-1"), fixTaskInput())
	if err != nil || id != "fix-1" {
		t.Fatalf("CreateFixTask = %q, %v", id, err)
	}
	got := f.creates[0]
	if got.GetRequestId() != "req-1" || got.GetProjectId() != "proj-1" || got.GetParentId() != "" || got.GetTaskType() != "bug" || got.GetDescription() != "fix it" {
		t.Fatalf("request = %v", got)
	}
	if got.GetAiContext() == "" || got.GetCreatorId() != "user-1" {
		t.Fatalf("context %q creator %q", got.GetAiContext(), got.GetCreatorId())
	}
	if md := f.md[0]; md.Get(grpcmw.MetadataTenantID)[0] != "tenant-1" || md.Get(grpcmw.MetadataUserID)[0] != "user-1" {
		t.Fatalf("identity not forwarded: %v", md)
	}
}

func TestTaskClient_CreateFixTask_Failures(t *testing.T) {
	cases := []struct {
		name  string
		ctx   context.Context
		in    func() usecase.PlanTreeInput
		srv   *createTaskServer
		check func(t *testing.T, err error)
	}{
		{"needs an acting user", ctxWith(""), fixTaskInput, &createTaskServer{}, func(t *testing.T, err error) {
			if err == nil || errCodeOf(err) != "REQUEST_REPORTER_REQUIRED" {
				t.Fatalf("err = %v", err)
			}
		}},
		{"two tasks are not a hotfix", ctxWith("user-1"), func() usecase.PlanTreeInput {
			in := fixTaskInput()
			in.Proposal.Tasks = append(in.Proposal.Tasks, in.Proposal.Tasks[0])
			return in
		}, &createTaskServer{}, func(t *testing.T, err error) {
			if errCodeOf(err) != "REQUEST_HOTFIX_PLAN_SHAPE" {
				t.Fatalf("err = %v", err)
			}
		}},
		{"task-service down is retryable", ctxWith("user-1"), fixTaskInput, &createTaskServer{createErr: status.Error(codes.Unavailable, "down")}, func(t *testing.T, err error) {
			if errCodeOf(err) != "REQUEST_HOTFIX_FIX_TASK_UNAVAILABLE" {
				t.Fatalf("err = %v", err)
			}
		}},
		{"task-service refuses", ctxWith("user-1"), fixTaskInput, &createTaskServer{createErr: status.Error(codes.PermissionDenied, "TASK_FORBIDDEN: no")}, func(t *testing.T, err error) {
			if err == nil || errCodeOf(err) == "REQUEST_HOTFIX_FIX_TASK_UNAVAILABLE" {
				t.Fatalf("a refusal is not an outage: %v", err)
			}
		}},
		{"an answer without an id is an error", ctxWith("user-1"), fixTaskInput, &createTaskServer{noID: true}, func(t *testing.T, err error) {
			if err == nil {
				t.Fatal("a task with no id must not count as created")
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := startCreateTaskServer(t, tc.srv)
			id, err := c.CreateFixTask(tc.ctx, tc.in())
			if id != "" {
				t.Fatalf("id = %q on failure", id)
			}
			tc.check(t, err)
		})
	}
}

func TestUnavailableTaskClient_CreateFixTaskReportsTheOutage(t *testing.T) {
	_, err := UnavailableTaskClient{}.CreateFixTask(context.Background(), fixTaskInput())
	if errCodeOf(err) != "REQUEST_HOTFIX_FIX_TASK_UNAVAILABLE" {
		t.Fatalf("err = %v", err)
	}
}

func errCodeOf(err error) string {
	var ae *apperrors.AppError
	if errors.As(err, &ae) {
		return ae.Code
	}
	return ""
}
