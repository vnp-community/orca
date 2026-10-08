# FE-REQ-TASK-022-06: `ApprovalInboxTab`: lắp ghép, xác nhận, từ chối, điều hướng, chấm số

**From Solution:** [FE-REQ-SOL-022](../solutions/FE-REQ-SOL-022-approval-inbox.md) mục 2.5, 2.6, bảng Correction C3, C7
**Priority:** P1
**Area:** frontend (components, store wiring)
**File:** `frontend/src/renderer/src/components/request/approval/ApprovalInboxTab.tsx` (mới) + test; sửa `components/request/RequestPage.tsx` (SOL-018) để gắn tab; sửa `components/sidebar/SidebarNav.test.tsx` (mở rộng)
**Depends on:** FE-REQ-TASK-022-02, 022-03, 022-05; FE-REQ-SOL-018 (`RequestPage`, `openRequestPage`, slice, chấm số sidebar), FE-REQ-SOL-020 (`RejectReasonDialog`)
**Status:** [x] DONE (verified 2026-10-07: ApprovalInboxTab.test.tsx 16/16; RequestPage.test.tsx 4/4; SidebarRequestNavButton.test.tsx đã có test badge 99+; oxlint sạch, không thêm lỗi tsc ở file của task)

## Context

- `RequestPage` (SOL-018) có `Tabs` với "Hộp duyệt (số)"; `ApprovalInboxTab` điền thân. Tab ẩn khi `requestFlowSupport === 'unsupported'`.
- `openRequestPage(data?)`, `setRequestPageSection`, `pendingApprovalCount`, `setPendingApprovalCount` thuộc slice `request` (SOL-018). `focus` cần `requestPage.focus` (yêu cầu bổ sung cho SOL-018; thiếu thì bỏ `focus`).
- `components/sidebar/SidebarNav.tsx` import `SidebarTaskNavButton`; SOL-018 thêm nút "Requests" và chấm số. Task này chỉ kiểm lại chấm số theo `pendingApprovalCount`, không viết lại nút.
- Thông báo hệ thống (CR-REQ-010) gọi `openRequestPage({section:'approvals'})`; không tạo thông báo ở đây.
- `sonner` `toast` đã dùng khắp repo (`TaskDetail.tsx`).

## Việc cần làm

1. `ApprovalInboxTab`: state `subjectGroup`, `projectId` (lấy từ bộ chọn dự án của `RequestPageHeader` qua slice hoặc props, theo SOL-018), `overdueOnly`; gọi `useApprovalInbox`, `useRequestSummaries(ids)`, `useMinuteClock()`; sắp xếp bằng `compareApprovals(now)` rồi `groupByRequest`.
2. Chọn UI theo trạng thái: `isLoading && rows.length===0` → Skeleton; `error.kind==='forbidden'` → `forbidden`; `error.kind==='network'` → banner + danh sách cũ `opacity-60`; không có hàng và không lọc → rỗng; không có hàng nhưng có lọc → rỗng khi lọc (nút "Xoá bộ lọc" đặt lại ba bộ lọc).
3. `onOpen(approval)`: `openRequestPage(openTargetFor(approval))`. Nếu `useRequestSummaries` báo `notFound` cho `requestId` thì toast `ApprovalRow.requestGone` và bỏ hàng, không điều hướng.
4. `onQuickApprove(approval)`: chỉ khi `canQuickApprove`; `confirm = useConfirmationDialog()` với `title` = tiêu đề Request + nhãn subject, `description` = `t(quickApproveConsequenceKey)`, `confirmLabel` = `ApprovalRow.approve`; nếu xác nhận gọi `approve(row)`; đặt `busyIds` trong lúc chờ. Kết cục:
   - `ok`: bỏ hàng, toast thành công ngắn.
   - `closed`: toast trung tính `alreadyDecided`.
   - `changed`: toast `changed` với nút "Mở" (`openTargetFor`).
   - `forbidden`: toast lỗi `error.forbidden` (khoá của SOL-018), giữ hàng, `refetch`.
   - `network`/`unknown`: toast lỗi chung; không đóng hàng.
