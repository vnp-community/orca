# TASK-FE-E2E-002: Trạng thái chặn có tên khi không tải được câu hỏi làm rõ

**From Solution:** SOL-FE-E2E-001 · **Resolves:** FE-E2E-A
**Priority:** P0 · **Area:** frontend
**Files:**
- frontend/src/renderer/src/hooks/useClarifications.ts
- frontend/src/renderer/src/components/request/clarification/ClarificationPanel.tsx
- frontend/src/renderer/src/components/request/RequestDetailPane.tsx
- i18n: auto.components.request.clarification.*
**Depends on:** TASK-E2E-001
**Status:** `[ ]` TODO

---

## Context

Hook đặt `error=null` khi `unsupported`. Panel không có gì để hiển thị khi Request ở `awaiting_information`.

## Việc cần làm

- [ ] Hook trả thêm `blocked: {{kind, code}} | null` khi Request cần Clarification (`awaiting_information`) mà liệt kê thất bại hoặc rỗng.
- [ ] Panel hiển thị trạng thái chặn: tiêu đề, nguyên nhân theo `kind`, nút "Thử lại", mã lỗi dùng token/primitive hiện có (G1).
- [ ] Giữ nguyên hành vi im lặng ở các trạng thái không chờ người.
- [ ] Chuỗi i18n mới (G3).

## Tiêu chí hoàn thành

- [ ] Vitest: `awaiting_information` + `unsupported` → `blocked`; `completed` + `unsupported` → không.
- [ ] Playwright `request-web`: kịch bản kênh thiếu.

## Kiểm thử

Vitest + Playwright (mock).
