> ⚠️ **SUPERSEDED (2026-08-10):** Tài liệu task/solution breakdown này được sinh ra từ các CR-ORCA-00x/CR-TASK-00x đã bị viết lại hoặc retired — dựa trên giả định sai rằng `vnp-planner` bị loại bỏ và `vnp-workplace` tự dựng `planner-service` mới. Kiến trúc đúng: xem [`docs/crs/v3/orca/README.md`](../../../../../docs/crs/v3/orca/README.md). Nội dung bên dưới chỉ còn giá trị tham khảo lịch sử — không dùng để implement.
>
> ---
>
# SOL-ORCA-004 — Orca Result Reporter

| Field | Value |
|-------|-------|
| **CR ref** | [CR-ORCA-004](../../../../../../docs/crs/v3/orca/CR-ORCA-004-orca-result-reporter.md) |
| **Title** | Orca Result Reporter — báo kết quả về `vnp-workplace` (callback push) |
| **Service** | Orca `TaskService`/`PlannerResultCollector` (TypeScript) **+** `planner-service` (`vnp-workplace`, Go, `:3013`) **+** `api-gateway` (reverse-proxy) |
| **Priority** | P1 |
| **Risk** | medium |
| **Status** | 📐 PROPOSED |
| **Ghi chú re-scope (2026-08-10)** | Phía nhận callback chuyển từ `plan-svc` (`vnp-planner`) sang `planner-service` (`vnp-workplace`, `:3013`) — xem [`docs/crs/v3/orca/README.md`](../../../../../../docs/crs/v3/orca/README.md) §Ghi chú Re-scope. Change 1–3 (Orca-side collector/publisher) **giữ nguyên hoàn toàn**. Cơ chế "chờ kết quả" (trước đây là Temporal Signal do `plan-svc`/`temporal-worker` sở hữu) **không** có tương đương đã chốt trong `vnp-workplace` — `planner-service` không dùng Temporal (§2, §3.5). Đây là **quyết định còn mở**, tài liệu này không tự bịa ra một cơ chế cụ thể. |
| **Phạm vi** | Phần **thu thập kết quả & publish callback** thuộc repo `orca` — **ngoài phạm vi Go monorepo `vnp-workplace`**, chỉ mô tả contract. Phần **nhận callback** (`POST /api/v1/planner/orca-callback`, qua `api-gateway` reverse-proxy) chạy trong `planner-service` (Go, thuộc `vnp-workplace`) — SOL này đặc tả **hợp đồng giao diện** (route, request/response, auth, hành vi update task) mà `planner-service` phải hiện thực theo đúng Clean Architecture chuẩn của monorepo (`backend/specs/tdd/v1/00-go-conventions.md`, `01-project-structure.md`). Thiết kế nội bộ đầy đủ domain/application của `planner-service` (ngoài phần callback handler này) không thuộc phạm vi bộ CR Orca — service này đang được viết song song trong cùng phiên làm việc. |
| **TDD refs** | `backend/specs/tdd/v1/03-api-gateway.md` §Reverse Proxy (pattern `ServiceProxy.targets`, dùng cho `sync`/`knowledge`/`mcp`/`notification`), `backend/specs/tdd/v1/00-go-conventions.md`, `backend/specs/tdd/v1/01-project-structure.md` §Per-Service Structure (`internal/presentation/http/handler`) |
| **Depends on** | [SOL-ORCA-001](./SOL-ORCA-001-orca-api-bridge.md) (contract Orca `POST /api/planner-tasks`, field `callback_url`) |

---

## 1. Tóm tắt vấn đề & mục tiêu

> ⚠️ **Xác thực với Orca thật (2026-08-10):** `PlannerResultCollector`, `PlannerCallbackPublisher`, và bất kỳ hook nào phát callback tới `callback_url` **hoàn toàn không tồn tại** trong Orca hiện tại — `grep -rn "orca-callback\|orca_callback\|callback_url"` trên toàn bộ `/opt/repos/orca` cho **0 kết quả**. Đây là tính năng phải xây mới 100% từ đầu, không phải một pattern có sẵn cần sửa lỗi kỹ thuật. Phần này không đổi so với khảo sát gốc — xem §9 cuối file.

