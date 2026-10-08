# FE-CV-TASK-062-02: Loader, ánh xạ `unavailable` và mô hình lọc/sắp xếp

**From Solution:** [FE-CV-SOL-062](../solutions/FE-CV-SOL-062-mobile-review-summary.md) mục 2.3
**Priority:** P2
**Area:** mobile / session (hàm thuần + loader)
**File:** `mobile/src/session/mobile-review-summary-loaders.ts`, `mobile-review-summary-model.ts` (mới) + test
**Depends on:** FE-CV-TASK-062-01
**Status:** [x] DONE (verified 2026-10-07: vitest mobile-review-summary-loaders.test.ts + -model.test.ts PASS; oxlint clean)

## Context

- Mẫu: `mobile-diff-review-loaders.ts` (`client.sendRequest`, `response.ok`, `response.error?.code/message`), `isMobileGitUnavailable` (`mobile-git-status.ts:93`).

## Việc cần làm

1. `loadMobileReviewSummary(client, worktreeId)` → `{kind:'ready'|'unavailable'|'error', …}`: `sendRequest('codeIntel.reviewSummary', {worktree:`id:${worktreeId}`})`; `forbidden`/`method_not_found`/"not available to mobile clients" ⇒ `unavailable` (host chưa hỗ trợ); `available:false` ⇒ `unavailable` theo `reason` (mỗi lý do một thông điệp + hướng xử lý, không "an toàn").
2. Model: `filterFindings({severity, onlyIntroduced})`, `sortFindings` (severity rồi origin), `riskLabel`, `formatTimeAgo`.

## Kiểm thử

- `RpcClient` giả: ok/forbidden/method_not_found/lỗi khác/`available:false` từng reason; lọc/sắp xếp.
- `pnpm --dir mobile test -- mobile-review-summary` (chưa kiểm chứng).

## Tiêu chí hoàn thành

- [ ] Test xanh.
- [ ] `unavailable` không bao giờ thành `error`.

## Rủi ro

- Thông điệp `reason` chưa có bản dịch (mobile không i18n).

## Ghi chú triển khai (2026-10-07)

- Thêm `canOpenMobileReviewFindingDiff`, `mobileReviewTruncationLabel` ở model. Thông điệp `unavailable` bằng tiếng Anh cố định (mobile không i18n).
