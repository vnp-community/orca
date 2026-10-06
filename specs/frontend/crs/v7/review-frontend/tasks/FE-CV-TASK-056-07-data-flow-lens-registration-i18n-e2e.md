# FE-CV-TASK-056-07: Đăng ký lens Luồng, i18n, e2e

**From Solution:** [FE-CV-SOL-056-dataflow-lens](../solutions/FE-CV-SOL-056-dataflow-lens.md) mục 8
**Priority:** P1
**Area:** frontend / review-map + i18n
**File:** `review-lens-registry.ts` (thêm `dataflow`), `i18n/locales/*.json`, `code-intel-locale-coverage.test.ts`, `tests/e2e/review-dataflow.spec.ts` (mới)
**Depends on:** FE-CV-TASK-056-06
**Status:** [ ] TODO

## Context

- Chip `flows` (051-04) chuyển lens; "Luồng liên quan" (053-04) gọi `setReviewLens('dataflow')`.

## Việc cần làm

1. Đăng ký lens; nối chip `flows` (bật công tắc "chạm thay đổi") và "Luồng liên quan" (`query=label`).
2. Khoá i18n (loại bước, `partial`, gap, export, "Suy luận", "đã đổi") đủ 5 locale.
3. Kiểm tay bắt buộc: sơ đồ 60 tin nhắn sáng/tối, phóng 200 %, tải `.svg` trong Electron đóng gói; ghi vào PR.
4. E2E: chọn luồng, danh sách bước, sao chép (fake backend).

## Kiểm thử

- Coverage i18n; e2e chưa chạy.

## Tiêu chí hoàn thành

- [ ] Lens mở được từ chip và panel; id lạ không lỗi.

## Rủi ro

- Điều hướng "Luồng liên quan" chỉ best-effort tới khi BE xác nhận id.
