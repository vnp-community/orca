# FE-CV-TASK-093-05: Mục Markdown có nhãn cho báo cáo (090)

**From Solution:** [FE-CV-SOL-093-ai-summary-panel](../solutions/FE-CV-SOL-093-ai-summary-panel.md) bảng sửa 8
**Priority:** P2
**Area:** frontend
**File:** `frontend/src/renderer/src/components/review-map/ai-summary/ai-summary-report-section.ts` (mới) + test
**Depends on:** FE-CV-TASK-093-01; FE-CV-SOL-090-review-report-export (`extraSections`)
**Status:** [x] DONE — `ai-summary-report-section.ts` chưa tồn tại. Rà soát 2026-10-07.

## Context

- Báo cáo chỉ nhận khi người dùng chủ động; mục nằm ngoài "Cổng chất lượng"; thoát ký tự Markdown.

## Việc cần làm

1. Hàm thuần dựng mục có nhãn "Tóm tắt do AI suy luận", model, level; thoát ký tự; cắt theo ngân sách.
2. Không thêm nút trong composer (câu hỏi mở 3).

## Kiểm thử

- Thoát `|`, backtick, `<`, `[`; vắng summary → không mục.
- Chạy: `pnpm --filter orca-frontend test -- <đường dẫn>` (chưa chạy).

## Tiêu chí hoàn thành

- [ ] Nhãn luôn có.

## Rủi ro

- Chưa có điểm gọi.
