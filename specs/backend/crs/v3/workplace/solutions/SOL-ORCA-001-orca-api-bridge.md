> ⚠️ **SUPERSEDED (2026-08-10):** Tài liệu task/solution breakdown này được sinh ra từ các CR-ORCA-00x/CR-TASK-00x đã bị viết lại hoặc retired — dựa trên giả định sai rằng `vnp-planner` bị loại bỏ và `vnp-workplace` tự dựng `planner-service` mới. Kiến trúc đúng: xem [`docs/crs/v3/orca/README.md`](../../../../../docs/crs/v3/orca/README.md). Nội dung bên dưới chỉ còn giá trị tham khảo lịch sử — không dùng để implement.
>
> ---
>
# SOL-ORCA-001 — Orca API Bridge (Task Submission Endpoint)

| Field | Value |
|-------|-------|
| **CR ref** | [CR-ORCA-001](../../../../../../docs/crs/v3/orca/CR-ORCA-001-orca-api-bridge.md) |
| **Title** | Orca API Bridge — `/api/planner-tasks` |
| **Service** | Orca HTTP Server (repo `orca`, Electron/TypeScript) |
| **Priority** | P0 |
| **Risk** | high |
| **Status** | 📐 PROPOSED |
| **Phạm vi** | **Ngoài phạm vi Go monorepo `vnp-workplace`.** Tài liệu này mô tả **contract/API** mà phía Orca (Electron/TypeScript) phải cung cấp để `planner-service` (`vnp-workplace`, `:3013` — xem [SOL-ORCA-002](./SOL-ORCA-002-planner-orca-dispatcher.md)) gọi tới. Orca team tự triển khai nội bộ (TaskService, TaskAgentExecutor, SQLite store) theo stack TypeScript của họ — không thiết kế trong tài liệu này. |
| **TDD refs** | TDD-01 (Project Structure — `planner-service` Clean Architecture), TDD-00 §6 (interface/client pattern phía consumer) |
| **Depends on** | — (nền tảng cho CR-ORCA-002..006) |
| **Ghi chú re-scope (2026-08-10)** | Contract Orca-side (§2–§9) **không đổi** — chỉ cập nhật thuật ngữ phía consumer: `vnp-planner`/`temporal-worker` (Go, đã loại khỏi kiến trúc) → `vnp-workplace`/`planner-service` (`:3013`, service Go mới đảm nhiệm vai trò này). Xem `docs/crs/v3/orca/README.md` §Ghi chú Re-scope. |

---

## 1. Tóm tắt vấn đề & mục tiêu

`planner-service` cần một REST endpoint phía Orca để: (1) submit một task cho AI agent thực thi trong worktree cô lập, (2) query trạng thái/­kết quả, (3) hủy task đang chạy. Mục tiêu của SOL này là **đóng băng hợp đồng API** (request/response JSON schema, mã lỗi, auth) mà `planner-service` sẽ code cứng vào `orcaclient.Client` (SOL-ORCA-002) — để hai đội (Go và TypeScript) phát triển song song không cần đồng bộ liên tục.

Vì đây là contract do phía Orca hiện thực, `vnp-workplace` **không kiểm soát** được việc triển khai nội bộ; SOL này chỉ ràng buộc **behaviour quan sát được qua HTTP**.

---

## 2. Kiến trúc mức contract

> ⚠️ **CẢNH BÁO QUAN TRỌNG (xác thực với Orca thật ngày 2026-08-10):** Toàn bộ khối `/api/planner-tasks*` mô tả dưới đây **CHƯA tồn tại** trong code Orca thật (`/opt/repos/orca`) — đây là **đề xuất 100% mới** cho Orca team, không phải hợp đồng của API có sẵn. Khảo sát `backend/src/server/http-server.ts` cho thấy `/api/*` HTTP thật hiện chỉ có 2 route: `GET /api/trace-stream` (SSE, global trace — không phải per-task) và `POST/GET /api/agent-token` (đăng ký dev-server relay agent, không liên quan task execution). `grep -rn "planner-tasks\|planner_task_id\|PlannerTask" /opt/repos/orca` cho **0 kết quả** trong toàn bộ repo. Xem §9 "Xác thực với Orca thật" ở cuối file để biết đầy đủ sai lệch và vị trí code đề xuất bổ sung.

