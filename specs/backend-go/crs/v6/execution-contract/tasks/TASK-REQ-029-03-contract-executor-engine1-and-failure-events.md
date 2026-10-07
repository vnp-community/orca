# TASK-REQ-029-03: `ExecuteWithContract` (nonce, `resultBlock`, `reportChanges`), Engine 1 cho task có spec, ghi bản ghi và `failure_class` vào sự kiện

**From Solution:** [BE-REQ-SOL-029](../solutions/BE-REQ-SOL-029-execution-contract-and-readiness-gate.md) mục 2.E
**Priority:** P0
**Service/Area:** `task-service` / usecase `ExecuteTask`, adapter `grpcclient.SimpleExecutor`, grpc server, wiring
**File:** `internal/usecase/contract_executor.go` (mới), `internal/usecase/execute_task.go` (sửa), `internal/usecase/task_run_events.go` (sửa, tạo ở TASK-REQ-013-01/02), `internal/adapter/grpcclient/simple_executor.go` (sửa), `internal/adapter/grpcclient/simple_executor_contract.go` (mới), `internal/adapter/grpc/server.go` (sửa dòng 280), `cmd/server/main.go` (sửa), và các `_test.go`
**Depends on:** TASK-REQ-029-01, TASK-REQ-029-02, TASK-REQ-011-02 (`domain.Task.RequestID`), TASK-REQ-013-01/02 (cổng sự kiện `statuschanged`), SOL-033 mục 2.I (hợp đồng tham số `agent.execPrompt`)
**Status:** [x] DONE

---

## Context

Đã đọc ngày 2026-10-06:
- `ExecuteTaskInput` (`execute_task.go:13`) có `TaskID`, `RequestID`, `Prompt`; chỉ một nơi dựng: `adapter/grpc/server.go:280` (`s.executeTask.Execute(ctx, usecase.ExecuteTaskInput{...})`).
- Từ chối `Prompt != ""` ngoài Engine 1 ở `execute_task.go:179 đến 182`. `selectEngine` ở dòng 399 đến 424.
- `dispatchDirectAgentAsync` (dòng 363 đến 395): gọi `uc.simple.Execute(dispatchCtx, tenantID, in.TaskID, in.RequestID, worktreePath, in.Prompt)`; lỗi thì `links.Complete(failed)` + `repo.UpdateStatus(previousStatus)` + log; thành công `CompleteExecution(..., StatusReview, actualHours)` rồi `links.Complete(completed)`. CR-REQ-013 (TASK-REQ-013-01, 013-02) đổi các nhánh này sang cổng có sự kiện `statuschanged` với `cause = execution_completed|execution_failed` và `error_message`; task này thêm `failure_class` và `execution_record_id` (`omitempty`) vào cùng payload.
- `SimpleExecutor` (`simple_executor.go`): `Execute` đọc task, dựng prompt bằng `buildExecutePrompt` (dòng 497) **khi `prompt` rỗng** (kiểm cách xử lý chuỗi `prompt` ở dòng khoảng 300 đến 340 trước khi sửa), gọi `agent.execPrompt` qua `RelayByDevServer`/`Relay` (dòng 456 đến 467), giải `agentExecPromptResult{Stdout,Stderr,ExitCode *int,TimedOut}`; `TimedOut` thành `TASK_EXECUTE_TIMED_OUT`, exit khác 0 thành `TASK_EXECUTE_FAILED`; thành công gọi `UpdateLastExecutionOutput` (cắt 8 KB). `trustPreset:"full"` cố định ở dòng 369.
- Interface `SimpleExecutor` ở `usecase/ports.go:259` có hai nơi cài: `adapter/grpcclient.SimpleExecutor` và các fake test. **Không đổi chữ ký** để không phá fake; thêm cổng mới.
- Tham số agent mới (CR-REQ-033 mục 2.1): `resultBlock:{nonce}`, `reportChanges:true`, `maxOutputBytes`; kết quả mới `changes`, `parsed`, `truncated`, `warnings`.
- `domain.Task.RequestID` do TASK-REQ-011-02 thêm (cột `request_id`, migration `0016`). Bảng `task_specs` do CR-REQ-027: cần cổng đọc "task có spec chưa"; nếu migration của CR-REQ-027 chưa có thì port rỗng (nil) và nhánh Engine 1 tắt.

## Việc cần làm

1. `usecase/contract_executor.go`:
   ```go
   type ContractExecuteInput struct {
       TenantID, TaskID, RequestID, WorktreePath, Prompt, ResultNonce string
       ExecutionLinkID string; Attempt int
       SpecDigest, PacketDigest, TemplateVersion string // do request-service gửi qua metadata (xem bước 6), có thể rỗng
   }
   type ContractExecuteOutput struct {
       ExecutionRef string; Record domain.ExecutionRecord
   }
   type ContractAgentExecutor interface {
       ExecuteWithContract(ctx context.Context, in ContractExecuteInput) (ContractExecuteOutput, error)
   }
   // ExecutionFailure là lỗi có kiểu: failure_class và record để dispatch ghi vào sự kiện.
   type ExecutionFailure struct{ Class domain.FailureClass; Code string; RecordID string; Cause error }
   func (e *ExecutionFailure) Error() string; func (e *ExecutionFailure) Unwrap() error
   ```
