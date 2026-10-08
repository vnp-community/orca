# FE-CV-TASK-088-07: `StackedSeverityBar`, `SparklineChart`, `TrendLineChart`, `DiffCoverageGauge`

**From Solution:** [FE-CV-SOL-088](../solutions/FE-CV-SOL-088-graphics-foundation-and-chart-primitives.md) mục 2.5, 2.6
**Priority:** P0
**Area:** frontend / components (hình không tương tác từng điểm)
**File:** `frontend/src/renderer/src/components/quality-charts/StackedSeverityBar.tsx`, `SparklineChart.tsx`, `TrendLineChart.tsx`, `DiffCoverageGauge.tsx` (mới) và `__tests__/*.test.tsx`
**Depends on:** FE-CV-TASK-088-03, 088-04, 088-06
**Status:** [x] DONE (verified 2026-10-07: StackedSeverityBar 3/3, SparklineChart 3/3, TrendLineChart 6/6, DiffCoverageGauge 5/5)

## Context

- F2, F3, F4, F5 của CR-088 2.1. F3-F5 là `role="img"` + bảng thay thế (không tương tác từng điểm ở MVP).
- PQ-33: `null` = không có số: đường **ngắt**, không nối qua, không 0.
- Hợp đồng §4.7: `CoverageReport.source:'estimated'` hiển thị khác `measured`; `diff.partial` phải nêu; thresholds từ `QualityProfile.definition.coverage.{diffCoverageWarnBelow,diffCoverageFailBelow}` (có thể `null`).
- Mẫu: `components/setup-guide/SetupGuideProgressRing.tsx` (SVG nhỏ, `aria-label` có "x/y", `Tooltip`); `ui/progress.tsx` **không** dùng (có chuyển tiếp cố định).
- Quy ước dữ liệu của solution này: phần trăm 0..100 đã chuẩn hoá; việc ép đơn vị của hợp đồng thuộc 087-15.

## Việc cần làm

1. `StackedSeverityBar`: props `{counts: {error,warning,info}; total?}`; ba đoạn HTML flex (hoặc SVG), mỗi đoạn có số và `SeverityGlyph` (không chỉ màu); đoạn 0 bị ẩn nhưng có trong bảng thay thế; tổng 0 hiển thị "Không có phát hiện trong phạm vi đã chạy" **chỉ khi** nơi gọi cho biết đã chạy (`ranCheck` prop) — nếu chưa chạy, hiển thị "Chưa có dữ liệu" (không suy diễn sạch).
2. `SparklineChart`: `<polyline>` theo từng đoạn liên tục (tách tại `null`), chấm cuối + nhãn số; ≥ 2 điểm số mới vẽ đường; một điểm: chỉ số + "Cần ít nhất hai điểm"; dùng `createLinearScale`.
3. `TrendLineChart`: trục X hạng mục rời (nhãn `xLabels`), tối đa `maxPoints` (50) — dư thì lấy 50 điểm mới nhất và hiển thị "Đang hiển thị 50/{N}"; mỗi chuỗi một `strokeDash` + glyph điểm theo `SEVERITY_ENCODING`; đánh dấu đổi kết luận bằng ◆ có nhãn (`marker`), không chỉ màu; `onSelectPoint?` biến điểm thành `<button>` trong lưới ẩn có thể tab (một điểm dừng Tab, dùng `useChartKeyboardNavigation` của 088-08 nếu đã có; nếu chưa, để `onSelectPoint` không dùng ở task này).
4. `DiffCoverageGauge`: bullet bar: thanh nền `--muted`, thanh giá trị theo `covered/total`, hai vạch ngưỡng (`warnBelow`, `failBelow`) hình dạng khác nhau (nét liền/đứt) kèm nhãn số; chữ "62% (124/200 dòng đã đổi được phủ)"; `source==='estimated'` thêm huy hiệu "Ước lượng từ cạnh test, không phải đo độ phủ"; `partial` thêm "Một phần phạm vi"; `total===0` → "Chưa có dữ liệu độ phủ cho phạm vi này" (không 0%, không chia 0); `role="meter"` với `aria-valuenow/min/max` và `aria-valuetext` đầy đủ (chưa kiểm chứng hỗ trợ `role="meter"` ở trình đọc màn hình trong Electron: dự phòng `role="img"` + `aria-label`).
5. Không `transition`; màu chỉ bằng lớp `text-quality-*`/token; không hex.
6. Khoá i18n `auto.components.qualityCharts.{stackedBar,sparkline,trend,gauge}.*`.

## Kiểm thử

- Stacked: tổng 0 với/không `ranCheck`; chữ số luôn hiện; glyph có mặt.
- Sparkline/Trend: `null` giữa dãy → hai đoạn; toàn `null` → trạng thái rỗng, không `NaN` trong thuộc tính SVG (quét `NaN` trong DOM); >50 điểm cắt có "X/Y"; marker đổi kết luận có nhãn.
- Gauge: 62/100 + ngưỡng 80; ngưỡng `null`; `estimated`; `total=0`; `covered>total` kẹp 100% và ghi cảnh báo trong bảng (dữ liệu bất thường, không ném).
- Chạy: `pnpm --filter orca-frontend test -- src/renderer/src/components/quality-charts/__tests__/StackedSeverityBar src/renderer/src/components/quality-charts/__tests__/SparklineChart src/renderer/src/components/quality-charts/__tests__/TrendLineChart src/renderer/src/components/quality-charts/__tests__/DiffCoverageGauge`.

## Tiêu chí hoàn thành

- [ ] Bốn hình có `role` + `aria-label` từ `chart-text-summary` + bảng thay thế (qua `ChartFrame`).
- [ ] Không dùng 0 thay cho `null`; không `NaN`.
- [ ] `estimated` luôn khác `measured` bằng chữ.

## Rủi ro

- `role="meter"` chưa kiểm chứng ở trình đọc màn hình; dự phòng đã nêu.
- Nhãn trục dài (hash commit) chồng nhau: rút gọn bằng `xLabels` do nơi gọi dựng + xoay bị cấm; thay bằng hiển thị mỗi k nhãn và có đủ trong bảng.

## Ghi chú triển khai (2026-10-07)

Lệch: mỗi hình nhận prop `frame` (id/title/…) và tự bọc `ChartFrame`; `SparklineChart` cho phép dùng gọn không frame (vẫn có role=img + bảng sr-only). `TrendLineChart` có `onSelectPoint` dùng `useChartKeyboardNavigation`. `role=meter` chưa kiểm với trình đọc màn hình. Thêm `chart-line-segments.ts`.
