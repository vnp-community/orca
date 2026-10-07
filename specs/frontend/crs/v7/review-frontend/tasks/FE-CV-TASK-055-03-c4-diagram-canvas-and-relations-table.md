# FE-CV-TASK-055-03: `C4DiagramCanvas` và `C4RelationsTable`

**From Solution:** [FE-CV-SOL-055-architecture-c4-lens](../solutions/FE-CV-SOL-055-architecture-c4-lens.md) mục 4.2
**Priority:** P1
**Area:** frontend / review-map
**File:** `ArchitectureLens.tsx`, `ArchitectureToolbar.tsx`, `C4ContainerPicker.tsx`, `C4DiagramCanvas.tsx`, `C4ComponentNode.tsx`, `C4ExternalNode.tsx`, `C4LayerBand.tsx`, `C4RelationsTable.tsx` (mới), tests
**Depends on:** FE-CV-TASK-055-01, 055-02, FE-CV-TASK-053-01
**Status:** [x] DONE

## Context

- `@xyflow/react` (mock như `TaskDAGView.test.tsx`); `ui/table.tsx`; `onlyRenderVisibleElements`; không hex.

## Việc cần làm

1. Canvas lazy: `nodesDraggable=false`, `fitView duration:0` khi reduced-motion, `colorMode`, `Controls`; nút: tên, icon theo `kind`, `symbolCount`, `techHint`, cờ lớp phủ, nhãn `origin`; `C4LayerBand` không chọn.
2. `C4RelationsTable`: Từ, Loại, Tới, Số lần, cờ; mặc định khi > 150 nút/> 500 quan hệ hoặc người dùng chọn; "Đang hiển thị {n}/{total}".
3. Toolbar: container, Đồ thị|Danh sách, "Hiện thành phần ẩn", "Chỉnh c4.yaml", chú giải; hiển thị `view.warnings`.
4. Trạng thái: không container ("Chưa suy ra container nào; kiểm tra quy ước thư mục"), `view===null`, lỗi truy vấn inline + Thử lại.

## Kiểm thử

- Hiển thị cột; chọn nút/cạnh; chuyển Danh sách; trạng thái.

## Tiêu chí hoàn thành

- [ ] Hành vi như SOL mục 7 (phần sơ đồ).

## Rủi ro

- `var()` trong SVG xyflow chưa kiểm.
