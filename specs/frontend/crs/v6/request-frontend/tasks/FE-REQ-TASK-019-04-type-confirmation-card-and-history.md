# FE-REQ-TASK-019-04: `TypeConfirmationCard`, đổi loại và lịch sử đổi loại

**From Solution:** [FE-REQ-SOL-019](../solutions/FE-REQ-SOL-019-request-list-detail-classification-ui.md) mục 2.4, 2.5
**Priority:** P0
**Area:** frontend / request
**File:** `frontend/src/renderer/src/components/request/TypeConfirmationCard.tsx`, `ChangeTypeConfirmDialog.tsx`, `RequestHistoryTab.tsx` (đều mới); test cùng tên
**Depends on:** FE-REQ-TASK-019-03, 018-03
**Status:** [ ] TODO

## Context

- Kênh: `request.confirmType {id,type,size?,urgency?,reason?}`, `request.changeType {id,toType,reason}` (reason bắt buộc), `request.classify {id}` (timeout 25 s), `request.typeHistory {id}` (CR-REQ-016 2.3).
- Mã lỗi: `REQUEST_TYPE_REQUIRED`, `REQUEST_TYPE_CHANGE_NOT_ALLOWED`, `REQUEST_TYPE_CHANGE_USE_CHILD`, `REQUEST_CLASSIFICATION_LIMIT` (tối đa 5 lần), `REQUEST_STATE_STALE`.
- Phím tắt: `isScreenSubmitShortcut` và `getScreenSubmitModifierLabel` (`lib/screen-submit-shortcut.ts`), chip qua `ShortcutKeyCombo`.
- `ui/progress.tsx`, `ui/select.tsx`, `ui/textarea.tsx` có sẵn. `requests.type` có thể rỗng tới đề xuất đầu tiên (README v6 mục 8 số 3).

## Việc cần làm

1. `TypeConfirmationCard({request})` hiện khi `status==='awaiting_type_confirmation'`: loại đề xuất (`RequestTypeBadge`), `confidence` bằng `Progress` + `%`, `classificationReason` (tối đa 6 dòng, "Xem thêm"), size và urgency đề xuất (có thể sửa), cảnh báo `isLowConfidence`.
2. Bộ chọn loại `Select` 11 mục: mỗi mục kèm mô tả một dòng (`RequestType.<type>.description`) và tóm tắt luồng từ registry (có Plan/Phase, cổng). Khác đề xuất thì hiện ô "Lý do" (tuỳ chọn) và nhãn "Sửa bởi người".
3. "Xác nhận loại" gọi `confirmType`; `Mod+Enter` từ ô lý do cũng gọi; nhãn phím trong tooltip. "Phân loại lại" gọi `classify` (trạng thái đang chạy, vô hiệu khi `rate_limited`/`REQUEST_CLASSIFICATION_LIMIT` kèm lời giải thích).
4. `status==='classifying'`: `Loader2` + "AI đang phân loại"; nếu quá 60 giây hiện nút "Phân loại lại".
5. `type` rỗng (chưa có đề xuất): ô chọn không có giá trị mặc định, nút xác nhận khoá, thông báo `REQUEST_TYPE_REQUIRED` cạnh trường.
6. `ChangeTypeConfirmDialog`: mở khi "Sửa loại" ở trạng thái sau xác nhận; nội dung "Solution, Plan, Task đã có được giữ làm tham chiếu, Request quay về bước xác nhận loại" (README 3.3); lý do bắt buộc; gọi `changeType`. `REQUEST_TYPE_CHANGE_USE_CHILD` thì hiện lời giải thích và nút "Tạo Request con" (019-05).
7. `RequestHistoryTab`: `request.typeHistory`; hàng `from → to`, người (`actorKind==='ai'` hiện "AI"), lý do, thời điểm; mới nhất trên cùng; rỗng "Chưa đổi loại lần nào"; lỗi `network` có "Thử lại".
8. `invalid_state`/`conflict`: toast, `refetch`, hộp tự biến mất khi trạng thái không còn `awaiting_type_confirmation`.

## Kiểm thử

- Component: hiển thị đề xuất, %; chọn loại khác bật "Lý do" và nhãn; `Mod+Enter` gọi `confirmType` với `metaKey` khi mock Mac và `ctrlKey` khi mock Linux/Windows (mock `getShortcutPlatform`); nhãn chip tương ứng; `classify` bị `REQUEST_CLASSIFICATION_LIMIT` thì khoá; `type` rỗng; `ChangeTypeConfirmDialog` lý do rỗng không gửi; lịch sử rỗng.
- `confirmType` được gọi đúng `{id,type,size,urgency,reason}` (không có `typeSource`).
- Chạy: `pnpm --filter orca-frontend test -- src/renderer/src/components/request/TypeConfirmationCard src/renderer/src/components/request/ChangeTypeConfirmDialog src/renderer/src/components/request/RequestHistoryTab`.

## Tiêu chí hoàn thành

- [ ] Đề xuất AI hiển thị đủ loại, %, lý do; cảnh báo độ tin cậy thấp.
- [ ] Đổi loại ghi một dòng mới ở lịch sử sau `refetch`.
- [ ] Phím tắt đúng nền tảng, nhãn khớp.
- [ ] Khoá i18n `TypeConfirmationCard.*`, `ChangeTypeConfirmDialog.*`, `RequestHistoryTab.*` đủ 5 locale.

## Rủi ro và lưu ý

- Quyền xác nhận loại (CR-REQ-010) chưa rõ; chỉ dựa `forbidden`.
- `request.confirmType` bắt buộc người, không MCP (CR-016 2.3): UI không có đường tắt tự duyệt.
