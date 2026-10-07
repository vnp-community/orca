# FE-CV-TASK-050-20: i18n và test phủ khoá `code-intel-locale-coverage`

**From Solution:** [FE-CV-SOL-050-review-tab-wiring](../solutions/FE-CV-SOL-050-review-tab-wiring.md) mục 4.6
**Priority:** P0
**Area:** frontend / i18n
**File:** `frontend/src/renderer/src/i18n/code-intel-locale-coverage.test.ts` (mới), `i18n/locales/{en,es,ja,ko,zh}.json` (sửa)
**Depends on:** không
**Status:** [x] DONE

## Context

- Mẫu `i18n/task-jira-link-locale-coverage.test.ts`; `i18n/no-top-level-translate.test.ts` cấm `translate()` cấp module. Tiền tố `auto.components.reviewMap.` và `auto.hooks.codeIntel.`.

## Việc cần làm

1. Test với mảng `KEYS` (rỗng ban đầu + khoá của 050: thông báo tab không khả dụng, nhãn tab "Review"): mỗi khoá là chuỗi không rỗng ở 5 locale; 4 locale ≠ `en` không trùng văn bản `en`.
2. Thêm khoá `auto.components.reviewMap.ReviewTabUnavailableNotice.*` (copy không overclaim, ví dụ "Tính năng Review chưa bật cho tổ chức này") đủ 5 locale.
3. Quy ước: mỗi solution sau thêm khoá vào `KEYS` cùng PR.

## Kiểm thử

- Chạy test mới và `no-top-level-translate.test.ts`.

## Tiêu chí hoàn thành

- [ ] Test xanh; 5 locale đủ khoá.

## Rủi ro

- Văn bản ja/ko/zh cần người dịch: không dùng bản dịch máy chưa rà cho thông báo nhạy cảm.
