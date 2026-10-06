# FE-CV-SOL-087-quality-trend-coverage-hotspot: Phủ test, diff coverage, xu hướng theo lượt, hotspot, DSM

> 📋 Proposed. Chưa triển khai. Viết ngày 2026-10-06; chưa chạy test hay ứng dụng. Solution 3/3 của CR-CV-087 ([1](./FE-CV-SOL-087-quality-scorecard-and-state.md), [2](./FE-CV-SOL-087-quality-diff-annotations.md)). Phân pha theo dữ liệu: **pha 2** (coverage, xu hướng) cần CR-083/085; **pha 3** (hotspot, DSM) cần dữ liệu CR-037.

**CR:** [CR-CV-087](../../../../../../docs/crs/v7/quality-visualization/CR-CV-087-quality-frontend-scorecard-and-annotations.md) mục 2.8
**Area:** frontend (`components/review-map/quality/`, `hooks/`, `store`)
**Làm sau:** FE-CV-SOL-088 (hình), FE-CV-SOL-087-quality-scorecard-and-state (state, lens, registry khối), FE-CV-SOL-054-structure-lens (dùng chung dữ liệu `structure`), FE-CV-SOL-060 (mốc lượt).
**TDD tham chiếu:** [v5/02-state-management](../../../../tdd/v5/02-state-management.md), [v5/05-ui-components](../../../../tdd/v5/05-ui-components.md).

## 0. Hợp đồng áp dụng, lệch, phụ thuộc chéo

| Mục hợp đồng | Dùng cho |
|---|---|
| §3.2 `quality.trend {from?, to?, limit ≤200 (50), groupBy?: 'commit'\|'turn'}` → `{points, truncated, totalCount}`; `quality.coverage {runId?\|headCommit?}` → `{report: CoverageReport\|null, reason?}` (20 s) | dữ liệu hai khối |
| §4.7 + PQ-33 `QualityTrendPoint {turnKey, headCommit, baseCommit, profileRef, verdict, counts{error,warning,info,byCategory}, metrics{diffCoverage?,newLayerViolations?,newCycles?,testsFailed?}, runIds, indexCommit, source, createdAt}`: **khoá vắng = không có số, không phải 0** | `TrendLineChart` ngắt đường |
| §4.7 `CoverageReport {source:'measured'\|'estimated', language, mode?, dirty, totals, diff\|null{changedExecutable,covered,uncovered,diffCoverage\|null,reason?,partial,excludedFiles[]}, files[]{path,stmts,covered,pct,uncoveredRanges?}, truncated, totalCount, toolVersions, estimatedNote?}` | gauge, treemap |
| §3.1 `findings` (`rules?:['hotspot.file']`, `Finding.metrics: Record<string,number>`, `owner?`), `Settings.hotspotWindowDays` | hotspot |
| §3.1 `structure {path?, depth 1..3, limit, pageToken}` → `Env<ModuleGraph>`; §4.2 `ModuleNode{id,kind,language?,symbolCount,loc?,cluster?,area?}`, `ModuleEdge{from,to,kind:'imports'\|'contains',count}`; PQ-10 (`architecture` là C4, không phải đồ thị cụm) | DSM |
| §3.2 `quality.profile.get` → `QualityProfile.definition.coverage {required, diffCoverageWarnBelow, diffCoverageFailBelow}` | vạch ngưỡng gauge |
| §4.7 quy tắc hiển thị | `estimated` khác `measured`; không điểm số |

**Lệch giữa CR và hợp đồng:**

