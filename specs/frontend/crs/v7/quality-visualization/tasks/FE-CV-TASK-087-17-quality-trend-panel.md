# FE-CV-TASK-087-17: `QualityTrendPanel`: xu hướng theo lượt

**From Solution:** [FE-CV-SOL-087-quality-trend-coverage-hotspot](../solutions/FE-CV-SOL-087-quality-trend-coverage-hotspot.md) mục 2.2, 2.3
**Priority:** P1
**Area:** frontend / components (pha 2)
**File:** `frontend/src/renderer/src/components/review-map/quality/QualityTrendPanel.tsx` (mới) và `*.test.tsx`
**Depends on:** 087-15, FE-CV-TASK-088-07
**Status:** [x] DONE (verified 2026-10-07: 12 test pass, components/review-map/quality/QualityTrendPanel.test.tsx; kèm QualityTrendPointTable)

## Context

- `TrendLineChart`, `SparklineChart` (088-07). `QualityTrendPoint.source` (`local|ci`), `verdict`, `turnKey`.
- So sánh lượt thuộc FE-CV-SOL-060 (`ReviewTurnSwitcher`; tên action chưa chốt).

## Việc cần làm

1. `ChartFrame` + `TrendLineChart` (error/warning/info) + ◆ đổi kết luận + `SparklineChart` diff coverage.
2. `groupBy` chuyển `commit|turn` (mặc định `turn` nếu có `turnKey`).
3. < 2 điểm → số + "Cần ít nhất hai lượt để thấy xu hướng"; không đường.
4. Bấm điểm có `turnKey` khớp → hành động so sánh của 060 nếu tồn tại, nếu không tắt (không nút chết).
5. Nguồn local/CI trong tooltip và bảng.

## Kiểm thử

1/2/50/51 điểm; `metrics` vắng; đổi `groupBy`; bấm điểm khi 060 chưa có hành động; `source:'ci'`. Chạy `pnpm --filter orca-frontend test -- src/renderer/src/components/review-map/quality/QualityTrendPanel`.

## Tiêu chí hoàn thành

- [ ] Không kết luận "cải thiện"; chỉ nêu số.
- [ ] Không nút chết khi 060 chưa có.

## Rủi ro

- `turnKey` định dạng chưa nêu.

## Ghi chú triển khai (2026-10-07)

- **Mặc định `groupBy='turn'`** (không đoán trước được `turnKey` có hay không trước khi tải); người dùng chuyển `commit` bằng nút nhóm. `turnKey` D5 = `"${paneKey}:${doneAt}"`.
- **Bấm điểm:** panel nhận thêm hai prop tuỳ chọn `turnLabels: Map<turnKey, nhãn>` và `onCompareTurn(turnKey)`. Chỉ khi có `onCompareTurn` thì biểu đồ mới tương tác (không `onCompareTurn` -> không ô chọn, không nút chết); chỉ điểm có `turnKey` nằm trong `turnLabels` mới gọi so sánh. Lead cần nối hai prop này khi action so sánh lượt của 060 (`ReviewTurnSwitcher`) lộ ra; hiện chưa nối.
- **Nguồn local/CI:** bảng chi tiết có chữ + glyph (● Cục bộ / ◇ CI), dòng "Figures at HEAD ... · nguồn"; sparkline `metrics.diffCoverage` đổi ratio -> % và để `null` khi vắng (đường ngắt, test không 0).
- ◆ đổi kết luận do `TrendLineChart` vẽ từ `marker`; chỉ marker khi verdict khác điểm trước. Cắt 50 điểm mới nhất + "X/Y". Panel không dùng từ "cải thiện/tệ hơn" (có test).
