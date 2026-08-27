> ⚠️ **SUPERSEDED (2026-08-10):** Tài liệu task/solution breakdown này được sinh ra từ các CR-ORCA-00x/CR-TASK-00x đã bị viết lại hoặc retired — dựa trên giả định sai rằng `vnp-planner` bị loại bỏ và `vnp-workplace` tự dựng `planner-service` mới. Kiến trúc đúng: xem [`docs/crs/v3/orca/README.md`](../../../../../docs/crs/v3/orca/README.md). Nội dung bên dưới chỉ còn giá trị tham khảo lịch sử — không dùng để implement.
>
> ---
>
## Status: 🔲 NOT STARTED

# TASK-ORCA-002-05 — `planner-service`: asynq Self-Rescheduling Poll Task (`orca:poll_status`) + Redis EventPublisher

**Phase:** 1 — Core dispatch
**Scope:** ✅ vnp-workplace — Go (`backend/services/planner-service`, `:3013`)
**Source:** [SOL-ORCA-002 §3.8–§3.9](../solutions/SOL-ORCA-002-planner-orca-dispatcher.md#38-internalinfrastructurequeueasynq--self-rescheduling-poll-task)
**Depends On:** [TASK-ORCA-002-01](./TASK-ORCA-002-01-shared-orcaclient.md) (`port.OrcaClient`, `port.QueuePublisher`, `port.EventPublisher`, `orcaclient` DTO/errors), [TASK-ORCA-002-02](./TASK-ORCA-002-02-temporal-worker-domain.md) (`orcatask.Tracking`/`Status`/`Result`), [TASK-ORCA-002-03](./TASK-ORCA-002-03-temporal-worker-infrastructure.md) (repository — cần cho integration test)
**Estimated Files:** ~6 files (4 code + 2 test)
**Working Dir:** `/opt/repos/vnp-workplace/backend/services/planner-service/internal/infrastructure/`

---

## Bối cảnh quan trọng — ĐỌC KỸ TRƯỚC KHI CODE

1. **Đây là phần thay thế hoàn toàn Temporal Workflow/Signal/Timer cũ.** Tên file cũ của task này (`temporal-worker-workflows.md`) được giữ để không phá vỡ link tham chiếu — nội dung đã viết lại 100% cho asynq. Không còn `workflow.Context`, không còn `workflow.Selector`/`workflow.GetSignalChannel`/`workflow.NewTimer`, không còn `OrcaTaskDispatchWorkflow`/`PlanDispatchWorkflow`, không còn `workflow.NewSemaphore` (vốn không tồn tại thật trong Temporal SDK — vấn đề của thiết kế cũ, không còn liên quan ở đây).
2. **Pattern "self-rescheduling task"** (SOL-ORCA-002 §2 bảng so sánh): mỗi lần worker xử lý `orca:poll_status`, nó thực hiện **đúng 1 lượt** `GetTaskStatus` rồi `return nil` — nếu Orca task chưa terminal, handler **tự enqueue lại chính task type này** (`hibikenasynq.ProcessIn(30*time.Second)`) trước khi return, thay vì giữ 1 goroutine/context sống trong vòng lặp 24h.
3. **Timeout tổng (8-24h) phải tự check thủ công** ở đầu mỗi lượt — asynq không có cơ chế tương đương `Temporal ActivityOptions.StartToCloseTimeout`. `DeadlineAt` (tính từ `TimeoutHours` lúc dispatch — TASK-ORCA-002-04) được mang theo trong payload của mỗi lượt poll.
4. **`port.OrcaPollStatusPayload` chỉ định nghĩa 1 lần** — tại `internal/application/port/queue_publisher.go` (TASK-ORCA-002-01). File `tasks.go` trong task này **chỉ khai báo hằng số tên task**, KHÔNG định nghĩa lại struct payload (SOL-ORCA-002 §2 D3 — sửa lỗi trùng lặp trong CR-ORCA-002 gốc).
5. **`orcatask.Repository.UpdateResult` nhận domain-level `orcatask.Result`**, KHÔNG nhận thẳng `orcaclient.OrcaTaskResult`/`orcaclient.OrcaTaskStatus` (SOL-ORCA-002 §2 D4). Handler trong task này chịu trách nhiệm map DTO HTTP → domain type trước khi gọi repo — viết hàm `toDomainResult`/`toDomainStatus` trong `handler_orca_poll.go`.
6. **`EventPublisher` dùng Redis Pub/Sub**, KHÔNG dùng NATS (SOL-ORCA-002 §2 D5) — `RedisPublisher` (`internal/infrastructure/events/redis_publisher.go`) cũng thuộc phạm vi task này (không có Change riêng trong CR-ORCA-002 gốc, CR chỉ khai báo interface `port.EventPublisher`).
7. **Đặt tên package `asynq` trùng tên thư viện `github.com/hibiken/asynq`** (đúng theo TDD-07 §Task Definitions — thư mục `infrastructure/queue/asynq/` package tên `asynq`). Import thư viện với alias `hibikenasynq` trong toàn bộ file của package này để tránh xung đột tên.

---

## Mục tiêu

Implement chuỗi poll tự lên lịch lại (`orca:poll_status`): task type constant, `Publisher.EnqueueOrcaPollStatus`, handler xử lý 1 lượt (timeout check → poll → terminal/reschedule), `RedisPublisher` cho event, và `cmd/worker/main.go` khởi động asynq server.

---

## Acceptance Criteria

- [ ] `TaskOrcaPollStatus = "orca:poll_status"` khai báo tại `internal/infrastructure/queue/asynq/tasks.go`, **không** định nghĩa lại `OrcaPollStatusPayload` ở đây
- [ ] `Publisher.EnqueueOrcaPollStatus` set `hibikenasynq.MaxRetry(3)`, `hibikenasynq.Timeout(45*time.Second)`, `hibikenasynq.Queue("orca_poll")`, `hibikenasynq.ProcessIn(delay)`
- [ ] Handler: nếu `time.Now().After(payload.DeadlineAt)` → gọi `orcaClient.CancelTask` + `repo.UpdateStatus(..., StatusBlocked, "timeout waiting for Orca")` + publish `orca.task.blocked` + **return nil, không reschedule**
- [ ] Handler: nếu `GetTaskStatus` lỗi (network/5xx) → return error (KHÔNG tự publish/update gì) để asynq retry đúng lượt này qua `MaxRetry(3)`
- [ ] Handler: nếu `status.IsTerminal()` → `repo.UpdateResult(..., domainStatus, domainResult)` + publish `orca.task.<status>` + **return nil, không reschedule**
- [ ] Handler: nếu chưa terminal và chưa timeout → gọi lại `publisher.EnqueueOrcaPollStatus(ctx, payload, 30*time.Second)` rồi return nil
- [ ] `RedisPublisher.Publish` publish JSON payload qua `redis.Client.Publish(ctx, subject, data)`
- [ ] `cmd/worker/main.go` đăng ký `TaskOrcaPollStatus` vào `hibikenasynq.NewServeMux()`, queue `"orca_poll"` concurrency ≥ 5
- [ ] `go build ./internal/infrastructure/queue/asynq/... ./internal/infrastructure/events/... ./cmd/worker/...` thành công
- [ ] Test dùng mock `port.OrcaClient`/`orcatask.Repository`/`port.EventPublisher` + `Publisher` giả (hoặc miniredis cho `EnqueueOrcaPollStatus` thật), coverage ≥ 80% các nhánh (timeout / terminal / reschedule / transient-error)

---

## File 1: `internal/infrastructure/queue/asynq/tasks.go`

```go
package asynq

// TaskOrcaPollStatus is the self-rescheduling task type used to poll one
// Orca task's status until it reaches a terminal state or times out. Its
// payload type is port.OrcaPollStatusPayload (application layer) — NOT
// redefined here, see SOL-ORCA-002 §2 D3.
const TaskOrcaPollStatus = "orca:poll_status"
```

## File 2: `internal/infrastructure/queue/asynq/publisher.go`

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
		hibikenasynq.MaxRetry(3), // retry ONLY this poll round's GetTaskStatus call
		hibikenasynq.Timeout(45*time.Second),
		hibikenasynq.Queue("orca_poll"),
		hibikenasynq.ProcessIn(delay), // self-reschedule mechanism
	)
	_, err = p.client.EnqueueContext(ctx, task)
	return err
}
```

## File 3: `internal/infrastructure/queue/asynq/handler_orca_poll.go`

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
	"github.com/vnptech/kwp/services/planner-service/internal/infrastructure/http/orcaclient"
)

// NewOrcaPollStatusHandler handles EXACTLY ONE poll round. Pattern:
// "self-rescheduling task" — if the Orca task is not yet terminal, the
// handler re-enqueues TaskOrcaPollStatus (ProcessIn 30s) before returning
// nil, instead of keeping a goroutine alive for up to 24h like the original
// Temporal Activity/Workflow.
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

		// Overall timeout — must be checked manually since asynq holds no
		// state across multiple task runs (see SOL-ORCA-002 §2 comparison table).
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
			// Transient (network/5xx): return error so asynq retries ONLY this
			// poll round (hibikenasynq.MaxRetry(3), publisher.go). Prior/future
			// rounds are unaffected.
			return fmt.Errorf("poll orca status: %w", err)
		}

		if status.IsTerminal() {
			domainStatus := orcatask.Status(status.Status)
			result := toDomainResult(status.Result)
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

// toDomainResult maps the infra HTTP DTO to the domain-level Result — see
// SOL-ORCA-002 §2 D4 (domain must not depend on infrastructure/http/orcaclient).
func toDomainResult(r *orcaclient.OrcaTaskResult) *orcatask.Result {
	if r == nil {
		return nil
	}
	return &orcatask.Result{
		Success:       r.Success,
		FilesCreated:  r.FilesCreated,
		FilesModified: r.FilesModified,
		CommitHash:    r.CommitHash,
		TestOutput:    r.TestOutput,
		ErrorMessage:  r.ErrorMessage,
	}
}
```

