# FE-REQ-TASK-023-03: `ReopenRequestDialog` và `CancelRequestDialog`

**From Solution:** [FE-REQ-SOL-023](../solutions/FE-REQ-SOL-023-backlog-screens.md) mục 2.7, bảng Correction C6, C7
**Priority:** P1
**Area:** frontend (components, hành động ghi)
**File:** `frontend/src/renderer/src/components/request/backlog/ReopenRequestDialog.tsx`, `CancelRequestDialog.tsx` (mới), test cùng tên (mới)
**Depends on:** FE-REQ-SOL-018 (`useRequestActions().reopen/cancel`, `RequestRpcError`); không phụ thuộc task 01, 02
**Status:** [x] DONE (verified 2026-10-07: ReopenRequestDialog.test.tsx 5/5; RequestBacklogTable.test.tsx (luồng Hủy); oxlint sạch, không thêm lỗi tsc ở file của task)

## Context

- `request.reopen {id}` và `request.cancel {id, reason}` (CR-016 mục 2.3). Không có `note` hay `expectedVersion` ở kênh (CR-006 proto có, CR-016 không): CR-023 mục 2.4 muốn ghi chú tuỳ chọn và `version`; theo CR-016 không gửi được. Dialog không có ô ghi chú cho Mở lại.
- Hành vi backend (CR-006 mục 2.4): mở lại chỉ từ `request_backlog`, đưa về `classifying` (phân loại lại), `type` cũ giữ làm gợi ý; Solution, Plan, Task cũ giữ làm tham chiếu. Lỗi: `REQUEST_REOPEN_NOT_ALLOWED` (không ở backlog), `REQUEST_TRANSITION_NOT_ALLOWED`, `REQUEST_STATE_STALE`, `REQUEST_VERSION_CONFLICT`. Hủy: `REQUEST_REASON_REQUIRED` có thể bắt buộc lý do.
- `ui/dialog.tsx` có `Dialog, DialogContent, DialogHeader, DialogFooter`; `ui/textarea.tsx`, `ui/button.tsx` (`variant="destructive"` chỉ cho hành động mất dữ liệu: Hủy Request đúng loại này).
- `components/confirmation-dialog.tsx` chỉ có hai nút, không có ô nhập, nên không dùng cho hủy (cần lý do).

## Việc cần làm

1. `ReopenRequestDialog({open, onOpenChange, request: {id, number, title, returnedFromStage}, onReopened})`: tiêu đề `ReopenRequestDialog.title`; nội dung `ReopenRequestDialog.body` nêu "Request #{{number}} bị trả về từ bước {{stage}} và sẽ được phân loại lại; Solution, Plan, Task đã có được giữ làm tham chiếu"; `stage` dịch bằng `ReturnedFromStage.<stage>`. Nút `ReopenRequestDialog.confirm`; trong lúc gọi khoá nút và hiện trạng thái đang xử lý.
2. Gọi `reopen({requestId})` (qua `useRequestActions`); xử lý `Result`: thành công → `onOpenChange(false)`, gọi `onReopened(requestId)`; `invalid_state` hoặc `conflict` (đã được mở lại hoặc đổi trạng thái) → toast trung tính `BacklogRow.alreadyHandled`, đóng dialog, gọi `onReopened(requestId)` để bỏ hàng và tải lại; `forbidden` → toast lỗi `error.forbidden`, giữ dialog; `network`/`unknown` → lỗi nội tuyến trong dialog, giữ mở.
3. `CancelRequestDialog({open, onOpenChange, request, onCancelled})`: `variant="destructive"` cho nút xác nhận; ô `Textarea` lý do tuỳ chọn (tối đa 2000 ký tự, bộ đếm); nội dung `CancelRequestDialog.body` nêu hủy không hoàn tác ở v6 (CR-006: `cancelled` không mở lại được).
4. Gọi `cancel({requestId, reason})`; nếu server trả `REQUEST_REASON_REQUIRED` hoặc `validation`: đánh dấu ô lý do bắt buộc, hiện lỗi cạnh ô, không đóng dialog. `Mod+Enter` trong ô lý do gửi form: dùng `isScreenSubmitShortcut(event)` từ `lib/screen-submit-shortcut.ts` và hiển thị nhãn bằng `getScreenSubmitShortcutLabel()` (Mac `⌘ Enter`, nơi khác `Ctrl+Enter`); không hardcode `metaKey`.
5. Cả hai dialog: đặt tiêu điểm đầu vào nút an toàn (Mở lại: nút xác nhận; Hủy: ô lý do), đóng bằng `Escape`, không gửi khi `busy`.
6. Không giữ trạng thái trong store; hàng được bỏ bởi `BacklogTab` qua callback.

## Kiểm thử

- `ReopenRequestDialog.test.tsx`: hiện đúng `stage` đã dịch; thành công gọi `reopen` đúng id và `onReopened`; `invalid_state` toast trung tính và đóng; `forbidden` giữ mở; hai lần bấm nhanh chỉ gọi một lần.
- `CancelRequestDialog.test.tsx`: nút xác nhận `destructive`; gửi `reason` rỗng khi không bắt buộc; sau `REQUEST_REASON_REQUIRED` ô trở thành bắt buộc; `Mod+Enter` đúng `metaKey` trên Mac và `ctrlKey` trên Windows/Linux (giả `getShortcutPlatform` hoặc `navigator.userAgent`), và **không** gửi khi chỉ nhấn `Enter`.
- Chạy (chưa chạy): `pnpm --filter orca-frontend test frontend/src/renderer/src/components/request/backlog/ReopenRequestDialog.test.tsx frontend/src/renderer/src/components/request/backlog/CancelRequestDialog.test.tsx`.

## Tiêu chí hoàn thành

- [ ] Mở lại gọi đúng `request.reopen {id}`; hủy gọi `request.cancel {id, reason}`.
- [ ] Dialog Mở lại nói rõ Request được phân loại lại (không hứa "quay về bước cũ").
- [ ] Hủy cần xác nhận, nút `destructive`, lý do tuỳ chọn trừ khi server bắt buộc.
- [ ] Lỗi `invalid_state` làm hàng biến mất kèm toast trung tính, không báo đỏ.
- [ ] Phím gửi theo nền tảng, không hardcode `metaKey`.

## Rủi ro và lưu ý

- Ô ghi chú khi mở lại bị thiếu so với CR-023 vì kênh chưa nhận `note` (SOL-023 Q2); thêm khi CR-016 cập nhật.
- Quyền: người xem không có quyền ghi vẫn thấy nút (CR-023 mục 2.4), lỗi xử lý bằng toast; không có `viewerCan` (SOL-018 Q4).

## Ghi chú triển khai (2026-10-07)

- Dùng lại `CancelRequestDialog` có sẵn (đợt 019, dựa `RequestReasonDialog`, đã có `Mod+Enter` qua `isScreenSubmitShortcut`) thay vì tạo bản thứ hai ở `backlog/`; thêm prop tuỳ chọn `reasonRequired`. Bắt `Mod+Enter` được kiểm ở test của `RequestReasonDialog`/`RejectReasonDialog`.
