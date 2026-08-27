> ⚠️ **SUPERSEDED (2026-08-10):** Tài liệu task/solution breakdown này được sinh ra từ các CR-ORCA-00x/CR-TASK-00x đã bị viết lại hoặc retired — dựa trên giả định sai rằng `vnp-planner` bị loại bỏ và `vnp-workplace` tự dựng `planner-service` mới. Kiến trúc đúng: xem [`docs/crs/v3/orca/README.md`](../../../../../docs/crs/v3/orca/README.md). Nội dung bên dưới chỉ còn giá trị tham khảo lịch sử — không dùng để implement.
>
> ---
>
# SOL-ORCA-002 — Planner → Orca Dispatcher (`planner-service`)

| Field | Value |
|-------|-------|
| **CR ref** | [CR-ORCA-002](../../../../../../docs/crs/v3/orca/CR-ORCA-002-planner-orca-dispatcher.md) |
| **Title** | Planner → Orca Dispatcher — `planner-service` gửi task sang Orca thật và theo dõi kết quả bằng asynq |
| **Service** | `planner-service` (`vnp-workplace`, **MỚI**, `:3013`) — Clean Architecture theo `backend/specs/tdd/v1/01-project-structure.md` |
| **Priority** | P0 |
| **Risk** | high |
| **Status** | 📐 PROPOSED |
| **TDD refs** | TDD-01 (Project Structure — per-service Clean Architecture template), TDD-00 §6–§8 (Interface Design, Configuration, Logger), TDD-07 §Worker & Queue (asynq — task definition/publisher/worker pattern thật của WKP), TDD-02 (migration/schema convention, goose) |
| **Depends on** | [SOL-ORCA-001](./SOL-ORCA-001-orca-api-bridge.md) (contract Orca); CR-TASK-001 (`docs/crs/v3/task-management/CR-TASK-001-task-planning-engine.md` — domain `plantask`/`plan` của `planner-service`, cung cấp `PlanTask` entity mà Change 4 gọi tới) |
| **Ghi chú re-scope (2026-08-10)** | Thay thế **hoàn toàn** thiết kế cũ dựa trên `vnp-planner`/`temporal-worker` (Temporal Workflow/Activity/Signal/Session). `planner-service` không có Temporal cluster — toàn bộ phần "chờ kết quả" trước đây dùng `workflow.Selector` (Signal + Timer) nay thay bằng **1 asynq task tự lên lịch lại chính nó** ("self-rescheduling task") mỗi 30s, xem CR-ORCA-002 §Problem Statement + §3.1. Không còn khái niệm "child workflow per task", "fan-out N task", hay "PlanDispatchWorkflow" — CR-ORCA-002 mới chỉ định nghĩa dispatch **1 task/1 lần gọi `DispatchToOrcaUseCase.Execute()`**; điều phối nhiều task của 1 Plan (nếu cần) là trách nhiệm của domain `plan`/`plantask` (CR-TASK-001..002), nằm ngoài phạm vi CR-ORCA-002. |

---

## 1. Tóm tắt vấn đề & mục tiêu

Trước re-scope, `temporal-worker` (Go, module `vnp-planner`) là service chịu trách nhiệm gửi task sang Orca và theo dõi tiến độ bằng Temporal Workflow. Sau quyết định kiến trúc 2026-08-10, `vnp-planner` bị loại bỏ; vai trò này chuyển vào `planner-service` (`vnp-workplace`, service Go **mới**, port `:3013`), dựng theo đúng layout Clean Architecture 4 lớp (`domain/application/infrastructure/presentation`) của TDD-01 — **không** dựng lại bất kỳ phần nào của Temporal.

Mục tiêu của SOL này — cụ thể hoá CR-ORCA-002 (đã viết lại 2026-08-10) thành kế hoạch thực thi theo từng file/layer:

1. **`OrcaClient`** — HTTP client gọi đúng contract Orca đã khoá ở SOL-ORCA-001 §3, đặt tại `internal/infrastructure/http/orcaclient/` (không phải package "dùng chung" `shared/pkg` như thiết kế cũ — `planner-service` là service Go duy nhất trong `vnp-workplace` gọi Orca ở thời điểm này; xem §9 Discoveries về việc không tái tạo package `shared`).
2. **`DispatchToOrcaUseCase`** — use case application-layer submit 1 task, thay thế hoàn toàn Temporal Activity `DispatchToOrca`.
3. **asynq self-rescheduling task** (`orca:poll_status`) — thay cho vòng lặp `workflow.Selector`(Signal+Timer) 24h trong Activity/Workflow cũ. Mỗi lượt chạy xong (`return nil`) rồi tự enqueue lại chính nó sau 30s nếu Orca task chưa terminal.
4. **`orcatask.Tracking`** — entity domain theo dõi map `PlannerTaskID ↔ OrcaTaskID ↔ trạng thái` (Postgres), thay cho Temporal workflow state + bảng `orca_task_dispatches` cũ.
5. **Tích hợp với domain `plan`/`plantask`** (CR-TASK-001) — khi 1 `PlanTask` cần dispatch tới AI agent, `plan.DispatchTaskUseCase.Execute()` gọi thẳng `DispatchToOrcaUseCase.Execute()` — lời gọi hàm Go bình thường, không qua hàng đợi/workflow.

---

## 2. Quyết định thiết kế chính

| # | Quyết định | Lý do |
|---|---|---|
| D1 | Bỏ hoàn toàn khái niệm "session"/"workflow"/"child workflow per task" — 1 lần dispatch = 1 bản ghi `orcatask.Tracking`, không có orchestration engine nào chen giữa | Khớp CR-ORCA-002 Change 4 ("không có bước workflow nào chen giữa `plan-svc` và `DispatchToOrcaUseCase`"); `planner-service` không có Temporal |
| D2 | **Bổ sung idempotency check** trong `DispatchToOrcaUseCase.Execute()` (kiểm tra `orcatask.Repository.FindByPlannerTaskID` trước khi gọi `orcaClient.SubmitTask`) — **CR-ORCA-002 gốc KHÔNG có bước này** | SOL-ORCA-001 §4/§6 yêu cầu bên gọi (`planner-service`) không được submit trùng khi retry; thiết kế cũ (Temporal) có được idempotency "miễn phí" qua activity retry + `FindByPlannerTaskID` check, thiết kế asynq mới (use case gọi trực tiếp, không tự động retry) sẽ **mất tính chất này** nếu không bổ sung tường minh — xem §9 Discoveries #1 |
| D3 | `OrcaPollStatusPayload` định nghĩa **DUY NHẤT** trong `internal/application/port/queue_publisher.go` (application layer), `infrastructure/queue/asynq` chỉ import và dùng lại — **CR-ORCA-002 gốc định nghĩa struct này 2 lần** (1 lần ngầm định qua `port.OrcaPollStatusPayload` dùng trong `dispatch_to_orca.go`/`handler_orca_poll.go`, 1 lần tường minh lặp lại trong `infrastructure/queue/asynq/tasks.go`) | Tránh duplicate type định nghĩa 2 nơi cho cùng 1 khái niệm — xem §9 Discoveries #2 |
| D4 | `orcatask.Repository.UpdateResult` nhận **domain-level** `orcatask.Result` (mapped từ `orcaclient.OrcaTaskStatus`/`OrcaTaskResult` tại application/infrastructure boundary), KHÔNG nhận thẳng DTO `orcaclient.OrcaTaskStatus` như code mẫu trong CR | TDD-00 §4 quy định domain layer zero-dependency ngoài stdlib; nhận thẳng DTO HTTP vào domain repository phá vỡ Clean Architecture layering (domain phụ thuộc ngược vào infra) — xem §9 Discoveries #3 |
| D5 | `EventPublisher` (publish `orca.task.submitted`/`orca.task.{status}`/`orca.task.blocked`) implement bằng **Redis Pub/Sub**, KHÔNG dùng NATS | WKP dùng Redis Pub/Sub cho toàn bộ event nội bộ (xem TDD-07 §Event Subscriber, `notification-service`), không có hạ tầng NATS trong `vnp-workplace` — CR-ORCA-002 không nói rõ cơ chế publish, chỉ khai báo interface `port.EventPublisher`; NATS là di sản thiết kế `vnp-planner` cũ, không mang sang — xem §9 Discoveries #4 |
| D6 | Bổ sung migration `planner.orca_task_tracking` (goose, schema `planner`) — **CR-ORCA-002 không có Change riêng cho migration** | `orcatask.Repository` cần bảng thật; theo TDD-02 §1/§8 convention (schema per service, `-- +goose Up/Down`, đặt tại `migrations/planner/`) — xem §9 Discoveries #5 |

