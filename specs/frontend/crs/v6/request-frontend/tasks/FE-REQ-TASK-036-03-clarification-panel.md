# FE-REQ-TASK-036-03: `ClarificationPanel`, danh sách câu hỏi và kiểm hợp lệ trả lời

**From Solution:** [FE-REQ-SOL-036](../solutions/FE-REQ-SOL-036-clarification-decision-readiness-impact-ui.md) mục 2.4
**Priority:** P0
**Area:** frontend / request / clarification
**File:** `frontend/src/renderer/src/components/request/clarification/{ClarificationPanel,ClarificationQuestionList,ClarificationQuestionField,ClarificationDeadlineNote}.tsx`, `clarification-answer-validation.ts` (mới); `components/request/RequestDetailPane.tsx`, `RequestStageTimeline.tsx`, `RequestDetailHeader.tsx` (sửa, FE-REQ-TASK-019-03); test cùng tên
**Depends on:** FE-REQ-TASK-036-01, 036-02; FE-REQ-TASK-019-01 (`RequestStageTimeline`), 019-03 (`RequestDetailPane`)
**Status:** [~] PARTIAL — vitest clarification/ClarificationPanel (9), clarification-answer-validation (8) pass — thiếu: nút "Trả lời" ở header, bộ lọc "Chờ bổ sung", định vị bước theo `resumeStatus`

## Context

- Hiện khi `request.status === 'awaiting_information'` (12 trạng thái, 036-01). Mỗi Request tối đa một Clarification `open` (CR-028 quyết định 3). Không mở trang riêng (CR-036 2.2).
- Câu hỏi: `kind` ∈ `text|single_choice|multi_choice|file|boolean`; `prompt`, `reason` (lý do hỏi, dòng phụ), `suggestedDefault` (điền sẵn, nhãn "Đề xuất", **không tự gửi**), `required`, `source` (chip: `readiness`, `solution_open_question`, `plan_assumption`, `task_blocked`). Backend: `text` tối đa 4000 ký tự; `file` v1 chỉ `{filename, mime, size, text}` với `text` tối đa 64 KB (CR-028 2.4 bước 2); `required` cần giá trị hoặc `acceptDefault` khi có `suggestedDefault`.
- Quyền: người không thuộc `assigneeIds` và không admin xem chỉ đọc; `REQUEST_CLARIFICATION_NOT_ASSIGNEE` là nguồn chân lý. Hạn: `dueAt` hiển thị "Còn N ngày" (cùng cách hiển thị hạn ở SOL-022; tái dùng `request-relative-time.ts` của FE-REQ-TASK-022-03 nếu đã có, ngược lại hàm cục bộ).
- Gửi một lần cho cả danh sách (`clarification.answer`, `complete: true`); phản hồi `stillMissing` thì Clarification vòng kế (`round + 1`); vượt `REQUEST_CLARIFICATION_MAX_ROUNDS` Request về backlog `missing_info`.
- Phím tắt: `isScreenSubmitShortcut(event)` và `getScreenSubmitModifierLabel()` từ `lib/screen-submit-shortcut.ts` (`metaKey` Mac, `ctrlKey` nơi khác); chip `ShortcutKeyCombo keys={[getScreenSubmitModifierLabel(), 'Enter']}`.
- Primitive: `ui/textarea.tsx`, `ui/input.tsx`, `ui/checkbox.tsx`, `ui/toggle-group.tsx`, `ui/label.tsx`, `ui/button.tsx`, `ui/badge.tsx`, `ui/skeleton.tsx`; **không có** `radio-group`, `alert`. Lỗi trường qua `aria-invalid` (STYLEGUIDE dòng 219: primitive tự vẽ vòng lỗi; không tự tô).

## Việc cần làm

