# TASK-FE-001.2: Instrument `removeWorktree()` — span bao trọn hook-trust dialog + xoá

**Phase:** 1
**SOL Ref:** [SOL-FE-TRACE-001 §1.4, §2.3](../solutions/SOL-FE-TRACE-001-worktree-management.md)
**CR Ref:** [CR-TRACE-001](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-001-worktree-management.md)
**Prerequisite:** Phase 0 (TASK-FE-000, TASK-FE-001) + TASK-FE-001.1 (dùng chung tracer `worktreeDelete` đã khai báo trong `tracers.ts` ở task đó)
**Status:** ✅ Done (2026-08-03) — Instrumented `removeWorktree()` with span wrapping `ensureHooksConfirmed()` + removal RPC, using `Tracers.uiWorktreeDeleteFlow` (see TASK-FE-001.1 note on the `ui:` prefix collision/resolution); confirmed `delete-worktree-flow.ts`'s `runWorktreeDeleteWithToast`/parallel-delete helpers create no tracer of their own, just forward to `removeWorktree()` (verified by grep, no tracer references); added 4 new tracing tests (start+step+ok, fail, no-traceId-leak-into-Electron-IPC, N-parallel-independent-spans) in worktrees.test.ts; `pnpm tsc --noEmit` clean, worktrees.test.ts 208/208 and delete-worktree-flow.test.ts 11/11 pass.

---

## Trước khi sửa (bắt buộc theo CLAUDE.md)

```bash
codegraph explore "removeWorktree"
```

Nếu symbol đã tồn tại (MODIFY case): chạy thêm

```
gitnexus_impact({ target: "removeWorktree", direction: "upstream" })
```

và báo cáo blast radius (caller trực tiếp, component/hook bị ảnh hưởng, risk level) trước khi sửa. Nếu risk HIGH/CRITICAL, dừng lại và xác nhận với người dùng trước khi tiếp tục.

## Mô tả

Không có `worktree.checkSafety` nào được gọi từ renderer (đã grep xác nhận) — thông tin "an toàn" trong dialog xác nhận đọc từ state đã sync sẵn trong store (`gitStatusByWorktree[worktreeId]`). Do đó BL-WT-03 phía frontend chỉ có **một** span (`ui:worktree.delete`), khác với backend vốn có thêm span "safety check" riêng (RPC đó không tồn tại nên không có gì để trace).

Hai entry point UI thật đều gọi cùng action `removeWorktree()` (`worktrees.ts:3229`) — `runWorktreeDeleteWithToast()` (`delete-worktree-flow.ts:130-139`) và `handleDelete(force=true)` trực tiếp trong `DeleteWorktreeDialog.tsx:251-270`. Vì span đặt trong action dùng chung, cả hai đường tự động được trace mà không cần instrument riêng từng nơi gọi.

## File: `src/renderer/src/store/slices/worktrees.ts` [MODIFY]

