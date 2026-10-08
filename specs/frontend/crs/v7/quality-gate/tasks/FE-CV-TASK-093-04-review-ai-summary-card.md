# FE-CV-TASK-093-04: Thẻ tóm tắt AI

**From Solution:** [FE-CV-SOL-093-ai-summary-panel](../solutions/FE-CV-SOL-093-ai-summary-panel.md) mục 2.3, 2.4
**Priority:** P1
**Area:** frontend
**File:** `frontend/src/renderer/src/components/review-map/ai-summary/ReviewAiSummaryCard.tsx` (mới) + test; chỗ đặt trong khung FE-CV-SOL-051
**Depends on:** FE-CV-TASK-093-02, 093-03
**Status:** [~] PARTIAL — thẻ đã mount trong `ReviewCompanionStrip` và chỉ hiện khi cờ AI bật (mặc định tắt; ReviewWorkspace.companions.test 8/8 PASS); thiếu: nút phản hồi vẫn chỉ render khi có `onFeedback` — chưa có sự kiện telemetry phản hồi nối

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

## Ghi chú triển khai (2026-10-07)

Thẻ hoàn chỉnh + test (injection hiển thị dạng chữ, ref ngoài tập tệp đổi không bấm được) nhưng chưa mount trong shell Review; nút phản hồi chỉ render khi có `onFeedback` (chưa có sự kiện telemetry phản hồi nối).

## Ghi chú tích hợp (W6, 2026-10-07)

Hook `useReviewAiSummary` được gọi ở `use-review-companions.ts`: cờ tắt ⇒ `hidden`, không RPC. `onFeedback` chưa truyền.
