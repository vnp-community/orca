# Tasks: quality-visualization (frontend, v7)

> 📋 Proposed. Chưa triển khai. Mỗi task 0,5 đến 2 ngày. Solutions: [../solutions/README.md](../solutions/README.md) (lệnh test, hợp đồng thiếu, quyết định chung). NN liên tục trong cùng CR: CR-088 = 01..09; CR-087 = 01..20 (scorecard 01-08, diff annotations 09-14, trend/coverage/hotspot 15-20).

## Bảng Task

| Task | Mô tả | Priority | Phụ thuộc | Status |
|---|---|---|---|---|
| [FE-CV-TASK-087-01](./FE-CV-TASK-087-01-quality-types-parsers-and-channel-contract.md) | Kiểu, parser, hằng kênh và lỗi của `codeIntel.quality.*` | P0 | FE-CV-SOL-050-types-and-runtime-bridge (bridge, `classifyCodeIntelError`); không cần backend | [ ] TODO |
| [FE-CV-TASK-087-02](./FE-CV-TASK-087-02-quality-store-state-and-event-handling.md) | State `codeIntelQualityByWorktree`, hành động chạy/huỷ, xử lý push và polling | P0 | 087-01; FE-CV-SOL-050-store-and-query-hooks | [ ] TODO |
| [FE-CV-TASK-087-03](./FE-CV-TASK-087-03-quality-support-gate-run-and-profile-hooks.md) | Hook `useQualitySupport`, `useQualityGate`, `useQualityRun`, `useQualityProfiles` | P0 | 087-01, 087-02; FE-CV-SOL-050-store-and-query-hooks (`useCodeIntelSupport`, settings) | [ ] TODO |
| [FE-CV-TASK-087-04](./FE-CV-TASK-087-04-quality-gate-copy-stale-scope-and-view-state-models.md) | Mô hình thuần: copy kết luận, độ cũ, phạm vi chạy, trạng thái hiển thị, chọn profile | P0 | 087-01 (kiểu); không cần backend | [ ] TODO |
| [FE-CV-TASK-087-05](./FE-CV-TASK-087-05-quality-scorecard-steps-provenance-and-ci-comparison.md) | Scorecard cổng, lý do, bước chạy, dòng nguồn, so sánh CI, chip | P0 | FE-CV-TASK-088-03, 088-07; 087-03, 087-04 | [ ] TODO |
| [FE-CV-TASK-087-06](./FE-CV-TASK-087-06-quality-run-control-and-progress.md) | `QualityRunControl`, tiến độ, huỷ, lỗi chạy | P0 | 087-03, 087-04 | [ ] TODO |
| [FE-CV-TASK-087-07](./FE-CV-TASK-087-07-quality-lens-registration-blocks-and-state-screens.md) | Lens `quality`: đăng ký, khung khối, màn trạng thái | P0 | 087-03..087-06; FE-CV-SOL-051-review-workspace-shell | [ ] TODO |
| [FE-CV-TASK-087-08](./FE-CV-TASK-087-08-quality-fake-backend-i18n-and-e2e.md) | Fake backend `quality.*`, phủ khoá i18n, e2e | P1 | 087-01..087-07, 088-09 | [ ] TODO |
| [FE-CV-TASK-087-09](./FE-CV-TASK-087-09-quality-marker-model-and-annotation-eligibility.md) | Mô hình marker thuần và quy tắc đủ điều kiện theo `DiffSource` | P0 | 087-01 (kiểu `QualityFinding`) | [ ] TODO |
| [FE-CV-TASK-087-10](./FE-CV-TASK-087-10-quality-finding-markers-hook-and-lifecycle.md) | Hook `useQualityFindingMarkers` và `useQualityFindingsForFile` | P0 | 087-09, 087-02, 087-03 | [ ] TODO |
| [FE-CV-TASK-087-11](./FE-CV-TASK-087-11-quality-annotations-diff-wiring-and-glyph-css.md) | Gắn hook vào `DiffViewer`, `DiffSectionItem`; CSS glyph; công tắc chú thích | P0 | 087-10, FE-CV-TASK-088-01 | [ ] TODO |
| [FE-CV-TASK-087-12](./FE-CV-TASK-087-12-quality-findings-list-filter-sort-and-virtualization.md) | Danh sách phát hiện kiểm tra: lọc, sắp, ảo hoá, phân trang | P0 | 087-02, 087-03, FE-CV-TASK-088-03 | [ ] TODO |
| [FE-CV-TASK-087-13](./FE-CV-TASK-087-13-quality-findings-dock-source-toggle-and-open-diff.md) | Ô nguồn "Cấu trúc | Kiểm tra" trong dock và liên kết mở diff | P1 | 087-12, 087-07; FE-CV-SOL-059, FE-CV-SOL-053 | [ ] TODO |
| [FE-CV-TASK-087-14](./FE-CV-TASK-087-14-quality-waive-and-revoke.md) | `QualityWaivePopover`: miễn trừ có hạn và bỏ miễn trừ | P1 | 087-12, 087-02 | [ ] TODO |
| [FE-CV-TASK-087-15](./FE-CV-TASK-087-15-quality-trend-coverage-hotspot-data-adapters.md) | Hook dữ liệu và bộ chuyển đổi thuần cho xu hướng, coverage, hotspot, DSM | P0 | 087-01, 087-02, FE-CV-TASK-088-04, 088-05 | [ ] TODO |
| [FE-CV-TASK-087-16](./FE-CV-TASK-087-16-quality-coverage-panel.md) | `QualityCoveragePanel`: diff coverage, treemap phủ test, dòng chưa phủ | P0 | 087-15, FE-CV-TASK-088-07, 088-08 | [ ] TODO |
| [FE-CV-TASK-087-17](./FE-CV-TASK-087-17-quality-trend-panel.md) | `QualityTrendPanel`: xu hướng theo lượt | P1 | 087-15, FE-CV-TASK-088-07 | [ ] TODO |
| [FE-CV-TASK-087-18](./FE-CV-TASK-087-18-quality-hotspot-panel.md) | `QualityHotspotPanel`: bản đồ nhiệt hotspot | P2 | 087-15, FE-CV-TASK-088-08 | [ ] TODO |
| [FE-CV-TASK-087-19](./FE-CV-TASK-087-19-quality-dependency-panel.md) | `QualityDependencyPanel`: ma trận phụ thuộc (DSM) | P2 | 087-15, FE-CV-TASK-088-05, 088-08; FE-CV-SOL-054-structure-lens (chung cache `structure`) | [ ] TODO |
| [FE-CV-TASK-087-20](./FE-CV-TASK-087-20-quality-visualization-fake-backend-i18n-and-e2e.md) | Fake backend trend/coverage/hotspot/structure, i18n, e2e | P1 | 087-15..087-19, 087-08 | [ ] TODO |
| [FE-CV-TASK-088-01](./FE-CV-TASK-088-01-quality-severity-and-heat-tokens.md) | Token `--quality-*` và `--quality-heat-1..5` trong `main.css` | P0 | không (làm được ngay, không cần backend) | [ ] TODO |
| [FE-CV-TASK-088-02](./FE-CV-TASK-088-02-quality-token-contrast-and-parity-tests.md) | Test tương phản và parity của token chất lượng | P0 | FE-CV-TASK-088-01 | [ ] TODO |
| [FE-CV-TASK-088-03](./FE-CV-TASK-088-03-severity-encoding-and-badges.md) | Bảng mã hoá mức nghiêm trọng, glyph, badge, chú giải | P0 | FE-CV-TASK-088-01 | [ ] TODO |
| [FE-CV-TASK-088-04](./FE-CV-TASK-088-04-chart-scale-intensity-and-text-summary.md) | Hàm thuần: thang tuyến tính, bậc cường độ, mô tả văn bản | P0 | FE-CV-TASK-088-03 (nhãn mức dùng trong mô tả) | [ ] TODO |
| [FE-CV-TASK-088-05](./FE-CV-TASK-088-05-squarified-treemap-and-dependency-matrix-ordering.md) | Hàm thuần: treemap squarified và thứ tự ma trận phụ thuộc (DSM) | P0 | không (làm được ngay) | [ ] TODO |
| [FE-CV-TASK-088-06](./FE-CV-TASK-088-06-chart-frame-text-alternative-and-layout-hooks.md) | `ChartFrame`, bảng thay thế, hover card, hook kích thước và vẽ lười | P0 | FE-CV-TASK-088-03 | [ ] TODO |
| [FE-CV-TASK-088-07](./FE-CV-TASK-088-07-stacked-bar-sparkline-trend-and-coverage-gauge.md) | `StackedSeverityBar`, `SparklineChart`, `TrendLineChart`, `DiffCoverageGauge` | P0 | FE-CV-TASK-088-03, 088-04, 088-06 | [ ] TODO |
| [FE-CV-TASK-088-08](./FE-CV-TASK-088-08-keyboard-grid-charts-treemap-heatmap-dsm.md) | Điều hướng bàn phím dạng lưới và ba hình: `MetricTreemap`, `HotspotHeatmap`, `DependencyMatrix` | P0 | FE-CV-TASK-088-04, 088-05, 088-06 (và 088-03 cho glyph) | [ ] TODO |
| [FE-CV-TASK-088-09](./FE-CV-TASK-088-09-chart-fixtures-i18n-coverage-and-performance-record.md) | Fixture cỡ trần, phủ khoá i18n, đo hiệu năng, ghi quyết định thư viện | P1 | FE-CV-TASK-088-03 đến 088-08 | [ ] TODO |

