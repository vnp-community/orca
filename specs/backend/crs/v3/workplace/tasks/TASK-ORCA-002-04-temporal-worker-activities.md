> ⚠️ **SUPERSEDED (2026-08-10):** Tài liệu task/solution breakdown này được sinh ra từ các CR-ORCA-00x/CR-TASK-00x đã bị viết lại hoặc retired — dựa trên giả định sai rằng `vnp-planner` bị loại bỏ và `vnp-workplace` tự dựng `planner-service` mới. Kiến trúc đúng: xem [`docs/crs/v3/orca/README.md`](../../../../../docs/crs/v3/orca/README.md). Nội dung bên dưới chỉ còn giá trị tham khảo lịch sử — không dùng để implement.
>
> ---
>
## Status: 🔲 NOT STARTED

# TASK-ORCA-002-04 — `planner-service`: Application Layer (`DispatchToOrcaUseCase`)

**Phase:** 1 — Core dispatch
**Scope:** ✅ vnp-workplace — Go (`backend/services/planner-service`, `:3013`)
**Source:** [SOL-ORCA-002 §3.7](../solutions/SOL-ORCA-002-planner-orca-dispatcher.md#37-internalapplicationdispatch--dispatchtoorcausecase), [SOL-ORCA-002 §2 D2](../solutions/SOL-ORCA-002-planner-orca-dispatcher.md#2-quyết-định-thiết-kế-chính)
**Depends On:** [TASK-ORCA-002-01](./TASK-ORCA-002-01-shared-orcaclient.md) (`orcaclient`, `port.OrcaClient`/`port.QueuePublisher`/`port.EventPublisher`), [TASK-ORCA-002-02](./TASK-ORCA-002-02-temporal-worker-domain.md) (`orcatask.Tracking`), [TASK-ORCA-002-03](./TASK-ORCA-002-03-temporal-worker-infrastructure.md) (repository impl — cần cho test tích hợp, không bắt buộc cho unit test dùng fake repo)
**Estimated Files:** ~4 files (2 code + 2 test)
**Working Dir:** `/opt/repos/vnp-workplace/backend/services/planner-service/internal/application/dispatch/`

---

## Bối cảnh quan trọng — ĐỌC KỸ TRƯỚC KHI CODE

1. **Đây KHÔNG phải Temporal Activity.** Tên file cũ của task này (`temporal-worker-activities.md`) được giữ để không phá vỡ link tham chiếu — nội dung đã viết lại 100%. Không còn `activity.GetLogger(ctx)`, không còn 5 activity riêng biệt (`SubmitOrcaTask`/`GetOrcaTaskStatus`/`CancelOrcaTask`/`FinalizeOrcaDispatch`/`PrepareOrcaContext`) — chỉ còn **1 use case duy nhất**: `DispatchToOrcaUseCase.Execute()`, gọi 1 lần, submit + enqueue lượt poll đầu tiên rồi return ngay (không block chờ Orca).
2. **Không còn `FinalizeOrcaDispatch`/`PrepareOrcaContext`** — CR-ORCA-002 mới KHÔNG có khái niệm "fan-out N task của 1 Plan" ở layer này (Change 4 nói rõ: "không có bước workflow nào chen giữa `plan-svc` và `DispatchToOrcaUseCase`"). Nếu 1 Plan có N task cần dispatch, đó là vòng lặp ở domain `plan`/`plantask` (CR-TASK-001/002), gọi `DispatchToOrcaUseCase.Execute()` N lần — KHÔNG phải trách nhiệm của use case này.
3. **Bổ sung bắt buộc so với CR-ORCA-002 gốc: idempotency check.** CR Change 3.2 gọi thẳng `orcaClient.SubmitTask` không kiểm tra gì trước — SOL-ORCA-002 §2 D2 chỉ rõ đây là lỗ hổng: thiết kế Temporal cũ có tính idempotent "miễn phí" nhờ activity luôn retry lại từ đầu qua `FindByPlannerTaskID`; use case gọi trực tiếp (không tự động retry) sẽ **mất tính chất này** nếu không tự thêm bước `repo.FindByPlannerTaskID` trước khi gọi `SubmitTask`. Đây là **acceptance criterion bắt buộc**, không phải optional.
4. `POST/GET /api/planner-tasks*` **chưa tồn tại phía Orca** (SOL-ORCA-001 §9) — use case dưới đây gọi đúng theo contract đã khoá, unit-test với `port.OrcaClient` fake (interface nhỏ, không cần `httptest.Server` ở layer này — đó là việc của TASK-ORCA-002-01). Không cần chờ Orca team để hoàn thành task này.

---

## Mục tiêu

Implement `DispatchToOrcaUseCase` — application use case duy nhất chịu trách nhiệm: (a) kiểm tra idempotency, (b) tạo `orcatask.Tracking`, (c) gọi Orca `SubmitTask`, (d) xử lý `409` (đã tồn tại), (e) enqueue lượt poll đầu tiên qua `port.QueuePublisher`, (f) publish `orca.task.submitted`.

---

## Acceptance Criteria

- [ ] `DispatchToOrcaUseCase.Execute()` gọi `repo.FindByPlannerTaskID` **trước** `orcaClient.SubmitTask` — nếu đã có `Tracking` với `OrcaTaskID != ""`, trả kết quả cũ, **không** gọi HTTP lại
- [ ] Khi Orca trả `409 orcaclient.ErrConflict` → gọi `GetTaskStatus` lấy lại `orca_task_id` hiện có, coi như thành công (không fail use case)
- [ ] Khi Orca trả lỗi khác (network, 5xx, 401, 422...) → trả lỗi wrap rõ ràng, **không** để lại `Tracking` ở trạng thái `pending` treo lơ lửng nếu tránh được (best-effort — không bắt buộc rollback transaction, nhưng phải log/document rõ)
- [ ] `SubmitTask` thành công → `tracking.MarkSubmitted(orcaTaskID)` + `repo.UpdateStatus(..., StatusInProgress, "")` + publish event `orca.task.submitted`
- [ ] `Execute()` enqueue `port.OrcaPollStatusPayload{OrcaTaskID, PlannerTaskID, DeadlineAt}` qua `publisher.EnqueueOrcaPollStatus(..., 30*time.Second)` **sau khi** submit thành công
- [ ] `Execute()` **không block** — không gọi `GetTaskStatus` lặp lại trong vòng lặp nào cả (polling loop thuộc TASK-ORCA-002-05, infra layer)
- [ ] `go build ./internal/application/dispatch/...` thành công
- [ ] `go test ./internal/application/dispatch/... -v -race -cover` pass, coverage ≥ 80%

---

## File 1: `internal/application/dispatch/dispatch_to_orca.go`

```go
// Package dispatch implements the use case that submits one task to Orca and
// starts its self-rescheduling poll chain. Replaces the old Temporal Activity
// DispatchToOrca entirely — see SOL-ORCA-002 §2 (comparison table) and §3.7.
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
	// Idempotency guard — NOT present in CR-ORCA-002 §3.2 original (see
	// SOL-ORCA-002 §2 D2). Without this, a caller retry would re-submit a
	// duplicate task to Orca.
	if existing, err := uc.repo.FindByPlannerTaskID(ctx, input.PlannerTaskID); err == nil {
		if existing.OrcaTaskID != "" {
			return &DispatchToOrcaResult{OrcaTaskID: existing.OrcaTaskID, Status: string(existing.Status)}, nil
		}
	} else if !errors.Is(err, orcatask.ErrNotFound) {
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
				if markErr := tracking.MarkSubmitted(status.OrcaTaskID); markErr == nil {
					_ = uc.repo.UpdateStatus(ctx, status.OrcaTaskID, orcatask.StatusInProgress, "")
				}
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

	// Enqueue the first poll round — 30s from now. Start of the
	// self-rescheduling chain (handler in TASK-ORCA-002-05).
	if err := uc.publisher.EnqueueOrcaPollStatus(ctx, port.OrcaPollStatusPayload{
		OrcaTaskID:    resp.OrcaTaskID,
		PlannerTaskID: input.PlannerTaskID,
		DeadlineAt:    deadline,
	}, 30*time.Second); err != nil {
		return nil, fmt.Errorf("enqueue orca poll: %w", err)
	}

	return &DispatchToOrcaResult{OrcaTaskID: resp.OrcaTaskID, Status: "pending"}, nil
}
```

## File 2: `internal/application/dispatch/helpers.go`

```go
package dispatch

func resolveTimeout(inputHours, defaultHours int) int {
	if inputHours > 0 {
		return inputHours
	}
	if defaultHours > 0 {
		return defaultHours
	}
	return 8
}

// selectAgentType maps a PlanTask.AgentType (CR-TASK-001, e.g.
// "ai-developer"/"ai-architect"/"ai-reviewer") to the Orca agent_type values
// SOL-ORCA-001 §3.2 accepts ("claude"/"codex"/"opencode").
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

---

## Test File 3: `internal/application/dispatch/dispatch_to_orca_test.go`

Mock `port.OrcaClient`/`orcatask.Repository`/`port.QueuePublisher`/`port.EventPublisher` bằng interface fake thủ công (không cần thư viện mock — `planner-service` chưa dùng `gomock`/`mockery`, giữ nhất quán với các service Go khác trong `vnp-workplace`). Test cases bắt buộc:

```go
package dispatch_test

func TestDispatchToOrcaUseCase_AlreadyDispatched_SkipsSubmitAndHTTPCall(t *testing.T) {
	// fake repo.FindByPlannerTaskID trả về *orcatask.Tracking với OrcaTaskID != ""
	// assert: orcaClient.SubmitTask KHÔNG được gọi; publisher.EnqueueOrcaPollStatus KHÔNG được gọi
}

func TestDispatchToOrcaUseCase_NotYetDispatched_ProceedsToSubmit(t *testing.T) {
	// fake repo.FindByPlannerTaskID trả về orcatask.ErrNotFound
	// assert: SubmitTask được gọi đúng 1 lần
}

func TestDispatchToOrcaUseCase_HTTPConflict_RecoversViaGetStatus(t *testing.T) {
	// orcaClient.SubmitTask trả orcaclient.ErrConflict; GetTaskStatus trả status hợp lệ
	// assert: Execute() trả kết quả thành công, không lỗi
}

func TestDispatchToOrcaUseCase_Success_SavesTrackingAndEnqueuesPoll(t *testing.T) {
	// assert: repo.Save được gọi 1 lần (pending), repo.UpdateStatus được gọi 1 lần (in_progress),
	// eventPub.Publish("orca.task.submitted", ...) được gọi, publisher.EnqueueOrcaPollStatus
	// được gọi với delay = 30*time.Second và payload đúng OrcaTaskID/PlannerTaskID/DeadlineAt
}

func TestDispatchToOrcaUseCase_SubmitError_NonConflict_ReturnsWrappedError(t *testing.T) {
	// orcaClient.SubmitTask trả lỗi khác 409 (vd orcaclient.ErrUnavailable)
	// assert: Execute() trả lỗi, publisher.EnqueueOrcaPollStatus KHÔNG được gọi
}

func TestDispatchToOrcaUseCase_ResolvesTimeoutFromInputOrDefault(t *testing.T) {
	// input.TimeoutHours = 0 → dùng cfg.DefaultTimeoutHours; input.TimeoutHours > 0 → dùng input
}
```

---

## Verification

```bash
cd /opt/repos/vnp-workplace/backend/services/planner-service

go build ./internal/application/dispatch/...
go vet ./internal/application/dispatch/...
go test ./internal/application/dispatch/... -v -race -cover
go test ./internal/application/dispatch/... -coverprofile=dispatch_cov.out
go tool cover -func=dispatch_cov.out | grep total   # kỳ vọng >= 80%
```
