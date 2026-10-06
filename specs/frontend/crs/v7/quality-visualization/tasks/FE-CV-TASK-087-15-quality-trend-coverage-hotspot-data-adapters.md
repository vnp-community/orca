# FE-CV-TASK-087-15: Hook dữ liệu và bộ chuyển đổi thuần cho xu hướng, coverage, hotspot, DSM

**From Solution:** [FE-CV-SOL-087-quality-trend-coverage-hotspot](../solutions/FE-CV-SOL-087-quality-trend-coverage-hotspot.md) mục 2.1, 2.2
**Priority:** P0
**Area:** frontend / hooks + pure functions
**File:** `frontend/src/renderer/src/hooks/useQualityTrend.ts`, `useQualityCoverage.ts`, `useQualityHotspots.ts`, `useQualityDependencyMatrix.ts`; `components/review-map/quality/coverage-percent-normalization.ts`, `quality-trend-series.ts`, `quality-coverage-treemap-items.ts`, `quality-hotspot-rows.ts`, `quality-dependency-graph-reduction.ts` (mới) và `*.test.ts(x)`
**Depends on:** 087-01, 087-02, FE-CV-TASK-088-04, 088-05; FE-CV-SOL-050-store-and-query-hooks (`useCodeIntelQuery` cho `structure`/`findings`)
**Status:** [ ] TODO

## Context

- Hợp đồng PQ-33: `metrics` vắng = không có số. §4.7 `CoverageReport`; §3.1 `findings` (`rules`, `Finding.metrics`), `structure` (`ModuleGraph`).
- Đơn vị phần trăm không nêu trong hợp đồng → cô lập ở `coverage-percent-normalization.ts` (giả định 0..100; test ghi giả định).
- Cột hotspot suy từ khoá `metrics` thật; không đặt tên cột cứng.

## Việc cần làm

1. `coverage-percent-normalization.ts`: `toPercent(value)` → số trong [0,100] hoặc `null`; ngoài khoảng → `null` (không kẹp thầm).
2. `quality-trend-series.ts`: `QualityTrendPoint[]` → `{series: error|warning|info, xLabels, markers (verdict đổi), diffCoverageSpark}`; `metrics` vắng → `null`; nhãn lượt khi `turnKey` khớp mốc, ngược lại commit rút gọn + giờ; ≤ 50 điểm mới nhất.
3. `quality-coverage-treemap-items.ts`: `files[]` → `{id,label,size=stmts,intensity=1-covered/stmts}`; `stmts=0` bị bỏ và đếm; lớp phủ "đã đổi" nếu có `ChangeOverlay`.
4. `quality-hotspot-rows.ts`: `Finding[]` (`rule==='hotspot.file'`) → `rows` (id=file) + `columns` từ hợp `metrics` (≤ 6 theo tần suất); thiếu → `null`.
5. `quality-dependency-graph-reduction.ts`: `ModuleGraph` → cạnh `imports`, top 59 theo bậc + nút "Khác" (gộp trọng số), trả `truncated` và tổng.
6. Bốn hook: `enabled` chỉ khi khối mở và `useQualitySupport()==='enabled'`; `quality.trend`/`quality.coverage` vào `codeIntelQualityByWorktree`; `structure`/`findings` qua `useCodeIntelQuery`; trả `status` đúng bảng 088 `ChartFrame`; `report:null` → `empty` + `reason`.

## Kiểm thử

Thuần: đơn vị biên (0, 1, 62, 100, 101, -1, NaN); trend với `metrics` rỗng, `truncated`, 1 điểm; treemap `stmts=0`, 1 000 tệp (cắt); hotspot khoá lạ/thiếu; DSM tự vòng/đồ thị rỗng/ >60 nút. Hook: không gọi khi khối đóng, huỷ khi gỡ. Chạy `pnpm --filter orca-frontend test -- src/renderer/src/components/review-map/quality/quality- src/renderer/src/hooks/useQuality`.

## Tiêu chí hoàn thành

- [ ] Không bao giờ biến `null` thành 0; mọi cắt có tổng.
- [ ] Giả định đơn vị chỉ ở một file.

## Rủi ro

- Đơn vị và khoá hotspot chưa chốt: nếu hợp đồng khác, chỉ sửa hai file này.
