# FE-CV-TASK-051-01: Slice `review-ui` và mô hình phạm vi `review-scope-model`

**From Solution:** [FE-CV-SOL-051-review-workspace-shell](../solutions/FE-CV-SOL-051-review-workspace-shell.md) mục 4.5
**Priority:** P0
**Area:** frontend / store + review-map
**File:** `store/slices/review-ui.ts` (mới), `components/review-map/review-scope-model.ts` (mới), `store/index.ts`, `store/types.ts`, `store-test-helpers.ts`, tests
**Depends on:** FE-CV-TASK-050-10
**Status:** [x] DONE

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
