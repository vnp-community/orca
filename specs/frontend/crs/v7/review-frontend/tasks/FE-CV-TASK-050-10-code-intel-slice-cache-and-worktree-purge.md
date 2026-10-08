# FE-CV-TASK-050-10: Slice `code-intel`, cache LRU, dọn rò rỉ hai đường

**From Solution:** [FE-CV-SOL-050-store-and-query-hooks](../solutions/FE-CV-SOL-050-store-and-query-hooks.md) mục 4.3
**Priority:** P0
**Area:** frontend / store
**File:** `frontend/src/renderer/src/store/slices/code-intel.ts` (mới), `store/index.ts`, `store/types.ts`, `store/slices/store-test-helpers.ts`, `store/slices/worktrees.ts` (sửa), tests `code-intel.test.ts`, `code-intel-worktree-removal-leak.test.ts`, `code-intel-bulk-purge-leak.test.ts`
**Depends on:** FE-CV-TASK-050-01, FE-CV-TASK-050-07
**Status:** [x] DONE (verified 2026-10-07: code-intel.test + 2 leak tests pass; slice was never registered in store/index.ts/types.ts and the purge paths ignored it: wired + purge added to buildWorktreePurgeState and removeWorktree)

## Context

- Đăng ký ở `store/index.ts` (:55-57, :124-126) và `types.ts` (:53-55, :119-121); `store-test-helpers.ts` `createTestStore` (:61) liệt kê từng slice.
- Hai đường xoá worktree: `removeWorktree` (`worktrees.ts:3725-3742`, optional chain) và `buildWorktreePurgeState` (:1960; gọi :2454, :2519, :2593, :4988; mẫu lọc map :2056-2062, :2262).
- Mẫu test: `generation-records-worktree-removal-leak.test.ts`, `bulk-worktree-purge-terminal-maps-leak.test.ts`. Chạy `impact` GitNexus trên `buildWorktreePurgeState`, `removeWorktree` trước khi sửa (chưa chạy).

## Việc cần làm

1. Slice theo SOL mục 4.3; action trả object một phần (không mutate).
2. Cache `codeIntelResultsByWorktree`: LRU 16 mục/worktree, giữ ≤ 6 worktree (loại cũ nhất).
3. `CODE_INTEL_WORKTREE_KEYED_STATE_KEYS` (export) và `pruneCodeIntelWorktrees(liveIds)`; huỷ timer/hàng chờ module-level theo worktree.
4. Gọi prune ở **cả hai** đường; thêm slice vào `store-test-helpers.ts` và `test-support/code-intel-test-store.ts`.
5. `invalidateCodeIntelWorktree`: xoá cache + đặt `codeIntelStaleSignalByWorktree`, không tăng counter.

## Kiểm thử

- `code-intel.test.ts`: LRU 16/6, invalidate, selector.
- Hai test rò rỉ: gieo từng khoá của hằng cho 2 worktree, xoá một bằng mỗi đường, chỉ khoá của worktree bị xoá biến mất; test duyệt chính hằng.

## Tiêu chí hoàn thành

- [ ] Test rò rỉ xanh cả hai đường; test cũ của worktrees không đỏ.

## Rủi ro

- `worktrees.ts` rất lớn: sửa tối thiểu; một số assembly test thiếu slice ⇒ dùng optional chain.
