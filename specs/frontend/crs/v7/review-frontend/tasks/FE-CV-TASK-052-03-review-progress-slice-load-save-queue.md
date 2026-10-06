# FE-CV-TASK-052-03: Slice `review-progress`: tải `reviewState.get`, ghi tuần tự `reviewState.save`

**From Solution:** [FE-CV-SOL-052-reading-order-and-progress](../solutions/FE-CV-SOL-052-reading-order-and-progress.md) mục 4.3
**Priority:** P0
**Area:** frontend / store
**File:** `store/slices/review-progress.ts` (mới), `store/index.ts`, `types.ts`, `store-test-helpers.ts`, tests `review-progress.test.ts`, `review-progress-worktree-removal-leak.test.ts`, `review-progress-bulk-purge-leak.test.ts`
**Depends on:** FE-CV-TASK-052-02, FE-CV-TASK-050-10, FE-CV-TASK-050-13, FE-CV-TASK-051-01
**Status:** [ ] TODO

## Context

- Mẫu hàng chờ: `store/slices/diff-comments-persist-queue.ts` (**đọc trước khi cài**). `reviewState.get` (chưa có ⇒ `version:0`), `save` (`expectedVersion`; `0` = tạo).
- Sẽ được SOL-060 dùng chung để ghi `notes`; thiết kế một writer.

## Việc cần làm

1. State và action theo SOL 4.3; `loadReviewProgress(worktreeId, base, head)`; giữ toàn bộ `ReviewState` (kể cả `notes`, `status`).
2. `setReadingItemSeen`, `setReadingGroupSeen`, `setReadingLastFocused`, `flushReviewProgress`; debounce 800 ms; hàng chờ một chạy + một chờ.
3. Conflict: `get` → `mergeReadingProgress` → ghi lại; offline → `dirty`, tự thử khi `established` hoặc 15 s; `forbidden` → `error` + cờ chỉ-cục-bộ; prune 64 KiB trước khi gửi.
4. Hook mở rộng cho ghi chú: `patchReviewState(worktreeId, patch)` (SOL-060 dùng) đi cùng hàng chờ.
5. Thêm khoá vào `CODE_INTEL_WORKTREE_KEYED_STATE_KEYS`; huỷ timer khi dọn.

## Kiểm thử

- Đồng hồ giả: debounce; nhiều thao tác ⇒ tối đa 1 chạy + 1 chờ; flush; conflict; offline; forbidden; `head` đổi ⇒ trống; hai test rò rỉ.

## Tiêu chí hoàn thành

- [ ] Không "Đã lưu" trước khi `save` thành công; rò rỉ hai đường xanh.

## Rủi ro

- Ngữ nghĩa `notes` vắng chưa chốt (SOL câu hỏi 1): luôn gửi lại bản đã biết.
