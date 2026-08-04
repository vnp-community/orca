# TASK-BE-002.3: Propagate `traceId` qua 2 caller thật của `spawn()`

**Phase:** 1
**SOL Ref:** [SOL-BE-TRACE-002](../solutions/SOL-BE-TRACE-002-agent-orchestration.md)
**CR Ref:** [CR-TRACE-002](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-002-agent-orchestration.md)
**Prerequisite:** Phase 0 (TASK-BE-000) + TASK-BE-002.2
**Status:** ✅ Done (2026-08-04) — Added `traceId?: string` to `AgentSpawnParam` (`project-rpc-handler.ts`, already spreads `params` into `spawn()` so no handler change needed), `ExecuteTaskParams` (`TaskAgentExecutor.ts`, forwarded into the `spawn()` call), and `ExecuteParam` (`task-rpc-handler.ts`, forwarded into `executor.executeTask()`). One drift: neither file imports the shared `OptionalString` helper the task doc used — both already use bare `z.string().optional()` for every other optional field, so `traceId` follows that same local convention instead of introducing a new import style. typecheck clean; 31/31 tests pass across `project-rpc.test.ts`, `TaskAgentExecutor.test.ts`, `task-rpc.test.ts`; detect_changes (staged) confirms LOW risk, only expected symbols touched.

---

## Trước khi sửa (bắt buộc theo CLAUDE.md)

```bash
codegraph explore "project.agentSpawn"
codegraph explore "TaskAgentExecutor.executeTask"
codegraph explore "task.execute"
```

Cả 3 là symbol đã tồn tại (MODIFY case, chỉ thêm field `traceId` vào schema + forward qua spread, không đổi logic). Chạy:

```
gitnexus_impact({ target: "TaskAgentExecutor.executeTask", direction: "upstream" })
```

`TaskAgentExecutor.executeTask` là symbol đáng chú ý nhất trong nhóm 3 vì nó cũng được `TASK-BE-018.5` sửa tiếp sau này — đọc kỹ báo cáo blast radius trước khi sửa. Nếu risk HIGH/CRITICAL, dừng lại và xác nhận với người dùng trước khi tiếp tục.

## Mô tả

Grep trực tiếp (`grep -rn "ProfileAwareAgentSpawner"`) xác nhận có đúng 2 RPC entry point thật gọi vào `spawn()`: `project.agentSpawn` (`project-rpc-handler.ts`) và `task.execute` → `TaskAgentExecutor.executeTask()` (`task-rpc-handler.ts`/`TaskAgentExecutor.ts`) — đây là phát hiện bổ sung so với CR-TRACE-002 gốc (CR chỉ xác nhận `spawn()` tồn tại, không truy ngược caller). Task này thêm field `traceId` vào 3 schema/interface liên quan và forward xuyên suốt, KHÔNG tạo span riêng ở layer trung gian (`TaskAgentExecutor` thuộc domain Task Graph, CR-TRACE-018 — chỉ propagate field ở đây để tránh double-instrument cùng 1 request).

**Phối hợp với `TASK-BE-015.4` (✅ Known Conflicts đã resolve 2026-08-02):** `TASK-BE-015.4` sẽ patch tiếp `project-rpc-handler.ts` sau task này để bọc `assertAccess(...)` bằng span `profile:agentSpawnRoute` và đổi dòng `return agentSpawner.spawn({ ...params, userId })` thành forward `traceId: routeSpan.id`. Task này (002.3) chỉ cần đảm bảo schema có field `traceId` và handler forward `params` nguyên vẹn qua spread — không xung đột merge với `TASK-BE-015.4` miễn `TASK-BE-015.4` chạy sau.

## File: `src/main/project/project-rpc-handler.ts` [MODIFY]

`project.agentSpawn` — thêm field `traceId` vào schema, handler không đổi (options đã forward nguyên vẹn qua spread):

