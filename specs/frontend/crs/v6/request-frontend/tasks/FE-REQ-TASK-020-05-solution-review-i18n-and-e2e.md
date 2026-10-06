# FE-REQ-TASK-020-05: i18n, test phủ khoá và e2e duyệt Solution

**From Solution:** [FE-REQ-SOL-020](../solutions/FE-REQ-SOL-020-solution-review-ui.md) mục 2.6, 5
**Priority:** P1
**Area:** frontend / i18n + e2e
**File:** `frontend/src/renderer/src/i18n/locales/{en,es,ja,ko,zh}.json` (sửa), `frontend/src/renderer/src/i18n/request-locale-coverage.test.ts` (sửa), `tests/e2e/request-solution-review.spec.ts` (mới), `docs/ui/pages/requests.md` (sửa)
**Depends on:** FE-REQ-TASK-020-02 đến 020-04
**Status:** [ ] TODO

## Context

- Tiền tố `auto.components.request.solution.`; mẫu test `i18n/task-jira-link-locale-coverage.test.ts`.
- E2E: `tests/e2e/` (Playwright, `tasks-page.spec.ts` làm mẫu: `window.__store`, `helpers/orca-app`). Cần backend CR-REQ-007/008/009 hoặc mock WS; nếu không có thì `test.skip` có lý do.
- Lệnh e2e từ gốc repo (chưa kiểm chứng): `npx playwright test tests/e2e/request-solution-review.spec.ts --config tests/playwright.config.ts --project electron-headless`.

## Việc cần làm

1. Thêm khoá còn thiếu của CR-020 2.6 và SOL-020 2.6 vào 5 locale (kể cả `error.conflict|invalidState` đã có từ 018-05: kiểm tra, không lặp).
2. Mở rộng `request-locale-coverage.test.ts` với toàn bộ khoá `solution.*`.
3. E2E: (a) Request `awaiting_analysis_approval` với Solution `change_request` hai phương án: thấy hai thẻ và bảng so sánh; (b) chọn một phương án, bấm "Duyệt phương án này": kênh `solution.choose` rồi `approval.approve` được gọi theo thứ tự (mock) và Request chuyển `planning`; (c) từ chối với lý do ngắn: nút khoá; với lý do hợp lệ: gửi `approval.reject` có `comment`; (d) `Mod+Enter` trong hộp từ chối gửi (Mac dùng `Meta`, Linux/Windows dùng `Control`); (e) `hotfix` không có thanh quyết định.
4. Cập nhật `docs/ui/pages/requests.md` mục Phân tích.

## Kiểm thử

- Vitest: `pnpm --filter orca-frontend test -- src/renderer/src/i18n/request-locale-coverage`.
- E2E theo lệnh ở Context; ghi "chưa chạy" nếu thiếu Electron/backend.
- Quét hex/emoji cho `components/request/solution/**` (test chuỗi nguồn).

## Tiêu chí hoàn thành

- [ ] Mọi khoá của CR-020 có chuỗi ở 5 locale; test phủ khoá xanh.
- [ ] E2E (a)-(e) hoặc `test.skip` có lý do.
- [ ] Tài liệu trang cập nhật.

## Rủi ro và lưu ý

- Thứ tự `choose` → `approve` có thể đổi khi CONTRACT ra (xem 020-04): test e2e sửa theo.
- Bản dịch cần người bản ngữ review.
