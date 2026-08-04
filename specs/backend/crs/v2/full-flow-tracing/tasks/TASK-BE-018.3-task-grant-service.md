# TASK-BE-018.3: Instrument `TaskGrantService.resolvePermission()` — hot path, chỉ 1 step tổng kết (BL-TG-03)

**Phase:** 3
**SOL Ref:** [SOL-BE-TRACE-018](../solutions/SOL-BE-TRACE-018-task-graph.md)
**CR Ref:** [CR-TRACE-018](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-018-task-graph.md)
**Prerequisite:** Phase 0 (TASK-BE-000) + TASK-BE-018.1
**Status:** ✅ Done (2026-08-04) — Implemented per spec: `TaskGrantService.resolvePermission()` wrapped with `Tracers.taskGraphGrantFlow`; exactly 1 summary `step('grant-match', {matchedScope, direct})` emitted per call (no per-candidate/per-grant noise); `span.fail('NO_GRANT_FOUND', {userId, taskId, ancestorCount})` + return `null` when no grant matches. `gitnexus_impact` on `resolvePermission` (hot path, fan-in 4 direct callers, 1 affected process) returned risk LOW, not HIGH as the task doc warned — proceeded. `pnpm tsc --noEmit` clean; `TaskGrantService.test.ts` 17/17 pass (13 original + 4 new tracing tests including an explicit "exactly 1 step() per call" assertion, see TASK-BE-018.6).

---

## Trước khi sửa (bắt buộc theo CLAUDE.md)

```bash
codegraph explore "TaskGrantService.resolvePermission"
```

Symbol đã tồn tại (MODIFY case) — đây là hot path chạy trên MỌI RPC cần permission check. Chạy:

```
gitnexus_impact({ target: "TaskGrantService.resolvePermission", direction: "upstream" })
```

Với hot path có fan-in lớn như thế này, đặc biệt chú ý risk HIGH có thể xuất hiện — đọc kỹ báo cáo trước khi sửa, và xác nhận chỉ thêm đúng 1 `step()` tổng kết (không step per-candidate/per-grant, tránh regressions về performance). Nếu risk HIGH/CRITICAL, dừng lại và xác nhận với người dùng trước khi tiếp tục.

## Mô tả

Bọc `TaskGrantService.resolvePermission()` bằng span `taskGraph:grantResolve`. **Quan trọng:** thuật toán thật KHÔNG có "level" cố định (`owner`/`admin`/...) như CR minh hoạ — nó duyệt `candidates` (task + ancestors) × `grants` rồi chọn permission **cao nhất** qua `TASK_PERMISSION_ORDER`; scope thật là `'everyone'|'user'|'team'|'role'`, không phải `'owner'|'admin'|'company'|'parent-tree'` như CR. Hàm này chạy trên **mọi** RPC cần permission check (hot path) nên chỉ trace tối thiểu: 1 `step()` tổng kết cuối cùng, KHÔNG `step()` cho từng candidate/grant trong vòng lặp lồng nhau (tránh noise N candidates × M grants mỗi lần gọi).

## File: `src/main/task/TaskGrantService.ts` [MODIFY]

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

- [ ] `Tracers.taskGraphGrantFlow` chỉ emit **đúng 1** `step()` tổng kết mỗi lần gọi — verify: KHÔNG `step()` per-candidate hoặc per-grant trong vòng lặp lồng nhau
- [ ] Không match được grant nào → `span.fail('NO_GRANT_FOUND', { userId, taskId, ancestorCount })`, trả `null`, KHÔNG im lặng nuốt lỗi ủy quyền
- [ ] Match trực tiếp (task chính) → `step('grant-match', { direct: true })`; match chỉ qua ancestor (applyTree) → `direct: false`
- [ ] `matchedScope` phản ánh đúng scope thật (`'everyone'|'user'|'team'|'role'`), không dùng nhầm scope minh hoạ của CR gốc (`'owner'|'admin'|'company'|'parent-tree'`)
- [ ] `pnpm tsc --noEmit` pass, không lỗi mới
