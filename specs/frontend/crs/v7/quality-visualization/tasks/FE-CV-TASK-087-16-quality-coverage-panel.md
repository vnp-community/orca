# FE-CV-TASK-087-16: `QualityCoveragePanel`: diff coverage, treemap phủ test, dòng chưa phủ

**From Solution:** [FE-CV-SOL-087-quality-trend-coverage-hotspot](../solutions/FE-CV-SOL-087-quality-trend-coverage-hotspot.md) mục 2.2
**Priority:** P0
**Area:** frontend / components (pha 2)
**File:** `frontend/src/renderer/src/components/review-map/quality/QualityCoveragePanel.tsx` (mới) và `*.test.tsx`; đăng ký khối ở `quality-lens-blocks.ts`
**Depends on:** 087-15, FE-CV-TASK-088-07, 088-08
**Status:** [x] DONE

## Context

- `CoverageReport.source:'estimated'` phải khác `measured` (hợp đồng §4.7); `estimatedNote?` do backend; `diff.partial`, `excludedFiles[]`.
- Mẫu mở diff tại dòng: `openReviewDiffAtPath` (053) hoặc `openAnnotationLocation` (`check-annotation-open.ts`).
- Ngưỡng từ `quality.profile.get` (`diffCoverageWarnBelow/FailBelow`).

## Việc cần làm

1. `ChartFrame` + `DiffCoverageGauge` (thresholds từ profile qua `toPercent`) + `MetricTreemap` (≤ 400 ô; đậm = tỉ lệ chưa phủ cao; chú giải).
2. `estimated` → huy hiệu "Ước lượng từ cạnh test, không phải đo độ phủ" + `estimatedNote`; `diff:null`/`report:null` → trạng thái rỗng có lý do và hành động ("Chạy kiểm tra" nếu profile có coverage).
3. Danh sách "Dòng đã đổi chưa phủ" (top 20 tệp theo `uncoveredRanges`), bấm mở diff.
4. `excludedFiles` gập, có lý do; `truncated` → "X/Y".

## Kiểm thử

Ma trận `measured|estimated` × `diff null|có` × `partial`; `total=0`; bấm ô treemap mở đúng tệp; `report:null`. Chạy `pnpm --filter orca-frontend test -- src/renderer/src/components/review-map/quality/QualityCoveragePanel`.

## Tiêu chí hoàn thành

- [ ] Không 0% khi không có dữ liệu; `estimated` luôn có chữ.
- [ ] Bảng thay thế đủ dữ liệu.

## Rủi ro

- Hiệu năng 400 ô trong Electron chưa đo.
