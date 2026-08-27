# TASK-223: Add `task.list`/`update`/`delete`/`getDependencies` — proto + repository + usecase

**From Solution:** SOL-034 ("`list`/`update`/`delete`/`getDependencies`" section)
**Priority:** P1
**Service:** `task-service`
**File:** `backend-go/proto/orca/task/v1/task.proto`, `internal/usecase/ports.go`, `internal/usecase/list_tasks.go`, `update_task.go`, `delete_task.go`, `get_dependencies.go` (new), `internal/adapter/postgres/repository.go`, `internal/adapter/grpc/server.go`, `cmd/server/main.go`
**Depends on:** none
**Status:** `[x]` DONE — proto RPCs (`ListTasks`/`UpdateTask`/`DeleteTask`/`GetDependencies`), `ports.go` extension, `list_tasks.go`/`update_task.go`/`delete_task.go`/`get_dependencies.go` usecases, `domain.Task.SetStatus`'s in-progress guard, postgres `List`/`Update`/`Delete`, gRPC server translation methods, and `cmd/server/main.go` wiring all present. `go build`/`go vet` clean for `task-service`. Full unit test coverage for all 4 usecases plus a domain test, gRPC contract tests, and testcontainers-go integration tests (List/Update/Delete + cascade-delete) all added and passing — see TASK-226.

---

## Context

Grounded in the actual columns (`migrations/0001_init.up.sql`,
`0002_task_project_execution_tracking.up.sql`): `task.tasks(id, tenant_id,
title, status, parent_id, project_id, created_at, updated_at)`,
`task.task_edges(id, tenant_id, from_task_id, to_task_id, edge_type,
created_at)`. This scaffold's schema is narrower than `task-service.md`
§5's fuller sketch (no `description`/`complexity`/`assignee_id`/
`active_execution_id` columns) — this task's fields follow what's actually
there, same "extend the real thing, not the doc sketch" posture SOL-033
took for `automation.Automation`.