### So sánh hành vi: Temporal Activity (cũ) vs asynq self-rescheduling task (mới)

> Giữ nguyên bảng so sánh gốc từ CR-ORCA-002 §3.1 — đây là phần giải thích quan trọng nhất cho engineer chưa quen asynq, không rút gọn.

| Khía cạnh | Thiết kế cũ (Temporal) | Thiết kế mới (asynq) |
|---|---|---|
| Đơn vị thực thi | 1 Activity execution sống liên tục tới khi terminal hoặc timeout 24h | Mỗi lượt poll là 1 asynq task hoàn chỉnh, `return nil` là kết thúc |
| Vòng lặp 30s | `for { select { case <-time.After(30s): } }` trong cùng 1 activity | Task tự `EnqueueOrcaPollStatus(..., 30*time.Second)` — không giữ goroutine sống giữa 2 lượt |
| "Còn sống" | `activity.RecordHeartbeat` | Không cần — trạng thái giữ trong payload task kế tiếp + Postgres |
| Retry khi lỗi | Activity retry replay lại toàn bộ activity | `asynq.MaxRetry` chỉ retry đúng lượt poll hiện tại |
| Timeout tổng (8-24h) | Temporal `StartToCloseTimeout`, do server enforce | Tự tính `DeadlineAt`, check thủ công đầu mỗi lượt poll (§3.8) |
| Crash worker | Temporal server tự resume | Task đã `ProcessIn` vẫn nằm trong Redis, worker restart pick up lại — không có "resume giữa activity", chỉ có "task tiếp theo trong Redis" |

---

## 3. Kiến trúc giải pháp

### 3.1 Cấu trúc thư mục

```
backend/services/planner-service/
├── cmd/
│   ├── server/main.go                              [MODIFY — thuộc CR-TASK-001, chỉ thêm route dispatch nếu có]
│   └── worker/main.go                              [NEW]    asynq worker — đăng ký orca:poll_status
├── config/
│   └── config.go                                   [MODIFY] thêm OrcaConfig — §3.11
├── internal/
│   ├── domain/
│   │   └── orcatask/
│   │       ├── entity.go                           [NEW] Tracking — §3.2
│   │       ├── repository.go                       [NEW] Repository interface — §3.3
│   │       └── errors.go                           [NEW]
│   ├── application/
│   │   ├── dispatch/
│   │   │   └── dispatch_to_orca.go                 [NEW] DispatchToOrcaUseCase — §3.7
│   │   ├── plan/
│   │   │   └── dispatch_task.go                    [MODIFY, thuộc CR-TASK-001] gọi thẳng use case — §3.10
│   │   └── port/
│   │       ├── orca_client.go                      [NEW] OrcaClient interface — §3.5
│   │       ├── queue_publisher.go                  [NEW] QueuePublisher + OrcaPollStatusPayload — §3.5
│   │       └── event_publisher.go                  [NEW] EventPublisher interface — §3.9
│   └── infrastructure/
│       ├── http/orcaclient/
│       │   ├── client.go                           [NEW] §3.6
│       │   ├── dto.go                               [NEW] §3.6
│       │   └── errors.go                            [NEW] §3.6
│       ├── queue/asynq/
│       │   ├── tasks.go                             [NEW] TaskOrcaPollStatus const — §3.8
│       │   ├── publisher.go                         [NEW] §3.8
│       │   └── handler_orca_poll.go                 [NEW] §3.8
│       ├── events/
│       │   └── redis_publisher.go                   [NEW] §3.9
│       └── persistence/postgres/
│           └── orcatask_repo.go                     [NEW] §3.3
└── migrations/planner/
    └── 00X_create_orca_task_tracking.sql            [NEW] §3.4 — số thứ tự = số tiếp theo sau các migration nền tảng plan/plantask của CR-TASK-001 tại thời điểm implement
```

### 3.2 Domain: `orcatask.Tracking` entity

`Tracking` là bản ghi vận hành/idempotency nội bộ của `planner-service` — theo dõi vòng đời **1 lần dispatch 1 task tới Orca**. Không phải nguồn sự thật nghiệp vụ của "task đã hoàn thành hay chưa" ở mức Plan — nguồn sự thật đó là `PlanTask.Status` (domain `plantask`, CR-TASK-001), được `DispatchTaskUseCase`/domain `plan` cập nhật sau khi nhận kết quả (§3.10). `Tracking` chỉ tồn tại để: (a) `DispatchToOrcaUseCase` biết đã submit task này chưa (idempotency, D2), (b) asynq poll handler biết `DeadlineAt`/`OrcaTaskID` giữa các lượt chạy.

**File:** `internal/domain/orcatask/entity.go`

```go
// Package orcatask defines the domain entity that tracks the lifecycle of one
// task dispatched to Orca — from submission through polling to a terminal
// result. This is an OPERATIONAL record (idempotency + poll-loop state), not
// the business source of truth for whether a PlanTask (CR-TASK-001, package
// plantask) is complete — that role belongs to PlanTask.Status, updated by
// application/plan.DispatchTaskUseCase after Tracking reaches a terminal state.
package orcatask

import (
	"fmt"
	"time"
)

type Status string

const (
	StatusPending    Status = "pending"     // created, not yet submitted to Orca
	StatusInProgress Status = "in_progress" // submitted, polling
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
// needs — kept separate from the infra DTO (see SOL-ORCA-002 §2 D4) so
// internal/domain never imports internal/infrastructure/http/orcaclient.
type Result struct {
	Success       bool
	FilesCreated  []string
	FilesModified []string
	CommitHash    string
	TestOutput    string
	ErrorMessage  string
}

// Tracking is the aggregate root for one planner_task_id → orca_task_id dispatch.
type Tracking struct {
	ID            string
	PlannerTaskID string // PlanTask.ID (CR-TASK-001, package plantask)
	PlannerJobID  string // Plan.ID — "job" terminology kept for CR-ORCA-002 field-name parity
	OrcaTaskID    string // empty until SubmitTask succeeds
	Status        Status
	Result        *Result
	DispatchedAt  time.Time
	DeadlineAt    time.Time
	CompletedAt   *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// NewTracking creates a record in PENDING state, before the Orca HTTP call.
func NewTracking(plannerTaskID, plannerJobID string, timeout time.Duration) (*Tracking, error) {
	if plannerTaskID == "" {
		return nil, fmt.Errorf("planner_task_id required")
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

// IsTimedOut reports whether DeadlineAt has passed while still in progress —
// checked manually at the top of every poll round (asynq has no built-in
// StartToCloseTimeout equivalent, see SOL-ORCA-002 §2 comparison table).
func (t *Tracking) IsTimedOut(now time.Time) bool {
	return t.Status == StatusInProgress && now.After(t.DeadlineAt)
}
```

`generateID()` — `github.com/google/uuid`, file riêng `internal/domain/orcatask/id.go`, cùng pattern các domain khác trong WKP.

**File:** `internal/domain/orcatask/errors.go`

```go
package orcatask

import "errors"

var (
	ErrNotFound          = errors.New("orca task tracking not found")
	ErrInvalidTransition = errors.New("invalid tracking status transition")
	ErrInvalidStatus     = errors.New("invalid tracking status")
)
```

### 3.3 Repository interface + Postgres implementation

**File:** `internal/domain/orcatask/repository.go`

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

**File:** `internal/infrastructure/persistence/postgres/orcatask_repo.go`

Theo pattern TDD-00 §4 (Row có `db` tags, mapper `toRow`/`toDomain`, wrap lỗi `sql.ErrNoRows → orcatask.ErrNotFound` tại boundary):