Sau khi AI agent hoàn tất (hoặc bị chặn) trong Orca, kết quả cần đến được `planner-service` (`vnp-workplace`) — service điều phối chính thay thế `plan-svc`/`temporal-worker` của `vnp-planner` theo quyết định kiến trúc 2026-08-10. CR gốc đề xuất 2 pattern: push (callback) và pull (poll). SOL này khoá:
1. **Payload callback** (Orca → `planner-service`) — không đổi so với thiết kế gốc, vì Change 1–3 (Orca-side) giữ nguyên.
2. **Route nhận + auth phía `planner-service`**, đi qua `api-gateway` reverse-proxy (thay vì một HTTP server riêng của `plan-svc` như bản gốc).
3. **Điểm còn để ngỏ, không tự quyết:** cơ chế "chờ/nhận" kết quả ở phía service điều phối task (trước đây là `client.SignalWorkflow` unblock `OrcaTaskDispatchWorkflow` trong `temporal-worker`) — `planner-service` không có Temporal, và `temporal-worker`/CR-ORCA-002 không nằm trong phạm vi re-scope đợt này. Xem §2 và §3.5.

## 2. Không còn "lỗi kỹ thuật cần sửa" — thay bằng Open Decision (Temporal)

Bản CR-ORCA-004 **gốc** (trước re-scope) viết:

```go
func DispatchToOrca(ctx context.Context, input DispatchToOrcaInput) (*DispatchToOrcaResult, error) {
    // ...
    if input.UseCallback {
        var result OrcaCallbackPayload
        workflow.GetSignalChannel(ctx, "orca.task.completed").Receive(ctx, &result) // ❌ SAI (Activity không có workflow.Context)
        return mapSignalToResult(result), nil
    }
}
```

Đây từng là lỗi kỹ thuật (`workflow.GetSignalChannel` chỉ hợp lệ trong Workflow code, không phải Activity) mà bản SOL-ORCA-004 trước re-scope đã sửa bằng `client.SignalWorkflow` gọi từ `plan-svc`'s HTTP handler tới `OrcaTaskDispatchWorkflow` (`temporal-worker`).

**Sau re-scope 2026-08-10, vấn đề này không còn là một "lỗi kỹ thuật cần sửa" mà là một khoảng trống kiến trúc chưa có quyết định:**

- `planner-service` (`vnp-workplace`) đi theo convention Go chuẩn của monorepo — Echo + Postgres + Redis + `asynq` (`backend/specs/tdd/v1/00-go-conventions.md`, `01-project-structure.md`) — **không có Temporal worker nào** trong `vnp-workplace`.
- `CR-ORCA-002` (Dispatcher, sở hữu `temporal-worker`, `OrcaTaskDispatchWorkflow`) **không** nằm trong phạm vi re-scope 2026-08-10 — theo tài liệu gốc của nó, `temporal-worker` vẫn còn ở `vnp-planner` (xem SOL-ORCA-002, không đổi trong đợt này).
- Việc `planner-service` (nhận callback, ở `vnp-workplace`) cần "đánh thức" một workflow đang chờ ở một service/repo **khác** (`temporal-worker` của `vnp-planner`) là một vấn đề tích hợp **cross-repo** — hiện **chưa có kênh giao tiếp nào** giữa `vnp-workplace` và `vnp-planner`'s `temporal-worker` được xác nhận trong tài liệu hiện có.

**Không tự bịa ra một API/kênh signal cụ thể ở đây.** SOL này chỉ khoá phần đã chắc chắn (route nhận, auth, cập nhật domain nội bộ `planner-service`) và tách phần chưa chốt ra một interface trung gian (`ResultNotifier`, §3.5) để không khoá cứng thiết kế `OrcaCallbackHandler` vào một cơ chế chưa quyết định.

## 3. Kiến trúc giải pháp

### 3.1 Luồng end-to-end

