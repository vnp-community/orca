# TASK-REQ-011-04: `ListTasks` lọc theo type, Request, parent (`ListFilter`) cho hai dialect

**From Solution:** BE-REQ-SOL-011
**Priority:** P0
**Service:** `task-service`
**File:** `proto/orca/task/v1/task.proto`, `internal/usecase/ports.go` (dòng 67), `internal/usecase/list_tasks.go`, `internal/adapter/postgres/task_list_query.go` (mới), `internal/adapter/mysql/task_list_query.go` (mới), `internal/adapter/{postgres,mysql}/repository.go` (xoá `List` cũ), `internal/adapter/grpc/server.go` (dòng 299), `internal/usecase/fakes_test.go`
**Depends on:** TASK-REQ-011-02
**Status:** [x] DONE (đã kiểm chứng 2026-10-07: `go test ./services/task-service/...` và `go test -tags=integration ./internal/adapter/postgres ./internal/adapter/mysql` (PG 16, MySQL 8.0.46 thật))

---

## Context

- Port hiện tại: `List(ctx, tenantID, projectID, pageToken string, pageSize int32) ([]domain.Task, string, error)`; chỉ `usecase/list_tasks.go` gọi (xác nhận lại bằng `gitnexus_impact` và grep `\.List(` trong `services/task-service`, vì mock `fakes_test.go` và `adapter/grpc/server_test.go` (`fakeTaskRepository`) cũng implement).
- Postgres `List` (repository.go dòng 376): `($2 = '' OR project_id::text = $2) AND ($3 = '' OR id::text > $3) ORDER BY id LIMIT $4`, mặc định `pageSize=50`, token là id cuối khi đủ trang. Cách so `::text` làm chỉ mục không dùng được.
- `ListTasksRequest` dùng số 1 đến 3; 4 đến 6 trống. `ListTasks` không lọc theo grant người gọi, chỉ tenant (giữ nguyên, ghi vào Rủi ro).
- Mặc định mới: `task_types` rỗng trả `task,bug,feature,epic`.

## Việc cần làm

1. Proto: `repeated string task_types = 4; repeated string request_ids = 5; string parent_id = 6;` vào `ListTasksRequest`.
2. `ports.go`: `type ListFilter struct { ProjectID string; TaskTypes, RequestIDs []string; ParentID, PageToken string; PageSize int32 }`; đổi `List(ctx, tenantID string, f ListFilter) ([]domain.Task, string, error)`.
3. `usecase/list_tasks.go`: `ListTasksInput` thêm `TaskTypes`, `RequestIDs`, `ParentID`; kiểm `len(RequestIDs) <= 100` (`TASK_LIST_TOO_MANY_REQUEST_IDS`, InvalidArgument); chuẩn hoá từng `TaskTypes` bằng `domain.ParseTaskType` (rỗng là bỏ qua; lạ thì `TASK_INVALID_TYPE`); nếu rỗng gán `[task,bug,feature,epic]`.
4. `adapter/postgres/task_list_query.go`: `buildListQuery(tenantID string, f usecase.ListFilter) (sql string, args []any)` dựng `WHERE tenant_id = $1` rồi thêm điều kiện theo thứ tự `project_id = $n::uuid`, `task_type = ANY($n::text[])`, `request_id = ANY($n::uuid[])`, `parent_id = $n::uuid`, `id > $n::uuid`; `ORDER BY id LIMIT`. Bỏ `List` cũ khỏi `repository.go`, thay bằng phương thức gọi `buildListQuery`.
5. `adapter/mysql/task_list_query.go`: cùng ý, `IN (?,?,...)` theo số phần tử, tham số dạng chuỗi (CHAR(36)).
6. `server.go` `ListTasks`: chuyển `TaskTypes`, `RequestIds`, `ParentId`. Gateway ánh xạ `requestId` đơn thành mảng một phần tử là việc của CR-REQ-016, ghi chú ở PR.
7. Cập nhật mọi fake implement `TaskRepository.List`.

## Kiểm thử

- Unit: `TestListTasks_DefaultHidesPlanPhase`, `TestListTasks_TooManyRequestIDs`, `TestListTasks_InvalidType`; test thuần `buildListQuery` (bảng tổ hợp bộ lọc, kiểm thứ tự tham số) cho mỗi dialect.
- Integration hai dialect: `TestRepository_List_FilterByType`, `_FilterByRequestIDs`, `_FilterByParent`, `_PaginationStable` (30 task, page_size 7, không trùng không sót), `_TenantIsolation`.
- `cd /opt/repos/orca/backend-go && go test ./services/task-service/... && go test -tags=integration ./services/task-service/internal/adapter/...`.

## Tiêu chí hoàn thành

- [x] Không truyền `task_types` thì không trả plan/phase; `task_types=[plan]` kèm `request_ids=[X]` trả đúng Plan của X.
- [x] 101 `request_ids` bị `TASK_LIST_TOO_MANY_REQUEST_IDS`.
- [x] Hành vi gọi cũ (chỉ `project_id`, `page_token`, `page_size`) cho kết quả y như trước trên dữ liệu không có plan/phase.
- [ ] `EXPLAIN` truy vấn theo `request_ids` dùng `idx_tasks_request` (ghi nhận kế hoạch trong PR, không bắt buộc test).
  - Chưa chạy EXPLAIN (tiêu chí ghi "không bắt buộc test"); câu lệnh dùng `request_id = ANY($n::uuid[])` nên dùng được `idx_tasks_request`.
- [x] Không còn nơi nào gọi chữ ký `List` cũ.

## Rủi ro và lưu ý

- `ListTasks` không kiểm grant người gọi; `request-service` phải tự lọc Request trước khi gọi (BE-REQ-SOL-015).
- Chưa có trần `page_size`; CR-REQ-015 sẽ gọi trang lớn: để quyết định trong Câu hỏi mở Q3 của CR, không tự thêm trần ở task này.
- `IN` rất dài ở MySQL: giới hạn 100 id đã chặn.

## Ghi chú triển khai (2026-10-07)

- Port `List(ctx, tenantID, ListFilter)`; xoá `List` cũ ở hai repository; cập nhật 3 fake (`usecase`, `adapter/grpc`, `adapter/grpcclient`) và hai test integration cũ. Người gọi duy nhất: `usecase/list_tasks.go`.
- Hành vi mới đáng chú ý: so `project_id`/`parent_id`/`request_id`/token bằng kiểu uuid native (Postgres) nên chuỗi không phải UUID giờ báo lỗi DB (`TASK_LIST_FAILED`) thay vì trả danh sách rỗng như so `::text` cũ.
- Test thuần `TestBuildListQuery` mỗi dialect; integration `_FilterByType/_FilterByRequestIDs/_FilterByParent/_PaginationStable/_TenantIsolation`; unit `TestListTasks_*`; gRPC `TestServer_ListTasks_ForwardsFiltersAndHidesContainersByDefault`.
