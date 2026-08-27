> ⚠️ **SUPERSEDED (2026-08-10):** Tài liệu task/solution breakdown này được sinh ra từ các CR-ORCA-00x/CR-TASK-00x đã bị viết lại hoặc retired — dựa trên giả định sai rằng `vnp-planner` bị loại bỏ và `vnp-workplace` tự dựng `planner-service` mới. Kiến trúc đúng: xem [`docs/crs/v3/orca/README.md`](../../../../../docs/crs/v3/orca/README.md). Nội dung bên dưới chỉ còn giá trị tham khảo lịch sử — không dùng để implement.
>
> ---
>
## Status: 🔲 NOT STARTED

# TASK-ORCA-002-06 — `planner-service`: Config, Wiring (`cmd/server`/`cmd/worker`), `plan` Domain Integration

**Phase:** 1 — Core dispatch
**Scope:** ✅ vnp-workplace — Go (`backend/services/planner-service`, `:3013`)
**Source:** [SOL-ORCA-002 §3.10–§3.12](../solutions/SOL-ORCA-002-planner-orca-dispatcher.md#310-internalapplicationplan--tích-hợp-domain-plan-change-4-của-cr)
**Depends On:** [TASK-ORCA-002-03](./TASK-ORCA-002-03-temporal-worker-infrastructure.md) (repository), [TASK-ORCA-002-04](./TASK-ORCA-002-04-temporal-worker-activities.md) (`DispatchToOrcaUseCase`), [TASK-ORCA-002-05](./TASK-ORCA-002-05-temporal-worker-workflows.md) (asynq worker + Redis events)
**Estimated Files:** ~4 files (config.go, `internal/application/plan/dispatch_task.go`, `cmd/worker/main.go` hoàn thiện, `.env.example` — tất cả MODIFY/NEW)
**Working Dir:** `/opt/repos/vnp-workplace/backend/services/planner-service/`

---

## Bối cảnh quan trọng

1. **Không có NATS consumer/breaking schema change để xử lý** — khác thiết kế `temporal-worker` cũ (task này trước đây tên `temporal-worker-wiring.md`, xử lý NATS `plan.activated` schema breaking change). CR-ORCA-002 mới **không có fan-out multi-task workflow** nên không có event NATS `plan.activated` nào cần consume ở layer này — `plan.DispatchTaskUseCase.Execute(taskID)` được gọi **trực tiếp per-task** (từ HTTP handler `POST /api/v1/tasks/{id}/dispatch` hoặc từ vòng lặp domain `plan`, cả hai thuộc CR-TASK-001/002, ngoài phạm vi task này) — không qua hàng đợi message nào ở bước "kích hoạt dispatch".
2. **`plantask.PlanTask` (CR-TASK-001) hiện thiếu field** mà `DispatchToOrcaInput` cần đầy đủ (`WorktreeRepo`, `WorktreeBranch`, `AcceptanceCriteria`, `AntiPatterns`, `RequiredPatterns`) — xem SOL-ORCA-002 §9 Discoveries #6. `dispatch_task.go` viết ở task này **chỉ map các field đã có thật** trên `PlanTask` hôm nay, đánh dấu rõ phần còn thiếu bằng comment `TODO(CR-TASK-003)` — KHÔNG tự ý thêm field vào `PlanTask` (thuộc phạm vi CR-TASK-001/003, không phải CR-ORCA-002).
3. Port Orca thật: HTTP `:6769` (KHÔNG phải `:3000` — comment gốc trong CR-ORCA-002 Change 5 còn sót giá trị này, xem SOL-ORCA-002 §9 Discoveries #7, đã sửa trong config mẫu dưới đây). Secret `ORCA_PLANNER_API_SECRET` **chưa tồn tại phía Orca** — vẫn khai báo `required:"true"` vì đây là secret `planner-service` cần chuẩn bị sẵn, giá trị thật nhận từ Orca team khi TASK-ORCA-001-13 xong.
4. `cmd/server/main.go` (HTTP server chính của `planner-service`) **thuộc phạm vi CR-TASK-001** — task này chỉ bổ sung phần wiring liên quan Orca dispatch vào DI container đã có (không viết lại toàn bộ `cmd/server/main.go`).

---

## Mục tiêu

Nối toàn bộ các phần đã viết ở TASK-ORCA-002-01..05 lại thành 1 service chạy được: `config.Config` đầy đủ field Orca, `cmd/worker/main.go` hoàn thiện (đã có khung ở TASK-05, task này bổ sung phần DB/Redis connect thật), `internal/application/plan/dispatch_task.go` (Change 4 của CR — gọi thẳng use case, không qua workflow), và điểm chạm vào `cmd/server/main.go`.

---

## Acceptance Criteria

- [ ] `config.Config` có đủ field Orca theo bảng dưới, comment mặc định `OrcaURL` ghi đúng `http://orca:6769` (KHÔNG phải `:3000`)
- [ ] `cmd/worker/main.go` kết nối `DatabaseURL` + `RedisURL` thật (không còn placeholder `mustConnectPostgres`/`mustConnectRedis` — implement theo TDD-02 §9 connection pool convention)
- [ ] `internal/application/plan/dispatch_task.go`: `DispatchTaskUseCase.Execute(ctx, taskID)` gọi thẳng `dispatchToOrcaUC.Execute()` — **không** qua `port.QueuePublisher`/asynq ở bước này (chỉ `DispatchToOrcaUseCase` bên trong nó mới enqueue poll)
- [ ] `plan.DispatchTaskUseCase` mapping chỉ dùng field có thật trên `PlanTask` (CR-TASK-001) — field thiếu đánh dấu `TODO(CR-TASK-003)`, không bịa field mới
- [ ] `.env.example` bổ sung đủ biến `ORCA_*` với giá trị mẫu đúng port `:6769`
- [ ] `go build ./...` (toàn bộ `planner-service`) thành công
- [ ] `go vet ./...` sạch

---

## File 1: `config/config.go` [MODIFY — bổ sung phần Orca]

```go
package config

import "github.com/kelseyhightower/envconfig"

type Config struct {
	Port        string `envconfig:"PORT" default:"3013"`
	Env         string `envconfig:"ENV" default:"development"`
	DatabaseURL string `envconfig:"DATABASE_URL" required:"true"`
	RedisURL    string `envconfig:"REDIS_URL" required:"true"`

	// Orca integration — SOL-ORCA-002 §3.11.
	OrcaURL string `envconfig:"ORCA_URL" required:"true"`
	// NOTE: CR-ORCA-002 §Change 5 gốc ghi default comment "http://orca:3000" —
	// SAI, đã xác nhận với Orca thật ở SOL-ORCA-001 §9: port HTTP thật là
	// :6769. Sửa tại đây — xem SOL-ORCA-002 §9 Discoveries #7.
	// gợi ý giá trị: http://orca:6769
	OrcaAPISecret           string `envconfig:"ORCA_PLANNER_API_SECRET" required:"true"` // secret MỚI, chưa tồn tại phía Orca hôm nay
	OrcaCallbackURL         string `envconfig:"ORCA_CALLBACK_URL" default:"http://planner-service:3013/api/v1/orca-callback"`
	OrcaPollIntervalSecs    int    `envconfig:"ORCA_POLL_INTERVAL_SECS" default:"30"`
	OrcaDefaultAgentType    string `envconfig:"ORCA_DEFAULT_AGENT_TYPE" default:"claude"`
	OrcaDefaultTimeoutHours int    `envconfig:"ORCA_DEFAULT_TIMEOUT_HOURS" default:"8"`
	OrcaHTTPTimeoutSecs     int    `envconfig:"ORCA_HTTP_TIMEOUT_SECS" default:"15"`
}

func Load() (*Config, error) {
	var cfg Config
	if err := envconfig.Process("", &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
```

`.env.example` bổ sung:

```bash
# planner-service — Orca integration
ORCA_URL=http://orca:6769
ORCA_PLANNER_API_SECRET=<strong-random-secret>
ORCA_CALLBACK_URL=http://planner-service:3013/api/v1/orca-callback
ORCA_POLL_INTERVAL_SECS=30
ORCA_DEFAULT_AGENT_TYPE=claude
ORCA_DEFAULT_TIMEOUT_HOURS=8
ORCA_HTTP_TIMEOUT_SECS=15
```

---

## File 2: `internal/application/plan/dispatch_task.go` [NEW — thuộc CR-ORCA-002 Change 4, tích hợp domain `plantask` của CR-TASK-001]

```go
// Package plan bridges planner-service's task-planning domain (plantask,
// CR-TASK-001) to Orca dispatch (dispatch, CR-ORCA-002). This file is the
// concrete implementation of CR-ORCA-002 Change 4: when a PlanTask needs an
// AI agent, call DispatchToOrcaUseCase.Execute() directly — NO workflow
// engine, NO queue hop for the dispatch trigger itself (DispatchToOrcaUseCase
// enqueues its own poll chain internally, see TASK-ORCA-002-04).
package plan

import (
	"context"
	"fmt"

	"github.com/vnptech/kwp/services/planner-service/internal/application/dispatch"
	"github.com/vnptech/kwp/services/planner-service/internal/domain/plantask" // CR-TASK-001
)

type DispatchTaskUseCase struct {
	taskRepo         plantask.Repository
	dispatchToOrcaUC *dispatch.DispatchToOrcaUseCase
}

func NewDispatchTaskUseCase(tr plantask.Repository, d *dispatch.DispatchToOrcaUseCase) *DispatchTaskUseCase {
	return &DispatchTaskUseCase{taskRepo: tr, dispatchToOrcaUC: d}
}

// Execute dispatches PlanTask[taskID] to Orca. Triggered from an HTTP handler
// (POST /api/v1/tasks/{id}/dispatch, CR-TASK-001/002 scope) or from the plan
// domain's own "pick next ready task" loop — either way, this method itself
// makes ONE synchronous call into DispatchToOrcaUseCase.Execute() and returns;
// no workflow/queue sits between plan and dispatch.
func (uc *DispatchTaskUseCase) Execute(ctx context.Context, taskID string) error {
	t, err := uc.taskRepo.FindByID(ctx, taskID)
	if err != nil {
		return err
	}

	// NOTE: PlanTask (CR-TASK-001 §Change 1, package plantask) does NOT yet
	// have WorktreeRepo/WorktreeBranch/AcceptanceCriteria/AntiPatterns/
	// RequiredPatterns fields that a fully-populated DispatchToOrcaInput could
	// use — those are expected to land via CR-TASK-003 (Task Context
	// Enrichment). Map only what exists today; do not invent fields on
	// PlanTask here (out of CR-ORCA-002's scope) — see SOL-ORCA-002 §9
	// Discoveries #6.
	result, err := uc.dispatchToOrcaUC.Execute(ctx, dispatch.DispatchToOrcaInput{
		PlannerTaskID:   t.ID,
		PlannerJobID:    t.PlanID,
		CRID:            t.ExternalRef, // KGP TASK ID — closest CR-facing identifier PlanTask has today
		Title:           t.Title,
		TaskFileContent: t.Description,
		AgentType:       t.AgentType,
		Priority:        t.Priority,
		WHYChain:        t.WHYChain,
		TimeoutHours:    8,
		// TODO(CR-TASK-003): WorktreeRepo, WorktreeBranch, AntiPatterns,
		// RequiredPatterns, AcceptanceCriteria — populate once PlanTask (or its
		// enrichment sidecar) carries them.
	})
	if err != nil {
		return fmt.Errorf("dispatch task %s to orca: %w", taskID, err)
	}

	// TODO(CR-TASK-001/004): PlanTask has no OrcaTaskID field today, and
	// plantask.Repository has no UpdateOrcaTaskID method — orcatask.Tracking
	// (TASK-ORCA-002-02) already holds the planner_task_id -> orca_task_id
	// mapping, so the plan domain can look it up via
	// orcatask.Repository.FindByPlannerTaskID instead of duplicating the value
	// on PlanTask. Final decision belongs to CR-TASK-001/CR-ORCA-004, not here.
	_ = result
	return nil
}
```

---

## File 3: `cmd/worker/main.go` [MODIFY — hoàn thiện phần connect thật, thay khung ở TASK-05]

```go
package main

import (
	"context"
	"log"
	"log/slog"
	"os/signal"
	"syscall"
	"time"

	hibikenasynq "github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"

	"github.com/vnptech/kwp/services/planner-service/config"
	"github.com/vnptech/kwp/services/planner-service/internal/infrastructure/events"
	"github.com/vnptech/kwp/services/planner-service/internal/infrastructure/http/orcaclient"
	"github.com/vnptech/kwp/services/planner-service/internal/infrastructure/persistence/postgres"
	asynqinfra "github.com/vnptech/kwp/services/planner-service/internal/infrastructure/queue/asynq"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	db := mustConnectPostgres(ctx, cfg.DatabaseURL) // sqlx wrapper over pgxpool, TDD-02 §9
	defer db.Close()

	rdb := redis.NewClient(&redis.Options{Addr: cfg.RedisURL})
	defer rdb.Close()

	orcaHTTP := orcaclient.New(cfg.OrcaURL, cfg.OrcaAPISecret,
		time.Duration(cfg.OrcaHTTPTimeoutSecs)*time.Second, slog.Default())
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

	slog.Info("planner-service worker started",
		"orca_url", cfg.OrcaURL, "port", cfg.Port)

	if err := srv.Start(mux); err != nil {
		log.Fatal(err)
	}
	defer srv.Shutdown()

	<-ctx.Done()
	slog.Info("planner-service worker stopped gracefully")
}

func mustConnectPostgres(ctx context.Context, databaseURL string) *sqlx.DB {
	// implement theo TDD-02 §9 (pgxpool.Config: MaxConns 20, MinConns 2,
	// MaxConnLifetime 30m, MaxConnIdleTime 5m, HealthCheckPeriod 1m), wrap
	// bằng sqlx.NewDb cho tương thích NamedExecContext/GetContext dùng ở
	// TASK-ORCA-002-03.
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		log.Fatalf("connect postgres: %v", err)
	}
	return sqlx.NewDb(stdlibOpenDB(pool), "pgx")
}
```

> `stdlibOpenDB` — helper chuyển `*pgxpool.Pool` sang `*sql.DB` (`github.com/jackc/pgx/v5/stdlib`), theo pattern kết nối Postgres chung đã dùng ở các service Go khác trong `vnp-workplace` — tái sử dụng helper có sẵn trong `pkg/common` nếu đã tồn tại, không viết trùng.

> **Điểm chạm `cmd/server/main.go` (thuộc CR-TASK-001, không viết lại toàn bộ ở đây):** thêm vào DI container hiện có — `orcaHTTP`, `dispatchRepo` (`postgres.NewOrcaTaskTrackingRepository`), `publisher` (`asynqinfra.NewPublisher`), `eventPub` (`events.NewRedisPublisher`) dùng chung instance với `cmd/worker`; khởi tạo `dispatchToOrcaUC := dispatch.NewDispatchToOrcaUseCase(orcaHTTP, dispatchRepo, publisher, eventPub, dispatch.Config{CallbackURL: cfg.OrcaCallbackURL, DefaultTimeoutHours: cfg.OrcaDefaultTimeoutHours})` rồi `dispatchTaskUC := plan.NewDispatchTaskUseCase(planTaskRepo, dispatchToOrcaUC)`; wire `dispatchTaskUC` vào handler `POST /api/v1/tasks/{id}/dispatch` (route cụ thể thuộc CR-TASK-001/002).

---

## Test File 4: `internal/application/plan/dispatch_task_test.go`

```go
func TestDispatchTaskUseCase_Execute_MapsAvailablePlanTaskFieldsAndCallsDispatchToOrcaUseCase(t *testing.T) {
	// fake plantask.Repository.FindByID trả PlanTask hợp lệ
	// fake DispatchToOrcaUseCase (qua interface nhỏ nếu cần cho test, hoặc dùng
	// dispatch.DispatchToOrcaUseCase thật với port fakes bên dưới nó)
	// assert: input tới DispatchToOrcaUseCase.Execute khớp field PlanTask (ID, PlanID,
	// ExternalRef, Title, Description, AgentType, Priority, WHYChain)
}

func TestDispatchTaskUseCase_Execute_TaskNotFound_ReturnsError(t *testing.T) {
	// fake plantask.Repository.FindByID trả lỗi — assert Execute() propagate lỗi, không panic
}

func TestDispatchTaskUseCase_Execute_DispatchError_WrapsWithTaskID(t *testing.T) {
	// dispatchToOrcaUC.Execute trả lỗi — assert error message chứa taskID
}
```

---

## Verification

```bash
cd /opt/repos/vnp-workplace/backend/services/planner-service

go build ./...
go vet ./...
go test ./... -v -race -cover

# Smoke test cục bộ (cần Postgres + Redis chạy qua deploy/dev/docker-compose.yml của vnp-workplace)
go run ./cmd/worker   # kỳ vọng log "planner-service worker started" với orca_url đúng cấu hình
```

**Không** cắm `OrcaURL` vào Orca thật ở bước smoke test này — endpoint `/api/planner-tasks*` chưa tồn tại (xem TASK-ORCA-001-13). Dùng `httptest.Server` local hoặc để `orca:poll_status` fail có kiểm soát (log `orcaclient.ErrUnavailable`/connection refused, tự retry theo `MaxRetry(3)`) là kết quả chấp nhận được cho tới khi Orca team hoàn thành.