```typescript
// AgentSpawnParam (schema hiện có) — thêm field:
const AgentSpawnParam = z.object({
  projectId: z.string().min(1),
  command: z.string().min(1),
  extraEnv: z.record(z.string(), z.string()).optional(),
  workdir: z.string().optional(),
  traceId: OptionalString, // [NEW CR-TRACE-002]
})

// Handler không đổi — options (bao gồm traceId) đã forward nguyên vẹn qua spread:
defineMethod({
  name: 'project.agentSpawn',
  params: AgentSpawnParam,
  handler: async (params, ctx) => {
    const userId = ctx.userId
    if (!userId) throw new Error('UNAUTHENTICATED')
    if (!agentSpawner) throw new Error('AGENT_SPAWNER_NOT_AVAILABLE')
    await projectService.assertAccess(params.projectId, userId)
    return agentSpawner.spawn({ ...params, userId }) // traceId đã nằm trong params, spread qua nguyên vẹn
  }
}),
```

## File: `src/main/task/TaskAgentExecutor.ts` [MODIFY]

Chỉ propagate field, KHÔNG tạo span riêng (xem SOL-BE-TRACE-002 §1.5 — `executeTask()` thuộc domain Task Graph, CR-TRACE-018 sẽ instrument riêng sau):

```typescript
export interface ExecuteTaskParams {
  taskId: string
  projectId: string
  userId: string
  worktreePath: string
  accountId?: string
  /** [NEW CR-TRACE-002] forwarded to ProfileAwareAgentSpawner.spawn(), không dùng ở layer này */
  traceId?: string
}

// Trong executeTask(), tại bước 5 (Spawn agent) — chỉ thêm 1 field:
await this.agentSpawner.spawn({
  projectId,
  userId,
  command: prompt,
  workdir: worktreePath,
  extraEnv: params.accountId ? { ORCA_ACCOUNT_ID: params.accountId } : undefined,
  traceId: params.traceId, // [NEW] — span agentOrch:spawn resume nếu caller (task.execute) gửi kèm
})
```

## File: `src/main/task/task-rpc-handler.ts` [MODIFY]

`task.execute` — thêm `traceId` vào `ExecuteParam` schema và forward vào `executor.executeTask()`:

```typescript
const ExecuteParam = z.object({
  taskId: z.string().min(1),
  projectId: z.string().min(1),
  worktreePath: z.string().min(1),
  accountId: OptionalString,
  traceId: OptionalString, // [NEW CR-TRACE-002]
})

defineMethod({
  name: 'task.execute',
  params: ExecuteParam,
  handler: async (params, ctx) => {
    const userId = ctx.userId ?? ''
    await executor.executeTask({
      taskId: params.taskId,
      projectId: params.projectId,
      userId,
      worktreePath: params.worktreePath,
      accountId: params.accountId,
      traceId: params.traceId, // [NEW]
    })
    return { started: true }
  },
}),
```

## Verification

```bash
pnpm tsc --noEmit
pnpm test --run src/main/project/__tests__/project-rpc-handler.test.ts
pnpm test --run src/main/task/__tests__/TaskAgentExecutor.test.ts
pnpm test --run src/main/task/__tests__/task-rpc-handler.test.ts
```

```bash
# Sau khi sửa xong, trước khi coi task DONE:
gitnexus_detect_changes()
```

Xác nhận chỉ các symbol/flow dự kiến bị ảnh hưởng — nếu detect_changes báo thêm symbol ngoài phạm vi task này, điều tra trước khi tiếp tục.

## Acceptance Criteria

- [ ] `project.agentSpawn` schema (`AgentSpawnParam`) có field `traceId?: string`, forward nguyên vẹn vào `agentSpawner.spawn()`
- [ ] `TaskAgentExecutor.ExecuteTaskParams` có field `traceId?: string`, forward vào `agentSpawner.spawn()` mà KHÔNG tạo tracer/span mới trong `TaskAgentExecutor` (verify bằng test: không có `createTracer`/`Tracers.*` nào được gọi trong file này)
- [ ] `task.execute` schema (`ExecuteParam`) có field `traceId?: string`, forward vào `executor.executeTask()`
- [ ] Cả 2 caller thật (`project.agentSpawn` và `task.execute` → `TaskAgentExecutor.executeTask`) đều propagate `traceId` xuyên tới `spawn()` mà không tạo span trùng lặp ở layer trung gian