`UpdateTask` deliberately does **not** become the general mechanism that
clears `StatusInProgress` back out (the one-way-transition gap
`execute_task.go`'s doc comment names) — allowing an arbitrary
client-driven `status` write to double as an execution-completion callback
would let a buggy or malicious client mark a still-running task done
early. `UpdateTask`'s domain-layer status setter rejects transitions
*into* `in_progress` (that's `ExecuteTask`'s job only) but otherwise allows
normal open/done/cancelled edits.

## Changes to make

### Step 1: Proto — `backend-go/proto/orca/task/v1/task.proto`

Add to the `TaskService` service block:

```protobuf
  rpc ListTasks(ListTasksRequest) returns (ListTasksResponse);
  rpc UpdateTask(UpdateTaskRequest) returns (UpdateTaskResponse);
  rpc DeleteTask(DeleteTaskRequest) returns (google.protobuf.Empty);
  rpc GetDependencies(GetDependenciesRequest) returns (GetDependenciesResponse);
```

Add imports if not already present:

```protobuf
import "google/protobuf/empty.proto";
import "google/protobuf/wrappers.proto";
```

Append messages:

```protobuf
message ListTasksRequest {
  string tenant_id = 1;
  string project_id = 2; // optional filter
  string page_token = 3;
  int32 page_size = 4;
}
message ListTasksResponse {
  repeated Task tasks = 1;
  string next_page_token = 2;
}

// UpdateTask: wrapper-typed optional fields, same field-mask shape as
// SOL-033's UpdateAutomationRequest — a status-only edit (the common case,
// e.g. marking done) shouldn't require resending title/parent_id.
message UpdateTaskRequest {
  string id = 1;
  google.protobuf.StringValue title = 2;
  google.protobuf.StringValue status = 3;
}
message UpdateTaskResponse {
  Task task = 1;
}

message DeleteTaskRequest {
  string id = 1;
}

// GetDependencies walks depends_on edges FROM task_id — distinct from
// AddEdge (write) and from GetAncestors (parent_child, not on this proto's
// surface yet either).
message GetDependenciesRequest {
  string task_id = 1;
}
message GetDependenciesResponse {
  repeated Task dependencies = 1;
}
```

### Step 2: Extend repository ports — `internal/usecase/ports.go`

Extend `TaskRepository`:

```go
	// List returns tasks for tenantID, optionally filtered by projectID
	// (empty = no filter), cursor-paginated.
	List(ctx context.Context, tenantID, projectID, pageToken string, pageSize int32) ([]domain.Task, string, error)
	// Update persists a partial field update (title/status). Status
	// transitions into StatusInProgress are rejected at the domain layer
	// (domain.Task.SetStatus) — see this task's Context note.
	Update(ctx context.Context, tenantID string, task domain.Task) error
	// Delete removes a task. task_edges/task_grants reference tasks(id)
	// with ON DELETE CASCADE (migrations/0001_init.up.sql) — no explicit
	// edge/grant cleanup needed.
	Delete(ctx context.Context, tenantID, id string) error
```

Extend `EdgeRepository` doc comment only — `ListFrom` (already defined,
`ports.go:53-57`) is reused as-is by `GetDependencies` below; no interface
change needed for edges.

### Step 3: Domain — status-transition guard

Add (or extend, if a `Task` domain type/method already exists for this) a
`SetStatus` method in `internal/domain` that rejects a transition into
`in_progress`:

```go
// SetStatus applies a client-driven status edit. Transitioning into
// StatusInProgress is rejected here — that transition is ExecuteTask's
// job only (see execute_task.go's one-way-transition doc comment); letting
// UpdateTask double as a completion-callback surface would let a client
// mark a still-running task done early or fake a dispatch it never made.
func (t *Task) SetStatus(status string) error {
	if status == StatusInProgress {
		return fmt.Errorf("domain: cannot set status to %q via UpdateTask — only ExecuteTask may transition a task into %q", StatusInProgress, StatusInProgress)
	}
	t.Status = status
	return nil
}
```

Adjust to this package's actual existing `Task`/status-constant shape
(check `internal/domain` for the real type before adding — the sketch
above assumes `Task` is a struct with a `Status string` field and
`StatusInProgress` is an existing string constant, matching the CHECK
constraint in `migrations/0001_init.up.sql`:
`status IN ('open','in_progress','done','cancelled')`).

### Step 4: Usecases — `internal/usecase/`

`list_tasks.go`:

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

type ListTasksInput struct {
	TenantID  string
	ProjectID string
	PageToken string
	PageSize  int32
}

type ListTasksResult struct {
	Tasks         []domain.Task
	NextPageToken string
}

type ListTasks struct {
	tasks TaskRepository
}

func NewListTasks(tasks TaskRepository) *ListTasks {
	return &ListTasks{tasks: tasks}
}

func (uc *ListTasks) Execute(ctx context.Context, in ListTasksInput) (ListTasksResult, error) {
	if in.TenantID == "" {
		return ListTasksResult{}, apperrors.New(apperrors.KindInvalidArgument, "TASK_MISSING_TENANT_ID", "tenant_id is required", nil)
	}
	tasks, nextToken, err := uc.tasks.List(ctx, in.TenantID, in.ProjectID, in.PageToken, in.PageSize)
	if err != nil {
		return ListTasksResult{}, apperrors.New(apperrors.KindInternal, "TASK_LIST_FAILED", "failed to list tasks", err)
	}
	return ListTasksResult{Tasks: tasks, NextPageToken: nextToken}, nil
}
```

`update_task.go`:

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

type UpdateTaskInput struct {
	TenantID string
	ID       string
	Title    *string
	Status   *string
}

type UpdateTask struct {
	tasks TaskRepository
}

func NewUpdateTask(tasks TaskRepository) *UpdateTask {
	return &UpdateTask{tasks: tasks}
}

func (uc *UpdateTask) Execute(ctx context.Context, in UpdateTaskInput) (domain.Task, error) {
	current, err := uc.tasks.Get(ctx, in.TenantID, in.ID)
	if err != nil {
		return domain.Task{}, apperrors.New(apperrors.KindNotFound, "TASK_NOT_FOUND", "task not found", err)
	}
	if in.Title != nil {
		current.Title = *in.Title
	}
	if in.Status != nil {
		if err := current.SetStatus(*in.Status); err != nil {
			return domain.Task{}, apperrors.New(apperrors.KindInvalidArgument, "TASK_INVALID_STATUS_TRANSITION", err.Error(), err)
		}
	}
	if err := uc.tasks.Update(ctx, in.TenantID, current); err != nil {
		return domain.Task{}, apperrors.New(apperrors.KindInternal, "TASK_UPDATE_FAILED", "failed to persist update", err)
	}
	return current, nil
}
```

`delete_task.go`:

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
)

