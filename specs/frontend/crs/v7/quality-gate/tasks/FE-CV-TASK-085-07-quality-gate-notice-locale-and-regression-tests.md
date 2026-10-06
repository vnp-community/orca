# FE-CV-TASK-085-07: Khoá i18n năm locale và test hồi quy

**From Solution:** [FE-CV-SOL-085-source-control-quality-notice](../solutions/FE-CV-SOL-085-source-control-quality-notice.md) mục 5, 6
**Priority:** P1
**Area:** frontend
**File:** `frontend/src/renderer/src/i18n/locales/{en,es,ja,ko,zh}.json` (sửa), `frontend/src/renderer/src/i18n/quality-gate-notice-locale-coverage.test.ts` (mới)
**Depends on:** FE-CV-TASK-085-02, 085-04
**Status:** [ ] TODO

## Context

- Khoá đọc theo tên không được sinh tự động: test phủ theo mẫu `task-jira-link-locale-coverage.test.ts`.
- Tool đồng bộ: `desktop/config/scripts/verify-localization-catalog.mjs` (script gốc `pnpm sync:localization-catalog`, chưa kiểm chứng chạy được sau khi tách monorepo).
- `no-top-level-translate.test.ts` cấm `translate()` ở top-level module.

## Việc cần làm

1. Liệt kê mọi khoá `qualityGateNotice.*` trong test; thêm giá trị năm locale.
2. Không gọi `translate()` ở top-level.

## Kiểm thử

- Test phủ khoá cho en/es/ja/ko/zh.
- Chạy: `pnpm --filter orca-frontend test -- <đường dẫn>` (chưa chạy).

## Tiêu chí hoàn thành

- [ ] Mọi locale có khoá không rỗng.
- [ ] `no-top-level-translate` xanh.

## Rủi ro

- Bản dịch cần người duyệt ngôn ngữ.
