# FE-REQ-TASK-019-02: `RequestsTab` (danh sách, bộ lọc, trạng thái, phím điều hướng)

**From Solution:** [FE-REQ-SOL-019](../solutions/FE-REQ-SOL-019-request-list-detail-classification-ui.md) mục 2.2
**Priority:** P0
**Area:** frontend / request
**File:** `frontend/src/renderer/src/components/request/RequestsTab.tsx`, `RequestListToolbar.tsx`, `RequestFilterBar.tsx`, `RequestList.tsx`, `RequestRow.tsx`, `RequestListStates.tsx`, `request-list-keyboard.ts` (đều mới); test cùng tên
**Depends on:** FE-REQ-TASK-018-03 (`useRequests`), 018-05 (badge, khung), 019-01
**Status:** [ ] TODO

## Context

- Kênh: `request.list {projectId?, status[]?, type[]?, sourceProvider?, sourceSite?, sourceRef?, pageSize, pageToken}` → `{requests, nextPageToken}` (CR-016 2.3). Không có `q`/`reporterId`.
- `ui/table.tsx`, `ui/resizable.tsx`, `ui/skeleton.tsx`, `ui/select.tsx` có sẵn. `requestPage.listFilters`, `requestPage.requestId` ở store (018-04).
- Sự kiện `request.event` qua `request-event-bus`.

## Việc cần làm

1. `RequestsTab`: ghép `RequestListToolbar` + `RequestList` + `RequestDetailPane` (placeholder tới 019-03) trong `ResizablePanelGroup` (trái 40%); dưới 768 px chi tiết thay danh sách kèm nút "Quay lại danh sách".
2. `RequestFilterBar`: nhóm lọc trạng thái, loại, nguồn (`RequestSourceProvider`), chip nhanh "Chờ tôi xác nhận" (`['awaiting_type_confirmation']`) và "Đang chạy" (`['analyzing','planning','executing']`); ghi vào `requestPage.listFilters`; nút "Xoá bộ lọc".
3. `RequestRow`: `#number`, tiêu đề, `RequestTypeBadge` (hoặc "Chưa phân loại" khi `type` rỗng, README mục 8 số 3), `RequestStatusBadge`, `RequestSourceBadge`, chip "Khẩn" khi `urgency==='urgent'`, cập nhật tương đối. Hàng là `button` có `aria-selected`.
4. `RequestListStates`: `RequestListSkeleton` (8 hàng), `emptyNone` (nút "Tạo Request" mở `CreateRequestDialog` ở 019-06), `emptyFiltered`, lỗi `network` (banner + "Thử lại"), `forbidden`; nút "Tải thêm" khi `nextPageToken`.
5. `request-list-keyboard.ts`: `handleRequestListKey(event, {ids, activeId, onMove, onOpen, onClose})`; `j`/`k`/`ArrowDown`/`ArrowUp` di chuyển, `Enter` mở, `Escape` đóng; bỏ qua khi `event.target` là `input|textarea|select|[contenteditable]`, khi `isComposing` hoặc có `ctrlKey|metaKey|altKey` (không hardcode `metaKey` như phím chức năng).
6. Cập nhật tại chỗ: nghe `request-event-bus`; khi `requestId` thuộc trang đang hiện thì refetch nhẹ hàng đó (`request.get`) hoặc cả trang nếu `status` không còn khớp bộ lọc.

## Kiểm thử

- Component: skeleton; rỗng không bộ lọc và có bộ lọc; lỗi `network` có "Thử lại"; `forbidden`; "Tải thêm" nối trang không trùng; chip nhanh đặt đúng filter; chọn hàng đặt `requestId` vào store.
- `request-list-keyboard.test.ts`: `j/k/Enter/Escape`, bỏ qua trong ô nhập, bỏ qua khi IME `isComposing`.
- Chạy: `pnpm --filter orca-frontend test -- src/renderer/src/components/request/Request`.
- Responsive: test `matchMedia` mock cho chế độ hẹp.

## Tiêu chí hoàn thành

- [ ] Đủ trạng thái tải/rỗng/lỗi/không quyền; không toast cho `unsupported`.
- [ ] Phím điều hướng không hoạt động trong ô nhập.
- [ ] Không màu hex, chỉ token; mọi chuỗi qua `translate` với khoá `auto.components.request.RequestsTab.*`, 5 locale.
- [ ] Chịu độ trễ 50 đến 200 ms (không nhấp nháy rỗng trước khi tải).

## Rủi ro và lưu ý

- Khoá i18n rỗng/lọc/tải thêm: `RequestsTab.emptyNone|emptyFiltered|loadMore`.
- `j/k` có thể xung đột phím tắt toàn cục: gắn `onKeyDown` vào container danh sách (cần `tabIndex`), không lên `window`.
