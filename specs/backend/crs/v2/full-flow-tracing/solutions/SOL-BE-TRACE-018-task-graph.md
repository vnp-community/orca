# SOL-BE-TRACE-018: Task Graph — Backend-Side Tracing Implementation

**CR Ref:** [CR-TRACE-018](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-018-task-graph.md)
**TDD Ref:** TDD-18 (Task Graph Management — [18-task-graph.md](../../../../tdd/v5/18-task-graph.md), §3 `TaskDAGValidator`, §4 `TaskGrantService`, §5 `TaskAIPlanner`, §6 `TaskAgentExecutor`); tham chiếu §F.11 HLD cross-reference trong `00-index.md` ("TaskService — Grant Resolution")
**Date:** 2026-08-02
**Status:** Proposed
**Strategy:** Additive-only — chỉ thêm tracer calls + mở rộng `AgentSpawnOptions` với 1 field optional; không đổi business logic

---

## 1. Phân tích phạm vi (Backend-side only)

Đã Read toàn bộ `TaskDAGValidator.ts`, `TaskGrantService.ts`, `TaskAIPlanner.ts`, `TaskAgentExecutor.ts`, `TaskService.ts` (phần `addEdge`), `task-rpc-handler.ts` — code thật khác pseudo-code CR ở vài điểm quan trọng, đã điều chỉnh cho khớp thực tế:

| File | Hàm/method | Dòng thực tế đã verify | Khác biệt so với CR |
|------|-----------|--------------------------|------------------------|
| `src/shared/trace/tracers.ts` | thêm 4 tracer | — | ❌ Thiếu `taskGraphEdgeFlow`/`taskGraphAiPlanFlow`/`taskGraphGrantFlow`/`taskGraphExecuteFlow` |
| `src/main/task/TaskDAGValidator.ts` | `wouldCreateCycle()` (28-61) | ✅ khớp CR (dòng 28) | **CR mô tả "BFS", code thật là DFS (dùng `stack`, không phải `queue`)** — đã xác nhận qua docstring dòng 25 ("DFS from `to`") và code dùng `stack.pop()`. `detectCycle()` (69) mới thật sự là BFS (`queue.shift()`), nhưng **không được gọi từ `addEdge()`** |
| `src/main/task/TaskService.ts` | `addEdge()` (231-238) | method thật gọi `dagValidator.wouldCreateCycle(fromTaskId, toTaskId, edgeType)` (233) | CR-TRACE-018 pseudo-code gọi `dagValidator.wouldCreateCycle()` trực tiếp trong `task-rpc-handler.ts` handler — thực tế lời gọi nằm bên trong `TaskService.addEdge()`, RPC handler chỉ gọi `taskService.addEdge()` |
| `src/main/task/TaskAIPlanner.ts` | `decompose(taskId, projectId, userId)` (46-80) | relay.call('ai.complete', ...) dòng 54 ✅ khớp CR | Tên hàm thật là `decompose()` không phải `decomposeTask()`; parse JSON nằm trong private helper `parseAIResponse()` (152-170) **nuốt lỗi parse, luôn trả `[]` thay vì throw** — cần patch nhỏ để tracing phân biệt "parse thất bại" vs "AI trả về 0 subtask hợp lệ" |
| `src/main/task/TaskGrantService.ts` | `resolvePermission()` (111-142) | ✅ khớp CR (dòng 111) | Thuật toán thật không có "level" cố định (`owner`/`admin`/...) như CR minh hoạ — nó duyệt `candidates` (task + ancestors) × `grants` rồi chọn permission **cao nhất** qua `TASK_PERMISSION_ORDER`; scope thật là `'everyone'\|'user'\|'team'\|'role'`, không phải `'owner'\|'admin'\|'company'\|'parent-tree'` như CR |
| `src/main/task/TaskAgentExecutor.ts` | `executeTask()` (45-103) | permission check dòng 49, spawn dòng 75-81 | Code thật gọi `this.agentSpawner.spawn({ projectId, userId, command: prompt, workdir: worktreePath, extraEnv })` — field tên `command`/`workdir` khớp `AgentSpawnOptions` hiện tại (không phải `prompt`/`taskId` như TDD-18 §6 pseudo-code — TDD đã lỗi thời, code là nguồn đúng) |
| `src/main/task/task-rpc-handler.ts` | `task.addEdge` (253-262), `task.aiDecompose` (351-359), `task.execute` (375-390) | ✅ khớp gần tuyệt đối CR (254/352/376) | — |

