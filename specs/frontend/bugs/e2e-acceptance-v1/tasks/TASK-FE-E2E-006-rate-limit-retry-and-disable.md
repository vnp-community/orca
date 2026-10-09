# TASK-FE-E2E-006: Xử lý `rate_limited`: khoá nút trong khoảng chờ và thử lại thao tác idempotent

**From Solution:** SOL-FE-E2E-003 · **Resolves:** FE-E2E-B
**Priority:** P1 · **Area:** frontend
**Files:**
- frontend/src/renderer/src/runtime/request-rpc-client.ts
- các hook ghi: useApprovals, useSolutions, request-plan-generation
**Depends on:** TASK-FE-E2E-004, TASK-E2E-031
**Status:** `[ ]` TODO

---

## Context

Bấm lại nhiều lần khi bị giới hạn làm tăng tải.

## Việc cần làm

- [ ] `callRequestRpc`: khi `kind === rate_limited` và thao tác được đánh dấu idempotent, thử lại một lần sau `retryAfterMs` (mặc định 1s).
- [ ] Thao tác không idempotent: không thử lại; hiển thị thông báo.
- [ ] Hook trả trạng thái `coolingDown` để khoá nút.

## Tiêu chí hoàn thành

- [ ] Test client với fake thời gian; test hook.

## Kiểm thử

Vitest.
