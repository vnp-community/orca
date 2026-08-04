# CR-TRACE-018 — Task Graph Flow Tracing

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-TRACE-018 |
| **Tên** | Task Graph — Full-Flow Tracing Instrumentation |
| **Loại** | Observability |
| **Priority** | P3 |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-08-01 |
| **Trạng thái** | Proposed |
| **Phụ thuộc** | CR-TRACE-000 |
| **Tác động** | `docs/flows/logic/task-graph.md`, `src/main/task/TaskService.ts`, `src/main/task/TaskDAGValidator.ts`, `src/main/task/TaskAIPlanner.ts`, `src/main/task/TaskGrantService.ts`, `src/main/task/TaskAgentExecutor.ts`, `src/main/task/task-rpc-handler.ts`, `src/main/project/ProfileAwareAgentSpawner.ts`, `src/shared/trace/tracers.ts` |

---

## 1. Vấn đề

Bốn sub-flow BL-TG-01→04 hiện **không có tracer nào**. Các điểm mù cụ thể theo code đã xác nhận:

1. **BL-TG-01 (CRUD + edge)**: `TaskDAGValidator` có 3 hàm cycle-detection (`wouldCreateCycle` dòng 28, `detectCycle` dòng 69, `getReachable` dòng 103) chạy BFS trên `orca_task_edges` — khi thêm dependency vào graph lớn, không biết BFS mất bao lâu hay bị reject vì cycle thật.
2. **BL-TG-02 (AI decompose)**: `TaskAIPlanner` gọi `relay.call('ai.complete', { prompt, format: 'json' })` (dòng 54) — đây là network hop ra ngoài (tới Dev Server rồi AI provider), có thể chậm hoặc trả JSON không hợp lệ; hiện không tách được "AI call chậm" khỏi "parse JSON lỗi".
3. **BL-TG-03 (grant resolution)**: `TaskGrantService.resolvePermission()` (dòng 111) chạy BFS ancestor-chain 5-level (comment đầu file: "BFS ancestor grant resolution") — đây là permission check chạy **trên mọi API call**, nên nếu chậm sẽ ảnh hưởng toàn bộ Task Graph UI, nhưng hiện không đo được.
4. **BL-TG-04 (agent execution)**: `TaskAgentExecutor.executeTask()` (dòng 45) là chuỗi 6 bước tuần tự (permission check → load task → build prompt → update status → spawn agent → update status kết quả) băng qua `ProfileAwareAgentSpawner.spawn()` (`ProfileAwareAgentSpawner.ts:67`) rồi relay tới Dev Server → PTY. Khi user báo "bấm Run Agent không thấy gì chạy", không biết bị chặn ở permission (`TASK_PERMISSION_DENIED`), ở blocking deps, hay agent spawn thất bại.

## 2. Thành phần & Transport liên quan

| Thành phần (flow doc) | Thực tế trong code | Layer | Transport | CR-TRACE-000 §3.3 row |
|---|---|---|---|---|
| Browser (Task graph UI, Kanban) | — | UI | WebSocket RPC (`task.*` namespace, `task-rpc-handler.ts`) | Row 1 |
| TaskService | `TaskService.ts:119` (class `TaskService`), có `getSubtree()` dòng 217 | Business Logic | in-process | n/a |
| TaskDAGValidator | `TaskDAGValidator.ts:18` — `wouldCreateCycle()` (28), `detectCycle()` (69), `getReachable()` (103) | Business Logic | in-process (BFS thuần, không network) | n/a |
| TaskGraphBuilder (BFS subtree + access filter) | **chưa xác định file cụ thể — cần điều tra thêm khi triển khai** — grep không tìm thấy class riêng tên này; chức năng BFS subtree gần nhất đã xác nhận là `TaskService.getSubtree()` (dòng 217) và `TaskGrantService`'s ancestor BFS (khác mục đích: permission, không phải subtree display) | Business Logic | in-process | n/a |
| TaskAIPlanner | `TaskAIPlanner.ts:30` — gọi `relay.call('ai.complete', { prompt, format: 'json' })` dòng 54 | Business Logic | `relay.call()` (Orca Server ↔ Dev Server, qua provider-resolved connection) | Row 2 |
| TaskGrantService | `TaskGrantService.ts:58` — `resolvePermission()` dòng 111 | Business Logic | in-process (multi-level SELECT tuần tự) | n/a |
| TaskAgentExecutor | `TaskAgentExecutor.ts:32` — `executeTask()` dòng 45 | Business Logic | gọi `ProfileAwareAgentSpawner.spawn()` → relay | n/a (điều phối) → Row 2 ở bước spawn |
| ProfileAwareAgentSpawner | `ProfileAwareAgentSpawner.ts:50` — `spawn()` dòng 67 | Business Logic | `relay.call('agent.exec'/tương tự)` tới Dev Server → PTY | Row 2 |
| Server Database | `orca_tasks`, `orca_task_edges`, `orca_task_grants` | Persistence | in-process | n/a |