```typescript
removeWorktree: async (worktreeId, force, options) => {
  const removalOwner = resolveWorktreeRemovalHost(get(), worktreeId)
  if (removalOwner.ambiguous) {
    return { ok: false, error: WORKTREE_REMOVAL_AMBIGUOUS_ERROR }
  }
  const hostId = removalOwner.hostId ?? undefined
  const forgetLocalOnly = options?.mode === 'forget-local'

  // Why: span tạo trước khi set() trạng thái "đang xoá" để bao trọn cả
  // ensureHooksConfirmed() (dialog hook trust) — bước này có thể chờ user tương tác,
  // nhưng CR-TRACE-001 §4 BL-WT-03 gộp nó vào cùng span "delete" (khác với
  // "checkSafety round-trip" — vốn không tồn tại trên frontend).
  const span = Tracers.worktreeDelete.start({ worktreeId, force, forgetLocalOnly })

  set((s) => ({
    deleteStateByWorktreeId: {
      ...s.deleteStateByWorktreeId,
      [worktreeId]: { isDeleting: true, phase: 'deleting', error: null, canForceDelete: false, forceDeleteReason: null }
    }
  }))

  try {
    const skipArchive = forgetLocalOnly
      ? true
      : (await ensureHooksConfirmed(get(), getRepoIdFromWorktreeId(worktreeId), 'archive', hostId)) === 'skip'

    const worktreeBeforeRemoval = get().allWorktrees().find((entry) => entry.id === worktreeId)
    const currentOwner = resolveWorktreeRemovalHost(get(), worktreeId)
    if (currentOwner.ambiguous || (hostId && currentOwner.hostId && currentOwner.hostId !== hostId)) {
      throw new Error(WORKTREE_REMOVAL_AMBIGUOUS_ERROR)
    }

    const target = getActiveRuntimeTarget(
      hostId ? settingsForExecutionHostOwner(get().settings, hostId) : settingsForWorktreeOwner(get(), worktreeId)
    )
    span.step('resolve-target', { targetKind: target.kind })

    const removalResult = await (forgetLocalOnly
      ? window.api.worktrees.forgetLocal({ worktreeId, hostId })
      : target.kind === 'local'
        ? window.api.worktrees.remove({ worktreeId, hostId, force, skipArchive })
        : callRuntimeRpc<RemoveWorktreeResult>(
            target,
            'worktree.rm',
            { worktree: toRuntimeWorktreeSelector(worktreeId), force, runHooks: !skipArchive, traceId: span.id },
            { timeoutMs: 60_000 }
          ))

    // ...existing forgetHugeRepoWarningDismissalsForWorktrees, automation snapshot,
    // shutdownWorktreeBrowsers/Terminals, cleanupEphemeralVmRuntimesForDeleted,
    // set() cleanup, prune*, toast logic — unchanged...

    const preservedBranch = removalResult?.preservedBranch
    pruneHostedReviewLinkMutationGenerations([worktreeId])

    span.ok({ worktreeId, preservedBranch: Boolean(preservedBranch) })
    return preservedBranch ? { ok: true as const, preservedBranch } : { ok: true as const }
  } catch (err) {
    console.warn('Failed to remove worktree:', err)
    const error = err instanceof Error ? err.message : String(err)
    const forceDeleteReason = classifyWorktreeForceDeleteReason(error, force)
    const locked = isLockedWorktreeRemovalError(error)
    set((s) => ({
      deleteStateByWorktreeId: {
        ...s.deleteStateByWorktreeId,
        [worktreeId]: {
          isDeleting: false, error, canForceDelete: forceDeleteReason !== null, forceDeleteReason,
          ...(locked ? { lockReason: getLockedWorktreeRemovalReason(error) } : {})
        }
      }
    }))
    // Why: git từ chối xoá non-force vì có uncommitted/untracked files là một decision
    // point người dùng xử lý qua toast (Force Delete), không phải app error thông thường —
    // nhưng vẫn đáng fail() để TracePanel thấy tỉ lệ xoá thất bại và lý do.
    span.fail(err, { worktreeId, force, forceDeleteReason: forceDeleteReason ?? '', locked })
    return { ok: false as const, error }
  }
}
```

## Verification

```bash
pnpm tsc --noEmit
pnpm test --run src/renderer/src/store/slices/worktrees.test.ts
pnpm test --run src/renderer/src/components/sidebar/__tests__/delete-worktree-flow.test.ts
```

```bash
# Sau khi sửa xong, trước khi coi task DONE:
gitnexus_detect_changes()
```

Xác nhận chỉ các symbol/flow dự kiến bị ảnh hưởng — nếu detect_changes báo thêm symbol ngoài phạm vi task này, điều tra trước khi tiếp tục.

## Acceptance Criteria

- [ ] `removeWorktree()` phát span `ui:worktree.delete` bao trọn cả bước `ensureHooksConfirmed()` (hook-trust dialog) lẫn `relay`/IPC removal call
- [ ] Cả 2 entry point UI (`delete-worktree-flow.ts` và `DeleteWorktreeDialog.tsx` force-branch) đều được trace tự động vì span nằm trong action dùng chung
- [ ] `span.step('resolve-target', { targetKind })` được gọi trước khi thực hiện removal call
- [ ] `traceId: span.id` chỉ thêm vào nhánh `callRuntimeRpc(target, 'worktree.rm', ...)` khi `target.kind !== 'local'`; KHÔNG thêm vào `window.api.worktrees.forgetLocal/remove`
- [ ] Thành công (kể cả `preservedBranch`) → `span.ok({ worktreeId, preservedBranch })`
- [ ] Lỗi (kể cả "dirty file rejection" → `ok:false` ở tầng return) → `span.fail(err, { worktreeId, force, forceDeleteReason, locked })`
- [ ] Test mới trong `worktrees.test.ts` (≥ 4 case) + `delete-worktree-flow.test.ts` mới (2 case: `runWorktreeDeleteWithToast()` không tự tạo span riêng — chỉ forward qua `removeWorktree()`; `runWorktreeDeletesInParallel()` với N worktree → N span `ui:worktree.delete` độc lập không share id)
- [ ] Test suite xác nhận `traceId` không bị leak vào nhánh `window.api.worktrees.create/remove` (Electron IPC desktop)