```
planner-service (orcaclient.Client)              Orca HTTP Server :6769 (HTTP) / :6768 (WS)
──────────────────────────────                   ────────────────────────────────────────
POST /api/planner-tasks  [CHƯA TỒN TẠI — đề xuất mới] ─▶ (tương tự TaskService.create(), nhưng
                                    ◀───────────── 201 { orca_task_id, ... }   KHÔNG dùng lại trực
                                                                                tiếp được — xem §9)

GET  /api/planner-tasks/{id}  [CHƯA TỒN TẠI]      ─────▶ (đề xuất mới)
                                    ◀───────────── 200 { status, progress, result }

POST /api/planner-tasks/{id}/cancel [CHƯA TỒN TẠI] ────▶ (đề xuất mới)
                                    ◀───────────── 200 { status: "cancelled" }

GET  /api/trace-stream (SSE)  [CÓ THẬT]           ─────▶ backend/src/server/trace-sse-routes.ts
                                    ◀───────────── text/event-stream — LƯU Ý: đây là stream
                                                    TraceEvent TOÀN CỤC của cả backend (mọi span
                                                    tracer), KHÔNG lọc theo task/orca_task_id, và
                                                    schema payload KHÁC với §3.5 mô tả — xem §9.

GET  /health, /health/ready, /health/metrics [CÓ THẬT] ─▶ backend/src/server/health-endpoint.ts
                                    ◀───────────── 200 { status, uptime, database: {...} } — 3 route
                                                    riêng biệt, không phải 1 route /health duy nhất.
```

`/api/agent-token` (đã có sẵn trong Orca, `backend/src/server/agent-token-routes.ts`) **không** nằm trong đường gọi của `planner-service` — nhưng vai trò thật của nó **khác** với giả định gốc: đây là cơ chế để đăng ký một **dev-server relay agent** (daemon `agent.js` chạy trên máy remote, cung cấp git/fs/pty cho Orca qua WebSocket `/agent`), auth bằng `ORCA_AGENT_API_SECRET` — **không phải** cơ chế "AI coding agent (Claude Code, Codex…) kết nối WebSocket sau khi `TaskAgentExecutor` spawn agent" như mô tả gốc. Trên thực tế, AI CLI agent (`claude`/`codex`/`gemini`) được Orca spawn như **PTY child process cục bộ** (`node-pty.spawn`, xem `docs/hld/backend-server-architecture.md` §6.3) — không tự kết nối WebSocket vào Orca. `planner-service` vẫn không cần gọi `/api/agent-token`.

---

## 3. API Contract đầy đủ

### 3.1 Auth

- Header: `Authorization: Bearer <ORCA_PLANNER_API_SECRET>` trên **mọi** request tới `/api/planner-tasks*`.
- Secret là shared secret cấu hình độc lập (KHÔNG trùng `ORCA_AGENT_API_SECRET` dùng cho WebSocket agent).
- Response khi thiếu/sai secret: `401 { "error": "unauthorized" }`. `planner-service` phải coi lỗi này là **non-retryable** — `orcaclient.ErrUnauthorized` (SOL-ORCA-002 §3.6) không được đưa vào chuỗi self-rescheduling poll (không tự enqueue lại), phải dừng ngay và đánh dấu `orcatask.Tracking` sang `blocked` để cảnh báo vận hành (secret mismatch), xem SOL-ORCA-002 §3.8.
- **Xác thực với Orca thật:** `ORCA_PLANNER_API_SECRET` **không tồn tại** trong Orca hiện tại — biến môi trường thật gần nhất là `ORCA_AGENT_API_SECRET` (guard cho `/api/agent-token` và `/api/trace-stream`, xem `backend/src/server/agent-token-routes.ts:19,36-51`). Pattern auth đề xuất ở trên (fail-secure nếu secret rỗng, so sánh `Bearer <secret>`) khớp đúng với cách `isAuthorized()` trong `agent-token-routes.ts:38-51` đã làm — Orca team có thể copy pattern này khi thêm secret mới, nhưng `ORCA_PLANNER_API_SECRET` bản thân nó phải được Orca team tạo mới, chưa hề cấu hình ở đâu.

