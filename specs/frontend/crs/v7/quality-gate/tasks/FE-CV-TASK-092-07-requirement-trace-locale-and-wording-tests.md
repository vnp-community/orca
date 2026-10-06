# FE-CV-TASK-092-07: Khoá i18n và test cấm từ

**From Solution:** [FE-CV-SOL-092-requirement-trace-view](../solutions/FE-CV-SOL-092-requirement-trace-view.md) mục 5, 6
**Priority:** P1
**Area:** frontend
**File:** `frontend/src/renderer/src/i18n/requirement-trace-locale-coverage.test.ts` (mới), `frontend/src/renderer/src/i18n/locales/*.json`
**Depends on:** FE-CV-TASK-092-01, 092-05
**Status:** [ ] TODO

## Context

- Khoá đọc theo tên cần test phủ năm locale.

## Việc cần làm

1. Phủ khoá; quét giá trị en/es/ja/ko/zh không có cụm cấm (bản en bắt buộc).

## Kiểm thử

- Chạy: `pnpm --filter orca-frontend test -- <đường dẫn>` (chưa chạy).

## Tiêu chí hoàn thành

- [ ] Năm locale đủ khoá.

## Rủi ro

- Dịch cần duyệt; kiểm cụm cấm ở locale khác là thủ công.
