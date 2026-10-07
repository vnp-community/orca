# FE-CV-TASK-056-06: `DataFlowDiagram`, `DataFlowStepList`, `DataFlowToolbar`

**From Solution:** [FE-CV-SOL-056-dataflow-lens](../solutions/FE-CV-SOL-056-dataflow-lens.md) mục 4.3, 4.4
**Priority:** P1
**Area:** frontend / review-map
**File:** `DataFlowLens.tsx`, `DataFlowDetailPane.tsx`, `DataFlowToolbar.tsx`, `DataFlowDiagram.tsx`, `DataFlowStepList.tsx`, `DataFlowStepDetail.tsx` (mới), tests
**Depends on:** FE-CV-TASK-056-01, 056-02, 056-03, 056-04, 056-05, FE-CV-TASK-052-04, FE-CV-TASK-053-04
**Status:** [x] DONE

## Context

- `MermaidBlock` (không sửa); `ui/table.tsx`; `useRovingListKeys`; `SymbolDetailPanel` (053-04).

## Việc cần làm

1. `DataFlowDiagram`: bọc `MermaidBlock` (`isDark` từ `useIsDarkTheme`, `htmlLabels={false}`), thu phóng CSS 50-200 %, `role="img"` + `aria-label`, `Skeleton` giữ chỗ; quá lớn ⇒ lý do + chỉ danh sách; cắt 60 ⇒ "{r}/{t}"; lỗi cú pháp ⇒ khung lỗi của `MermaidBlock` + "Sao chép nguồn".
2. `DataFlowStepList`: bảng cột như SOL 4.4; hàng đã đổi, `unimplemented`, `gap`, bước ngoài sơ đồ mờ; Enter/bấm ⇒ `SymbolDetailPanel` hoặc `DataFlowStepDetail`; ảo hoá > 150; chip phím bằng `ShortcutKeyCombo`.
3. Toolbar: tên, trigger, công tắc Dịch vụ|Thành phần, Sao chép, Xuất ▾, thu phóng, chú giải; banner `partial`.
4. Nhãn "Suy luận" khi `origin` ≠ `declared`; chuỗi backend văn bản thuần.

## Kiểm thử

- `DataFlowLens.test.tsx` (mock `MermaidBlock` ghi lại `content`), `DataFlowStepList.test.tsx` (phím, ảo hoá, gap); sao chép mock clipboard.

## Tiêu chí hoàn thành

- [ ] Không hex; không sửa `MermaidBlock.tsx`.

## Rủi ro

- Hàng đợi render Mermaid toàn cục: đổi luồng nhanh có thể hiện kết quả cũ chốc lát.
