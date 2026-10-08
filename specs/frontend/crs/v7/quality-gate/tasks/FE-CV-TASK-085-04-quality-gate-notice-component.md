# FE-CV-TASK-085-04: Component `SourceControlQualityGateNotice`

**From Solution:** [FE-CV-SOL-085-source-control-quality-notice](../solutions/FE-CV-SOL-085-source-control-quality-notice.md) mục 2.5
**Priority:** P0
**Area:** frontend
**File:** `frontend/src/renderer/src/components/right-sidebar/source-control-quality-gate-notice.tsx` (mới) + `.test.tsx`
**Depends on:** FE-CV-TASK-085-02, 085-03
**Status:** [x] DONE (verified 2026-10-07: 7 tests notice.test.tsx)

## Context

- STYLEGUIDE: token `border`/`muted-foreground`/`destructive`, lucide, `Button variant="outline" size="xs"`, không hex, không emoji; chữ không overclaim.
- Tên PR/MR từ `localizedHostedReviewCopy(resolveSupportedHostedReviewCopyProvider(provider))`.

## Việc cần làm

1. Prop `{gate: SourceControlQualityGate, provider}`; render theo view-model; `role="status"`, `aria-live="polite"`, không `role="alert"`.
2. Icon khác nhau cho warn/fail/unknown/running (không chỉ màu); spinner `Loader2` hiện sau ~200 ms, nút khoá ngay.
3. "Xem lý do" → `openReason`; "Chạy kiểm tra" chỉ khi `canRunChecks`.
4. Chuỗi qua `translate()`; reason hiển thị văn bản thuần.

## Kiểm thử

- Mỗi view; GitLab → "merge request"; reason chứa `<b>` hiện nguyên chữ; nút khoá ngay (`// @vitest-environment happy-dom`).
- Chạy: `pnpm --filter orca-frontend test -- <đường dẫn>` (chưa chạy).

## Tiêu chí hoàn thành

- [ ] `pass`/hidden render null.
- [ ] Không có cụm "an toàn", "đã đáp ứng".

## Rủi ro

- Chiều rộng sidebar hẹp: dùng `break-words`.

## Ghi chú triển khai (2026-10-07)

`translate` là prop tuỳ chọn, mặc định `translateCatalogKey` (i18n/catalog-key-translate.ts).