### 3.2 `POST /api/planner-tasks`

**Request** (`Content-Type: application/json`):

```json
{
  "planner_task_id": "TASK-SAGENT-004",
  "planner_job_id": "JOB-execute_task-abc123",
  "planner_cr_id": "scan-agent-v0",
  "title": "Implement Nmap Scanner Runner",
  "description": "# TASK-SAGENT-004\n## Mục tiêu\n...\n## Acceptance Criteria\n- [ ] go build",
  "worktree_repo": "git@github.com:vnp-blc/vnp-security.git",
  "worktree_branch": "main",
  "agent_type": "claude",
  "agent_account_id": null,
  "priority": "P1",
  "why_chain": ["Implement Nmap Runner", "Solution: Scanner Architecture", "CR: scan-agent-v0", "Goal: Security Scanning"],
  "anti_patterns": ["Don't use global state", "Don't panic in production code"],
  "required_patterns": ["Wrap errors with fmt.Errorf(\"context: %w\", err)"],
  "acceptance_criteria": ["go build ./...", "go test ./internal/scanner/nmap/... -v"],
  "callback_url": "http://planner-service:3013/api/v1/orca-callback",
  "timeout_hours": 8
}
```

Ràng buộc:
- `planner_task_id`, `title`, `description`, `worktree_repo`, `agent_type`, `priority` — bắt buộc (Orca trả `400` nếu thiếu).
- `priority` ∈ `{P0, P1, P2}`.
- `agent_type` ∈ `{claude, codex, opencode}` — Orca trả `422` nếu agent type chưa cấu hình account.
- `timeout_hours` mặc định `8` nếu không truyền — Orca **không tự hủy** task theo timeout này (đó là trách nhiệm của `planner-service`, tự tính `DeadlineAt` và check thủ công ở đầu mỗi lượt poll asynq — không có cơ chế timer tự động như Temporal, xem SOL-ORCA-002 §3.8); trường này chỉ dùng để Orca ước tính `estimated_completion` khi trả status.

> **Xác thực với Orca thật:** Schema `OrcaTask`/`CreateTaskParams` thật (`agent/src/shared/task-types.ts:39-80`) **không có bất kỳ field nào** trong số: `worktree_repo`, `worktree_branch`, `agent_type`, `agent_account_id`, `why_chain`, `anti_patterns`, `required_patterns`, `acceptance_criteria`, `callback_url`, `timeout_hours`, `planner_cr_id`, `planner_job_id`. Field thật của `OrcaTask` là: `id, projectId?, parentId?, title, description?, type, status, priority(critical|high|medium|low — KHÁC thang P0/P1/P2 ở đây), labels[], visibility, reporterId?, assigneeId?, estimatedHours?, progressPercent, aiContext?, promptTemplate?, dueDate?`. `TaskAgentExecutor.executeTask()` (`backend/src/main/task/TaskAgentExecutor.ts:25-33`) chỉ nhận `{taskId, projectId, userId, worktreePath, accountId?, traceId?}` — **`worktreePath` phải được truyền sẵn (đã tồn tại)**, không có cơ chế `worktree_repo`+`worktree_branch` để Orca tự tạo worktree. Toàn bộ request schema ở trên là đề xuất field mới cho Orca team — không map được vào `OrcaTask` hiện tại nếu không sửa type + `TaskService.create()` + `TaskAgentExecutor.buildPrompt()`. Xem §9.

**Response `201 Created`:**

