# FE-CV-TASK-093-03: Dialog xem trước dữ liệu gửi và xác nhận

**From Solution:** [FE-CV-SOL-093-ai-summary-panel](../solutions/FE-CV-SOL-093-ai-summary-panel.md) mục 2.3, 2.4
**Priority:** P0
**Area:** frontend
**File:** `frontend/src/renderer/src/components/review-map/ai-summary/AiSummaryDataPreviewDialog.tsx` (mới) + test
**Depends on:** FE-CV-TASK-093-02
**Status:** [ ] TODO

## Context

- STYLEGUIDE: `Dialog` cho quyết định cần trước khi tiếp tục; Hủy là ghost, không destructive; focus mặc định an toàn.

## Việc cần làm

1. Hiện tệp, bytes, số lần che, `withheld`, ước lượng token, cảnh báo `suspectedInjection`, dòng nêu dữ liệu tới nhà cung cấp LLM.
2. Focus mặc định nút Hủy; nút "Gửi và tạo tóm tắt" là `default`.
3. Khoá nút ngay khi gửi.

## Kiểm thử

- Hiển thị manifest thiếu trường; Enter không gửi khi focus Hủy; Esc đóng.
- Chạy: `pnpm --filter orca-frontend test -- <đường dẫn>` (chưa chạy).

## Tiêu chí hoàn thành

- [ ] Chữ không overclaim ("sẽ gửi", không "an toàn").

## Rủi ro

- Đường dẫn trong manifest hiển thị thuần.
