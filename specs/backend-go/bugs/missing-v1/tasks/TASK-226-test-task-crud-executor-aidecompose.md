# TASK-226: Tests for `task.list`/`update`/`delete`/`getDependencies`, real `SimpleExecutor`, and `aiDecompose`/`aiApply`

**From Solution:** SOL-034 (Test plan)
**Priority:** P1
**Service:** `task-service` + `api-gateway`
**File:** `backend-go/services/task-service/internal/usecase/list_tasks_test.go`, `update_task_test.go`, `delete_task_test.go`, `get_dependencies_test.go`, `ai_decompose_test.go` (new), `internal/adapter/grpcclient/simple_executor_test.go` (replace), `internal/adapter/postgres/repository_test.go`, `internal/adapter/grpc/server_test.go`, `backend-go/services/api-gateway/internal/adapter/wscompat/channels_test.go`
**Depends on:** TASK-222, TASK-223, TASK-224, TASK-225
**Status:** `[x]` DONE — task-service coverage is fully done and passing: `list_tasks_test.go`, `update_task_test.go` (incl. the verbatim status-transition-guard regression case), `delete_task_test.go`, `get_dependencies_test.go` (incl. hydration-failure-propagates case), `ai_decompose_test.go` (incl. the not-connected → `KindFailedPrecondition` regression case) and `ai_apply_test.go` (incl. a new mid-loop-failure case locking in the non-transactional gap's error-surfacing behavior — see TASK-224) in `internal/usecase`; a replacement `internal/adapter/grpcclient/simple_executor_test.go` plus a new `grpcclient_test.go` (`ProjectExecutionResolver`/`AICompleter`/`AIProviderContextResolver`); `internal/adapter/postgres/repository_test.go` extended with `List`/`Update`/`Delete` + cascade-delete tests (`-tags=integration`, testcontainers-go — ran successfully, Docker available; one pre-existing container-startup flake, confirmed to pass on retry); `internal/adapter/grpc/server_test.go` with contract tests for all 6 new RPCs plus a CreateTask/GetTask regression guard. Step 6 (`api-gateway/internal/adapter/wscompat/channels_automation_task_test.go` coverage for the 6 new `task.*` channels, incl. `TestTaskCreateGetChannels_StillRegistered` and `TestTaskUpdateChannel_LeavesUnsetFieldsAsNilWrapperValues`) is now present from the concurrent wscompat-owning agent's pass. All task-service suites pass: `go test ./... -count=1` (unit) and `go test -tags=integration ./internal/adapter/postgres/...` (with Docker); api-gateway's `wscompat` suite passes too.

---

## Context

Per SOL-034's test plan. `ComplexExecutor` is explicitly out of scope for
this test plan — no test here should assert it does anything beyond its
current stub behavior.

## Changes to make

### Step 1: Usecase unit tests — `internal/usecase/`

`list_tasks_test.go`, `update_task_test.go`, `delete_task_test.go`,
`get_dependencies_test.go` — fakes for `TaskRepository`/`EdgeRepository`,
no real Postgres, per `03-clean-architecture-guidelines.md`. Key case in
`update_task_test.go` — the status-transition guard from TASK-223:

```go
func TestUpdateTask_RejectsTransitionIntoInProgress(t *testing.T) {
	repo := &fakeTaskRepository{tasks: map[string]domain.Task{
		"t1": {ID: "t1", Status: domain.StatusOpen},
	}}
	uc := NewUpdateTask(repo)

	status := domain.StatusInProgress
	_, err := uc.Execute(context.Background(), UpdateTaskInput{ID: "t1", Status: &status})
	if err == nil {
		t.Fatal("expected error: UpdateTask must not be able to transition a task into in_progress")
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Code != "TASK_INVALID_STATUS_TRANSITION" {
		t.Fatalf("expected TASK_INVALID_STATUS_TRANSITION, got %v", err)
	}
}

func TestUpdateTask_AllowsOtherTransitions(t *testing.T) {
	repo := &fakeTaskRepository{tasks: map[string]domain.Task{
		"t1": {ID: "t1", Status: domain.StatusOpen},
	}}
	uc := NewUpdateTask(repo)

	status := domain.StatusDone
	_, err := uc.Execute(context.Background(), UpdateTaskInput{ID: "t1", Status: &status})
	if err != nil {
		t.Fatalf("unexpected error transitioning open -> done: %v", err)
	}
}
```

`get_dependencies_test.go` — fake `EdgeRepository.ListFrom` returning a
mix of `depends_on` edges, fake `TaskRepository.Get` for hydration, assert
the returned task list matches the edge targets in order and that a
hydration failure on one edge propagates as an error rather than silently
skipping it.

### Step 2: `ai_decompose_test.go`

```go
package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

type fakeAIProviderContextResolver struct {
	ctx string
	err error
}

func (f *fakeAIProviderContextResolver) ResolveContext(ctx context.Context, tenantID, userID string) (string, error) {
	return f.ctx, f.err
}

type fakeProjectExecutionResolver struct {
	connectionID string
	connected    bool
	err          error
}

func (f *fakeProjectExecutionResolver) ResolveConnection(ctx context.Context, tenantID, projectID string) (string, bool, error) {
	return f.connectionID, f.connected, f.err
}

type fakeAICompleter struct {
	content string
	err     error
	gotConn string
}

func (f *fakeAICompleter) Complete(ctx context.Context, connectionID, prompt string) (string, error) {
	f.gotConn = connectionID
	return f.content, f.err
}

func TestAIDecompose_NotConnected_ReturnsFailedPrecondition(t *testing.T) {
	tasks := &fakeTaskRepository{tasks: map[string]domain.Task{"t1": {ID: "t1", ProjectID: "p1"}}}
	uc := NewAIDecompose(tasks, &fakeAIProviderContextResolver{}, &fakeProjectExecutionResolver{connected: false}, &fakeAICompleter{})

	_, err := uc.Execute(context.Background(), AIDecomposeInput{TaskID: "t1"})
	if err == nil {
		t.Fatal("expected error when project has no connected dev server")
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Kind != apperrors.KindFailedPrecondition {
		t.Fatalf("expected KindFailedPrecondition — a not-connected project must never silently return an empty proposal list, got %v", err)
	}
}

func TestAIDecompose_Connected_ReturnsParsedProposals(t *testing.T) {
	tasks := &fakeTaskRepository{tasks: map[string]domain.Task{"t1": {ID: "t1", ProjectID: "p1", Title: "Build widget"}}}
	completer := &fakeAICompleter{content: "1. Design API\n2. Implement handler"}
	uc := NewAIDecompose(tasks, &fakeAIProviderContextResolver{ctx: "anthropic"}, &fakeProjectExecutionResolver{connectionID: "conn-1", connected: true}, completer)

	got, err := uc.Execute(context.Background(), AIDecomposeInput{TaskID: "t1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("expected at least one parsed proposal")
	}
	if completer.gotConn != "conn-1" {
		t.Errorf("expected resolved connectionID to be passed through, got %q", completer.gotConn)
	}
}
```

### Step 3: `internal/adapter/grpcclient/simple_executor_test.go` — replace the stub's test

```go
func TestSimpleExecutor_Execute_RelaysAgentExec(t *testing.T) {
	tasks := &fakeTaskRepository{tasks: map[string]domain.Task{"t1": {ID: "t1", ProjectID: "p1", Title: "Do the thing"}}}
	resolver := &fakeProjectExecutionResolver{connectionID: "conn-1", connected: true}
	relay := &fakeInfraFleetServiceClient{
		relayResp: &infrafleetv1.RelayResponse{ResultJson: `{"executionRef":"exec-123"}`},
	}
	exec := NewSimpleExecutor(tasks, resolver, relay)

	ref, err := exec.Execute(context.Background(), "tenant-1", "t1", "req-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ref != "exec-123" {
		t.Errorf("expected executionRef to pass through, got %q", ref)
	}
	if relay.gotRelay.GetMethod() != "agent.exec" {
		t.Errorf("expected method=agent.exec, got %q", relay.gotRelay.GetMethod())
	}
	if relay.gotRelay.GetConnectionId() != "conn-1" {
		t.Errorf("expected resolved connectionId to be used, got %q", relay.gotRelay.GetConnectionId())
	}
}

// TestSimpleExecutor_NotConnected_ReturnsTypedError locks in that the stub
// behavior (a synthesized placeholder ref, no error) is actually gone.
func TestSimpleExecutor_NotConnected_ReturnsTypedError(t *testing.T) {
	tasks := &fakeTaskRepository{tasks: map[string]domain.Task{"t1": {ID: "t1", ProjectID: "p1"}}}
	resolver := &fakeProjectExecutionResolver{connected: false}
	exec := NewSimpleExecutor(tasks, resolver, &fakeInfraFleetServiceClient{})

	_, err := exec.Execute(context.Background(), "tenant-1", "t1", "req-1")
	if err == nil {
		t.Fatal("expected a real error for a not-connected project, not a synthesized placeholder ref")
	}
}
```

`fakeTaskRepository`, `fakeProjectExecutionResolver`, and
`fakeInfraFleetServiceClient` follow the same fake-the-generated-client
pattern `git-gateway-service`'s `grpcclient_test.go` and `wscompat`'s
`channels_test.go` both already use — add a local `fakeTaskRepository` to
this package's tests if one doesn't already exist (mirroring
`dispatch_test.go`'s `fakeConnectionResolver`/`fakeGitExecutor`
convention).

### Step 4: `internal/adapter/postgres/repository_test.go` — `testcontainers-go` integration tests

`List`/`Update`/`Delete` tenant-scoping, plus a cascade-delete assertion:
insert a task with a `task_edges` row (parent_child or depends_on), delete
the task, confirm the edge row is gone via `ON DELETE CASCADE`.

### Step 5: `internal/adapter/grpc/server_test.go` — contract tests

Contract tests for `ListTasks`/`UpdateTask`/`DeleteTask`/`GetDependencies`/
`AIDecompose`/`AIApply` (6 new RPCs), following this file's existing
per-RPC shape.

### Step 6: `wscompat/channels_test.go` — channel tests

One test per new channel (6), following `TestDevServerListChannel_Success`'s
shape (and `fakeTaskServiceClient`'s embed-and-override pattern, added if
not already present from earlier `task.*` channel tests). Include the
explicit regression guard SOL-034 calls for:

```go
func TestTaskCreateGetChannels_StillRegistered(t *testing.T) {
	r := NewRegistry()
	registerTaskChannels(r, &fakeTaskServiceClient{
		createTaskFunc: func(ctx context.Context, in *taskv1.CreateTaskRequest) (*taskv1.CreateTaskResponse, error) {
			return &taskv1.CreateTaskResponse{Task: &taskv1.Task{Id: "t1"}}, nil
		},
		getTaskFunc: func(ctx context.Context, in *taskv1.GetTaskRequest) (*taskv1.GetTaskResponse, error) {
			return &taskv1.GetTaskResponse{Task: &taskv1.Task{Id: in.GetId()}}, nil
		},
	})

	// Guards the "keep, don't remove" decision (TASK-222) against a future
	// contributor treating BUG-034's dead-code finding as license to delete
	// these two channels.
	if _, err := r.Dispatch(context.Background(), Identity{}, "task.create", argsJSON(t, map[string]any{"title": "x"})); err != nil {
		t.Errorf("expected task.create to remain registered: %v", err)
	}
	if _, err := r.Dispatch(context.Background(), Identity{}, "task.get", argsJSON(t, map[string]any{"id": "t1"})); err != nil {
		t.Errorf("expected task.get to remain registered: %v", err)
	}
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go/services/task-service
go test ./internal/usecase/... -count=1 -v
go test ./internal/adapter/grpcclient/... -count=1 -v
go test ./internal/adapter/postgres/... -count=1 -v   # requires Docker for testcontainers-go
go test ./internal/adapter/grpc/... -count=1 -v
cd /opt/repos/orca/backend-go/services/api-gateway
go test ./internal/adapter/wscompat/... -run TestTask -count=1 -v
```