```json
{
  "orca_task_id": "orca-task-uuid-001",
  "planner_task_id": "TASK-SAGENT-004",
  "status": "pending",
  "trace_stream": "/api/trace-stream"
}
```

**Response lỗi:**

| Status | Khi nào | Hành vi mong đợi ở `planner-service` |
|---|---|---|
| `400` | JSON không hợp lệ / thiếu field bắt buộc | Non-retryable — `DispatchToOrcaUseCase.Execute()` trả lỗi ngay, không tự retry |
| `401` | Sai `ORCA_PLANNER_API_SECRET` | Non-retryable — cảnh báo vận hành (secret mismatch) |
| `409` | `planner_task_id` đã tồn tại (submit trùng) | Coi như thành công — GET lại task hiện có (idempotency, §5) |
| `422` | `agent_type` không khả dụng | Non-retryable — escalate cho người vận hành chọn agent khác |
| `503` | Orca quá tải / không còn capacity worktree | Retryable — `orcaclient.ErrUnavailable` (`IsRetryable() == true`); vì `DispatchToOrcaUseCase` không tự động retry (không còn Activity RetryPolicy), lớp gọi (`plan.DispatchTaskUseCase`/HTTP handler) chịu trách nhiệm retry với backoff nếu cần, xem SOL-ORCA-002 §2 D2 |

### 3.3 `GET /api/planner-tasks/{orca_task_id}`

**Response `200`:**

```json
{
  "orca_task_id": "orca-task-uuid-001",
  "planner_task_id": "TASK-SAGENT-004",
  "planner_job_id": "JOB-execute_task-abc123",
  "status": "in_progress",
  "worktree_path": "/workspace/worktrees/orca-task-uuid-001",
  "agent_session_id": "session-xxx",
  "progress": 65,
  "started_at": "2026-08-09T16:00:00Z",
  "completed_at": null,
  "result": null
}
```

`status` ∈ `{pending, in_progress, review, done, blocked, cancelled}`. Trạng thái **terminal**: `done`, `blocked`, `cancelled` (`planner-service` dừng chuỗi self-rescheduling poll — không tự enqueue lượt tiếp theo — khi gặp 1 trong 3 trạng thái này, xem SOL-ORCA-002 §3.8).

Khi `status ∈ {done, blocked}`, `result` khác `null`:

```json
{
  "success": true,
  "files_created": ["backend/services/scan-agent/internal/scanner/nmap_runner.go"],
  "files_modified": [],
  "commit_hash": "abc123def",
  "test_output": "--- PASS: TestNmapRunner_Scan (0.12s)",
  "error_message": null,
  "agent_output": "...last 4000 chars of agent transcript..."
}
```

`404` nếu `orca_task_id` không tồn tại → non-retryable, `planner-service` coi là lỗi dispatch vĩnh viễn.

> **Xác thực với Orca thật:** `result` với `files_created/files_modified/commit_hash/test_output/agent_output` **không có cơ chế thu thập tương ứng** trong Orca — `TaskAgentExecutor.executeTask()` khi thành công chỉ `taskService.update(taskId, {status:'review'})` + 1 activity comment text "Agent execution completed successfully" (`TaskAgentExecutor.ts:87-94`); khi lỗi, `status:'blocked'` + comment chứa `err.message` (`TaskAgentExecutor.ts:96-104`). Không có bước git diff/git log/test-output collector nào. Đây là tính năng phải xây mới hoàn toàn — xem §9.

### 3.4 `POST /api/planner-tasks/{orca_task_id}/cancel`

**Response `200`:** `{ "orca_task_id": "...", "status": "cancelled" }`. Idempotent — gọi lại trên task đã terminal trả `200` không đổi trạng thái (không lỗi).

### 3.5 `GET /api/trace-stream` (SSE) — [CÓ THẬT, nhưng schema/auth khác mô tả gốc]

