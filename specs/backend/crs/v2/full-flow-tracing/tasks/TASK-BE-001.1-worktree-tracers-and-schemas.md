# TASK-BE-001.1: Đăng ký tracer `worktree:*` và thêm field `traceId` vào schema

**Phase:** 1
**SOL Ref:** [SOL-BE-TRACE-001](../solutions/SOL-BE-TRACE-001-worktree-management.md)
**CR Ref:** [CR-TRACE-001](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-001-worktree-management.md)
**Prerequisite:** Phase 0 (TASK-BE-000) + none (task đầu tiên của CR-TRACE-001)
**Status:** ✅ Done (2026-08-03) — `worktreeCreate`/`worktreeDelete` and all 5 `terminal:*`/`agentOrch:*`/`codeReview:*` tracers were already registered in `tracers.ts` by the concurrent sibling (agent-domain) effort; only added the 3 reserved-name entries (`worktreeFanOut`, `worktreeCompare`, `worktreeMerge`) plus `traceId?` on `WorktreeCreate`, `WorktreeRemove` (worktree-schemas.ts) and `ProjectWorktreeParam` (git-remote.ts). typecheck clean (pre-existing unrelated `aiProviderService` unused-var error confirmed present before this change too).

---

## Trước khi sửa (bắt buộc theo CLAUDE.md)

```bash
codegraph explore "Tracers"
```

`Tracers` (`src/shared/trace/tracers.ts`) là object đã tồn tại (MODIFY case) — chạy thêm:

```
gitnexus_impact({ target: "Tracers", direction: "upstream" })
```

Task này chỉ thêm entry mới (`worktreeCreate`/`worktreeFanOut`/`worktreeDelete`/`worktreeCompare`/`worktreeMerge`) và field `traceId` optional vào 2-3 schema — không đổi bất kỳ entry/field cũ nào. `Tracers` có fan-in lớn (import ở hầu hết mọi domain) — đây là đặc điểm bình thường của 1 registry object, không tự nó là rủi ro; chỉ dừng lại nếu báo cáo cho thấy risk HIGH/CRITICAL đến từ nguyên nhân khác ngoài fan-in thuần tuý (vd. trùng tên entry cũ), và xác nhận với người dùng trước khi tiếp tục.

## Mô tả

Khai báo 5 tracer `worktree:create|fanOut|delete|compare|merge` trong `tracers.ts` (2 tracer `fanOut`/`compare`/`merge` chỉ đăng ký tên, chưa có call site vì RPC method tương ứng chưa tồn tại trong code), và thêm field `traceId?: string` optional vào các schema params liên quan để chuẩn bị cho việc wire tracer ở các task sau.

## File: `src/shared/trace/tracers.ts` [MODIFY]

Thêm khối sau vào object `Tracers` đã tồn tại (giữ nguyên các entry hiện có như `browseDirFlow`, `mkdirFlow`, `rmdirFlow`, `agentWsFlow`, `ipcProxyFlow`):

```typescript
export const Tracers = {
  // ...existing entries (browseDirFlow, mkdirFlow, rmdirFlow, agentWsFlow, ipcProxyFlow) unchanged...

  // ─── CR-TRACE-001: Worktree Management (BL-WT-01→05) ──────────────────────
  /** worktree.create + git.worktree.add — BL-WT-01 */
  worktreeCreate:  createTracer('worktree:create'),
  /** worktree.fanOut — BL-WT-02 — reserved, chưa có RPC method thật */
  worktreeFanOut:  createTracer('worktree:fanOut'),
  /** worktree.rm + git.worktree.remove — BL-WT-03 */
  worktreeDelete:  createTracer('worktree:delete'),
  /** worktree.compare — BL-WT-04 — reserved, chưa có RPC method thật */
  worktreeCompare: createTracer('worktree:compare'),
  /** worktree.merge — BL-WT-05 — reserved, chưa có RPC method thật */
  worktreeMerge:   createTracer('worktree:merge'),
} as const
```

> Lưu ý: `worktreeFanOut`, `worktreeCompare`, `worktreeMerge` chỉ đăng ký tên tracer — KHÔNG viết call site cho các RPC method `worktree.fanOut`/`worktree.compare`/`worktree.merge` vì các method này chưa tồn tại trong code hiện tại (verify qua grep, xem SOL-BE-TRACE-001 §1.2). Không tự tạo RPC method mới ở task này.

## File: `src/main/runtime/rpc/methods/worktree-schemas.ts` [MODIFY]

Thêm field `traceId` vào 2 schema `WorktreeCreate` và `WorktreeRemove` (import `OptionalString` đã có sẵn ở đầu file, từ `'../schemas'`):

```typescript
export const WorktreeCreate = z
  .object({
    repo: z
      .unknown()
      .transform((v) => (typeof v === 'string' ? v : ''))
      .pipe(z.string().min(1, 'Missing repo selector')),
    // ...existing fields unchanged (name, baseBranch, linkedIssue, ...)...
    traceId: OptionalString, // [NEW CR-TRACE-001] wire-propagated span id, xem CR-TRACE-000 §3.2
  })
  // ...existing .superRefine(...) chain unchanged...

export const WorktreeRemove = WorktreeSelector.extend({
  force: OptionalBoolean,
  runHooks: OptionalBoolean,
  traceId: OptionalString, // [NEW CR-TRACE-001]
})
```

**Ràng buộc quan trọng:** KHÔNG sửa `WorktreeSelector` gốc (dùng chung bởi `worktree.activate`, `worktree.forceDeleteBranch`, ...) — chỉ `.extend()` tại `WorktreeRemove` để field `traceId` không lan sang các method không liên quan.

## File: `src/main/runtime/rpc/methods/git-remote.ts` [MODIFY — chỉ phần schema]

Thêm field `traceId` vào schema dùng chung cho `git.worktree.add`/`git.worktree.remove`:

```typescript
// ── Common schemas ─────────────────────────────────────────────────────────────
const ProjectWorktreeParam = z.object({
  projectId: z.string().min(1),
  worktreePath: z.string().min(1),
  traceId: OptionalString, // [NEW CR-TRACE-001], dùng bởi git.worktree.add/remove bên dưới
})
```

Task này chỉ thêm field vào schema — phần wire tracer thật sự vào handler `git.worktree.add`/`git.worktree.remove` thuộc TASK-BE-001.3.

## Verification

```bash
pnpm tsc --noEmit
pnpm test --run src/shared/trace/__tests__/tracers.test.ts
```

```bash
# Sau khi sửa xong, trước khi coi task DONE:
gitnexus_detect_changes()
```

Xác nhận chỉ các symbol/flow dự kiến bị ảnh hưởng — nếu detect_changes báo thêm symbol ngoài phạm vi task này, điều tra trước khi tiếp tục.

## Acceptance Criteria

- [ ] `Tracers.worktreeCreate`, `worktreeFanOut`, `worktreeDelete`, `worktreeCompare`, `worktreeMerge` tồn tại trong `src/shared/trace/tracers.ts` với đúng flow name `worktree:create|fanOut|delete|compare|merge`
- [ ] `WorktreeCreate`/`WorktreeRemove` (`worktree-schemas.ts`) có field `traceId?: string` optional, không phá vỡ backward compatibility
- [ ] `ProjectWorktreeParam` (`git-remote.ts`) có field `traceId?: string` optional
- [ ] Không có call site giả định nào được viết cho `worktree.fanOut`/`worktree.compare`/`worktree.merge` (các RPC method này chưa tồn tại)
- [ ] `pnpm tsc --noEmit` pass, không lỗi mới