## Thứ tự phụ thuộc

```
088-01 ─▶ 088-02
   └────▶ 088-03 ─▶ 088-04 ─┐
088-05 ─────────────────────┼▶ 088-06 ─▶ 088-07 ─▶ 088-08 ─▶ 088-09
                            └─────────────────────▲
087-01 ─▶ 087-02 ─▶ 087-03 ─▶ 087-04 ─▶ 087-05 ─▶ 087-06 ─▶ 087-07 ─▶ 087-08      (cần 088-03, 088-07 cho 087-05)
087-01 ─▶ 087-09 ─▶ 087-10 ─▶ 087-11 ; 087-12 ─▶ 087-13, 087-14                      (solution 2; sau 087-02/03)
087-15 ─▶ 087-16 (088-07/08), 087-17, 087-18, 087-19 ─▶ 087-20                        (solution 3; sau 087-02/07)
```

Làm được ngay, không cần backend: 088-01, 088-05, 087-01, 087-04, 087-09. Nhóm còn lại cần CR-050/051 (bridge, slice, registry lens, fake backend G4); 087-13 cần FE-CV-SOL-053/059; 087-17 cần FE-CV-SOL-060 cho so sánh lượt (không nút chết khi chưa có).

## Ghi chú

- Đề xuất đợt PR: (1) 088-01..09; (2) 087-01..08; (3) 087-09..14; (4) 087-15..20. 087-11 chạm `DiffViewer`/`DiffSectionItem` nên tách PR riêng.
- Task mới mọi đường dẫn "(mới)"; phần chưa kiểm chứng ghi trong từng task; không `max-lines` disable; không file tên `helpers/utils/common`.
- Cảnh báo trước tạo review đã gửi không nằm ở đây: thuộc FE-CV-SOL-085-source-control-quality-notice (khe `qualityNotice`; điểm gọi `SourceControl.tsx:5247`, `ChecksPanel.tsx:3611`).