```go
package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"

	"github.com/vnptech/kwp/services/planner-service/internal/domain/orcatask"
)

type orcaTaskTrackingRow struct {
	ID            string         `db:"id"`
	PlannerTaskID string         `db:"planner_task_id"`
	PlannerJobID  sql.NullString `db:"planner_job_id"`
	OrcaTaskID    sql.NullString `db:"orca_task_id"`
	Status        string         `db:"status"`
	ResultJSON    []byte         `db:"result"` // JSONB, NULL until terminal
	DispatchedAt  sql.NullTime   `db:"dispatched_at"`
	DeadlineAt    sql.NullTime   `db:"deadline_at"`
	CompletedAt   sql.NullTime   `db:"completed_at"`
	CreatedAt     sql.NullTime   `db:"created_at"`
	UpdatedAt     sql.NullTime   `db:"updated_at"`
}

// toDomain/toRow map orcaTaskTrackingRow ↔ orcatask.Tracking; Result marshals
// to/from the `result` JSONB column via encoding/json (mapper.go, omitted here
// for brevity — implement per TDD-00 §11 file naming: mapper.go).

type OrcaTaskTrackingRepository struct {
	db *sqlx.DB
}

func NewOrcaTaskTrackingRepository(db *sqlx.DB) orcatask.Repository {
	return &OrcaTaskTrackingRepository{db: db}
}

func (r *OrcaTaskTrackingRepository) Save(ctx context.Context, t *orcatask.Tracking) error {
	row := toRow(t)
	_, err := r.db.NamedExecContext(ctx, `
		INSERT INTO planner.orca_task_tracking (
			id, planner_task_id, planner_job_id, orca_task_id, status,
			dispatched_at, deadline_at, created_at, updated_at
		) VALUES (
			:id, :planner_task_id, :planner_job_id, :orca_task_id, :status,
			:dispatched_at, :deadline_at, :created_at, :updated_at
		)`, row)
	if err != nil {
		return fmt.Errorf("inserting orca_task_tracking: %w", err)
	}
	return nil
}

func (r *OrcaTaskTrackingRepository) FindByPlannerTaskID(ctx context.Context, plannerTaskID string) (*orcatask.Tracking, error) {
	var row orcaTaskTrackingRow
	err := r.db.GetContext(ctx, &row,
		`SELECT * FROM planner.orca_task_tracking WHERE planner_task_id = $1`, plannerTaskID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, orcatask.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("querying orca_task_tracking by planner_task_id: %w", err)
	}
	return row.toDomain(), nil
}

func (r *OrcaTaskTrackingRepository) FindByOrcaTaskID(ctx context.Context, orcaTaskID string) (*orcatask.Tracking, error) {
	var row orcaTaskTrackingRow
	err := r.db.GetContext(ctx, &row,
		`SELECT * FROM planner.orca_task_tracking WHERE orca_task_id = $1`, orcaTaskID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, orcatask.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("querying orca_task_tracking by orca_task_id: %w", err)
	}
	return row.toDomain(), nil
}

func (r *OrcaTaskTrackingRepository) UpdateStatus(ctx context.Context, orcaTaskID string, status orcatask.Status, reason string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE planner.orca_task_tracking
		SET status = $2, completed_at = CASE WHEN $3 THEN NOW() ELSE completed_at END, updated_at = NOW()
		WHERE orca_task_id = $1`, orcaTaskID, string(status), status.IsTerminal())
	if err != nil {
		return fmt.Errorf("updating orca_task_tracking status: %w", err)
	}
	return nil
}

func (r *OrcaTaskTrackingRepository) UpdateResult(ctx context.Context, orcaTaskID string, status orcatask.Status, result *orcatask.Result) error {
	data, err := marshalResult(result) // encoding/json, NULL-safe
	if err != nil {
		return fmt.Errorf("marshaling orca task result: %w", err)
	}
	_, err = r.db.ExecContext(ctx, `
		UPDATE planner.orca_task_tracking
		SET status = $2, result = $3, completed_at = NOW(), updated_at = NOW()
		WHERE orca_task_id = $1`, orcaTaskID, string(status), data)
	if err != nil {
		return fmt.Errorf("updating orca_task_tracking result: %w", err)
	}
	return nil
}
```

### 3.4 Migration — `planner.orca_task_tracking`

> **Không có trong CR-ORCA-002 gốc** (xem §2 D6 / §9 Discoveries #5). Schema `planner` là schema mới cho `planner-service` (chưa liệt kê trong TDD-02 v3.0, vốn viết trước khi `planner-service` tồn tại — CR-TASK-001 tạo migration nền tảng `plan_node`/`goal_node`/`plan_task` trước, migration này **land sau** migration nền tảng đó; xác nhận số thứ tự thật bằng `ls migrations/planner/` tại thời điểm implement, không hard-code số ở đây).

```sql
-- migrations/planner/00X_create_orca_task_tracking.sql
-- +goose Up
CREATE TABLE IF NOT EXISTS planner.orca_task_tracking (
    id              UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),
    planner_task_id VARCHAR(64)  NOT NULL,      -- PlanTask.ID (CR-TASK-001)
    planner_job_id  VARCHAR(64),                -- Plan.ID
    orca_task_id    VARCHAR(128),               -- NULL until SubmitTask succeeds
    status          VARCHAR(20)  NOT NULL DEFAULT 'pending',
    result          JSONB,                      -- orcatask.Result, NULL until terminal
    dispatched_at   TIMESTAMPTZ  NOT NULL,
    deadline_at     TIMESTAMPTZ  NOT NULL,
    completed_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_orca_task_tracking_planner_task
    ON planner.orca_task_tracking(planner_task_id);

CREATE INDEX IF NOT EXISTS idx_orca_task_tracking_orca_task
    ON planner.orca_task_tracking(orca_task_id);

CREATE INDEX IF NOT EXISTS idx_orca_task_tracking_active
    ON planner.orca_task_tracking(status) WHERE status = 'in_progress';

-- +goose Down
DROP TABLE IF EXISTS planner.orca_task_tracking;
```

Không có RLS/`org_id` — `PlanTask` (CR-TASK-001) hiện không mang trường org/workspace nào; nếu multi-tenant RLS cần cho `planner-service` sau này, bổ sung cột `workspace_id` + policy cùng lúc với các bảng `plan`/`plantask` khác (không làm riêng lẻ ở bảng tracking này).

### 3.5 `internal/application/port` — interfaces

**File:** `internal/application/port/orca_client.go`

```go
package port

import (
	"context"

	"github.com/vnptech/kwp/services/planner-service/internal/infrastructure/http/orcaclient"
)

// OrcaClient — app-level interface; implemented by infrastructure/http/orcaclient.Client.
// Application layer depends on this abstraction (TDD-00 §6 Interface Segregation),
// never imports infrastructure/http/orcaclient directly outside of wiring (cmd/*/main.go).
type OrcaClient interface {
	SubmitTask(ctx context.Context, req orcaclient.OrcaTaskRequest) (*orcaclient.OrcaTaskResponse, error)
	GetTaskStatus(ctx context.Context, orcaTaskID string) (*orcaclient.OrcaTaskStatus, error)
	CancelTask(ctx context.Context, orcaTaskID string) error
}
```

**File:** `internal/application/port/queue_publisher.go`

```go
package port

import (
	"context"
	"time"
)

// OrcaPollStatusPayload carries state between poll rounds of the self-rescheduling
// orca:poll_status asynq task (replaces Temporal activity heartbeat/context — see
// SOL-ORCA-002 §2 comparison table). Defined ONCE here (application layer) — see
// §2 D3 / §9 Discoveries #2: CR-ORCA-002 duplicated this type in the infra
// asynq package, which this SOL corrects by having infra import this one.
type OrcaPollStatusPayload struct {
	OrcaTaskID    string    `json:"orca_task_id"`
	PlannerTaskID string    `json:"planner_task_id"`
	DeadlineAt    time.Time `json:"deadline_at"`
}

// QueuePublisher enqueues (or re-enqueues) a poll round. Implemented by
// infrastructure/queue/asynq.Publisher.
type QueuePublisher interface {
	EnqueueOrcaPollStatus(ctx context.Context, payload OrcaPollStatusPayload, delay time.Duration) error
}
```

**File:** `internal/application/port/event_publisher.go`

```go
package port

import "context"

