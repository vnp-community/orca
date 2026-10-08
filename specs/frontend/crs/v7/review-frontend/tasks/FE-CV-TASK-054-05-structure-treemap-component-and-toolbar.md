# FE-CV-TASK-054-05: `StructureTreemap` và `StructureToolbar`

**From Solution:** [FE-CV-SOL-054-structure-lens](../solutions/FE-CV-SOL-054-structure-lens.md) mục 4.3
**Priority:** P1
**Area:** frontend / review-map
**File:** `StructureTreemap.tsx`, `StructureToolbar.tsx` (mới), tests
**Depends on:** FE-CV-TASK-054-01, 054-02, 054-03, FE-CV-TASK-053-01
**Status:** [x] DONE (verified 2026-10-07: structure/StructureTreemap.test 5/5 pass; color-mix() in SVG fill style not checked in Electron; drag-resize performance not measured)

## Context

- `overlaySvgProps` (053-01); `ResizeObserver`; `ui/toggle-group`, `ui/tooltip`; `color-mix(in srgb, var(--review-area-N) 28%, var(--card))` cho `fill` qua `style` (chưa kiểm Electron).

## Việc cần làm

1. SVG: ô = thư mục/file của mức hiện tại; tiêu đề ô ≥ 120×80 px; "+N nhỏ"; tooltip (đường dẫn, symbol, dòng, file đổi); bấm thư mục đi vào, file chọn, "+N nhỏ" mở cây; `Esc` lên một cấp; `role="img"` + `aria-label` tóm tắt.
2. Toolbar: Kích thước (Symbol|Dòng, "Dòng" vô hiệu khi thiếu `loc`), Màu (Khu vực|Ngôn ngữ), breadcrumb, chú giải (nhãn chữ + `ReviewOverlayLegend`).
3. Dưới 560 px: `ToggleGroup` Treemap|Cây.
4. Không chuyển tiếp (reduced-motion mặc định).

## Kiểm thử

- Số ô, nhãn khu vực, đổi Symbol/Dòng, bấm ô, breadcrumb, `Esc`.

## Tiêu chí hoàn thành

- [ ] Không hex; quét chuỗi test xanh.

## Rủi ro

- Hiệu năng khi kéo kích thước panel chưa đo.
