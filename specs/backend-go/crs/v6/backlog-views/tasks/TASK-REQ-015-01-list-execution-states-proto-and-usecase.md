# TASK-REQ-015-01: `task-service` RPC `ListExecutionStates`: proto, port và use case

**From Solution:** BE-REQ-SOL-015
**Priority:** P1
**Service:** `task-service`
**File:** `backend-go/proto/orca/task/v1/task.proto`, `internal/usecase/list_execution_states.go` (mới), `internal/usecase/ports.go`, `internal/adapter/grpc/server_execution_states.go` (mới), `internal/usecase/list_execution_states_test.go` (mới), `internal/usecase/fakes_test.go`
**Depends on:** không (độc lập SOL-011; chỉ đọc dữ liệu có sẵn)
**Status:** [x] DONE (đã kiểm chứng 2026-10-08: `go test ./... && go test -tags integration ./internal/adapter/{postgres,mysql,eventbus}` trong `request-service`; task-service `go test ./internal/usecase ./internal/adapter/grpc`)

---

## Context

- Không RPC nào hiện trả `execution_links`; `GetDependencies` chỉ cho một task (`usecase/get_dependencies.go`, proto dòng 425 đến 430). `ListTasks` (`usecase/list_tasks.go`) chỉ kiểm tenant.
- Dữ liệu nguồn: `execution_links (task_id, engine, status_mirror, started_at, completed_at)` với `idx_execution_links_task (task_id, started_at DESC)` (`migrations/*/0010_execution_links.up.sql`); `task_edges` `edge_type='depends_on'`, `task_edges_from_idx (tenant_id, from_task_id, edge_type)`.
- `status_mirror` có `in_progress`, `completed`, `failed`; link Engine 2/3 bị đánh dấu `completed` ngay lúc dispatch (CR-TG-008) nên `last_link_status` chỉ đáng tin để phát hiện `failed`.
- Quy ước repo: mỗi use case một file, cổng ở `ports.go`; handler gRPC ở file riêng như `server_task_source.go` (không phình `server.go` 661 dòng).
- Cổng `ExecutionStateReader` hiện thực ở hai adapter ở task 02.

## Việc cần làm

1. Proto: thêm `rpc ListExecutionStates` và message `ListExecutionStatesRequest{task_ids=1}`, `ExecutionState{task_id=1, blocked_by_task_ids=2, last_engine=3, last_link_status=4, last_started_at=5, last_completed_at=6, failed_attempts=7}`, `ListExecutionStatesResponse{states=1}` (`make proto-gen && make proto-lint`).
2. `ports.go`:
   ```go
   type ExecutionStateReader interface {
       ListExecutionStates(ctx context.Context, tenantID string, taskIDs []string) ([]domain.ExecutionState, error)
   }
   ```
   và kiểu `domain.ExecutionState` (file mới `domain/execution_state.go`: `TaskID`, `BlockedByTaskIDs []string`, `LastEngine`, `LastLinkStatus string`, `LastStartedAt`, `LastCompletedAt *time.Time`, `FailedAttempts int`).
3. `list_execution_states.go`: `ListExecutionStates.Execute(ctx, in ListExecutionStatesInput{TaskIDs []string}) ([]domain.ExecutionState, error)`: `RequireTenantID`; loại trùng và rỗng; `len > 500` thì `TASK_STATES_TOO_MANY_IDS` (InvalidArgument); rỗng thì trả rỗng; gọi cổng; đảm bảo mỗi `task_id` hợp lệ trả một phần tử (task không link có `LastLinkStatus` rỗng, `FailedAttempts` 0), thứ tự theo `task_ids` đầu vào.
4. Mã lỗi nội bộ: `TASK_STATES_FAILED` (Internal).
5. `server_execution_states.go`: ánh xạ domain sang proto; quyền: chỉ kiểm tenant như `ListTasks` (ghi rõ trong comment rằng `request-service` phải lọc người xem trước).
6. Đăng ký use case ở `cmd/server/main.go` (dựng `NewListExecutionStates(repo)` bằng `Repository` đã thoả cổng ở task 02).

## Kiểm thử

- `TestListExecutionStates_TooManyIDs`, `_DedupesAndKeepsOrder`, `_EmptyInput`, `_FillsMissingTasks`, `_NoTenant_Unauthenticated`.
- `server_test.go`: `TestServer_ListExecutionStates_MapsFields`.
- `cd /opt/repos/orca/backend-go && go test ./services/task-service/internal/usecase/... ./services/task-service/internal/adapter/grpc/... -run ExecutionStates -v`.

## Tiêu chí hoàn thành

- [x] Proto biên dịch, `buf lint` xanh, `buf breaking` không phá.
- [x] Hơn 500 id bị `TASK_STATES_TOO_MANY_IDS`.
- [x] Use case trả phần tử cho mọi id hợp lệ, kể cả task chưa từng chạy.
- [x] Không file nào tên `helpers`, `utils`, `common`, `misc`.

## Rủi ro và lưu ý

- Quyền: RPC không lọc theo grant người gọi; gọi trực tiếp từ client sẽ lộ trạng thái chạy của task người khác trong cùng tenant. Chỉ `request-service` gọi, gateway không định tuyến (CR-REQ-016 cần liệt kê loại trừ).
- Trần 500 id là đề xuất của CR, chưa đo.
- Giữ `last_link_status` là chuỗi gốc của `status_mirror`, không ánh xạ lại.

## Ghi chú triển khai

Phần task-service đã có từ đợt trước (agent task-a); kiểm lại ngày 2026-10-08.