// EventPublisher publishes integration events. Implemented by
// infrastructure/events (Redis Pub/Sub — see §2 D5 / §9 Discoveries #4).
type EventPublisher interface {
	Publish(ctx context.Context, subject string, payload any) error
}
```

### 3.6 `internal/infrastructure/http/orcaclient` — Orca HTTP client

Nội dung **không đổi** so với CR-ORCA-002 Change 1/2 (chỉ đổi import path, đã đóng băng theo contract SOL-ORCA-001 §3). Giữ nguyên logic thuần HTTP, không phụ thuộc Temporal:

- `client.go` — `Client{baseURL, apiSecret, httpClient, logger *slog.Logger}` (slog, TDD-00 §8 — **không** dùng zap như thiết kế `temporal-worker` cũ), 3 method `SubmitTask`/`GetTaskStatus`/`CancelTask` — xem CR-ORCA-002 Change 1 cho code đầy đủ, giữ nguyên khi implement.
- `dto.go` — `OrcaTaskRequest`/`OrcaTaskResponse`/`OrcaTaskStatus`/`OrcaTaskResult`, json tags `snake_case` khớp SOL-ORCA-001 §3.2–§3.3 — xem CR-ORCA-002 Change 2.
- `errors.go` **[bổ sung — không có trong CR Change 1/2]**: map status code → typed errors, theo đúng pattern đã kiểm chứng ở thiết kế `temporal-worker` cũ (`ErrUnauthorized`/`ErrNotFound`/`ErrConflict`/`ErrUnprocessable`/`ErrUnavailable` + `IsRetryable`) — CR-ORCA-002 mới không nhắc lại bảng lỗi này tường minh nhưng bảng mã lỗi ở SOL-ORCA-001 §3.2 vẫn là hợp đồng bắt buộc; giữ nguyên cách map lỗi cũ (đã test kỹ ở SOL-ORCA-002 v1), chỉ đổi package.

```go
// internal/infrastructure/http/orcaclient/errors.go
package orcaclient

import (
	"errors"
	"fmt"
	"net/http"
)

var (
	ErrUnauthorized  = errors.New("orca: unauthorized (check ORCA_PLANNER_API_SECRET)")
	ErrNotFound      = errors.New("orca: task not found")
	ErrConflict      = errors.New("orca: duplicate planner_task_id")
	ErrUnprocessable = errors.New("orca: agent type unavailable")
	ErrUnavailable   = errors.New("orca: server busy or unreachable") // retryable
)

func mapStatusError(resp *http.Response) error {
	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return ErrUnauthorized
	case http.StatusNotFound:
		return ErrNotFound
	case http.StatusConflict:
		return ErrConflict
	case http.StatusUnprocessableEntity:
		return ErrUnprocessable
	case http.StatusServiceUnavailable, http.StatusTooManyRequests:
		return ErrUnavailable
	default:
		return fmt.Errorf("orca: unexpected status %d", resp.StatusCode)
	}
}

// IsRetryable reports whether the caller should retry (e.g. re-enqueue a poll
// round, or — for SubmitTask — surface a retryable error up to the plan
// domain caller). Only transient server-side capacity issues are retryable.
func IsRetryable(err error) bool {
	return errors.Is(err, ErrUnavailable)
}
```

### 3.7 `internal/application/dispatch` — `DispatchToOrcaUseCase`

Thay thế hoàn toàn Temporal Activity `DispatchToOrca`. **Khác với CR-ORCA-002 Change 3.2 gốc**: bổ sung bước idempotency check (D2) trước khi gọi `orcaClient.SubmitTask`.

```go
// internal/application/dispatch/dispatch_to_orca.go
package dispatch

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/vnptech/kwp/services/planner-service/internal/application/port"
	"github.com/vnptech/kwp/services/planner-service/internal/domain/orcatask"
	"github.com/vnptech/kwp/services/planner-service/internal/infrastructure/http/orcaclient"
)

type DispatchToOrcaUseCase struct {
	orcaClient port.OrcaClient
	repo       orcatask.Repository
	publisher  port.QueuePublisher
	eventPub   port.EventPublisher
	cfg        Config
}

type Config struct {
	CallbackURL         string
	DefaultTimeoutHours int
}

func NewDispatchToOrcaUseCase(c port.OrcaClient, r orcatask.Repository, p port.QueuePublisher, ep port.EventPublisher, cfg Config) *DispatchToOrcaUseCase {
	return &DispatchToOrcaUseCase{orcaClient: c, repo: r, publisher: p, eventPub: ep, cfg: cfg}
}

type DispatchToOrcaInput struct {
	PlannerTaskID      string
	PlannerJobID       string
	CRID               string
	Title              string
	TaskFileContent    string
	WorktreeRepo       string
	WorktreeBranch     string
	AgentType          string
	Priority           string
	WHYChain           []string
	AntiPatterns       []string
	RequiredPatterns   []string
	AcceptanceCriteria []string
	TimeoutHours       int // 0 = dùng cfg.DefaultTimeoutHours
}

type DispatchToOrcaResult struct {
	OrcaTaskID string
	Status     string
}

// Execute submits a task to Orca and starts the poll chain. Does NOT block
// until Orca finishes — returns as soon as the first poll round is enqueued
// (asynchronous by design, mirrors sync-service's TaskSyncIncremental enqueue
// pattern, TDD-07).
func (uc *DispatchToOrcaUseCase) Execute(ctx context.Context, input DispatchToOrcaInput) (*DispatchToOrcaResult, error) {
	// Idempotency guard — NOT present in CR-ORCA-002 §3.2 original; added per
	// SOL-ORCA-002 §2 D2. Without this, a caller retry (e.g. plan.DispatchTaskUseCase
	// re-invoked after a timeout on its side) would submit a duplicate task to Orca,
	// which SOL-ORCA-001 §4 explicitly tries to prevent via 409 — but relying on
	// Orca's 409 alone means paying for a duplicate LLM-agent spin-up attempt every
	// retry. Check locally first.
	if existing, err := uc.repo.FindByPlannerTaskID(ctx, input.PlannerTaskID); err == nil && existing.OrcaTaskID != "" {
		return &DispatchToOrcaResult{OrcaTaskID: existing.OrcaTaskID, Status: string(existing.Status)}, nil
	} else if err != nil && !errors.Is(err, orcatask.ErrNotFound) {
		return nil, fmt.Errorf("checking existing dispatch for %q: %w", input.PlannerTaskID, err)
	}

	timeoutHours := resolveTimeout(input.TimeoutHours, uc.cfg.DefaultTimeoutHours)
	deadline := time.Now().Add(time.Duration(timeoutHours) * time.Hour)

	tracking, err := orcatask.NewTracking(input.PlannerTaskID, input.PlannerJobID, time.Duration(timeoutHours)*time.Hour)
	if err != nil {
		return nil, fmt.Errorf("creating tracking record: %w", err)
	}
	if err := uc.repo.Save(ctx, tracking); err != nil {
		return nil, fmt.Errorf("saving tracking record: %w", err)
	}

	req := orcaclient.OrcaTaskRequest{
		PlannerTaskID:      input.PlannerTaskID,
		PlannerJobID:       input.PlannerJobID,
		PlannerCRID:        input.CRID,
		Title:              input.Title,
		Description:        input.TaskFileContent,
		WorktreeRepo:       input.WorktreeRepo,
		WorktreeBranch:     input.WorktreeBranch,
		AgentType:          selectAgentType(input.AgentType),
		Priority:           input.Priority,
		WHYChain:           input.WHYChain,
		AntiPatterns:       input.AntiPatterns,
		RequiredPatterns:   input.RequiredPatterns,
		AcceptanceCriteria: input.AcceptanceCriteria,
		CallbackURL:        uc.cfg.CallbackURL,
		TimeoutHours:       timeoutHours,
	}

	resp, err := uc.orcaClient.SubmitTask(ctx, req)
	if err != nil {
		if errors.Is(err, orcaclient.ErrConflict) {
			// Orca already has this planner_task_id (SOL-ORCA-001 §4) — recover
			// via GetTaskStatus instead of failing the whole dispatch.
			status, statusErr := uc.orcaClient.GetTaskStatus(ctx, input.PlannerTaskID)
			if statusErr == nil {
				_ = tracking.MarkSubmitted(status.OrcaTaskID)
				_ = uc.repo.Save(ctx, tracking) // best-effort; UpdateStatus can be used if Save is insert-only
				return &DispatchToOrcaResult{OrcaTaskID: status.OrcaTaskID, Status: status.Status}, nil
			}
		}
		return nil, fmt.Errorf("orca submit failed for %q: %w", input.PlannerTaskID, err)
	}

	if err := tracking.MarkSubmitted(resp.OrcaTaskID); err != nil {
		return nil, fmt.Errorf("marking tracking submitted: %w", err)
	}
	if err := uc.repo.UpdateStatus(ctx, resp.OrcaTaskID, orcatask.StatusInProgress, ""); err != nil {
		return nil, fmt.Errorf("persisting submitted status: %w", err)
	}
	_ = uc.eventPub.Publish(ctx, "orca.task.submitted", tracking)

	// Enqueue the first poll round — 30s from now. Start of the self-rescheduling chain.
	if err := uc.publisher.EnqueueOrcaPollStatus(ctx, port.OrcaPollStatusPayload{
		OrcaTaskID:    resp.OrcaTaskID,
		PlannerTaskID: input.PlannerTaskID,
		DeadlineAt:    deadline,
	}, 30*time.Second); err != nil {
		return nil, fmt.Errorf("enqueue orca poll: %w", err)
	}

	return &DispatchToOrcaResult{OrcaTaskID: resp.OrcaTaskID, Status: "pending"}, nil
}