| # | CR-087 | Hợp đồng / backend | Quyết định |
|---|---|---|---|
| 1 | `QualityTrendPoint {turnId, at, runId, error, warning, info, coverage, diffCoverage, verdict}` | PQ-33 hình dạng khác (xem trên); không có `coverage` tổng ở điểm, có `byCategory`, `metrics` | Theo hợp đồng; sparkline "coverage" bỏ (không có số tổng), thay bằng sparkline `metrics.diffCoverage` |
| 2 | `CoverageReport {estimated, lines, files[]{covered,total,changedCovered...}, diff{covered,total}}` | `source`, `totals`, `files[]{stmts,covered,pct,uncoveredRanges}`, `diff{changedExecutable,covered,...}` | Theo hợp đồng; kích thước ô = `stmts`, độ đậm = tỉ lệ chưa phủ = `1 - covered/stmts` |
| 3 | `QualityHotspot[] {path,churn,complexity,findings,uncoveredRatio,score}` "đề xuất" | **Không có kênh/kiểu hotspot** trong hợp đồng; chỉ `Finding` `kind:'hotspot'`/`rule:'hotspot.file'` với `metrics` (khoá không liệt kê) và `hotspotWindowDays`; BE-CV-SOL-085-* không nêu hotspot | Nguồn = `codeIntel.findings {rules:['hotspot.file']}`; cột lấy từ **hợp của khoá `metrics`** đã nhận (nhãn qua `translate()` theo khoá, rơi về khoá thô); không bịa tên cột. **Hợp đồng thiếu** (câu hỏi mở 1) |
| 4 | DSM từ `codeIntel.architecture`/`structure` chưa chốt | PQ-10: `architecture` = C4; `structure` = `ModuleGraph` có `imports` với `count` | DSM từ `structure` (cùng cache với FE-CV-SOL-054), cạnh `kind:'imports'` |
| 5 | Nhãn lượt theo `ReviewTurnMarker.turnId` | `turnKey` (định dạng không nêu) | Khớp theo bằng nhau chuỗi; không khớp → nhãn commit rút gọn + giờ. `groupBy:'turn'` chỉ khi có `turnKey` |
| 6 | Trend phân biệt `source` (CR 7.4) | `QualityTrendPoint.source: 'local'\|'ci'` | Hiển thị glyph nguồn trong bảng thay thế và tooltip |
| 7 | Ngưỡng gauge "parse từ `QualityGate.reasons`" | Profile có `diffCoverageWarnBelow/FailBelow` | Lấy từ `quality.profile.get` (không parse chuỗi) |

**Đơn vị chưa chốt trong hợp đồng:** `diffCoverage`, `totals.pct`, `files[].pct`, các ngưỡng, `metrics.diffCoverage` (0..1 hay 0..100). Giải pháp: một hàm `coverage-percent-normalization.ts` cô lập giả định (xem 2.2).

**Phụ thuộc chéo khu vực:** `BE-CV-SOL-083-coverage-storage-and-diff` (+ agent `AG-CV-SOL-083-coverage-collection`: không có coverage thì `report:null` + `reason`), `BE-CV-SOL-085-waivers-and-trend` (điểm xu hướng, khoá `source`), `BE-CV-SOL-037-structure-findings-and-dismissals` (hotspot, `Finding.metrics`; phát hiện `hotspot.file` mặc định `info`), `BE-CV-SOL-040-codeintel-quality-channels` + view channels (`structure`, `findings`), `BE-CV-SOL-036-*` (khi cần `ChangeOverlay` cho lớp phủ "đã đổi"); FE: `FE-CV-SOL-054-structure-lens`, `FE-CV-SOL-053` (`OVERLAY_ENCODING`, `openReviewDiffAtPath`), `FE-CV-SOL-060` (so sánh lượt, tên action chưa chốt), `FE-CV-SOL-050-*` (`useCodeIntelQuery`).

## 1. Trạng thái hiện tại (Re-verify)

Đã đọc: `components/status-bar/workspace-space-layout.ts` (`buildTreemapLayout`:117), `WorkspaceSpaceManagerPanel.tsx` (:89-93, :917), `components/ui/{collapsible,table,toggle}` (liệt kê), `package.json`, hợp đồng §3.1/3.2/4.2/4.7, `docs/crs/v7/quality-visualization/CR-CV-087` mục 2.8, `CR-CV-088`, `specs/backend-go/crs/v7/quality-gate/solutions/BE-CV-SOL-085-waivers-and-trend.md` (chỉ nêu FE đối ứng; **không** nêu hotspot hay đơn vị coverage; không đọc kỹ toàn file). Không có mã coverage/trend/hotspot/DSM trong `frontend/src` (grep `quality` trong `assets/main.css` rỗng; `components/quality-charts` và `review-map` chưa tồn tại). Không có công cụ đo độ phức tạp (nghiên cứu 11 B4 xếp "Sau") ⇒ cột "độ phức tạp" **không** có nguồn.

**Correction relative to CR-087:** bảng "Lệch" #1-#7.

## 2. Giải pháp

