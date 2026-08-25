# TASK-112: Tests for `GetDispatchContextForTask` and `orchestration.dispatchShow`

**From Solution:** SOL-018
**Priority:** P2
**Service:** `orchestration-service`, `api-gateway`
**File:** `services/orchestration-service/internal/usecase/get_dispatch_context_for_task_test.go` (new), `services/orchestration-service/internal/adapter/postgres/repository_test.go` (extend), `services/api-gateway/internal/adapter/wscompat/channels_orchestration_test.go` (new)
**Depends on:** TASK-110, TASK-111
**Status:** `[partial]` — get_dispatch_context_for_task_test.go (usecase, 4 tests) and channels_orchestration_test.go (3 tests) all written and pass. `internal/adapter/postgres/repository_test.go` extended with `TestRepository_GetLatestForTask_ReturnsMostRecentAfterRetry`/`_NoRows_ReturnsErrDispatchContextNotFound`, type-checks under `-tags=integration` but NOT executed — no live Postgres/Docker in this environment.

---

## Context

Implements SOL-018's "Test plan" section exactly: usecase-level not-found
-is-not-an-error and empty-input-validation cases, a repository-level
retry-picks-latest-row case, and the `wscompat` regression guard for the
`handle` → `assignee_handle` translation this whole design rests on.

## Changes to make

### 1. `internal/usecase/get_dispatch_context_for_task_test.go` (new)

Follow this package's existing `fakes_test.go` pattern (see
`create_dispatch_context_test.go` for the sibling convention — a fake
`DispatchContextRepository`):

```go
package usecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/orchestration-service/internal/domain"
	"github.com/stablyai/orca-go/services/orchestration-service/internal/usecase"
)

func TestGetDispatchContextForTask_Found(t *testing.T) {
	want := domain.DispatchContext{ID: "dc-1", Handle: "terminal-3", OrchestrationTaskID: "task-1", CreatedAt: time.Now()}
	repo := &fakeDispatchContextRepository{getLatestReturns: want}
	uc := usecase.NewGetDispatchContextForTask(repo)
	ctx := tenant.WithTenantID(context.Background(), "tenant-1")

	dc, found, err := uc.Execute(ctx, "task-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !found {
		t.Fatal("want found=true")
	}
	if dc.Handle != "terminal-3" {
		t.Errorf("want Handle=terminal-3, got %q", dc.Handle)
	}
}

func TestGetDispatchContextForTask_NotFound_ReturnsFalseNotError(t *testing.T) {
	repo := &fakeDispatchContextRepository{getLatestErr: usecase.ErrDispatchContextNotFound}
	uc := usecase.NewGetDispatchContextForTask(repo)
	ctx := tenant.WithTenantID(context.Background(), "tenant-1")

	dc, found, err := uc.Execute(ctx, "task-missing")
	if err != nil {
		t.Fatalf("want nil error for not-found, got %v", err)
	}
	if found {
		t.Fatal("want found=false")
	}
	if dc != (domain.DispatchContext{}) {
		t.Errorf("want zero-value DispatchContext, got %+v", dc)
	}
}

func TestGetDispatchContextForTask_EmptyTaskID_FailsBeforeRepoCall(t *testing.T) {
	repo := &fakeDispatchContextRepository{
		getLatestFunc: func(ctx context.Context, tenantID, taskID string) (domain.DispatchContext, error) {
			t.Fatal("repo must not be called for an empty task id")
			return domain.DispatchContext{}, nil
		},
	}
	uc := usecase.NewGetDispatchContextForTask(repo)
	ctx := tenant.WithTenantID(context.Background(), "tenant-1")

	_, _, err := uc.Execute(ctx, "")
	if err == nil {
		t.Fatal("expected error for empty task id")
	}
}
```

Add a `fakeDispatchContextRepository` to this package's shared
`fakes_test.go` (extend the existing fake used by
`create_dispatch_context_test.go` with `GetLatestForTask` — do not
redefine `CreateDispatchContext` on a second type):

```go
type fakeDispatchContextRepository struct {
	// existing fields for CreateDispatchContext...
	getLatestReturns domain.DispatchContext
	getLatestErr     error
	getLatestFunc    func(ctx context.Context, tenantID, taskID string) (domain.DispatchContext, error)
}

func (f *fakeDispatchContextRepository) GetLatestForTask(ctx context.Context, tenantID, taskID string) (domain.DispatchContext, error) {
	if f.getLatestFunc != nil {
		return f.getLatestFunc(ctx, tenantID, taskID)
	}
	if f.getLatestErr != nil {
		return domain.DispatchContext{}, f.getLatestErr
	}
	return f.getLatestReturns, nil
}
```

### 2. `internal/adapter/postgres/repository_test.go` — extend (testcontainers)

```go
func TestRepository_GetLatestForTask_ReturnsMostRecentAfterRetry(t *testing.T) {
	repo := newTestRepository(t) // reuse this file's existing test-pool helper
	ctx := context.Background()

	first, err := repo.CreateDispatchContext(ctx, "tenant-1", "handle-a", "run-1", "task-1")
	if err != nil {
		t.Fatalf("create first dispatch context: %v", err)
	}
	_ = first
	time.Sleep(10 * time.Millisecond) // ensure created_at strictly orders the second row after the first
	second, err := repo.CreateDispatchContext(ctx, "tenant-1", "handle-b", "run-1", "task-1")
	if err != nil {
		t.Fatalf("create second (retry) dispatch context: %v", err)
	}

	got, err := repo.GetLatestForTask(ctx, "tenant-1", "task-1")
	if err != nil {
		t.Fatalf("get latest for task: %v", err)
	}
	if got.ID != second.ID {
		t.Errorf("want the later dispatch context (id=%s), got id=%s", second.ID, got.ID)
	}
}

func TestRepository_GetLatestForTask_NoRows_ReturnsErrDispatchContextNotFound(t *testing.T) {
	repo := newTestRepository(t)
	ctx := context.Background()

	_, err := repo.GetLatestForTask(ctx, "tenant-1", "task-never-dispatched")
	if !errors.Is(err, usecase.ErrDispatchContextNotFound) {
		t.Fatalf("want ErrDispatchContextNotFound, got %v", err)
	}
}
```

### 3. `services/api-gateway/internal/adapter/wscompat/channels_orchestration_test.go` (new)

```go
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
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go test ./services/orchestration-service/internal/usecase/... -run TestGetDispatchContextForTask -count=1 -v
go test ./services/orchestration-service/internal/adapter/postgres/... -run TestRepository_GetLatestForTask -count=1 -v
go test ./services/api-gateway/internal/adapter/wscompat/... -run TestDispatchShow -count=1 -v
```