> **Xác thực với Orca thật:** không có bước nào trong sơ đồ dưới đây tồn tại phía Orca hôm nay. Vị trí tích hợp tự nhiên nhất trong code thật là bên trong `TaskAgentExecutor.executeTask()`'s try/catch (`backend/src/main/task/TaskAgentExecutor.ts:76-106`) — nơi hiện tại chỉ làm `taskService.update(taskId, {status:'review'|'blocked'})` + 1 activity comment. `PlannerResultCollector.collect()` (git status/log/test) và `PlannerCallbackPublisher.publish()` cần được thêm mới vào đúng điểm này, có điều kiện (chỉ chạy khi task có `labels` chứa `planner:*` hoặc field đánh dấu nguồn gốc từ planner — field này cũng chưa tồn tại, xem SOL-ORCA-003 §9 pt.1). Phần này **không đổi** so với thiết kế trước re-scope — chỉ đích đến của bước POST cuối cùng thay đổi.

```
Orca TaskAgentExecutor.executeTask() — try/catch thật (TaskAgentExecutor.ts:76-106)
   │  [ĐIỂM TÍCH HỢP ĐỀ XUẤT — chưa có code nào ở đây hôm nay]
   ▼
PlannerResultCollector.collect()      ← git status/log/test trong worktree (Orca-side, TS) [MỚI HOÀN TOÀN, không đổi]
   │
   ▼
PlannerCallbackPublisher.publish(callback_url, result)   ← POST callback_url (Orca-side, TS) [MỚI HOÀN TOÀN, không đổi]
   │
   ▼ HTTP POST /api/v1/planner/orca-callback  (qua api-gateway reverse-proxy — §3.4, ĐỔI so với bản gốc)
api-gateway ServiceProxy.Forward("planner")   ← route ngoài group user-auth, forward nguyên header Authorization
   │
   ▼  reverse-proxy, path KHÔNG rewrite (httputil.NewSingleHostReverseProxy chỉ đổi Host/Scheme)
planner-service: POST /api/v1/planner/orca-callback  (route nội bộ cùng path, KHÔNG dùng tiền tố /internal/v1/...)
OrcaCallbackHandler.Handle()
   ├─ 1. Verify Authorization: Bearer ORCA_PLANNER_API_SECRET (tự validate trong handler — không qua middleware JWT/PAT)
   ├─ 2. Idempotency check (planner_task_id + status đã xử lý chưa) — khuyến nghị giữ, xem §6
   ├─ 3. Update planner task domain trong planner-service (repository nội bộ)
   ├─ 4. Trigger dependency resolution (nội bộ planner-service)
   ├─ 5. ResultNotifier.NotifyTaskCompleted(...)  ← Open Decision, xem §3.5 — KHÔNG phải Temporal Signal
   └─ 6. Trả 200 ngay (không chờ bước 4/5 xử lý xong — fire-and-forget, khớp Orca không retry callback)
```

### 3.2 Callback Payload (Orca → `planner-service`)

**Không đổi so với thiết kế gốc** — Change 1–3 (Orca-side) giữ nguyên hoàn toàn, chỉ đích đến (`callback_url`) trỏ vào endpoint public của `planner-service` sau `api-gateway` thay vì `plan-svc`.

**Request `POST {callback_url}`** (`callback_url` = giá trị `callback_url` mà bên gửi task cho Orca cấu hình lúc submit — xem CR-ORCA-001, và SOL-ORCA-002 nếu/khi `temporal-worker` tiếp tục là bên submit theo Option A §3.5):

```json
{
  "orca_task_id": "orca-task-uuid-001",
  "planner_task_id": "TASK-SAGENT-004",
  "planner_job_id": "JOB-execute_task-abc123",
  "success": true,
  "files_created": ["backend/services/scan-agent/internal/scanner/nmap_runner.go"],
  "files_modified": [],
  "commit_hash": "abc123def",
  "test_output": "--- PASS: TestNmapRunner_Scan (0.12s)",
  "error_message": null,
  "agent_output": "...last 4000 chars..."
}
```

Header bắt buộc: `Authorization: Bearer <ORCA_PLANNER_API_SECRET>` (cùng secret 2 chiều — Orca xác thực khi nhận task, `planner-service` xác thực khi nhận callback), `X-Orca-Source: orca-planner-callback`.

