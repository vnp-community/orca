# FE-REQ-TASK-023-05: `RequestBacklogTable` và hàng Request bị trả về (Mở lại, Hủy, điều hướng)

**From Solution:** [FE-REQ-SOL-023](../solutions/FE-REQ-SOL-023-backlog-screens.md) mục 2.4, 2.7, bảng Correction C8
**Priority:** P1
**Area:** frontend (components)
**File:** `frontend/src/renderer/src/components/request/backlog/RequestBacklogTable.tsx`, `RequestBacklogRow.tsx` (mới), test cùng tên (mới); nối vào `BacklogTab.tsx`
**Depends on:** FE-REQ-TASK-023-01, 023-03, 023-04; FE-REQ-TASK-022-03 (`formatRelativeTime`, `formatAbsoluteTime`, `useMinuteClock`); FE-REQ-SOL-018 (`RequestTypeBadge`, `RequestSourceBadge`, `openRequestPage`)
**Status:** [ ] TODO

## Context

- Cột theo CR-023 mục 2.3, bổ sung `returnedCategory` (CR-006/015): Request, Nguồn, Loại, Giai đoạn bị trả, Nhóm lý do, Lý do, Người trả, Thời điểm, Hành động.
- `RequestSourceBadge` (SOL-018, mẫu `components/task/TaskSourceBadge.tsx`) đã chặn giao thức ngoài http/https; parser task 01 cũng bỏ `sourceUrl` không hợp lệ.
- `ui/table.tsx` (`Table, TableHeader, TableRow, TableHead, TableBody, TableCell`) có sẵn; tiêu đề cột dính khi cuộn dùng `sticky top-0 bg-background`.
- `returnedBy` có thể là `system` hoặc rỗng khi AI trả: hiển thị "AI/Hệ thống" (`ApprovalRow.systemActor` thuộc SOL-022; ở đây dùng khoá riêng `RequestBacklogTable.actorSystem`, không mượn khoá hộp duyệt).

## Việc cần làm

1. `RequestBacklogRow({row, now, onOpen, onReopen, onCancel})`: ô Request (`#number` + tiêu đề, là nút mở chi tiết); `RequestSourceBadge` khi có `sourceProvider`; `RequestTypeBadge` khi có `type` (null thì `-`); `ReturnedFromStage.<stage>` (`unknown` → `-`); `Badge variant="outline"` cho `ReturnedCategory.<c>`; lý do cắt 2 dòng (`line-clamp-2`) kèm `Tooltip` đầy đủ; người trả; thời điểm tương đối (tooltip tuyệt đối); nút "Mở lại" (`outline`) và "Hủy" (`ghost`).
2. `RequestBacklogTable({page, now, filters, onRetry, ...})`: nhận kết quả đã lọc client (`filterRequestRows`), dựng bảng, gắn `useRowListKeyboardNavigation` (`j`/`k`/`Enter` → `onOpen`; `Enter` trên nút trong hàng không mở). Hàng đầu theo thứ tự server (`updated_at DESC, id DESC`); không sắp xếp lại ở client.
3. Điều hướng: `onOpen(row)` → `openRequestPage({section:'requests', requestId: row.requestId})`.
4. Nối dialog: giữ `reopenTarget` và `cancelTarget` trong state của bảng; render `ReopenRequestDialog` / `CancelRequestDialog` (task 03); `onReopened(id)` và `onCancelled(id)` bỏ hàng khỏi danh sách hiển thị ngay (state `removedIds` cục bộ) rồi gọi `refetch`.
5. Toast sau Mở lại thành công: `BacklogRow.reopened` kèm nút hành động `ReopenRequestDialog.viewRequest` gọi `openRequestPage({section:'requests', requestId})` (CR-023 mục 2.4).
6. Không có hành động ghi nào khác; không chạm trạng thái Task.
7. Cột cuối "Hành động" luôn hiện cả hai nút (không phụ thuộc `viewerCan`); lỗi `forbidden` xử lý bằng toast ở dialog (task 03).
8. Trạng thái rỗng, lỗi, tải do `BacklogTab` dựng bằng `BacklogStates` (task 04): bảng chỉ chịu trách nhiệm khi có dữ liệu.

## Kiểm thử

- `RequestBacklogRow.test.tsx`: đủ cột; `returnedFromStage: 'plan'` hiện nhãn đã dịch; `sourceUrl` `javascript:` không thành liên kết; lý do dài có `line-clamp` và tooltip; người trả `system` hiện "AI/Hệ thống".
- `RequestBacklogTable.test.tsx`: "Mở lại" mở dialog, xác nhận gọi `request.reopen`, hàng biến mất, toast có nút "Xem Request"; "Hủy" cần xác nhận; `j`/`k`/`Enter` mở đúng Request; `Enter` trên nút "Hủy" không điều hướng; lỗi `invalid_state` bỏ hàng kèm toast trung tính.
- Chạy (chưa chạy): `pnpm --filter orca-frontend test frontend/src/renderer/src/components/request/backlog/RequestBacklogTable.test.tsx`.

## Tiêu chí hoàn thành

- [ ] Hiện `returnedFromStage`, `returnedCategory`, lý do, người trả, thời điểm, liên kết gốc (chỉ http/https).
- [ ] Mở lại gọi `request.reopen`, hàng biến mất, toast có "Xem Request"; hủy cần xác nhận.
- [ ] Không có màu hex hay lớp màu thô; tiêu đề cột dính khi cuộn.
- [ ] Phím `j`/`k`/`Enter` hoạt động và bị bỏ qua ở ô nhập.
- [ ] Mọi chuỗi qua `translate`.

## Rủi ro và lưu ý

- `parentRequestIds` chưa có chỗ hiển thị (CR-023 không yêu cầu); chỉ giữ trong dữ liệu.
- Thứ tự "ổn định khi có hàng mới chen vào" do keyset phía backend; client không sắp xếp lại.
- Bảng rộng: trên cửa sổ hẹp cuộn ngang trong vùng bảng, không làm cuộn ngang cả trang.