func resolveTimeout(inputHours, defaultHours int) int {
	if inputHours > 0 {
		return inputHours
	}
	if defaultHours > 0 {
		return defaultHours
	}
	return 8
}

func selectAgentType(plannerAgentType string) string {
	switch plannerAgentType {
	case "ai-developer", "ai-architect":
		return "claude"
	case "ai-reviewer":
		return "codex"
	default:
		return "claude"
	}
}
```

### 3.8 `internal/infrastructure/queue/asynq` — self-rescheduling poll task

Nội dung khớp CR-ORCA-002 Change 3.3, sửa 2 điểm: (a) dùng `port.OrcaPollStatusPayload` thay vì định nghĩa lại (D3), (b) `UpdateResult`/`UpdateStatus` nhận domain `orcatask.Result`/`orcatask.Status`, map từ `orcaclient.OrcaTaskStatus` ngay trong handler (D4) thay vì truyền thẳng DTO xuống repo.

> **Lưu ý implement:** package tên `asynq` (khớp TDD-07 §Task Definitions, cùng tên thư mục) trùng tên với thư viện `github.com/hibiken/asynq` — import thư viện với alias, ví dụ `hibikenasynq "github.com/hibiken/asynq"`, để tránh xung đột tên trong chính package này.

**File:** `internal/infrastructure/queue/asynq/tasks.go`

```go
package asynq

// TaskOrcaPollStatus — self-rescheduling task type. Payload type is
// port.OrcaPollStatusPayload (application layer) — NOT redefined here, see
// SOL-ORCA-002 §2 D3.
const TaskOrcaPollStatus = "orca:poll_status"
```

**File:** `internal/infrastructure/queue/asynq/publisher.go`

```go
package asynq

import (
	"context"
	"encoding/json"
	"time"

	hibikenasynq "github.com/hibiken/asynq"

	"github.com/vnptech/kwp/services/planner-service/internal/application/port"
)

type Publisher struct {
	client *hibikenasynq.Client
}

func NewPublisher(redisAddr string) *Publisher {
	return &Publisher{client: hibikenasynq.NewClient(hibikenasynq.RedisClientOpt{Addr: redisAddr})}
}

// EnqueueOrcaPollStatus enqueues (or re-enqueues) one poll round, to run after `delay`.
func (p *Publisher) EnqueueOrcaPollStatus(ctx context.Context, payload port.OrcaPollStatusPayload, delay time.Duration) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	task := hibikenasynq.NewTask(TaskOrcaPollStatus, data,
		hibikenasynq.MaxRetry(3),        // retry ONLY this poll round's GetTaskStatus call
		hibikenasynq.Timeout(45*time.Second),
		hibikenasynq.Queue("orca_poll"),
		hibikenasynq.ProcessIn(delay),   // self-reschedule mechanism
	)
	_, err = p.client.EnqueueContext(ctx, task)
	return err
}
```

**File:** `internal/infrastructure/queue/asynq/handler_orca_poll.go`

```go
package asynq

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	hibikenasynq "github.com/hibiken/asynq"

	"github.com/vnptech/kwp/services/planner-service/internal/application/port"
	"github.com/vnptech/kwp/services/planner-service/internal/domain/orcatask"
)

// NewOrcaPollStatusHandler handles EXACTLY ONE poll round. Pattern: "self-
// rescheduling task" — if the Orca task is not yet terminal, the handler
// re-enqueues TaskOrcaPollStatus (ProcessIn 30s) before returning nil, instead
// of keeping a goroutine alive for up to 24h like the original Temporal Activity.
func NewOrcaPollStatusHandler(
	orcaClient port.OrcaClient,
	repo orcatask.Repository,
	publisher *Publisher,
	eventPub port.EventPublisher,
) hibikenasynq.HandlerFunc {
	return func(ctx context.Context, t *hibikenasynq.Task) error {
		var payload port.OrcaPollStatusPayload
		if err := json.Unmarshal(t.Payload(), &payload); err != nil {
			return fmt.Errorf("unmarshal orca poll payload: %w", err)
		}

		// Overall timeout (equivalent of Temporal's StartToCloseTimeout) — must be
		// checked manually since asynq holds no state across multiple task runs.
		if time.Now().After(payload.DeadlineAt) {
			slog.Warn("orca task timeout", "orca_task_id", payload.OrcaTaskID)
			_ = orcaClient.CancelTask(context.Background(), payload.OrcaTaskID)
			_ = repo.UpdateStatus(ctx, payload.OrcaTaskID, orcatask.StatusBlocked, "timeout waiting for Orca")
			_ = eventPub.Publish(ctx, "orca.task.blocked", map[string]any{
				"orca_task_id": payload.OrcaTaskID, "reason": "timeout",
			})
			return nil // end of chain — do NOT reschedule
		}

		status, err := orcaClient.GetTaskStatus(ctx, payload.OrcaTaskID)
		if err != nil {
			// Transient (network/5xx): return error so asynq retries ONLY this poll
			// round (hibikenasynq.MaxRetry(3), configured in publisher.go). Prior/
			// future rounds are unaffected.
			return fmt.Errorf("poll orca status: %w", err)
		}

		if isTerminal(status.Status) {
			domainStatus := orcatask.Status(status.Status)
			result := toDomainResult(status.Result) // maps orcaclient.OrcaTaskResult → orcatask.Result (§2 D4)
			if err := repo.UpdateResult(ctx, payload.OrcaTaskID, domainStatus, result); err != nil {
				return fmt.Errorf("persist orca result: %w", err)
			}
			_ = eventPub.Publish(ctx, "orca.task."+status.Status, map[string]any{
				"orca_task_id":    payload.OrcaTaskID,
				"planner_task_id": payload.PlannerTaskID,
			})
			return nil // terminal — stop the chain
		}

		// Not done yet: reschedule this task type after 30s.
		if err := publisher.EnqueueOrcaPollStatus(ctx, payload, 30*time.Second); err != nil {
			return fmt.Errorf("reschedule orca poll: %w", err)
		}
		return nil
	}
}

func isTerminal(status string) bool {
	return status == "done" || status == "blocked" || status == "cancelled"
}
```

**File:** `cmd/worker/main.go`

```go
package main