**Ngoài phạm vi (agent-side):** phần Dev Server nhận `agent.exec` (qua `ProfileAwareAgentSpawner` → relay) và `ai.complete` (qua `TaskAIPlanner` → relay) rồi thực thi thật. Backend chỉ trace tới điểm `relay.call()` rời process.

**Phối hợp với SOL-BE-TRACE-002, SOL-BE-TRACE-015 và SOL-BE-TRACE-017 (✅ Known Conflicts đã resolve 2026-08-02, xem `tasks/00-index.md`):**
- `ProfileAwareAgentSpawner.spawn()` được bọc bởi **đúng 1 span canonical**: `Tracers.agentOrchSpawn` (`agentOrch:spawn`, SOL-BE-TRACE-002 §2.2) — đây là điểm hội tụ thật sự "spawn 1 AI agent", không phải `profile:agentSpawnRoute`. `AgentSpawnOptions.traceId` (dòng 29) do SOL-BE-TRACE-002 §2.2 sở hữu — **solution này (018) không thêm lại field đó**, chỉ dùng nó: `TaskAgentExecutor.executeTask()` tự sở hữu span `taskGraph:execute` (§2.5) bao trọn permission-check + AI-planning + lời gọi `spawn()`, rồi forward `span.id` làm `traceId` để `agentOrch:spawn` **resume** đúng id đó (CR-TRACE-000 §3.1). Lưu ý: nhánh Task Graph này KHÔNG đi qua `profile:agentSpawnRoute` (span đó chỉ tồn tại ở `project-rpc-handler.ts` cho nhánh `project.agentSpawn` RPC trực tiếp, xem SOL-BE-TRACE-015 §2.7) — `taskGraph:execute` resume THẲNG vào `agentOrch:spawn`.
- Áp dụng **cùng pattern `parentTraceId`** như SOL-BE-TRACE-017 (field nghiệp vụ, không phải `resume`) cho việc nhóm nhiều sub-flow trên cùng 1 task (vd. `addEdge` rồi `execute` liên tiếp) — nhưng đây là **optional**, không bắt buộc như Workflow (Task Graph không có cấu trúc wave/DAG-dispatch multi-span-per-execution như Workflow).

---

## 2. Full Implementation

### 2.1 `src/shared/trace/tracers.ts` — thêm 4 tracer

```typescript
import { createTracer } from './index'

export const Tracers = {
  // ...existing entries unchanged (profile:*, aiProvider:*, workflow:* từ SOL-015/016/017)...

  // ── Task Graph (CR-TRACE-018) ─────────────────────────────────────────────────
  /** BL-TG-01: add dependency edge — cycle detection là phần đáng trace nhất */
  taskGraphEdgeFlow:    createTracer('taskGraph:addEdge'),
  /** BL-TG-02: AI decompose — tách rõ "AI call chậm" vs "parse JSON lỗi" */
  taskGraphAiPlanFlow:  createTracer('taskGraph:aiPlan'),
  /** BL-TG-03: BFS/multi-level ancestor grant resolution — chạy trên mọi permission check */
  taskGraphGrantFlow:   createTracer('taskGraph:grantResolve'),
  /** BL-TG-04: task prompt → agent execution, span cha có thể resume từ taskGraph:execute nếu gọi lồng */
  taskGraphExecuteFlow: createTracer('taskGraph:execute'),
} as const
```

### 2.2 `src/main/task/TaskService.ts` — BL-TG-01 (`addEdge()`)

