> ⚠️ **SUPERSEDED (2026-08-10):** Tài liệu task/solution breakdown này được sinh ra từ các CR-ORCA-00x/CR-TASK-00x đã bị viết lại hoặc retired — dựa trên giả định sai rằng `vnp-planner` bị loại bỏ và `vnp-workplace` tự dựng `planner-service` mới. Kiến trúc đúng: xem [`docs/crs/v3/orca/README.md`](../../../../../docs/crs/v3/orca/README.md). Nội dung bên dưới chỉ còn giá trị tham khảo lịch sử — không dùng để implement.
>
> ---
>
## Status: 🔲 NOT STARTED

# TASK-ORCA-004-07 — `planner-service`: `OrcaCallbackHandler` (nhận callback push từ Orca, qua `api-gateway`)

**Phase:** 2 — song song, không chặn
**Scope:** ✅ `vnp-workplace` — Go (`backend/services/planner-service`) + `backend/services/api-gateway` (reverse-proxy wiring)
**Source:** [SOL-ORCA-004 §3.3–§3.4](../solutions/SOL-ORCA-004-orca-result-reporter.md#33-route-nhận-phía-planner-service--orcacallbackhandler)
**Ghi chú re-scope (2026-08-10):** File task này trước đây tên `plan-svc-orca-callback-handler` (phía nhận là `plan-svc`, `vnp-planner`, dùng `client.SignalWorkflow` unblock `temporal-worker`). Sau quyết định kiến trúc 2026-08-10, phía nhận là **`planner-service`** (`vnp-workplace`, `:3013`), đăng ký reverse-proxy tại **`api-gateway`** — **không** có Temporal trong `vnp-workplace`. Tên file giữ nguyên (không đổi tên file theo yêu cầu), nội dung viết lại toàn bộ theo CR-ORCA-004 đã re-scope.
**Depends On:** không còn phụ thuộc `TASK-ORCA-002-05` (quy ước `workflow_id`, thuộc `temporal-worker`/`vnp-planner`, không đổi trong đợt re-scope này) — xem "Open Decision" bên dưới.
**Blocked by (Orca team, chỉ cho integration thật):** [TASK-ORCA-004-15](./TASK-ORCA-004-15-orca-result-collector-callback-publisher.md) — Orca chưa gọi callback này (chưa tồn tại `PlannerResultCollector`/`PlannerCallbackPublisher`). Task này **không bị block** để code + unit test (giả lập request bằng Echo test context / `httptest`).
**Estimated Files:** ~5 files (2 ở `planner-service`, 1 ở `api-gateway`, 2 test)
**Working Dir:** `/opt/repos/vnp-workplace/backend/services/planner-service/` (+ `backend/services/api-gateway/` cho phần wiring reverse-proxy)

---

## Bối cảnh quan trọng

1. **`planner-service` là service MỚI, đang được viết song song trong cùng phiên làm việc này** (xem `docs/crs/v3/orca/README.md` §Ghi chú Re-scope). Task này chỉ đặc tả phần `OrcaCallbackHandler` — layer `internal/presentation/http/handler/` theo đúng template chuẩn của monorepo (`backend/specs/tdd/v1/01-project-structure.md` §Per-Service Structure), dùng **Echo** (không phải `net/http`/`gorilla/mux` như convention cũ của `plan-svc`/`vnp-planner`). Nếu router HTTP của `planner-service` đã được dựng trước bởi phần công việc khác trong cùng phiên, **tái sử dụng** `internal/presentation/http/router.go` đã có — chỉ thêm route này vào, không tạo `echo.New()` thứ hai.

2. **Không còn "sửa lỗi `workflow.GetSignalChannel` trong Activity"** — bản gốc (`plan-svc`/`temporal-worker`) có lỗi kỹ thuật này, nhưng `planner-service` không dùng Temporal nên vấn đề không còn áp dụng. Bước "chờ/nhận" kết quả ở phía điều phối task là **Open Decision** (SOL-ORCA-004 §3.5, Option A/B, chưa chốt) — task này chỉ cần implement `ResultNotifier` interface + 1 impl no-op/log-only (Option A baseline), **không** tự triển khai Option B (cross-repo webhook) hay bất kỳ cơ chế signal nào chưa được xác nhận trong tài liệu.

3. `PlannerCallbackPublisher`/`PlannerResultCollector` phía Orca **chưa tồn tại** (0 kết quả `grep -rn "orca-callback\|callback_url" /opt/repos/orca`) — route này sẵn sàng nhận request đúng schema đã khoá ở SOL-ORCA-004 §3.2 ngay khi Orca team hoàn thành TASK-ORCA-004-15.

4. **Không block trên xử lý nội bộ nặng** — Orca có timeout callback ngắn (mặc định 10s, xem TASK-ORCA-006-16). `Handle` phải trả response ngay sau khi cập nhật task + gọi `ResultNotifier` (fire-and-forget về mặt độ trễ cho phép), không chờ các bước phụ trợ (vd. gọi ra service khác) xử lý xong nếu chúng không cần thiết cho response.

5. **Route đi qua `api-gateway` reverse-proxy, KHÔNG public trực tiếp `planner-service`** — Orca là external service (deploy headless, CR-ORCA-006), không nằm trong mạng nội bộ `vnp-workplace`. Endpoint public là `POST /api/v1/planner/orca-callback` tại `api-gateway`; `planner-service` đăng ký **cùng path** trên router nội bộ của nó (reverse-proxy không rewrite path). Route này nằm **ngoài** group yêu cầu user-auth (PAT/JWT) của gateway — cùng nhóm với `/api/v1/auth/*` — vì Orca xác thực bằng secret riêng (`ORCA_PLANNER_API_SECRET`), không phải WKP user.

---

## Mục tiêu

Implement `OrcaCallbackHandler` trong `planner-service`, nhận `POST /api/v1/planner/orca-callback` (qua `api-gateway` reverse-proxy), xác thực secret, cập nhật domain task nội bộ, và đăng ký route reverse-proxy `"planner"` tại `api-gateway`.

---

## Acceptance Criteria

### `planner-service`

- [ ] `POST /api/v1/planner/orca-callback` với `Authorization` sai → `401`, không cập nhật task
- [ ] Body không hợp lệ (JSON lỗi) → `400`
- [ ] Thiếu `planner_task_id` → `400`
- [ ] `planner_task_id` không tìm thấy trong domain của `planner-service` → `404`
- [ ] `Authorization` đúng + body hợp lệ → cập nhật task (`MarkCompleted`/`MarkFailed` theo `success`), gọi `DependencyResolver.OnTaskCompleted`, gọi `ResultNotifier.NotifyTaskCompleted`, trả `200 {"status":"ok"}`
- [ ] `ResultNotifier.NotifyTaskCompleted` trả lỗi → **vẫn** trả `200` (best-effort, không phải lỗi caller) — chỉ log cảnh báo
- [ ] Gọi callback 2 lần cho cùng `orca_task_id` (double-callback, retry mạng phía Orca) → cả 2 lần đều trả `200`, không lỗi 500
- [ ] Handler không chờ xử lý phụ trợ nặng trước khi trả response
- [ ] Giới hạn kích thước body (phòng vệ `agent_output`/`test_output` payload lớn — Orca giới hạn ~5000 ký tự cuối nhưng handler nên tự giới hạn thêm, ví dụ Echo `middleware.BodyLimit("256K")` áp riêng cho route này)
- [ ] `ResultNotifier` có ít nhất 1 impl `NoopResultNotifier` (Option A baseline, SOL-ORCA-004 §3.5) — ghi log, không gọi cross-repo
- [ ] `go build ./...` thành công, `go test ./internal/presentation/http/... -v -race -cover` coverage ≥ 60% (mức presentation layer, TDD-00 §12.3)

### `api-gateway`

- [ ] `ServiceProxy.targets` có thêm `"planner": mustParseURL(cfg.PlannerServiceURL)`
- [ ] Route `POST /api/v1/planner/orca-callback` đăng ký **ngoài** group `AuthMiddleware`/`RateLimitMiddleware` chuẩn (cùng nhóm `/api/v1/auth/*`)
- [ ] Reverse-proxy forward nguyên header `Authorization` cho route này (không strip/override như với `X-User-ID`/`X-Workspace-ID`)
- [ ] Config mới `PlannerServiceURL` (`envconfig:"PLANNER_SERVICE_URL"`, ví dụ `http://planner-service:3013`), theo convention `SYNC_SERVICE_URL`/`KNOWLEDGE_SERVICE_URL`/...
- [ ] Test: request `POST /api/v1/planner/orca-callback` không có PAT/JWT vẫn được forward tới `planner-service` (không bị `AuthMiddleware` chặn ở tầng gateway)

---

## File 1: `backend/services/planner-service/internal/presentation/http/handler/orca_callback_handler.go`

```go
// Package handler implements planner-service's HTTP presentation layer.
package handler

import (
	"github.com/labstack/echo/v4"
)

// OrcaCallbackPayload mirrors the JSON body Orca posts to callback_url
// (SOL-ORCA-004 §3.2). Field order/names must not change independently of
// the Orca-side contract — see TASK-ORCA-004-15.
type OrcaCallbackPayload struct {
	OrcaTaskID    string   `json:"orca_task_id"`
	PlannerTaskID string   `json:"planner_task_id"`
	PlannerJobID  string   `json:"planner_job_id"`
	Success       bool     `json:"success"`
	FilesCreated  []string `json:"files_created"`
	FilesModified []string `json:"files_modified"`
	CommitHash    string   `json:"commit_hash"`
	TestOutput    string   `json:"test_output"`
	ErrorMessage  string   `json:"error_message"`
	AgentOutput   string   `json:"agent_output"`
}

// TaskRepository, DependencyResolver, ResultNotifier are internal planner-service
// ports — full definitions live wherever planner-service's domain/application
// layers are wired up (out of scope for this task; only the shape used here matters).
type TaskRepository interface {
	GetByExternalRef(ctx context.Context, plannerTaskID string) (*PlannerTask, error)
	Update(ctx context.Context, task *PlannerTask) error
}

type DependencyResolver interface {
	OnTaskCompleted(ctx context.Context, taskID string)
}

// ResultNotifier — Open Decision (SOL-ORCA-004 §3.5). Do NOT implement a concrete
// cross-repo signal mechanism here; only wire the interface + NoopResultNotifier
// (File 2). Option A/B is not chốt yet.
type ResultNotifier interface {
	NotifyTaskCompleted(ctx context.Context, taskID string, payload OrcaCallbackPayload) error
}

// OrcaCallbackHandler receives Orca's result push at POST /api/v1/planner/orca-callback
// (reached via api-gateway reverse-proxy, unchanged path — see File 3). It does NOT
// implement retry/backoff — matches Orca's no-retry callback behaviour (SOL-ORCA-004 §3.2).
type OrcaCallbackHandler struct {
	APISecret          string
	TaskRepo           TaskRepository
	DependencyResolver DependencyResolver
	ResultNotifier     ResultNotifier
	Logger             *zap.Logger
}

// Handle implements POST /api/v1/planner/orca-callback.
//
// IMPORTANT — auth: this route MUST be registered at api-gateway outside the
// user-auth group (see File 3) because Orca authenticates with a shared secret,
// not a WKP user PAT/JWT. This handler validates that secret itself.
func (h *OrcaCallbackHandler) Handle(c echo.Context) error {
	auth := c.Request().Header.Get("Authorization")
	if auth != "Bearer "+h.APISecret {
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}

	var payload OrcaCallbackPayload
	if err := c.Bind(&payload); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid body")
	}
	if payload.PlannerTaskID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "planner_task_id required")
	}

	ctx := c.Request().Context()

	task, err := h.TaskRepo.GetByExternalRef(ctx, payload.PlannerTaskID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "planner task not found")
	}
	if payload.Success {
		task.MarkCompleted(payload.FilesCreated, payload.FilesModified, payload.CommitHash)
	} else {
		task.MarkFailed(payload.ErrorMessage)
	}
	if err := h.TaskRepo.Update(ctx, task); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "update task failed")
	}

	h.DependencyResolver.OnTaskCompleted(ctx, task.ID)

	if err := h.ResultNotifier.NotifyTaskCompleted(ctx, task.ID, payload); err != nil {
		// Best-effort — not a caller error. Log and still return 200 so Orca does not
		// treat this as a delivery failure it might otherwise (fruitlessly) retry.
		h.Logger.Warn("orca callback: result notifier failed",
			zap.String("planner_task_id", payload.PlannerTaskID), zap.Error(err))
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}
```

> `context`/`net/http`/`go.uber.org/zap` imports cần thêm khi hiện thực thật — lược bớt ở đây để tập trung vào chữ ký/hành vi. `PlannerTask`, `MarkCompleted`, `MarkFailed` là domain entity/method của `planner-service` — định nghĩa cụ thể thuộc phần công việc khác đang dựng domain layer của service này, task này chỉ cần chữ ký tối thiểu để `Handle` biên dịch được.

---

## File 2: `backend/services/planner-service/internal/infrastructure/notifier/noop_result_notifier.go` [NEW]

```go
// Package notifier — ResultNotifier implementations.
package notifier

import (
	"context"

	"go.uber.org/zap"

	"github.com/vnptech/kwp/services/planner-service/internal/presentation/http/handler"
)

// NoopResultNotifier is the Option A baseline (SOL-ORCA-004 §3.5): it does not call out
// to any cross-repo signal channel — none is chốt yet. It only logs. Replace with a real
// implementation once Option A (poll, no code change here) or Option B (cross-repo webhook,
// needs a new CR/decision) is chosen — do NOT add cross-repo call logic speculatively.
type NoopResultNotifier struct {
	Logger *zap.Logger
}

func (n *NoopResultNotifier) NotifyTaskCompleted(ctx context.Context, taskID string, payload handler.OrcaCallbackPayload) error {
	n.Logger.Info("orca callback processed (no cross-repo notify — Option A baseline)",
		zap.String("task_id", taskID), zap.Bool("success", payload.Success))
	return nil
}
```

---

## File 3: `backend/services/planner-service/internal/presentation/http/router.go` [MODIFY nếu đã tồn tại, NEW nếu chưa]

```go
// Trong hàm dựng router (Echo) của planner-service — thêm route này vào group
// KHÔNG áp middleware nội bộ tương đương "user session" (planner-service không có
// khái niệm PAT/JWT của WKP user — auth cho route này hoàn toàn nằm trong handler,
// §File 1). Nếu planner-service có middleware auth nội bộ khác (service-to-service),
// route này KHÔNG dùng middleware đó — Orca là external caller, không phải internal caller.
e.POST("/api/v1/planner/orca-callback", orcaCallbackHandler.Handle,
	middleware.BodyLimit("256K")) // defensive limit — Orca truncates agent_output to ~5000 chars
```

---

## File 4: `backend/services/api-gateway/internal/presentation/http/proxy/reverse_proxy.go` [MODIFY]

```go
func NewServiceProxy(cfg *config.Config) *ServiceProxy {
  targets := map[string]*url.URL{
    "sync":         mustParseURL(cfg.SyncServiceURL),
    "knowledge":    mustParseURL(cfg.KnowledgeServiceURL),
    "mcp":          mustParseURL(cfg.MCPServiceURL),
    "notification": mustParseURL(cfg.NotificationServiceURL),
    "planner":      mustParseURL(cfg.PlannerServiceURL), // [NEW] planner-service :3013
  }
  return &ServiceProxy{targets: targets}
}
```

Thêm `PlannerServiceURL string \`envconfig:"PLANNER_SERVICE_URL" required:"true"\`` vào `config.Config` của `api-gateway`.

---

## File 5: `backend/services/api-gateway/internal/presentation/http/router.go` [MODIFY]

```go
// Đăng ký NGOÀI group user-auth (api.Group("/api/v1", AuthMiddleware, RateLimitMiddleware))
// — cùng cách /api/v1/auth/* được đăng ký trực tiếp trên `e`, không qua `api` group.
// Orca xác thực bằng ORCA_PLANNER_API_SECRET riêng, tự validate trong OrcaCallbackHandler
// (planner-service) — KHÔNG dùng PAT/JWT của WKP user, KHÔNG áp AuthMiddleware ở đây.
e.POST("/api/v1/planner/orca-callback", proxy.Forward("planner"))
```

> Xác nhận `proxy.Forward()` (theo `03-api-gateway.md` §Reverse Proxy) **không** set/override `Authorization` — chỉ set `X-User-ID`/`X-Workspace-ID`/`X-Request-ID` (các header này sẽ rỗng/không set cho request Orca, không sao vì `planner-service` không cần chúng cho route này). Nếu implementation thật của `Forward()` có logic bắt buộc phải có `user_id` trong context (vd. panic khi type-assert `c.Get("user_id").(string)` mà giá trị là `nil`) — route Orca này **phải** dùng một code path/handler riêng không đi qua assumption đó, vì request không qua `AuthMiddleware` nên không có `user_id` trong Echo context.

---

## Test File 6: `backend/services/planner-service/internal/presentation/http/handler/orca_callback_handler_test.go`

```go
func TestOrcaCallbackHandler_ValidSecret_UpdatesTaskAndReturns200(t *testing.T) {
	// fake TaskRepository/DependencyResolver/ResultNotifier; assert task.MarkCompleted called,
	// DependencyResolver.OnTaskCompleted called with task.ID, response 200 {"status":"ok"}
}
func TestOrcaCallbackHandler_InvalidSecret_Returns401(t *testing.T)
func TestOrcaCallbackHandler_MissingPlannerTaskID_Returns400(t *testing.T)
func TestOrcaCallbackHandler_InvalidJSONBody_Returns400(t *testing.T)
func TestOrcaCallbackHandler_TaskNotFound_Returns404(t *testing.T)
func TestOrcaCallbackHandler_ResultNotifierError_StillReturns200(t *testing.T) {
	// fake ResultNotifier returns error; assert response is still 200, warning logged
}
func TestOrcaCallbackHandler_DoubleCallback_BothReturn200NoError(t *testing.T) {
	// gọi Handle 2 lần liên tiếp cùng payload; cả 2 lần đều 200
}
func TestOrcaCallbackHandler_OversizedBody_Rejected(t *testing.T) {
	// body > BodyLimit → 413/400 tuỳ hành vi middleware.BodyLimit của Echo
}
```

## Test File 7: `backend/services/api-gateway/internal/presentation/http/router_test.go` [MODIFY hoặc thêm case]

```go
func TestRouter_PlannerOrcaCallback_NotBlockedByAuthMiddleware(t *testing.T) {
	// POST /api/v1/planner/orca-callback không có header Authorization của WKP (PAT/JWT) —
	// request vẫn được forward tới target "planner" (httptest.Server giả lập planner-service),
	// KHÔNG bị AuthMiddleware trả 401 ở tầng gateway.
}
func TestReverseProxy_PlannerOrcaCallback_ForwardsAuthorizationHeaderUnmodified(t *testing.T) {
	// gửi Authorization: Bearer <orca-secret> — assert upstream request tới planner-service
	// nhận đúng header này nguyên vẹn (không bị strip/override).
}
```

---

## Verification

```bash
cd /opt/repos/vnp-workplace

go build ./services/planner-service/... ./services/api-gateway/...
go vet ./services/planner-service/... ./services/api-gateway/...

go test ./services/planner-service/internal/presentation/http/... -v -race -cover
go test ./services/planner-service/internal/presentation/http/... -coverprofile=orca_callback_cov.out
go tool cover -func=orca_callback_cov.out | grep total   # kỳ vọng >= 60%

go test ./services/api-gateway/internal/presentation/http/... -v -race

# Smoke test thủ công qua gateway (không cần Orca thật — mô phỏng callback):
curl -X POST http://localhost:3000/api/v1/planner/orca-callback \
  -H "Authorization: Bearer $ORCA_PLANNER_API_SECRET" \
  -H "Content-Type: application/json" \
  -d '{"orca_task_id":"orca-1","planner_task_id":"TASK-1","success":true}'
# Kỳ vọng: 200 {"status":"ok"} — request đi qua api-gateway (:3000), reverse-proxy sang
# planner-service (:3013), không bị chặn bởi AuthMiddleware của gateway.
```
