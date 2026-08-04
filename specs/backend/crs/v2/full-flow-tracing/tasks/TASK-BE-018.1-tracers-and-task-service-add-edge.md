# TASK-BE-018.1: Đăng ký 4 tracer `taskGraph:*` và instrument `TaskService.addEdge()` — DFS thật, không phải BFS (BL-TG-01)

**Phase:** 3
**SOL Ref:** [SOL-BE-TRACE-018](../solutions/SOL-BE-TRACE-018-task-graph.md)
**CR Ref:** [CR-TRACE-018](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-018-task-graph.md)
**Prerequisite:** Phase 0 (TASK-BE-000) + none (task đầu tiên của CR-TRACE-018)
**Status:** ✅ Done (2026-08-04) — Implemented per spec: 4 `taskGraph:*` tracers added additively to `tracers.ts`; `TaskService.addEdge()` wrapped with `Tracers.taskGraphEdgeFlow` (`step('cycle-check')` then `ok()`/`fail('TASK_DEPENDENCY_CYCLE')`); `EdgeParam` in `task-rpc-handler.ts` got optional `traceId`. Drift: kept the real thrown error message (`TASK_DAG_CYCLE: ...`) unchanged — only the `span.fail()` code is `TASK_DEPENDENCY_CYCLE` per spec, the actual `Error` message stays `TASK_DAG_CYCLE` (pre-existing, asserted by `TaskService.test.ts`). Confirmed `wouldCreateCycle()` is DFS (stack-based) and `detectCycle()` (BFS) is untouched. Note: mid-session, this file (and TaskAIPlanner.ts/TaskGrantService.ts/TaskAgentExecutor.ts/task-rpc-handler.ts) briefly reverted to pre-edit state due to concurrent multi-agent activity on the shared tree — re-applied and re-verified on disk via `grep`/`bash` after the revert. `pnpm tsc --noEmit` clean; `TaskService.test.ts` 26/26 pass (24 original + 2 new tracing tests, see TASK-BE-018.6).

---

## Trước khi sửa (bắt buộc theo CLAUDE.md)

```bash
codegraph explore "TaskService.addEdge"
```

Symbol đã tồn tại (MODIFY case) — cycle detection là phần nhạy cảm nhất của method này. Chạy:

```
gitnexus_impact({ target: "TaskService.addEdge", direction: "upstream" })
```

(Phần đăng ký 4 tracer mới vào `Tracers` là additive-only — chỉ cần `codegraph explore "Tracers"`.) Báo cáo blast radius trước khi sửa — xác nhận `TaskDAGValidator.wouldCreateCycle()` (DFS thật, KHÔNG phải BFS) không bị đụng logic, chỉ thêm `span.step()` bao quanh. Nếu risk HIGH/CRITICAL, dừng lại và xác nhận với người dùng trước khi tiếp tục.

## Mô tả

Khai báo 4 tracer mới (`taskGraphEdgeFlow`/`taskGraphAiPlanFlow`/`taskGraphGrantFlow`/`taskGraphExecuteFlow`) trong `tracers.ts`, rồi bọc `TaskService.addEdge()` bằng span `taskGraph:addEdge`. **Sửa nhầm lẫn thuật ngữ trong CR-TRACE-018 gốc:** CR mô tả `TaskDAGValidator.wouldCreateCycle()` dùng BFS — code thật là **DFS** (dùng `stack`, không phải `queue`; docstring dòng 25 ghi rõ "DFS from `to`"). `TaskDAGValidator.detectCycle()` mới thật sự là BFS (`queue.shift()`), nhưng **không** được gọi từ `addEdge()` — không đụng tới trong task này.

## File: `src/shared/trace/tracers.ts` [MODIFY]

```typescript
import { createTracer } from './index'

export const Tracers = {
  // ...existing entries unchanged (profile:*, aiProvider:*, workflow:* từ TASK-BE-015.1/016.1/017.2)...

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

## File: `src/main/task/TaskService.ts` [MODIFY]

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

> **Lưu ý:** `task-rpc-handler.ts`'s `task.addEdge` handler (dòng 253-262) chỉ gọi `taskService.addEdge(...)` sau khi `requirePermission(grantService, userId, params.fromTaskId, 'edit')` — permission check này dùng chung `taskGraphGrantFlow` (TASK-BE-018.3), KHÔNG lồng vào `taskGraphEdgeFlow` vì đây là 2 sub-flow riêng biệt (permission check là cross-cutting, chạy trước hầu hết mọi RPC method, không riêng `addEdge`).

## File: `src/main/task/task-rpc-handler.ts` [MODIFY — chỉ phần schema]

```typescript
const EdgeParam = z.object({
  fromTaskId: z.string().min(1),
  toTaskId: z.string().min(1),
  edgeType: z.enum(['depends_on', 'blocks', 'relates_to', 'duplicates']),
  traceId: z.string().optional(),   // [NEW]
})
```

> `taskService.addEdge()` hiện chưa nhận `traceId` param ở entry point (chỉ tự `start()` span mới bên trong) — theo đúng SOL-BE-TRACE-018 §2.7, việc forward `traceId` từ RPC layer vào `addEdge()` là bổ sung nhỏ không chặn phần lõi, để lại cho patch sau nếu cần FE-initiated resume.

## Verification

```bash
pnpm tsc --noEmit
```

```bash
# Sau khi sửa xong, trước khi coi task DONE:
gitnexus_detect_changes()
```

Xác nhận chỉ các symbol/flow dự kiến bị ảnh hưởng — nếu detect_changes báo thêm symbol ngoài phạm vi task này, điều tra trước khi tiếp tục.

## Acceptance Criteria

- [ ] 4 tracer `taskGraphEdgeFlow`/`taskGraphAiPlanFlow`/`taskGraphGrantFlow`/`taskGraphExecuteFlow` tồn tại trong `tracers.ts` với đúng flow name `taskGraph:addEdge`/`taskGraph:aiPlan`/`taskGraph:grantResolve`/`taskGraph:execute`
- [ ] `Tracers.taskGraphEdgeFlow` phân biệt `TASK_DEPENDENCY_CYCLE` (fail) và edge hợp lệ (ok), kèm `step('cycle-check', { wouldCycle })`
- [ ] Tài liệu/comment ghi rõ `wouldCreateCycle()` là **DFS** (stack-based), không phải BFS — sửa đúng nhầm lẫn thuật ngữ của CR-TRACE-018 gốc
- [ ] `EdgeParam` (`task-rpc-handler.ts`) có field `traceId?: string` optional
- [ ] `TaskDAGValidator.detectCycle()` (BFS thật, không dùng bởi `addEdge()`) KHÔNG bị đụng tới trong task này
- [ ] `pnpm tsc --noEmit` pass, không lỗi mới