```typescript
import { Tracers } from '../../shared/trace/tracers'

async addEdge(fromTaskId: string, toTaskId: string, edgeType: TaskEdgeType): Promise<void> {
  const span = Tracers.taskGraphEdgeFlow.start({ fromTaskId, toTaskId, edgeType })

  // wouldCreateCycle() là DFS (stack-based) — KHÔNG phải BFS như một số flow doc mô tả.
  // Vẫn đáng step() riêng theo CR-TRACE-000 §5 rule 1+3: có thể chậm trên graph lớn (N SELECT
  // tuần tự theo độ sâu DFS) và là điểm rẽ nhánh quan trọng để troubleshoot "vì sao bị reject".
  const wouldCycle = await this.dagValidator.wouldCreateCycle(fromTaskId, toTaskId, edgeType)
  span.step('cycle-check', { wouldCycle })

  if (wouldCycle) {
    span.fail('TASK_DEPENDENCY_CYCLE', { fromTaskId, toTaskId, edgeType })
    throw new Error('TASK_DEPENDENCY_CYCLE')
  }

  await this.pool.withConnection((db) =>
    db.query(
      `INSERT INTO orca_task_edges (from_task_id, to_task_id, edge_type) VALUES (?, ?, ?)`,
      [fromTaskId, toTaskId, edgeType]
    )
  )
  span.ok({ fromTaskId, toTaskId, edgeType })
}
```

> **Lưu ý:** `task-rpc-handler.ts`'s `task.addEdge` handler (dòng 253-262) chỉ gọi `taskService.addEdge(...)` sau khi `requirePermission(grantService, userId, params.fromTaskId, 'edit')` — permission check này dùng chung `taskGraphGrantFlow` (§2.4), KHÔNG lồng vào `taskGraphEdgeFlow` vì đây là 2 sub-flow riêng biệt (permission check là cross-cutting, chạy trước hầu hết mọi RPC method, không riêng `addEdge`).

### 2.3 `src/main/task/TaskAIPlanner.ts` — BL-TG-02

Cần tách `parseAIResponse()` thành phiên bản có "diagnostics" để tracing phân biệt được lỗi parse JSON thật (regex không match, hoặc `JSON.parse` throw) với trường hợp AI trả về mảng rỗng hợp lệ — public behavior của `decompose()` giữ nguyên (vẫn trả `[]` khi parse lỗi), chỉ thêm observability nội bộ.

```typescript
import { Tracers } from '../../shared/trace/tracers'

async decompose(taskId: string, projectId: string, userId: string): Promise<OrcaTask[]> {
  const span = Tracers.taskGraphAiPlanFlow.start({ taskId, projectId, userId })

  try {
    const task = await this.taskService.get(taskId)
    if (!task) { span.fail('TASK_NOT_FOUND', { taskId }); throw new Error(`TASK_NOT_FOUND: ${taskId}`) }

    const prompt = this.buildDecomposePrompt(task)

    const relay = await this.router.getRelayForProject(projectId, userId)
    span.step('ai-call', { method: 'ai.complete', promptLength: prompt.length })
    const response = (await relay.call('ai.complete', {
      prompt, format: 'json', taskId,
      traceId: span.id,   // [NEW] — forward vào relay envelope theo CR-TRACE-000 §3.3 hàng 2
    })) as { content?: string; text?: string } | string

    const { proposals, parseOk } = this.parseAIResponseWithDiagnostics(response)
    span.step('parse-plan', { subtaskCount: proposals.length, parseOk })

    const result = proposals.map((p) => ({
      id: `proposed:${Date.now()}:${Math.random().toString(36).slice(2)}`,
      parentId: taskId, projectId: task.projectId, title: p.title, description: p.description,
      type: p.type ?? 'subtask', status: 'backlog' as const, priority: task.priority, labels: [],
      visibility: task.visibility, progressPercent: 0, estimatedHours: p.estimatedHours,
      createdAt: new Date(), updatedAt: new Date(),
    }))

    span.ok({ subtaskCount: result.length, parseOk })
    return result
  } catch (err) {
    span.fail(err, { taskId })
    throw err
  }
}

// [NEW] — wrapper quanh parseAIResponse() hiện có, KHÔNG đổi hành vi public (vẫn trả [] khi lỗi),
// chỉ expose thêm parseOk để decompose() có thể trace chính xác "AI call chậm" (ai-call step latency)
// tách biệt khỏi "parse JSON lỗi" (parseOk: false) — trước đây 2 nguyên nhân này không phân biệt được
// từ bên ngoài vì parseAIResponse() nuốt mọi lỗi thành mảng rỗng.
private parseAIResponseWithDiagnostics(response: unknown): { proposals: SubtaskProposal[]; parseOk: boolean } {
  try {
    let text = ''
    if (typeof response === 'string') {
      text = response
    } else if (response && typeof response === 'object') {
      const r = response as Record<string, unknown>
      text = (r['content'] ?? r['text'] ?? '') as string
    }
    const match = text.match(/\[[\s\S]*\]/)
    if (!match) return { proposals: [], parseOk: false }
    return { proposals: JSON.parse(match[0]) as SubtaskProposal[], parseOk: true }
  } catch {
    console.warn('[TaskAIPlanner] Failed to parse AI response')
    return { proposals: [], parseOk: false }
  }
}

// parseAIResponse() (152-170) giữ nguyên KHÔNG đổi — chỉ dùng nội bộ bởi các call site khác nếu có.
```

