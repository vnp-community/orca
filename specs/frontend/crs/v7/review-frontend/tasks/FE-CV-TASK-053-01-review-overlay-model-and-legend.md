# FE-CV-TASK-053-01: `review-overlay-model` và `ReviewOverlayLegend`

**From Solution:** [FE-CV-SOL-053-impact-lens-and-symbol-detail](../solutions/FE-CV-SOL-053-impact-lens-and-symbol-detail.md) mục 4.3
**Priority:** P0
**Area:** frontend / review-map
**File:** `components/review-map/review-overlay-model.ts`, `ReviewOverlayLegend.tsx` (mới), tests
**Depends on:** FE-CV-TASK-050-19, FE-CV-TASK-050-01
**Status:** [ ] TODO

## Context

- Bảng mã hoá duy nhất cho mọi lens (Ảnh hưởng, Cấu trúc, C4, Luồng, ERD, cột trái). Dữ liệu: `ChangedSymbol.tested`, `uncoveredSymbols`, `ViolationRef` (theo file).

## Việc cần làm

1. `OVERLAY_ENCODING: Record<OverlayFlag, {labelKey; tokenVar; icon; strokeWidth; dashed}>`.
2. `computeOverlayFlags({symbolKey?, file?}, overlay, impact?)`: `changed`, `affected` (chỉ khi có dữ liệu `impact`), `untested` (`tested==='no'`), `violation` (theo file).
3. `overlayClassNames(flags)` (class dạng `border-[color:var(--review-changed)]`), `overlaySvgProps(flags)` (`stroke`, `strokeWidth`, `strokeDasharray`); ưu tiên `changed > affected`, `untested` chỉ thêm nét đứt.
4. `ReviewOverlayLegend` dựng từ chính bảng, chỉ cờ có dữ liệu, `Tooltip` giải nghĩa, swatch `aria-hidden`, nhãn chữ "Chưa tìm thấy test phủ trong index".

## Kiểm thử

- Tổ hợp cờ; không chứa hex (`/#[0-9a-f]{3,8}/i`); legend sinh từ cùng bảng; `unknown` không nét đứt.

## Tiêu chí hoàn thành

- [ ] Một nguồn duy nhất; test xanh.

## Rủi ro

- `var()` trong SVG chưa kiểm chứng (xem 053-03).
