package grpcclient

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/stablyai/orca-go/common/grpcmw"
	"github.com/stablyai/orca-go/common/tenant"
	taskv1 "github.com/stablyai/orca-go/proto/gen/go/orca/task/v1"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

const (
	taskReadTimeout     = 30 * time.Second
	taskDispatchTimeout = 2 * time.Minute
	taskListPageSize    = 200
	// task-service caps request_ids per ListTasks call and task_ids per ListExecutionStates call.
	taskListMaxRequestIDs = 100
	taskStatesMaxIDs      = 500
)

// TaskClient is request-service's only door to task-service.
type TaskClient struct {
	client taskv1.TaskServiceClient
}

var _ usecase.TaskClient = (*TaskClient)(nil)

func NewTaskClient(c taskv1.TaskServiceClient) *TaskClient { return &TaskClient{client: c} }

// callContext forwards the tenant, and the acting user when there is one. Reads work without a user (background
// loops); writes need one because task-service checks that user's permission.
func callContext(ctx context.Context, needUser bool, timeout time.Duration) (context.Context, context.CancelFunc, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, nil, domain.ErrRequestTenantRequired()
	}
	pairs := []string{grpcmw.MetadataTenantID, tenantID}
	if userID, _ := tenant.UserID(ctx); userID != "" {
		pairs = append(pairs, grpcmw.MetadataUserID, userID)
	} else if needUser {
		return nil, nil, domain.ErrRequestReporterRequired()
	}
	ctx, cancel := context.WithTimeout(metadata.AppendToOutgoingContext(ctx, pairs...), timeout)
	return ctx, cancel, nil
}

// taskErrorCode extracts the stable code task-service puts before the colon of the status message.
func taskErrorCode(err error) string {
	st, ok := status.FromError(err)
	if !ok {
		return ""
	}
	if i := strings.Index(st.Message(), ":"); i > 0 {
		return st.Message()[:i]
	}
	return ""
}

func toTaskView(t *taskv1.Task) domain.TaskView {
	v := domain.TaskView{
		ID: t.GetId(), ParentID: t.GetParentId(), Type: t.GetTaskType(), Status: t.GetStatus(), RequestID: t.GetRequestId(),
		Title: t.GetTitle(), AssigneeID: t.GetAssigneeId(), WorktreeID: t.GetWorktreeId(), Labels: t.GetLabels(),
	}
	if h := t.GetEstimatedHours(); h != nil {
		hours := h.GetValue()
		v.EstimatedHours = &hours
	}
	return v
}

// ListTasks follows page tokens to the end and splits request ids into the chunks task-service accepts.
func (c *TaskClient) ListTasks(ctx context.Context, q usecase.ListTasksQuery) ([]domain.TaskView, error) {
	ctx, cancel, err := callContext(ctx, false, taskReadTimeout)
	if err != nil {
		return nil, err
	}
	defer cancel()
	chunks := chunkStrings(q.RequestIDs, taskListMaxRequestIDs)
	if len(chunks) == 0 {
		chunks = [][]string{nil}
	}
	var out []domain.TaskView
	for _, ids := range chunks {
		token := ""
		for {
			resp, err := c.client.ListTasks(ctx, &taskv1.ListTasksRequest{
				ProjectId: q.ProjectID, PageToken: token, PageSize: taskListPageSize, TaskTypes: q.TaskTypes, RequestIds: ids, ParentId: q.ParentID,
			})
			if err != nil {
				return nil, transportError("list tasks", err)
			}
			for _, t := range resp.GetTasks() {
				out = append(out, toTaskView(t))
			}
			if token = resp.GetNextPageToken(); token == "" {
				break
			}
		}
	}
	return out, nil
}

func (c *TaskClient) GetSubtree(ctx context.Context, rootID string) (usecase.SubtreeView, error) {
	ctx, cancel, err := callContext(ctx, false, taskReadTimeout)
	if err != nil {
		return usecase.SubtreeView{}, err
	}
	defer cancel()
	resp, err := c.client.GetSubtree(ctx, &taskv1.GetSubtreeRequest{RootId: rootID})
	if err != nil {
		return usecase.SubtreeView{}, transportError("get subtree", err)
	}
	var out usecase.SubtreeView
	for _, t := range resp.GetTasks() {
		out.Tasks = append(out.Tasks, toTaskView(t))
	}
	for _, e := range resp.GetDependsOnEdges() {
		out.DependsOn = append(out.DependsOn, usecase.TaskEdge{From: e.GetFromTaskId(), To: e.GetToTaskId()})
	}
	return out, nil
}

