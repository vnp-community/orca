# FE-REQ-TASK-022-05: `ApprovalRow`, `ApprovalList`, thanh lọc và các trạng thái rỗng/tải/lỗi

**From Solution:** [FE-REQ-SOL-022](../solutions/FE-REQ-SOL-022-approval-inbox.md) mục 2.1, 2.5
**Priority:** P1
**Area:** frontend (components)
**File:** trong `frontend/src/renderer/src/components/request/approval/` (mới): `ApprovalRow.tsx`, `ApprovalList.tsx`, `ApprovalSubjectIcon.tsx`, `ApprovalDueLabel.tsx`, `ApprovalInboxToolbar.tsx`, `ApprovalSubjectFilter.tsx`, `ApprovalInboxStates.tsx` và test cùng tên
**Depends on:** FE-REQ-TASK-022-01, 022-03, 022-04; FE-REQ-SOL-018 (`RequestTypeBadge`); FE-REQ-SOL-020 (`RejectReasonDialog`)
**Status:** [x] DONE (verified 2026-10-07: approval/ApprovalRow 13, ApprovalList 4, ApprovalInboxStates 4 tests; oxlint sạch, không thêm lỗi tsc ở file của task)

## Context

- `ui/toggle-group.tsx`, `ui/table.tsx`, `ui/skeleton.tsx` (`Skeleton({className})`), `ui/badge.tsx`, `ui/button.tsx`, `ui/tooltip.tsx` đều có sẵn. Không có `alert`/`switch`: banner lỗi viết bằng `div` token.
- `components/confirmation-dialog.tsx`: `useConfirmationDialog()` trả `(opts: {title, description?, confirmLabel?, cancelLabel?, confirmVariant?}) => Promise<boolean>`.
- Màu chỉ dùng token (STYLEGUIDE, `guides/STYLEGUIDE.md`): `text-destructive` cho quá hạn, `text-muted-foreground` cho phần phụ. Cấm hex và lớp màu thô (khác `ExecutionEngineBadge` hiện tại).
- Kích cỡ icon trống: `size-7` theo CR-022 mục 2.5; lucide: `Inbox`, `AlertTriangle`, `ShieldCheck`, `ListChecks`, `Layers`, `Rocket`, `HelpCircle`.
- Component nhận dữ liệu và callback qua props (không tự gọi RPC) để test dễ; việc nạp ở task 06.

## Việc cần làm

1. `ApprovalSubjectIcon`: map `subjectType → icon` (8 + unknown); `aria-hidden`; nhãn chữ đi kèm (không dựa màu).
2. `ApprovalDueLabel({dueAt, now})`: dùng `formatDueState`; quá hạn thì icon `AlertTriangle` + `text-destructive` + `title` với `formatAbsoluteTime`; không có hạn thì không hiện gì.
3. `ApprovalRow(props: {approval; request?: OrcaRequest; now; canQuick: boolean; busy: boolean; onOpen; onQuickApprove; onReject})`: hiện icon + nhãn `ApprovalSubjectType.<type>`, tiêu đề Request (`#number title`; chưa nạp thì `#` + 8 ký tự đầu của `requestId`, `aria-busy`), `RequestTypeBadge` nếu có `request.type`, `requestedBy` (`system` → `ApprovalRow.systemActor`), thời điểm tạo tương đối (tooltip tuyệt đối), `ApprovalDueLabel`. Nút: "Mở" (`variant="outline"`), "Duyệt nhanh" chỉ khi `canQuick` (`variant="default"`), "Từ chối" (`variant="ghost"`; `destructive` chỉ dành cho mất dữ liệu theo STYLEGUIDE). Khi `busy` khoá cả ba nút. Không phím tắt cho Duyệt/Từ chối.
4. `ApprovalList({groups; now; busyIds; onOpen; onQuickApprove; onReject; onLoadMore; hasMore; isLoadingMore})`: mỗi nhóm có tiêu đề Request (tiêu đề, `RequestTypeBadge`, số mục); gắn `useRowListKeyboardNavigation` cho toàn danh sách phẳng; `Enter` gọi `onOpen`; nút "Tải thêm" (`ApprovalInboxTab.loadMore`).
5. `ApprovalSubjectFilter({value, onChange})`: `ToggleGroup type="single"`, 7 mục (`all, requestType, solution, plan, phase, preDeploy, other`); khi người dùng bỏ chọn thì quay về `all`. `OverdueToggle` (`ui/toggle`, nhãn `OverdueToggle.label`). `ProjectFilter` tái dùng bộ chọn dự án của `RequestPageHeader` (SOL-018): nhận `projectId` và `onChange` qua props, không tự dựng bộ chọn mới.
6. `ApprovalInboxStates.tsx`: `ApprovalInboxSkeleton` (6 hàng), `ApprovalInboxEmptyState` (`Inbox size-7` + `ApprovalInboxTab.empty`), `ApprovalInboxEmptyFiltered` (+ nút `clearFilters`), `ApprovalInboxErrorState({kind:'network'|'forbidden'; onRetry})` (banner `div` token + nút "Thử lại" chỉ cho `network`).
7. Duyệt nhanh và Từ chối **không** xử lý trong `ApprovalRow`: phát `onQuickApprove(approval)` / `onReject(approval)`; xác nhận và dialog do `ApprovalInboxTab` (task 06) chịu trách nhiệm.

## Kiểm thử

- `ApprovalRow.test.tsx`: 8 `subjectType` × nút hiện đúng ("Duyệt nhanh" ẩn với `solution`, `plan`; ẩn khi `canQuick=false`); quá hạn có icon cảnh báo; `busy` khoá nút; Request chưa nạp hiện id rút gọn.
- `ApprovalList.test.tsx`: nhóm theo Request; `j`/`k`/`Enter` (gọi `onOpen`); phím bị bỏ qua khi tiêu điểm ở nút; "Tải thêm".
- `ApprovalInboxStates.test.tsx`: bốn trạng thái + rỗng khi lọc; nút "Xoá bộ lọc".
- Mock `@/i18n/i18n` `translate` theo cách `task-jira-link` các test khác đang làm (kiểm thử theo khoá, không theo chuỗi tiếng Anh).

Chạy (chưa chạy): `pnpm --filter orca-frontend test frontend/src/renderer/src/components/request/approval`.

## Tiêu chí hoàn thành

- [ ] Không có màu hex hoặc lớp màu thô trong file mới (`grep -nE "#[0-9a-fA-F]{3,6}|(text|bg|border)-(red|blue|green|purple|gray)-"` rỗng).
- [ ] "Duyệt nhanh" chỉ hiện đúng bảng 8 giá trị.
- [ ] Danh sách có `role` và tên truy cập; chỉ một hàng `tabIndex=0`.
- [ ] Mọi chuỗi qua `translate` với khoá ở SOL-022 mục 2.6.
- [ ] Không file lớn hơn mức `max-lines`; không thêm `max-lines` disable.

## Rủi ro và lưu ý

- Danh sách dài (nhiều trang) chưa ảo hoá; 50 hàng mỗi trang chấp nhận được, đo lại nếu người dùng tải nhiều trang.
- Trang hộp duyệt chạy trong cửa sổ Electron và web; không dùng API chỉ có ở desktop.

## Ghi chú triển khai (2026-10-07)

- `ProjectFilter` không dựng mới: dùng bộ chọn dự án của `RequestPageHeader` (đọc `listFilters.projectId` từ store). Hàng bọc `div role=option` trong `ApprovalList`; `ApprovalRow` chỉ là nội dung.