1. `clarification-answer-validation.ts` (thuần): `validateAnswer(question, value): { ok: true } | { ok: false; reasonKey: string }` theo `kind` (`text` ≤ 4000 và không rỗng sau trim khi `required`; `single_choice` thuộc `options`; `multi_choice` tập con, rỗng chỉ khi không `required`; `boolean` là true/false; `file` `{filename, mime, size, text}` với `text.length ≤ 64 * 1024` ký tự (đếm theo byte UTF-8 bằng `TextEncoder`) và nhị phân bị chặn bằng kiểm `mime` chữ hoặc chứa ký tự NUL), `canSubmit(clarification, draft): boolean`, `toAnswerPayload(clarification, draft, acceptedDefaults): AnswerPayload` (giá trị thành `valueJson = JSON.stringify(value)`; câu `acceptDefault` gửi `acceptDefault: true` và `valueJson: ''`). Hằng `MAX_TEXT_LENGTH = 4000`, `MAX_FILE_TEXT_BYTES = 64 * 1024`.
2. `ClarificationQuestionField.tsx`: render theo `kind`: `text` `Textarea` (cao tự giãn, `maxLength`); `single_choice` `ToggleGroup type="single"` (hoặc `<input type="radio">` có `Label` nếu số lựa chọn trên 6); `multi_choice` danh sách `Checkbox`; `boolean` hai nút "Có"/"Không" (`ToggleGroup`); `file` `<input type="file">` đọc `File.text()` (từ chối nhị phân và quá 64 KB bằng lỗi cạnh trường, không tải lên đâu cả). Dòng phụ `reason`; chip `source`; nếu có `suggestedDefault` và giá trị chưa đụng: nhãn "Đề xuất" và nút "Dùng đề xuất" (đặt `acceptDefault`); `aria-invalid` và dòng lỗi `id` gắn bằng `aria-describedby`; dấu bắt buộc có văn bản "Bắt buộc" ẩn cho trình đọc màn hình qua `sr-only`.
3. `ClarificationQuestionList.tsx`: danh sách theo `seq`, chỉ đọc khi `readOnly`; tiêu điểm vào câu bắt buộc đầu tiên còn thiếu khi bấm gửi mà chưa đủ.
4. `ClarificationDeadlineNote.tsx`: "Còn 2 ngày" / "Quá hạn"; quá hạn thì nút gửi vô hiệu và dòng "Request sẽ về backlog (thiếu thông tin)" (khớp backlog `missing_info`, SOL-023).
5. `ClarificationPanel.tsx`: props `{ request: RequestView; currentUserId: string; isAdmin: boolean }`; dùng `useClarifications(request.id)`. Hiện nếu `request.status==='awaiting_information'` và `open!=null`; chưa có Clarification mở thì không render (nhưng nếu `status==='awaiting_information'` mà `open==null` hiện skeleton rồi dòng "Đang chờ thông tin cần bổ sung" sau 3 giây, vì Clarification có thể chưa kịp tải). Tiêu đề "Cần bổ sung thông tin" kèm `displayId`, `round`. Nút chính "Gửi câu trả lời" (`Button` mặc định) khoá tới khi `canSubmit`; sau khi gửi hiện "Đã nhận, AI đang chạy lại bước <tên bước>" (tên bước theo `resumeStatus`) và chờ sự kiện `request.status_changed`, **không** tự đổi trạng thái ở client. `stillMissing=true` thì hiện "Còn thiếu: …" và vòng mới tự nạp. `Mod+Enter` trong ô văn bản gọi gửi khi hợp lệ. Cảnh báo khi rời trang có nháp: dùng `beforeunload` trong `useEffect` chỉ khi `hasDraft`, dọn khi unmount; điều hướng nội bộ giữ nguyên (không chặn router, app không có router).
6. Người xem không phải đối tượng: `readOnly`, dòng "Đang chờ <tên> trả lời" (nếu có tên từ `assigneeIds`; chưa có tên thì "Đang chờ người được chỉ định trả lời"), không nút gửi.
7. `RequestStageTimeline` (sửa): khi `awaiting_information` hiện bước đang chờ (`resumeStatus`) với nhãn "Chờ bổ sung thông tin" thay vì nút chính của bước; `RequestDetailHeader` thêm nút "Trả lời" (cuộn tới `ClarificationPanel`) khi đúng đối tượng; `RequestListToolbar`: bộ lọc nhanh "Chờ bổ sung" (đặt cho 019-02, ghi chú trong PR nếu chưa merge).
8. Lỗi theo `RequestRpcError`: `conflict` → `refetch` giữ nháp; `expired` hoặc `invalid_state` → `refetch` chỉ đọc; `validation` (`INVALID_ANSWER`) → lỗi trường (cần `questionId` trong thông điệp; nếu không có thì toast chung); `forbidden` → chỉ đọc; `network` → banner "Thử lại" giữ nháp.

## Bảng tham chiếu nhanh

| Tình huống | UI |
|---|---|
| Đang tải Clarification | skeleton; sau 3 giây nếu vẫn chưa có dòng "Đang chờ thông tin cần bổ sung" |
| Không có Clarification mở | không render panel |
| Người xem không phải đối tượng | chỉ đọc + "Đang chờ người được chỉ định trả lời" |
| Quá hạn | nút gửi vô hiệu + "Request sẽ về backlog (thiếu thông tin)" |
| `still_missing` sau khi gửi | danh sách "Còn thiếu" và vòng kế |
| Mất mạng | banner "Thử lại", giữ nháp |

