# TASK-REQ-011-02: Domain `task_type`, `RequestID`, cột `request_id` trong repository, không cấp số cho container

**From Solution:** BE-REQ-SOL-011
**Priority:** P0
**Service:** `task-service`
**File:** `internal/domain/task_type.go` (mới), `internal/domain/task.go`, `internal/adapter/postgres/repository.go`, `internal/adapter/mysql/repository.go`, `proto/orca/task/v1/task.proto`, `internal/adapter/grpc/server.go` (`toProtoTask`)
**Depends on:** TASK-REQ-011-01
**Status:** `[x] DONE`

---

## Context

- `domain/task.go` dòng 92: `Type string // task|bug|feature|epic`; chưa có `RequestID`. Lỗi domain đặt ở khối `var (...)` quanh dòng 53 đến 65.
- `adapter/postgres/repository.go`: `taskColumns` (dòng 98 đến 107) liệt kê 30 cột và `scanTask` quét đúng thứ tự đó; `Create` (dòng ~170) có `nextval('task.task_number_seq')` cố định ở INSERT. `Update` (dòng 425) **không** ghi `request_id` (đúng: bất biến).
- `adapter/mysql/repository.go`: `Create` dòng 184 đến ~235 gọi `INSERT INTO task_number_seq VALUES (NULL)` rồi `LastInsertId`; bảng cột ở dòng 83 và 306 (alias).
- `scanTask` Postgres đã `COALESCE(task_number, 0)`, nên NULL an toàn; MySQL xem comment `mysql/0008_task_outbox_and_number.up.sql`.
- Proto: `Task` kết thúc ở `share_token = 30`, nên `request_id = 31`.
- Chạy `gitnexus_impact` trên `taskColumns`, `scanTask`, `Repository.Create` trước khi sửa (quy ước repo); `scanTask` được dùng bởi `Get`, `List`, `GetAncestors`, subtree, velocity.

## Việc cần làm

1. `domain/task_type.go`: hằng `TypeTask`...`TypePhase`; `ParseTaskType(s string) (string, error)` (rỗng thành `task`, lạ thành `ErrInvalidTaskType`); `IsContainerType(s string) bool`.
2. `domain/task.go`: thêm `RequestID string` (comment: id Request ở `request-service`, không FK, bất biến); sửa comment `Type`; thêm 5 lỗi domain (`ErrInvalidTaskType`, `ErrPlanCannotHaveParent`, `ErrPhaseRequiresPlanParent`, `ErrContainerUnderWorkTask`, `ErrContainerStatusDerived`). Không đổi chữ ký `NewTask`.
3. Proto: `string request_id = 31;` vào `Task`; cập nhật comment `task_type` ghi sáu giá trị; `make proto-gen` rồi `make proto-lint`.
4. Postgres: thêm `COALESCE(request_id::text, '')` vào cuối `taskColumns` và thêm đích quét tương ứng ở `scanTask` (cả bản alias dòng ~157). `Create`: thêm cột `request_id` (`nullableUUID(task.RequestID)`); nếu `domain.IsContainerType(task.Type)` thì INSERT không có cột `task_number`, không `nextval`, và `RETURNING` bỏ `task_number` (giữ `TaskNumber = 0`).
5. MySQL: tương tự `taskColumns` (dòng 83 và 306), `scanTask`, `Create` (`request_id` CHAR(36) NULL qua `nullableString`; container không `INSERT INTO task_number_seq`, không `LastInsertId`).
6. `toProtoTask`: gán `RequestId: t.RequestID`; `task_number` giữ như cũ (0 với container).
7. `FindByNumber` giữ nguyên: container không có số nên không bao giờ khớp.

## Kiểm thử

- Unit `domain/task_type_test.go`: `TestParseTaskType` (rỗng, sáu giá trị, lạ), `TestIsContainerType`.
- Integration cả hai dialect, thêm vào `repository_test.go`: `TestRepository_Create_ContainerHasNoTaskNumber`, `TestRepository_Create_PersistsRequestID`, `TestRepository_Create_ConcurrentTaskNumbersUnique` (20 goroutine, không trùng số, container không tiêu số: số của task thường liên tiếp không nhảy do container).
- `cd /opt/repos/orca/backend-go && go test ./services/task-service/internal/domain/... && go test -tags=integration ./services/task-service/internal/adapter/...`.

## Tiêu chí hoàn thành

- [x] `Task.RequestID` ghi và đọc đúng qua `Get`, `List`, `GetAncestors` ở cả hai DB.
- [x] Plan/phase `task_number = 0` (cột NULL); task thường vẫn nhận số tăng dần, 20 tạo đồng thời không trùng.
- [x] `Update` không đổi `request_id` (test chứng minh).
- [x] `make proto-lint` xanh, không phá số trường cũ.
- [x] Mọi test cũ của `postgres`, `mysql`, `usecase`, `grpc` vẫn xanh.

## Rủi ro và lưu ý

- Thứ tự cột trong `taskColumns` và `scanTask` phải khớp tuyệt đối; sai thứ tự gây lỗi quét ở mọi truy vấn. Thêm cột ở **cuối**.
- Hai dialect có hai danh sách cột (có alias): sửa cả hai chỗ mỗi bên.
- `gitnexus` có thể cũ so với repo (chỉ mục 247k symbol): xác nhận lại bằng grep `scanTask` trước khi sửa.