**Response mong đợi từ `planner-service`:** `200 { "status": "ok" }`. Orca timeout sau `ORCA_PLANNER_CALLBACK_TIMEOUT_MS` (mặc định 10000ms — Orca-side config, xem CR-ORCA-006) — nếu không nhận được `200` trong thời gian đó, Orca log lỗi nhưng **không retry callback** (`PlannerCallbackPublisher.publish` không có retry loop — xem TASK-ORCA-004-15). **Poll fallback vẫn cần tồn tại** ở phía service điều phối task để không phụ thuộc 100% vào callback — nhưng cơ chế poll cụ thể tuỳ vào Option A/B ở §3.5 (chưa chốt), không giả định như bản gốc (`OrcaTaskDispatchWorkflow` poll 5 phút).

### 3.3 Route nhận phía `planner-service` — `OrcaCallbackHandler`

**File:** `backend/services/planner-service/internal/presentation/http/handler/orca_callback_handler.go` (theo layout chuẩn `backend/specs/tdd/v1/01-project-structure.md` §Per-Service Structure — `internal/presentation/http/handler/`, framework Echo như các service khác của `vnp-workplace`, KHÔNG dùng `net/http`/`gorilla/mux`/`internal/interface/http` như convention cũ của `plan-svc`/`vnp-planner`).

> Thiết kế chi tiết layer domain/application đầy đủ của `planner-service` (task repository, dependency resolver) **không** thuộc phạm vi SOL này — SOL này đặc tả **chữ ký/hành vi bắt buộc** của `OrcaCallbackHandler` để tương thích với contract Orca (§3.2) và với `api-gateway` (§3.4).

```go
// backend/services/planner-service/internal/presentation/http/handler/orca_callback_handler.go
package handler

import (
	"github.com/labstack/echo/v4"
)

// OrcaCallbackPayload mirrors the JSON body Orca posts to callback_url (§3.2).
// Field order/names must not change independently of the Orca-side contract —
// see TASK-ORCA-004-15 (Orca-side collector/publisher, unchanged by this CR).
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

// OrcaCallbackHandler receives Orca's result push at POST /api/v1/planner/orca-callback
// (routed here unchanged via api-gateway reverse-proxy, §3.4) and updates planner-service's
// own task domain. It does NOT block on downstream notification — see ResultNotifier (§3.5),
// matching Orca's no-retry callback behaviour (§3.2).
type OrcaCallbackHandler struct {
	APISecret          string
	TaskRepo           TaskRepository     // internal planner-service port, out of scope here
	DependencyResolver DependencyResolver // internal planner-service port, out of scope here
	ResultNotifier      ResultNotifier     // §3.5 — Open Decision, no-op/log-only until chốt
}

// Handle implements POST /api/v1/planner/orca-callback.
//
// IMPORTANT — auth: this route is registered at api-gateway OUTSIDE the user-auth group
// (same group as /api/v1/auth/*, §3.4) because Orca is an external caller authenticating
// with a shared secret, not a WKP user with a PAT/JWT. Do NOT wrap this route with the
// gateway's standard AuthMiddleware — the handler validates ORCA_PLANNER_API_SECRET itself.
func (h *OrcaCallbackHandler) Handle(c echo.Context) error {
	auth := c.Request().Header.Get("Authorization")
	if auth != "Bearer "+h.APISecret {
		return echo.NewHTTPError(401, "unauthorized")
	}

	var payload OrcaCallbackPayload
	if err := c.Bind(&payload); err != nil {
		return echo.NewHTTPError(400, "invalid body")
	}
	if payload.PlannerTaskID == "" {
		return echo.NewHTTPError(400, "planner_task_id required")
	}

	ctx := c.Request().Context()

	// 1. Update planner task status — domain của planner-service, KHÔNG phải KGP
	//    (Knowledge Graph Planner là khái niệm thuộc vnp-planner, không mang sang đây).
	task, err := h.TaskRepo.GetByExternalRef(ctx, payload.PlannerTaskID)
	if err != nil {
		return echo.NewHTTPError(404, "planner task not found")
	}
	if payload.Success {
		task.MarkCompleted(payload.FilesCreated, payload.FilesModified, payload.CommitHash)
	} else {
		task.MarkFailed(payload.ErrorMessage)
	}
	if err := h.TaskRepo.Update(ctx, task); err != nil {
		return echo.NewHTTPError(500, "update task failed")
	}

	// 2. Trigger dependency resolution (nội bộ planner-service) — best-effort, không chặn response.
	h.DependencyResolver.OnTaskCompleted(ctx, task.ID)

	// 3. Notify waiting callers — §3.5, Open Decision. KHÔNG phải Temporal Signal (không có
	//    Temporal trong vnp-workplace) — implementation cụ thể chưa chốt, xem §3.5.
	h.ResultNotifier.NotifyTaskCompleted(ctx, task.ID, payload)

	return c.JSON(200, map[string]string{"status": "ok"})
}
```

