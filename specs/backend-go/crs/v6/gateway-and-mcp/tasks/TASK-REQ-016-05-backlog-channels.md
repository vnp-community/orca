# TASK-REQ-016-05: Kênh `backlog.requests`, `backlog.tasks`, `backlog.execute`

**From Solution:** BE-REQ-SOL-016
**Priority:** P1
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels_request_backlog.go` (mới), `.../channels_request_backlog_test.go` (mới), `.../excluded_channels.yaml`
**Depends on:** TASK-REQ-016-02; CR-REQ-015 (RPC `ListBacklog`)
**Status:** `[x] DONE`

---

## Context

- CR-REQ-015 mục 2.2: một RPC `ListBacklog(ListBacklogRequest{view, project_id, request_types, request_id, plan_task_id, phase_task_id, assignee_id, page_size, page_token, categories}) returns ListBacklogResponse{request_rows, groups, next_page_token}`; `view` bắt buộc (`REQUEST_BACKLOG_INVALID_VIEW`); `page_size` mặc định 20, tối đa 100, đếm theo Request.
- CR-016 đã nói "RPC của CR-REQ-015 (tên chốt ở đó)"; ở đây chốt: ba kênh, một hàm.
- CONTRACT mục 2.4.

## Việc cần làm

1. `func listBacklog(view requestv1.BacklogView, r *Registry, name string, client ...)`: đăng ký ba kênh bằng vòng lặp qua bảng `{name, view, fields}`.
2. Tham số theo kênh (CONTRACT 2.4): `requests` nhận `projectId`, `requestTypes[]`, `categories[]`; `tasks` nhận `projectId`, `requestId`, `planTaskId`, `assigneeId`; `execute` nhận `projectId`, `requestId`, `phaseTaskId`, `assigneeId`. Tham số không thuộc kênh bị bỏ qua (không chuyển sang RPC).
3. Kết quả: `requests` trả `{requestRows, nextPageToken}`; `tasks` và `execute` trả `{groups, nextPageToken}`. Mảng rỗng là `[]`.
4. `pageSize` kẹp 1 đến 100 ở gateway; `pageToken` chuyển nguyên văn (opaque).
5. Timeout 8s. Lỗi qua `requestChannelError` (`REQUEST_BACKLOG_BAD_PAGE_TOKEN`, `REQUEST_BACKLOG_TASK_SERVICE_UNAVAILABLE` giữ nguyên).
6. `excluded_channels.yaml`: ba dòng tạm; task 017-01 thay bằng `ToolSpec`.

## Kiểm thử

- Fake: mỗi kênh gửi đúng `view` (assert enum), bỏ tham số không thuộc kênh, phân trang chuyển `nextPageToken`.
- Kết quả rỗng thành `[]` (JSON không có `null`).
- `go test ./internal/adapter/wscompat/... -run Backlog`.

## Tiêu chí hoàn thành

- [x] Ba kênh có test; parity xanh.
- [x] Không kênh nào nhận `tenantId`.

## Rủi ro và lưu ý

- Hình dạng `BacklogGroup`, `BacklogTaskRow` có thể đổi khi CR-REQ-015 hiện thực; sửa view, ghi vào CONTRACT mục 1 và báo frontend (CR-REQ-023 đang dùng tên `backlog.list`, đã chốt bỏ).
- Không tính view ở gateway; backend là nơi duy nhất định nghĩa ba view (README v6 mục 3.8).