- Kênh WS và payload: `clarification.list {requestId, status:'open'}`, `clarification.answer {clarificationId, answers:[{questionId, valueJson, acceptDefault}], complete:true, expectedVersion}`.
- Phím tắt: `Mod+Enter` gửi (`metaKey` Mac, `ctrlKey` Linux/Windows; chip `ShortcutKeyCombo`); `Esc` không có hành vi riêng.
- Khoá i18n: `auto.components.request.clarification.{title,submit,submitted,answeredResume,stillMissing,required,suggested,useSuggested,readOnlyWaiting,dueIn,expired,leaveWarning,fileTooLarge,fileBinary,source.readiness,source.solution_open_question,source.plan_assumption,source.task_blocked,yes,no}`.

## Trình tự làm gợi ý

1. Viết `clarification-answer-validation.test.ts` trước, rồi hàm thuần.
2. Viết `ClarificationQuestionField` và test từng kiểu.
3. Viết `ClarificationQuestionList`, `ClarificationDeadlineNote`, `ClarificationPanel` và test (kể cả `Mod+Enter` hai nền tảng).
4. Sửa `RequestStageTimeline`, `RequestDetailHeader` đúng dòng cần thiết.
5. Thêm khoá i18n vào danh sách của 036-08 và chạy test.

## Kiểm thử

- `clarification-answer-validation.test.ts`: 5 kiểu × (hợp lệ, thiếu bắt buộc, quá dài); `file` đúng 64 KB và 64 KB + 1 byte (UTF-8 nhiều byte); nhị phân; `acceptDefault` thay giá trị; `toAnswerPayload` đúng `valueJson`.
- `ClarificationQuestionField.test.tsx`: mỗi kiểu render đúng control; nhãn "Đề xuất" khi có mặc định; `aria-invalid` khi lỗi; `file` quá lớn lỗi cạnh trường.
- `ClarificationPanel.test.tsx` (mock `useClarifications`): đủ 5 kiểu; nút gửi khoá tới khi hợp lệ; gửi gọi `answer` một lần với cả danh sách; `Mod+Enter` (`metaKey` giả lập Mac, `ctrlKey` giả lập Linux/Windows; `ctrlKey` trên Mac không gửi; mock `navigator.userAgent`); người không phải đối tượng chỉ đọc; quá hạn khoá gửi; `status!=='awaiting_information'` không render; cảnh báo `beforeunload` chỉ khi có nháp.
- `RequestStageTimeline.test.tsx` (mở rộng): nhãn "Chờ bổ sung thông tin" ở đúng bước.
- Chạy: `pnpm --filter orca-frontend test src/renderer/src/components/request/clarification src/renderer/src/components/request/RequestStageTimeline`.
- i18n: khoá `auto.components.request.clarification.*` (task 036-08 gom; thêm khoá tới đó và cập nhật test phủ khoá).

## Tiêu chí hoàn thành

- [ ] `awaiting_information` hiện `ClarificationPanel` đủ 5 kiểu; thiếu câu bắt buộc thì khoá gửi.
- [ ] Gửi một lần với cả danh sách (`complete: true`); không tự gửi giá trị mặc định.
- [ ] Người không phải đối tượng chỉ đọc; quá hạn khoá gửi.
- [ ] Nháp chỉ ở bộ nhớ hook; cảnh báo khi rời nếu có nháp.
- [ ] `Mod+Enter` đúng `metaKey` Mac, `ctrlKey` nơi khác; chip phím đúng nền tảng.
- [ ] `file` chỉ văn bản tối đa 64 KB, lỗi rõ khi vượt.

## Rủi ro và lưu ý

- Tải tệp: không có thành phần dùng lại; bản v1 là văn bản (backend). Người dùng muốn tệp nhị phân sẽ thất vọng: nêu rõ ở thông báo lỗi.
- `ToggleGroup` cho lựa chọn đơn có thể chưa quen thuộc bằng radio; nếu a11y kém, chuyển `<input type="radio">` bọc `Label` (không thêm primitive mới).
- `RequestStageTimeline` và `RequestListToolbar` do 019 sở hữu: chỉ thêm đúng dòng cần thiết, tránh xung đột.
- Tên người trả lời có thể không có (chỉ id); không tự tra cứu thêm để khỏi thêm N+1.
- Mọi chuỗi UI qua i18n; không emoji.

## Ghi chú triển khai (2026-10-07)

`ClarificationPanel` gắn vào `RequestDetailPane`; `RequestStageTimeline` chỉ thêm dòng "Đang chờ bổ sung thông tin" (bước hiện tại vẫn là `analysis` theo `STATUS_TO_CURRENT_STEP`, chưa dùng `resumeStatus`). Chưa thêm nút "Trả lời" vào `RequestDetailHeader` và bộ lọc nhanh vào `RequestListToolbar`. Tên người được chỉ định chưa có (chỉ id) nên dùng dòng chung.
