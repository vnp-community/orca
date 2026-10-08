# FE-CV-TASK-087-18: `QualityHotspotPanel`: bản đồ nhiệt hotspot

**From Solution:** [FE-CV-SOL-087-quality-trend-coverage-hotspot](../solutions/FE-CV-SOL-087-quality-trend-coverage-hotspot.md) mục 2.2
**Priority:** P2
**Area:** frontend / components (pha 3)
**File:** `frontend/src/renderer/src/components/review-map/quality/QualityHotspotPanel.tsx` (mới) và `*.test.tsx`
**Depends on:** 087-15, FE-CV-TASK-088-08
**Status:** [x] DONE (verified 2026-10-07: 9 test pass, components/review-map/quality/QualityHotspotPanel.test.tsx)

## Context

- Dữ liệu từ `Finding` `rule:'hotspot.file'` (CR-037; mặc định `info`, độ chính xác chưa đo), cửa sổ `Settings.hotspotWindowDays`. Không kênh hotspot riêng; không công cụ độ phức tạp.
- Cột là khoá `metrics` thật; `owner?` có thể hiển thị.

## Việc cần làm

1. `ChartFrame` + `HotspotHeatmap` (≤ 40 hàng × ≤ 6 cột), số luôn hiện, `null` "—"; mô tả ghi cửa sổ ngày.
2. Không dữ liệu → "Chưa có dữ liệu hotspot" + lý do (chưa bật/không đủ lịch sử).
3. Chọn hàng → "Xem diff"/mở tệp (`openAnnotationLocation`); hiển thị `owner` nếu có (văn bản thuần).
4. Ghi rõ nguồn là phát hiện cấu trúc (CR-037), không phải điểm chất lượng.

## Kiểm thử

Cột suy từ dữ liệu; `null`; 41 hàng cắt "X/Y"; rỗng; `owner` chứa HTML hiển thị chữ. Chạy `pnpm --filter orca-frontend test -- src/renderer/src/components/review-map/quality/QualityHotspotPanel`.

## Tiêu chí hoàn thành

- [ ] Không cột "độ phức tạp" giả; không điểm tổng.
- [ ] Bàn phím lưới theo 088-08.

## Rủi ro

- Khoá `metrics` chưa chốt: phụ thuộc BE-CV-SOL-037.

## Ghi chú triển khai (2026-10-07)

- Cột hoàn toàn suy từ khoá `Finding.metrics` (test: `authors`, `churn`, `recentFixes`; không có cột complexity, không điểm tổng). Nhãn cột qua `translate('auto.components.reviewQuality.hotspotMetric.<khoá>', khoá)` nên khoá lạ rơi về khoá thô; chưa có nhãn dịch cho khoá cụ thể vì hợp đồng không liệt kê khoá (câu hỏi mở 1 vẫn còn).
- `owner` hiển thị bằng React text (test với chuỗi HTML hiển thị chữ). Chọn hàng (Enter) -> thanh chi tiết + nút "Open diff" (`onOpenDiff(path)`).
- >40 hàng: chart tự cắt và ghi "Showing 40/41"; backend còn tệp chưa tải (`totalCount` > số đã tải) cũng được ghi. >6 khoá metric: bảng "All metrics (N)" đầy đủ.
- Không ghi số ngày `hotspotWindowDays` vì giá trị không có trong store renderer.
