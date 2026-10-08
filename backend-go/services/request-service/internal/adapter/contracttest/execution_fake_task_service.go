package contracttest

import (
	"context"
	"fmt"
	"net"
	"slices"
	"strconv"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/grpcmw"
	taskv1 "github.com/stablyai/orca-go/proto/gen/go/orca/task/v1"
	"github.com/stablyai/orca-go/services/request-service/internal/adapter/grpcclient"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/proto"
)

// StatusChange is what the real task-service would put in an orca.task.task.statuschanged event.
type StatusChange struct {
	TaskID, TaskType, ParentID, RequestID string
	Previous, New, Cause, Error, LinkID   string
}

// FakeTaskService is a stateful, in-memory task-service behind a bufconn gRPC server. It follows the rules
// request-service relies on: Execute claims open->in_progress and creates one worktree per task, only done
// unblocks dependents, container statuses are derived from their children, and run results arrive as
// execution_completed / execution_failed events.
type FakeTaskService struct {
	mu sync.Mutex
	taskv1.UnimplementedTaskServiceServer
	order       []string
	tasks       map[string]*taskv1.Task
	deps        map[string][]string
	seq         int
	worktrees   int
	executedBy  []string
	executed    []string
	forbidden   map[string]bool
	executeErrs map[string][]error
	links       map[string]string
	// OnChange receives every status change; the test decides whether the event is delivered, delayed or lost.
	OnChange func(StatusChange)
	Client   *grpcclient.TaskClient
}

// StartFakeTaskService serves the fake over bufconn and returns the real request-service TaskClient pointed at it.
func StartFakeTaskService(t *testing.T) *FakeTaskService {
	t.Helper()
	f := &FakeTaskService{tasks: map[string]*taskv1.Task{}, deps: map[string][]string{}, forbidden: map[string]bool{}, executeErrs: map[string][]error{}, links: map[string]string{}}
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
	f.Client = grpcclient.NewTaskClient(taskv1.NewTaskServiceClient(conn))
	return f
}

func (f *FakeTaskService) next(prefix string) string {
	f.seq++
	return fmt.Sprintf("%s-%03d", prefix, f.seq)
}

// Add stores a task and returns its id. parent may be empty.
func (f *FakeTaskService) Add(requestID, taskType, title, parent, taskStatus string, labels ...string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	id := uuid.NewString()
	f.order = append(f.order, id)
	f.tasks[id] = &taskv1.Task{Id: id, RequestId: requestID, TaskType: taskType, Title: title, ParentId: parent, Status: taskStatus, Labels: labels}
	return id
}

func (f *FakeTaskService) DependsOn(task, on string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deps[task] = append(f.deps[task], on)
}

func (f *FakeTaskService) Task(id string) *taskv1.Task {
	f.mu.Lock()
	defer f.mu.Unlock()
	return clone(f.tasks[id])
}

func clone(t *taskv1.Task) *taskv1.Task {
	if t == nil {
		return nil
	}
	return proto.Clone(t).(*taskv1.Task)
}

// Executed lists the execution references in call order; ExecutedBy the acting users.
func (f *FakeTaskService) Executed() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.executed)
}

func (f *FakeTaskService) ExecutedBy() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.executedBy)
}

// FailExecuteWith makes the next Execute calls on a task return err (nil entries pass through).
func (f *FakeTaskService) FailExecuteWith(taskID string, errs ...error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.executeErrs[taskID] = errs
}

func (f *FakeTaskService) emit(c StatusChange) {
	if f.OnChange != nil {
		f.OnChange(c)
	}
}

// RunSucceeded ends a run: in_progress -> review, reported as execution_completed.
func (f *FakeTaskService) RunSucceeded(taskID string) { f.endRun(taskID, true, "") }

// RunFailed ends a run with an error: the task goes back to open, reported as execution_failed.
func (f *FakeTaskService) RunFailed(taskID, message string) { f.endRun(taskID, false, message) }

