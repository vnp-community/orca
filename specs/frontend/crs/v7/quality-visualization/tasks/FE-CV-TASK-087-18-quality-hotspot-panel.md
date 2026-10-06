# FE-CV-TASK-087-18: `QualityHotspotPanel`: bản đồ nhiệt hotspot

**From Solution:** [FE-CV-SOL-087-quality-trend-coverage-hotspot](../solutions/FE-CV-SOL-087-quality-trend-coverage-hotspot.md) mục 2.2
**Priority:** P2
**Area:** frontend / components (pha 3)
**File:** `frontend/src/renderer/src/components/review-map/quality/QualityHotspotPanel.tsx` (mới) và `*.test.tsx`
**Depends on:** 087-15, FE-CV-TASK-088-08
**Status:** [ ] TODO

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
