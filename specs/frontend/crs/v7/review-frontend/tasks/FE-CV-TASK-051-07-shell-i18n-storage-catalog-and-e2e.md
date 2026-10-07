# FE-CV-TASK-051-07: i18n khung, catalog lưu trữ và e2e

**From Solution:** [FE-CV-SOL-051-review-workspace-shell](../solutions/FE-CV-SOL-051-review-workspace-shell.md) mục 8
**Priority:** P1
**Area:** frontend / i18n + docs + e2e
**File:** `i18n/locales/*.json`, `i18n/code-intel-locale-coverage.test.ts` (mở rộng `KEYS`), `specs/frontend/storage/browser-storage-catalog.md` (thêm dòng `orca.review.layout.v1` mục 2), `tests/e2e/review-shell.spec.ts` (mới)
**Depends on:** FE-CV-TASK-051-05, 051-06, FE-CV-TASK-050-20
**Status:** [x] DONE

## Context

- Catalog mục 2 "Layout / panel geometry" (bảng: Key, Type, Feature, Data, file:line, Ephemeral/Important, Duplicates store?).

## Việc cần làm

1. Thêm khoá `auto.components.reviewMap.*` của 051 đủ 5 locale vào `KEYS`.
2. Thêm dòng catalog cho `orca.review.layout.v1` (cosmetic, sole copy).
3. E2E (mẫu `tests/e2e/tasks-page.spec.ts`, chưa chạy): mở tab Review qua store, thấy trạng thái theo fake; cờ tắt ⇒ không tab; xoá worktree khi tab mở.

## Kiểm thử

- `pnpm --dir frontend test -- src/renderer/src/i18n/code-intel-locale-coverage`.

## Tiêu chí hoàn thành

- [ ] Test phủ khoá xanh; catalog cập nhật; e2e có kịch bản.

## Rủi ro

- E2E cần kênh thật hoặc fake nạp vào web build: chưa kiểm chứng cách tiêm fake.