## File 4: `internal/infrastructure/events/redis_publisher.go`

```go
// Package events provides the Redis Pub/Sub implementation of
// port.EventPublisher — WKP's standard event transport (TDD-07 §Event
// Subscriber), replacing the NATS-based design inherited from vnp-planner
// (SOL-ORCA-002 §2 D5).
package events

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/redis/go-redis/v9"
)

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

## File 5: `cmd/worker/main.go`

```go
package main

import (
	"log"
	"log/slog"
	"time"

	hibikenasynq "github.com/hibiken/asynq"

	"github.com/vnptech/kwp/services/planner-service/config"
	"github.com/vnptech/kwp/services/planner-service/internal/infrastructure/events"
	"github.com/vnptech/kwp/services/planner-service/internal/infrastructure/http/orcaclient"
	"github.com/vnptech/kwp/services/planner-service/internal/infrastructure/persistence/postgres"
	asynqinfra "github.com/vnptech/kwp/services/planner-service/internal/infrastructure/queue/asynq"
	// ... mustConnectPostgres/mustConnectRedis helpers — cùng convention TDD-01 §Dependency Injection
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	db := mustConnectPostgres(cfg.DatabaseURL)
	rdb := mustConnectRedis(cfg.RedisURL)

	orcaHTTP := orcaclient.New(cfg.OrcaURL, cfg.OrcaAPISecret, 15*time.Second, slog.Default())
	repo := postgres.NewOrcaTaskTrackingRepository(db)
	publisher := asynqinfra.NewPublisher(cfg.RedisURL)
	eventPub := events.NewRedisPublisher(rdb)

	srv := hibikenasynq.NewServer(
		hibikenasynq.RedisClientOpt{Addr: cfg.RedisURL},
		hibikenasynq.Config{
			Concurrency: 5,
			Queues:      map[string]int{"orca_poll": 10, "default": 1},
		},
	)

	mux := hibikenasynq.NewServeMux()
	mux.HandleFunc(asynqinfra.TaskOrcaPollStatus, asynqinfra.NewOrcaPollStatusHandler(orcaHTTP, repo, publisher, eventPub))

	slog.Info("planner-service worker started", "orca_url", cfg.OrcaURL)
	if err := srv.Run(mux); err != nil {
		log.Fatal(err)
	}
}
```

---

## Test File 6: `internal/infrastructure/queue/asynq/handler_orca_poll_test.go`

Mock `port.OrcaClient`/`orcatask.Repository`/`port.EventPublisher` bằng interface fake thủ công; `Publisher` có thể dùng `hibikenasynq` thật trỏ vào `miniredis` (`github.com/alicebob/miniredis/v2`, thêm test-only dependency nếu chưa có) hoặc 1 fake nhỏ implement `port.QueuePublisher`-tương-đương để assert `EnqueueOrcaPollStatus` được gọi đúng tham số. Test cases bắt buộc:

```go
func TestOrcaPollStatusHandler_NotTerminal_ReschedulesIn30s(t *testing.T) {
	// orcaClient.GetTaskStatus trả status="in_progress"
	// assert: publisher (fake/miniredis) nhận đúng 1 lần EnqueueOrcaPollStatus với payload
	// giống hệt input (OrcaTaskID/PlannerTaskID/DeadlineAt không đổi) và delay = 30s
	// assert: repo.UpdateResult KHÔNG được gọi, eventPub.Publish KHÔNG được gọi
}

