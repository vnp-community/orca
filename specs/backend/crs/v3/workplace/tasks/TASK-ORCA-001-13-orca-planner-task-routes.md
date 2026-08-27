> ⚠️ **SUPERSEDED (2026-08-10):** Tài liệu task/solution breakdown này được sinh ra từ các CR-ORCA-00x/CR-TASK-00x đã bị viết lại hoặc retired — dựa trên giả định sai rằng `vnp-planner` bị loại bỏ và `vnp-workplace` tự dựng `planner-service` mới. Kiến trúc đúng: xem [`docs/crs/v3/orca/README.md`](../../../../../docs/crs/v3/orca/README.md). Nội dung bên dưới chỉ còn giá trị tham khảo lịch sử — không dùng để implement.
>
> ---
>
## Status: 🔲 NOT STARTED

# TASK-ORCA-001-13 — Orca: `POST/GET /api/planner-tasks*` (Task Submission Endpoint)

**Phase:** 0 — Nền tảng (chặn integration thật của toàn bộ CR-ORCA-002/003/004/005)
**Scope:** 🟠 **Orca TypeScript CONTRACT — KHÔNG thực thi trong repo `vnp-workplace`.** Đây là spec bàn giao cho team Orca; code mới cần thêm vào repo `orca` (`/opt/repos/orca`), cụ thể `backend/src/server/planner-task-routes.ts` (file mới).
**Source:** [SOL-ORCA-001 §3, §9](../solutions/SOL-ORCA-001-orca-api-bridge.md#3-api-contract-đầy-đủ)
**Depends On:** — (nền tảng)
**Người thực thi:** Orca team (repo `orca`, không phải AI agent làm việc trong `vnp-workplace`)

---

## Vì sao task này tồn tại trong bộ task của vnp-workplace

`temporal-worker` (TASK-ORCA-002-01/04) code sẵn `shared/pkg/orcaclient` gọi đúng theo contract JSON dưới đây, build/test bằng `httptest.Server`. Endpoint thật **hoàn toàn chưa tồn tại phía Orca** — xác nhận `grep -rn "planner-tasks\|planner_task_id\|PlannerTask" /opt/repos/orca` → 0 kết quả (khảo sát 2026-08-10). Task này là **đặc tả yêu cầu (requirements spec)** để Orca team implement, không phải mô tả code có sẵn.

---

## Vị trí code thật cần thêm (đã khảo sát)

- File mới: `backend/src/server/planner-task-routes.ts`, theo đúng pattern factory của `backend/src/server/agent-token-routes.ts` (`createAgentTokenApiHandler`) — trả về 1 `apiHandler` tương thích `HttpServerOptions.apiHandler` (`backend/src/server/http-server.ts:61-67`).
- Mount trong `backend/src/server/index.ts`, cạnh dòng `apiHandler: createAgentTokenApiHandler(agentWsServer, devServerManager)` — có thể compose nhiều `apiHandler` bằng cách thử từng handler theo thứ tự (return `true` nếu handler đó nhận request, `false` để fallback sang handler tiếp theo), theo đúng contract `apiHandler` hiện tại (trả `boolean`).
- **KHÔNG thể tái sử dụng trực tiếp `TaskService.create()`/`TaskAgentExecutor.executeTask()`:**
  - `OrcaTask`/`CreateTaskParams` (`agent/src/shared/task-types.ts:39-80`) không có field `worktree_repo/worktree_branch/agent_type/why_chain/anti_patterns/required_patterns/acceptance_criteria/callback_url/timeout_hours/planner_cr_id/planner_job_id` mà request schema dưới đây yêu cầu.
  - `TaskAgentExecutor.executeTask()` (`backend/src/main/task/TaskAgentExecutor.ts:25-33,48`) đòi hỏi `userId` có Orca session + `execute`/`manage` grant hợp lệ qua `TaskGrantService` — không có mô hình auth "service-to-service qua API secret" cho luồng này. `POST /api/planner-tasks` cần một code path riêng, xác thực bằng `ORCA_PLANNER_API_SECRET` (không phải session người dùng).
- Auth pattern tham khảo: `isAuthorized()` trong `agent-token-routes.ts:38-51` (fail-secure nếu secret rỗng — **áp dụng nguyên văn pattern này** cho `ORCA_PLANNER_API_SECRET`, secret hoàn toàn mới, KHÔNG trùng `ORCA_AGENT_API_SECRET`).

---

## Acceptance Criteria

- [ ] `POST /api/planner-tasks` — tạo task mới, trả `201` kèm `orca_task_id`
- [ ] `POST /api/planner-tasks` với `planner_task_id` đã tồn tại (non-terminal) → `409` kèm `orca_task_id` hiện có (idempotent theo `planner_task_id` — bắt buộc, xem §4 SOL-ORCA-001)
- [ ] Thiếu field bắt buộc (`planner_task_id`, `title`, `description`, `worktree_repo`, `agent_type`, `priority`) → `400`
- [ ] `agent_type` không thuộc `{claude, codex, opencode}` hoặc chưa cấu hình account → `422`
- [ ] Thiếu/sai `Authorization: Bearer <ORCA_PLANNER_API_SECRET>` → `401`, theo đúng fail-secure pattern của `agent-token-routes.ts`
- [ ] `GET /api/planner-tasks/{orca_task_id}` → `200` với `status`/`progress`/`result` (result chỉ khác `null` khi status terminal); `404` nếu không tồn tại
- [ ] `POST /api/planner-tasks/{orca_task_id}/cancel` → `200`, idempotent (gọi lại trên task terminal không lỗi, không đổi trạng thái)
- [ ] Server log rõ ràng khi `ORCA_PLANNER_API_SECRET` không được cấu hình (endpoint bị vô hiệu hoàn toàn, không fallback bypass — theo đúng "FIX TASK-AWS-001" pattern đã áp dụng ở `agent-token-routes.ts`)

---

## Contract — JSON Schema (khoá cứng, nguồn: SOL-ORCA-001 §3)

**`POST /api/planner-tasks` request:**

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

**Response `201 Created`:**

```json
{ "orca_task_id": "orca-task-uuid-001", "planner_task_id": "TASK-SAGENT-004", "status": "pending", "trace_stream": "/api/trace-stream" }
```

**`GET /api/planner-tasks/{orca_task_id}` response `200`** (`status` ∈ `{pending, in_progress, review, done, blocked, cancelled}`, terminal = `{done, blocked, cancelled}`):

```json
{
  "orca_task_id": "orca-task-uuid-001", "planner_task_id": "TASK-SAGENT-004",
  "planner_job_id": "JOB-execute_task-abc123", "status": "in_progress",
  "worktree_path": "/workspace/worktrees/orca-task-uuid-001", "agent_session_id": "session-xxx",
  "progress": 65, "started_at": "2026-08-09T16:00:00Z", "completed_at": null, "result": null
}
```

`result` (khi terminal): `{success, files_created[], files_modified[], commit_hash, test_output, error_message, agent_output}` — **thu thập kết quả này (git diff/log/test output) là phạm vi của TASK-ORCA-004-15**, KHÔNG phải task này. Task này chỉ cần trả `result: null` cho tới khi TASK-ORCA-004-15 xong (`PlannerResultCollector` set field này khi task chuyển terminal).

---

## Code mẫu tham khảo (mirror style `agent-token-routes.ts`)

```ts
/**
 * Planner Task HTTP Routes (CR-ORCA-001 / SOL-ORCA-001)
 *
 * Exposes REST endpoints so vnp-workplace's temporal-worker can submit AI-agent
 * tasks to Orca and poll/cancel them, independent of any logged-in Orca user
 * session (service-to-service, unlike the interactive Task Graph JSON-RPC API).
 *
 * Endpoints:
 *   POST /api/planner-tasks              — submit a new task, idempotent by planner_task_id
 *   GET  /api/planner-tasks/:id           — poll status/result
 *   POST /api/planner-tasks/:id/cancel    — cancel (idempotent on terminal tasks)
 *
 * Auth: requires ORCA_PLANNER_API_SECRET env var as Bearer token. Fail-secure:
 *       if not configured, ALL requests to these routes are blocked — mirrors
 *       the ORCA_AGENT_API_SECRET pattern in agent-token-routes.ts (no bypass
 *       header fallback for a production-facing service-to-service endpoint).
 *
 * Contract source of truth: backend/specs/crs/v1/orca/solutions/
 *   SOL-ORCA-001-orca-api-bridge.md §3 (vnp-workplace repo)
 */

import type { IncomingMessage, ServerResponse } from 'node:http'
import { randomUUID } from 'node:crypto'
import { createTracer } from '../shared/trace'

const plannerTaskTracer = createTracer('plannerTask:api')

// ─── Types (mirror SOL-ORCA-001 §3.2-3.4 JSON schema exactly) ────────────────

type PlannerTaskStatus = 'pending' | 'in_progress' | 'review' | 'done' | 'blocked' | 'cancelled'
const TERMINAL_STATUSES: ReadonlySet<PlannerTaskStatus> = new Set(['done', 'blocked', 'cancelled'])

type PlannerTaskResult = {
  success: boolean
  files_created: string[]
  files_modified: string[]
  commit_hash: string | null
  test_output: string | null
  error_message: string | null
  agent_output: string | null
}

type PlannerTaskRecord = {
  orcaTaskId: string
  plannerTaskId: string
  plannerJobId?: string
  plannerCrId?: string
  title: string
  description: string
  worktreeRepo: string
  worktreeBranch?: string
  agentType: 'claude' | 'codex' | 'opencode'
  agentAccountId?: string | null
  priority: 'P0' | 'P1' | 'P2'
  callbackUrl?: string
  timeoutHours: number
  status: PlannerTaskStatus
  progress: number
  worktreePath: string | null
  agentSessionId: string | null
  startedAt: string | null
  completedAt: string | null
  result: PlannerTaskResult | null
  createdAt: number
}

// ─── Storage ──────────────────────────────────────────────────────────────────
// TODO(TASK-ORCA-001-13): replace this in-memory Map with the real Orca task
// store (SQLite/MySQL/PostgreSQL/TiDB per ORCA_STORAGE_BACKEND — see
// docs/hld/backend-server-architecture.md §6.5) so tasks survive a restart.
// In-memory is acceptable ONLY for a first local dev/test pass.
const tasksByPlannerTaskId = new Map<string, PlannerTaskRecord>()
const tasksByOrcaTaskId = new Map<string, PlannerTaskRecord>()

const VALID_AGENT_TYPES = new Set(['claude', 'codex', 'opencode'])
const VALID_PRIORITIES = new Set(['P0', 'P1', 'P2'])

// ─── Auth helper — mirrors agent-token-routes.ts:38-51 fail-secure pattern ───
function isAuthorized(req: IncomingMessage): boolean {
  const apiSecret = process.env['ORCA_PLANNER_API_SECRET']?.trim()
  if (!apiSecret) {
    console.error(
      '[SECURITY] ORCA_PLANNER_API_SECRET not configured. ' +
        '/api/planner-tasks* is BLOCKED. Set ORCA_PLANNER_API_SECRET to a ' +
        'strong random secret (distinct from ORCA_AGENT_API_SECRET) to enable it.'
    )
    return false
  }
  const auth = req.headers['authorization'] ?? ''
  return auth === `Bearer ${apiSecret}`
}

function sendJson(res: ServerResponse, status: number, body: unknown): void {
  const json = JSON.stringify(body)
  res.writeHead(status, {
    'Content-Type': 'application/json',
    'Content-Length': Buffer.byteLength(json),
    'Cache-Control': 'no-store'
  })
  res.end(json)
}

function readBody(req: IncomingMessage): Promise<string> {
  return new Promise((resolve, reject) => {
    let data = ''
    req.on('data', (chunk: Buffer) => { data += chunk.toString() })
    req.on('end', () => resolve(data))
    req.on('error', reject)
  })
}

// ─── POST /api/planner-tasks ──────────────────────────────────────────────────

async function handleCreate(req: IncomingMessage, res: ServerResponse): Promise<void> {
  let body: Record<string, unknown>
  try {
    const raw = await readBody(req)
    body = raw.trim() ? (JSON.parse(raw) as Record<string, unknown>) : {}
  } catch {
    sendJson(res, 400, { error: 'bad_request', message: 'Invalid JSON body' })
    return
  }

  const plannerTaskId = body['planner_task_id'] as string | undefined
  const title = body['title'] as string | undefined
  const description = body['description'] as string | undefined
  const worktreeRepo = body['worktree_repo'] as string | undefined
  const agentType = body['agent_type'] as string | undefined
  const priority = body['priority'] as string | undefined

  if (!plannerTaskId || !title || !description || !worktreeRepo || !agentType || !priority) {
    sendJson(res, 400, {
      error: 'bad_request',
      message: 'planner_task_id, title, description, worktree_repo, agent_type, priority are required'
    })
    return
  }
  if (!VALID_PRIORITIES.has(priority)) {
    sendJson(res, 400, { error: 'bad_request', message: 'priority must be one of P0, P1, P2' })
    return
  }
  if (!VALID_AGENT_TYPES.has(agentType)) {
    sendJson(res, 422, { error: 'agent_unavailable', message: `agent_type "${agentType}" is not configured` })
    return
  }

  // Idempotency: SOL-ORCA-001 §4 — same planner_task_id while non-terminal → 409
  // with the existing orca_task_id instead of creating a duplicate agent run.
  const existing = tasksByPlannerTaskId.get(plannerTaskId)
  if (existing && !TERMINAL_STATUSES.has(existing.status)) {
    sendJson(res, 409, {
      error: 'duplicate_planner_task_id',
      orca_task_id: existing.orcaTaskId,
      status: existing.status
    })
    return
  }

  const span = plannerTaskTracer.start({ plannerTaskId, agentType, priority })
  const orcaTaskId = randomUUID()
  const record: PlannerTaskRecord = {
    orcaTaskId,
    plannerTaskId,
    plannerJobId: body['planner_job_id'] as string | undefined,
    plannerCrId: body['planner_cr_id'] as string | undefined,
    title,
    description,
    worktreeRepo,
    worktreeBranch: body['worktree_branch'] as string | undefined,
    agentType: agentType as PlannerTaskRecord['agentType'],
    agentAccountId: (body['agent_account_id'] as string | null | undefined) ?? null,
    priority: priority as PlannerTaskRecord['priority'],
    callbackUrl: body['callback_url'] as string | undefined,
    timeoutHours: Number(body['timeout_hours'] ?? 8),
    status: 'pending',
    progress: 0,
    worktreePath: null,
    agentSessionId: null,
    startedAt: null,
    completedAt: null,
    result: null,
    createdAt: Date.now()
  }
  tasksByPlannerTaskId.set(plannerTaskId, record)
  tasksByOrcaTaskId.set(orcaTaskId, record)

  // TODO(TASK-ORCA-001-13): actually dispatch to TaskAgentExecutor here — this
  // requires solving the worktree-creation gap (SOL-ORCA-003 §3, §9 pt.2:
  // TaskAgentExecutor.executeTask() expects worktreePath to already exist,
  // it does not run `git worktree add` itself) and the prompt-building gap
  // (TASK-ORCA-003-14). Until both land, this endpoint only tracks task state
  // — it does not yet actually run an AI agent.
  span.ok({ orcaTaskId, status: 'pending' })

  sendJson(res, 201, {
    orca_task_id: orcaTaskId,
    planner_task_id: plannerTaskId,
    status: 'pending',
    trace_stream: '/api/trace-stream'
  })
}

// ─── GET /api/planner-tasks/:id ────────────────────────────────────────────────

function handleGet(res: ServerResponse, orcaTaskId: string): void {
  const record = tasksByOrcaTaskId.get(orcaTaskId)
  if (!record) {
    sendJson(res, 404, { error: 'not_found' })
    return
  }
  sendJson(res, 200, {
    orca_task_id: record.orcaTaskId,
    planner_task_id: record.plannerTaskId,
    planner_job_id: record.plannerJobId ?? null,
    status: record.status,
    worktree_path: record.worktreePath,
    agent_session_id: record.agentSessionId,
    progress: record.progress,
    started_at: record.startedAt,
    completed_at: record.completedAt,
    result: record.result
  })
}

// ─── POST /api/planner-tasks/:id/cancel ───────────────────────────────────────

function handleCancel(res: ServerResponse, orcaTaskId: string): void {
  const record = tasksByOrcaTaskId.get(orcaTaskId)
  if (!record) {
    sendJson(res, 404, { error: 'not_found' })
    return
  }
  if (!TERMINAL_STATUSES.has(record.status)) {
    record.status = 'cancelled'
    record.completedAt = new Date().toISOString()
    // TODO(TASK-ORCA-001-13): actually cancel the underlying agent process /
    // TaskAgentExecutor run once dispatch (above TODO) is implemented.
  }
  sendJson(res, 200, { orca_task_id: record.orcaTaskId, status: record.status })
}

// ─── Route matching ────────────────────────────────────────────────────────────

const ID_ROUTE = /^\/api\/planner-tasks\/([^/]+)(\/cancel)?\/?$/

async function handlePlannerTaskRequest(req: IncomingMessage, res: ServerResponse): Promise<void> {
  if (!isAuthorized(req)) {
    sendJson(res, 401, { error: 'unauthorized' })
    return
  }

  const url = req.url ?? ''
  const method = req.method?.toUpperCase() ?? 'GET'

  if (url === '/api/planner-tasks' && method === 'POST') {
    await handleCreate(req, res)
    return
  }

  const match = url.match(ID_ROUTE)
  if (match) {
    const [, orcaTaskId, cancelSuffix] = match
    if (cancelSuffix && method === 'POST') {
      handleCancel(res, orcaTaskId!)
      return
    }
    if (!cancelSuffix && method === 'GET') {
      handleGet(res, orcaTaskId!)
      return
    }
  }

  sendJson(res, 405, { error: 'method_not_allowed' })
}

// ─── Factory — compatible with HttpServerOptions.apiHandler ──────────────────

export function createPlannerTaskApiHandler(): (req: IncomingMessage, res: ServerResponse) => boolean {
  console.log('[PlannerTaskAPI] Route registered: /api/planner-tasks*')

  return (req: IncomingMessage, res: ServerResponse): boolean => {
    const url = req.url ?? ''
    if (!url.startsWith('/api/planner-tasks')) {
      return false // not ours — let the next apiHandler (e.g. agent-token) or static fallback try
    }
    void handlePlannerTaskRequest(req, res).catch((err: Error) => {
      console.error('[PlannerTaskAPI] Unhandled error:', err.message)
      if (!res.headersSent) {
        sendJson(res, 500, { error: 'internal_error' })
      }
    })
    return true
  }
}
```

---

## Ghi chú tích hợp cho các task khác

- `callback_url` lưu trong `PlannerTaskRecord` là điểm nối cho TASK-ORCA-004-15 (`PlannerCallbackPublisher.publish(callbackUrl, ...)`).
- `result: null` cho tới khi TASK-ORCA-004-15 hoàn thành — không tự chế dữ liệu giả cho `files_created`/`commit_hash`.
- Dispatch thật (spawn AI agent) yêu cầu TASK-ORCA-003-14 (prompt builder + worktree automation) — endpoint này ở mức tối thiểu chỉ cần track state đúng schema để `temporal-worker` (đã code sẵn ở TASK-ORCA-002-01/04) build/test thành công; dispatch thật là bước tiếp theo, có thể làm sau trong cùng CR.

---

## Verification (phía Orca team, ngoài phạm vi CI của `vnp-workplace`)

```bash
cd /opt/repos/orca
npm run build   # hoặc lệnh build thật của repo orca — kiểm tra package.json
npm test -- planner-task-routes   # nếu có test runner tương ứng

# Smoke test thủ công
curl -X POST http://localhost:6769/api/planner-tasks \
  -H "Authorization: Bearer $ORCA_PLANNER_API_SECRET" \
  -H "Content-Type: application/json" \
  -d '{"planner_task_id":"TASK-1","title":"t","description":"d","worktree_repo":"git@x","agent_type":"claude","priority":"P1"}'
# Kỳ vọng: 201 { orca_task_id, planner_task_id: "TASK-1", status: "pending", trace_stream: "/api/trace-stream" }
```
