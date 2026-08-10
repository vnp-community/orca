# TASK-BIGFILE-040 — Move (composition): Resolved-worktree cache/lineage domain

**Loại:** Move — composition pattern (KHÔNG phải barrel, class có `this`)
· **Effort:** S · **Phụ thuộc:** TASK-BIGFILE-008, 009
**Status:** ⬜ Todo
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-002-orca-runtime.md` (Giai
đoạn 3) · Sinh ra từ `TASK-BIGFILE-035`

## Input

- File nguồn: `frontend/src/main/runtime/orca-runtime.ts`
- Đọc **đúng dòng 19,655–19,910** (KHÔNG đọc phần khác của file).
- Method cần chuyển (9 method, xác nhận lại khi đọc):
  `listResolvedWorktrees`, `computeResolvedWorktrees`,
  `attachLineageToResolvedWorktrees`, `pruneLineageForMissingRepoWorktrees`,
  `listRepoWorktreesForResolution`, `listStoredSshWorktreesForResolution`,
  `getResolvedWorktreeMap`, `invalidateResolvedWorktreeCache`,
  `notifyBranchRenamed`
- Field private cần: `resolvedWorktreeCache`, `resolvedWorktreeInFlight`,
  `resolvedWorktreeGeneration`.
- Type liên quan: `ResolvedWorktree` — **LƯU Ý**: type này được GIỮ LẠI ở
  `orca-runtime.ts` (export type, không di chuyển — quyết định từ
  TASK-008 vì dùng chéo pervasive, xem
  `TASK-BIGFILE-008-orca-runtime-tail-buffer.md`). Domain mới import type
  này từ `orca-runtime.ts`, KHÔNG từ `orca-runtime-types.ts`.

## Output

- File mới:
  `frontend/src/main/runtime/orca-runtime-resolved-worktree-cache.ts` —
  class mới (ví dụ `ResolvedWorktreeCacheDomain`) nhận dependency qua
  constructor.
- `orca-runtime.ts`: thêm field `private resolvedWorktrees = new
  ResolvedWorktreeCacheDomain({ ... })`, 9 method forward — GIỮ NGUYÊN
  chữ ký.

## Các bước

1. `gitnexus impact({target: "listResolvedWorktrees", direction: "upstream"})`
   — dừng nếu risk HIGH/CRITICAL.
2. Đọc dòng 19,655–19,910, xác nhận method + field.
3. Tạo file mới, copy nguyên văn, đổi `this.xxx` → `this.deps.xxx`. Import
   `type { ResolvedWorktree } from './orca-runtime'` (type-only, không
   circular runtime — cùng pattern đã dùng ở `orca-runtime-tail-buffer.ts`).
4. Sửa `orca-runtime.ts`: thêm field, forward 9 method.

## Xác minh xong

- [ ] `pnpm run typecheck && pnpm run lint`
- [ ] `gitnexus detect_changes({scope: "all"})` — risk low
- [ ] `node scripts/find-frontend-bigfiles.mjs` — giảm ~180-250 dòng
- [ ] Test worktree-listing/lineage liên quan pass

## Rollback

```
git checkout -- frontend/src/main/runtime/orca-runtime.ts
rm frontend/src/main/runtime/orca-runtime-resolved-worktree-cache.ts
```