type DeleteTaskInput struct {
	TenantID string
	ID       string
}

type DeleteTask struct {
	tasks TaskRepository
}

func NewDeleteTask(tasks TaskRepository) *DeleteTask {
	return &DeleteTask{tasks: tasks}
}

func (uc *DeleteTask) Execute(ctx context.Context, in DeleteTaskInput) error {
	if in.ID == "" {
		return apperrors.New(apperrors.KindInvalidArgument, "TASK_MISSING_ID", "id is required", nil)
	}
	if err := uc.tasks.Delete(ctx, in.TenantID, in.ID); err != nil {
		return apperrors.New(apperrors.KindInternal, "TASK_DELETE_FAILED", "failed to delete task", err)
	}
	return nil
}
```

`get_dependencies.go` (reuses `EdgeRepository.ListFrom`, which
`ExecuteTask`'s `isComplex` check already calls for the identical edge kind
— no new repository method needed for the edge read itself, only the
task-hydration step edge→full Task that `ListFrom`'s existing callers
haven't needed):

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

type GetDependenciesInput struct {
	TenantID string
	TaskID   string
}

type GetDependencies struct {
	tasks TaskRepository
	edges EdgeRepository
}

func NewGetDependencies(tasks TaskRepository, edges EdgeRepository) *GetDependencies {
	return &GetDependencies{tasks: tasks, edges: edges}
}

func (uc *GetDependencies) Execute(ctx context.Context, in GetDependenciesInput) ([]domain.Task, error) {
	if in.TaskID == "" {
		return nil, apperrors.New(apperrors.KindInvalidArgument, "TASK_MISSING_TASK_ID", "task_id is required", nil)
	}
	edges, err := uc.edges.ListFrom(ctx, in.TenantID, in.TaskID, domain.EdgeKindDependsOn)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "TASK_GET_DEPENDENCIES_FAILED", "failed to list dependency edges", err)
	}
	tasks := make([]domain.Task, 0, len(edges))
	for _, e := range edges {
		t, err := uc.tasks.Get(ctx, in.TenantID, e.ToTaskID)
		if err != nil {
			return nil, apperrors.New(apperrors.KindInternal, "TASK_GET_DEPENDENCIES_FAILED", "failed to hydrate dependency task", err)
		}
		tasks = append(tasks, t)
	}
	return tasks, nil
}
```

### Step 5: Repository (Postgres) — `internal/adapter/postgres/repository.go`