2. `ExecuteTaskInput` thêm `ResultNonce string`, `Attempt int`; `adapter/grpc/server.go:280` truyền `req.GetResultNonce()`; `Attempt` suy ra từ phần cuối của `request_id` dạng `req:<request_id>:<task_id>:<attempt>` (SOL-013 mục 2.4); parse lỗi thì `Attempt=1`. Chỉ khi `ResultNonce != ""` thì `ExecuteTask` dùng đường hợp đồng.
3. `ExecuteTask` thêm trường tuỳ chọn `contract ContractAgentExecutor` và `specs TaskSpecLookup` (`HasSpec(ctx, tenantID, taskID string) (bool, error)`); phương thức `WithContract(c ContractAgentExecutor, specs TaskSpecLookup) *ExecuteTask` (builder, như cách `NewExecuteTask(...).` ở `main.go:326` đang nối thêm cấu hình). Nil thì hành vi cũ.
4. `selectEngine`: sau kiểm `WorkflowTemplateID` và kiểm con (`parent_child`), **trước** kiểm `depends_on`, thêm: `if uc.specs != nil && task.RequestID != "" { has, err := uc.specs.HasSpec(...); if err != nil { return "", err }; if has { return domain.EngineDirectAgent, nil } }`. Task có con vẫn Engine 2 (container không chạy, SOL-011). Cập nhật comment đầu hàm nêu lý do (spec task cần prompt ghi đè, chỉ Engine 1 hỗ trợ; thứ tự phụ thuộc do `AdvanceExecution` bảo đảm).
5. `dispatchDirectAgentAsync`: nếu `in.ResultNonce != "" && uc.contract != nil` thì gọi `ExecuteWithContract` thay `Execute`:
   - lỗi là `*ExecutionFailure` thì truyền `Class` và `RecordID` vào sự kiện `execution_failed` (qua tham số mới của hàm dựng payload ở `task_run_events.go`)
   - thành công và `Record.ParseStatus == ok` và `Result.Status == done` thì như cũ (`CompleteExecution(review)`) kèm `execution_record_id` trong sự kiện `execution_completed`
   - `Result.Status` là `needs_info|blocked` thì coi là thất bại lớp `needs_info` (hoàn tác trạng thái như nhánh lỗi, **không** để task ở `review`).
6. `adapter/grpcclient/simple_executor_contract.go` cài `ExecuteWithContract` trên `*SimpleExecutor` (cùng struct, file riêng để không đẩy `simple_executor.go` quá `max-lines`): tái dùng phần dựng kết nối/relay của `Execute` bằng cách **tách** hàm `relayExecPrompt(ctx, tenantID, params agentExecPromptParams, devServerID, connectionID string) (agentExecPromptResult, error)` khỏi `Execute` (sửa `Execute` gọi hàm này; hành vi y nguyên, test hiện có xanh). Với hợp đồng: `Prompt` = `in.Prompt` (bắt buộc không rỗng, `TASK_EXECUTE_PROMPT_REQUIRED` nếu rỗng):
   - `params.ResultBlock = &resultBlockParam{Nonce}`, `ReportChanges=true`, `MaxOutputBytes = 1 MiB`
   - `TrustPreset` vẫn `"full"` (CR-REQ-029 mục 2.6b: chỉ vì cổng `ready` và `VerifyExecution` bắt buộc, kiểm `request_id` có tiền tố `req:`; request_id không có tiền tố thì lỗi `TASK_EXECUTE_CONTRACT_REQUIRES_REQUEST`).
7. Sau khi có kết quả: `parsed := domain.ParseExecutionResult(result.Stdout, in.ResultNonce, agentParsed)`:
   - dựng `ExecutionRecord` (`StdoutTail = domain.TailUTF8(result.Stdout, 16*1024)`, `Changes = result.Changes` thô, `Result = parsed.Raw`)
   - phân loại ban đầu bằng hàm thuần `domain.ClassifyRunFailure(timedOut bool, exit *int, p domain.ParsedExecution, relayErr error) (FailureClass, string)` theo bảng của solution (hết giờ hoặc lỗi relay `Unavailable|DeadlineExceeded` thì `retryable`; không có khối hoặc khối sai thì `agent_defect`; `status=needs_info|blocked` thì `needs_info`; `status=failed` thì `agent_defect`; exit khác 0 mà có khối `done` thì `agent_defect` kèm code `EXIT_NONZERO_WITH_DONE`)
   - `InsertExecutionRecord` (lỗi ghi thì log và vẫn trả kết quả, **không** nuốt thất bại của run). Trả `*ExecutionFailure` khi không thành công, ngược lại `ContractExecuteOutput`.
