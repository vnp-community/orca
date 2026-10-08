# FE-REQ-TASK-019-07: Khoá i18n, test phủ khoá và e2e cho danh sách/chi tiết Request

**From Solution:** [FE-REQ-SOL-019](../solutions/FE-REQ-SOL-019-request-list-detail-classification-ui.md) mục 2.7, 5
**Priority:** P1
**Area:** frontend / i18n + e2e
**File:** `frontend/src/renderer/src/i18n/locales/{en,es,ja,ko,zh}.json` (sửa), `frontend/src/renderer/src/i18n/request-locale-coverage.test.ts` (sửa từ 018-05), `tests/e2e/request-list-detail.spec.ts` (mới), `docs/ui/pages/requests.md` (sửa)
**Depends on:** FE-REQ-TASK-019-02 đến 019-06
**Status:** [~] PARTIAL — 5-locale keys + request-locale-coverage.test + docs done and green; e2e request-list-detail.spec.ts not written; verify:localization-* scripts not run

## Context

- Khoá đọc theo tên, tiền tố `auto.components.request.`; mẫu `i18n/task-jira-link-locale-coverage.test.ts` (duyệt từng locale, lấy chuỗi bằng đường dẫn khoá).
- E2E ở `/opt/repos/orca/tests/e2e` (Playwright, Electron); mẫu `tests/e2e/tasks-page.spec.ts` dùng `window.__store`, `helpers/orca-app`, `helpers/store`. Lệnh gốc repo: `npx playwright test tests/e2e/<file> --config tests/playwright.config.ts --project electron-headless` (chưa kiểm chứng; `package.json` gốc khai `test:e2e`).
- Không có backend `request-service` trong môi trường e2e mặc định: e2e phải mock kênh hoặc đánh dấu `test.skip` khi runtime không có `request.flowStatus`.

## Việc cần làm

1. Gom mọi khoá mới của 019-02..06 (danh sách ở SOL-019 2.7) vào 5 file locale; chuỗi dịch có nghĩa, không để tiếng Anh ở locale khác.
2. Mở rộng `request-locale-coverage.test.ts`: danh sách khoá đầy đủ của CR-019 và mô tả 11 loại `RequestType.<type>.label|description`.
3. Chạy các script kiểm danh mục của repo nếu có (`verify:localization-catalog`, `verify:localization-coverage` trong `package.json` gốc; chưa kiểm chứng chạy được) và ghi kết quả.
4. `request-list-detail.spec.ts`: (a) mở trang Requests qua `openRequestPage()`; (b) với runtime giả có kênh: danh sách hiện, mở chi tiết, hộp xác nhận loại hiện, xác nhận gọi `request.confirmType`; (c) runtime không có kênh: nút sidebar ẩn; (d) phím `j/k` đổi hàng. Dùng mock WS hoặc bỏ qua rõ ràng khi thiếu backend.
5. Cập nhật `docs/ui/pages/requests.md` với cây component và phím tắt.

## Kiểm thử

- Vitest: `pnpm --filter orca-frontend test -- src/renderer/src/i18n/request-locale-coverage`.
- E2E: lệnh ở Context; ghi rõ "chưa chạy" nếu môi trường không có Electron.
- Quét "không hex, không emoji" cho `components/request/**` bằng test đơn giản.

## Tiêu chí hoàn thành

- [ ] Mọi khoá của CR-019 có chuỗi ở đủ 5 locale.
- [ ] Test phủ khoá xanh.
- [ ] E2E có (a)-(d) hoặc `test.skip` có lý do ghi trong spec.
- [ ] `docs/ui/pages/requests.md` cập nhật.

## Rủi ro và lưu ý

- Không để khoá "băm" (hash) trong mã: dùng khoá đọc theo tên như các khoá Jira link.
- Dịch máy không được thay review người bản ngữ; đánh dấu cần review.
