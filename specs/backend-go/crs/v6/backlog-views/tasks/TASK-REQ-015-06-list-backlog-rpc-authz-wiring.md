# TASK-REQ-015-06: RPC `ListBacklog`, lọc quyền xem, mã lỗi và wiring

**From Solution:** BE-REQ-SOL-015
**Priority:** P1
**Service:** `request-service`
**File:** `backend-go/proto/orca/request/v1/request_backlog.proto` (mới), `internal/usecase/list_backlog.go` (mới), `internal/adapter/grpc/server_backlog.go` (mới), `internal/usecase/request_visibility.go` (mới), `cmd/server/main.go`, `internal/usecase/list_backlog_test.go`, `internal/adapter/grpc/server_backlog_test.go` (mới)
**Depends on:** TASK-REQ-015-04, 05
**Status:** `[x] DONE`

---

## Context

- README v6 3.6: `ListBacklog` thuộc `RequestService`; mọi RPC của `request-service` **tự kiểm quyền** vì gateway không kiểm OPA trước định tuyến (README v6 mục 8 điều 13). Kênh `backlog.*` của gateway thuộc CR-REQ-016, không làm ở đây.
- Chính sách xem Request chưa chốt (CR-REQ-003/010 để mở: "Request chưa có `Grant`"); `ListTasks` và `ListExecutionStates` không lọc quyền, nên lọc phải xảy ra trước khi gọi `task-service` (SOL-015 mục 6).
- Chỉ đọc, không ghi, không phát sự kiện, không `OpenApproval`.
- `proto` message khớp CR-REQ-015 mục 2.1 (số trường cố định để frontend CR-REQ-023 và gateway dùng cùng tên).

## Việc cần làm

1. `request_backlog.proto`: `BacklogView`, `ListBacklogRequest` (trường 1 đến 10), `ListBacklogResponse`, `BacklogRequestRow` (1 đến 13), `BacklogGroup` (1 đến 7), `BacklogTaskRow` (1 đến 10, `estimated_hours` là `DoubleValue`); thêm `rpc ListBacklog` vào `RequestService`. `make proto-gen && make proto-lint`.
2. `request_visibility.go`: cổng `RequestVisibility.Filter(ctx, actor, requests []domain.Request) ([]domain.Request, error)`; mặc định hiện thực tạm theo chính sách đang có ở CR-REQ-010 (nếu chưa có: chủ Request, người trong team của project, admin). Không bịa quy tắc: để cổng và ghi vào Câu hỏi mở; test dùng fake.
3. `list_backlog.go`: `ListBacklog.Execute`: `RequireTenantID`; kiểm `view` khác `UNSPECIFIED` (`REQUEST_BACKLOG_INVALID_VIEW`); `REQUEST` → `ListBacklogRequests`; `TASK`/`EXECUTE` → `ListBacklogTasks`; trước đó lọc danh sách Request ứng viên qua `RequestVisibility` (cả ba view). Người dùng không có quyền xem Request không thấy dòng của Request đó ở cả ba view.
4. `server_backlog.go`: ánh xạ domain → proto, mã lỗi → gRPC (`REQUEST_BACKLOG_INVALID_VIEW`, `REQUEST_BACKLOG_BAD_PAGE_TOKEN` InvalidArgument; `REQUEST_BACKLOG_TASK_SERVICE_UNAVAILABLE` Unavailable).
5. `main.go`: dựng `TaskClient`, các reader, `ListBacklog`; đăng ký RPC.
6. Danh sách loại trừ gateway/MCP: ghi chú trong PR rằng RPC này sẽ cần ánh xạ ở CR-REQ-016 (kênh `backlog.list`) và `parity_test.go` của MCP đỏ nếu thêm channel mà không có `ToolSpec` hoặc dòng loại trừ (README v6 mục 8 điều 13).

## Kiểm thử

- `TestListBacklog_InvalidView`, `_RequestView_NoTaskServiceCall`, `_TaskView_FiltersByVisibility`, `_ExecuteView_FiltersByVisibility`, `_BadPageToken`, `_TaskServiceDown_RequestViewStillWorks`.
- `server_backlog_test.go`: `TestServer_ListBacklog_MapsRowsAndGroups`, `_ErrorCodes`.
- Hợp đồng: `make proto-lint`; test proto giữ số trường; fixture cho mỗi điều kiện README 3.8.
- E2E (CR-REQ-025): Request đi qua vòng đời, task chuyển view đúng thời điểm.
- `cd /opt/repos/orca/backend-go && go test ./services/request-service/... -run 'ListBacklog' -v`.

## Tiêu chí hoàn thành

- [x] Ba view hoạt động qua gRPC với đủ bộ lọc trong `ListBacklogRequest`.
- [x] Người dùng không có quyền xem Request không thấy dòng của Request đó ở cả ba view.
- [x] `task-service` không sẵn sàng: TASK/EXECUTE trả `REQUEST_BACKLOG_TASK_SERVICE_UNAVAILABLE`, REQUEST vẫn chạy.
- [x] `buf lint`/`buf breaking` xanh; số trường khớp CR.
- [x] Không file nào tên `helpers`/`utils`/`common`/`misc`.

## Rủi ro và lưu ý

- Quy tắc "ai xem được Request" là điểm mở lớn nhất; sai ở đây làm lộ dữ liệu giữa người dùng cùng tenant. Cần chốt ở CR-REQ-010/016 trước khi bật cờ `request_flow_enabled`.
- `page_size × 100` task mỗi lần có thể chậm; chưa đo, cân nhắc giảm trần nếu cần.
- Các CR bổ sung (026 đến 036) có thể thêm view hoặc trường; chỉ thêm trường mới (additive).