## 3. Tracer mới cần thêm vào `tracers.ts`

```typescript
export const Tracers = {
  // ...existing entries unchanged...
  taskGraphEdgeFlow:    createTracer('taskGraph:addEdge'),     // BL-TG-01 (cycle detection là phần dễ chậm nhất)
  taskGraphAiPlanFlow:  createTracer('taskGraph:aiPlan'),      // BL-TG-02
  taskGraphGrantFlow:   createTracer('taskGraph:grantResolve'),// BL-TG-03
  taskGraphExecuteFlow: createTracer('taskGraph:execute'),     // BL-TG-04
}
```

## 4. Instrumentation theo từng sub-flow

### BL-TG-01 — Task Graph CRUD & Structural Management

Chỉ instrument phần "Add Dependency (Edge)" — CREATE TASK/UPDATE STATUS đơn thuần là 1-2 UPDATE/INSERT, không đáng span riêng theo nguyên tắc §5 CR-TRACE-000.

| Bước | span event | fields | File:function |
|------|-----------|--------|----------------|
| Nhận RPC `task.addEdge` | `start` | `fromTaskId`, `toTaskId`, `edgeType` | `task-rpc-handler.ts:254` |
| Chạy cycle detection (BFS — có thể chậm trên graph lớn, đáng trace) | `step('cycle-check')` | `wouldCycle: boolean` | `TaskDAGValidator.ts:28` (`wouldCreateCycle()`) |
| INSERT edge + auto-block downstream | `ok({ blocked: boolean })` / `fail('CYCLE_DETECTED')` | | `TaskService.ts` (nơi gọi `TaskDAGValidator` rồi INSERT) |

```typescript
// task-rpc-handler.ts — trong handler 'task.addEdge'
handler: async (params) => {
  const span = Tracers.taskGraphEdgeFlow.start({
    fromTaskId: params.fromId, toTaskId: params.toId, edgeType: params.edgeType
  })
  const wouldCycle = await dagValidator.wouldCreateCycle(params.fromId, params.toId)
  span.step('cycle-check', { wouldCycle })
  if (wouldCycle) {
    span.fail('CYCLE_DETECTED')
    throw new Error('CYCLE_DETECTED')
  }
  const result = await taskService.addEdge(params)
  span.ok({ blocked: result.blocked })
  return result
}
```

### BL-TG-02 — AI-Assisted Task Planning & Decomposition

| Bước | span event | fields | File:function |
|------|-----------|--------|----------------|
| Nhận RPC `task.aiDecompose` | `start` | `taskId`, `userId` | `task-rpc-handler.ts:352` |
| Load task + context | (gộp vào `ok`/`fail`, chỉ SELECT đơn — không step riêng) | | `TaskAIPlanner.ts` |
| Gọi AI qua relay | `step('ai-call')` | `method: 'ai.complete'`, `promptLength` | `TaskAIPlanner.ts:54` |
| Parse JSON plan | `step('parse-plan')` | `subtaskCount`, `parseOk: boolean` | `TaskAIPlanner.ts` (sau dòng 54) |
| Kết thúc | `ok({ subtaskCount })` / `fail(err)` | | `TaskAIPlanner.ts` |

