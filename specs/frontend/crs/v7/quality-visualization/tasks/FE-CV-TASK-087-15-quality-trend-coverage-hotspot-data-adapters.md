# FE-CV-TASK-087-15: Hook dữ liệu và bộ chuyển đổi thuần cho xu hướng, coverage, hotspot, DSM

**From Solution:** [FE-CV-SOL-087-quality-trend-coverage-hotspot](../solutions/FE-CV-SOL-087-quality-trend-coverage-hotspot.md) mục 2.1, 2.2
**Priority:** P0
**Area:** frontend / hooks + pure functions
**File:** `frontend/src/renderer/src/hooks/useQualityTrend.ts`, `useQualityCoverage.ts`, `useQualityHotspots.ts`, `useQualityDependencyMatrix.ts`; `components/review-map/quality/coverage-percent-normalization.ts`, `quality-trend-series.ts`, `quality-coverage-treemap-items.ts`, `quality-hotspot-rows.ts`, `quality-dependency-graph-reduction.ts` (mới) và `*.test.ts(x)`
**Depends on:** 087-01, 087-02, FE-CV-TASK-088-04, 088-05; FE-CV-SOL-050-store-and-query-hooks (`useCodeIntelQuery` cho `structure`/`findings`)
**Status:** [x] DONE (verified 2026-10-07: 51 test pass — coverage-percent-normalization 12, quality-trend-series 7, quality-coverage-treemap-items 7 (kèm danh sách dòng chưa phủ), quality-hotspot-rows 6, quality-dependency-graph-reduction 6, hooks/useQualityVisualizationHooks.test.tsx 13)

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

## Ghi chú triển khai (2026-10-07)

- **Đơn vị coverage (lệch spec):** hợp đồng D5 đã chốt coverage là tỉ lệ 0..1, không phải 0..100 như giả định trong task/solution. `coverage-percent-normalization.ts` export `toPercent(ratio)`: nhân 100 (làm tròn 2 chữ số), giá trị ngoài [0,1] / NaN / null -> `null` (không kẹp). Test dùng đúng quy ước này (0, 1, 0.62, 0.07, 1.01, -1, 62 -> null). Ngưỡng profile (`diffCoverageWarnBelow/FailBelow`) cũng là tỉ lệ nên đi qua `toPercent` trước khi vẽ vạch gauge.
- **Hook:** bốn hook thêm trạng thái `idle` (quality không bật) và `unavailable` (không có selector worktree) ngoài `loading|ready|empty|error|stale`, qua `quality-block-status-model.ts`. Làm mới thất bại giữ dữ liệu cũ và trả `stale`. Hook trend/coverage nạp lại khi `epoch` (run xong) hoặc `codeIntelResyncCounter` đổi. Hook hotspot/DSM nhận tham số `callFn` để test. Hook không có cờ "open": panel chỉ được lens mount khi khối mở.
- **Hotspot:** nguồn `codeIntel.findings {rules:['hotspot.file'], scope:'all', limit:100}`, cột = hợp khoá `Finding.metrics` hữu hạn (<=6 theo tần suất, phần còn lại ở bảng đầy đủ); sắp xếp hàng theo cột đầu chỉ để ổn định, không phải điểm. `owner` là `Owner {names[]}` nối bằng dấu phẩy.
- **DSM:** `codeIntel.structure {depth:2}` (cùng khoá query với gốc lens Cấu trúc nhưng lens này tự giữ state nên cache chia sẻ chỉ ở mức khoá). Rút gọn: giữ 59 nút bậc cao nhất + nút "Other (N)" (dùng chuỗi `matrix.other` của chart), cạnh vào/ra nhóm "Other" gộp trọng số, cạnh tự vòng bị bỏ và đếm.
- **Chưa làm:** lớp phủ `ChangeOverlay` "đã đổi" cho treemap (chưa có nguồn nối trong panel). `Settings.hotspotWindowDays` không nằm trong store renderer nên mô tả hotspot không ghi số ngày.
