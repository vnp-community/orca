# TASK-REQ-012-02: Handler gRPC `CreatePlanTree`, bắt lỗi duy nhất hai dialect, integration test

**From Solution:** BE-REQ-SOL-012
**Priority:** P0
**Service:** `task-service`
**File:** `internal/adapter/grpc/server_plan_tree.go` (mới), `internal/adapter/grpc/server.go` (đăng ký dependency), `internal/adapter/postgres/unique_violation.go` (mới), `internal/adapter/mysql/unique_violation.go` (mới), `internal/adapter/postgres/create_plan_tree_integration_test.go` (mới), `internal/adapter/mysql/create_plan_tree_integration_test.go` (mới), `cmd/server/main.go`
**Depends on:** TASK-REQ-012-01
**Status:** `[x] DONE`

---

## Context

- `server.go` (661 dòng) giữ các use case dạng field và `txRunner` (xem `AddEdge` handler dòng 162 gọi `s.txRunner.RunInTx`). Thêm handler vào file riêng như `server_task_source.go` để không phình `server.go`; không thêm `max-lines` disable.
- Postgres dùng `pgx` (lỗi `*pgconn.PgError` mã `23505`), MySQL dùng `database/sql` với `go-sql-driver/mysql` (lỗi `*mysql.MySQLError` số `1062`). Cả hai adapter hiện bọc lỗi bằng `fmt.Errorf("postgres: ... %w")` nên `errors.As` hoạt động.
- Mẫu integration: `adapter/postgres/add_edge_integration_test.go`, `repository_test.go` (tag `integration`, testcontainers), `adapter/mysql/repository_test.go`.
- `toProtoTask` đã có; trả `request_id` từ SOL-011 task 02.

## Việc cần làm

1. `postgres/unique_violation.go`: `func IsUniqueViolation(err error) bool` (`errors.As(err, &pgErr) && pgErr.Code == "23505"`). `mysql/unique_violation.go`: cùng tên, kiểm `Number == 1062`. Trong `Repository.Create` của mỗi adapter, khi lỗi là vi phạm chỉ mục `uq_tasks_active_plan_per_request` (kiểm tên ràng buộc ở Postgres `pgErr.ConstraintName`, MySQL chuỗi `Message`) thì trả `usecase.ErrActivePlanExists` bọc lỗi gốc.
2. `server_plan_tree.go`: `func (s *Server) CreatePlanTree(ctx, req) (*taskv1.CreatePlanTreeResponse, error)` chuyển proto sang `CreatePlanTreeInput`, gọi use case, ánh xạ kết quả bằng `toProtoTask`; lỗi qua `apperrors.ToGRPCStatus`.
3. Dependency: thêm field `createPlanTree *usecase.CreatePlanTree` vào `Server` và vào constructor (xem cách các use case khác được truyền ở `cmd/server/main.go`); dựng `NewCreatePlanTree(txRunner, repo, grantRepo)`.
4. Quyền: RPC này chỉ gọi nội bộ từ `request-service` (CR-REQ-012); `ResolvePermission` không áp. Ghi rõ trong comment rằng gateway không được định tuyến RPC này (CR-REQ-016) và liệt kê vào danh sách loại trừ nếu `parity_test.go` của MCP yêu cầu.
5. Integration hai dialect: dữ liệu thật (Plan, 3 Phase, 12 task, `depends_on` giữa Phase và task).
6. Thêm test tải nhẹ: 8 goroutine gọi `CreatePlanTree` cùng `request_id`, đúng một Plan hoạt động còn lại, các lần còn lại `AlreadyExists=true`, không deadlock MySQL.

## Kiểm thử

- `TestCreatePlanTree_Integration_FullTree` (cả hai DB): đủ số task, cạnh, `request_id` kế thừa, `task_number=0` cho Plan/Phase, `blocked` đúng chỗ.
- `TestCreatePlanTree_Integration_RollbackLeavesNothing` (chèn task có `parent_id` không tồn tại ở vị trí N).
- `TestCreatePlanTree_Integration_ConcurrentSameRequest_OnePlan` (8 goroutine).
- `TestCreatePlanTree_Integration_SupersedeThenRecreate` (chỉ mục duy nhất cho tạo lại sau `cancelled`).
- `TestServer_CreatePlanTree_MapsErrors` (unit, fake use case): `TASK_PLAN_HAS_RUNNING_TASKS` thành FailedPrecondition.
- `cd /opt/repos/orca/backend-go && go test -tags=integration ./services/task-service/internal/adapter/... -run CreatePlanTree -v` (cần Docker; chưa chạy).

## Tiêu chí hoàn thành

- [x] RPC `CreatePlanTree` gọi được qua gRPC với dữ liệu mẫu và trả đủ `plan`, `phases`, `tasks`.
- [x] Hai dialect cùng kết quả; đua 8 goroutine chỉ một Plan hoạt động.
- [x] Rollback không để lại task mồ côi.
- [x] Task thuộc cây có quyền đọc và `execute` cho `creator_id` qua `ResolvePermission` (test grant kế thừa ba cấp).
- [x] `server.go` không phình thêm quá vài dòng đăng ký.

## Rủi ro và lưu ý

- Nếu MySQL deadlock do `ListByKindForUpdate`, giảm bằng cách thêm cạnh theo lô nhỏ hoặc truy vấn khoá theo tập task của cây thay vì toàn bộ cạnh theo loại; ghi kết quả đo vào PR (chưa đo).
- Nhận diện lỗi theo tên chỉ mục phụ thuộc thông điệp driver; giữ test cho từng dialect để phát hiện đổi phiên bản driver.
- RPC có thể bị gọi lặp bởi retry: idempotency dựa vào `request_id` và chỉ mục duy nhất, không dựa vào khoá phía client.
