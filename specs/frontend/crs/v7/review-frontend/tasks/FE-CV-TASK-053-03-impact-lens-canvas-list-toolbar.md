# FE-CV-TASK-053-03: `ImpactLens`, canvas xyflow, danh sách, thanh công cụ

**From Solution:** [FE-CV-SOL-053-impact-lens-and-symbol-detail](../solutions/FE-CV-SOL-053-impact-lens-and-symbol-detail.md) mục 4.2
**Priority:** P0
**Area:** frontend / review-map
**File:** `ImpactLens.tsx`, `ImpactToolbar.tsx`, `ImpactGraphCanvas.tsx`, `ImpactSymbolNode.tsx`, `ImpactColumnsList.tsx`, `store/slices/review-ui.ts` (thêm `impactFocusKey`), tests
**Depends on:** FE-CV-TASK-053-01, 053-02, 051-05, 052-04, FE-CV-TASK-050-13
**Status:** [ ] TODO

## Context

- `@xyflow/react` ^12.11.2 (mẫu `TaskDAGView.tsx`: import `@xyflow/react/dist/style.css`; hex ở đó **không** dùng). Mock xyflow như `TaskDAGView.test.tsx`.
- `impact` params §3.1; `usePerceivedLoadingStage` (051-02); ambiguous ⇒ `AmbiguousSymbolDialog` (051-06).

## Việc cần làm

1. `ImpactLens(ReviewLensProps)`: tâm, hai truy vấn song song theo hướng bật, `depth` 1..3, `includeTests:false`; trạng thái rỗng/tải/lỗi riêng (`timeout`, `too-large`, `tool-failed`) có Thử lại; "Không tìm thấy phụ thuộc trong index" (không "không ảnh hưởng gì").
2. Canvas lazy (`React.lazy`), `nodesDraggable=false`, `onlyRenderVisibleElements`, `colorMode` theo chủ đề, `MiniMap` > 80 nút, `fitView duration:0` khi reduced-motion; nút: icon theo `kind`, `file:dòng`, cờ lớp phủ, "trực tiếp"; menu ngữ cảnh: Xem diff, Đặt làm trung tâm, Mở trong editor, Sao chép khoá.
3. Dòng "Dữ liệu chưa có thông tin cạnh; chỉ hiển thị theo tầng" luôn hiện; `ImpactColumnsList` mặc định khi > 300 nút; `risk` hiển thị.
4. Chọn nút ⇒ `onSelectSymbol`; nhấp đúp/Enter ⇒ `onOpenDiff`.

## Kiểm thử

- Tâm ở giữa, hướng đúng, đổi hướng/độ sâu gọi lại; không cạnh giả; danh sách thay thế; ambiguous; lỗi từng truy vấn.

## Tiêu chí hoàn thành

- [ ] Không hex/lớp màu thô; kiểm tay `var()` sáng/tối.

## Rủi ro

- Phím Space/Enter trong nút xyflow xung đột phím danh sách (chưa kiểm).