### 2.4 `src/main/task/TaskGrantService.ts` — BL-TG-03

`resolvePermission()` chạy trên **mọi** RPC cần permission check (`requirePermission()` helper trong `task-rpc-handler.ts` gọi nó ở hầu hết mọi method) — theo CR-TRACE-000 §5, đây là hot path nên chỉ trace tối thiểu: 1 `ok()`/`fail()` cuối cùng, không `step()` cho từng candidate/grant trong vòng lặp lồng nhau (sẽ tạo noise cực lớn — N candidates × M grants mỗi lần gọi).

```typescript
import { Tracers } from '../../shared/trace/tracers'

async resolvePermission(userId: string, taskId: string): Promise<TaskPermission | null> {
  const span = Tracers.taskGraphGrantFlow.start({ userId, taskId })
  const now = Date.now()

  const ancestorIds = await this.getAncestorIds(taskId)
  const candidates: Array<{ taskId: string; requireApplyTree: boolean }> = [
    { taskId, requireApplyTree: false },
    ...ancestorIds.map(id => ({ taskId: id, requireApplyTree: true })),
  ]

  let highest: TaskPermission | null = null
  let matchedScope: string | undefined
  let matchedDirect: boolean | undefined

  for (const { taskId: tid, requireApplyTree } of candidates) {
    const grants = await this.getGrantsForTask(tid, requireApplyTree)
    for (const grant of grants) {
      if (grant.expiresAt && grant.expiresAt.getTime() < now) continue
      const matches = await this.matchesScope(userId, grant)
      if (!matches) continue

      const level = TASK_PERMISSION_ORDER[grant.permission] ?? 0
      const currentLevel = highest ? (TASK_PERMISSION_ORDER[highest] ?? 0) : -1
      if (level > currentLevel) {
        highest = grant.permission
        matchedScope = grant.scope
        matchedDirect = tid === taskId
      }
    }
  }

  if (highest === null) {
    span.fail('NO_GRANT_FOUND', { userId, taskId, ancestorCount: ancestorIds.length })
    return null
  }

  // Chỉ 1 step tổng kết — không step per-candidate/per-grant (tránh noise hot path)
  span.step('grant-match', { matchedScope, direct: matchedDirect })
  span.ok({ permission: highest, matchedScope, direct: matchedDirect })
  return highest
}
```

### 2.5 `src/main/task/TaskAgentExecutor.ts` — BL-TG-04

