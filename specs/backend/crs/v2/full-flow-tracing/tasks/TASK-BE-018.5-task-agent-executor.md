# TASK-BE-018.5: Instrument `TaskAgentExecutor.executeTask()` — resume vào `agentOrch:spawn` (BL-TG-04)

**Phase:** 3
**SOL Ref:** [SOL-BE-TRACE-018](../solutions/SOL-BE-TRACE-018-task-graph.md)
**CR Ref:** [CR-TRACE-018](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-018-task-graph.md)
**Prerequisite:** Phase 0 (TASK-BE-000) + TASK-BE-018.1, TASK-BE-018.3, TASK-BE-018.4, **TASK-BE-002.2** (mới thêm — span `agentOrch:spawn` + resume logic mà task này resume vào phải tồn tại trước)
**Status:** ✅ Done (2026-08-04) — Implemented per spec: `TaskAgentExecutor.executeTask()` wrapped with `Tracers.taskGraphExecuteFlow` (fresh root span, no resume-in), `step('permission-check')`, `step('agent-spawn')`, `span.ok({status:'review'})` / `span.fail(err, {status:'blocked'})` on the spawn sub-block, `span.fail('TASK_PERMISSION_DENIED'|'TASK_NOT_FOUND', ...)` on the early-exit branches with an outer catch that only re-throws (no double-fail). `agentSpawner.spawn({..., traceId: span.id})` forwards this span's id. DRIFT (see TASK-BE-018.4 status for detail): at edit time, `AgentSpawnOptions` on `ProfileAwareAgentSpawner.ts` had temporarily lost its `traceId` field due to unrelated concurrent activity — used a local type-widening cast (`AgentSpawnOptions & { traceId?: string }`) in `TaskAgentExecutor.ts` only, so the call typechecks today and wires up automatically once TASK-BE-002.2's field/resume logic is present, without touching the forbidden file. `ExecuteParam` in `task-rpc-handler.ts` already had `traceId` from TASK-BE-002.3 (re-added when that file's edits were re-applied after the same concurrent revert); did not duplicate. `pnpm tsc --noEmit` clean; `TaskAgentExecutor.test.ts` 16/16 pass (10 original + 6 new tracing tests, see TASK-BE-018.6).

---

## ✅ Known Conflicts với TASK-BE-002.3 — đã giải quyết 2026-08-02 (xem `tasks/00-index.md`)

`TASK-BE-002.3-agent-spawn-callers-propagate-traceid.md` (Phase 1) patch `TaskAgentExecutor.ts` để thêm `traceId?: string` vào `ExecuteTaskParams` và **acceptance criteria gốc của task đó** yêu cầu "KHÔNG tạo tracer/span mới trong `TaskAgentExecutor`" — điều này ban đầu mâu thuẫn trực tiếp với SOL-BE-TRACE-018, vốn yêu cầu `executeTask()` tự sở hữu span `taskGraph:execute`.

Quyết định resolve: **cả hai đúng, ở 2 layer khác nhau.** `TASK-BE-002.3`'s "no span" constraint áp dụng cho layer field-propagation cơ bản (schema `traceId`, không tạo tracer) — vẫn đúng như đã viết, không đổi. Task này (018.5) BỔ SUNG thêm: `TaskAgentExecutor.executeTask()` **DOES get its own span** (`taskGraph:execute`) bao trọn permission-check + AI-planning + lời gọi `spawn()` — đây không phải là điều `TASK-BE-002.3` cấm (task đó chỉ nói không tạo span TRÙNG LẶP cho cùng công việc `spawn()` đã làm; `taskGraph:execute` là span MỚI cho công việc riêng của Task Graph, không phải bản sao của `agentOrch:spawn`). `span.id` của `taskGraph:execute` forward làm `traceId` khi gọi `agentSpawner.spawn()`, để `agentOrch:spawn` (span canonical duy nhất bọc `spawn()`, TASK-BE-002.2) **resume** đúng id đó — KHÔNG qua `profile:agentSpawnRoute` (span đó chỉ tồn tại ở nhánh `project.agentSpawn` RPC trực tiếp, xem TASK-BE-015.4).

## Trước khi sửa (bắt buộc theo CLAUDE.md)

```bash
codegraph explore "TaskAgentExecutor.executeTask"
```

Symbol đã tồn tại (MODIFY case) — đã bị `TASK-BE-002.3` patch trước đó (field `traceId` trong `ExecuteTaskParams`). Chạy:

```
gitnexus_impact({ target: "TaskAgentExecutor.executeTask", direction: "upstream" })
```

**Lưu ý phối hợp:** task này PHẢI chạy sau `TASK-BE-002.2` (span `agentOrch:spawn` phải tồn tại để resume vào). Báo cáo blast radius trước khi sửa — xác nhận span `taskGraph:execute` mới là BỔ SUNG hợp lệ, không trùng lặp với `agentOrch:spawn`. Nếu risk HIGH/CRITICAL, dừng lại và xác nhận với người dùng trước khi tiếp tục.

## Mô tả

Bọc `TaskAgentExecutor.executeTask()` bằng span `taskGraph:execute`. Đây là ví dụ "resume thật" (khác `parentTraceId` của Workflow, xem `TASK-BE-018.4`): `taskGraph:execute` → `agentOrch:spawn` (cùng `id`, do resume) → `relay:agentCall` hiển thị như MỘT chuỗi liên tục trên TracePanel, vì đây thực sự là 1 lời gọi hàm nội bộ nối tiếp, không phải N span song song.

## File: `src/main/task/TaskAgentExecutor.ts` [MODIFY]

```typescript
import { Tracers } from '../../shared/trace/tracers'

async executeTask(params: ExecuteTaskParams): Promise<void> {
  const { taskId, projectId, userId, worktreePath } = params
  const span = Tracers.taskGraphExecuteFlow.start({ taskId, projectId, userId })

  try {
    // 1. Permission check — dùng chung taskGraph:grantResolve (TASK-BE-018.3, đã có tracer riêng);
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
    // để agentOrch:spawn (TASK-BE-002.2 — span canonical duy nhất bọc spawn(), theo Known Conflicts
    // resolution 2026-08-02) RESUME cùng id thay vì tạo span độc lập. Lưu ý: nhánh này KHÔNG đi qua
    // profile:agentSpawnRoute (span đó chỉ tồn tại ở project-rpc-handler.ts cho nhánh project.agentSpawn
    // RPC trực tiếp, xem TASK-BE-015.4) — taskGraph:execute resume THẲNG vào agentOrch:spawn.
    // Cho phép TracePanel hiển thị taskGraph:execute + agentOrch:spawn + relay:agentCall như MỘT
    // chuỗi liên tục (khác với parentTraceId của Workflow — ở đây là true resume vì đây thực sự là
    // 1 lời gọi hàm nội bộ nối tiếp, không phải N span độc lập song song).
    try {
      span.step('agent-spawn', { worktreePath, hasAccountOverride: !!params.accountId })
      await this.agentSpawner.spawn({
        projectId, userId, command: prompt, workdir: worktreePath,
        extraEnv: params.accountId ? { ORCA_ACCOUNT_ID: params.accountId } : undefined,
        traceId: span.id,   // [NEW] — xem AgentSpawnOptions mở rộng ở TASK-BE-018.4
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

## File: `src/main/task/task-rpc-handler.ts` [MODIFY — chỉ phần schema]

```typescript
const ExecuteParam = z.object({
  taskId: z.string().min(1),
  projectId: z.string().min(1),
  worktreePath: z.string().min(1),
  accountId: z.string().optional(),
  traceId: z.string().optional(),   // [NEW] — có thể đã tồn tại nếu TASK-BE-002.3 chạy trước;
                                     // không thêm trùng field nếu đã có.
})
```

> **Lưu ý:** `executor.executeTask()` hiện chưa nhận `resume`/`traceId` param ở entry point riêng từ RPC layer (chỉ tự `start()` span mới bên trong, KHÔNG dùng `params.traceId` của caller làm resume — theo đúng thiết kế SOL-BE-TRACE-018, `taskGraph:execute` luôn là span gốc mới cho mỗi lần `task.execute` được gọi). Việc forward `traceId` từ RPC layer (nếu FE gửi) vào `executeTask()` là bổ sung nhỏ, không chặn phần lõi.

## Verification

```bash
pnpm tsc --noEmit
pnpm test --run src/main/task/__tests__/TaskAgentExecutor.test.ts
```

```bash
# Sau khi sửa xong, trước khi coi task DONE:
gitnexus_detect_changes()
```

Xác nhận chỉ các symbol/flow dự kiến bị ảnh hưởng — nếu detect_changes báo thêm symbol ngoài phạm vi task này, điều tra trước khi tiếp tục.

## Acceptance Criteria

- [ ] `Tracers.taskGraphExecuteFlow` bao phủ trọn `executeTask()`: `permission-check` → `agent-spawn` → `ok`/`fail`, field `status` cuối cùng khớp giá trị thực tế ghi vào `orca_tasks.status`
- [ ] `executeTask()` gọi `agentSpawner.spawn({ ..., traceId: span.id })` — `taskGraph:execute` và `agentOrch:spawn` chia sẻ CÙNG `id` qua resume (TASK-BE-002.2/TASK-BE-018.4), không phải `parentTraceId`, và KHÔNG đi qua `profile:agentSpawnRoute`
- [ ] Permission denied → `span.fail('TASK_PERMISSION_DENIED')`, KHÔNG có `step('agent-spawn')`
- [ ] Spawn throw → `span.fail(err, { status: 'blocked' })`, task status được cập nhật `blocked` trước khi fail
- [ ] Không có `span.step()` riêng cho `buildPrompt()` (in-memory transform) hay các SELECT/UPDATE đơn dòng
- [ ] Known Conflict với `TASK-BE-002.3` (§ trên) đã resolve: field-propagation cơ bản của `TASK-BE-002.3` không đổi; `taskGraph:execute` là span BỔ SUNG hợp lệ (không trùng lặp `agentOrch:spawn`), forward `span.id` làm resume id
- [ ] `pnpm tsc --noEmit` pass, không lỗi mới
