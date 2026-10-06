# FE-CV-TASK-093-04: Thẻ tóm tắt AI

**From Solution:** [FE-CV-SOL-093-ai-summary-panel](../solutions/FE-CV-SOL-093-ai-summary-panel.md) mục 2.3, 2.4
**Priority:** P1
**Area:** frontend
**File:** `frontend/src/renderer/src/components/review-map/ai-summary/ReviewAiSummaryCard.tsx` (mới) + test; chỗ đặt trong khung FE-CV-SOL-051
**Depends on:** FE-CV-TASK-093-02, 093-03
**Status:** [ ] TODO

## Context

- Nhãn bắt buộc; văn bản thuần; thu gọn mặc định.

## Việc cần làm

1. Nhãn, model, level, `refsDropped`, nút Tạo lại (`forceRefresh`), Hữu ích/Không đúng (gọi wrapper telemetry nếu có).
2. Trạng thái generating/lỗi/hết hạn; spinner sau ~200 ms.
3. Không import hook cổng chất lượng.

## Kiểm thử

- Injection hiển thị; nhãn luôn có; cụm cấm; không render khi hidden (`happy-dom`).
- Chạy: `pnpm --filter orca-frontend test -- <đường dẫn>` (chưa chạy).

## Tiêu chí hoàn thành

- [ ] Không `dangerouslySetInnerHTML`.

## Rủi ro

- Chỗ đặt phụ thuộc 051.
