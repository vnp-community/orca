# FE-REQ-TASK-023-04: Khung `BacklogTab`: phân đoạn, thanh công cụ, phím `1/2/3`, trạng thái chung

**From Solution:** [FE-REQ-SOL-023](../solutions/FE-REQ-SOL-023-backlog-screens.md) mục 2.1, 2.5, 2.6
**Priority:** P1
**Area:** frontend (components)
**File:** trong `frontend/src/renderer/src/components/request/backlog/` (mới): `BacklogTab.tsx`, `BacklogSegmentControl.tsx`, `BacklogToolbar.tsx`, `BacklogStates.tsx`, `backlog-view-columns.ts` và test cùng tên; sửa `components/request/RequestPage.tsx` (gắn tab)
**Depends on:** FE-REQ-TASK-023-02; FE-REQ-TASK-022-03, 022-04 (`useRequestSummaries`, `useMinuteClock`, `useRowListKeyboardNavigation`, `isTypingTarget`); FE-REQ-SOL-018 (slice `requestPage.backlogView`, `requestFlowSupport`)
**Status:** [x] DONE (verified 2026-10-07: BacklogTab.test.tsx 9/9, BacklogSegmentControl.test.tsx 3/3, backlog-view-columns.test.ts 4/4; oxlint sạch, không thêm lỗi tsc ở file của task)

## Context

- Phân đoạn đang chọn lưu ở `requestPage.backlogView` (slice SOL-018, không persist qua khởi động lại). Sidebar ẩn tab khi `requestFlowSupport==='unsupported'`.
- `ui/toggle-group.tsx` (`ToggleGroup`, `ToggleGroupItem`), `ui/tooltip.tsx`, `ui/table.tsx`, `ui/skeleton.tsx`, `components/ShortcutKeyCombo.tsx` (`keys: string[]`) có sẵn.
- Số mục: `useBacklog` không có `total`; dùng `countLabel()` (số đã tải, `+` khi còn trang); phân đoạn chưa mở thì không hiện số (D5).
- Bộ lọc client vì `backlog.*` không có `q`/`type`: tìm kiếm theo tiêu đề và `#number`; loại Request chỉ cho view request; nhóm lý do (`ReturnedCategory`) chỉ cho view request. Bộ lọc dự án: bộ chọn dự án của `RequestPageHeader` (SOL-018), truyền `projectId` thẳng vào hook (có ở server).

## Việc cần làm

1. `BacklogTab`: tạo ba hook `useBacklog('request'|'task'|'execute', filters, {active: view===current})` (hoặc ba hook mỏng của task 02); render bảng của phân đoạn đang mở (`RequestBacklogTable`, `TaskBacklogTable`, `ExecuteBacklogTable` do task 05, 06 cung cấp; trong task này đặt placeholder `null` có comment ngắn để khung chạy được độc lập, bỏ khi task 05, 06 xong). Đổi phân đoạn qua `setRequestPageBacklogView` (hoặc action tương ứng của slice SOL-018).
2. `BacklogSegmentControl({value, onChange, counts})`: `ToggleGroup type="single"`, ba mục (`BacklogSegmentControl.{request,task,execute}`), số mục nhỏ bên cạnh khi có; mỗi mục có `Tooltip` chứa `ShortcutKeyCombo keys={['1']}` (hoặc `2`, `3`). Bỏ chọn thì giữ giá trị cũ.
3. Phím `1`, `2`, `3`: `onKeyDown` gắn vào vùng bọc phân đoạn + danh sách; bỏ qua khi `isTypingTarget(event.target)`, khi có `ctrlKey|metaKey|altKey|shiftKey`, khi `isComposing`. Không có nhánh Mac/Windows vì không dùng phím sửa đổi. `j`/`k`/`Enter` do bảng (task 05, 06) dùng `useRowListKeyboardNavigation`.
4. `BacklogToolbar`: ô tìm kiếm (`Input`, debounce 200 ms, lọc client), chọn loại Request (`Select`, 11 loại + tất cả, chỉ view request), chọn nhóm lý do (view request), nút "Làm mới" gọi `refetch` của phân đoạn đang mở. Mọi nhãn qua `translate`.
5. `backlog-view-columns.ts`: mô tả cột theo phân đoạn (`id`, khoá i18n, cờ `sticky`); hàm `filterRequestRows(rows, {q, type, category})`, `filterGroups(groups, {q})` (lọc theo tiêu đề task/Plan/Phase và `#number` nếu có). Thuần, có test.
6. `BacklogStates.tsx`: `BacklogSkeleton({columns})` (8 hàng đúng số cột), `BacklogEmptyState({view})` (ba chuỗi khác nhau), `BacklogEmptyFiltered` + "Xoá bộ lọc", `BacklogErrorState({kind, onRetry})`: `network` banner + "Thử lại" (giữ dữ liệu cũ mờ `opacity-60`), `forbidden` thông điệp "Bạn không có quyền xem backlog của dự án này".
7. Sự kiện/polling nằm ở `useBacklog` (task 02); `BacklogTab` chỉ truyền `active`.
8. `unsupported`: `BacklogTab` trả `null` (sidebar và `RequestPage` ẩn tab); không toast.

## Kiểm thử

- `BacklogSegmentControl.test.tsx`: đổi phân đoạn bằng chuột và bằng phím `1/2/3`; phím bị bỏ qua trong `<input>`; bị bỏ qua khi có `ctrlKey` hoặc `metaKey`; số mục kèm `+` khi còn trang; tooltip có chip phím.
- `backlog-view-columns.test.ts`: bộ lọc `q` (tiêu đề, `#number`), `type`, `category`; chọn cột theo phân đoạn.
- `BacklogTab.test.tsx`: chỉ view đang mở gọi RPC (đếm); lỗi view task không làm hỏng view request; `unsupported` trả `null`; trạng thái rỗng/lỗi/tải đúng; "Xoá bộ lọc".
- Chạy (chưa chạy): `pnpm --filter orca-frontend test frontend/src/renderer/src/components/request/backlog`.

## Tiêu chí hoàn thành

- [ ] Ba phân đoạn đổi được; chuyển phân đoạn không gọi lại phân đoạn khác.
- [ ] Phím `1/2/3` hoạt động ở thanh phân đoạn và danh sách, bị bỏ qua ở ô nhập.
- [ ] Mỗi phân đoạn có UI riêng cho rỗng, đang tải, lỗi mạng, lỗi quyền.
- [ ] Không có màu hex hoặc lớp màu thô.
- [ ] Số mục ghi rõ `+` khi chỉ là số đã tải.

## Rủi ro và lưu ý

- Nếu SOL-018 chưa có action đổi `backlogView`, thêm `setRequestPageBacklogView` vào slice (báo người điều phối, tránh sửa trùng).
- Bộ lọc client chỉ áp lên trang đã tải; lọc "rỗng" có thể xảy ra khi kết quả nằm ở trang sau: hiện nút "Tải thêm" kể cả khi lọc rỗng nếu còn `nextPageToken`.

## Ghi chú triển khai (2026-10-07)

- Đổi phân đoạn qua `setRequestPageData({backlogView})`. Phím `1/2/3` bắt ở vùng bọc `BacklogTab`. Nhãn số mục chỉ hiện sau khi phân đoạn đã tải.
