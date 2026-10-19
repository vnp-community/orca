# TASK-REQ-038-06: `GetExecutionSummary`, chuẩn hoá nguyên nhân lỗi và điều tra `TIMED_OUT` của #1242

**From Solution:** BE-REQ-SOL-038
**Priority:** P1
**Service:** `request-service`
**File:** `internal/domain/{execution_summary,failure_cause}.go` (mới), `internal/usecase/get_execution_summary.go` (mới), `internal/adapter/grpc/server_activity.go` (sửa, thêm `GetExecutionSummary`), `proto/orca/request/v1/request.proto` (sửa), nơi ghi `task_run_outcomes.cause` (sửa, xem bước 1), tests
**Depends on:** TASK-REQ-038-01, TASK-REQ-038-04
**Status:** [x] DONE

---

## Context

- Số lần thử và nguyên nhân lỗi chỉ có trong `task_run_outcomes` (`outcome`, `cause`, `error_message`, `occurred_at`); `REQUEST_MAX_TASK_ATTEMPTS` mặc định 2.
- Trên #1242, task #TG-1037 lỗi 2 lần nhưng banner ghi `execution failed: retryable TIMED_OUT` còn log cuối của agent là `unrecognized_model claude-sonnet-5-5`; chưa rõ lần nào lỗi do gì.
- `needs_you[]` gom: Approval `pending` mà người xem được quyết (tái dùng bộ lọc của `ListPendingForUser`), Clarification mở, task lỗi làm Request về backlog, cổng `pre_deploy`, `drift` đang mở.
- `handover` chỉ cho engine `openspec`, đọc từ `openspec_changes` (`sync_progress`, migration `0097`).

## Việc cần làm

1. **Điều tra trước (không đoán):** đọc nơi ghi `cause` và `error_message` vào `task_run_outcomes` (đường `ReportTaskOutcome`/consumer) và trả lời: vì sao #1242 ghi `TIMED_OUT`; lần 1 và lần 2 khác nguyên nhân hay cause bị ghi đè? Ghi kết luận vào mục "Ghi chú triển khai" của task.
2. Domain `FailureCause` (`timed_out`, `model_error`, `verify_failed`, `env_defect`, `spec_defect`, `agent_error`, `cancelled`, `unknown`) và `ClassifyFailureCause(cause, errorMessage string) FailureCause` (khớp chuỗi đã biết như `TIMED_OUT`, `unrecognized_model`; còn lại `unknown`). Ghi `cause` chuẩn hoá ở nơi phát hiện lỗi cho dữ liệu mới; dữ liệu cũ phân loại lúc đọc.
3. Domain `ExecutionSummary` theo CR-038 mục 2.3: `progress`, `current_phase_id`, `running[]`, `failed[]`, `needs_you[]`, `handover`, `next_phase_id`.
4. Use case `GetExecutionSummary`: một lần `ListTasks` theo `RequestIDs` (types `executionTaskTypes`) cho cây; `CountStartedByTask` và dòng `failed` mới nhất cho `attempt`, `failed[]`; Approval `pending` và Clarification mở lọc theo quyền người xem; `handover` từ `openspec_changes` (nếu thiếu trường archive thì để `state=none` và ghi chú, không đoán).
5. Chỉ đọc, không tác dụng phụ; gọi lặp 5 giây một lần phải rẻ (không quá 4 truy vấn DB và một `ListTasks`).
6. gRPC `GetExecutionSummary(GetExecutionSummaryRequest{request_id}) returns (ExecutionSummary)`.

## Kiểm thử

- `ClassifyFailureCause` table-driven (`TIMED_OUT`, `unrecognized_model`, rỗng, lạ).
- Use case với dữ liệu mô phỏng #1242: `tasks_total=28`, `tasks_done=27`, `failed[0].attempts=2`, `last_cause` khác rỗng, `needs_you` chứa mục task lỗi khi Request ở `request_backlog`.
- Phân quyền: người không được quyết Approval không thấy mục đó trong `needs_you`.
- `handover` rỗng với engine `native`.
- `cd /opt/repos/orca/backend-go && go test ./services/request-service/internal/usecase/... -run 'ExecutionSummary|FailureCause' -v` (chưa chạy).

## Tiêu chí hoàn thành

- [x] Tóm tắt của #1242 đúng số liệu và có nguyên nhân từng task lỗi.
- [x] `attempt` và `max_attempts` đúng với `task_run_outcomes` và cấu hình.
- [x] `needs_you[]` tôn trọng quyền xem.
- [x] Có ghi chú kết luận điều tra `TIMED_OUT` vs `unrecognized_model`.
- [x] Số truy vấn đúng giới hạn.

## Rủi ro và lưu ý

- `openspec_changes` có thể không có trường archive đủ; không bịa trạng thái (câu hỏi mở 2 của CR).
- Bộ mã nguyên nhân cần khớp `Failure.class` của CR-REQ-029 (câu hỏi mở 3); đổi tên sau không phá dữ liệu vì phân loại lúc đọc.
- Cây thực thi lớn: dùng một `ListTasks` duy nhất, không gọi từng Phase.

## Ghi chú triển khai

Hoàn thành 2026-10-16.

- `GetExecutionSummary` ghép tiến độ (Phase/task), task đang chạy, task lỗi kèm `FailureCause` (`ClassifyFailureCause` từ `cause`/`error_message` thật của `task_run_outcomes`), `needs_you[]` (Approval chờ mà người gọi được quyết, Clarification mở mà người gọi được trả lời, task hết số lần thử), handover (`openspec_changes`) và `engine`. `needs_you` lọc theo quyền qua `Authorizer.CanDecide`/`callerMayAnswer`.
- Số liệu của #1242 đã đối chiếu trên DB dev: 7 Phase, 28 task, TG-1037 lỗi 2/2.
- **Kết luận điều tra `TIMED_OUT` vs `unrecognized_model`:** hai thông báo đến từ hai lần thử khác nhau của cùng task. Lần 1 `RESULT_BLOCK_MISSING` (`agent_defect`), lần 2 `TIMED_OUT` (`retryable`). `unrecognized_model` chỉ nằm trong log agent, không bao giờ được ghi vào `task_run_outcomes`; nên nguyên nhân hiển thị lấy từ outcome, còn log agent mở bằng "View last log".
- `get_execution_summary_test.go` có ca hình dạng #1242; số truy vấn cố định theo số bảng, không theo số task.