```go
func (r *Repository) List(ctx context.Context, tenantID, projectID, pageToken string, pageSize int32) ([]domain.Task, string, error) {
	if pageSize <= 0 {
		pageSize = 50
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id, tenant_id, title, status, COALESCE(parent_id::text, ''), COALESCE(project_id::text, '')
		FROM task.tasks
		WHERE tenant_id = $1
		  AND ($2 = '' OR project_id::text = $2)
		  AND ($3 = '' OR id > $3)
		ORDER BY id
		LIMIT $4
	`, tenantID, projectID, pageToken, pageSize)
	if err != nil {
		return nil, "", fmt.Errorf("postgres: query tasks: %w", err)
	}
	defer rows.Close()

	var out []domain.Task
	for rows.Next() {
		var t domain.Task
		if err := rows.Scan(&t.ID, &t.TenantID, &t.Title, &t.Status, &t.ParentID, &t.ProjectID); err != nil {
			return nil, "", fmt.Errorf("postgres: scan task row: %w", err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("postgres: iterate task rows: %w", err)
	}
	nextToken := ""
	if len(out) == int(pageSize) {
		nextToken = out[len(out)-1].ID
	}
	return out, nextToken, nil
}

func (r *Repository) Update(ctx context.Context, tenantID string, t domain.Task) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE task.tasks SET title = $3, status = $4, updated_at = now()
		WHERE tenant_id = $1 AND id = $2
	`, tenantID, t.ID, t.Title, t.Status)
	if err != nil {
		return fmt.Errorf("postgres: update task: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("postgres: task %s not found for tenant %s", t.ID, tenantID)
	}
	return nil
}

func (r *Repository) Delete(ctx context.Context, tenantID, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM task.tasks WHERE tenant_id = $1 AND id = $2`, tenantID, id)
	if err != nil {
		return fmt.Errorf("postgres: delete task: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("postgres: task %s not found for tenant %s", id, tenantID)
	}
	return nil
}
```

### Step 6: gRPC adapter — `internal/adapter/grpc/server.go`

Add 4 fields to `Server` (`listTasks`, `updateTask`, `deleteTask`,
`getDependencies`), extend `New`'s params, and add 4 translation methods
following `CreateTask`/`GetTask`'s existing shape:

```go
func (s *Server) ListTasks(ctx context.Context, req *taskv1.ListTasksRequest) (*taskv1.ListTasksResponse, error) {
	result, err := s.listTasks.Execute(ctx, usecase.ListTasksInput{
		TenantID: req.GetTenantId(), ProjectID: req.GetProjectId(),
		PageToken: req.GetPageToken(), PageSize: req.GetPageSize(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := make([]*taskv1.Task, 0, len(result.Tasks))
	for _, t := range result.Tasks {
		out = append(out, toProtoTask(t)) // reuse CreateTask/GetTask's existing translation helper
	}
	return &taskv1.ListTasksResponse{Tasks: out, NextPageToken: result.NextPageToken}, nil
}

func (s *Server) UpdateTask(ctx context.Context, req *taskv1.UpdateTaskRequest) (*taskv1.UpdateTaskResponse, error) {
	in := usecase.UpdateTaskInput{ID: req.GetId()}
	if req.GetTitle() != nil {
		v := req.GetTitle().GetValue()
		in.Title = &v
	}
	if req.GetStatus() != nil {
		v := req.GetStatus().GetValue()
		in.Status = &v
	}
	task, err := s.updateTask.Execute(ctx, in)
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &taskv1.UpdateTaskResponse{Task: toProtoTask(task)}, nil
}

func (s *Server) DeleteTask(ctx context.Context, req *taskv1.DeleteTaskRequest) (*emptypb.Empty, error) {
	if err := s.deleteTask.Execute(ctx, usecase.DeleteTaskInput{ID: req.GetId()}); err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *Server) GetDependencies(ctx context.Context, req *taskv1.GetDependenciesRequest) (*taskv1.GetDependenciesResponse, error) {
	deps, err := s.getDependencies.Execute(ctx, usecase.GetDependenciesInput{TaskID: req.GetTaskId()})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := make([]*taskv1.Task, 0, len(deps))
	for _, t := range deps {
		out = append(out, toProtoTask(t))
	}
	return &taskv1.GetDependenciesResponse{Dependencies: out}, nil
}
```

`UpdateTaskInput`/`DeleteTaskInput`/`GetDependenciesInput` above omit
`TenantID` from the request translation only where the proto message
itself has no `tenant_id` field (`task.proto`'s existing messages mostly
rely on tenant context propagated via gRPC metadata, per
`tenant.RequireTenantID`'s pattern already used in `automation-service`'s
`RunNow` — confirm which convention `task-service`'s existing `CreateTask`/
`GetTask` server methods use for tenant resolution and match it exactly
rather than introducing a second convention in this task).

`toProtoTask` should already exist in this file — reuse it as-is.

### Step 7: Composition root — `cmd/server/main.go`

```go
	listTasksUC := usecase.NewListTasks(taskRepo)
	updateTaskUC := usecase.NewUpdateTask(taskRepo)
	deleteTaskUC := usecase.NewDeleteTask(taskRepo)
	getDependenciesUC := usecase.NewGetDependencies(taskRepo, edgeRepo)
```

Extend the `taskgrpc.New(...)` call with all 4 (variable names
illustrative — match whatever this service's real composition root already
calls its repository/usecase variables).

## Verify

```bash
cd /opt/repos/orca/backend-go
buf generate proto
buf breaking proto --against '.git#branch=main'
cd services/task-service
go build ./... && go vet ./...
```

Expected: clean build, `buf breaking` reports only additions. Full test
coverage for this work is TASK-226, not this task.
