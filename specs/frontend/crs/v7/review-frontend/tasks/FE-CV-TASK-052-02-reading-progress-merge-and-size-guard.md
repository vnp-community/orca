# FE-CV-TASK-052-02: `mergeReadingProgress` và chặn kích thước 64 KiB

**From Solution:** [FE-CV-SOL-052-reading-order-and-progress](../solutions/FE-CV-SOL-052-reading-order-and-progress.md) mục 1 (dòng 7, 8), 4.3
**Priority:** P0
**Area:** frontend / review-map
**File:** `components/review-map/reading-progress-merge.ts` (mới), test
**Depends on:** FE-CV-TASK-050-01
**Status:** [ ] TODO

## Context

- `ReadingProgress {version:1, entries: Record<stepKey,{state,at}>, lastFocusedKey}`; giới hạn ≤ 64 KiB.

## Việc cần làm

1. `mergeReadingProgress(local, remote)`: từng khoá chọn `at` lớn hơn; `lastFocusedKey` từ bên mới hơn; thuần, không mutate.
2. `pruneReadingProgress(progress, maxBytes=57344)`: giữ mọi `seen`, bỏ tombstone `unseen` cũ nhất tới khi vừa; trả `{progress, droppedUnseen, stillTooLarge}`.
3. `measureReadingProgressBytes` bằng `TextEncoder`.

## Kiểm thử

- LWW; `unseen` mới thắng `seen` cũ; tỉa; ký tự nhiều byte; `stillTooLarge`.

## Tiêu chí hoàn thành

- [ ] Xác định, không ném; test xanh.

## Rủi ro

- Lệch đồng hồ client đảo thứ tự LWW (chấp nhận).
