# TASK-FE-E2E-003: Ô nhập theo `valueType`

**From Solution:** SOL-FE-E2E-002 · **Resolves:** FE-E2E-D
**Priority:** P2 · **Area:** frontend
**Files:**
- frontend/src/shared/request-artifact-types.ts
- frontend/src/shared/request-artifact-parsers.ts
- frontend/src/renderer/src/components/request/clarification/ClarificationQuestionField.tsx
- frontend/src/renderer/src/components/request/clarification/clarification-answer-validation.ts (+ test)
**Depends on:** TASK-E2E-032
**Status:** `[x]` DONE (2026-10-10); `MTX-13` của backend chưa chạy từ phía tôi (bị cấm chạy `run_acceptance.py`)

---

## Context

`time_box_hours` là `kind: text` nhưng cần chuỗi số.

## Việc cần làm

- [x] Thêm `valueType?` vào kiểu và `parseQuestion` (thiếu hoặc giá trị lạ thì để trống = coi là `string`). Chấp nhận `valueType` và `value_type`.
- [x] Nhánh `text`: `valueType === 'number'` → ô một dòng `inputmode="decimal"`, kiểm tra sớm (báo ngay khi đã gõ, vì nút gửi đang bị khoá), gửi chuỗi số đã cắt khoảng trắng dưới dạng JSON string (đúng thứ `ValidateAnswer` của kind `text` đòi).
- [x] Lỗi `REQUEST_CLARIFICATION_INVALID_ANSWER` hiển thị cạnh câu hỏi: trước đây panel chỉ nhận `question_id=<id>` nên thông điệp thật của máy chủ (`question "<questionKey>": <lý do>`) rơi xuống banner chung. Nay khớp theo `questionKey` và chỉ hiện phần lý do.
- [x] Tương thích backend cũ (không có trường): hành vi cũ nguyên vẹn (textarea).

## Tiêu chí hoàn thành

- [x] Test validation và render mới (xem dưới).
- [ ] `MTX-13` backend xanh: **chưa kiểm** từ phía frontend; điều phối viên chạy.

## Kiểm thử

Vitest.

## Kết quả thực thi

**File đổi**
- `shared/request-artifact-types.ts`: `ClarificationValueType`, `ClarificationQuestion.valueType?`.
- `shared/request-artifact-parsers.ts`: đọc `valueType`/`value_type`. **Sửa thêm một lỗi hợp đồng thật**: máy chủ gửi `options` là `[{id,label}]` (`QuestionOption` trong Go) còn parser chỉ đọc `value`, nên mọi câu `single_choice` mất hết lựa chọn; nay đọc `value` hoặc `id`.
- `clarification-answer-validation.ts`: `isNumberText`, `isNumberAnswer` (chỉ số thập phân thuần, vì máy chủ dùng `ParseFloat`: dấu `,` và dấu cách bị từ chối), nhánh `text` của `validateAnswer`, cắt khoảng trắng khi dựng payload, `questionIdFromServerMessage`, `serverReasonText`.
- `ClarificationQuestionField.tsx`: ô số dùng `Input` shadcn (type=text + `inputMode=decimal`, không dùng type=number để tránh dấu phân cách theo locale/cuộn chuột), gợi ý "Enter a number..."; `valueType=list` thêm gợi ý "One item per line" (vẫn là textarea, máy chủ tách dòng).
- `ClarificationPanel.tsx`: dùng hai hàm trên cho lỗi `validation`.
- Locale 5 ngôn ngữ: `clarification.invalidNumber|numberHint|listHint`.
- Test: `clarification-answer-validation.test.ts` (+8), `ClarificationPanel.test.tsx` (+3 render), `request-artifact-parsers.test.ts` (+1).

**Quyết định có chủ ý (G4)**: chỉ `number` được kiểm sớm. `boolean` với `kind: text` không thể làm đúng: `ValidateAnswer` đòi chuỗi JSON cho kind `text` còn trường đích `bool` đòi boolean, nên máy chủ tự mâu thuẫn; để máy chủ quyết. `list` giữ textarea. Số `0` hay âm vẫn gửi được (máy chủ coi `n<=0` là "chưa điền" ở mức ready, không phải lỗi hình thức).

**Selector Playwright**: ô số `input[data-value-type="number"]` có `inputmode="decimal"` trong `[data-question-id="<id>"]`; lỗi cạnh câu hỏi là `[data-question-id] [role="alert"]`, ô nhập có `aria-invalid="true"`.

**Lệnh**: `pnpm exec vitest run --config config/vitest.config.ts src/renderer/src/components/request/clarification src/shared/request-artifact-parsers.test.ts src/renderer/src/i18n/request` → 7 file, 85 test xanh.

**Blast radius (rà tay)**: `parseClarification`/`parseQuestion` dùng bởi `useClarifications` và `ClarificationPanel` và nơi khác dùng kiểu `ClarificationQuestion` (chỉ thêm trường tuỳ chọn). Sửa `options` chỉ làm câu hỏi có lựa chọn xuất hiện lại. Dùng chung cho Electron/web.