```typescript
import { Tracers } from '../../shared/trace/tracers'

async executeTask(params: ExecuteTaskParams): Promise<void> {
  const { taskId, projectId, userId, worktreePath } = params
  const span = Tracers.taskGraphExecuteFlow.start({ taskId, projectId, userId })

  try {
    // 1. Permission check — dùng chung taskGraph:grantResolve (đã có tracer riêng ở §2.4);
    // chỉ step() ở đây để đo latency TỔNG của bước này trong bối cảnh executeTask(), không lồng span.
    const perm = await this.grantService.resolvePermission(userId, taskId)
    const permLevel = perm ? (TASK_PERMISSION_ORDER[perm] ?? 0) : 0
    span.step('permission-check', { permLevel, permission: perm ?? 'none' })
    if (permLevel < MIN_EXECUTE_LEVEL) {
      span.fail('TASK_PERMISSION_DENIED', { userId, taskId })
      throw new Error(`TASK_PERMISSION_DENIED: user "${userId}" needs "execute" or "manage" to run agent on task "${taskId}"`)
    }

    // 2. Load task — SELECT đơn, gộp vào ok()/fail() (CR-TRACE-000 §5)
    const task = await this.taskService.get(taskId)
    if (!task) { span.fail('TASK_NOT_FOUND', { taskId }); throw new Error(`TASK_NOT_FOUND: ${taskId}`) }

    // 3. Build prompt — in-memory transform, KHÔNG step riêng
    const prompt = this.buildPrompt(task)

    await this.taskService.update(taskId, { status: 'in_progress' })
    await this.taskService.addComment(taskId, userId, `Agent execution started by ${userId}`, 'activity')

    // 4. Spawn agent — network hop, forward span.id làm traceId cho ProfileAwareAgentSpawner.spawn()
    // để agentOrch:spawn (SOL-BE-TRACE-002 §2.2 — span canonical duy nhất bọc spawn(), theo Known
    // Conflicts resolution 2026-08-02) RESUME cùng id thay vì tạo span độc lập. Lưu ý: nhánh này
    // KHÔNG đi qua profile:agentSpawnRoute (span đó chỉ tồn tại ở project-rpc-handler.ts cho nhánh
    // project.agentSpawn RPC trực tiếp, xem SOL-BE-TRACE-015 §2.7) — taskGraph:execute resume THẲNG
    // vào agentOrch:spawn. Cho phép TracePanel hiển thị taskGraph:execute + agentOrch:spawn +
    // relay:agentCall như MỘT chuỗi liên tục (khác với parentTraceId của Workflow — ở đây là true
    // resume vì đây thực sự là 1 lời gọi hàm nội bộ nối tiếp, không phải N span độc lập song song).
    try {
      span.step('agent-spawn', { worktreePath, hasAccountOverride: !!params.accountId })
      await this.agentSpawner.spawn({
        projectId, userId, command: prompt, workdir: worktreePath,
        extraEnv: params.accountId ? { ORCA_ACCOUNT_ID: params.accountId } : undefined,
        traceId: span.id,   // [NEW] — xem AgentSpawnOptions mở rộng bên dưới
      })

      await this.taskService.update(taskId, { status: 'review' })
      await this.taskService.addComment(taskId, userId, `Agent execution completed successfully`, 'activity')
      span.ok({ status: 'review' })
    } catch (err) {
      const errMsg = err instanceof Error ? err.message : String(err)
      await this.taskService.update(taskId, { status: 'blocked' }).catch(() => {})
      await this.taskService.addComment(taskId, userId, `Agent execution failed: ${errMsg}`, 'activity').catch(() => {})
      span.fail(err, { status: 'blocked' })
      throw err
    }
  } catch (err) {
    // permission/task-not-found đã fail() ở trên; nhánh này chỉ re-throw, không double-fail.
    throw err
  }
}
```

### 2.6 `src/main/project/ProfileAwareAgentSpawner.ts` — ✅ resolved 2026-08-02, KHÔNG còn thuộc phạm vi patch của solution này

Bản đề xuất ban đầu ở đây (tự thêm `AgentSpawnOptions.traceId` + patch `spawn()` dùng `Tracers.profileAgentSpawnFlow.start(fields, { id: traceId })`) xung đột trực tiếp với SOL-BE-TRACE-002 — cả 2 solution độc lập viết lại toàn bộ thân `spawn()`. Theo Known Conflicts resolution (`tasks/00-index.md`, 2026-08-02):

- `AgentSpawnOptions.traceId?: string` **đã tồn tại** — do SOL-BE-TRACE-002 §2.2 thêm (không phải solution này). Solution này không patch lại field đó.
- `spawn()` chỉ có 1 span canonical: `Tracers.agentOrchSpawn` (SOL-BE-TRACE-002 §2.2), đã tự resume theo `options.traceId` sẵn (`options.traceId ? { id: options.traceId } : undefined`). Không có `Tracers.profileAgentSpawnFlow`/`taskGraphExecuteFlow` nào mở bên trong `spawn()` cả.
- Việc "resume" mà solution này cần chỉ đơn giản là: `TaskAgentExecutor.executeTask()` (§2.5) forward `span.id` (của chính span `taskGraph:execute`) vào `agentSpawner.spawn({ ..., traceId: span.id })` — không cần patch gì thêm ở `ProfileAwareAgentSpawner.ts`.

