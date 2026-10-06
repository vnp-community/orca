# FE-CV-TASK-093-02: Hook `useReviewAiSummary`

**From Solution:** [FE-CV-SOL-093-ai-summary-panel](../solutions/FE-CV-SOL-093-ai-summary-panel.md) mục 2.3
**Priority:** P0
**Area:** frontend
**File:** `frontend/src/renderer/src/components/review-map/ai-summary/use-review-ai-summary.ts`, `ai-summary-consent-state.ts` (mới) + test
**Depends on:** FE-CV-TASK-085-01, 093-01; FE-CV-SOL-050-store-and-query-hooks
**Status:** [ ] TODO

## Context

- `quality.summary`: `dryRun`, `level`, `forceRefresh`, `locale`; `CODEINTEL_TIMEOUT` `inProgress` thử lại (PQ-13); quyền `quality_read ∧ read_source`.

## Việc cần làm

1. `hidden` khi `flags.ai=false`; `allowedLevels` ≤ `tenant.aiReviewLevel`.
2. `preview` (dryRun) → `confirmAndGenerate`; xác nhận một lần mỗi phiên và mỗi mức.
3. Retry mỗi `retryAfterMs` ≤ 90 s, huỷ bằng `AbortSignal`.
4. Ánh xạ lỗi: ai-disabled/quality-disabled → hidden; no-relay; bad-output; rate-limited; forbidden; timeout.

## Kiểm thử

- Cờ tắt 0 lời gọi; preview không tạo; retry đồng hồ giả; huỷ; từng lỗi.
- Chạy: `pnpm --filter orca-frontend test -- <đường dẫn>` (chưa chạy).

## Tiêu chí hoàn thành

- [ ] Không tự gọi tạo.
- [ ] Không lưu bền.

## Rủi ro

- Thời gian hoàn tất nền chưa đo.