func (f *FakeTaskService) endRun(taskID string, ok bool, message string) {
	f.mu.Lock()
	t := f.tasks[taskID]
	prev := t.Status
	cause, next := "execution_completed", "review"
	if !ok {
		cause, next = "execution_failed", "open"
	}
	t.Status = next
	change := StatusChange{TaskID: t.Id, TaskType: t.TaskType, ParentID: t.ParentId, RequestID: t.RequestId, Previous: prev, New: next, Cause: cause, Error: message, LinkID: f.links[taskID]}
	f.mu.Unlock()
	f.emit(change)
}

// ReleaseUnlinkedSilently mimics the bulk release of an in_progress task with no link: it reverts without any event.
func (f *FakeTaskService) ReleaseUnlinkedSilently(taskID string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.tasks[taskID].Status = "open"
}

func callerOf(ctx context.Context) string {
	md, _ := metadata.FromIncomingContext(ctx)
	if v := md.Get(grpcmw.MetadataUserID); len(v) > 0 {
		return v[0]
	}
	return ""
}

func (f *FakeTaskService) ListTasks(_ context.Context, req *taskv1.ListTasksRequest) (*taskv1.ListTasksResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var all []*taskv1.Task
	for _, id := range f.order {
		t := f.tasks[id]
		if len(req.GetRequestIds()) > 0 && !slices.Contains(req.GetRequestIds(), t.GetRequestId()) {
			continue
		}
		if len(req.GetTaskTypes()) > 0 && !slices.Contains(req.GetTaskTypes(), t.GetTaskType()) {
			continue
		}
		if req.GetParentId() != "" && t.GetParentId() != req.GetParentId() {
			continue
		}
		all = append(all, clone(t))
	}
	start := 0
	if req.GetPageToken() != "" {
		start, _ = strconv.Atoi(req.GetPageToken())
	}
	size := int(req.GetPageSize())
	if size <= 0 || size > 3 {
		size = 3 // small pages on purpose: the client must follow the token
	}
	end := min(start+size, len(all))
	resp := &taskv1.ListTasksResponse{Tasks: all[start:end]}
	if end < len(all) {
		resp.NextPageToken = strconv.Itoa(end)
	}
	return resp, nil
}

func (f *FakeTaskService) GetSubtree(_ context.Context, req *taskv1.GetSubtreeRequest) (*taskv1.GetSubtreeResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	in := map[string]bool{req.GetRootId(): true}
	for changed := true; changed; {
		changed = false
		for _, id := range f.order {
			if !in[id] && in[f.tasks[id].GetParentId()] {
				in[id], changed = true, true
			}
		}
	}
	resp := &taskv1.GetSubtreeResponse{}
	for _, id := range f.order {
		if in[id] {
			resp.Tasks = append(resp.Tasks, clone(f.tasks[id]))
			for _, d := range f.deps[id] {
				resp.DependsOnEdges = append(resp.DependsOnEdges, &taskv1.AddEdgeRequest{FromTaskId: id, ToTaskId: d})
			}
		}
	}
	return resp, nil
}

func (f *FakeTaskService) Execute(ctx context.Context, req *taskv1.TaskServiceExecuteRequest) (*taskv1.TaskServiceExecuteResponse, error) {
	f.mu.Lock()
	user := callerOf(ctx)
	f.executedBy = append(f.executedBy, user)
	if f.forbidden[user] {
		f.mu.Unlock()
		return nil, status.Error(codes.PermissionDenied, "TASK_FORBIDDEN: no execute permission")
	}
	id := req.GetTaskId()
	if errs := f.executeErrs[id]; len(errs) > 0 {
		f.executeErrs[id] = errs[1:]
		if errs[0] != nil {
			f.mu.Unlock()
			return nil, errs[0]
		}
	}
	t, ok := f.tasks[id]
	switch {
	case !ok:
		f.mu.Unlock()
		return nil, status.Error(codes.NotFound, "TASK_NOT_FOUND: no such task")
	case t.GetTaskType() == "plan" || t.GetTaskType() == "phase":
		f.mu.Unlock()
		return nil, status.Error(codes.FailedPrecondition, "TASK_EXECUTE_CONTAINER_NOT_EXECUTABLE: containers cannot run")
	case t.GetStatus() != "open":
		f.mu.Unlock()
		return nil, status.Error(codes.FailedPrecondition, "TASK_EXECUTE_ALREADY_IN_PROGRESS: task already has a dispatch in progress")
	}
	if t.WorktreeId == "" {
		f.worktrees++
		t.WorktreeId = fmt.Sprintf("wt-%d", f.worktrees)
	}
	prev := t.Status
	t.Status = "in_progress"
	link := uuid.NewString()
	f.links[id] = link
	f.executed = append(f.executed, req.GetRequestId())
	change := StatusChange{TaskID: id, TaskType: t.TaskType, ParentID: t.ParentId, RequestID: t.RequestId, Previous: prev, New: "in_progress", Cause: "execute_claim", LinkID: link}
	f.mu.Unlock()
	f.emit(change)
	return &taskv1.TaskServiceExecuteResponse{Async: true}, nil
}

