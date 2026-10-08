# FE-CV-TASK-052-04: `useRovingListKeys` (phím danh sách dùng chung)

**From Solution:** [FE-CV-SOL-052-reading-order-and-progress](../solutions/FE-CV-SOL-052-reading-order-and-progress.md) mục 4.4
**Priority:** P0
**Area:** frontend / review-map
**File:** `components/review-map/useRovingListKeys.ts` (mới), test `.test.tsx`
**Depends on:** không
**Status:** [x] DONE (verified 2026-10-07: useRovingListKeys.test 7 cases)

## Context

- `lib/editable-target.ts` (`isEditableTarget`); WAI-ARIA listbox; dùng lại ở SOL-056 (`DataFlowStepList`) và có thể ở SOL-053 (`ImpactColumnsList`).

## Việc cần làm

1. Hook nhận `{count, activeIndex, onActiveChange, onActivate (Enter), onToggle (Space)?, isGroupRow?, onCollapse?}` và trả `onKeyDown`, `getItemProps`.
2. Phím: `j/↓`, `k/↑`, `Home/End`, `Enter`, `Space`, `←/→` cho hàng nhóm; bỏ qua khi `isEditableTarget(event.target)`; `preventDefault` đúng lúc (Space không cuộn).
3. Không phím nền tảng-phụ thuộc.

## Kiểm thử

- Mô phỏng phím; bỏ qua `input`; bỏ qua hàng nhóm thu gọn; biên đầu/cuối.

## Tiêu chí hoàn thành

- [ ] Test xanh; không phụ thuộc component cụ thể.

## Rủi ro

- Ảo hoá làm hàng đang chọn chưa mount: caller cuộn bằng `scrollToIndex`.