### 2.1 Cây file

```
hooks/useQualityTrend.ts, useQualityCoverage.ts, useQualityHotspots.ts, useQualityDependencyMatrix.ts   (mới)
components/review-map/quality/
  QualityCoveragePanel.tsx  QualityTrendPanel.tsx  QualityHotspotPanel.tsx  QualityDependencyPanel.tsx
  coverage-percent-normalization.ts     ép đơn vị về phần trăm 0..100 (một nơi, có test, giả định ghi rõ)
  quality-trend-series.ts               QualityTrendPoint[] -> series cho TrendLineChart + nhãn lượt
  quality-coverage-treemap-items.ts     CoverageReport.files -> items MetricTreemap
  quality-hotspot-rows.ts               Finding[] (hotspot.file) -> rows/columns HotspotHeatmap
  quality-dependency-graph-reduction.ts ModuleGraph -> nodes/edges ≤ 60 (top theo bậc + "Khác")
  quality-lens-blocks.ts                (sửa, solution 1) đăng ký bốn khối
```

### 2.2 Khối và dữ liệu

| Khối | Dữ liệu | Hình (088) | Nhãn/lưu ý |
|---|---|---|---|
| Diff coverage + Phủ test | `quality.coverage` + ngưỡng từ `profile.get` | `DiffCoverageGauge` (`covered=diff.covered`, `total=diff.changedExecutable`), `MetricTreemap` (≤ 400 ô) | `source:'estimated'` → huy hiệu + `estimatedNote`; `diff:null` hoặc `report:null` → "Chưa có dữ liệu độ phủ cho phạm vi này" + `reason`/`diff.reason` (không 0%); `partial` → "Một phần phạm vi"; `excludedFiles` liệt kê gập; `files[].uncoveredRanges` → danh sách "Dòng chưa phủ" bấm mở diff |
| Xu hướng | `quality.trend {limit:50, groupBy}` | `TrendLineChart` (error/warning/info), đánh dấu ◆ khi `verdict` đổi, `SparklineChart` cho `metrics.diffCoverage` | Cần ≥ 2 điểm ("Cần ít nhất hai lượt để thấy xu hướng"); `metrics` vắng → ngắt đường; `truncated` → "X/Y"; bấm điểm có `turnKey` khớp → so sánh lượt của 060 (tên action chưa chốt) |
| Hotspot | `codeIntel.findings {rules:['hotspot.file'], scope:'all'}` | `HotspotHeatmap` ≤ 40×6 | Cột = hợp khoá `metrics` (≤ 6 khoá đầu theo tần suất; còn lại trong bảng thay thế); `null` → "—"; không dữ liệu → "Chưa có dữ liệu hotspot"; cửa sổ `hotspotWindowDays` ghi trong mô tả; không cột độ phức tạp |
| Phụ thuộc (DSM) | `codeIntel.structure {depth:2}` (dùng chung cache với lens Cấu trúc) | `DependencyMatrix` ≤ 60 nút | Cạnh `imports` với `count`; `truncated` → "X/Y"; bấm ô → liệt kê cạnh và mở lens Cấu trúc (nếu có) |

`coverage-percent-normalization.ts`: `toPercent(value, hint)`: giả định **giá trị `diffCoverage|pct` là phần trăm 0..100** (ngưỡng profile cũng số tương ứng) khi chưa có xác nhận; hàm trả `null` cho giá trị ngoài [0,100] và test ghi rõ giả định để đổi một chỗ khi hợp đồng chốt. Gauge luôn tính từ `covered/total`, nên chỉ vạch ngưỡng và sparkline phụ thuộc giả định.

Mỗi khối là `ChartFrame` (088) với `status` suy từ hook (`loading|ready|empty|error|stale`), chỉ nạp khi mở; mặc định mở "Phủ test" nếu có coverage. Lỗi: inline + "Thử lại" (không toast). Dữ liệu khối nằm trong `codeIntelQualityByWorktree` (khoá `trend`, `coverage`, `hotspots`) để cùng đường prune; DSM dùng cache `structure` của CR-050.

### 2.3 Wireframe khối Xu hướng

