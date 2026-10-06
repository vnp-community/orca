# FE-CV-TASK-052-05: `ReadingOrderList`, hàng, nhóm, thanh tiến độ

**From Solution:** [FE-CV-SOL-052-reading-order-and-progress](../solutions/FE-CV-SOL-052-reading-order-and-progress.md) mục 4.1, 4.4
**Priority:** P0
**Area:** frontend / review-map
**File:** `ReadingOrderList.tsx`, `ReadingOrderRow.tsx`, `ReadingOrderGroupHeader.tsx`, `ReadingProgressBar.tsx` (mới), tests
**Depends on:** FE-CV-TASK-052-01, 052-03, 052-04, FE-CV-TASK-051-05, FE-CV-TASK-053-01
**Status:** [ ] TODO

## Context

- Mẫu `useVirtualizer` ở `CsvViewer.tsx`; token git `--git-decoration-*` (STYLEGUIDE "Git decoration colors", chỉ cho trạng thái git); cờ lớp phủ từ `review-overlay-model` (053-01), chấm/icon + tooltip.

## Việc cần làm

1. Danh sách `role="listbox"`; hàng: ô tick (`aria-label`), `n`, icon, tên file + số symbol, A/M/D, nhãn `reason`, cờ lớp phủ, nút mở rộng hiện `symbols`/`tests`; bấm chọn, bấm đúp hoặc nút "Xem diff" mở diff tại `hunks[0].startLine` qua `onOpenDiff`.
2. `ReadingProgressBar`: `seen/total`, `Progress`, trạng thái `saved|dirty|saving|error` ("Chưa lưu" + Thử lại), `aria-live="polite"` sau 500 ms.
3. Ảo hoá khi > 150 hàng; tiêu điểm mặc định `lastFocusedKey` hoặc bước chưa xem đầu (chỉ khi mở có chủ đích).
4. `ReadingOrderFilterNotice` ("Lọc: … ✕", "Đang lọc k/N").

## Kiểm thử

- `ReadingOrderList.test.tsx` (`// @vitest-environment happy-dom`): phím, `aria-selected`, thu gọn bằng phím, ảo hoá 500 mục, bỏ qua ô nhập; `ReadingProgressBar.test.tsx`.

## Tiêu chí hoàn thành

- [ ] Hành vi như tiêu chí SOL mục 7; không hex.

## Rủi ro

- `aria-activedescendant` + ảo hoá chưa kiểm với trình đọc màn hình.
