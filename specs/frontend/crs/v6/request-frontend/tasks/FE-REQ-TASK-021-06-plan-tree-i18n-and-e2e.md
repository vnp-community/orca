# FE-REQ-TASK-021-06: i18n, test phủ khoá và e2e cây Plan

**From Solution:** [FE-REQ-SOL-021](../solutions/FE-REQ-SOL-021-plan-phase-tree-and-approval-ui.md) mục 2.7, 5
**Priority:** P1
**Area:** frontend / i18n + e2e
**File:** `frontend/src/renderer/src/i18n/locales/{en,es,ja,ko,zh}.json` (sửa), `frontend/src/renderer/src/i18n/request-locale-coverage.test.ts` (sửa), `tests/e2e/request-plan-tree.spec.ts` (mới), `docs/ui/pages/requests.md` và `docs/ui/pages/tasks.md` (sửa: công tắc Plan/Phase)
**Depends on:** FE-REQ-TASK-021-01 đến 021-05
**Status:** [ ] TODO

## Context

- Tiền tố `auto.components.request.plan.`, riêng `TaskGraph.showPlanning` và `TaskDetail.phaseNotApproved` nằm dưới `auto.components.task.`.
- E2E: mẫu `tests/e2e/tasks-page.spec.ts`; cần backend CR-REQ-011/012/013 hoặc mock WS. Lệnh gốc repo (chưa kiểm chứng): `npx playwright test tests/e2e/request-plan-tree.spec.ts --config tests/playwright.config.ts --project electron-headless`.

## Việc cần làm

1. Thêm toàn bộ khoá của CR-021 2.7 và SOL-021 2.7 vào 5 locale; mở rộng `request-locale-coverage.test.ts`.
2. E2E: (a) Request `change_request` `planning` với Plan 2 Phase: cây đúng thứ tự, tiến độ; (b) Board mặc định không có Plan/Phase, bật công tắc thì có, Task con vẫn thấy ở gốc; (c) duyệt Plan gọi `approval.approve` (mock), từ chối với lý do ngắn bị khoá; (d) Phase chưa duyệt: Task không chạy được (nút khoá); duyệt Phase rồi "Bắt đầu Phase" gọi `request.startPhase {id, phaseTaskId}`; (e) runtime thiếu kênh: tab báo không hỗ trợ, không lỗi đỏ.
3. Cập nhật tài liệu trang Tasks (công tắc) và Requests (tab Plan).

## Kiểm thử

- Vitest: `pnpm --filter orca-frontend test -- src/renderer/src/i18n/request-locale-coverage`.
- E2E theo lệnh ở Context; nếu thiếu backend/Electron ghi "chưa chạy" và `test.skip` có lý do.
- Quét hex/emoji cho `components/request/plan/**`.

## Tiêu chí hoàn thành

- [ ] 5 locale đủ khoá của CR-021.
- [ ] E2E (a)-(e) hoặc `test.skip` có lý do.
- [ ] Tài liệu cập nhật.

## Rủi ro và lưu ý

- E2E phụ thuộc cách backend dựng Plan (CR-REQ-012): dữ liệu mock phải khớp `Task.type` `plan|phase` và Approval `subjectId`.
- Bản dịch cần người bản ngữ review.