```
▾ Xu hướng theo lượt                                             [Xem dạng bảng]
  Lỗi ⊗ 12 ─┐                    ◆ kết luận đổi: Chưa đạt → Có cảnh báo
            ⊗ 7 ─┐   ⊗ 3
  Cảnh báo ▲ 8 ──▲ 8 ──▲ 8       Nguồn: ● cục bộ  ◇ CI
  diff coverage  ▁▂▃▅ 62%  (— lượt 2 không có số)
  Số liệu tại HEAD a41c9e0 · cục bộ
```

## 3. Quyết định thiết kế

- Khối nào chưa có nguồn hiển thị "Chưa có dữ liệu" kèm lý do, không ẩn, không đoán.
- Hotspot chỉ từ `Finding`; không tạo tín hiệu "độ phức tạp" hay "điểm số".
- DSM từ `structure` để dùng chung cache với lens Cấu trúc.
- Đơn vị coverage cô lập một nơi, gauge không phụ thuộc.
- Nhãn trục "Lượt" khi `turnKey` khớp mốc lượt; nếu không, commit + giờ.

## 4. Tiêu chí chấp nhận

- [ ] Diff coverage `estimated` hiển thị khác `measured`; không dữ liệu → "Chưa có dữ liệu", không 0%; `partial` và `excludedFiles` hiển thị.
- [ ] Xu hướng ≥ 2 điểm; ≤ 50 điểm có "X/Y"; `metrics` vắng ngắt đường (test không 0); đổi kết luận có ◆ + nhãn; nguồn local/CI phân biệt không chỉ bằng màu.
- [ ] Hotspot: cột suy từ dữ liệu; `null` "—"; không có dữ liệu → "Chưa có dữ liệu hotspot"; không cột độ phức tạp.
- [ ] DSM ≤ 60 nút + "Khác (N)"; khối vòng đánh dấu không chỉ màu; bấm ô liệt kê cạnh.
- [ ] Mỗi khối có bảng thay thế, mô tả, trạng thái `loading|empty|error|stale`; nạp khi mở; lens lazy.
- [ ] `translate()` đủ 5 locale; không hex; Electron và web; không dependency mới; không `components/code-review/*`; không `max-lines` disable.

## 5. Kiểm thử

Vitest + Testing Library; `pnpm --filter orca-frontend test -- <đường dẫn>`. **Chưa chạy.** Thuần: `coverage-percent-normalization`, `quality-trend-series` (null, `groupBy`, nhãn lượt), `quality-coverage-treemap-items` (cắt 400 + "+N"), `quality-hotspot-rows` (khoá lạ, null), `quality-dependency-graph-reduction` (top theo bậc, "Khác", tự vòng). Hook: không gọi khi khối đóng; huỷ khi gỡ; `report:null` + `reason`. Component: bốn panel × (`loading|empty|error|stale|ready`), `estimated`, `partial`, truncated; bấm điểm xu hướng; bấm ô DSM. Fake backend: các kịch bản trend/coverage/hotspot/structure (087-20).

## 6. Rủi ro và điểm chưa kiểm chứng

1. Đơn vị coverage chưa chốt (giả định 0..100).
2. Khoá `Finding.metrics` của hotspot không được liệt kê: cột có thể không ổn định giữa các worktree.
3. `turnKey` so `ReviewTurnMarker.turnId`: định dạng chưa nêu.
4. Chưa đo vẽ 700 ô DSM, 400 ô treemap trong Electron.
5. `structure` ở `depth:2` có thể vượt 60 nút; gộp "Khác" mất thông tin; thử trên dữ liệu thật.
6. Thứ tự DSM có thể kém đọc (xem SOL-088 6.4).

## 7. Câu hỏi mở

1. **Hợp đồng thiếu:** kênh/kiểu hotspot (hoặc danh sách khoá `Finding.metrics` cho `hotspot.file`); nguồn độ phức tạp.
2. **Hợp đồng thiếu:** đơn vị các số phần trăm coverage.
3. Định dạng `turnKey`.
4. Tên action "so sánh lượt" của FE-CV-SOL-060.
5. Có nên cho phép DSM theo cụm/C4 (`architecture`) thay `structure` không.

## 8. Tasks

[FE-CV-TASK-087-15](../tasks/FE-CV-TASK-087-15-quality-trend-coverage-hotspot-data-adapters.md) đến [087-20](../tasks/FE-CV-TASK-087-20-quality-visualization-fake-backend-i18n-and-e2e.md).
