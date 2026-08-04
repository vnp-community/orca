# TASK-BE-001.2: Instrument handler `worktree.create` / `worktree.rm`

**Phase:** 1
**SOL Ref:** [SOL-BE-TRACE-001](../solutions/SOL-BE-TRACE-001-worktree-management.md)
**CR Ref:** [CR-TRACE-001](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-001-worktree-management.md)
**Prerequisite:** Phase 0 (TASK-BE-000) + TASK-BE-001.1
**Status:** ✅ Done (2026-08-03) — Instrumented `worktree.create`/`worktree.rm` exactly as specced; `createManagedWorktree()`'s real result shape is `{ worktree: Worktree & {...} }` (not `{worktreeId, path}` as the task doc's pseudo-code implied), so `span.ok()` reads `result.worktree?.id`/`result.worktree?.path`. typecheck clean (only pre-existing unrelated `aiProviderService` error); 22/22 existing tests in `worktree.test.ts` pass; gitnexus detect_changes (staged) confirms LOW risk, only expected symbols touched.

---

## Trước khi sửa (bắt buộc theo CLAUDE.md)

```bash
codegraph explore "worktree.create"
codegraph explore "worktree.rm"
```

Cả 2 là RPC handler đã tồn tại trong `src/main/runtime/rpc/methods/worktree.ts` (MODIFY case) — chạy thêm:

```
gitnexus_impact({ target: "worktree.create", direction: "upstream" })
gitnexus_impact({ target: "worktree.rm", direction: "upstream" })
```

Báo cáo blast radius (caller trực tiếp, process bị ảnh hưởng, risk level) trước khi sửa — đặc biệt chú ý logic `automationProvenance` reserve/release trong `worktree.create` không được đổi thứ tự khi thêm span. Nếu risk HIGH/CRITICAL, dừng lại và xác nhận với người dùng trước khi tiếp tục.

## Mô tả

Bọc (wrap) 2 RPC handler `worktree.create` và `worktree.rm` (`src/main/runtime/rpc/methods/worktree.ts`) bằng tracer `Tracers.worktreeCreate`/`Tracers.worktreeDelete`, resume span từ `params.traceId` nếu có, giữ nguyên toàn bộ business logic hiện có (bao gồm `automationProvenance` reserve/release).

## File: `src/main/runtime/rpc/methods/worktree.ts` [MODIFY]

Thêm import và bọc 2 handler như sau (chỉ thêm span, KHÔNG đổi thứ tự hay logic nghiệp vụ hiện có):

```typescript
import { Tracers } from '../../../../shared/trace/tracers'

// ...

defineMethod({
  name: 'worktree.create',
  params: WorktreeCreate,
  handler: async (params, { runtime }) => {
    const span = Tracers.worktreeCreate.start(
      { repoSelector: params.repo, baseBranch: params.baseBranch ?? '' },
      params.traceId ? { id: params.traceId } : undefined
    )
    const repo = await runtime.showRepo(params.repo)
    const automationProvenance = resolveAutomationWorkspaceProvenance({
      authority: runtime,
      repoSelector: params.repo,
      repo,
      request: params.automationProvenanceRequest
    })
    // Why: provenance tokens are reserved before creation so retries can recover,
    // but failed create attempts must release the reservation for a safe retry.
    try {
      span.step('resolve-repo', { repoId: repo.id })
      const result = await runtime.createManagedWorktree({
        // ...existing args unchanged (name, baseBranch, compareBaseRef, linkedIssue, ...)...
        automationProvenance,
        // ...existing lineage/startup blocks unchanged...
      })
      finishAutomationWorkspaceProvenanceRequest(params.automationProvenanceRequest)
      span.ok({ worktreeId: result.worktreeId ?? result.id, path: result.path })
      // Why: agent callers need a stable dispatch target without traversing
      // terminal-list layout duplicates after creating the worktree.
      return params.startupAgent && result.startupTerminal?.handle
        ? { ...result, agentTerminalHandle: result.startupTerminal.handle }
        : result
    } catch (error) {
      releaseAutomationWorkspaceProvenanceRequest(params.automationProvenanceRequest)
      span.fail(error, { repoSelector: params.repo })
      throw error
    }
  }
}),

// ...

defineMethod({
  name: 'worktree.rm',
  params: WorktreeRemove,
  handler: async (params, { runtime }) => {
    const span = Tracers.worktreeDelete.start(
      { worktreeId: params.worktree, force: params.force === true },
      params.traceId ? { id: params.traceId } : undefined
    )
    try {
      const result = await runtime.removeManagedWorktree(
        params.worktree,
        params.force === true,
        params.runHooks === true
      )
      span.ok({ worktreeId: params.worktree })
      return { removed: true, ...result }
    } catch (error) {
      span.fail(error, { worktreeId: params.worktree })
      throw error
    }
  }
})
```

**Lưu ý quan trọng (giữ nguyên trong comment code nếu review sau này cần tham chiếu):** KHÔNG dùng `resume` giữa `worktree.checkSafety` và `worktree.rm` — method `worktree.checkSafety` không tồn tại trong code hiện tại (theo CR-TRACE-001 §4 BL-WT-03), nên vấn đề "2 round-trip cách nhau bởi user-confirmation" hiện chưa áp dụng được. Ghi chú này giữ lại cho lần implement `worktree.checkSafety` sau này (không được `resume` từ `worktree.rm`).

## Verification

```bash
pnpm tsc --noEmit
pnpm test --run src/main/runtime/rpc/methods/__tests__/worktree.test.ts
```

```bash
# Sau khi sửa xong, trước khi coi task DONE:
gitnexus_detect_changes()
```

Xác nhận chỉ các symbol/flow dự kiến bị ảnh hưởng — nếu detect_changes báo thêm symbol ngoài phạm vi task này, điều tra trước khi tiếp tục.

## Acceptance Criteria

- [ ] Handler `worktree.create` phát `worktree:create` span, `ok()` chứa `worktreeId`/`path`; logic `automationProvenance` reserve/release giữ nguyên không đổi
- [ ] Handler `worktree.rm` phát `worktree:delete` span độc lập (không `resume` từ `worktree.create`)
- [ ] Cả 2 handler resume span id từ `params.traceId` khi có, tạo id mới khi không có (backward-compatible với params cũ không có `traceId`)
- [ ] `span.fail()` được gọi trước khi throw, kèm `releaseAutomationWorkspaceProvenanceRequest` chạy đúng trên nhánh lỗi của `worktree.create`
- [ ] Không có `span.step()` nào được thêm cho thao tác DB đơn dòng (theo CR-TRACE-000 §5)
