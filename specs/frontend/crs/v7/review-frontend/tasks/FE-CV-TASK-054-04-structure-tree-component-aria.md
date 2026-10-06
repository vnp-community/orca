# FE-CV-TASK-054-04: `StructureTree` (WAI-ARIA Tree, ảo hoá)

**From Solution:** [FE-CV-SOL-054-structure-lens](../solutions/FE-CV-SOL-054-structure-lens.md) mục 4.4
**Priority:** P1
**Area:** frontend / review-map
**File:** `components/review-map/StructureTree.tsx` (mới), test
**Depends on:** FE-CV-TASK-054-02, 054-03
**Status:** [ ] TODO

## Context

- Mẫu `useVirtualizer` ở `CsvViewer.tsx`; STYLEGUIDE "List rows: hover, selected, current" (`bg-accent`, `data-current="true"`); `isEditableTarget`.

## Việc cần làm

1. `role="tree"`/`treeitem` + `aria-level/expanded/setsize/posinset/selected`; tiêu điểm lăn.
2. Phím `↑↓`, `→` (mở/vào con), `←` (đóng/lên cha), `Home/End`, `Enter` (thư mục: đi vào treemap; file: chọn); không `*`; bỏ qua ô nhập.
3. Hàng: thụt lề, chevron, icon `Folder`/`File`, tên cắt giữa, `symbolCount` (`tabular-nums`), chấm đổi + số; `useVirtualizer` 28 px, `overscan` 12, `getItemKey=path`.
4. Hàng lỗi + "Thử lại"; `{shown}/{total}` + "Tải thêm".

## Kiểm thử

- Phím, `aria-*`, số hàng DOM < tổng (10 000), mở nhanh một lần.

## Tiêu chí hoàn thành

- [ ] Hành vi Tree chuẩn; không hex.

## Rủi ro

- `aria-activedescendant` với ảo hoá chưa kiểm.