### 2.7 `src/main/task/task-rpc-handler.ts` — forward `traceId`

```typescript
const EdgeParam = z.object({
  fromTaskId: z.string().min(1),
  toTaskId: z.string().min(1),
  edgeType: z.enum(['depends_on', 'blocks', 'relates_to', 'duplicates']),
  traceId: z.string().optional(),   // [NEW]
})

const AiDecomposeParam = z.object({
  taskId: z.string().min(1),
  projectId: z.string().min(1),
  traceId: z.string().optional(),   // [NEW]
})

const ExecuteParam = z.object({
  taskId: z.string().min(1),
  projectId: z.string().min(1),
  worktreePath: z.string().min(1),
  accountId: z.string().optional(),
  traceId: z.string().optional(),   // [NEW]
})
```

> **Lưu ý:** `taskService.addEdge()`, `aiPlanner.decompose()`, và `executor.executeTask()` hiện chưa nhận `resume`/`traceId` param ở entry point (chỉ tự `start()` span mới bên trong). Để FE-initiated `traceId` (nếu Browser gửi kèm RPC request) resume đúng, cần overload 3 hàm này thêm tham số cuối `traceId?: string` tương tự pattern đã áp dụng ở `AIProviderService.writeCredentialToDevServer()` (SOL-BE-TRACE-016 §2.5). Solution này ưu tiên implement nội bộ `parentTraceId`/`resume` giữa các layer backend trước (đã xong ở §2.3/§2.5/§2.6); việc forward từ RPC layer là bổ sung nhỏ, liệt kê trong Acceptance Criteria nhưng không chặn phần lõi.

---

## 3. Test Plan (Vitest)

| Test file | Test case | Verify |
|-----------|-----------|--------|
| `src/main/task/__tests__/TaskService.test.ts` | `addEdge() hợp lệ → span.step('cycle-check', { wouldCycle: false }) rồi ok()` | |
| | `addEdge() tạo cycle → span.fail('TASK_DEPENDENCY_CYCLE')`, KHÔNG có INSERT chạy | |
| `src/main/task/__tests__/TaskAIPlanner.test.ts` | `decompose() AI trả JSON hợp lệ → step('parse-plan', { parseOk: true, subtaskCount: N })` | |
| | `decompose() AI trả text không có mảng JSON → step('parse-plan', { parseOk: false, subtaskCount: 0 })`, vẫn `ok()` (không throw) | phân biệt với lỗi network |
| | `decompose() relay.call ném lỗi (network) → span.fail(err)`, KHÔNG có step parse-plan | đảm bảo phân biệt "AI call chậm/lỗi" khỏi "parse lỗi" |
| | `decompose() forward traceId: span.id vào relay.call() params` | |
| `src/main/task/__tests__/TaskGrantService.test.ts` | `resolvePermission() direct grant match → step('grant-match', { direct: true })` | |
| | `resolvePermission() chỉ ancestor grant (applyTree) match → step('grant-match', { direct: false })` | |
| | `resolvePermission() không match gì → span.fail('NO_GRANT_FOUND')`, trả `null` | |
| | `resolvePermission() KHÔNG emit step() nào ngoài 1 'grant-match' duy nhất` (chống noise) | assert đúng 1 `level: 'step'` event |
| `src/main/task/__tests__/TaskAgentExecutor.test.ts` | `executeTask() permission denied → span.fail('TASK_PERMISSION_DENIED')`, KHÔNG có step agent-spawn | |
| | `executeTask() spawn thành công → span.ok({ status: 'review' })` | |
| | `executeTask() spawn throw → span.fail(err, { status: 'blocked' })` | |
| | `executeTask() forward traceId: span.id vào agentSpawner.spawn() options` | mock spawner, assert `options.traceId === span.id` |
| `src/main/project/__tests__/ProfileAwareAgentSpawner.test.ts` | `spawn({ traceId }) → agentOrch:spawn span.id === traceId` (resume — test này đã thuộc phạm vi SOL-BE-TRACE-002 §3, liệt kê lại ở đây để xác nhận Task Graph path cũng đi qua đúng cơ chế) | |
| | `spawn() không có traceId → span.id là random mới (không resume)` | đảm bảo default path (SOL-BE-TRACE-002) không bị phá vỡ |