import (
	hibikenasynq "github.com/hibiken/asynq"

	"github.com/vnptech/kwp/services/planner-service/config"
	asynqinfra "github.com/vnptech/kwp/services/planner-service/internal/infrastructure/queue/asynq"
	// ... wire orcaClient, repo, eventPub (xem §3.12)
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}
	// ... wire orcaClient, repo, publisher, eventPub — xem §3.12 cho DI đầy đủ

	srv := hibikenasynq.NewServer(
		hibikenasynq.RedisClientOpt{Addr: cfg.RedisURL},
		hibikenasynq.Config{
			Concurrency: 5,
			Queues:      map[string]int{"orca_poll": 10, "default": 1},
		},
	)

	mux := hibikenasynq.NewServeMux()
	mux.HandleFunc(asynqinfra.TaskOrcaPollStatus, asynqinfra.NewOrcaPollStatusHandler(orcaClient, repo, publisher, eventPub))

	if err := srv.Run(mux); err != nil {
		panic(err)
	}
}
```

### 3.9 `internal/infrastructure/events` — Redis Pub/Sub `EventPublisher`

**Không có trong CR-ORCA-002** (interface `port.EventPublisher` được dùng nhưng cách publish không được chỉ định — CR di sản dùng NATS, WKP không có NATS, xem §2 D5). Implement theo pattern TDD-07 §Event Subscriber (đối xứng — bên publish, không phải bên subscribe):

```go
// internal/infrastructure/events/redis_publisher.go
package events

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/redis/go-redis/v9"
)

// RedisPublisher implements port.EventPublisher via Redis Pub/Sub — same
// transport notification-service's EventSubscriber listens on (TDD-07).
type RedisPublisher struct {
	rdb *redis.Client
}

func NewRedisPublisher(rdb *redis.Client) *RedisPublisher {
	return &RedisPublisher{rdb: rdb}
}

func (p *RedisPublisher) Publish(ctx context.Context, subject string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshaling event payload for %q: %w", subject, err)
	}
	if err := p.rdb.Publish(ctx, subject, data).Err(); err != nil {
		return fmt.Errorf("publishing event %q: %w", subject, err)
	}
	return nil
}
```

Channel/subject dùng nguyên tên đã publish trong §3.7/§3.8: `orca.task.submitted`, `orca.task.done`, `orca.task.blocked`, `orca.task.cancelled`. `diagnostics-service` (thay `signal-svc` cũ, xem README re-scope) là consumer tiềm năng — nằm trong phạm vi CR-ORCA-005, không thiết kế subscriber ở SOL này.

### 3.10 `internal/application/plan` — tích hợp domain `plan` (Change 4 của CR)

Đúng theo CR-ORCA-002 Change 4: domain `plan` (CR-TASK-001) gọi thẳng `DispatchToOrcaUseCase.Execute()` như một lời gọi hàm Go bình thường — **không qua hàng đợi, không qua workflow engine**.

```go
// internal/application/plan/dispatch_task.go
package plan

import (
	"context"
	"fmt"

	"github.com/vnptech/kwp/services/planner-service/internal/application/dispatch"
	"github.com/vnptech/kwp/services/planner-service/internal/domain/plantask" // CR-TASK-001
)

// DispatchTaskUseCase — khi 1 PlanTask (CR-TASK-001) chuyển sang trạng thái cần
// AI agent thực thi, gọi thẳng DispatchToOrcaUseCase — KHÔNG qua workflow engine.
type DispatchTaskUseCase struct {
	taskRepo         plantask.Repository
	dispatchToOrcaUC *dispatch.DispatchToOrcaUseCase
}

func NewDispatchTaskUseCase(tr plantask.Repository, d *dispatch.DispatchToOrcaUseCase) *DispatchTaskUseCase {
	return &DispatchTaskUseCase{taskRepo: tr, dispatchToOrcaUC: d}
}

func (uc *DispatchTaskUseCase) Execute(ctx context.Context, taskID string) error {
	t, err := uc.taskRepo.FindByID(ctx, taskID)
	if err != nil {
		return err
	}

	// NOTE: PlanTask (CR-TASK-001 §Change 1) hiện KHÔNG có các trường
	// WorktreeRepo/WorktreeBranch/EnrichedContext/AcceptanceCriteria mà
	// DispatchToOrcaInput cần — CR-ORCA-002 Change 4 tham chiếu các trường này
	// như thể đã tồn tại trên PlanTask. Đây là điểm cần CR-TASK-003 (Task
	// Context Enrichment) bổ sung trước khi task này chạy được thật — xem §9
	// Discoveries #6. Mapping bên dưới giả định các trường đó đã có (qua
	// CR-TASK-003), không phát minh thêm field mới ngoài những gì 2 CR đã nêu.
	result, err := uc.dispatchToOrcaUC.Execute(ctx, dispatch.DispatchToOrcaInput{
		PlannerTaskID:      t.ID,
		PlannerJobID:       t.PlanID,
		CRID:               t.ExternalRef, // KGP TASK ID — dùng làm định danh CR-facing gần nhất có sẵn trên PlanTask hôm nay
		Title:              t.Title,
		TaskFileContent:    t.Description,
		AgentType:          t.AgentType,
		Priority:           t.Priority,
		WHYChain:           t.WHYChain,
		TimeoutHours:       8,
	})
	if err != nil {
		return fmt.Errorf("dispatch task %s to orca: %w", taskID, err)
	}

	// TODO(CR-TASK-001): PlanTask cần thêm field OrcaTaskID (hoặc bảng phụ) để
	// domain plan tự tra cứu dispatch hiện tại — plantask.Repository hiện chưa
	// có UpdateOrcaTaskID; orcatask.Tracking (§3.2) đã giữ mapping này, domain
	// plan có thể đọc qua orcatask.Repository.FindByPlannerTaskID thay vì lưu
	// trùng OrcaTaskID trên PlanTask — quyết định cụ thể thuộc CR-TASK-001/004.
	_ = result
	return nil
}
```

### 3.11 Config

```go
// config/config.go [MODIFY — chỉ phần bổ sung Orca]
package config

import "github.com/kelseyhightower/envconfig"

type Config struct {
	Port        string `envconfig:"PORT" default:"3013"`
	Env         string `envconfig:"ENV" default:"development"`
	DatabaseURL string `envconfig:"DATABASE_URL" required:"true"`
	RedisURL    string `envconfig:"REDIS_URL" required:"true"`

	// Orca integration
	OrcaURL                 string `envconfig:"ORCA_URL" required:"true"`
	// NOTE: default gợi ý trong comment gốc của CR-ORCA-002 §Change 5 là
	// "http://orca:3000" — SAI, đã kiểm chứng với Orca thật ở SOL-ORCA-001 §9:
	// port HTTP thật là :6769. Sửa tại đây — xem §9 Discoveries #7.
	// gợi ý: http://orca:6769
	OrcaAPISecret           string `envconfig:"ORCA_PLANNER_API_SECRET" required:"true"`
	OrcaCallbackURL         string `envconfig:"ORCA_CALLBACK_URL" default:"http://planner-service:3013/api/v1/orca-callback"`
	OrcaPollIntervalSecs    int    `envconfig:"ORCA_POLL_INTERVAL_SECS" default:"30"`
	OrcaDefaultAgentType    string `envconfig:"ORCA_DEFAULT_AGENT_TYPE" default:"claude"`
	OrcaDefaultTimeoutHours int    `envconfig:"ORCA_DEFAULT_TIMEOUT_HOURS" default:"8"`
}

