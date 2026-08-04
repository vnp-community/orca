# TASK-FE-001.1: Đăng ký tracer worktree + instrument `createWorktree()`

**Phase:** 1
**SOL Ref:** [SOL-FE-TRACE-001 §2.1, §2.2](../solutions/SOL-FE-TRACE-001-worktree-management.md)
**CR Ref:** [CR-TRACE-001](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-001-worktree-management.md)
**Prerequisite:** Phase 0 (TASK-FE-000, TASK-FE-001) + external TASK-BE-000
**Status:** ✅ Done (2026-08-03) — Instrumented `createWorktree()` retry loop with a single span; drift: task doc's plan to rename shared `Tracers.worktreeCreate`/`worktreeDelete` (`worktree:create`/`worktree:delete`) to `ui:` prefix collided with agent/backend-domain ownership of those exact entries (concurrent sibling edit confirmed `worktree:create`/`worktree:delete` are agent/backend-side, used by `src/relay/agent-git-handler.ts`) — resolved by adding NEW distinct entries `uiWorktreeCreateFlow`/`uiWorktreeFanOutFlow`/`uiWorktreeCompareFlow`/`uiWorktreeMergeFlow` (`ui:worktree.*`) instead, and pointing the renderer call sites at those; fixed 2 pre-existing exact-match tests in worktrees.test.ts that broke once `traceId` was added to RPC params (now use `expect.any(String)`); added 3 new tracing tests (start/step/fail) in worktrees.test.ts; `pnpm tsc --noEmit` clean, `worktrees.test.ts` 208/208 pass.

---

## Trước khi sửa (bắt buộc theo CLAUDE.md)

```bash
codegraph explore "createWorktree"
```

Nếu symbol đã tồn tại (MODIFY case): chạy thêm

```
gitnexus_impact({ target: "createWorktree", direction: "upstream" })
```

và báo cáo blast radius (caller trực tiếp, component/hook bị ảnh hưởng, risk level) trước khi sửa. Nếu risk HIGH/CRITICAL, dừng lại và xác nhận với người dùng trước khi tiếp tục.

## Mô tả

Renderer không gọi RPC trực tiếp từ component cho worktree — mọi entry point UI đi qua 2 action Zustand dùng chung trong `src/renderer/src/store/slices/worktrees.ts`: `createWorktree()` (dòng 2928) và `removeWorktree()` (dòng 3229, xem TASK-FE-001.2). Task này chỉ xử lý `createWorktree()` (BL-WT-01), thêm 5 tracer mới (2 trong số đó chưa có call site — BL-WT-02/04/05 chưa implement, chỉ khai báo tên sẵn).

**Quyết định kiến trúc quan trọng:** span đặt ở action `createWorktree()`, KHÔNG đặt ở `callRuntimeRpc()` (hàm transport dùng chung cho hàng trăm RPC method khác nhau) — vì field ngữ cảnh domain (`repoId`, `baseBranch`) chỉ có sẵn tại call site domain.

## File: `src/shared/trace/tracers.ts` [MODIFY, additive]

```typescript
// src/shared/trace/tracers.ts
export const Tracers = {
  // ...existing entries unchanged...
  worktreeCreate:  createTracer('ui:worktree.create'),   // BL-WT-01
  worktreeFanOut:  createTracer('ui:worktree.fanOut'),    // BL-WT-02 — chưa có call site, đặt tên sẵn
  worktreeDelete:  createTracer('ui:worktree.delete'),    // BL-WT-03 — dùng ở TASK-FE-001.2
  worktreeCompare: createTracer('ui:worktree.compare'),   // BL-WT-04 — chưa có call site, đặt tên sẵn
  worktreeMerge:   createTracer('ui:worktree.merge'),     // BL-WT-05 — chưa có call site, đặt tên sẵn
} as const
```

> **N.B. prefix `ui:`:** 5 tracer trên dùng prefix `ui:` — nhất quán với convention mà TASK-FE-001 áp dụng cho toàn bộ 10 CR (xem TASK-FE-001 §Mô tả, đã fix 2026-08-02). `TracePanel.tsx:42`'s `isBackend` heuristic chỉ gắn nhãn "backend" cho flow name chứa `:` mà KHÔNG bắt đầu bằng `ui:` — prefix này đảm bảo các event `ui:worktree.*` phát từ renderer được TracePanel nhận diện đúng là frontend event.

## File: `src/renderer/src/store/slices/worktrees.ts` [MODIFY]

Thêm import `Tracers`, bọc span quanh toàn bộ retry loop (tối đa 25 lần retry tên trùng) — **một span duy nhất bao trọn cả loop**, mỗi lần retry chỉ `span.step()`, không tạo span mới:

