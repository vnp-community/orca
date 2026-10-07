# FE-CV-TASK-055-07: Đăng ký lens Kiến trúc, i18n, e2e

**From Solution:** [FE-CV-SOL-055-architecture-c4-lens](../solutions/FE-CV-SOL-055-architecture-c4-lens.md) mục 8
**Priority:** P1
**Area:** frontend / review-map + i18n
**File:** `review-lens-registry.ts` (thêm `architecture`), `i18n/locales/*.json`, `code-intel-locale-coverage.test.ts`, `tests/e2e/review-architecture.spec.ts` (mới)
**Depends on:** FE-CV-TASK-055-03, 055-04, 055-06
**Status:** [x] DONE

## Context

- Registry chỉ lens đã đăng ký có tab; thứ tự: Ảnh hưởng, Kiến trúc, …

## Việc cần làm

1. Đăng ký lens (icon `lucide-react`, tải lười).
2. Khoá i18n (nhãn lớp, loại quan hệ, nhãn `origin`, trình soạn, lỗi) đủ 5 locale; copy không overclaim.
3. Kiểm tay trên ít nhất một container TypeScript (`frontend`) để đánh giá bố cục hexagonal; ghi kết quả vào PR.
4. E2E: chọn container, mở trình soạn, lưu, conflict (fake backend).

## Kiểm thử

- Coverage i18n; e2e chưa chạy.

## Tiêu chí hoàn thành

- [ ] Tab hiện khi đăng ký; test phủ khoá xanh.

## Rủi ro

- Cần dữ liệu thật để đánh giá chất lượng sơ đồ (heuristic).