- `Content-Type: text/event-stream`. Endpoint **có thật** — `backend/src/server/trace-sse-routes.ts`, đã mount sẵn trong `startHttpServer()`.
- **Sai lệch đã sửa:** mỗi event thật là `data: <JSON.stringify(TraceEvent)>\n\n` với `TraceEvent` là **span tracer nội bộ toàn cục của cả backend** (`shared/trace`, ví dụ `{op, ...fields}` từ bất kỳ `Tracers.*` nào trong toàn hệ thống — không riêng cho task), **không có field `orca_task_id`/`event_type` cố định** như mô tả gốc. Muốn lọc theo task, consumer phải tự parse `TraceEvent` và biết trước tên tracer nào tương ứng với task execution (ví dụ `taskGraph:execute`, xem `docs/crs/v2/full-flow-tracing/CR-TRACE-018-task-graph.md`) — Orca hiện **không tag task ID trực tiếp** vào mọi trace event.
- **Auth thật khác:** `Authorization: Bearer $ORCA_AGENT_API_SECRET` HOẶC header `X-Orca-Admin: 1` HOẶC `X-Orca-Trace-Client: 1` — nếu server không cấu hình `ORCA_AGENT_API_SECRET` thì cho phép truy cập **không cần auth** (`trace-sse-routes.ts:51-70`). Không có khái niệm `ORCA_PLANNER_API_SECRET` riêng cho endpoint này.
- Không có `event:` field riêng — đúng như mô tả gốc, client parse JSON trong `data:`.
- Kết nối là **best-effort, không đảm bảo delivery** — đúng như mô tả gốc, nhưng lý do khác: server gửi `: connected\n\n` khi mở kết nối và `: heartbeat\n\n` (SSE **comment**, không phải `data:` event) mỗi 15s (`trace-sse-routes.ts:111,119-126`), không có event ID/replay buffer.
- Server có thể đóng kết nối bất kỳ lúc nào (Orca restart, idle timeout) — client bắt buộc có reconnect-with-backoff. Đúng với code thật.

### 3.6 `GET /health` — [CÓ THẬT, nhưng có 3 route riêng biệt và response shape khác]

**Sai lệch đã sửa:** Orca thật có **3 route riêng biệt** (`backend/src/server/health-endpoint.ts`), không phải 1 route `/health` duy nhất:

| Route | Mục đích | Response |
|---|---|---|
| `GET /health` | Cached, không query DB live | `200/503 { status, uptime, database: {status,dialect,latencyMs,checkedAt,pool?} \| null }` |
| `GET /health/ready` | Live DB check có timeout (mặc định 5000ms) | Cùng shape, `503` nếu timeout/unhealthy |
| `GET /health/metrics` | Prometheus text format | `orca_db_latency_ms`, `orca_db_pool_total/idle/acquired` |

Không có field `version` trong response — response thật KHÔNG chứa `version`. Không yêu cầu auth — đúng như mô tả gốc. `docker-compose.yml`/`Dockerfile` thật của Orca (`deploy/prod/`) dùng `GET /health/ready` (không phải `/health` trơn) làm healthcheck.

---

## 4. Idempotency & Retry Semantics (ràng buộc cho phía Orca)

`planner-service` có thể gửi lại `POST /api/planner-tasks` với cùng `planner_task_id` khi:
- `DispatchToOrcaUseCase.Execute()` bị gọi lại (retry ở lớp gọi phía trên) nhưng request trước đó đã thành công phía Orca (network partition khi đọc response).
- Retry thủ công sau lỗi `503` (`orcaclient.ErrUnavailable`).

**Yêu cầu bắt buộc với Orca:** endpoint `POST /api/planner-tasks` phải **idempotent theo `planner_task_id`** — nếu đã tồn tại task với cùng `planner_task_id` ở trạng thái non-terminal, trả `409` kèm `orca_task_id` hiện có thay vì tạo task trùng (tránh spawn 2 AI agent cho cùng 1 task, tốn chi phí LLM + xung đột worktree).

```json
// 409 Conflict — trùng planner_task_id
{
  "error": "duplicate_planner_task_id",
  "orca_task_id": "orca-task-uuid-001",
  "status": "in_progress"
}
```

---

