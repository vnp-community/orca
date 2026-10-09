# TASK-FE-E2E-003: Ô nhập theo `valueType`

**From Solution:** SOL-FE-E2E-002 · **Resolves:** FE-E2E-D
**Priority:** P2 · **Area:** frontend
**Files:**
- frontend/src/shared/request-artifact-types.ts
- frontend/src/shared/request-artifact-parsers.ts
- frontend/src/renderer/src/components/request/clarification/ClarificationQuestionField.tsx
- frontend/src/renderer/src/components/request/clarification/clarification-answer-validation.ts (+ test)
**Depends on:** TASK-E2E-032
**Status:** `[ ]` TODO

---

## Context

`time_box_hours` là `kind: text` nhưng cần chuỗi số.

## Việc cần làm

- [ ] Thêm `valueType?` vào kiểu và `parseQuestion` (thiếu thì `string`).
- [ ] Nhánh `text`: `valueType === number` → ô số `inputmode="decimal"`, kiểm tra sớm, gửi chuỗi số.
- [ ] Lỗi `REQUEST_CLARIFICATION_INVALID_ANSWER` hiển thị cạnh câu hỏi.
- [ ] Tương thích backend cũ (không có trường).

## Tiêu chí hoàn thành

- [ ] Test validation và render mới; `MTX-13` backend xanh.

## Kiểm thử

Vitest.