5. `onReject(approval)`: mở `RejectReasonDialog` với `minLength: 10`, `onSubmit(comment)` gọi `reject(row, comment)`; kết cục như bước 4; `validation` hiện lỗi trong dialog. `Mod+Enter` gửi do dialog tự xử lý bằng `isScreenSubmitShortcut`; nhãn từ `getScreenSubmitShortcutLabel()` (`⌘ Enter` trên Mac, `Ctrl+Enter` nơi khác). Không tự đóng dialog khi lỗi `network`.
6. Chấm số: sau mỗi lần tải trang đầu, nếu `rows` của trang đầu hết hàng và không còn `nextPageToken` thì `setPendingApprovalCount(0)`. Nguồn chính của số là lời gọi đếm `pageSize=100` của SOL-018 (`99+` khi `>99` hoặc còn `nextPageToken`); tab chỉ cập nhật khi trang đầu chưa lọc (`subjectGroup==='all'`, không lọc client) để số không phụ thuộc bộ lọc đang chọn.
7. Gắn `ApprovalInboxTab` vào `RequestPage` thay chỗ tạm của SOL-018; tiêu đề tab dùng `ApprovalInboxTab.title` + `pendingApprovalCount` (hiển thị `99+`).
8. Không giữ danh sách Approval trong store (chỉ `pendingApprovalCount`).

## Kiểm thử

- `ApprovalInboxTab.test.tsx` (mock `useApprovalInbox`, `useRequestSummaries`, `useConfirmationDialog`, `openRequestPage`): rỗng, rỗng khi lọc, lỗi mạng giữ danh sách mờ, `forbidden`; "Mở" đúng đích cho 8 `subjectType`; duyệt nhanh gọi hộp xác nhận rồi `approve`; xác nhận bị từ chối thì không gọi; Từ chối với lý do 9 ký tự không gọi RPC (nút khoá), 10 ký tự gọi với đúng `comment`, hàng biến mất; `closed` không toast đỏ; `changed` toast có nút "Mở".
- Mở rộng `SidebarNav.test.tsx`: chấm số hiện `pendingApprovalCount`, `99+` khi 100, ẩn khi 0 hoặc `unsupported`.
- Test điều kiện Mac/Windows của `Mod+Enter` nằm ở SOL-020 (`RejectReasonDialog`); ở đây chỉ kiểm nhãn bằng cách mock `getShortcutPlatform`.
- E2E (cần backend): hai người dùng, A duyệt, hàng của B biến mất sau `refetch`; mở từ hộp duyệt tới Solution (`tests/` Playwright, lệnh `pnpm test:e2e` ở gốc, chưa kiểm chứng).

Chạy (chưa chạy): `pnpm --filter orca-frontend test frontend/src/renderer/src/components/request/approval/ApprovalInboxTab.test.tsx frontend/src/renderer/src/components/sidebar/SidebarNav.test.tsx`.

## Tiêu chí hoàn thành

- [ ] Hộp duyệt chỉ hiện `pending` của chính người dùng (do `listPending`), nhóm theo Request, quá hạn trên cùng.
- [ ] `solution` và `plan` chỉ có "Mở".
- [ ] Từ chối lý do rỗng: nút khoá, không có RPC.
- [ ] Approval đã bị xử lý: hàng biến mất kèm toast trung tính.
- [ ] Runtime không hỗ trợ `approval.*`: không tab, không chấm số, không toast.
- [ ] `Mod+Enter` trong dialog đúng `metaKey` Mac và `ctrlKey` nơi khác (kiểm chứng ở SOL-020, xác nhận lại bằng thủ công).

## Rủi ro và lưu ý

- Duyệt nhanh `phase` và `pre_deploy` là hành động mạnh (chạy agent); hộp xác nhận ngắn có thể chưa đủ (CR-022 mục 6); `description` phải nêu hậu quả cụ thể.
- Duyệt tự (người yêu cầu cũng là người duyệt): backend chặn bằng `REQUEST_APPROVAL_SELF_APPROVAL_FORBIDDEN`, UI không tự chặn, chỉ báo `forbidden`.
- SSH/remote: mọi lời gọi qua `callRequestRpc`; không dùng đường dẫn cục bộ.

## Ghi chú triển khai (2026-10-07)

- Chưa mở rộng `SidebarNav.test.tsx` (badge sidebar thuộc `SidebarRequestNavButton`, đã có test). Đếm sidebar: `useRequestEvents` đổi `approval.listPending` sang `pageSize: 100`. Tiêu đề tab Approvals hiển thị số (99+). Không có hàm `openRequestPage` sẵn: thêm `request-page-navigation.ts`; `RequestDetailPane` đọc `requestPage.focus` làm tab khởi đầu rồi xoá. E2E chưa chạy (cần backend).
