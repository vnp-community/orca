# FE-CV-TASK-053-08: Đăng ký lens Ảnh hưởng, i18n, e2e

**From Solution:** [FE-CV-SOL-053-impact-lens-and-symbol-detail](../solutions/FE-CV-SOL-053-impact-lens-and-symbol-detail.md) mục 8
**Priority:** P1
**Area:** frontend / review-map + i18n
**File:** `review-lens-registry.ts` (thêm mục `impact`, mặc định), `ReviewDetailDrawer` (mặc định `SymbolDetailPanel`), `i18n/locales/*.json`, `code-intel-locale-coverage.test.ts`, `tests/e2e/review-impact.spec.ts` (mới)
**Depends on:** FE-CV-TASK-053-03, 053-04, 053-06
**Status:** [x] DONE

## Context

- Registry: chỉ lens đã đăng ký có tab; lens Ảnh hưởng là mặc định.

## Việc cần làm

1. Đăng ký `impact` (icon `lucide-react`, `labelKey`, `load` lười).
2. Thêm khoá i18n của 053 (mã hoá lớp phủ, panel, nhãn `sourceOmitted`, thông báo không cạnh) đủ 5 locale; copy "Chưa tìm thấy ..." không overclaim.
3. E2E kịch bản: chọn symbol ⇒ lens + panel ⇒ "Xem diff" cuộn đúng dòng (fake backend).

## Kiểm thử

- `pnpm --dir frontend test -- src/renderer/src/i18n/code-intel-locale-coverage`; e2e chưa chạy.

## Tiêu chí hoàn thành

- [ ] Lens hiển thị là tab đầu; test phủ khoá xanh.

## Rủi ro

- E2E phụ thuộc cách tiêm backend giả vào web build (chưa kiểm).