Ghi chú thiết kế quan trọng:
- **Không** để `Handle` chờ xử lý nặng (vd. gọi `knowledge-service`/`skills-service`) rồi mới trả response — Orca có timeout callback ngắn (mặc định 10s); nếu handler block, Orca coi là lỗi callback dù dữ liệu đã tới.
- Idempotency: Orca có thể gọi callback nhiều lần cho cùng `orca_task_id` (retry ở tầng network) — `TaskRepo.Update` nên là idempotent theo trạng thái đích (set lại cùng status không lỗi), tránh phải dựng bảng chống trùng riêng cho task đã terminal.
- **Khác biệt so với bản gốc (`plan-svc`):** bỏ bước "Update KGP via MCP tool" — KGP (Knowledge Graph Planner) là khái niệm thuộc `vnp-planner`, không có xác nhận `planner-service` (`vnp-workplace`) tích hợp MCP tool tương đương; việc cập nhật domain model của `planner-service` gói gọn trong bước 1 (`TaskRepo.Update`), không tách MCP tool call riêng — không bịa thêm chi tiết ngoài phạm vi CR.

### 3.4 Đăng ký reverse-proxy tại `api-gateway`

**Design Decision — Ingress Path** (theo CR-ORCA-004 Change 4): Orca chạy như một **external service** (deploy headless theo CR-ORCA-006), không nằm trong mạng nội bộ của các Go microservices `vnp-workplace`. `callback_url` Orca gọi không thể là địa chỉ chỉ resolve nội bộ (`http://planner-service:3013/...`) trừ khi Orca cùng mạng riêng (không đảm bảo theo CR-ORCA-006) — nên phải expose qua `api-gateway`.

Theo đúng pattern `ServiceProxy.targets` đã có ở `backend/specs/tdd/v1/03-api-gateway.md` §Reverse Proxy (dùng cho `sync`/`knowledge`/`mcp`/`notification`), và tương tự cách MCP Proxy (`SOL-051-mcp-proxy-api.md`) expose ingress cho một external caller (OpenWork Desktop) không phải là user WKP thông thường:

```go
// internal/presentation/http/proxy/reverse_proxy.go [MODIFY, api-gateway]
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

```go
// internal/presentation/http/router.go [MODIFY, api-gateway]
// Đăng ký NGOÀI group user-auth — cùng nhóm với /api/v1/auth/* (no user-auth-required),
// vì Orca xác thực bằng ORCA_PLANNER_API_SECRET riêng (bên trong OrcaCallbackHandler),
// không phải PAT/JWT của WKP user.
e.POST("/api/v1/planner/orca-callback", proxy.Forward("planner"))
```

Các điểm bắt buộc:
- Endpoint public, đi qua gateway: **`POST /api/v1/planner/orca-callback`**. `httputil.NewSingleHostReverseProxy` không rewrite path (chỉ đổi `Host`/`Scheme`), nên `planner-service` phải tự đăng ký route **cùng path** trên router nội bộ của nó — **không** dùng tiền tố `/internal/v1/...` (tiền tố đó quy ước cho API chỉ gọi service-to-service qua mạng nội bộ, ví dụ `/internal/v1/agent-gateway/dispatch-worker-group`; ở đây caller là Orca — external service — nên endpoint nằm trong không gian route public `/api/v1/...` dù implement trong `planner-service`).
- **Auth không đi qua middleware JWT/PAT chuẩn của gateway** — route nằm ngoài group yêu cầu user-auth, `OrcaCallbackHandler` tự validate `Authorization: Bearer <ORCA_PLANNER_API_SECRET>` bên trong handler (§3.3).
- Gateway reverse-proxy **phải forward nguyên header `Authorization`** (không strip/override như nó làm với `X-User-ID`/`X-Workspace-ID` cho các route đã qua auth ở `Forward()` — xem `03-api-gateway.md` §Reverse Proxy) để `planner-service` tự verify được secret của Orca.
- `PlannerServiceURL` — biến môi trường mới cho `api-gateway` (`PLANNER_SERVICE_URL=http://planner-service:3013`), theo đúng convention `SYNC_SERVICE_URL`/`KNOWLEDGE_SERVICE_URL`/... đã có ở `03-api-gateway.md` §Environment Variables.

