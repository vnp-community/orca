# FE-CV-TASK-093-06: Khoá i18n và test hiển thị injection

**From Solution:** [FE-CV-SOL-093-ai-summary-panel](../solutions/FE-CV-SOL-093-ai-summary-panel.md) mục 5, 6
**Priority:** P1
**Area:** frontend
**File:** `frontend/src/renderer/src/i18n/ai-summary-locale-coverage.test.ts` (mới), `frontend/src/renderer/src/i18n/locales/*.json`
**Depends on:** FE-CV-TASK-093-03, 093-04
**Status:** [x] DONE (verified 2026-10-07: 7 tests ai-summary-locale-coverage.test.ts + 12 tests card (injection))

## Context

- Khoá đọc theo tên cần test phủ năm locale.

## Việc cần làm

1. Phủ khoá; bộ ca injection hiển thị tổng hợp; quét cụm cấm ở khoá en.

## Kiểm thử

- Chạy: `pnpm --filter orca-frontend test -- <đường dẫn>` (chưa chạy).

## Tiêu chí hoàn thành

- [ ] Năm locale đủ khoá.
- [ ] Không cụm "AI đã review".

## Rủi ro

- Dịch cần duyệt.

## Ghi chú triển khai (2026-10-07)

41 khoá locale năm ngôn ngữ.
