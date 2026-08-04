# SOL-FE-TRACE-001: Worktree Management — Frontend-Side Tracing Implementation

**CR Ref:** [CR-TRACE-001](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-001-worktree-management.md)
**TDD Ref:** TDD-FE-03 (Runtime Client Layer — `runtime-rpc-client.ts`), TDD-FE-05 (UI Components — App Shell)
**Status:** Proposed
**Dependency:** F40 core tracing infra (đã implement) — `src/shared/trace/browser.ts`, `src/shared/trace/tracers.ts`, TracePanel. CR-TRACE-000 (naming convention, `resume` param cho `Tracer.start()`, quy ước field `traceId`).

---

## 1. Điểm khởi tạo trace trong Renderer

### 1.1 Kiến trúc thực tế đã xác nhận (khác một phần so với mô tả CR-TRACE-001 §1)

Renderer **không gọi RPC trực tiếp từ component** cho worktree — mọi UI entry point đều đi qua 2 action dùng chung trong Zustand slice `src/renderer/src/store/slices/worktrees.ts`:

- `createWorktree(...)` — action, định nghĩa tại `worktrees.ts:2928`
- `removeWorktree(worktreeId, force, options)` — action, định nghĩa tại `worktrees.ts:3229`

Cả hai đều rẽ nhánh theo `target.kind` (kết quả của `getActiveRuntimeTarget()`, import từ `src/renderer/src/runtime/runtime-rpc-client.ts`):

```typescript
// worktrees.ts:3019-3025 (rút gọn)
const target = getActiveRuntimeTarget(settingsForRepoOwner(get(), repoId))
const result =
  target.kind === 'local'
    ? await window.api.worktrees.create(createArgs)                 // Electron contextBridge IPC
    : await callRuntimeRpc(target, 'worktree.create', { ... })       // WebSocket RPC (web/remote target)
```

```typescript
// worktrees.ts:3279-3292 (rút gọn)
const removalResult = await (forgetLocalOnly
  ? window.api.worktrees.forgetLocal({ worktreeId, hostId })
  : target.kind === 'local'
    ? window.api.worktrees.remove({ worktreeId, hostId, force, skipArchive })
    : callRuntimeRpc(target, 'worktree.rm', { worktree: toRuntimeWorktreeSelector(worktreeId), force, runHooks: !skipArchive }, { timeoutMs: 60_000 }))
```