### 3.5 Cơ chế "chờ/nhận" kết quả phía điều phối task — Open Decision (thay Temporal Signal)

Bản gốc dùng `temporalClient.SignalWorkflow(ctx, plannerJobID, "", "orca.task.completed", result)` để unblock một `DispatchToOrca`/`OrcaTaskDispatchWorkflow` đang chờ trong `temporal-worker` (`vnp-planner`).

**Không có Temporal trong `vnp-workplace`** (§2). `CR-ORCA-002` (Dispatcher, sở hữu `temporal-worker`) **không** nằm trong phạm vi re-scope 2026-08-10 và vẫn ở `vnp-planner` theo tài liệu gốc của nó. Việc `planner-service` (nhận callback) cần đánh thức một workflow ở một repo/service khác là vấn đề tích hợp **cross-repo** chưa có kênh giao tiếp nào được xác nhận trong tài liệu hiện có.

`OrcaCallbackHandler` gọi qua một interface trung gian để không khoá cứng vào cơ chế chưa chốt:

```go
// backend/services/planner-service/internal/application/port/result_notifier.go
package port

import "context"

type ResultNotifier interface {
	NotifyTaskCompleted(ctx context.Context, taskID string, payload handler.OrcaCallbackPayload) error
}
```

Hai lựa chọn implement (**chưa chốt — cần quyết định khi CR-ORCA-002 được review lại, ngoài phạm vi CR-004/SOL này**):

| Option | Mô tả | Trade-off |
|---|---|---|
| **A — Poll only (an toàn, không cần đổi CR-ORCA-002)** | `planner-service` chỉ cập nhật status trong domain của chính nó (§3.3 bước 1); `temporal-worker` (CR-ORCA-002, `vnp-planner`, không đổi) tiếp tục dùng **Mode 1 (Poll)** — ticker gọi `GET /api/v1/planner/orca-sessions/{id}` (CR-ORCA-005 Change 4) qua `api-gateway` thay vì gọi thẳng Orca | Đơn giản, không cần thay đổi gì ở `vnp-planner`; độ trễ tối đa = chu kỳ poll (không còn "callback-driven, hiệu quả hơn" như Mode 2 gốc) |
| **B — Cross-repo webhook** | `planner-service` gọi ngược một endpoint webhook do `vnp-planner`/`temporal-worker` expose để relay signal | Giữ độ trễ thấp của Mode 2 gốc, nhưng cần CR-ORCA-002 (hoặc CR mới) định nghĩa endpoint đó — hiện **chưa tồn tại** trong tài liệu |

**Khuyến nghị của CR-ORCA-004 (giữ nguyên trong SOL này):** dùng **Option A (Poll)** làm baseline — implement `ResultNotifier` **no-op/log-only** ở D1 (bản đầu tiên), để trạng thái luôn nhất quán qua REST API dashboard (CR-ORCA-005 Change 4) mà không phụ thuộc cơ chế cross-repo chưa chốt. Option B chỉ nên làm khi CR-ORCA-002 được đưa vào phạm vi re-scope hoặc có CR riêng định nghĩa cross-repo signal contract.

