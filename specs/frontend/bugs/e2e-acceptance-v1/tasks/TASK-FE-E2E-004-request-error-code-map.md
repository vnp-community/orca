# TASK-FE-E2E-004: Bổ sung bảng ánh xạ mã lỗi Request

**From Solution:** SOL-FE-E2E-003 · **Resolves:** FE-E2E-B
**Priority:** P1 · **Area:** frontend
**Files:**
- frontend/src/shared/request-errors.ts
- frontend/src/shared/request-errors.test.ts
**Depends on:** TASK-E2E-015, 024, 031, 033 (mã lỗi)
**Status:** `[ ]` TODO

---

## Context

`REQUEST_RATE_LIMITED` và các mã mới rơi vào `unknown`.

## Việc cần làm

- [ ] Thêm vào `CODE_TO_KIND`: `REQUEST_RATE_LIMITED`, `REQUEST_FLOW_DISABLED`, `REQUEST_PROJECT_NOT_FOUND`, `REQUEST_AI_NOT_CONFIGURED`, `REQUEST_AI_CREDENTIAL_UNAVAILABLE`, `REQUEST_AI_AGENT_UNAVAILABLE`, `REQUEST_AI_SESSION_EXPIRED`, `REQUEST_AI_QUOTA_EXHAUSTED`, `PROJECT_DEV_SERVER_*`.
- [ ] Thêm kiểu `no_ai` (và `feature_off` nếu cần) vào union; sửa mọi `switch` để TypeScript không báo thiếu nhánh.
- [ ] Đọc `retryAfterMs` từ thông điệp/chi tiết khi có.

## Tiêu chí hoàn thành

- [ ] Test bảng cho từng mã.
- [ ] Không còn mã nào trong bảng ở bảng SOL-FE-E2E-003 bị phân loại `unknown`.

## Kiểm thử

Vitest.