func Load() (*Config, error) {
	var cfg Config
	if err := envconfig.Process("", &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
```

`.env.example` bổ sung — giữ nguyên như CR-ORCA-002 Change 5, chỉ sửa giá trị mẫu `ORCA_URL`:

```bash
# planner-service — Orca integration
ORCA_URL=http://orca:6769
ORCA_PLANNER_API_SECRET=<strong-random-secret>
ORCA_CALLBACK_URL=http://planner-service:3013/api/v1/orca-callback
ORCA_POLL_INTERVAL_SECS=30
ORCA_DEFAULT_AGENT_TYPE=claude
ORCA_DEFAULT_TIMEOUT_HOURS=8
```

### 3.12 Wiring — `cmd/worker/main.go` (đầy đủ) và điểm chạm `cmd/server/main.go`

```go
// cmd/worker/main.go — đầy đủ, thay bản rút gọn ở §3.8
func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	slogger.Init(cfg.Env) // TDD-00 §8

	db := mustConnectPostgres(cfg.DatabaseURL)     // pgx/sqlx pool, TDD-02 §9
	rdb := mustConnectRedis(cfg.RedisURL)

	orcaHTTP := orcaclient.New(cfg.OrcaURL, cfg.OrcaAPISecret, 15*time.Second)
	repo := postgres.NewOrcaTaskTrackingRepository(db)
	publisher := asynqinfra.NewPublisher(cfg.RedisURL)
	eventPub := events.NewRedisPublisher(rdb)

	srv := hibikenasynq.NewServer(
		hibikenasynq.RedisClientOpt{Addr: cfg.RedisURL},
		hibikenasynq.Config{Concurrency: 5, Queues: map[string]int{"orca_poll": 10, "default": 1}},
	)
	mux := hibikenasynq.NewServeMux()
	mux.HandleFunc(asynqinfra.TaskOrcaPollStatus, asynqinfra.NewOrcaPollStatusHandler(orcaHTTP, repo, publisher, eventPub))

	slog.Info("planner-service worker started", "orca_url", cfg.OrcaURL)
	if err := srv.Run(mux); err != nil {
		log.Fatal(err)
	}
}
```

`cmd/server/main.go` (HTTP server, thuộc CR-TASK-001 chủ yếu) chỉ cần wire thêm `DispatchToOrcaUseCase` + `plan.DispatchTaskUseCase` vào DI container hiện có, dùng chung `db`/`rdb`/`orcaHTTP`/`publisher`/`eventPub` — không lặp lại toàn bộ `main.go` HTTP server ở đây (ngoài phạm vi CR-ORCA-002).

---

## 4. Tích hợp giữa các service

```
plan (CR-TASK-001, trong planner-service)
   │  PlanTask cần AI agent thực thi
   ▼
plan.DispatchTaskUseCase.Execute()  ──gọi hàm Go trực tiếp──▶  dispatch.DispatchToOrcaUseCase.Execute()
                                                                    │
                                                                    ├─ orcaClient.SubmitTask ──▶ Orca POST /api/planner-tasks (SOL-ORCA-001)
                                                                    ├─ repo.Save(Tracking)     ──▶ Postgres planner.orca_task_tracking
                                                                    └─ publisher.EnqueueOrcaPollStatus (30s) ──▶ Redis (asynq queue "orca_poll")
                                                                                                        │
                                                                                        cmd/worker  ◀───┘
                                                                                        orca:poll_status handler
                                                                                            │
                                                                          ┌─────────────────┴─────────────────┐
                                                                    chưa terminal                        terminal (done/blocked/cancelled)
                                                                          │                                     │
                                                            tự enqueue lại (30s)                 repo.UpdateResult + eventPub.Publish("orca.task.<status>")
                                                                                                                 │
                                                                                                    diagnostics-service (CR-ORCA-005, tái dùng)
                                                                                                    subscribe Redis Pub/Sub
```

- **CR-ORCA-001**: `orcaclient.Client` implement đúng contract §3.
- **CR-ORCA-003**: `WHYChain/AntiPatterns/RequiredPatterns/AcceptanceCriteria` trong `DispatchToOrcaInput` là input trực tiếp cho prompt builder phía Orca — không đổi tên/thứ tự field.
- **CR-ORCA-004**: `OrcaCallbackURL` truyền cho Orca — CR-ORCA-004 (ngoài phạm vi SOL này) định nghĩa handler nhận callback tại `planner-service`; SOL này **không** thiết kế route callback, chỉ đảm bảo cấu hình `OrcaCallbackURL` sẵn sàng.
- **CR-ORCA-005**: `orca.task.*` events (Redis Pub/Sub) là nguồn cho `diagnostics-service` (thay `signal-svc`).
- **CR-ORCA-006**: `OrcaURL` trỏ tới instance headless đã deploy.
- **CR-TASK-001/003**: `plan.DispatchTaskUseCase` phụ thuộc `plantask.PlanTask` có đủ field cần thiết (§3.10 note) — phối hợp khi implement, không phải trách nhiệm 1 chiều của CR-ORCA-002.

---

## 5. Kế hoạch test

Domain (`internal/domain/orcatask`) — coverage ≥ 90%:

```go
TestTracking_NewTracking_RequiresPlannerTaskID
TestTracking_MarkSubmitted_FromPending_Succeeds
TestTracking_MarkSubmitted_FromInProgress_ReturnsErrInvalidTransition
TestTracking_Complete_NonTerminalStatus_ReturnsErrInvalidStatus
TestTracking_Complete_Terminal_SetsCompletedAtAndResult
TestTracking_IsTimedOut_PastDeadlineWhileInProgress_ReturnsTrue
TestTracking_IsTimedOut_WhenTerminal_ReturnsFalse
TestStatus_IsTerminal_TrueForDoneBlockedCancelled
```

`infrastructure/http/orcaclient` — coverage ≥ 80%, `httptest.Server`:

```go
TestClient_SubmitTask_201_ReturnsOrcaTaskID
TestClient_SubmitTask_409_ReturnsErrConflict
TestClient_SubmitTask_401_ReturnsErrUnauthorized
TestClient_GetTaskStatus_404_ReturnsErrNotFound
TestIsRetryable_OnlyTrueForErrUnavailable
```

`application/dispatch` — coverage ≥ 80%, mock `port.OrcaClient`/`orcatask.Repository`/`port.QueuePublisher`/`port.EventPublisher` (interface fake thủ công, không dùng `gomock`):

```go
TestDispatchToOrcaUseCase_AlreadyDispatched_SkipsSubmitAndHTTPCall
TestDispatchToOrcaUseCase_HTTPConflict_RecoversViaGetStatus
TestDispatchToOrcaUseCase_Success_SavesTrackingAndEnqueuesPoll
TestDispatchToOrcaUseCase_SubmitError_ReturnsWrappedError_NoTrackingLeftDangling
```

`infrastructure/queue/asynq` (poll handler) — coverage ≥ 80%, dùng `asynqtest`/mock `hibikenasynq.Task` thủ công:

```go
TestOrcaPollStatusHandler_NotTerminal_ReschedulesIn30s
TestOrcaPollStatusHandler_Terminal_PersistsResultAndPublishesEvent_NoReschedule
TestOrcaPollStatusHandler_PastDeadline_CancelsAndMarksBlocked_NoReschedule
TestOrcaPollStatusHandler_TransientClientError_ReturnsErrorForAsynqRetry
```

`infrastructure/persistence/postgres` — coverage ≥ 70% (mức infra, TDD-00 §12.3 nếu áp dụng), `sqlmock` hoặc testcontainers Postgres:

```go
TestOrcaTaskTrackingRepository_Save_InsertsRow
TestOrcaTaskTrackingRepository_FindByPlannerTaskID_NotFound_ReturnsOrcataskErrNotFound
TestOrcaTaskTrackingRepository_UpdateResult_SetsCompletedAtAndResultJSON
```

---

## 6. Rủi ro & giảm thiểu

| Rủi ro | Mức độ | Giảm thiểu |
|---|---|---|
| `DispatchToOrcaUseCase.Execute()` bị gọi lại (retry ở phía `plan.DispatchTaskUseCase` hoặc HTTP handler) → submit trùng nếu thiếu idempotency check | Cao | §2 D2 — `FindByPlannerTaskID` trước khi submit; phụ thuộc thêm Orca tuân thủ `409` (SOL-ORCA-001 §4) làm lớp phòng thủ thứ 2 |
| 1 task treo mãi (Orca không bao giờ trả terminal, hoặc `orca:poll_status` bị dead-letter sau `MaxRetry`) | Trung bình | `DeadlineAt` check đầu mỗi lượt poll (§3.8) đảm bảo tự cắt sau `TimeoutHours` dù có mất vài lượt do lỗi transient; asynq dead-letter monitoring theo TDD-07 §Dead Letter Handling (goroutine kiểm tra `dead` queue mỗi 5 phút) — bổ sung khi implement `cmd/worker` |
| Worker crash giữa lúc đang xử lý 1 lượt poll | Thấp | asynq: task chưa `Ack` (chưa `return nil`) sẽ được redeliver theo cơ chế lease của asynq — không mất tiến trình; task đã `ProcessIn` cho lượt tiếp theo vẫn nằm trong Redis |
| Breaking dependency ngầm: `plan.DispatchTaskUseCase` giả định `PlanTask` có field chưa tồn tại (`WorktreeRepo`, `AcceptanceCriteria`, …) | Cao | §3.10 note + §9 Discoveries #6 — cần đồng bộ với CR-TASK-001/003 trước khi coi TASK-06 (wiring) hoàn tất; không phải lỗi thiết kế của CR-ORCA-002 riêng lẻ |
| Redis Pub/Sub không đảm bảo delivery (subscriber offline mất event) | Trung bình | Không dùng `orca.task.*` event làm nguồn sự thật duy nhất — trạng thái chính thức luôn đọc được lại từ `planner.orca_task_tracking` (giống nguyên tắc SOL-ORCA-001 §6 áp dụng cho SSE) |
| `MaxRetry(3)` của `orca:poll_status` hết hạn do Orca lỗi liên tục (không phải do task đã terminal) → task rơi vào `dead` queue, không tự enqueue lại | Trung bình | Alert từ dead-letter monitor (TDD-07) phải phân biệt được "dead vì lỗi" vs "kết thúc bình thường" — `Tracking.Status` vẫn `in_progress` là dấu hiệu cần vận hành viên can thiệp thủ công (re-enqueue hoặc cancel) |

---

## 7. Ước tính công việc theo layer

| Layer | Hạng mục | Giờ |
|---|---|---|
| Domain | `orcatask.Tracking`, `Status`, `Result`, `Repository` interface, errors | 3h |
| Application/port | `OrcaClient`, `QueuePublisher` (+ `OrcaPollStatusPayload`), `EventPublisher` interfaces | 2h |
| Infrastructure/http | `orcaclient.Client` + DTO + errors (HTTP client thuần) | 4h |
| Infrastructure/persistence | Postgres repo (`orcatask_repo.go`) + migration `planner.orca_task_tracking` | 4h |
| Application/dispatch | `DispatchToOrcaUseCase` (kèm idempotency check D2) | 3h |
| Infrastructure/queue/asynq | `tasks.go`, `publisher.go`, `handler_orca_poll.go` (self-rescheduling) | 5h |
| Infrastructure/events | Redis Pub/Sub `EventPublisher` | 2h |
| Application/plan | `DispatchTaskUseCase` — gọi thẳng use case (Change 4) | 1h |
| Config + wiring | `config.go`, `cmd/worker/main.go`, điểm chạm `cmd/server/main.go` | 3h |
| Test | domain + http client + dispatch usecase + asynq handler + persistence | 11h |
| **Tổng** | | **38h** |

So với effort estimate gốc của CR-ORCA-002 (21h): SOL cao hơn ~17h do (a) bổ sung migration/persistence layer chưa có Change riêng trong CR, (b) bổ sung `EventPublisher` Redis (CR chỉ khai báo interface, không có phần triển khai), (c) test coverage đầy đủ theo từng layer thay vì gộp chung "Tests: 6h" như CR.

---

## 8. Dependencies

Phụ thuộc: CR-ORCA-001 (contract Orca — qua SOL-ORCA-001); CR-TASK-001 (domain `plan`/`plantask` phải tồn tại trước khi §3.10 chạy được, dù §3.1–§3.9 (client/domain/persistence/dispatch usecase/asynq) có thể implement + test độc lập không cần chờ CR-TASK-001 hoàn tất — chỉ cần mock `plantask.PlanTask`). Là nền tảng cho CR-ORCA-003 (field mapping prompt), CR-ORCA-004 (callback handler — dùng chung `orcatask.Repository`), CR-ORCA-005 (event consumer `orca.task.*`), CR-ORCA-006 (địa chỉ `OrcaURL`).

---

## 9. Discoveries — chi tiết bổ sung so với CR-ORCA-002 gốc

CR-ORCA-002 (viết lại 2026-08-10) đúng về mặt kiến trúc lớn (asynq thay Temporal, self-rescheduling task, bỏ workflow) nhưng là một CR — không đi sâu vào toàn bộ chi tiết thực thi. Đối chiếu với TDD-00/01/02/07 khi viết SOL này, phát hiện các điểm sau cần làm rõ/sửa trước khi giao task cho engineer:

1. **Thiếu idempotency check trong `DispatchToOrcaUseCase.Execute()`** — CR Change 3.2 gọi `orcaClient.SubmitTask` ngay không kiểm tra `Repository` trước. Thiết kế Temporal cũ có được tính chất này "miễn phí" nhờ activity retry luôn chạy lại từ đầu qua `FindByPlannerTaskID`; use case gọi trực tiếp (không phải task hàng đợi tự retry) thì **không** có cơ chế tương đương trừ khi tự thêm — đã bổ sung ở §2 D2/§3.7.
2. **`OrcaPollStatusPayload` bị định nghĩa 2 lần** trong CR (`port.OrcaPollStatusPayload` dùng ngầm định trong `dispatch_to_orca.go`/`handler_orca_poll.go`, và định nghĩa tường minh lặp lại trong `infrastructure/queue/asynq/tasks.go`) — hợp nhất về 1 định nghĩa duy nhất tại `application/port` (§2 D3/§3.5/§3.8).
3. **`orcatask.Repository.UpdateResult` trong CR nhận thẳng `orcaclient.OrcaTaskStatus`** (HTTP DTO) — vi phạm TDD-00 §4 (domain layer không phụ thuộc DTO của lớp infrastructure). Sửa thành nhận `orcatask.Result` domain-level, map tại boundary trong handler (§2 D4/§3.2/§3.8).
4. **Cơ chế publish event không được CR chỉ định** (`port.EventPublisher` chỉ là interface) — CR-ORCA-002 phiên bản trước (dựa `vnp-planner`) dùng NATS; WKP không có hạ tầng NATS, dùng Redis Pub/Sub cho mọi event nội bộ (TDD-07 §Event Subscriber). Thiết kế `RedisPublisher` bổ sung tại §3.9 — không có trong CR.
5. **Không có Change nào cho migration** trong CR-ORCA-002 — bảng `planner.orca_task_tracking` (hoặc tương đương) là điều kiện tiên quyết bắt buộc cho `orcatask.Repository`; bổ sung tại §3.4 theo convention goose của TDD-02.
6. **`plan.DispatchTaskUseCase` (CR Change 4) tham chiếu các field của `PlanTask`** (`t.WorktreeRepo`, `t.WorktreeBranch`, `t.EnrichedContext.TaskFileContent/WHYChain/AntiPatterns/RequiredPatterns`, `t.AcceptanceCriteria`, `t.CRID`) **không tồn tại** trên `PlanTask` entity như CR-TASK-001 §Change 1 định nghĩa thật (`backend/services/planner-service/internal/domain/plantask/entity.go`, xem field list đầy đủ ở CR-TASK-001) — các field này nhiều khả năng do CR-TASK-003 (Task Context Enrichment) bổ sung sau. §3.10 đã điều chỉnh mapping cho khớp field thật có sẵn hôm nay (`t.ID`, `t.PlanID`, `t.ExternalRef`, `t.Title`, `t.Description`, `t.AgentType`, `t.Priority`, `t.WHYChain`) và đánh dấu rõ phần còn thiếu bằng `TODO`/note, không tự ý thêm field vào `PlanTask` (ngoài phạm vi CR-ORCA-002).
7. **`OrcaURL` default comment trong CR Change 5 ghi `"http://orca:3000"`** — sai theo xác nhận thật đã có ở SOL-ORCA-001 §9 (port HTTP thật của Orca là `:6769`). CR-ORCA-002 mới không kế thừa lỗi port `:3000` xuyên suốt như CR/SOL v1 cũ (SOL-ORCA-001 đã sửa các chỗ khác), nhưng dòng comment ở Change 5 vẫn còn sót giá trị `:3000` — sửa tại §3.11.
8. **`shared/pkg/orcaclient` (thiết kế cũ, dùng chung `temporal-worker`+`signal-svc`) không còn lý do tồn tại** — CR-ORCA-002 mới đặt `OrcaClient` trực tiếp trong `planner-service/internal/infrastructure/http/orcaclient` (Change 1), không phải package chia sẻ liên-service. Nếu `diagnostics-service` (CR-ORCA-005) sau này cũng cần gọi Orca `GET /health`/`GET /api/planner-tasks/{id}`, cân nhắc trích xuất thành `pkg/orcaclient` dùng chung theo TDD-01 (`pkg/` — shared Go packages) **tại thời điểm đó**, không làm trước khi có 2 consumer thật (tránh trừu tượng hoá sớm).
