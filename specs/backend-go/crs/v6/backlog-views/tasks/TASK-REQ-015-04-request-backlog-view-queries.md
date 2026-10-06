# TASK-REQ-015-04: View REQUEST: truy vấn `requests` keyset hai dialect, `request_links`, `request_return_history`

**From Solution:** BE-REQ-SOL-015
**Priority:** P1
**Service:** `request-service`
**File:** `backend-go/services/request-service/internal/usecase/ports.go`, `internal/adapter/postgres/backlog_requests.go` (mới), `internal/adapter/mysql/backlog_requests.go` (mới), `internal/usecase/list_backlog_requests.go` (mới), `internal/adapter/postgres/backlog_requests_test.go`, `internal/adapter/mysql/backlog_requests_test.go` (mới, tag `integration`)
**Depends on:** TASK-REQ-015-03 (page token), CR-REQ-002 (`requests`, `request_links`), CR-REQ-006 (`returned_category`, `request_return_history`)
**Status:** `[ ] TODO`

---

## Context

- Chỉ mục kỳ vọng `(tenant_id, status, updated_at DESC)` trên `requests` (CR-REQ-002); `request_return_history` có chỉ mục `(tenant_id, request_id, at)` (CR-REQ-006). Kiểm chỉ mục thật ở migration đã merge; nếu thiếu thì tạo migration bổ sung (số theo quy tắc "lớn nhất + 1").
- MySQL tối ưu kém so sánh hàng `(a,b) < (?,?)`; dùng dạng khai triển `updated_at < ? OR (updated_at = ? AND id < ?)` cho **cả hai** dialect để cùng kết quả.
- Không dùng `updated_at` làm thời điểm trả: lấy `at` lớn nhất của `request_return_history` với `action='returned'`.
- View này không gọi `task-service`.
- Tên cột trả về (`returned_from_stage`, `returned_category`, `return_reason`) theo CR-REQ-006.

## Việc cần làm

1. Cổng:
   ```go
   type BacklogRequestReader interface {
       ListReturnedRequests(ctx context.Context, tenantID string, f BacklogRequestFilter) ([]domain.Request, error)   // trả page_size+1 hàng
       ParentRequestIDs(ctx context.Context, tenantID string, childIDs []string) (map[string][]string, error)
       LatestReturns(ctx context.Context, tenantID string, requestIDs []string) (map[string]domain.ReturnEvent, error)
   }
   ```
   `BacklogRequestFilter{ProjectID string; Types, Categories []string; Cursor *Cursor; Limit int}`.
2. Adapter (mỗi dialect):
   - `ListReturnedRequests`: `WHERE tenant_id = ? AND status = 'request_backlog'` cộng lọc tuỳ chọn (`project_id`, `type IN`, `returned_category IN`) và điều kiện keyset khai triển; `ORDER BY updated_at DESC, id DESC LIMIT n+1`.
   - `ParentRequestIDs`: `SELECT child_request_id, parent_request_id FROM request_links WHERE tenant_id = ? AND child_request_id IN (...)`.
   - `LatestReturns`: `SELECT request_id, actor_id, at FROM request_return_history WHERE tenant_id = ? AND action = 'returned' AND request_id IN (...)`; chọn dòng `at` lớn nhất mỗi Request trong bộ nhớ.
3. `list_backlog_requests.go`: `ListBacklogRequests.Execute(ctx, in) (rows []domain.BacklogRequestRow, next string, err error)`: kẹp `page_size` mặc định 20, tối đa 100; giải mã `page_token`; lấy `n+1` để biết còn trang; lắp `BacklogRequestRow` (`returned_by`, `returned_at`, `parent_request_ids`); mã hoá token từ Request cuối.
4. Mã lỗi: `REQUEST_BACKLOG_BAD_PAGE_TOKEN` (InvalidArgument).
5. Không lọc quyền ở đây; cổng lọc người xem là task 06.

## Kiểm thử

Integration hai dialect, 1.000 Request với `updated_at` trùng nhau ở nhiều hàng:
- `TestListReturnedRequests_Keyset_NoDuplicatesNoGaps` (đi hết 1.000 hàng bằng token, tổng đúng, không trùng).
- `_InsertDuringPaging_StableOrder` (chèn hàng mới có `updated_at` mới nhất giữa hai trang: không làm trùng/sót hàng đã thấy).
- `_FilterByTypeAndCategory`, `_OnlyBacklogStatus`, `_TenantIsolation`.
- `TestParentRequestIDs_BatchesOneQuery`, `TestLatestReturns_UsesHistoryNotUpdatedAt`.
- Unit: `TestListBacklogRequests_PageSizeClamp`, `_BadToken`, `_NextTokenOnlyWhenMore`.
- `cd /opt/repos/orca/backend-go && go test ./services/request-service/... -run 'Backlog' && go test -tags=integration ./services/request-service/internal/adapter/... -run 'ReturnedRequests|ParentRequest|LatestReturns' -v` (chưa chạy).

## Tiêu chí hoàn thành

- [ ] View REQUEST chỉ trả Request `request_backlog`, đúng `updated_at DESC, id DESC`, phân trang ổn định khi có hàng mới chen vào.
- [ ] `parent_request_ids` đúng với `request_links`; `returned_at` lấy từ lịch sử trả.
- [ ] Hai dialect cùng kết quả cho cùng dữ liệu.
- [ ] Không gọi `task-service`.

## Rủi ro và lưu ý

- Chưa đo hiệu năng với số Request lớn; nếu `EXPLAIN` cho thấy không dùng chỉ mục, thêm chỉ mục qua migration mới.
- `IN (...)` tối đa 100 phần tử (kích thước trang), an toàn cho MySQL.
- CR-REQ-006 chưa merge thì cột `returned_category` chưa có: task chặn, đừng tự thêm cột.
