# FE-CV-TASK-053-06: `openReviewDiffAtSymbol` và mở trong editor

**From Solution:** [FE-CV-SOL-053-impact-lens-and-symbol-detail](../solutions/FE-CV-SOL-053-impact-lens-and-symbol-detail.md) mục 4.5
**Priority:** P0
**Area:** frontend / lib
**File:** `lib/review-diff-navigation.ts` (mới), test
**Depends on:** FE-CV-TASK-053-05, FE-CV-TASK-051-01
**Status:** [x] DONE (verified 2026-10-07: lib/review-diff-navigation.test 10/10 pass; O-13 synthetic compare for range/hostedReview still unverified)

## Context

- `openBranchDiff(worktreeId, worktreePath, entry, compare, language, options)` (`editor.ts:2627`), `openDiff(worktreeId, filePath, relativePath, language, staged, options)` (:2538), `gitBranchChangesByWorktree`, `gitStatusByWorktree`; mẫu `check-annotation-open.ts` (`openAnnotationLocation`: hai khung `requestAnimationFrame`); `shared/cross-platform-path.ts`; `findReusableRightSplitGroupId`.

## Việc cần làm

1. `openReviewDiffAtSymbol(worktreeId, ref, scope, {line?})`: chặn đường dẫn thoát worktree (`..`, tuyệt đối, `\`; lỗi inline); chọn bộ mở theo nơi thay đổi; `range/hostedReview` dùng compare tổng hợp **(chưa kiểm chứng; nếu không hỗ trợ ⇒ báo inline)**; file ngoài thay đổi ⇒ `openFile` + ghi chú.
2. Đặt `setPendingDiffReveal` sau hai khung; dùng nhóm tab đang hoạt động, hoặc nhóm phải đã tồn tại; không tự tạo split.
3. `openReviewFileInEditor` theo `openAnnotationLocation` (`setPendingEditorReveal`).
4. Hỗ trợ `line` tường minh (hunk đầu của bước ở SOL-052).

## Kiểm thử

- Chọn đúng bộ mở; đường dẫn thoát bị chặn; compare tổng hợp; `pendingDiffReveal` sau hai khung (`requestAnimationFrame` giả).

## Tiêu chí hoàn thành

- [ ] Không thoát worktree; không lệnh git mới.

## Rủi ro

- O-13: `git.branchDiff` với compare tổng hợp.
