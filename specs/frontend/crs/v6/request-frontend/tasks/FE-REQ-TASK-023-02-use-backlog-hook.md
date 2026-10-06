# FE-REQ-TASK-023-02: Hook `useBacklog` ba view (phân trang, sự kiện, polling, lỗi riêng từng view)

**From Solution:** [FE-REQ-SOL-023](../solutions/FE-REQ-SOL-023-backlog-screens.md) mục 2.3, bảng Correction C1, C2, C9
**Priority:** P1
**Area:** frontend (hooks)
**File:** `frontend/src/renderer/src/hooks/useBacklog.ts` (SOL-018 tạo, task này chốt chữ ký; nếu chưa có thì tạo mới), `hooks/useBacklog.test.tsx` (mới)
**Depends on:** FE-REQ-TASK-023-01; FE-REQ-SOL-018 (`callRequestRpc`, `RequestRpcError`, `subscribeRequestEvents`, `setRequestFlowSupport`)
**Status:** [ ] TODO

## Context

- CR-016 mục 2.4: `backlog.requests {projectId?, groupBy?, pageSize, pageToken}`, `backlog.tasks {projectId?, planTaskId?, pageSize, pageToken}`, `backlog.execute {projectId?, phaseTaskId?, pageSize, pageToken}`, timeout 8 s. CR-018 và CR-023 viết `backlog.list {view}`: sai theo CR-016, không dùng.
- CR-015 mục 2.5: trang tính theo Request; một trang có thể ít nhóm hơn `page_size`; `next_page_token` là chuỗi mờ (base64). Lỗi `REQUEST_BACKLOG_TASK_SERVICE_UNAVAILABLE` (view task, execute), `REQUEST_BACKLOG_BAD_PAGE_TOKEN`, `REQUEST_BACKLOG_INVALID_VIEW`.
- Sự kiện: chỉ `request.event {requestId, eventType, status, type, occurredAt}` từ `request.subscribe` (CR-016 mục 2.5), nhận qua `subscribeRequestEvents` của SOL-018.
- Mẫu hook: `hooks/useTaskSource.ts` (huỷ khi unmount), `useTaskDependencyEdges.ts` (`refetchKey`).

## Việc cần làm

1. Chữ ký: `useBacklog(view: BacklogView, filters: {projectId?: string; planTaskId?: string; phaseTaskId?: string}, options?: {active?: boolean})` trả `{items, nextPageToken, isLoading, isLoadingMore, error, loadedOnce, hasMore, loadMore, refetch, supported}`. `items` có kiểu theo `view`: `RequestBacklogRowData[]` hoặc `BacklogGroupData[]` (dùng kiểu có điều kiện hoặc ba hook mỏng `useRequestBacklog`, `useTaskBacklog`, `useExecuteBacklog` bọc một lõi chung `useBacklogPages`; chọn cách thứ hai nếu kiểu có điều kiện làm rối kiểu trả về).
2. Lõi `useBacklogPages`: tải trang đầu khi `active` và khi `filters` đổi (so sánh theo giá trị); `pageSize` 50 (request), 20 (task, execute); ghép trang, khử trùng theo `requestId` (view request) hoặc `phaseTaskId ?? planTaskId ?? requestId` (view nhóm); nếu trang trả về rỗng nhưng còn `nextPageToken` thì tự gọi tiếp tối đa 3 lần liên tiếp rồi dừng (CR-015 mục 2.5).
3. Trạng thái **riêng cho từng view**: hook chỉ giữ state của view mình; `BacklogTab` giữ ba instance (chỉ `active` cho view đang mở) để cache khi chuyển view và không gọi view khác (tiêu chí "chuyển phân đoạn không gọi lại phân đoạn khác").
4. Lỗi: dùng `RequestRpcError.kind`; `unsupported` → `setRequestFlowSupport('unsupported')` và `supported=false`, không toast; `REQUEST_BACKLOG_BAD_PAGE_TOKEN` → xoá token và tải lại từ trang đầu một lần; `REQUEST_BACKLOG_TASK_SERVICE_UNAVAILABLE` và lỗi truyền tải → `error.kind='network'`; `forbidden` → `error.kind='forbidden'`. Giữ `items` cũ khi lỗi.
5. Làm mới: sự kiện có `eventType` thuộc `{request.returned, request.status_changed, plan.generated, phase.started, phase.completed, approval.decided}` (so khớp theo hậu tố sau tiền tố `orca.request.` nếu có) thì `refetch()` debounce 500 ms, chỉ khi `active`. Polling 30 s khi `active` và `document.visibilityState==='visible'`. `refetch()` giữ `items` cũ cho tới khi trang đầu mới về (tránh nhấp nháy).
6. Không có `total`: `hasMore = nextPageToken !== null`. Hook xuất thêm `countLabel(): {count: number; plus: boolean}` (số hàng/nhóm đã tải, `plus` khi `hasMore`) cho nhãn phân đoạn (task 04).
7. Runtime local, environment và SSH: chỉ qua `callRequestRpc`.

## Kiểm thử

`useBacklog.test.tsx` (mock `callRequestRpc`, fake timers):
- đúng kênh theo view (`backlog.requests|tasks|execute`) và đúng tham số; không có `tenantId`;
- phân trang `loadMore`; khử trùng; trang rỗng có token tự gọi tiếp tối đa 3 lần;
- `active=false` không gọi RPC; chuyển view không gọi view khác;
- lỗi view task không ảnh hưởng state của view request (hai instance);
- `unsupported` không toast; `BAD_PAGE_TOKEN` tải lại một lần; `TASK_SERVICE_UNAVAILABLE` → `network`;
- sự kiện `phase.completed` gọi lại trang đầu sau debounce; sự kiện lạ thì không;
- polling dừng khi tab ẩn, dọn timer khi unmount.

Chạy (chưa chạy): `pnpm --filter orca-frontend test frontend/src/renderer/src/hooks/useBacklog.test.tsx`.

## Tiêu chí hoàn thành

- [ ] Không có lời gọi `backlog.list`.
- [ ] Lỗi một view không làm hỏng view khác; không gọi RPC của view không mở.
- [ ] Trang rỗng có token không làm bảng nhấp nháy "rỗng" (tự tải tiếp tới 3 lần).
- [ ] Timer, đăng ký sự kiện được dọn khi unmount.
- [ ] Không `any`.

## Rủi ro và lưu ý

- Chữ ký này thay đổi hợp đồng của SOL-018 (`useBacklog(view, filters)` trả `{items, ...}` phẳng); báo người điều phối để SOL-018 cập nhật.
- Polling 30 s nhân với số cửa sổ; chỉ hoạt động khi tab hiển thị.
- Khoá lọc `type`/`q` không đi qua hook (lọc client ở task 04).