```go
// backend/services/planner-service/internal/infrastructure/notifier/noop_result_notifier.go
// NoopResultNotifier — Option A baseline. Ghi log, không gọi ra ngoài. Thay bằng implementation
// thật khi Option A/B được chốt (xem bảng trên) — KHÔNG tự thêm logic gọi cross-repo ở đây
// cho tới khi có quyết định kiến trúc.
type NoopResultNotifier struct{ Logger *zap.Logger }

func (n *NoopResultNotifier) NotifyTaskCompleted(ctx context.Context, taskID string, payload handler.OrcaCallbackPayload) error {
	n.Logger.Info("orca callback processed (no cross-repo notify — Option A baseline)",
		zap.String("task_id", taskID), zap.Bool("success", payload.Success))
	return nil
}
```

## 4. Tích hợp với các CR khác

- **CR-ORCA-001**: `callback_url` là field trong request submit task cho Orca (SOL-ORCA-001 §3.2) — không đổi bởi CR này.
- **CR-ORCA-002**: `temporal-worker`/`OrcaTaskDispatchWorkflow` **không đổi**, vẫn ở `vnp-planner`, ngoài phạm vi re-scope. Nếu Option A (§3.5) được chọn, `temporal-worker` cần đổi nguồn poll sang `GET /api/v1/planner/orca-sessions/{id}` (qua `api-gateway`, CR-ORCA-005) — đây là thay đổi thuộc phạm vi CR-ORCA-002 nếu/khi nó được review lại, **không** thực hiện trong CR-004 này.
- **CR-ORCA-005**: cung cấp endpoint dashboard `GET /api/v1/planner/orca-sessions/{id}` mà Option A cần, và tiêu thụ trạng thái task do `OrcaCallbackHandler` cập nhật.

## 5. Kế hoạch test (phần Go — `planner-service` handler)

```go
TestOrcaCallbackHandler_ValidSecret_UpdatesTaskAndReturns200
TestOrcaCallbackHandler_InvalidSecret_Returns401
TestOrcaCallbackHandler_MissingPlannerTaskID_Returns400
TestOrcaCallbackHandler_InvalidJSONBody_Returns400
TestOrcaCallbackHandler_TaskNotFound_Returns404
TestOrcaCallbackHandler_DoubleCallback_BothReturn200NoError
TestOrcaCallbackHandler_ResultNotifierError_StillReturns200 // fire-and-forget, không chặn response
```

Coverage target: handler layer ≥ 60% (TDD-00 §12.3 — presentation layer). Test dùng `TaskRepository`/`DependencyResolver`/`ResultNotifier` mock/fake qua interface (`httptest`/Echo test context) — không cần Temporal, không cần `api-gateway` thật (test riêng route reverse-proxy ở phía `api-gateway`, xem Verification).

## 6. Rủi ro & giảm thiểu

| Rủi ro | Giảm thiểu |
|---|---|
| Orca không retry callback khi `planner-service` tạm downtime | Cần poll fallback độc lập — nhưng cơ chế cụ thể phụ thuộc Option A/B (§3.5) **chưa chốt**; cho tới khi chốt, rủi ro này **chưa có giảm thiểu đầy đủ** ngoài retry hạ tầng (LB/health check) |
| Cross-repo notify (Option B) chưa có kênh giao tiếp | Dùng Option A (poll, §3.5) làm baseline — không phụ thuộc kênh chưa tồn tại |
| Giả mạo callback (không đúng `ORCA_PLANNER_API_SECRET`) | Auth bắt buộc trong handler (§3.3); secret không log ra plaintext; gateway forward nguyên header `Authorization` không tự ý validate/strip (§3.4) |
| `agent_output`/`test_output` payload lớn gây nghẽn HTTP body | Orca giới hạn 4000–5000 ký tự cuối (theo TASK-ORCA-004-15, không đổi) — `planner-service` handler nên set giới hạn body size phòng vệ thêm (vd. Echo `BodyLimit` middleware, tương tự `MaxBytesReader` ở thiết kế `plan-svc` cũ) |
| Route `/api/v1/planner/orca-callback` bị nhầm là cần user-auth khi thêm middleware toàn cục mới ở `api-gateway` sau này | Ghi rõ trong code comment tại router (§3.4) rằng route này cố ý nằm ngoài group `AuthMiddleware`/`RateLimitMiddleware` chuẩn — cùng nhóm `/api/v1/auth/*` |

## 7. Ước tính công việc

