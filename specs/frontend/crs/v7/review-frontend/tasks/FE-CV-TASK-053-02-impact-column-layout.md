# FE-CV-TASK-053-02: `layoutImpactColumns` (cột theo tầng, không cạnh)

**From Solution:** [FE-CV-SOL-053-impact-lens-and-symbol-detail](../solutions/FE-CV-SOL-053-impact-lens-and-symbol-detail.md) mục 4.2
**Priority:** P0
**Area:** frontend / review-map
**File:** `components/review-map/impact-column-layout.ts` (mới), test
**Depends on:** FE-CV-TASK-050-01
**Status:** [x] DONE (verified 2026-10-07: impact/impact-column-layout.test 7/7 pass)

## Context

- `ImpactGraph.levels[{depth, symbols[{symbol, via, confidence?, direct}]}]`; không cạnh (PQ-19(5)); ≤ 300 nút mỗi hướng; tổng ≤ 1 500.

## Việc cần làm

1. `layoutImpactColumns(upstream, downstream, {rowHeight:56, columnWidth:260, columnGap:80, maxPerColumn:60, expandedColumns})` ⇒ `{nodes, edges, columns, hiddenCountByColumn}`.
2. Cột `-d`/`0`/`+d`; sắp trong cột theo thư mục (hai đoạn cuối `filePath`) rồi tên (ổn định); nút trùng giữa hai hướng giữ ở mỗi cột; cạnh chỉ tâm↔`direct`.
3. Cột > 60 nút ⇒ 60 + nút "+N khác" (mở rộng); không ném khi 1 500 nút; `via` hiển thị trong tooltip.

## Kiểm thử

- Chỉ số cột, sắp ổn định, giới hạn 60, không cạnh giả cho tầng 2+, trùng nút, 1 500 nút.

## Tiêu chí hoàn thành

- [ ] Hàm thuần xác định; test xanh.

## Rủi ro

- Nếu BE thêm `edges` sau này, mở rộng tham số, không đổi chữ ký hiện tại.
