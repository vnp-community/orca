# FE-CV-TASK-095-01: Schema tám sự kiện telemetry

**From Solution:** [FE-CV-SOL-095-review-telemetry](../solutions/FE-CV-SOL-095-review-telemetry.md) mục 2.2
**Priority:** P0
**Area:** frontend
**File:** `frontend/src/shared/review-telemetry-events.ts` (mới) + `.test.ts`; `frontend/src/shared/telemetry-events.ts` (sửa +2 dòng: import và `...reviewEventSchemas`)
**Depends on:** —
**Status:** [ ] TODO

## Context

- Mẫu `mcp-telemetry-events.ts`; học thuyết phiên bản (telemetry-events.ts ~1399).
- `telemetry-events.ts` có disable max-lines sẵn; không mở rộng.

## Việc cần làm

1. Viết schema đúng mục 2.2, `.strict()`; `toolEnum` đóng.
2. Spread vào `eventSchemas`; `EventMap` tự suy ra.
3. Test theo `mcp-telemetry-events.test.ts` + khoá cấm (`repo_id`, `worktree_id`, `commit`, `path`, `rule_id`, `message`, `task_id`, `tenant_id`, `user`, `email`, `count`).

## Kiểm thử

- Hợp lệ qua; khoá lạ/enum lạ bị từ chối; đã đăng ký trong `eventSchemas`.
- Chạy: `pnpm --filter orca-frontend test -- <đường dẫn>` (chưa chạy).

## Tiêu chí hoàn thành

- [ ] Test xanh.
- [ ] Không `max-lines` disable mới.

## Rủi ro

- Enum lens/reason phụ thuộc CR khác.
