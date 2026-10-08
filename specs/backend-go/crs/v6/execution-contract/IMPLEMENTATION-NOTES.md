# Ghi chú triển khai: execution-contract

## Phần task-service của CR-REQ-029 (TASK-REQ-029-01, 02, 03), 2026-10-08

Kiểm chứng: `go build/vet/test ./...` của `task-service` xanh (kèm test mới cho domain, usecase, grpcclient, grpc); fuzz `FuzzFindResultBlock` 30 giây và `FuzzCanonicalJSON` 15 giây không lỗi; integration Postgres 16 và MySQL 8.0 thật cho migration `0021`, ghi/liệt kê/`latest_only`/cascade/CHECK/cách ly tenant/Unicode; RLS kiểm bằng role không phải superuser; `buf breaking` so với `main` xanh (`buf lint` chỉ còn cảnh báo `Empty` có từ trước).

### Đã làm
- Migration `0021_task_execution_records` (hai dialect, RLS `FORCE`), `domain.ExecutionRecord`, `TailUTF8`, `StorableJSON`, `TaskExecutionRecordRepository`.
- `domain.ParseExecutionResult`/`FindResultBlock`/`OutputsByName`/`ClassifyRunFailure`. Chỉ nhận khối có đúng nonce (16 đến 64 ký tự chữ số), marker phải chiếm trọn một dòng, lấy cặp cuối, tối đa 256 KiB; từ chối khoá lạ ở cấp gốc (phần tử `checks_run` thì khoan dung).
- Proto: `result_nonce = 4` ở `TaskServiceExecuteRequest`; RPC `ListExecutionRecords`; message `ExecutionRecord`.
- `ExecuteWithContract` ở `grpcclient.SimpleExecutor` (file `simple_executor_contract.go`); `Execute` giữ nguyên hành vi: thân được tách thành `runAgentPrompt` dùng chung. `trustPreset:"full"` chỉ khi `request_id` có tiền tố `req:`, `prompt` bắt buộc.
- `ExecuteTask.WithContract`: task có `request_id` và có spec chạy Engine 1 (trước kiểm `depends_on`), có `result_nonce` thì đi đường hợp đồng; thất bại hoàn tác trạng thái, thành công vào `review` chỉ khi `parse_status=ok` và `status=done`.
- Sự kiện `orca.task.task.statuschanged` cho chạy hợp đồng có thêm `failure_class` và `execution_record_id` (`omitempty`).
- Người gọi đã kiểm: `selectEngine` và `dispatchDirectAgentAsync` chỉ gọi từ `ExecuteTask.Execute`; `ExecuteTaskInput` dựng ở `adapter/grpc/server.go` và test; `SimpleExecutor.Execute` gọi qua interface `usecase.SimpleExecutor` (không đổi chữ ký) và test; `NewSimpleExecutor`/`NewExecuteTask`/`NewUpdateTask` giữ chữ ký, thêm builder `With*`.

### Quyết định lệch so với task
1. Đã hợp nhất (2026-10-08): đường hợp đồng dùng cổng sự kiện cùng giao dịch của CR-REQ-013 (`runEventsWithOutcome` + `CompleteExecution`/`ReleaseExecution`); struct `contractRunEventPayload`, `WithContractEvents` và ghi outbox độc lập đã xoá. `failure_class` và `execution_record_id` nằm trong `taskStatusChangedPayload` (omitempty). Test: `TestExecuteTask_ContractFailure_RevertsAndEmitsFailureClass`, `TestExecuteTask_ContractSuccess_GoesToReview`, `TestExecuteTask_PlainPath_EmitsExactlyOneStatusChanged` đều khẳng định đúng một `statuschanged` mỗi lần chạy.
2. Lỗi trước khi gọi agent (không có kết nối, thiếu worktree) không tạo bản ghi và sự kiện không có `failure_class`; chỉ lần chạy đã tới agent mới có bản ghi.
3. `SpecDigest`, `PacketDigest`, `TemplateVersion` để trống (phương án (b) của task); request-service nối theo `(task_id, attempt)`.
4. Quyền `ListExecutionRecords`: có `user` thì cần `read` từng task; lời gọi giữa service (chỉ tenant) không kiểm từng task, như các RPC nội bộ khác. Câu hỏi mở.
5. Lỗi relay không tạm thời là `env_defect`; hết giờ và relay `Unavailable`/`DeadlineExceeded` là `retryable`. `exit` thiếu với kết quả `done` tính là `agent_defect` (`EXIT_MISSING`).
6. Bản ghi `result`/`changes` có `\u0000` hoặc không phải JSON hợp lệ bị bỏ khỏi cột (JSONB không nhận), `stdout_tail` bỏ NUL.

### Chưa kiểm chứng / còn mở
- `agent.execPrompt` giao thức 2 (`resultBlock`, `reportChanges`, `parsed`, `changes`) trên dev server thật: test dùng relay giả.
- Chính sách dọn `stdout_tail` (đề xuất theo CR-REQ-035).
- `gitnexus_impact` không phản ánh worktree (chỉ mục của cây chính); thay bằng kiểm người gọi bằng grep.
- Proto sinh lại đã nằm trong cây (`proto/gen/go/orca/task/v1`); điều phối sinh lại sau khi hợp nhất với task.proto của agent task-a.