| Component | Task | Giờ |
|---|---|---|
| Orca (ngoài phạm vi Go monorepo) | `PlannerResultCollector`, `PlannerCallbackPublisher`, `TaskService` hook (không đổi — xem TASK-ORCA-004-15) | 9h |
| `planner-service` (Go) | `OrcaCallbackHandler` + domain update + dependency resolution | 5h |
| `planner-service` (Go) | `ResultNotifier` interface (Option A no-op impl) | 1h |
| `api-gateway` (Go) | `ServiceProxy.targets["planner"]` + route ngoài group user-auth + `PlannerServiceURL` config | 2h |
| Both (Go) | Test handler + route reverse-proxy | 5h |
| **Tổng phía `vnp-workplace`** | | **13h** |

> Không còn hạng mục "Temporal signal integration" (3h ở bản gốc) — thay bằng `ResultNotifier` interface (Option A) + config gateway. Option B (cross-repo webhook), nếu được chọn sau này, cần effort riêng ngoài estimate này — chưa chốt nên không ước tính.

## 8. Dependencies

Phụ thuộc CR-ORCA-001 (contract `callback_url`). Là bên nhận callback cho toàn bộ luồng task dispatch — cho tới khi Option A/B ở §3.5 được chốt và implement, `planner-service` chỉ cập nhật domain nội bộ của chính nó khi nhận callback; bất kỳ service điều phối nào cần biết kết quả real-time (thay vì poll REST) phải chờ quyết định đó.

---

## 9. Xác thực với Orca thật (khảo sát ngày 2026-08-10)

> Phần này không đổi so với khảo sát trước re-scope — Orca-side (Change 1–3) không bị ảnh hưởng bởi việc đổi phía nhận từ `plan-svc` sang `planner-service`.

1. **`PlannerResultCollector`, `PlannerCallbackPublisher` — CHƯA tồn tại trong Orca hiện tại** (xác nhận: `grep -rn "orca-callback\|orca_callback\|callback_url" /opt/repos/orca` → 0 kết quả). Đây là yêu cầu bổ sung cho Orca team, không phải sửa lỗi trên code có sẵn.
   - **Vị trí đề xuất bổ sung code thật:** trong try/catch của `TaskAgentExecutor.executeTask()` (`backend/src/main/task/TaskAgentExecutor.ts:76-106`) — cụ thể ngay sau dòng `await this.taskService.update(taskId, { status: 'review' })` (nhánh thành công, dòng 88) và ngay sau dòng `await this.taskService.update(taskId, { status: 'blocked' }).catch(...)` (nhánh lỗi, dòng 98). Cần: (a) một service mới `PlannerResultCollector` chạy `git diff`/`git log -1 --format=%H`/đọc test output trong `worktreePath` đã có sẵn trong `params`, (b) một `PlannerCallbackPublisher.publish(callbackUrl, payload)` — cả hai chỉ nên chạy khi task có đánh dấu nguồn gốc từ planner (field/label này cũng chưa tồn tại — xem SOL-ORCA-003 §9 pt.1).
2. **Sơ đồ §3.1 phản ánh đúng entry point thật** (`TaskAgentExecutor.executeTask()`'s try/catch) — không đổi so với khảo sát gốc, chỉ đích đến POST cuối cùng (đích callback) đã cập nhật sang `planner-service` qua `api-gateway`.
3. **Phần Go (`OrcaCallbackHandler`) đã được viết lại toàn bộ theo re-scope** — không còn `client.SignalWorkflow`/Temporal, xem §2, §3.3, §3.5.
4. **`callback_url` là field của request submit task cho Orca** (CR-ORCA-001) — bản thân endpoint đó cũng chưa tồn tại phía Orca thật (SOL-ORCA-001 §9), nên toàn bộ luồng callback trong SOL này phụ thuộc **2 tầng** công việc mới phía Orca: (a) endpoint nhận task phải hỗ trợ lưu `callback_url`, (b) `TaskAgentExecutor` phải đọc lại `callback_url` đã lưu và POST khi hoàn tất. Rủi ro §6 "Orca không retry callback" nên được hiểu là "chưa có callback nào để retry" cho tới khi Orca team xây xong (a) và (b).
