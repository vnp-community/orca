# FE-CV-TASK-090-07: Chèn báo cáo vào mô tả PR/MR

**From Solution:** [FE-CV-SOL-090-review-report-export](../solutions/FE-CV-SOL-090-review-report-export.md) mục 2.4
**Priority:** P0
**Area:** frontend
**File:** `frontend/src/renderer/src/components/review-map/report/merge-review-report-into-body.ts` (mới); `frontend/src/renderer/src/components/right-sidebar/CreateHostedReviewComposer.tsx`, `CreateHostedReviewComposerFields.tsx`, `SourceControl.tsx`, `ChecksPanel.tsx` (sửa)
**Depends on:** FE-CV-TASK-090-05
**Status:** [x] DONE (verified 2026-10-07: 7 tests merge + 5 tests use-insert-review-report + 3 tests nút trong composer)

## Context

- Hai nơi gọi composer sản phẩm; `generating` khoá field; `useTemplate` chưa rõ.

## Việc cần làm

1. Chạy GitNexus `impact` trước khi sửa (chưa chạy).
2. `mergeReviewReportIntoBody` idempotent.
3. Prop `onInsertReviewReport`; nút disable khi `generating`/`fieldsLocked`.

## Kiểm thử

- Có/không khối; CRLF; hai khối; không chèn khi generating; GitLab chữ MR; test composer hiện có xanh.
- Chạy: `pnpm --filter orca-frontend test -- <đường dẫn>` (chưa chạy).

## Tiêu chí hoàn thành

- [ ] Không đổi `createDisabled`.

## Rủi ro

- Đua với field revisions.

## Ghi chú triển khai (2026-10-07)

Nút "Chèn báo cáo review" trong `CreateHostedReviewComposerFields`, nối ở SourceControl và ChecksPanel qua `useInsertReviewReport`; ẩn khi cờ quality tắt, khoá khi đang generate.