```typescript
// TaskAIPlanner.ts — decompose()
async decompose(taskId: string, userId: string) {
  const span = Tracers.taskGraphAiPlanFlow.start({ taskId, userId })
  try {
    const prompt = this.buildPrompt(/* ... */)
    span.step('ai-call', { method: 'ai.complete', promptLength: prompt.length })
    const response = await relay.call('ai.complete', { prompt, format: 'json' })
    let plan
    try {
      plan = JSON.parse(response.content)
      span.step('parse-plan', { subtaskCount: plan.subtasks?.length ?? 0, parseOk: true })
    } catch (parseErr) {
      span.step('parse-plan', { parseOk: false })
      throw parseErr
    }
    span.ok({ subtaskCount: plan.subtasks?.length ?? 0 })
    return plan
  } catch (err) {
    span.fail(err, { taskId })
    throw err
  }
}
```

### BL-TG-03 — Task Access Control & Sharing

`resolvePermission()` chạy trên **mọi API call** cần permission check — nếu instrument đầy đủ span cho mỗi call sẽ tạo noise lớn. Áp dụng nguyên tắc §5 CR-TRACE-000: đây là "single in-process lookup" xét riêng lẻ, NHƯNG vì nó là chuỗi 5-level SELECT tuần tự (không phải 1 query đơn) và là điểm rẽ nhánh quan trọng cho troubleshoot "vì sao user bị deny", nó đáng có tracer — khuyến nghị chỉ bật `step()` chi tiết khi `ORCA_TRACE=1`, và luôn `fail()` khi permission bị deny hẳn (không tìm thấy ở level nào) để không im lặng nuốt lỗi ủy quyền.

| Bước | span event | fields | File:function |
|------|-----------|--------|----------------|
| Bắt đầu resolve | `start` | `userId`, `taskId` | `TaskGrantService.ts:111` |
| Match ở level nào (rẽ nhánh quan trọng) | `step('level-match')` | `matchedLevel: 'owner'\|'admin'\|'user'\|'team'\|'company'\|'parent-tree'` | `TaskGrantService.ts:111-...` |
| Không match ở bất kỳ level | `fail('NO_GRANT_FOUND')` | | |
| Match | `ok({ permission, matchedLevel })` | | |

```typescript
// TaskGrantService.ts — resolvePermission()
async resolvePermission(userId: string, taskId: string): Promise<TaskPermission | null> {
  const span = Tracers.taskGraphGrantFlow.start({ userId, taskId })
  // ...owner/admin/user/team/company/parent-tree checks theo thứ tự...
  if (matched) {
    span.step('level-match', { matchedLevel })
    span.ok({ permission: matched, matchedLevel })
    return matched
  }
  span.fail('NO_GRANT_FOUND', { userId, taskId })
  return null
}
```

### BL-TG-04 — Task Prompt → Agent Execution

| Bước | span event | fields | File:function |
|------|-----------|--------|----------------|
| Nhận RPC `task.execute` | `start` | `taskId`, `projectId`, `userId` | `task-rpc-handler.ts:376` |
| Permission check | `step('permission-check')` | `permLevel` | `TaskAgentExecutor.ts:52` (`grantService.resolvePermission`) — có thể `resume` vào cùng `taskGraphGrantFlow` span nếu muốn liên kết, nhưng mặc định là step riêng trong span cha `taskGraphExecuteFlow` |
| Load task | (gộp, SELECT đơn) | | `TaskAgentExecutor.ts:58` |
| Build prompt | (gộp, in-memory transform, không step riêng theo §5 CR-TRACE-000) | | `TaskAgentExecutor.ts:61` (`buildPrompt`) |
| Update status → in_progress | (gộp vào step spawn) | | `TaskAgentExecutor.ts:64` |
| Spawn agent (network hop — Dev Server + PTY) | `step('agent-spawn')` | `worktreePath`, `hasAccountOverride: boolean` | `ProfileAwareAgentSpawner.ts:67` (`spawn()`) |
| Kết quả | `ok({ status: 'review' })` / `fail(err, { status: 'blocked' })` | | `TaskAgentExecutor.ts:76-92` |

