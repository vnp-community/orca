# FE-CV-TASK-087-19: `QualityDependencyPanel`: ma trận phụ thuộc (DSM)

**From Solution:** [FE-CV-SOL-087-quality-trend-coverage-hotspot](../solutions/FE-CV-SOL-087-quality-trend-coverage-hotspot.md) mục 2.2
**Priority:** P2
**Area:** frontend / components (pha 3)
**File:** `frontend/src/renderer/src/components/review-map/quality/QualityDependencyPanel.tsx` (mới) và `*.test.tsx`
**Depends on:** 087-15, FE-CV-TASK-088-05, 088-08; FE-CV-SOL-054-structure-lens (chung cache `structure`)
**Status:** [ ] TODO

## Context

- `ModuleGraph` (§4.2): `imports` có `count`; `structure` có `truncated`. PQ-10: `architecture` là C4, không dùng.
- DSM ≤ 60 nút; trên ~300 nút, DSM là cách trình bày chính (CR-088 2.2-B).
- Quy ước đọc (hàng = nguồn, cột = đích) phải ở `description`.

## Việc cần làm

1. `ChartFrame` + `DependencyMatrix` với dữ liệu từ `quality-dependency-graph-reduction`; `description` quy ước đọc và khối vòng.
2. `truncated`/gộp "Khác" → "Đang hiển thị X/Y".
3. Bấm ô → panel liệt kê cạnh (từ/đến/`count`) và nút "Mở trong lens Cấu trúc" nếu lens tồn tại.
4. Đồ thị rỗng → "Chưa có dữ liệu phụ thuộc" + lý do.

## Kiểm thử

Đồ thị có vòng (khối đậm + ký hiệu), DAG, rỗng, 61 nút; bấm ô liệt kê cạnh. Chạy `pnpm --filter orca-frontend test -- src/renderer/src/components/review-map/quality/QualityDependencyPanel`.

## Tiêu chí hoàn thành

- [ ] Vòng đánh dấu không chỉ màu.
- [ ] Không gọi `structure` khi khối đóng.

## Rủi ro

- Thứ tự DSM có thể kém đọc với đồ thị đầy; cần thử dữ liệu Orca.