func (f *FakeTaskService) UpdateTask(ctx context.Context, req *taskv1.UpdateTaskRequest) (*taskv1.UpdateTaskResponse, error) {
	f.mu.Lock()
	if f.forbidden[callerOf(ctx)] {
		f.mu.Unlock()
		return nil, status.Error(codes.PermissionDenied, "TASK_FORBIDDEN: no write permission")
	}
	t, ok := f.tasks[req.GetId()]
	if !ok {
		f.mu.Unlock()
		return nil, status.Error(codes.NotFound, "TASK_NOT_FOUND: no such task")
	}
	if req.GetWorktreeId() != nil {
		t.WorktreeId = req.GetWorktreeId().GetValue()
	}
	var changes []StatusChange
	if req.GetStatus() != nil && req.GetStatus().GetValue() != t.Status {
		changes = f.setStatusLocked(t, req.GetStatus().GetValue(), "user_update")
	}
	f.mu.Unlock()
	for _, c := range changes {
		f.emit(c)
	}
	return &taskv1.UpdateTaskResponse{Task: clone(t)}, nil
}

// setStatusLocked applies a status, unblocks dependents on done, derives ancestors, and returns the events to emit.
func (f *FakeTaskService) setStatusLocked(t *taskv1.Task, next, cause string) []StatusChange {
	prev := t.Status
	t.Status = next
	out := []StatusChange{{TaskID: t.Id, TaskType: t.TaskType, ParentID: t.ParentId, RequestID: t.RequestId, Previous: prev, New: next, Cause: cause}}
	if next == "done" {
		for _, id := range f.order {
			d := f.tasks[id]
			if d.Status != "blocked" || !slices.Contains(f.deps[id], t.Id) {
				continue
			}
			ready := true
			for _, dep := range f.deps[id] {
				ready = ready && f.tasks[dep].Status == "done"
			}
			if ready {
				d.Status = "open"
			}
		}
	}
	for cur := t; cur.ParentId != ""; {
		parent, ok := f.tasks[cur.ParentId]
		if !ok {
			break
		}
		done, any := true, false
		for _, id := range f.order {
			if c := f.tasks[id]; c.ParentId == parent.Id {
				any = true
				done = done && (c.Status == "done" || c.Status == "cancelled")
			}
		}
		want := parent.Status
		switch {
		case any && done:
			want = "done"
		case parent.Status == "done":
			want = "open"
		}
		if want != parent.Status {
			out = append(out, StatusChange{TaskID: parent.Id, TaskType: parent.TaskType, ParentID: parent.ParentId, RequestID: parent.RequestId, Previous: parent.Status, New: want, Cause: "derived"})
			parent.Status = want
		}
		cur = parent
	}
	return out
}

func (f *FakeTaskService) ListExecutionStates(_ context.Context, req *taskv1.ListExecutionStatesRequest) (*taskv1.ListExecutionStatesResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	resp := &taskv1.ListExecutionStatesResponse{}
	for _, id := range req.GetTaskIds() {
		st := &taskv1.ExecutionState{TaskId: id}
		for _, d := range f.deps[id] {
			if f.tasks[d].GetStatus() != "done" {
				st.BlockedByTaskIds = append(st.BlockedByTaskIds, d)
			}
		}
		resp.States = append(resp.States, st)
	}
	return resp, nil
}

// errNoConnection is what task-service answers when the project has no dev server.
func errNoConnection() error {
	return status.Error(codes.FailedPrecondition, "TASK_EXECUTE_NO_CONNECTION: task's project has no connected dev server")
}