## 5. Tích hợp với các CR khác

- **CR-ORCA-002** (`planner-service`): `orcaclient.Client` implement 1:1 theo contract §3; xử lý mã lỗi theo bảng §3.2.
- **CR-ORCA-003**: nội dung `description`/`why_chain`/`anti_patterns`/`required_patterns` gửi trong request chính là input cho `PlannerPromptBuilder` phía Orca — schema field phải khớp tuyệt đối.
- **CR-ORCA-004**: `callback_url` trong request là điểm nối để Orca gọi ngược `POST {callback_url}` khi `status` chuyển sang `done`/`blocked` — payload mô tả tại SOL-ORCA-004.
- **CR-ORCA-005**: `/api/trace-stream` và `/health` là nguồn cho `OrcaTraceProxy`/`OrcaHealthMonitor` trong `signal-svc`.
- **CR-ORCA-006**: endpoint này chỉ khả dụng khi Orca chạy ở headless server mode (`orca serve`) — xem SOL-ORCA-006.

---

## 6. Rủi ro & giảm thiểu (góc nhìn `planner-service`)

| Rủi ro | Giảm thiểu |
|---|---|
| Orca không idempotent thật (race condition khi 2 request đồng thời) | `DispatchToOrcaUseCase` kiểm tra `orcatask.Repository.FindByPlannerTaskID` trước khi gọi `SubmitTask` + luôn GET lại theo `planner_task_id`→`orca_task_id` mapping đã lưu trước khi retry (xem SOL-ORCA-002 §2 D2, §3.7) |
| Breaking change schema phía Orca không báo trước | Contract test (SOL-ORCA-002 §5) chạy bằng JSON schema cố định trong `internal/infrastructure/http/orcaclient` (`vnp-workplace`), fail CI nếu Orca trả field thiếu/sai kiểu khi test integration thủ công |
| SSE không đảm bảo delivery | Không dùng trace-stream làm nguồn trạng thái chính thức (§3.5); trạng thái chính thức luôn từ `GET /api/planner-tasks/{id}` hoặc callback |

---

## 7. Ước tính công việc

Không áp dụng layer Go — công việc thuộc repo `orca` (TypeScript), xem effort estimate gốc trong CR-ORCA-001 (17h). Phía `vnp-workplace`/`planner-service`: 0h (chỉ tiêu thụ contract, effort tính trong SOL-ORCA-002).

## 8. Dependencies

Không phụ thuộc CR nào khác. Là nền tảng (khối xây dựng) cho CR-ORCA-002, 003, 004, 005, 006.

---

## 9. Xác thực với Orca thật (cập nhật — khảo sát ngày 2026-08-10)

Đã đối chiếu toàn bộ SOL này với code thật tại `/opt/repos/orca` (dùng CodeGraph index + grep toàn repo). Tóm tắt các sửa đổi:

1. **`POST/GET /api/planner-tasks`, `POST /api/planner-tasks/{id}/cancel` — CHƯA tồn tại trong Orca hiện tại** (xác nhận qua khảo sát `/opt/repos/orca` ngày hôm nay: `grep -rn "planner-tasks\|planner_task_id\|PlannerTask"` cho 0 kết quả trong toàn bộ repo). Đây là **yêu cầu bổ sung cho Orca team**, không phải API có sẵn — CR-ORCA-001 gốc (`docs/crs/v3/orca/CR-ORCA-001-orca-api-bridge.md:39-44,100-113`) cũng tự thừa nhận đây là code `[NEW]`, nhưng SOL trước đó trình bày như thể đã "đóng băng hợp đồng" của API có thật, gây hiểu nhầm.
   - **Vị trí đề xuất bổ sung code thật:** file mới `backend/src/server/planner-task-routes.ts`, theo đúng pattern của `backend/src/server/agent-token-routes.ts` (factory trả về `apiHandler` compatible với `HttpServerOptions.apiHandler` trong `backend/src/server/http-server.ts:61-67`), mount tương tự cách `createAgentTokenApiHandler()` được truyền vào `options.apiHandler`. **Không thể tái sử dụng trực tiếp `TaskService.create()`/`TaskAgentExecutor.executeTask()`** vì 2 lý do: (a) `OrcaTask`/`CreateTaskParams` (`agent/src/shared/task-types.ts:39-80`) không có các field `worktree_repo/worktree_branch/agent_type/why_chain/anti_patterns/required_patterns/acceptance_criteria/callback_url/timeout_hours` mà request schema §3.2 yêu cầu; (b) `TaskAgentExecutor.executeTask()` (`backend/src/main/task/TaskAgentExecutor.ts:48-107`) đòi hỏi một `userId` có Orca session + `execute`/`manage` grant hợp lệ qua `TaskGrantService` — không có mô hình auth "service-to-service qua API secret" cho flow này.
