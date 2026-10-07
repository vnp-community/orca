# FE-CV-TASK-055-01: `c4-layer-layout`, `c4-edge-style`, `c4-overlay-model`

**From Solution:** [FE-CV-SOL-055-architecture-c4-lens](../solutions/FE-CV-SOL-055-architecture-c4-lens.md) mục 4.2, 4.4
**Priority:** P1
**Area:** frontend / review-map
**File:** `c4-layer-layout.ts`, `c4-edge-style.ts`, `c4-overlay-model.ts` (mới), tests
**Depends on:** FE-CV-TASK-050-01, FE-CV-TASK-053-01
**Status:** [x] DONE

## Context

- `C4Component.kind`, `C4Relation.kind` (7), `count`, `confidence`, `violatesLayering`, `evidence: SymbolRef[]`; PQ-29 tiền tố `C4*`.

## Việc cần làm

1. `layoutC4Layers(view)` ⇒ nút + dải: cột theo `kind` (bảng SOL), externals cột 4, `config/other` bên dưới; dải trống bỏ; quan hệ tới nút không có bị bỏ (không ném); xác định.
2. `c4EdgeStyle(relation)`: `strokeWidth=clamp(1+log2(count),1,5)`; bảng 7 loại ⇒ `strokeDasharray`; `confidence<0.8` nét nhạt; bảng duy nhất để dựng `C4EdgeLegend`.
3. `computeC4OverlayFlags(component, overlay, impact?)`: `changed` theo tiền tố `path`/`packagePaths` (chuẩn hoá phân cách); `untested`; `violation`; `affected` chỉ khi có `impact`; cạnh giao `evidence[].key` với `changedSymbols`.

## Kiểm thử

- Ánh xạ kind→cột; dải trống; 150 nút; `count` 1/2/8/1000; Windows/POSIX; thiếu `affected`.

## Tiêu chí hoàn thành

- [ ] Thuần; test xanh; không hex.

## Rủi ro

- Container không hexagonal: phần lớn component rơi vào `other` (đo ở 055-07).
