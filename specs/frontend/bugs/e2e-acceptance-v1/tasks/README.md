# Tasks frontend: bug nghiệm thu e2e (v1)

Ký hiệu: `[ ]` chưa làm. Không có thay đổi frontend nào đã được thực hiện trong đợt này.

## Trạng thái thực thi (2026-10-09)

**Chưa bắt đầu: 0/8 task frontend.** Phần backend mà các task này phụ thuộc đã sẵn sàng và đã kiểm trên stack thật:

- `testdata/channel_inventory.json` của gateway (TASK-FE-E2E-001) và test parity RPC↔kênh;
- `valueType` (`string|number|boolean|list`) trên câu hỏi Clarification (TASK-FE-E2E-003);
- thông báo `REQUEST_RATE_LIMITED ... {"retryAfterMs":N}` (TASK-FE-E2E-006) và các mã `REQUEST_AI_*`, `REQUEST_PLAN_*`, `REQUEST_CLARIFICATION_*` (TASK-FE-E2E-004, 005);
- bộ lọc `request.list` nhận `awaiting_information`; `request.generatePlan` (propose chạy nền, đọc bằng `request.planProposal`, commit) đã chạy thật.


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

- [ ] [TASK-FE-E2E-001](./TASK-FE-E2E-001-rpc-method-contract-test.md) P0 · Test hợp đồng: mọi kênh frontend gọi đều có ở gateway · phụ thuộc: TASK-E2E-003
- [ ] [TASK-FE-E2E-002](./TASK-FE-E2E-002-clarification-dead-end-state.md) P0 · Trạng thái chặn có tên khi không tải được câu hỏi làm rõ · phụ thuộc: TASK-E2E-001

### SOL-FE-E2E-002

- [ ] [TASK-FE-E2E-003](./TASK-FE-E2E-003-typed-clarification-input.md) P2 · Ô nhập theo `valueType` · phụ thuộc: TASK-E2E-032

### SOL-FE-E2E-003

- [ ] [TASK-FE-E2E-004](./TASK-FE-E2E-004-request-error-code-map.md) P1 · Bổ sung bảng ánh xạ mã lỗi Request · phụ thuộc: TASK-E2E-015, 024, 031 (mã lỗi)
- [ ] [TASK-FE-E2E-005](./TASK-FE-E2E-005-ai-not-configured-banner.md) P1 · Thông báo cấu hình AI có hướng khắc phục · phụ thuộc: TASK-FE-E2E-004, TASK-E2E-024
- [ ] [TASK-FE-E2E-006](./TASK-FE-E2E-006-rate-limit-retry-and-disable.md) P1 · Xử lý `rate_limited`: khoá nút trong khoảng chờ và thử lại thao tác idempotent · phụ thuộc: TASK-FE-E2E-004, TASK-E2E-031
- [ ] [TASK-FE-E2E-007](./TASK-FE-E2E-007-task-tree-empty-vs-error.md) P2 · Cây task: phân biệt rỗng, lỗi và không có quyền · phụ thuộc: TASK-E2E-011, 012
- [ ] [TASK-FE-E2E-008](./TASK-FE-E2E-008-live-stack-playwright-smoke.md) P2 · Playwright smoke chạy trên stack thật cho luồng làm rõ (có điều kiện) · phụ thuộc: TASK-E2E-001, 026

## Định nghĩa "xong"

- [ ] `pnpm` chạy được lint, typecheck và vitest của frontend, tất cả xanh.
- [ ] Mỗi chuỗi mới có i18n; không màu/cỡ chữ mới (STYLEGUIDE).
- [ ] Suite Playwright `request-web` xanh; spec live (FE-008) xanh khi có stack.
- [ ] Kiểm tra trên cả Electron và web (G2).
