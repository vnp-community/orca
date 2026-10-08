# FE-CV-TASK-051-01: Slice `review-ui` và mô hình phạm vi `review-scope-model`

**From Solution:** [FE-CV-SOL-051-review-workspace-shell](../solutions/FE-CV-SOL-051-review-workspace-shell.md) mục 4.5
**Priority:** P0
**Area:** frontend / store + review-map
**File:** `store/slices/review-ui.ts` (mới), `components/review-map/review-scope-model.ts` (mới), `store/index.ts`, `store/types.ts`, `store-test-helpers.ts`, tests
**Depends on:** FE-CV-TASK-050-10
**Status:** [x] DONE (verified 2026-10-08: use-review-branch-compare-fetch.test 4/4, review-scope-model.test PASS, ReviewWorkspace.companions.test 9/9 + ReviewWorkspace.test 13/13 PASS)

## Context

- `ReviewUiState {scope, lens, chipFilter, selectedSymbolKey, scopePickerOpen}`; solution sau thêm `impactFocusKey` (053), `c4ContainerId/c4Drafts` (055), `dataFlowId` (056).
- `GitBranchCompareSummary` (`shared/types.ts:3793`), `gitBranchCompareSummaryByWorktree` (`editor.ts:696`).
- `ChangeOverlay.scope` trả `{baseRef, baseOid, mergeBase, headOid, mode, includesUncommitted}` (§4.3).

## Việc cần làm

1. Slice `reviewUiByWorktree`; đổi `scope` xoá `chipFilter` và `selectedSymbolKey`; thêm khoá vào `CODE_INTEL_WORKTREE_KEYED_STATE_KEYS`; đăng ký ở ba nơi.
2. `resolveDefaultReviewScope`, `toChangeOverlayParams` (gửi `mode`), `scopeKey` (dùng `overlay.scope` khi có).
3. Nếu chưa có summary: gọi `git.branchCompare` qua `runtime-git-client` rồi `setGitBranchCompareResult` (không lệnh git mới).

## Kiểm thử

- `review-scope-model.test.ts` (mặc định từ từng `status`, `scopeKey` ổn định, ba loại); `review-ui.test.ts`; hai test rò rỉ (mẫu 050-10).

## Tiêu chí hoàn thành

- [ ] `scopeKey` chứa commit đã phân giải; rò rỉ hai đường xanh.

## Rủi ro

- O-13 (ngữ nghĩa `head` vắng) và compare tổng hợp cho `range`: ghi chưa kiểm chứng.

## Ghi chú hoàn thiện (2026-10-08, P4)

- Mục 3: `shell/use-review-branch-compare-fetch.ts` — khi chưa có summary, gọi `getRuntimeGitBranchCompare` (runtime-git-client: IPC local / SSH qua `getConnectionId` / runtime từ xa, định tuyến theo chủ repo `getRepoOwnerRoutedSettings`) rồi `setGitBranchCompareResult`; không thêm lệnh git mới.
- Base = `worktree.baseRef` rồi `repo.worktreeBaseRef`; không có base thì không gọi. Một lần mỗi (worktree, base) mỗi lần mount.
- `beginGitBranchCompareRequest(..., { preserveExistingSummary: true })` để không chèn summary `loading` (giữ phạm vi mặc định từ base đã ghim); lỗi không ghi summary `error` (Source Control báo lỗi git).
