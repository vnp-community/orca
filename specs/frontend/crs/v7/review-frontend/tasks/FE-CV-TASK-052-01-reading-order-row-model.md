# FE-CV-TASK-052-01: Mô hình hàng thứ tự đọc (`reading-order-model`)

**From Solution:** [FE-CV-SOL-052-reading-order-and-progress](../solutions/FE-CV-SOL-052-reading-order-and-progress.md) mục 4.2
**Priority:** P0
**Area:** frontend / review-map
**File:** `components/review-map/reading-order-model.ts`, `reading-reason-labels.ts` (mới), tests
**Depends on:** FE-CV-TASK-050-01, FE-CV-TASK-051-04
**Status:** [x] DONE (verified 2026-10-07: reading-order-model.test 7 cases; reading-reason-labels covered by locale test)

## Context

- PQ-31 `ReadingStep`, `ComponentGroup`, `ReasonCode`; `ChangedFile.status`.

## Việc cần làm

1. `buildReadingOrderItems(steps, components, changedFiles)`; trùng `stepKey` giữ mục đầu; enum lạ ⇒ `unknown`.
2. `buildReadingOrderRows(items, {collapsedGroupIds, filter})` (hàng nhóm + bước, tiến độ nhóm); bước không có nhóm ⇒ "Khác" cuối; `filter` ẩn hàng nhưng không đổi tổng.
3. `reading-reason-labels.ts`: ánh xạ mã `ReasonCode` → khoá i18n; mã lạ nguyên văn.

## Kiểm thử

- Nhóm, thứ tự nhóm theo `n` nhỏ nhất, "Khác", trùng khoá, lọc, thu gọn, `cycleGroup`, `overflow`.

## Tiêu chí hoàn thành

- [ ] Hàm thuần, xác định; test xanh.

## Rủi ro

- `ComponentGroup.stepKeys` có thể trùng giữa nhóm: gán vào nhóm đầu.
