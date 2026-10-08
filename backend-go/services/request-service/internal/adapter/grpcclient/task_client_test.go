package grpcclient

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"sync"
	"testing"

	"github.com/stablyai/orca-go/common/grpcmw"
	"github.com/stablyai/orca-go/common/tenant"
	taskv1 "github.com/stablyai/orca-go/proto/gen/go/orca/task/v1"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// fakeTaskServer is the slice of task-service the client talks to.
type fakeTaskServer struct {
	taskv1.UnimplementedTaskServiceServer
	mu          sync.Mutex
	tasks       []*taskv1.Task
	pageSize    int
	listCalls   []*taskv1.ListTasksRequest
	stateCalls  [][]string
	executeErr  error
	updateErr   error
	updates     []*taskv1.UpdateTaskRequest
	md          []metadata.MD
	subtreeEdge []*taskv1.AddEdgeRequest
}

func (f *fakeTaskServer) record(ctx context.Context) {
	md, _ := metadata.FromIncomingContext(ctx)
	f.md = append(f.md, md)
}

func (f *fakeTaskServer) ListTasks(ctx context.Context, req *taskv1.ListTasksRequest) (*taskv1.ListTasksResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record(ctx)
	f.listCalls = append(f.listCalls, req)
	start := 0
	if req.GetPageToken() != "" {
		start, _ = strconv.Atoi(req.GetPageToken())
	}
	var matching []*taskv1.Task
	for _, t := range f.tasks {
		if len(req.GetRequestIds()) == 0 || contains(req.GetRequestIds(), t.GetRequestId()) {
			matching = append(matching, t)
		}
	}
	end := min(start+f.pageSize, len(matching))
	resp := &taskv1.ListTasksResponse{Tasks: matching[start:end]}
	if end < len(matching) {
		resp.NextPageToken = strconv.Itoa(end)
	}
	return resp, nil
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func (f *fakeTaskServer) GetSubtree(ctx context.Context, req *taskv1.GetSubtreeRequest) (*taskv1.GetSubtreeResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record(ctx)
	return &taskv1.GetSubtreeResponse{Tasks: f.tasks, DependsOnEdges: f.subtreeEdge}, nil
}

func (f *fakeTaskServer) Execute(ctx context.Context, req *taskv1.TaskServiceExecuteRequest) (*taskv1.TaskServiceExecuteResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record(ctx)
	if f.executeErr != nil {
		return nil, f.executeErr
	}
	return &taskv1.TaskServiceExecuteResponse{Async: true}, nil
}

func (f *fakeTaskServer) UpdateTask(ctx context.Context, req *taskv1.UpdateTaskRequest) (*taskv1.UpdateTaskResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record(ctx)
	f.updates = append(f.updates, req)
	return &taskv1.UpdateTaskResponse{}, f.updateErr
}

func (f *fakeTaskServer) ListExecutionStates(ctx context.Context, req *taskv1.ListExecutionStatesRequest) (*taskv1.ListExecutionStatesResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record(ctx)
	f.stateCalls = append(f.stateCalls, req.GetTaskIds())
	resp := &taskv1.ListExecutionStatesResponse{}
	for _, id := range req.GetTaskIds() {
		resp.States = append(resp.States, &taskv1.ExecutionState{TaskId: id, LastEngine: "direct_agent", LastLinkStatus: "failed", FailedAttempts: 3, BlockedByTaskIds: []string{"b-" + id}})
	}
	return resp, nil
}

func startTaskServer(t *testing.T, f *fakeTaskServer) *TaskClient {
	t.Helper()
	lis := bufconn.Listen(1 << 20)
	srv := grpc.NewServer()
	taskv1.RegisterTaskServiceServer(srv, f)
	go func() { _ = srv.Serve(lis) }()
	conn, err := grpc.NewClient("passthrough:///bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close(); srv.Stop() })
	return NewTaskClient(taskv1.NewTaskServiceClient(conn))
}

func ctxWith(user string) context.Context {
	ctx := tenant.WithTenantID(context.Background(), "tenant-1")
	if user != "" {
		ctx = tenant.WithUserID(ctx, user)
	}
	return ctx
}

func TestTaskClient_ListTasks_FollowsPagesAndMapsFields(t *testing.T) {
	f := &fakeTaskServer{pageSize: 2}
	for i := 0; i < 5; i++ {
		f.tasks = append(f.tasks, &taskv1.Task{
			Id: fmt.Sprintf("t%d", i), RequestId: "r1", TaskType: "task", Status: "open", ParentId: "plan", Title: "T", WorktreeId: "wt", Labels: []string{"a"},
			AssigneeId: "al", EstimatedHours: wrapperspb.Double(1.5),
		})
	}
	c := startTaskServer(t, f)
	got, err := c.ListTasks(ctxWith(""), usecase.ListTasksQuery{RequestIDs: []string{"r1"}, TaskTypes: []string{"task", "plan"}, ParentID: "plan"})
	if err != nil || len(got) != 5 {
		t.Fatalf("every page must be read: %d %v", len(got), err)
	}
	if len(f.listCalls) != 3 {
		t.Fatalf("5 rows at 2 per page is 3 calls, got %d", len(f.listCalls))
	}
	first := f.listCalls[0]
	if first.GetParentId() != "plan" || len(first.GetTaskTypes()) != 2 || first.GetRequestIds()[0] != "r1" {
		t.Fatalf("query not forwarded: %v", first)
	}
	v := got[0]
	if v.ID != "t0" || v.RequestID != "r1" || v.Type != "task" || v.ParentID != "plan" || v.WorktreeID != "wt" || v.AssigneeID != "al" || v.EstimatedHours == nil || *v.EstimatedHours != 1.5 || len(v.Labels) != 1 {
		t.Fatalf("task not mapped: %+v", v)
	}
	if md := f.md[0]; md.Get(grpcmw.MetadataTenantID)[0] != "tenant-1" {
		t.Fatalf("tenant not forwarded: %v", md)
	}
	if len(f.md[0].Get(grpcmw.MetadataUserID)) != 0 {
		t.Fatal("a background read carries no user")
	}
}

func TestTaskClient_ListTasks_ChunksRequestIDsAt100(t *testing.T) {
	f := &fakeTaskServer{pageSize: 1000}
	var ids []string
	for i := 0; i < 250; i++ {
		id := fmt.Sprintf("r%d", i)
		ids = append(ids, id)
		f.tasks = append(f.tasks, &taskv1.Task{Id: "t" + id, RequestId: id})
	}
	got, err := startTaskServer(t, f).ListTasks(ctxWith("u"), usecase.ListTasksQuery{RequestIDs: ids})
	if err != nil || len(got) != 250 {
		t.Fatalf("got %d %v", len(got), err)
	}
	if len(f.listCalls) != 3 {
		t.Fatalf("250 ids go out in 3 chunks, got %d calls", len(f.listCalls))
	}
	for _, c := range f.listCalls {
		if len(c.GetRequestIds()) > 100 {
			t.Fatalf("task-service accepts at most 100 request ids, sent %d", len(c.GetRequestIds()))
		}
	}
}

func TestTaskClient_Execute_ErrorMapping(t *testing.T) {
	cases := []struct {
		name string
		err  error
		is   error
		code string
	}{
		{"already running", status.Error(codes.FailedPrecondition, "TASK_EXECUTE_ALREADY_IN_PROGRESS: task already has a dispatch in progress"), domain.ErrTaskAlreadyRunning, ""},
		{"no connection", status.Error(codes.FailedPrecondition, "TASK_EXECUTE_NO_CONNECTION: task's project has no connected dev server"), domain.ErrTaskDispatchTransient, "TASK_EXECUTE_NO_CONNECTION"},
		{"worktree", status.Error(codes.Internal, "TASK_EXECUTE_WORKTREE_FAILED: failed to provision worktree"), domain.ErrTaskDispatchTransient, "TASK_EXECUTE_WORKTREE_FAILED"},
		{"dispatch failed", status.Error(codes.Internal, "TASK_EXECUTE_FAILED: execution dispatch failed"), domain.ErrTaskDispatchTransient, "TASK_EXECUTE_FAILED"},
		{"forbidden", status.Error(codes.PermissionDenied, "TASK_FORBIDDEN: no execute permission"), domain.ErrTaskForbidden, ""},
		{"gone", status.Error(codes.NotFound, "TASK_NOT_FOUND: x"), domain.ErrTaskNotFoundRemote, ""},
		{"outage", status.Error(codes.Unavailable, "down"), domain.ErrTaskDispatchTransient, "TASK_SERVICE_UNAVAILABLE"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := &fakeTaskServer{executeErr: tc.err}
			err := startTaskServer(t, f).Execute(ctxWith("approver-1"), "task-1", "req:r:task-1:1")
			if !errors.Is(err, tc.is) {
				t.Fatalf("got %v, want %v", err, tc.is)
			}
			var de *domain.DispatchError
			if tc.code != "" && (!errors.As(err, &de) || de.Code != tc.code) {
				t.Fatalf("dispatch error code: %v", err)
			}
		})
	}
	t.Run("other failure surfaces", func(t *testing.T) {
		f := &fakeTaskServer{executeErr: status.Error(codes.FailedPrecondition, "TASK_EXECUTE_PROMPT_UNSUPPORTED: x")}
		err := startTaskServer(t, f).Execute(ctxWith("u"), "t", "ref")
		if err == nil || errors.Is(err, domain.ErrTaskDispatchTransient) || errors.Is(err, domain.ErrTaskAlreadyRunning) {
			t.Fatalf("an unknown failure is not transient: %v", err)
		}
	})
}

func TestTaskClient_Execute_CarriesTheActingUser(t *testing.T) {
	f := &fakeTaskServer{}
	c := startTaskServer(t, f)
	if err := c.Execute(ctxWith("approver-1"), "task-1", "req:r:task-1:1"); err != nil {
		t.Fatal(err)
	}
	if got := f.md[0].Get(grpcmw.MetadataUserID); len(got) != 1 || got[0] != "approver-1" {
		t.Fatalf("task-service checks this user's permission: %v", f.md[0])
	}
	if err := c.Execute(ctxWith(""), "task-1", "ref"); err == nil {
		t.Fatal("writes need an acting user")
	}
}

func TestTaskClient_SetWorktreeAndStatus(t *testing.T) {
	f := &fakeTaskServer{}
	c := startTaskServer(t, f)
	if err := c.SetWorktree(ctxWith("u"), "t1", "wt-7"); err != nil {
		t.Fatal(err)
	}
	if err := c.SetStatus(ctxWith("u"), "t1", "done"); err != nil {
		t.Fatal(err)
	}
	if len(f.updates) != 2 || f.updates[0].GetWorktreeId().GetValue() != "wt-7" || f.updates[0].GetStatus() != nil ||
		f.updates[1].GetStatus().GetValue() != "done" || f.updates[1].GetWorktreeId() != nil {
		t.Fatalf("each call must change only its own field: %v", f.updates)
	}
	f.updateErr = status.Error(codes.PermissionDenied, "TASK_FORBIDDEN: x")
	if err := c.SetStatus(ctxWith("u"), "t1", "done"); !errors.Is(err, domain.ErrTaskForbidden) {
		t.Fatalf("got %v", err)
	}
}

func TestTaskClient_ListExecutionStates_BatchesAt500(t *testing.T) {
	f := &fakeTaskServer{}
	var ids []string
	for i := 0; i < 1200; i++ {
		ids = append(ids, fmt.Sprintf("t%d", i))
	}
	got, err := startTaskServer(t, f).ListExecutionStates(ctxWith(""), ids)
	if err != nil || len(got) != 1200 {
		t.Fatalf("got %d %v", len(got), err)
	}
	if len(f.stateCalls) != 3 || len(f.stateCalls[0]) != 500 || len(f.stateCalls[2]) != 200 {
		t.Fatalf("1200 ids = 500+500+200, got %d calls", len(f.stateCalls))
	}
	s := got["t7"]
	if s.LastEngine != "direct_agent" || s.LastLinkStatus != "failed" || s.FailedAttempts != 3 || s.BlockedByTaskIDs[0] != "b-t7" {
		t.Fatalf("state not mapped: %+v", s)
	}
}

func TestTaskClient_GetSubtreeMapsEdges(t *testing.T) {
	f := &fakeTaskServer{tasks: []*taskv1.Task{{Id: "p", TaskType: "phase"}}, subtreeEdge: []*taskv1.AddEdgeRequest{{FromTaskId: "b", ToTaskId: "a"}}}
	sub, err := startTaskServer(t, f).GetSubtree(ctxWith(""), "p")
	if err != nil || len(sub.Tasks) != 1 || len(sub.DependsOn) != 1 || sub.DependsOn[0] != (usecase.TaskEdge{From: "b", To: "a"}) {
		t.Fatalf("got %+v %v", sub, err)
	}
}

func TestTaskClient_OutageIsMarked(t *testing.T) {
	lis := bufconn.Listen(1 << 10)
	conn, _ := grpc.NewClient("passthrough:///bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	defer conn.Close()
	// No server behind the listener: every call fails with Unavailable.
	_ = lis.Close()
	c := NewTaskClient(taskv1.NewTaskServiceClient(conn))
	if _, err := c.ListTasks(ctxWith(""), usecase.ListTasksQuery{RequestIDs: []string{"r"}}); !errors.Is(err, domain.ErrTaskServiceDown) {
		t.Fatalf("an outage must be recognisable: %v", err)
	}
	if _, err := c.ListExecutionStates(ctxWith(""), []string{"t"}); !errors.Is(err, domain.ErrTaskServiceDown) {
		t.Fatalf("got %v", err)
	}
}

func TestTaskClient_NeedsATenant(t *testing.T) {
	c := startTaskServer(t, &fakeTaskServer{})
	if _, err := c.ListTasks(context.Background(), usecase.ListTasksQuery{}); err == nil {
		t.Fatal("no tenant, no call")
	}
}

func TestUnavailableTaskClient(t *testing.T) {
	var c usecase.TaskClient = UnavailableTaskClient{}
	if _, err := c.ListTasks(ctxWith("u"), usecase.ListTasksQuery{}); !errors.Is(err, domain.ErrTaskServiceDown) {
		t.Fatalf("got %v", err)
	}
	if err := c.Execute(ctxWith("u"), "t", "r"); !errors.Is(err, domain.ErrTaskServiceDown) {
		t.Fatalf("got %v", err)
	}
}

func TestChunkStrings(t *testing.T) {
	if got := chunkStrings(nil, 3); len(got) != 0 {
		t.Fatalf("got %v", got)
	}
	got := chunkStrings([]string{"a", "b", "c", "d", "e"}, 2)
	if len(got) != 3 || len(got[2]) != 1 {
		t.Fatalf("got %v", got)
	}
}
