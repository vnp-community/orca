# Tasks frontend: bug nghiệm thu e2e (v1)

Ký hiệu: `[x]` xong, `[~]` một phần, `[ ]` chưa làm, `[-]` không làm.

## Trạng thái thực thi (2026-10-10)

**8/8 task đã làm, 1 task có điều kiện còn `[~]` (FE-007).** Thêm TASK-FE-E2E-009 cho các lỗi tìm thấy khi chạy UI thật. Bằng chứng chính: `run_ui_flow.py --ai-stub` 3/3 xanh (change_request, ask-back, question), `ask-back` 6/6 liên tiếp, và `change_request` với Claude thật xanh (có một vòng "Hỏi lại" thật); gateway không restart. Suite Playwright mock `request-web` 35 xanh, 1 skipped có sẵn. Vitest: tất cả test file thay đổi xanh; còn các file đỏ có sẵn trên HEAD (thiếu `src/main`, `src/relay`, `window.api`, `jsdom`...). Chi tiết số liệu ở từng file task.

Lỗi sản phẩm tìm thấy nhờ chạy qua UI thật (đều đã sửa): gateway panic `send on closed channel`; `task.list` không phục vụ tab Plan; Solution không tự sinh sau Clarification (consumer thiếu danh tính); phân tích chỉ đọc thiếu `repo_path`; câu hỏi chặn của Solution không có đường trả lời (kênh `clarification.request`); `RequestClarification` bị đánh dấu nội bộ; UI giữ Solution cũ sau khi bị thay.

Còn mở: `ApprovalGates.CheckPlan` (chặn duyệt Plan khi còn giả định chưa xác nhận) chưa được handler duyệt Plan nào gọi, nên UI đã sẵn sàng hiển thị `REQUEST_PLAN_UNCONFIRMED_ASSUMPTIONS` nhưng không có luồng thật phát mã này (cần quyết định nghiệp vụ theo CR-REQ-028); duyệt Plan để thực thi agent AI chưa được e2e.

## Thứ tự

```
TASK-E2E-003 ──► FE-001
TASK-E2E-001 ──► FE-002 ──► FE-008
TASK-E2E-032 ──► FE-003
TASK-E2E-015/024/031 ──► FE-004 ──► FE-005, FE-006
TASK-E2E-011/012 ──► FE-007
```

FE-001 và FE-002 làm trước (P0): chúng làm cho lỗi hợp đồng kế tiếp **hiện ra** thay vì im lặng.

## Checklist

### SOL-FE-E2E-001

- [x] [TASK-FE-E2E-001](./TASK-FE-E2E-001-rpc-method-contract-test.md) P0 · Test hợp đồng: mọi kênh frontend gọi đều có ở gateway · phụ thuộc: TASK-E2E-003
- [x] [TASK-FE-E2E-002](./TASK-FE-E2E-002-clarification-dead-end-state.md) P0 · Trạng thái chặn có tên khi không tải được câu hỏi làm rõ · phụ thuộc: TASK-E2E-001

### SOL-FE-E2E-002

- [x] [TASK-FE-E2E-003](./TASK-FE-E2E-003-typed-clarification-input.md) P2 · Ô nhập theo `valueType` · phụ thuộc: TASK-E2E-032

### SOL-FE-E2E-003

- [x] [TASK-FE-E2E-004](./TASK-FE-E2E-004-request-error-code-map.md) P1 · Bổ sung bảng ánh xạ mã lỗi Request · phụ thuộc: TASK-E2E-015, 024, 031 (mã lỗi)
- [x] [TASK-FE-E2E-005](./TASK-FE-E2E-005-ai-not-configured-banner.md) P1 · Thông báo cấu hình AI có hướng khắc phục · phụ thuộc: TASK-FE-E2E-004, TASK-E2E-024
- [x] [TASK-FE-E2E-006](./TASK-FE-E2E-006-rate-limit-retry-and-disable.md) P1 · Xử lý `rate_limited`: khoá nút trong khoảng chờ và thử lại thao tác idempotent · phụ thuộc: TASK-FE-E2E-004, TASK-E2E-031
- [~] [TASK-FE-E2E-007](./TASK-FE-E2E-007-task-tree-empty-vs-error.md) P2 · Cây task: phân biệt rỗng, lỗi và không có quyền · phụ thuộc: TASK-E2E-011, 012
- [x] [TASK-FE-E2E-008](./TASK-FE-E2E-008-live-stack-playwright-smoke.md) P2 · Playwright smoke chạy trên stack thật cho luồng làm rõ (có điều kiện) · phụ thuộc: TASK-E2E-001, 026

### Phát sinh khi chạy UI thật

- [x] [TASK-FE-E2E-009](./TASK-FE-E2E-009-ui-live-findings-frontend.md) P1 · Tab Plan, nút Generate solution, thanh duyệt, câu hỏi chặn + "Hỏi lại", lỗi `REQUEST_PLAN_UNCONFIRMED_ASSUMPTIONS`, Solution cũ sau supersede · phụ thuộc: TASK-FE-E2E-004, 005, 006 và các sửa backend TASK-V2-022/023

## Định nghĩa "xong"

- [~] Vitest của các file đã đổi xanh; `tsc` và `oxlint` sạch trên file đã đổi. Cả bộ `pnpm lint`/`typecheck`/vitest toàn frontend **không xanh** do lỗi có sẵn trên HEAD (thiếu `src/main`, `src/relay`, `window.api`, `jsdom`; `FleetServerStatusBadge`; `RequestBacklogRow`...), đã đối chiếu bằng bản sao HEAD sạch và không có test nào xanh ở HEAD mà đỏ bây giờ.
- [x] Mỗi chuỗi mới có i18n đủ 5 ngôn ngữ (en, es, ja, ko, zh); không màu/cỡ chữ mới.
- [x] Suite Playwright `request-web` xanh (35 xanh, 1 skipped có sẵn) sau khi sửa hạ tầng mock (project `request-web`, import, `webServer.cwd`, phím Mod+Enter theo userAgent); spec live xanh trên stack local.
- [~] Kiểm tra trên cả Electron và web (G2): mã dùng chung `callRequestRpc`/`callRuntimeRpc`; chỉ kiểm bằng trình duyệt web, **chưa kiểm Electron**.
