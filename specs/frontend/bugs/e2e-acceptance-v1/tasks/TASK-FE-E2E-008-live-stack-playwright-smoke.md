# TASK-FE-E2E-008: Playwright smoke chạy trên stack thật cho luồng làm rõ (có điều kiện)

**From Solution:** SOL-FE-E2E-003 · **Resolves:** BUG-E2E-001
**Priority:** P2 · **Area:** frontend
**Files:**
- tests/stages/e2e/legacy/request-web/ (spec mới dùng stack thật)
- tests/configs/playwright.web.config.ts
**Depends on:** TASK-E2E-001, 026
**Status:** `[ ]` TODO

---

## Context

Các spec `request-web` dùng `mock-request-ws.ts` nên không bắt được lỗi hợp đồng thật.

## Việc cần làm

- [ ] Spec mới đăng nhập vào stack thật, tạo Request, xác nhận loại, trả lời Clarification trên UI, kiểm Request sang `planning`.
- [ ] Chỉ chạy khi có `ORCA_E2E_LIVE=1` và stack sẵn sàng; dùng AI stub (TASK-E2E-026).
- [ ] Dùng dữ liệu có tiền tố riêng và tự dọn.

## Tiêu chí hoàn thành

- [ ] Spec xanh trên stack local; bị bỏ qua khi thiếu biến.

## Kiểm thử

Playwright.
