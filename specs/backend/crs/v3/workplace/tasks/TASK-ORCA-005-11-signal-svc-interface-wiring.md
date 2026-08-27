> ⚠️ **SUPERSEDED (2026-08-10):** Tài liệu task/solution breakdown này được sinh ra từ các CR-ORCA-00x/CR-TASK-00x đã bị viết lại hoặc retired — dựa trên giả định sai rằng `vnp-planner` bị loại bỏ và `vnp-workplace` tự dựng `planner-service` mới. Kiến trúc đúng: xem [`docs/crs/v3/orca/README.md`](../../../../../docs/crs/v3/orca/README.md). Nội dung bên dưới chỉ còn giá trị tham khảo lịch sử — không dùng để implement.
>
> ---
>
## Status: 🔲 NOT STARTED

# TASK-ORCA-005-11 — `planner-service`: HTTP Interface + Config/`cmd/server/main.go` Wiring

**Phase:** 3 — Read-model / observability
**Scope:** ✅ `vnp-workplace` — Go (`planner-service`, module `orcasession`)
**Source:** [SOL-ORCA-005 §3.6, §3.8](../solutions/SOL-ORCA-005-planner-orca-session-monitor.md#38-session-dashboard-rest-api-planner-service-không-phải-diagnostics-service)
**Depends On:** [TASK-ORCA-005-09](./TASK-ORCA-005-09-signal-svc-application.md), [TASK-ORCA-005-10](./TASK-ORCA-005-10-signal-svc-infrastructure.md)
**Estimated Files:** ~6 files
**Working Dir:** `backend/services/planner-service/`

> **Đổi tên khỏi bản gốc:** giữ nguyên số hiệu/tên file (`TASK-ORCA-005-11`, nhắc `signal-svc`) nhưng nội dung nói về REST API + wiring của module `orcasession` trong `planner-service` (`:3013`). **KHÔNG có Temporal `client.Client`** — bản gốc cần nó cho `/cancel` (Signal cross-service), CR mới bỏ hoàn toàn Temporal (xem SOL-ORCA-005 §3.10).

---

## Bối cảnh quan trọng

- `planner-service` là service **MỚI** — task này (cùng với CR-ORCA-002/004, ngoài phạm vi ở đây) là một trong những task đầu tiên tạo ra `cmd/server/main.go`/`config/config.go` của service. Nếu CR-ORCA-002/004 đã tạo các file này trước (thứ tự thực thi không cố định), **thêm vào router/config đã có**, không tạo `http.Server`/`echo.Echo` thứ hai — nguyên tắc "1 router 1 port" (đã dùng xuyên suốt `diagnostics-service`/`skills-service`/`plugin-registry`, xem `internal/presentation/http/router.go` của các service đó).
- Framework HTTP: **Echo v4** (`github.com/labstack/echo/v4`) — khớp mọi service Go thật hiện có trong monorepo (`diagnostics-service`, `skills-service`, `agent-gateway`, ...), KHÔNG dùng `gorilla/mux` như bản gốc `signal-svc` từng giả định.
- **KHÔNG có Temporal dependency** — route `POST /api/v1/planner/orca-sessions/{id}/cancel` gọi `CancelSessionCommand` (TASK-ORCA-005-09), forward tới `DispatchCancelPort` **in-process** (module dispatch cùng service), không phải Signal qua network.
- **Redis client**: `planner-service` giờ cần một `*redis.Client` (cho `ReloadEventPublisher`, TASK-ORCA-005-10) — dependency MỚI cho service này nếu CR-ORCA-002/004 chưa có sẵn lý do khác để cần Redis. Phối hợp: nếu CR-ORCA-002 đã tạo `redisClient` trong `main.go` cho mục đích khác (vd. idempotency lock), tái dùng CÙNG client (giống cách `skills-service` share 1 `redis.Client` giữa `messaging.NewReloadEventPublisherFromClient` và các use case khác).
- **Trace proxy** (`GET /api/v1/planner/orca-sessions/{id}/trace`) là **on-demand SSE pass-through**, KHÔNG phải background republisher (SOL-ORCA-005 §3.6) — handler mở 1 request `GET {orca_base_url}/api/trace-stream` mỗi khi có client gọi, copy byte-stream response thẳng cho client, đóng khi client hoặc Orca đóng kết nối. Không cần logic reconnect-with-backoff (client's `EventSource` tự retry theo SSE spec, gọi lại route này).

---

## Mục tiêu

REST API `/api/v1/planner/orca-sessions/**` + `/api/v1/planner/orca/health`, và nối domain/application/infrastructure (TASK-ORCA-005-08..10) thành 1 module chạy được trong `planner-service`.

---

## Acceptance Criteria

- [ ] `GET /api/v1/planner/orca-sessions` — list, lọc theo `workspace_id` query param nếu có (nếu không có, trả toàn bộ — theo `ListSessionsQuery.Handle`, TASK-ORCA-005-09)
- [ ] `GET /api/v1/planner/orca-sessions/{planner_task_id}` — trả `404` nếu không tìm thấy (`errors.Is(err, orcasession.ErrNotFound)`)
- [ ] `GET /api/v1/planner/orca-sessions/{planner_task_id}/trace` — SSE **pass-through on-demand**, KHÔNG publish Redis, KHÔNG dùng `SessionStore`/`Broadcaster` nội bộ nào (khác hẳn `diagnostics-service`'s SSE — đây proxy trực tiếp response của Orca)
- [ ] `POST /api/v1/planner/orca-sessions/{planner_task_id}/cancel` — gọi `CancelSessionCommand.Handle`, KHÔNG gọi `OrcaClientPort.CancelTask` trực tiếp từ handler
- [ ] `GET /api/v1/planner/orca/health` — trả trạng thái cached từ `OrcaHealthMonitor` (không gọi Orca đồng bộ trong request)
- [ ] `cmd/server/main.go` khởi động `OrcaSessionMonitor.Run`, `OrcaHealthMonitor.Run` như goroutine độc lập, gọi `OrcaSessionMonitor.Bootstrap` TRƯỚC khi HTTP server bắt đầu nhận traffic (giống nguyên tắc thứ tự khởi động của `diagnostics-service.Watcher.Start`, TDD-25 §13)
- [ ] `config.go` có đủ field: `OrcaBaseURL`, `OrcaSessionCheckInterval` (default 60s), `OrcaHealthInterval` (default 30s), `RedisAddr`/`RedisURL` (nếu chưa có sẵn từ CR-ORCA-002)
- [ ] `go build ./...` (toàn bộ `planner-service`, ít nhất phần module `orcasession` + main) thành công
- [ ] KHÔNG có import `go.temporal.io/sdk/client` ở bất kỳ file nào của task này

---

## File 1: `config/config.go` [MODIFY — thêm vào config đã có của `planner-service`, không ghi đè]

```go
package config

import "time"

// ... field hiện có của planner-service (Port, Env, DatabaseURL, RedisAddr/RedisURL,
// dispatch/callback config từ CR-ORCA-002/004 — KHÔNG liệt kê lại ở đây) ...

// Orca session monitor (CR-ORCA-005, module orcasession) — thêm vào struct
// Config hiện có, không tạo struct mới.
type Config struct {
	// ... field hiện có ...

	OrcaBaseURL              string        `envconfig:"ORCA_BASE_URL" required:"true"` // vd. http://orca:6769 — port thật cần xác nhận qua CR-ORCA-001/006
	OrcaSessionCheckInterval time.Duration `envconfig:"ORCA_SESSION_CHECK_INTERVAL" default:"60s"`
	OrcaHealthInterval       time.Duration `envconfig:"ORCA_HEALTH_INTERVAL" default:"30s"`

	// RedisAddr — CHỈ thêm nếu planner-service chưa có field Redis nào từ
	// CR-ORCA-002/004. Nếu đã có (vd. RedisURL cho mục đích khác), tái dùng
	// field đó thay vì thêm field trùng — kiểm tra config.go hiện tại
	// trước khi thêm.
	RedisAddr string `envconfig:"REDIS_ADDR" required:"true"`
}
```

(Thêm `import "time"` nếu chưa có.)

---

## File 2: `internal/presentation/http/handler/orca_session_handler.go`

```go
package handler

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"

	appsession "github.com/vnptech/kwp/services/planner-service/internal/application/orcasession"
	domain "github.com/vnptech/kwp/services/planner-service/internal/domain/orcasession"
)

type OrcaSessionHandler struct {
	Get    *appsession.GetSessionQuery
	List   *appsession.ListSessionsQuery
	Cancel *appsession.CancelSessionCommand
}

func (h *OrcaSessionHandler) ListHandler(c echo.Context) error {
	workspaceID := c.QueryParam("workspace_id")
	sessions, err := h.List.Handle(c.Request().Context(), workspaceID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal_error"})
	}
	return c.JSON(http.StatusOK, sessions)
}

func (h *OrcaSessionHandler) GetHandler(c echo.Context) error {
	plannerTaskID := c.Param("planner_task_id")
	session, err := h.Get.Handle(c.Request().Context(), plannerTaskID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "not_found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal_error"})
	}
	return c.JSON(http.StatusOK, session)
}

func (h *OrcaSessionHandler) CancelHandler(c echo.Context) error {
	plannerTaskID := c.Param("planner_task_id")
	if err := h.Cancel.Handle(c.Request().Context(), plannerTaskID); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "cancel_failed"})
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "cancel_requested"})
}
```

---

## File 3: `internal/presentation/http/handler/orca_health_handler.go`

```go
package handler

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// OrcaHealthHandler serves the CACHED health state maintained by
// orcasession.OrcaHealthMonitor — does NOT call Orca synchronously per
// request (SOL-ORCA-005 §3.8).
type OrcaHealthHandler struct {
	// Snapshot reads OrcaHealthMonitor's last-known state — implement as a
	// small accessor method added to OrcaHealthMonitor (TASK-ORCA-005-09),
	// e.g. `func (m *OrcaHealthMonitor) LastKnownHealthy() (healthy bool, ok bool)`,
	// rather than duplicating health-check logic here.
	Snapshot func() (healthy bool, known bool)
}

func (h *OrcaHealthHandler) Handle(c echo.Context) error {
	healthy, known := h.Snapshot()
	if !known {
		return c.JSON(http.StatusServiceUnavailable, map[string]any{"healthy": false, "checked": false})
	}
	status := http.StatusOK
	if !healthy {
		status = http.StatusServiceUnavailable
	}
	return c.JSON(status, map[string]any{"healthy": healthy, "checked": true})
}
```

> **Ghi chú:** `OrcaHealthMonitor` (TASK-ORCA-005-09) hiện chỉ có field private `lastHealthy *bool` — cần thêm accessor `LastKnownHealthy() (bool, bool)` (trả `healthy, known`) khi implement task này. Không sửa `TASK-ORCA-005-09` để thêm field này retroactively — chỉ cần thêm method, tương thích ngược.

---

## File 4: `internal/presentation/http/handler/orca_trace_handler.go`

```go
package handler

import (
	"io"
	"net/http"

	"github.com/labstack/echo/v4"
)

// OrcaTraceHandler proxies GET {orca_base_url}/api/trace-stream ON DEMAND,
// per client connection — NOT a background republisher (SOL-ORCA-005 §3.6,
// contrast with the pre-rewrite signal-svc OrcaTraceProxy which ran as a
// standalone goroutine forwarding to NATS). No reconnect-with-backoff logic
// here: if Orca's stream drops, this handler's response simply ends: the
// client's EventSource reconnects per the SSE spec, which re-invokes this
// route and opens a fresh upstream connection.
type OrcaTraceHandler struct {
	OrcaBaseURL string
	HTTPClient  *http.Client
}

func (h *OrcaTraceHandler) Handle(c echo.Context) error {
	req, err := http.NewRequestWithContext(c.Request().Context(), http.MethodGet, h.OrcaBaseURL+"/api/trace-stream", nil)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "trace_proxy_build_failed"})
	}

	resp, err := h.HTTPClient.Do(req)
	if err != nil {
		return c.JSON(http.StatusBadGateway, map[string]string{"error": "orca_trace_stream_unreachable"})
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return c.JSON(http.StatusBadGateway, map[string]string{"error": "orca_trace_stream_error"})
	}

	w := c.Response()
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	// Straight byte-stream pass-through — SOL-ORCA-005 §3.6 notes the real
	// Orca trace-stream is UNFILTERED by orca_task_id (global backend
	// trace), so this handler cannot filter server-side either; document
	// that limitation for the client (Open Task, out of scope to fix here).
	buf := make([]byte, 4096)
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := w.Write(buf[:n]); writeErr != nil {
				return nil // client gone
			}
			w.Flush()
		}
		if readErr == io.EOF {
			return nil
		}
		if readErr != nil {
			return nil // upstream closed — client's EventSource will reconnect
		}
	}
}
```

---

## File 5: `internal/presentation/http/router.go` [MODIFY — thêm vào router đã có]

```go
// Register (đã tồn tại từ CR-ORCA-002/004) — chỉ thêm nhóm route dưới đây,
// KHÔNG tạo echo.Echo/http.Server mới.
func registerOrcaSessionRoutes(api *echo.Group, h OrcaSessionHandlers) {
	api.GET("/planner/orca-sessions", h.Session.ListHandler)
	api.GET("/planner/orca-sessions/:planner_task_id", h.Session.GetHandler)
	api.GET("/planner/orca-sessions/:planner_task_id/trace", h.Trace.Handle)
	api.POST("/planner/orca-sessions/:planner_task_id/cancel", h.Session.CancelHandler)
	api.GET("/planner/orca/health", h.Health.Handle)
}

type OrcaSessionHandlers struct {
	Session *handler.OrcaSessionHandler
	Health  *handler.OrcaHealthHandler
	Trace   *handler.OrcaTraceHandler
}
```

> Gắn `registerOrcaSessionRoutes` vào `Register(e *echo.Echo, ...)` đã có của `planner-service` (do CR-ORCA-002/004 tạo) — dưới cùng `api := e.Group("/api/v1")` với auth middleware đã áp dụng cho các route khác (`middleware.ExtractUserContext()`/`RequireRole`, theo đúng pattern `skills-service`), không tạo group auth riêng.

---

## File 6: `cmd/server/main.go` [MODIFY — bổ sung vào wiring đã có]

```go
// Bổ sung vào main() hiện có của planner-service, sau khi Redis client đã kết nối:

sessionStore := memory.NewSessionStore()
reloadPublisher := messaging.NewReloadEventPublisherFromClient(redisClient) // redisClient: tái dùng nếu CR-ORCA-002/004 đã tạo

orcaAdapter := orcaclientinfra.NewAdapter(cfg.OrcaBaseURL, 15*time.Second)

timeoutHandler := &appsession.OrcaTimeoutHandler{
	OrcaClient:       orcaAdapter,
	TaskRetryUseCase: nil, // CR-TASK-006, ngoài phạm vi bộ Orca — wire khi có
}

sessionMonitor := &appsession.OrcaSessionMonitor{
	Store: sessionStore, OrcaClient: orcaAdapter, Publisher: reloadPublisher,
	TimeoutHandler: timeoutHandler, CheckInterval: cfg.OrcaSessionCheckInterval, Logger: logger,
}

healthMonitor := &appsession.OrcaHealthMonitor{
	Client: orcaAdapter, Publisher: reloadPublisher,
	ActiveWorkspaces: func() []string {
		seen := map[string]struct{}{}
		var out []string
		for _, s := range sessionStore.List() {
			if _, ok := seen[s.WorkspaceID]; !ok {
				seen[s.WorkspaceID] = struct{}{}
				out = append(out, s.WorkspaceID)
			}
		}
		return out
	},
	Interval: cfg.OrcaHealthInterval, Logger: logger,
}

// Bootstrap TRƯỚC khi HTTP server nhận traffic — giống nguyên tắc thứ tự
// khởi động của diagnostics-service.Watcher.Start (TDD-25 §13). dispatchQuery
// đến từ module dispatch (CR-ORCA-002) — Open Task nếu module đó chưa export
// ActiveDispatchQueryPort implementation lúc wiring task này.
if dispatchQuery != nil { // nil-safe cho tới khi CR-ORCA-002 cung cấp thật
	if err := sessionMonitor.Bootstrap(ctx, dispatchQuery); err != nil {
		logger.Error("orca session monitor bootstrap failed", "err", err)
	}
}

go sessionMonitor.Run(ctx)
go healthMonitor.Run(ctx)

sessionHandler := &handler.OrcaSessionHandler{
	Get:    &appsession.GetSessionQuery{Store: sessionStore},
	List:   &appsession.ListSessionsQuery{Store: sessionStore},
	Cancel: &appsession.CancelSessionCommand{Dispatch: dispatchCancelPort}, // module dispatch, Open Task
}
healthHandler := &handler.OrcaHealthHandler{Snapshot: healthMonitor.LastKnownHealthy}
traceHandler := &handler.OrcaTraceHandler{OrcaBaseURL: cfg.OrcaBaseURL, HTTPClient: &http.Client{Timeout: 0}} // 0: SSE dài hạn, dựa vào ctx cancellation

registerOrcaSessionRoutes(api, OrcaSessionHandlers{Session: sessionHandler, Health: healthHandler, Trace: traceHandler})

// e.Start(":" + cfg.Port) — ĐÃ có sẵn từ CR-ORCA-002/004, KHÔNG gọi lần 2.
```

> Port `:3013` là port thật của `planner-service` (chốt theo `docs/crs/v3/orca/README.md` §Ghi chú Re-scope: "`planner-service` nhận port `:3013`") — không phải giá trị gợi ý cần đoán như bản gốc `signal-svc` (`:8005`).

---

## Test File 7: `internal/presentation/http/handler/*_test.go`

```go
func TestOrcaSessionHandler_ListHandler_ReturnsSessions(t *testing.T)
func TestOrcaSessionHandler_GetHandler_ReturnsDetail(t *testing.T)
func TestOrcaSessionHandler_GetHandler_UnknownID_Returns404(t *testing.T)
func TestOrcaSessionHandler_CancelHandler_CallsCancelCommand_NotOrcaClientDirectly(t *testing.T)
func TestOrcaHealthHandler_ReturnsServiceUnavailable_WhenUnhealthy(t *testing.T)
func TestOrcaHealthHandler_ReturnsServiceUnavailable_WhenNotYetChecked(t *testing.T)
func TestOrcaTraceHandler_ProxiesUpstreamBytes(t *testing.T)          // httptest.Server giả lập Orca /api/trace-stream
func TestOrcaTraceHandler_ReturnsBadGateway_WhenOrcaUnreachable(t *testing.T)
```

---

## Verification

```bash
cd backend/services/planner-service

go build ./...
go vet ./...
go test ./internal/presentation/... -v -race -cover
go test ./internal/presentation/... -coverprofile=iface_cov.out
go tool cover -func=iface_cov.out | grep total   # kỳ vọng >= 60%

go run ./cmd/server/main.go   # kỳ vọng log khởi động không lỗi — không cần Orca thật chạy
```
