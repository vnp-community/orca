# FE-CV-TASK-090-08: Khoá i18n và test XSS

**From Solution:** [FE-CV-SOL-090-review-report-export](../solutions/FE-CV-SOL-090-review-report-export.md) mục 5, 6
**Priority:** P1
**Area:** frontend
**File:** `frontend/src/renderer/src/i18n/review-report-locale-coverage.test.ts` (mới), `frontend/src/renderer/src/i18n/locales/*.json`
**Depends on:** FE-CV-TASK-090-02, 090-04
**Status:** [x] DONE — locale và XSS test file chưa tồn tại. Rà soát 2026-10-07.

## Context

- Khoá đọc theo tên cần test phủ năm locale.

## Việc cần làm

1. Liệt kê khoá và dịch.
2. Test XSS tổng hợp Markdown + HTML.

## Kiểm thử

- Chạy: `pnpm --filter orca-frontend test -- <đường dẫn>` (chưa chạy).

## Tiêu chí hoàn thành

- [ ] Năm locale đủ khoá.

## Rủi ro

- Dịch cần duyệt.