**Test Targets:**

| Module | Target tests |
|--------|--------------|
| TaskService (addEdge) tracing | ≥ 2 |
| TaskAIPlanner tracing | ≥ 4 |
| TaskGrantService tracing | ≥ 4 |
| TaskAgentExecutor tracing | ≥ 4 |
| ProfileAwareAgentSpawner resume (cross-file với SOL-002, không phải SOL-015 — xem Known Conflicts resolution) | ≥ 2 |
| **Total** | **≥ 16** |

---

## 4. Acceptance Criteria

- [ ] `Tracers.taskGraphEdgeFlow` phân biệt `TASK_DEPENDENCY_CYCLE` (fail) và edge hợp lệ (ok), kèm `step('cycle-check', { wouldCycle })` đo latency DFS — tài liệu ghi rõ đây là **DFS**, không phải BFS (sửa nhầm lẫn thuật ngữ trong CR-TRACE-018 gốc)
- [ ] `Tracers.taskGraphAiPlanFlow` tách rõ 2 nguyên nhân: `step('ai-call')` đo latency relay call riêng biệt với `step('parse-plan', { parseOk })` — 2 field độc lập, không gộp; đạt được nhờ `parseAIResponseWithDiagnostics()` wrapper không đổi hành vi public của `parseAIResponse()`
- [ ] `Tracers.taskGraphGrantFlow` chỉ emit **đúng 1** `step()` tổng kết mỗi lần gọi (không step per-candidate/per-grant) để tránh noise trên hot path chạy mọi RPC — nhưng luôn `fail()` rõ ràng khi `NO_GRANT_FOUND`, không im lặng nuốt lỗi ủy quyền
- [ ] `Tracers.taskGraphExecuteFlow` bao phủ trọn `executeTask()`: `permission-check` → `agent-spawn` → `ok`/`fail`, field `status` cuối cùng khớp giá trị thực tế ghi vào `orca_tasks.status`
- [ ] `AgentSpawnOptions` (`ProfileAwareAgentSpawner.ts:29`) đã có sẵn optional field `traceId?: string` — do SOL-BE-TRACE-002 §2.2 thêm, **solution này không patch lại**; khi `TaskAgentExecutor` forward `span.id` vào đó, `agentOrch:spawn` (span canonical duy nhất bọc `spawn()`) **resume** đúng id đó thay vì tạo span mới; khi không có, hành vi giữ nguyên (span độc lập)
- [ ] Không có `span.step()` riêng cho `buildPrompt()` (in-memory transform) hay các SELECT/UPDATE đơn dòng trong `TaskService`/`TaskAgentExecutor` — tuân thủ CR-TRACE-000 §5
- [ ] TracePanel hiển thị 4 tracer mới dưới namespace `taskGraph:*`, không đụng `workflow:*` (SOL-BE-TRACE-017), `profile:*` (SOL-BE-TRACE-015), hay `agentOrch:*` (SOL-BE-TRACE-002) dù cùng khái niệm "agent spawn qua relay"
- [ ] Khi `TaskAgentExecutor.executeTask()` gọi `agentSpawner.spawn()` với `traceId: span.id`, TracePanel hiển thị `taskGraph:execute` → `agentOrch:spawn` (cùng `id`, do resume — KHÔNG qua `profile:agentSpawnRoute`, span đó chỉ tồn tại ở nhánh `project.agentSpawn` RPC trực tiếp, xem SOL-BE-TRACE-015 §2.7) → `relay:agentCall` như 1 chuỗi liên tục — khác với mô hình `parentTraceId` (nhóm nhiều span độc lập) dùng ở SOL-BE-TRACE-017, vì đây là 1 lời gọi hàm nội bộ nối tiếp thật sự, không phải N step song song
