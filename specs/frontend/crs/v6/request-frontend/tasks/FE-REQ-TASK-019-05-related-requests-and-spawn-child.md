# FE-REQ-TASK-019-05: `RequestRelatedTab` và tạo Request con

**From Solution:** [FE-REQ-SOL-019](../solutions/FE-REQ-SOL-019-request-list-detail-classification-ui.md) mục 2.5
**Priority:** P1
**Area:** frontend / request
**File:** `frontend/src/renderer/src/components/request/RequestRelatedTab.tsx`, `SpawnChildRequestDialog.tsx` (mới); test cùng tên
**Depends on:** FE-REQ-TASK-019-03, 019-01 (`CHILD_REQUEST_RULES`)
**Status:** [ ] TODO

## Context

- Kênh: `request.spawnChild {id, reason, title, body, type?}`; `reason ∈ spawned_by_spike|spawned_by_question|followup_hotfix|escalation` (CR-016 2.3).
- CR-016 không có kênh liệt kê liên kết (câu hỏi mở 1 của CR-016): đọc `links` từ `request.get` nếu có, `linksSupported=false` nếu không (tạm; đã đặt ở `useRequest`, 018-03).
- Bảng loại con theo `CHILD_REQUEST_RULES` (019-01): suy luận, cần CR-REQ-006.

## Việc cần làm

1. `RequestRelatedTab`: nhóm `RequestLink` theo `reason` (cha, con, theo dõi); mỗi dòng là Request (loại, trạng thái, tiêu đề) bấm để `setRequestPageRequest(id)`; Request con tải qua `request.get` theo id (chịu `not_found`/`forbidden` bằng dòng "Không xem được").
2. Rỗng: "Không có Request liên quan"; `linksSupported=false`: "Runtime chưa hỗ trợ xem liên kết" (không lỗi đỏ).
3. `SpawnChildRequestDialog({request})`: `reason` mặc định theo `CHILD_REQUEST_RULES[request.type]`; `title` điền sẵn "<tiêu đề cha> (<reason>)", `body` điền từ kết quả phân tích nếu có (tóm tắt), `type` chọn trong `suggestedTypes` hoặc "Để AI phân loại" (`type` bỏ trống).
4. Chỉ hiện nút "Tạo Request con" ở header (019-03) khi loại cha có luật và Request có kết quả (`spike|question|hotfix` đã qua `analyzing`; `bug|task|...` leo thang bất kỳ lúc nào): điều kiện trạng thái là đề xuất, backend quyết định bằng `REQUEST_TYPE_CHANGE_USE_CHILD`/`REQUEST_TRANSITION_NOT_ALLOWED`.
5. Gửi qua `useRequestActions.spawnChild`; thành công `openRequestPage({section:'requests', requestId: child.id})` và toast kèm "Quay lại Request cha".
6. `Mod+Enter` (`isScreenSubmitShortcut`) gửi từ ô nội dung.

## Kiểm thử

- Component: nhóm theo `reason`; rỗng; `linksSupported=false`; con `forbidden` hiện dòng phù hợp; dialog mặc định `reason` đúng cho `spike`, `question`, `hotfix`, `bug`, `security`.
- `spawnChild` gọi đúng `{id,reason,title,body,type?}`; thiếu tiêu đề thì nút khoá.
- Chạy: `pnpm --filter orca-frontend test -- src/renderer/src/components/request/RequestRelatedTab src/renderer/src/components/request/SpawnChildRequestDialog`.

## Tiêu chí hoàn thành

- [ ] Liên kết cha/con hiển thị đúng nhóm hoặc thông báo "chưa hỗ trợ".
- [ ] Tạo con mở thẳng Request con.
- [ ] Phím tắt đa nền tảng; chuỗi i18n `RequestRelatedTab.*`, `SpawnChildRequestDialog.*` đủ 5 locale.

## Rủi ro và lưu ý

- Hợp đồng `links` chưa có: khi CONTRACT ra, thay `linksSupported` bằng RPC thật. Nếu không ai thêm, tab chỉ có giá trị khi `request.get` trả `links`.
- Bảng loại con có thể bị CR-REQ-006 đổi.
