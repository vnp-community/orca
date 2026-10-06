# FE-CV-TASK-093-01: Parser phản hồi và bảo vệ văn bản

**From Solution:** [FE-CV-SOL-093-ai-summary-panel](../solutions/FE-CV-SOL-093-ai-summary-panel.md) mục 2.2, bảng sửa 4-6
**Priority:** P0
**Area:** frontend
**File:** `frontend/src/renderer/src/components/review-map/ai-summary/ai-summary-wire-parser.ts` (mới) + test
**Depends on:** FE-CV-SOL-050-types-and-runtime-bridge
**Status:** [ ] TODO

## Context

- Hợp đồng 4.7 `AiReviewSummary`; `manifest`/`cache`/`labels` chưa có kiểu trong hợp đồng.
- U9: văn bản không tin cậy.

## Việc cần làm

1. `parseAiSummaryResponse`: chịu thiếu trường, `aiInferred`/`ai_inferred`; cắt độ dài hiển thị (summary 600, risk 200, why 120).
2. `isRefAllowed(ref, changedFiles)` kiểm lần hai.
3. Loại ký tự điều khiển/bidi khi hiển thị.

## Kiểm thử

- Fixture thiếu trường, dài, bidi, ref lạ.
- Chạy: `pnpm --filter orca-frontend test -- <đường dẫn>` (chưa chạy).

## Tiêu chí hoàn thành

- [ ] Hàm thuần; không trả HTML.

## Rủi ro

- Kiểu manifest là đề xuất.
