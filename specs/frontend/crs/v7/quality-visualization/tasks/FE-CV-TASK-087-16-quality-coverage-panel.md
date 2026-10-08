# FE-CV-TASK-087-16: `QualityCoveragePanel`: diff coverage, treemap phủ test, dòng chưa phủ

**From Solution:** [FE-CV-SOL-087-quality-trend-coverage-hotspot](../solutions/FE-CV-SOL-087-quality-trend-coverage-hotspot.md) mục 2.2
**Priority:** P0
**Area:** frontend / components (pha 2)
**File:** `frontend/src/renderer/src/components/review-map/quality/QualityCoveragePanel.tsx` (mới) và `*.test.tsx`; đăng ký khối ở `quality-lens-blocks.ts`
**Depends on:** 087-15, FE-CV-TASK-088-07, 088-08
**Status:** [x] DONE (verified 2026-10-07: 13 test pass, components/review-map/quality/QualityCoveragePanel.test.tsx; kèm QualityCoverageUncoveredList + quality-coverage-uncovered-files)

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

## Ghi chú triển khai (2026-10-07)

- **Đơn vị:** ngưỡng và `pct` là tỉ lệ 0..1 (D5) -> vạch gauge qua `toPercent` (test: 0.8 -> `left: 80%`). Gauge vẫn tính từ `covered/changedExecutable`.
- **`diff:null`, `changedExecutable<=0`, `report:null`:** khung rỗng có lý do (`diff.reason`/`reason` của backend), không vẽ gauge nên không bao giờ ra 0%. `report:null` có nút "Chạy kiểm tra" chỉ khi `profile.definition.coverage.required` và có profile được chọn; nút gọi `startQualityRun(worktree, {profile, scope:'changed'})`.
- **Danh sách "Dòng chưa phủ":** `CoverageFile.uncoveredRanges` không nói là dòng đã đổi, nên tiêu đề ghi trung thực "Lines not covered" (không phải "đã đổi chưa phủ"); top 20 tệp theo số dòng chưa phủ, tối đa 20 khoảng mỗi tệp, bấm mở `onOpenDiff(path, từ dòng)`.
- `estimated`: câu cảnh báo + `estimatedNote` thành chữ; `measured`: "Measured coverage". `partial`, `excludedFiles` (gập), `dirty`, `X/Y` khi `truncated` đều có test.
- Panel `export default` và named; đăng ký lazy ở `quality-lens-blocks.ts` do lead làm. Hiệu năng 400 ô trong Electron chưa đo.
