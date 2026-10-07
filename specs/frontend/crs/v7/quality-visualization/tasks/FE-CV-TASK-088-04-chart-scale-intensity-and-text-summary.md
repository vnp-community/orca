# FE-CV-TASK-088-04: Hàm thuần: thang tuyến tính, bậc cường độ, mô tả văn bản

**From Solution:** [FE-CV-SOL-088](../solutions/FE-CV-SOL-088-graphics-foundation-and-chart-primitives.md) mục 2.5, 2.7
**Priority:** P0
**Area:** frontend / pure functions
**File:** `frontend/src/renderer/src/components/quality-charts/chart-linear-scale.ts`, `heat-intensity-scale.ts`, `chart-text-summary.ts` (mới) và `__tests__/*.test.ts`
**Depends on:** FE-CV-TASK-088-03 (nhãn mức dùng trong mô tả)
**Status:** [x] DONE

## Context

- Trục X của xu hướng là **hạng mục rời** (lượt), không phải thời gian liên tục (CR-088 2.2-A, 3): chỉ cần thang tuyến tính Y và tick đẹp.
- PQ-33: `metrics` vắng = không có số, không phải 0 → hàm nhận `number | null`.
- CR-088 2.5: mô tả văn bản **chỉ nêu số đo**, không kết luận ("cải thiện", "an toàn").
- Điều kiện xem lại sang `d3-scale` (SOL-088 2.1): thang/tick vượt ~120 dòng hoặc lỗi lặp lại → ghi vào PR.

## Việc cần làm

1. `chart-linear-scale.ts`: `createLinearScale(domain: [number, number], range: [number, number])` trả `{ scale(v), invert(px), ticks(count) }`; `niceDomain(min, max)`; miền suy biến (`min===max`) mở rộng 1 đơn vị; không ném với `NaN`/`Infinity` (trả thang rỗng và `ticks=[]`). `ticks(count)` bước 1/2/5×10^k, không lặp, không vượt miền.
2. `heat-intensity-scale.ts`: `bucketIntensity(value, {min,max})` → 1..5 (phân vị cố định đều nhau trên miền; `min===max` → 3; ngoài miền kẹp); `bucketIntensityByQuantile(values: number[])` (tuỳ chọn cho hotspot, ổn định khi có giá trị trùng). Giá trị `null` **không** có bậc (trả `null`), hình hiển thị "—".
3. `chart-text-summary.ts`: `describeSeriesRange({label, points: (number|null)[]})` → câu qua `translate()` kiểu "{label}: từ {first} xuống/lên/giữ {last} qua {n} lượt"; `describeCoverage({covered,total,source})`; `describeGrid({rows, columns, shown, total})`. Điểm `null` được nêu ("{k} lượt không có số"). Không có từ kết luận. Mã/nhãn lạ không ném. Động từ "tăng/giảm" là mô tả số học, chấp nhận; không dùng "tốt hơn/xấu đi".
4. Khoá i18n `auto.components.qualityCharts.summary.*`, đọc theo tên.

## Kiểm thử

- Thang: miền suy biến, âm, rất lớn, ticks 5/10, `NaN`; `invert(scale(v)) ≈ v`.
- Bậc: biên (min→1, max→5), trùng giá trị, `null`.
- Mô tả: `src` quét chuỗi mặc định của file không chứa `an toàn|sạch|cải thiện|tốt hơn|xấu đi` (và bản tiếng Anh `safe|clean|improved`); dãy toàn `null`; một điểm; hai điểm.
- Chạy: `pnpm --filter orca-frontend test -- src/renderer/src/components/quality-charts/__tests__/chart-`.

## Tiêu chí hoàn thành

- [ ] Ba hàm thuần có test, không phụ thuộc DOM.
- [ ] Mô tả chỉ nêu số đo.
- [ ] Dưới 120 dòng cho thang + tick (nếu vượt, ghi lý do trong PR theo điều kiện A2).

## Rủi ro

- Làm tròn tick với số thập phân nhị phân (0,1+0,2); dùng bước nguyên × lũy thừa 10 rồi nhân, không cộng dồn.