```typescript
// TaskAgentExecutor.ts — executeTask()
async executeTask(params: ExecuteTaskParams): Promise<void> {
  const { taskId, projectId, userId, worktreePath } = params
  const span = Tracers.taskGraphExecuteFlow.start({ taskId, projectId, userId })

  const perm = await this.grantService.resolvePermission(userId, taskId)
  const permLevel = perm ? (TASK_PERMISSION_ORDER[perm] ?? 0) : 0
  span.step('permission-check', { permLevel })
  if (permLevel < MIN_EXECUTE_LEVEL) {
    span.fail('TASK_PERMISSION_DENIED', { userId, taskId })
    throw new Error(`TASK_PERMISSION_DENIED: user "${userId}" needs "execute" or "manage"...`)
  }

  const task = await this.taskService.get(taskId)
  if (!task) { span.fail('TASK_NOT_FOUND'); throw new Error(`TASK_NOT_FOUND: ${taskId}`) }

  const prompt = this.buildPrompt(task)
  await this.taskService.update(taskId, { status: 'in_progress' })

  try {
    span.step('agent-spawn', { worktreePath, hasAccountOverride: !!params.accountId })
    await this.agentSpawner.spawn({ projectId, userId, command: prompt, workdir: worktreePath, /* ... */ })
    await this.taskService.update(taskId, { status: 'review' })
    span.ok({ status: 'review' })
  } catch (err) {
    await this.taskService.update(taskId, { status: 'blocked' }).catch(() => {})
    span.fail(err, { status: 'blocked' })
    throw err
  }
}
```

## 5. Lan truyền traceId qua transport của flow này

- **Browser → Orca Server (`task.execute`, `task.aiDecompose`, `task.addEdge` WS RPC)**: theo CR-TRACE-000 §3.3 hàng 1 — `task-rpc-handler.ts` đọc `params.traceId` (nếu FE gửi) và truyền `resume: params.traceId ? { id: params.traceId } : undefined` vào `start()` tương ứng.
- **`TaskAgentExecutor` → `ProfileAwareAgentSpawner.spawn()` → relay**: theo CR-TRACE-000 §3.3 hàng 2 — `spawn()` cần nhận `traceId` (từ `span.id` của `taskGraphExecuteFlow`) và forward vào `relay.call('agent.exec', { ..., traceId })`. Hiện `spawn()` (`ProfileAwareAgentSpawner.ts:67`) không có tham số `traceId` — CR này yêu cầu bổ sung optional field `traceId?: string` vào `AgentSpawnOptions` (interface dòng 29) để truyền xuyên qua.
- **`TaskAIPlanner` → relay `ai.complete`**: tương tự, `traceId: span.id` của `taskGraphAiPlanFlow` được đính vào params envelope của `relay.call('ai.complete', { prompt, format: 'json', traceId })`.
- **Tương tự CR-TRACE-017**: nếu muốn nhóm nhiều sub-flow của cùng 1 task (vd: `addEdge` rồi `execute` liên tiếp trên cùng task), có thể dùng field nghiệp vụ `parentTraceId` giống mô hình đề xuất ở CR-TRACE-017 §4, nhưng đây là optional — CR này không phụ thuộc vào CR-TRACE-017 để giữ tính độc lập.

## Acceptance Criteria

- [ ] `Tracers.taskGraphEdgeFlow` phân biệt được `CYCLE_DETECTED` (fail) và edge hợp lệ được thêm thành công (ok), kèm thời gian BFS cycle-check
- [ ] `Tracers.taskGraphAiPlanFlow` tách rõ latency của "AI call qua relay" vs "parse JSON" — 2 field riêng, không gộp
- [ ] `Tracers.taskGraphGrantFlow` cho biết chính xác `matchedLevel` khi permission được cấp, và `fail()` rõ ràng khi bị từ chối hoàn toàn (không tìm thấy grant ở level nào)
- [ ] `Tracers.taskGraphExecuteFlow` bao phủ trọn `executeTask()`: permission-check → agent-spawn → ok/fail, với field `status` cuối cùng khớp giá trị thực tế ghi vào `orca_tasks.status`
- [ ] `AgentSpawnOptions` (`ProfileAwareAgentSpawner.ts:29`) có thêm optional field `traceId` và `spawn()` forward nó vào relay call
- [ ] Không có `span.step()` riêng cho `buildPrompt()` (in-memory transform) hay các SELECT/UPDATE đơn dòng trong `TaskService` — tuân thủ nguyên tắc §5 CR-TRACE-000
- [ ] TracePanel hiển thị 4 tracer mới dưới namespace `taskGraph:*`, không đụng `workflow:*` (CR-TRACE-017) dù cùng dùng chung khái niệm agent spawn qua relay
