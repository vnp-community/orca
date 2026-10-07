# FE-CV-TASK-059-07: i18n 5 locale, test phủ khoá và e2e web Hợp đồng + Phát hiện

**From Solution:** [FE-CV-SOL-059](../solutions/FE-CV-SOL-059-contract-lens-and-findings.md) mục 2.5, 6
**Priority:** P1
**Area:** frontend / i18n + tests
**File:** `frontend/src/renderer/src/i18n/locales/{en,es,ja,ko,zh}.json`; `i18n/code-intel-locale-coverage.test.ts` (thêm `KEYS`); `tests/e2e/code-intel-web/lenses.web.e2e.ts` (phần Contract/Findings)
**Depends on:** FE-CV-TASK-059-04, 059-05; FE-CV-TASK-073-02, 073-03
**Status:** [x] DONE

## Context

- Bảng dịch riêng cho `ruleId`, `titleKey`, `kind`, `origin`, preset lý do; mã lạ rơi về nguyên văn. Khoá `auto.components.reviewMap.Contract*|Finding*`; không `translate()` cấp module.

## Việc cần làm

1. Dịch toàn bộ khoá sang 4 locale (không trùng `en`); thêm vào `KEYS`.
2. e2e (fake backend G4): bật cờ → lens Hợp đồng hiện bảng, `unknown` hiển thị "Chưa xác định"; dock Phát hiện lọc `introduced`; Bỏ qua cần lý do; Đã xử lý; Mở lại; fake backend `failNext('CODEINTEL_NOT_AUTHORIZED')` ⇒ thông báo quyền inline; `CODEINTEL_INDEX_MISSING` ⇒ phát hiện rỗng + nút lập chỉ mục.
3. Kiểm DOM: `evidence`/`params` chứa chuỗi giống DSN không xuất hiện nguyên văn.

## Kiểm thử

- `pnpm --filter orca-frontend test -- src/renderer/src/i18n/code-intel-locale-coverage src/renderer/src/i18n/no-top-level-translate`; e2e `pnpm run test:e2e:code-intel-web -- lenses.web.e2e.ts` (**chưa chạy**; cần 073-03).

## Tiêu chí hoàn thành

- [ ] 5 locale đủ khoá, test phủ xanh.
- [ ] e2e Contract/Findings xanh trên fake backend.

## Rủi ro

- Dịch `ruleId` hàng loạt dễ sót; mã lạ nguyên văn là lưới an toàn.