func TestOrcaPollStatusHandler_Terminal_PersistsResultAndPublishesEvent_NoReschedule(t *testing.T) {
	// orcaClient.GetTaskStatus trả status="done", result != nil
	// assert: repo.UpdateResult được gọi với orcatask.StatusDone + orcatask.Result đúng field
	// assert: eventPub.Publish("orca.task.done", ...) được gọi
	// assert: publisher KHÔNG được gọi lại (không reschedule)
}

func TestOrcaPollStatusHandler_PastDeadline_CancelsAndMarksBlocked_NoReschedule(t *testing.T) {
	// payload.DeadlineAt ở quá khứ
	// assert: orcaClient.CancelTask được gọi; repo.UpdateStatus(..., StatusBlocked, "timeout waiting for Orca")
	// assert: eventPub.Publish("orca.task.blocked", ...) được gọi; orcaClient.GetTaskStatus KHÔNG được gọi
	// assert: publisher KHÔNG được gọi
}

func TestOrcaPollStatusHandler_TransientClientError_ReturnsErrorForAsynqRetry(t *testing.T) {
	// orcaClient.GetTaskStatus trả lỗi (vd orcaclient.ErrUnavailable)
	// assert: handler trả error != nil; repo.UpdateResult/UpdateStatus KHÔNG được gọi;
	// publisher.EnqueueOrcaPollStatus KHÔNG được gọi (asynq tự retry lượt này qua MaxRetry)
}

func TestOrcaPollStatusHandler_UnmarshalError_ReturnsError(t *testing.T) {
	// t.Payload() trả JSON không hợp lệ — assert error != nil, không panic
}
```

---

## Verification

```bash
cd /opt/repos/vnp-workplace/backend/services/planner-service

go build ./internal/infrastructure/queue/asynq/... ./internal/infrastructure/events/... ./cmd/worker/...
go vet ./internal/infrastructure/queue/asynq/... ./internal/infrastructure/events/...
go test ./internal/infrastructure/queue/asynq/... ./internal/infrastructure/events/... -v -race -cover
go test ./internal/infrastructure/queue/asynq/... -coverprofile=asynq_cov.out
go tool cover -func=asynq_cov.out | grep total   # kỳ vọng >= 80%
```

**Không** cắm `OrcaURL` vào Orca thật ở bước smoke test `cmd/worker` — endpoint `/api/planner-tasks*` chưa tồn tại (xem TASK-ORCA-001-13). Để activity fail có kiểm soát (log `orcaclient.ErrUnavailable`/connection refused) là kết quả chấp nhận được cho tới khi Orca team hoàn thành.