**Quyết định kiến trúc:** đặt span ở 2 action này (không phải ở `runtime-rpc-client.ts`'s `callRuntimeRpc()`), vì:
1. `callRuntimeRpc()` là hàm transport dùng chung cho **hàng trăm** RPC method khác nhau (`git.status`, `fs.readDir`, `terminal.create`, ...) — không biết method nào cần tracer nào, sẽ phải nhúng một bảng tra cứu method→tracer vào một hàm transport thuần túy, vi phạm nguyên tắc tách concern.
2. Field ngữ cảnh domain (`repoId`, `baseBranch`, `attempt`, ...) chỉ có sẵn tại call site domain (action `createWorktree`/`removeWorktree`), không có ở `callRuntimeRpc()`.
3. `traceId` chỉ là một field bên trong `params` (theo CR-TRACE-000 §3.2) — không cần đổi signature của `callRuntimeRpc()` để hỗ trợ nó, chỉ cần thêm `traceId: span.id` vào object `params` tại call site.

### 1.2 BL-WT-01 — Tạo Worktree

**Entry point UI thật:** nút "Create" trong New Workspace Composer → `submit()` (`useCallback`) tại `src/renderer/src/hooks/useComposerState.ts:3414`, gọi `createWorktree(...)` tại `useComposerState.ts:3644`.

`createWorktree()` action (`worktrees.ts:2928`) tự retry tối đa 25 lần khi tên branch/worktree bị trùng (`nextCandidateName()`, `retryableConflictPatterns`, `worktrees.ts:2955-2975`) — đây là chi tiết quan trọng cho việc đặt span: **một span duy nhất bao trọn toàn bộ retry loop**, mỗi lần retry là một `span.step('retry-name-conflict', { attempt })`, không phải một span/attempt (tránh spam TracePanel khi backend liên tục từ chối tên trùng).

Có một call phụ, **không đáng instrument riêng**: `prefetchWorktreeCreateBase()` (`worktrees.ts:2906`) gọi `callRuntimeRpc(target, 'worktree.prefetchCreateBase', ...)` khi user mới mở composer (latency hedge). Comment trong code (`worktrees.ts:2925-2926`) đã nói rõ "the create path awaits the same backend refresh and owns user-visible error reporting" — theo CR-TRACE-000 §5, đây là một optimization phụ, lỗi của nó không quan trọng với người dùng, không cần span.

### 1.3 BL-WT-02 (fan-out) / BL-WT-04 (compare) / BL-WT-05 (merge) — Xác nhận lại: không có call site

Đã grep `worktree.fanOut`, `worktree.compare`, `worktree.merge`, `fanOut(`, `compareWorktree(` (loại trừ các hit false-positive là hàm sort `compareWorktreeSortLabel`/`compareWorktreeCandidates` — không liên quan) trên toàn bộ `src/renderer/src` — không có kết quả. Khớp với phát hiện của CR-TRACE-001 §1 điểm 5: các sub-flow này **chưa có implementation** ở bất kỳ layer nào, kể cả renderer. Mục 3 dưới đây vẫn khai báo tracer để sẵn sàng cắm vào khi 2 sub-flow này được implement.

### 1.4 BL-WT-03 — Xóa Worktree An Toàn

**Không có `worktree.checkSafety` nào được gọi từ renderer** — xác nhận lại phát hiện của CR-TRACE-001 §4 điểm 4 (grep `checkSafety` trên `src/renderer/src` không có kết quả). Thông tin "an toàn" hiển thị trong dialog xác nhận (uncommitted changes, agent đang chạy) được đọc từ state **đã sync sẵn** trong store (`gitStatusByWorktree[worktreeId]`, xem `src/renderer/src/components/sidebar/delete-worktree-flow.ts:151-152`), không phải một RPC round-trip riêng. Do đó ở frontend, BL-WT-03 chỉ có **một** span (`ui:worktree.delete`), không có span "safety check" như CR-TRACE-001 §4 mô tả cho phía backend (RPC `worktree.checkSafety` không tồn tại nên không có gì để trace).

Hai entry point UI thật, cả hai đều gọi cùng action `removeWorktree()`:
- **Đường xác nhận chính**: `runWorktreeDeleteWithToast()` tại `src/renderer/src/components/sidebar/delete-worktree-flow.ts:130-139` → `useAppStore.getState().removeWorktree(worktreeId, options.force === true)`. Hàm này còn được `runWorktreeDeletesInParallel()` (cùng file, dòng 50-115) gọi tuần tự theo từng repo khi user xoá hàng loạt.
- **Đường force-delete trực tiếp trong dialog**: `handleDelete(force=true)` tại `src/renderer/src/components/sidebar/DeleteWorktreeDialog.tsx:251-270` → gọi thẳng `removeWorktree(worktreeId, true)` (bỏ qua `runWorktreeDeleteWithToast` wrapper).

Vì span được đặt bên trong action `removeWorktree()` (không phải ở 2 call site UI), cả hai đường đều tự động được trace mà không cần instrument riêng từng nơi gọi.

---

## 2. Full Implementation

### 2.1 Thêm tracer mới vào `tracers.ts`

```typescript
// src/shared/trace/tracers.ts
import { createTracer } from './index'

export const Tracers = {
  // ...existing entries unchanged...
  worktreeCreate:  createTracer('ui:worktree.create'),   // BL-WT-01
  worktreeFanOut:  createTracer('ui:worktree.fanOut'),    // BL-WT-02 — chưa có call site, đặt tên sẵn
  worktreeDelete:  createTracer('ui:worktree.delete'),    // BL-WT-03
  worktreeCompare: createTracer('ui:worktree.compare'),   // BL-WT-04 — chưa có call site, đặt tên sẵn
  worktreeMerge:   createTracer('ui:worktree.merge'),     // BL-WT-05 — chưa có call site, đặt tên sẵn
} as const
```

> File này là shared giữa renderer/main/relay (`src/shared/trace/`) — thêm 1 lần, dùng chung cho cả 3 phía (frontend/agent/backend). Không cần fork riêng cho renderer.

### 2.2 `createWorktree()` — `src/renderer/src/store/slices/worktrees.ts`

```typescript
import { Tracers } from '../../../../shared/trace/tracers'

// ...
createWorktree: async (
  repoId,
  name,
  baseBranch,
  setupDecision = 'inherit',
  sparseCheckout,
  telemetrySource,
  displayName,
  linkedIssue,
  linkedPR,
  pushTarget,
  createdWithAgent,
  linkedLinearIssue,
  branchNameOverride,
  workspaceStatus,
  linkedGitLabMR,
  linkedGitLabIssue,
  startup,
  pendingFirstAgentMessageRename,
  creationId,
  linkedLinearIssueWorkspaceId,
  linkedLinearIssueOrganizationUrlKey,
  linkedBitbucketPR,
  linkedAzureDevOpsPR,
  linkedGiteaPR,
  compareBaseRef,
  options
) => {
  const automationProvenanceRequest = options?.automationProvenanceRequest
  const retryableConflictPatterns = [
    /already exists locally/i,
    /already exists on a remote/i,
    /^Branch ".+" already exists\./i,
    /already has pr #\d+/i
  ]
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
      const candidateBranchNameOverride = branchNameOverride
        ? nextCandidateName(branchNameOverride, attempt)
        : undefined
      try {
        // ...existing manualOrder/activeScope/parentWorkspace/createArgs build...
        const createArgs = { /* ...existing fields, unchanged... */ }
        const target = getActiveRuntimeTarget(settingsForRepoOwner(get(), repoId))
        const result =
          target.kind === 'local'
            ? await window.api.worktrees.create(createArgs)
            : await callRuntimeRpc<Awaited<ReturnType<typeof window.api.worktrees.create>>>(
                target,
                'worktree.create',
                {
                  repo: repoId,
                  name: candidateName,
                  baseBranch,
                  // ...existing 25+ optional fields, unchanged...
                  traceId: span.id
                }
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

> Ghi chú field `traceId`: chỉ thêm vào nhánh `callRuntimeRpc(...)` (đi WebSocket RPC tới Orca Server, đúng CR-TRACE-000 §3.3 hàng "WebSocket RPC (Browser ↔ Orca Server)"). Nhánh `window.api.worktrees.create(createArgs)` là Electron `contextBridge` IPC (`ipcRenderer.invoke`) tới Main process cùng máy — **không nằm trong 6 hàng transport của CR-TRACE-000 §3.3**. Đây là một gap trong bảng gốc (mọi luồng desktop-local đều đi qua kênh này) — khuyến nghị mở rộng quy ước sang Electron IPC theo cùng pattern (`traceId` field phẳng trong `params`/`opts`), nhưng việc *tiêu thụ* nó ở phía Main (`ipcMain.handle('worktrees:create', ...)`) thuộc phạm vi CR backend companion, không phải CR này.

### 2.3 `removeWorktree()` — `src/renderer/src/store/slices/worktrees.ts`

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
  // "checkSafety round-trip" — vốn không tồn tại trên frontend, xem mục 1.4).
  const span = Tracers.worktreeDelete.start({ worktreeId, force, forgetLocalOnly })

  set((s) => ({
    deleteStateByWorktreeId: {
      ...s.deleteStateByWorktreeId,
      [worktreeId]: {
        isDeleting: true,
        phase: 'deleting',
        error: null,
        canForceDelete: false,
        forceDeleteReason: null
      }
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
            {
              worktree: toRuntimeWorktreeSelector(worktreeId),
              force,
              runHooks: !skipArchive,
              traceId: span.id
            },
            { timeoutMs: 60_000 }
          ))

    // ...existing forgetHugeRepoWarningDismissalsForWorktrees, automation snapshot,
    // shutdownWorktreeBrowsers/Terminals, cleanupEphemeralVmRuntimesForDeleted,
    // set() cleanup, prune*, toast logic — unchanged...

    const preservedBranch = removalResult?.preservedBranch
    // ...existing preservedBranch toast wiring...
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
          isDeleting: false,
          error,
          canForceDelete: forceDeleteReason !== null,
          forceDeleteReason,
          ...(locked ? { lockReason: getLockedWorktreeRemovalReason(error) } : {})
        }
      }
    }))
    // Why: git từ chối xoá non-force vì có uncommitted/untracked files là một
    // decision point người dùng xử lý qua toast (Force Delete), không phải app
    // error theo nghĩa thông thường — nhưng vẫn đáng fail() để TracePanel thấy
    // được tỉ lệ xoá thất bại và lý do (forceDeleteReason/locked).
    span.fail(err, { worktreeId, force, forceDeleteReason: forceDeleteReason ?? '', locked })
    return { ok: false as const, error }
  }
}
```

### 2.4 BL-WT-02/04/05 — Chưa implement, không thêm code call site

Không thêm bất kỳ call site nào cho `worktree.fanOut`/`worktree.compare`/`worktree.merge` trong renderer ở CR này — chỉ tracer trong `tracers.ts` (mục 2.1) được khai báo sẵn. Khi các sub-flow này được implement (ngoài phạm vi CR này), engineer chỉ cần copy pattern ở mục 2.2/2.3 (tạo span đầu action, `step()` cho mỗi vòng lặp/round-trip, `traceId: span.id` vào params của `callRuntimeRpc`, `ok()`/`fail()` ở cuối).

---

## 3. Test Plan (Vitest)

Theo pattern `node` environment mặc định của `config/vitest.config.ts` (xem `specs/frontend/tdd/v5/00-index.md` — các slice/store action test không cần `happy-dom`, chỉ component test cần).

```
src/renderer/src/store/slices/worktrees.test.ts   (file đã tồn tại — thêm test case mới)
├── createWorktree() tracing
│   ├── gọi Tracers.worktreeCreate.start() với { repoId, baseBranch } trước khi gọi callRuntimeRpc/window.api.worktrees.create
│   ├── truyền traceId: span.id vào params của callRuntimeRpc('worktree.create', ...) khi target.kind !== 'local'
│   ├── KHÔNG truyền traceId khi target.kind === 'local' (window.api.worktrees.create không nhận field này) — hoặc xác nhận field được bỏ qua an toàn nếu thêm
│   ├── mỗi lần retry tên trùng gọi span.step('retry-name-conflict', { attempt }) — không tạo span mới
│   ├── thành công → span.ok({ worktreeId, path, attempt })
│   └── lỗi không-retryable → span.fail(error, { repoId, attempt }) đúng 1 lần (không double-fail qua outer catch)
├── removeWorktree() tracing
│   ├── gọi Tracers.worktreeDelete.start({ worktreeId, force, forgetLocalOnly }) trước ensureHooksConfirmed()
│   ├── truyền traceId: span.id vào params của callRuntimeRpc('worktree.rm', ...) khi target.kind !== 'local'
│   ├── thành công (kể cả preservedBranch) → span.ok({ worktreeId, preservedBranch })
│   └── lỗi (kể cả "dirty file rejection" → ok:false ở tầng return) → span.fail(err, { worktreeId, force, ... })

src/renderer/src/components/sidebar/__tests__/delete-worktree-flow.test.ts   (mới)
├── runWorktreeDeleteWithToast() không tự tạo span riêng — chỉ forward qua removeWorktree() (đã có span)
└── runWorktreeDeletesInParallel() với N worktree cùng repo → N lần gọi removeWorktree() tuần tự, N span ui:worktree.delete độc lập (không share id)

src/shared/trace/__tests__/tracers.test.ts   (file đã tồn tại — thêm assertion)
└── Tracers.worktreeCreate/worktreeFanOut/worktreeDelete/worktreeCompare/worktreeMerge tồn tại với đúng flow name 'ui:worktree.create|fanOut|delete|compare|merge'
```

**Mock pattern cho `callRuntimeRpc`:** dùng `vi.mock('../../runtime/runtime-rpc-client')` để spy tham số thứ 3 (`params`) truyền vào, assert `params.traceId` bằng `span.id` đã capture từ mock của `Tracers.worktreeCreate.start` (`vi.spyOn(Tracers.worktreeCreate, 'start')`).

**Target:** ≥ 10 test case mới (7 cho worktrees.ts, 2 cho delete-worktree-flow.ts, 1 cho tracers.ts).

---

## 4. Acceptance Criteria

- [ ] `Tracers.worktreeCreate/worktreeFanOut/worktreeDelete/worktreeCompare/worktreeMerge` được thêm vào `src/shared/trace/tracers.ts` đúng tên `ui:worktree.create|fanOut|delete|compare|merge`
- [ ] `createWorktree()` (`worktrees.ts:2928`) mở 1 span duy nhất bao trọn toàn bộ retry loop 25 lần, không tạo span mới mỗi lần retry
- [ ] `callRuntimeRpc(target, 'worktree.create', ...)` và `callRuntimeRpc(target, 'worktree.rm', ...)` đính kèm `traceId: span.id` trong params khi `target.kind !== 'local'`
- [ ] `removeWorktree()` (`worktrees.ts:3229`) phát span `ui:worktree.delete` bao trọn cả bước `ensureHooksConfirmed()` (hook-trust dialog) lẫn `relay`/IPC removal call
- [ ] Cả 2 entry point UI (`delete-worktree-flow.ts` và `DeleteWorktreeDialog.tsx` force-branch) đều được trace tự động vì span nằm trong action dùng chung, không cần instrument riêng từng nơi gọi
- [ ] Không có span/tracer nào được thêm cho `prefetchWorktreeCreateBase()` (latency hedge, lỗi không user-facing — theo CR-TRACE-000 §5)
- [ ] `ui:worktree.fanOut`/`ui:worktree.compare`/`ui:worktree.merge` chỉ được khai báo tên trong `tracers.ts`, không có call site code nào được thêm (BL-WT-02/04/05 chưa tồn tại implementation ở renderer)
- [ ] Test suite `worktrees.test.ts` xác nhận `traceId` không bị leak vào nhánh `window.api.worktrees.create/remove` (Electron IPC desktop) — hoặc, nếu quyết định mở rộng quy ước sang kênh này, có test riêng xác nhận field được truyền nhất quán