// Execute maps task-service's dispatch failures onto the domain errors AdvanceExecution branches on.
func (c *TaskClient) Execute(ctx context.Context, taskID, executionRequestID string) error {
	ctx, cancel, err := callContext(ctx, true, taskDispatchTimeout)
	if err != nil {
		return err
	}
	defer cancel()
	_, err = c.client.Execute(ctx, &taskv1.TaskServiceExecuteRequest{TaskId: taskID, RequestId: executionRequestID})
	if err == nil {
		return nil
	}
	code := taskErrorCode(err)
	switch {
	case code == "TASK_EXECUTE_ALREADY_IN_PROGRESS":
		return domain.ErrTaskAlreadyRunning
	case code == "TASK_EXECUTE_NO_CONNECTION", code == "TASK_EXECUTE_WORKTREE_FAILED", code == "TASK_EXECUTE_FAILED":
		return &domain.DispatchError{Code: code, Err: domain.ErrTaskDispatchTransient}
	case status.Code(err) == codes.PermissionDenied:
		return domain.ErrTaskForbidden
	case status.Code(err) == codes.NotFound:
		return domain.ErrTaskNotFoundRemote
	case status.Code(err) == codes.Unavailable, status.Code(err) == codes.DeadlineExceeded:
		return &domain.DispatchError{Code: "TASK_SERVICE_UNAVAILABLE", Err: domain.ErrTaskDispatchTransient}
	}
	return transportError("execute task", err)
}

func (c *TaskClient) update(ctx context.Context, req *taskv1.UpdateTaskRequest) error {
	ctx, cancel, err := callContext(ctx, true, taskReadTimeout)
	if err != nil {
		return err
	}
	defer cancel()
	if _, err := c.client.UpdateTask(ctx, req); err != nil {
		if status.Code(err) == codes.PermissionDenied {
			return domain.ErrTaskForbidden
		}
		return transportError("update task", err)
	}
	return nil
}

func (c *TaskClient) SetWorktree(ctx context.Context, taskID, worktreeID string) error {
	return c.update(ctx, &taskv1.UpdateTaskRequest{Id: taskID, WorktreeId: wrapperspb.String(worktreeID)})
}

func (c *TaskClient) SetStatus(ctx context.Context, taskID, statusValue string) error {
	return c.update(ctx, &taskv1.UpdateTaskRequest{Id: taskID, Status: wrapperspb.String(statusValue)})
}

// ListExecutionStates sends at most taskStatesMaxIDs ids per call.
func (c *TaskClient) ListExecutionStates(ctx context.Context, taskIDs []string) (map[string]usecase.ExecutionStateView, error) {
	ctx, cancel, err := callContext(ctx, false, taskReadTimeout)
	if err != nil {
		return nil, err
	}
	defer cancel()
	out := make(map[string]usecase.ExecutionStateView, len(taskIDs))
	for _, ids := range chunkStrings(taskIDs, taskStatesMaxIDs) {
		resp, err := c.client.ListExecutionStates(ctx, &taskv1.ListExecutionStatesRequest{TaskIds: ids})
		if err != nil {
			return nil, transportError("list execution states", err)
		}
		for _, s := range resp.GetStates() {
			out[s.GetTaskId()] = usecase.ExecutionStateView{
				LastEngine: s.GetLastEngine(), LastLinkStatus: s.GetLastLinkStatus(), FailedAttempts: int(s.GetFailedAttempts()),
				BlockedByTaskIDs: s.GetBlockedByTaskIds(),
			}
		}
	}
	return out, nil
}

func chunkStrings(in []string, size int) [][]string {
	var out [][]string
	for len(in) > 0 {
		n := min(size, len(in))
		out = append(out, in[:n])
		in = in[n:]
	}
	return out
}

// transportError marks an outage so callers can tell "task-service is down" from "task-service said no".
func transportError(op string, err error) error {
	if c := status.Code(err); c == codes.Unavailable || c == codes.DeadlineExceeded {
		return fmt.Errorf("grpcclient: %s: %w: %v", op, domain.ErrTaskServiceDown, err)
	}
	return fmt.Errorf("grpcclient: %s: %w", op, err)
}

// UnavailableTaskClient keeps the service startable without TASK_SERVICE_ADDR: every call reports the outage,
// so execution and the task backlog views fail loudly and the execution guard fails closed.
type UnavailableTaskClient struct{}

var _ usecase.TaskClient = UnavailableTaskClient{}

var errNoTaskService = fmt.Errorf("grpcclient: TASK_SERVICE_ADDR is not set: %w", domain.ErrTaskServiceDown)

func (UnavailableTaskClient) ListTasks(context.Context, usecase.ListTasksQuery) ([]domain.TaskView, error) {
	return nil, errNoTaskService
}
func (UnavailableTaskClient) GetSubtree(context.Context, string) (usecase.SubtreeView, error) {
	return usecase.SubtreeView{}, errNoTaskService
}
func (UnavailableTaskClient) Execute(context.Context, string, string) error { return errNoTaskService }
func (UnavailableTaskClient) SetWorktree(context.Context, string, string) error {
	return errNoTaskService
}
func (UnavailableTaskClient) SetStatus(context.Context, string, string) error {
	return errNoTaskService
}
func (UnavailableTaskClient) ListExecutionStates(context.Context, []string) (map[string]usecase.ExecutionStateView, error) {
	return nil, errNoTaskService
}