8. `UpdateLastExecutionOutput` **không** gọi trên đường hợp đồng (bản ghi mới thay thế); đường cũ giữ nguyên.
9. `domain.ClassifyRunFailure` đặt trong `internal/domain/failure_class.go` (thuần, có bảng test).
10. `main.go`: `executeTaskUC := usecase.NewExecuteTask(...).WithContract(simpleExecutor, repo)` với `repo` cài `TaskSpecLookup` (truy vấn `SELECT 1 FROM task.task_specs WHERE tenant_id=$1 AND task_id=$2`, hai dialect; nếu bảng chưa tồn tại ở môi trường chưa migrate thì lỗi `undefined_table` được coi là `false` kèm log một lần, không làm hỏng `Execute` của task thường).

## Kiểm thử

- `TestSelectEngine_SpecTaskWithDependsOn_IsDirectAgent`, `_NoSpecWithDependsOn_StaysOrchestration`, `_ContainerWithChildren_StaysOrchestration`, `_WorkflowTemplateWins`, `_NilSpecLookup_OldBehavior`, `_LookupErrorFailsClosed`.
- `TestExecuteTask_PromptOverrideAllowedForSpecTaskWithDeps` (không còn `TASK_EXECUTE_PROMPT_UNSUPPORTED`).
- `TestClassifyRunFailure_Table` (hết giờ, lỗi relay, thiếu khối, khối sai, `needs_info`, `blocked`, `failed`, exit khác 0 kèm `done`).
- `TestExecuteWithContract_SendsResultBlockAndReportChanges` (relay giả: kiểm `params_json` có `resultBlock.nonce`, `reportChanges:true`, `trustPreset:"full"` chỉ khi `request_id` bắt đầu `req:`), `_PersistsRecordWithTail16KB`, `_ParsedFromAgent`, `_FallbackScansStdoutForOldAgent`, `_MissingBlockIsAgentDefect`, `_NeedsInfoNotReview`, `_InsertFailureDoesNotMaskRunFailure`.
- Hồi quy: toàn bộ `simple_executor_test.go` và `execute_task_test.go` hiện có phải xanh sau khi tách `relayExecPrompt`; golden `buildExecutePrompt` cho task không spec không đổi.
- Sự kiện: `TestStatusChangedPayload_IncludesFailureClassAndRecordID` và `_OmitsWhenEmpty` (`omitempty`).
- Lệnh: `cd /opt/repos/orca/backend-go && go test ./services/task-service/internal/usecase/... ./services/task-service/internal/adapter/grpcclient/... ./services/task-service/internal/adapter/grpc/...` rồi bản `-tags=integration`.

## Tiêu chí hoàn thành

- [x] Task thuộc Request có spec và có `depends_on` chạy Engine 1, nhận đúng `prompt` và `result_nonce`.
- [x] Task không spec: golden `buildExecutePrompt` và hành vi `UpdateLastExecutionOutput` không đổi.
- [x] Mỗi lần chạy hợp đồng có đúng một `task_execution_records` (kể cả thất bại).
- [x] Thiếu hoặc sai khối kết quả: `parse_status` đúng, `failure_class=agent_defect`, task không ở `review`.
- [x] `trustPreset` không bao giờ `"full"` trên đường hợp đồng khi `request_id` không có tiền tố `req:`.
- [x] `gitnexus_impact` đã chạy cho `selectEngine`, `dispatchDirectAgentAsync`, `ExecuteTaskInput`, `SimpleExecutor.Execute` và báo cáo trong PR (quy ước repo).

## Rủi ro và lưu ý

- Tách `relayExecPrompt` chạm đường nóng của mọi task; làm thành commit riêng chỉ-refactor, chạy hồi quy trước khi thêm hành vi.
- `SpecDigest`, `PacketDigest`, `TemplateVersion` do `request-service` biết chứ `task-service` không: cần cách truyền. Đề xuất hai: (a) thêm ba trường `string` vào `TaskServiceExecuteRequest` (số 5 đến 7), hoặc (b) `request-service` ghi `execution_packets` và `task-service` để trống ba cột. Hiện chọn (b) (không đổi proto thêm); bản ghi có cột rỗng, `request-service` nối theo `(task_id, attempt)`. Ghi quyết định vào PR.
- Chưa kiểm chứng `agent.execPrompt` mới trên dev server thật; test dùng relay giả.
- Hoàn tác trạng thái khi `needs_info` dùng `previousStatus` của link (CR-REQ-013 mục 1); chưa kiểm chứng task quay về `open` có đủ để chạy lại sạch.