```typescript
import { Tracers } from '../../../../shared/trace/tracers'

createWorktree: async (repoId, name, baseBranch, /* ...existing params unchanged... */) => {
  const automationProvenanceRequest = options?.automationProvenanceRequest
  const retryableConflictPatterns = [/* ...existing patterns unchanged... */]
  const nextCandidateName = (current: string, attempt: number): string =>
    attempt === 0 ? current : `${current}-${attempt + 1}`

  // Why: một span duy nhất bao trọn toàn bộ retry loop (CR-TRACE-000 §5) —
  // 25 lần retry tên trùng không nên tạo 25 span riêng, mỗi lần chỉ step().
  const span = Tracers.worktreeCreate.start({
    repoId,
    baseBranch: baseBranch ?? '',
    telemetrySource: telemetrySource ?? ''
  })

  try {
    for (let attempt = 0; attempt < 25; attempt += 1) {
      const candidateName = nextCandidateName(name, attempt)
      if (attempt > 0) {
        span.step('retry-name-conflict', { attempt, candidateName })
      }
      try {
        // ...existing manualOrder/activeScope/parentWorkspace/createArgs build unchanged...
        const target = getActiveRuntimeTarget(settingsForRepoOwner(get(), repoId))
        const result =
          target.kind === 'local'
            ? await window.api.worktrees.create(createArgs)
            : await callRuntimeRpc<Awaited<ReturnType<typeof window.api.worktrees.create>>>(
                target,
                'worktree.create',
                { repo: repoId, name: candidateName, baseBranch, /* ...existing optional fields... */ traceId: span.id }
              )
        // ...existing set() merge into worktreesByRepo, toasts...
        span.ok({ worktreeId: result.worktree.id, path: result.worktree.path, attempt })
        return result
      } catch (error) {
        const message = error instanceof Error ? error.message : String(error)
        const shouldRetry = retryableConflictPatterns.some((pattern) => pattern.test(message))
        if (!shouldRetry || attempt === 24) {
          span.fail(error, { repoId, attempt })
          throw error
        }
      }
    }
    throw new Error('Failed to create worktree after retrying branch conflicts.')
  } catch (err) {
    console.error('Failed to create worktree:', err)
    throw err
  }
}
```

**Ghi chú field `traceId`:** chỉ thêm vào nhánh `callRuntimeRpc(...)` (WebSocket RPC, đúng CR-TRACE-000 §3.3). Nhánh `window.api.worktrees.create(createArgs)` là Electron `contextBridge` IPC — KHÔNG nằm trong 6 hàng transport của CR-TRACE-000 §3.3, KHÔNG thêm `traceId` ở đây (gap cần một CR-TRACE-000 addendum riêng, ngoài phạm vi task này).

Không thêm bất kỳ call site nào cho `worktree.fanOut`/`worktree.compare`/`worktree.merge` — BL-WT-02/04/05 chưa có implementation ở renderer, chỉ tracer đã khai báo sẵn ở trên.

## Verification

```bash
pnpm tsc --noEmit
pnpm test --run src/renderer/src/store/slices/worktrees.test.ts
pnpm test --run src/shared/trace/__tests__/tracers.test.ts
```

```bash
# Sau khi sửa xong, trước khi coi task DONE:
gitnexus_detect_changes()
```

Xác nhận chỉ các symbol/flow dự kiến bị ảnh hưởng — nếu detect_changes báo thêm symbol ngoài phạm vi task này, điều tra trước khi tiếp tục.

## Acceptance Criteria

- [ ] `Tracers.worktreeCreate/worktreeFanOut/worktreeDelete/worktreeCompare/worktreeMerge` thêm vào `tracers.ts` đúng tên `ui:worktree.create|fanOut|delete|compare|merge`
- [ ] `createWorktree()` mở đúng 1 span bao trọn toàn bộ retry loop 25 lần, không tạo span mới mỗi lần retry
- [ ] `callRuntimeRpc(target, 'worktree.create', ...)` đính kèm `traceId: span.id` trong params khi `target.kind !== 'local'`
- [ ] KHÔNG thêm `traceId` vào nhánh `window.api.worktrees.create(...)` (Electron IPC, ngoài phạm vi CR-TRACE-000 §3.3 hiện tại)
- [ ] Thành công → `span.ok({ worktreeId, path, attempt })`; lỗi không-retryable → `span.fail(error, { repoId, attempt })` đúng 1 lần (không double-fail qua outer catch)
- [ ] Không có span/tracer nào được thêm cho `prefetchWorktreeCreateBase()` (latency hedge, lỗi không user-facing)
- [ ] `ui:worktree.fanOut`/`ui:worktree.compare`/`ui:worktree.merge` chỉ khai báo tên trong `tracers.ts`, không có call site code nào
- [ ] Test suite `worktrees.test.ts` có thêm ≥ 6 test case theo Test Plan của SOL-FE-TRACE-001 §3 (start trước RPC, traceId trong params khi target không phải local, step() mỗi retry, ok()/fail() đúng field)
