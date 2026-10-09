# SOL-FE-E2E-002: Ô nhập câu hỏi làm rõ theo kiểu giá trị

**Resolves:** FE-E2E-D (OBS-B) · **Gốc:** RC-F
**Tasks:** TASK-FE-E2E-003

## Vấn đề

`ClarificationQuestionField` chọn control theo `kind` (`text`, `single_choice`, `multi_choice`, `boolean`, `file`). Câu hỏi `type_fields.time_box_hours` có `kind: text` nhưng máy chủ chỉ nhận chuỗi chứa số. Người dùng gõ câu dài bị từ chối với lỗi nói về kiểu `number` mà ô nhập không hề gợi ý.

## Thiết kế

Backend thêm `valueType` (`string|number|boolean|list`) vào câu hỏi (TASK-E2E-032, cộng thêm). Frontend:

1. Mở rộng kiểu `ClarificationQuestion` và `parseQuestion` (`shared/request-artifact-types.ts`, `request-artifact-parsers.ts`) với `valueType?`; thiếu thì coi là `string` (tương thích máy chủ cũ).
2. `ClarificationQuestionField`, nhánh `text`: khi `valueType === 'number'` dùng ô nhập số (`inputmode="decimal"`), kiểm tra sớm, gửi giá trị như máy chủ yêu cầu (chuỗi số cho `kind: text`); hiển thị gợi ý đơn vị/định dạng từ `reason`/`prompt`.
3. Kiểm tra sớm chỉ khi kiểu **chắc chắn**; mọi quyết định cuối cùng vẫn là của máy chủ (G4). Lỗi `REQUEST_CLARIFICATION_INVALID_ANSWER` hiển thị cạnh đúng câu hỏi (đã có `clarification-answer-validation.ts` làm chỗ đặt).
4. Giữ bản nháp khi người dùng đổi kiểu hoặc khi gửi lỗi (hành vi hiện có).

## Tương thích

Thiếu `valueType` (backend cũ): hành vi như hiện tại.

## Tiêu chí hoàn thành

- Với `valueType: number`, gõ chữ bị chặn/ báo ngay; gõ `8` gửi được; `MTX-13` (backend) xanh.
- Test đơn vị `clarification-answer-validation.test.ts` có trường hợp mới.
- Giao diện dùng đúng token và primitive hiện có.
