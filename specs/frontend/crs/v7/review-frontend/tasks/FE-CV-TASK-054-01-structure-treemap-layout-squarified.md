# FE-CV-TASK-054-01: `layoutSquarifiedTreemap`

**From Solution:** [FE-CV-SOL-054-structure-lens](../solutions/FE-CV-SOL-054-structure-lens.md) mục 4.3
**Priority:** P1
**Area:** frontend / review-map
**File:** `components/review-map/structure-treemap-layout.ts` (mới), test
**Depends on:** không
**Status:** [x] DONE (verified 2026-10-07: structure/structure-treemap-layout.test 7/7 pass; reuses quality-charts `squarify`, status-bar bisecting layout not used)

## Context

- Đã có `components/status-bar/workspace-space-layout.ts` (`buildTreemapLayout`, chia đôi cân bằng, bounds 0..100): so sánh tỷ lệ cạnh trước; không sửa file cũ.

## Việc cần làm

1. `layoutSquarifiedTreemap(items: {id; value}[], rect: {x,y,w,h}, {padding}) ⇒ {id,x,y,w,h}[]`; sắp giảm dần theo `value`, khoá phụ `id`; bỏ `value ≤ 0`.
2. Một mức; ≤ 400 ô; ô < 24×16 px hoặc ngoài top 400 gộp thành ô tổng "+N nhỏ" (id đặc biệt).
3. So sánh tỷ lệ cạnh trung bình với `buildTreemapLayout`; ghi kết quả vào PR (quyết định dùng lại hay giữ riêng).

## Kiểm thử

- Tổng diện tích = khung (< 0,5 %), không chồng, xác định, giá trị 0, một phần tử, padding, 5 000 phần tử không ném (ngưỡng thời gian rộng để bắt hồi quy lớn).

## Tiêu chí hoàn thành

- [ ] Hàm thuần, test xanh.

## Rủi ro

- Sai số làm tròn ở ô cuối: dồn phần dư vào ô cuối.
