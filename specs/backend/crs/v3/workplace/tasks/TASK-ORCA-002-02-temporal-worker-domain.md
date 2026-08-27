> ⚠️ **SUPERSEDED (2026-08-10):** Tài liệu task/solution breakdown này được sinh ra từ các CR-ORCA-00x/CR-TASK-00x đã bị viết lại hoặc retired — dựa trên giả định sai rằng `vnp-planner` bị loại bỏ và `vnp-workplace` tự dựng `planner-service` mới. Kiến trúc đúng: xem [`docs/crs/v3/orca/README.md`](../../../../../docs/crs/v3/orca/README.md). Nội dung bên dưới chỉ còn giá trị tham khảo lịch sử — không dùng để implement.
>
> ---
>
## Status: 🔲 NOT STARTED

# TASK-ORCA-002-02 — `planner-service`: Domain Layer (`orcatask.Tracking`)

**Phase:** 0 — Nền tảng
**Scope:** ✅ vnp-workplace — Go (`backend/services/planner-service`, `:3013`)
**Source:** [SOL-ORCA-002 §3.2](../solutions/SOL-ORCA-002-planner-orca-dispatcher.md#32-domain-orcatasktracking-entity)
**Depends On:** — (không phụ thuộc task nào khác — domain layer không có external dependency)
**Estimated Files:** ~6 files (3 code + 3 test)
**Working Dir:** `/opt/repos/vnp-workplace/backend/services/planner-service/internal/domain/orcatask/`

---

## Bối cảnh quan trọng

> **Đây là thiết kế mới thay thế hoàn toàn `temporal-worker`/`OrcaDispatch`** (tên file cũ của task này, `temporal-worker-domain.md`, được giữ nguyên để không phá vỡ link tham chiếu — nội dung bên trong đã viết lại 100% cho kiến trúc `planner-service`/asynq, KHÔNG còn Temporal/workflow_id/run_id nào).

- `orcatask.Tracking` (bảng `planner.orca_task_tracking`) là **bản ghi vận hành/idempotency nội bộ** của `planner-service` — KHÔNG phải nguồn sự thật nghiệp vụ. Nguồn sự thật là `PlanTask.Status` (domain `plantask`, CR-TASK-001), được `plan.DispatchTaskUseCase` cập nhật sau khi `Tracking` đạt trạng thái terminal (xem TASK-ORCA-002-06). Ghi rõ điều này trong doc-comment của entity — không thiết kế `Tracking` như thể nó là bảng "chốt hạ" cuối cùng cho toàn Plan.
- **Khác thiết kế Temporal cũ:** không có `WorkflowID`/`RunID` (không có workflow engine), không có `OrgID`/`PlanID` riêng biệt (CR-TASK-001's `PlanTask` hiện không mang trường org/workspace — xem SOL-ORCA-002 §9 Discoveries #6) — chỉ có `PlannerTaskID`/`PlannerJobID` để map về `PlanTask.ID`/`Plan.ID`.
- `planner-service` **chưa có** package `internal/domain/orcatask` — đây là entity domain đầu tiên của bounded context "Orca dispatch tracking" trong service này (các domain khác như `plan`/`plantask`/`plannode` thuộc CR-TASK-001, không đụng tới ở task này).

---

## Mục tiêu

Implement domain layer cho việc theo dõi 1 lần dispatch task tới Orca (`Tracking` aggregate), theo đúng pattern TDD-00 (Entity/Repository interface/Errors, zero external dependency ngoài stdlib + `github.com/google/uuid`).

---

## Acceptance Criteria

- [ ] `go build ./internal/domain/orcatask/...` thành công (zero import ngoài stdlib + `github.com/google/uuid`)
- [ ] `go test ./internal/domain/orcatask/... -v -race -cover` pass 100%, coverage ≥ 90%
- [ ] `Tracking.MarkSubmitted` chỉ hợp lệ từ `StatusPending`
- [ ] `Tracking.Complete` chỉ nhận status terminal (`StatusDone/StatusBlocked/StatusCancelled`)
- [ ] `Tracking.IsTimedOut` đúng logic: chỉ `true` khi đang `StatusInProgress` và `now` sau `DeadlineAt`
- [ ] `Status.IsTerminal()` đúng cho `done/blocked/cancelled`, sai cho `pending/in_progress`
- [ ] Doc-comment của `Tracking` ghi rõ vai trò "operational/idempotency record, không phải nguồn sự thật nghiệp vụ" (đối lập với `PlanTask.Status`)
- [ ] `Result` (domain-level, KHÔNG phải `orcaclient.OrcaTaskResult`) không import bất kỳ package `internal/infrastructure/...` nào (SOL-ORCA-002 §2 D4)

---

## File 1: `internal/domain/orcatask/entity.go`

```go
// Package orcatask defines the domain entity that tracks the lifecycle of one
// task dispatched to Orca — from submission through polling to a terminal
// result. This is an OPERATIONAL record (idempotency + poll-loop state), not
// the business source of truth for whether a PlanTask (CR-TASK-001, package
// plantask) is complete — that role belongs to PlanTask.Status, updated by
// application/plan.DispatchTaskUseCase after Tracking reaches a terminal
// state (see TASK-ORCA-002-06). Do not build dashboards or external APIs
// directly on top of this table.
package orcatask

import (
	"fmt"
	"time"
)

type Status string

const (
	StatusPending    Status = "pending"     // created, not yet submitted to Orca
	StatusInProgress Status = "in_progress" // submitted, being polled
	StatusDone       Status = "done"
	StatusBlocked    Status = "blocked"
	StatusCancelled  Status = "cancelled"
)

func (s Status) IsTerminal() bool {
	switch s {
	case StatusDone, StatusBlocked, StatusCancelled:
		return true
	}
	return false
}

// Result mirrors the subset of orcaclient.OrcaTaskResult the domain layer
// needs — kept as a separate domain-level type (see SOL-ORCA-002 §2 D4) so
// internal/domain never imports internal/infrastructure/http/orcaclient.
// Mapping orcaclient.OrcaTaskResult -> Result happens in the caller
// (infrastructure/queue/asynq poll handler, TASK-ORCA-002-05), not here.
type Result struct {
	Success       bool
	FilesCreated  []string
	FilesModified []string
	CommitHash    string
	TestOutput    string
	ErrorMessage  string
}

// Tracking is the aggregate root for one planner_task_id -> orca_task_id dispatch.
type Tracking struct {
	ID            string
	PlannerTaskID string // PlanTask.ID (CR-TASK-001, package plantask)
	PlannerJobID  string // Plan.ID
	OrcaTaskID    string // empty until SubmitTask succeeds
	Status        Status
	Result        *Result
	DispatchedAt  time.Time
	DeadlineAt    time.Time
	CompletedAt   *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// NewTracking creates a record in StatusPending, before the Orca HTTP call.
func NewTracking(plannerTaskID, plannerJobID string, timeout time.Duration) (*Tracking, error) {
	if plannerTaskID == "" {
		return nil, fmt.Errorf("planner_task_id required")
	}
	if timeout <= 0 {
		return nil, fmt.Errorf("timeout must be positive")
	}
	now := time.Now().UTC()
	return &Tracking{
		ID:            generateID(),
		PlannerTaskID: plannerTaskID,
		PlannerJobID:  plannerJobID,
		Status:        StatusPending,
		DispatchedAt:  now,
		DeadlineAt:    now.Add(timeout),
		CreatedAt:     now,
		UpdatedAt:     now,
	}, nil
}

// MarkSubmitted records the Orca task ID after a successful POST /api/planner-tasks.
func (t *Tracking) MarkSubmitted(orcaTaskID string) error {
	if t.Status != StatusPending {
		return fmt.Errorf("%w: cannot mark submitted from status %s", ErrInvalidTransition, t.Status)
	}
	if orcaTaskID == "" {
		return fmt.Errorf("orca_task_id required")
	}
	t.OrcaTaskID = orcaTaskID
	t.Status = StatusInProgress
	t.UpdatedAt = time.Now().UTC()
	return nil
}

// Complete transitions to a terminal status with its Orca result.
func (t *Tracking) Complete(status Status, result *Result) error {
	if !status.IsTerminal() {
		return fmt.Errorf("%w: %s is not terminal", ErrInvalidStatus, status)
	}
	now := time.Now().UTC()
	t.Status = status
	t.Result = result
	t.CompletedAt = &now
	t.UpdatedAt = now
	return nil
}

// IsTimedOut reports whether DeadlineAt has passed while still in progress.
// Checked manually at the top of every asynq poll round — unlike Temporal,
// asynq has no built-in StartToCloseTimeout equivalent (see SOL-ORCA-002 §2
// comparison table).
func (t *Tracking) IsTimedOut(now time.Time) bool {
	return t.Status == StatusInProgress && now.After(t.DeadlineAt)
}
```

## File 2: `internal/domain/orcatask/id.go`

```go
package orcatask

import "github.com/google/uuid"

func generateID() string { return uuid.NewString() }
```

## File 3: `internal/domain/orcatask/errors.go`

```go
package orcatask

import "errors"

var (
	ErrNotFound          = errors.New("orca task tracking not found")
	ErrInvalidTransition = errors.New("invalid tracking status transition")
	ErrInvalidStatus     = errors.New("invalid tracking status")
)
```

---

## File 4: `internal/domain/orcatask/repository.go`

```go
package orcatask

import "context"

// Repository persists Tracking records. See entity doc-comment — this is an
// operational/idempotency store, not the business source of truth.
type Repository interface {
	Save(ctx context.Context, t *Tracking) error
	FindByPlannerTaskID(ctx context.Context, plannerTaskID string) (*Tracking, error)
	FindByOrcaTaskID(ctx context.Context, orcaTaskID string) (*Tracking, error)
	UpdateStatus(ctx context.Context, orcaTaskID string, status Status, reason string) error
	UpdateResult(ctx context.Context, orcaTaskID string, status Status, result *Result) error
}
```

---

## Test File 5: `internal/domain/orcatask/entity_test.go`

```go
package orcatask_test

// Test cases bắt buộc — implement đầy đủ thân hàm:

func TestNewTracking_RequiresPlannerTaskID(t *testing.T)
func TestNewTracking_RequiresPositiveTimeout(t *testing.T)
func TestNewTracking_Success_StartsInPending(t *testing.T)
func TestTracking_MarkSubmitted_FromPending_Succeeds(t *testing.T)
func TestTracking_MarkSubmitted_FromInProgress_ReturnsErrInvalidTransition(t *testing.T)
func TestTracking_MarkSubmitted_EmptyOrcaTaskID_ReturnsError(t *testing.T)
func TestTracking_Complete_NonTerminalStatus_ReturnsErrInvalidStatus(t *testing.T)
func TestTracking_Complete_Terminal_SetsCompletedAtAndResult(t *testing.T)
func TestTracking_IsTimedOut_PastDeadlineWhileInProgress_ReturnsTrue(t *testing.T)
func TestTracking_IsTimedOut_BeforeDeadline_ReturnsFalse(t *testing.T)
func TestTracking_IsTimedOut_WhenTerminal_ReturnsFalse(t *testing.T)
```

## Test File 6: `internal/domain/orcatask/status_test.go`

```go
package orcatask_test

func TestStatus_IsTerminal_TrueForDoneBlockedCancelled(t *testing.T)
func TestStatus_IsTerminal_FalseForPendingInProgress(t *testing.T)
```

---

## Verification

```bash
cd /opt/repos/vnp-workplace/backend/services/planner-service

go build ./internal/domain/orcatask/...
go vet ./internal/domain/orcatask/...
go test ./internal/domain/orcatask/... -v -race -cover

go test ./internal/domain/orcatask/... -coverprofile=domain_cov.out
go tool cover -func=domain_cov.out | grep total   # kỳ vọng >= 90%
```