2. **Port sai:** mô tả gốc dùng `:3000` xuyên suốt — Orca thật mặc định HTTP `:6769`, WebSocket `:6768` (`backend/src/server/index.ts:14-15,46-47`; `deploy/prod/Dockerfile` EXPOSE 6768/6769). Đã sửa trong sơ đồ kiến trúc §2.
3. **`ORCA_PLANNER_API_SECRET` không tồn tại** — biến môi trường thật gần nhất phục vụ mục đích tương tự là `ORCA_AGENT_API_SECRET` (`backend/src/server/agent-token-routes.ts:19,36-51`), dùng cho mục đích khác (đăng ký dev-server agent). Đã ghi chú tại §3.1.
4. **`GET /api/trace-stream` có thật nhưng schema/auth khác** — stream `TraceEvent` toàn cục (`backend/src/shared/trace/index.ts`), không có `orca_task_id`/`event_type` như mô tả gốc; auth qua `ORCA_AGENT_API_SECRET`/`X-Orca-Admin`/`X-Orca-Trace-Client`, không auth nếu secret rỗng (`backend/src/server/trace-sse-routes.ts`). Đã sửa tại §3.5.
5. **`/health` có thật nhưng là 3 route** (`/health`, `/health/ready`, `/health/metrics`), response shape `{status, uptime, database}` không có `version` (`backend/src/server/health-endpoint.ts`). Đã sửa tại §3.6.
6. **`/api/agent-token` mô tả sai vai trò** — không phải "AI agent kết nối WebSocket sau khi TaskAgentExecutor spawn agent". Thật ra là đăng ký **dev-server relay agent** (daemon `agent.js` trên máy remote cung cấp git/fs/pty), auth `ORCA_AGENT_API_SECRET`, token dạng đoán được `agt-<devServerId>-<timestamp>` (`backend/src/server/agent-token-routes.ts`). AI CLI agent (`claude`/`codex`/`gemini`) thật ra được spawn như **PTY child process cục bộ** qua `node-pty.spawn()`, không tự kết nối WebSocket (`docs/hld/backend-server-architecture.md` §6.3). Đã sửa tại §2.
7. **Kết quả task (`files_created`/`files_modified`/`commit_hash`/`test_output`/`agent_output`) không có cơ chế thu thập tương ứng** trong Orca thật — `TaskAgentExecutor.executeTask()` chỉ cập nhật `status` + 1 comment text (`TaskAgentExecutor.ts:87-104`), không có git-diff/test-output collector nào. Đã ghi chú tại §3.3.

**Kết luận:** SOL-ORCA-001 vẫn có giá trị như một **đặc tả yêu cầu (requirements spec)** cho Orca team xây mới, nhưng phải đọc với hiểu biết rằng **không có phần nào của `/api/planner-tasks*` tồn tại hôm nay** — khác với các route thật `/api/trace-stream`, `/api/agent-token`, `/health*` vốn đã hoạt động nhưng phục vụ mục đích khác. Xem tham chiếu chi tiết theo từng dòng ở các ghi chú "Xác thực với Orca thật" rải trong §2, §3.1, §3.2, §3.3, §3.5, §3.6 ở trên.
