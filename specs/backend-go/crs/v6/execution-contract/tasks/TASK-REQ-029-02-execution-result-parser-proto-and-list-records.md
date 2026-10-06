# TASK-REQ-029-02: Bộ phân tích `ExecutionResult`, proto `result_nonce` và RPC `ListExecutionRecords`

**From Solution:** [BE-REQ-SOL-029](../solutions/BE-REQ-SOL-029-execution-contract-and-readiness-gate.md) mục 2.E
**Priority:** P0
**Service/Area:** `task-service` / domain thuần, usecase, proto, adapter grpc
**File:** `internal/domain/execution_result.go` (mới), `internal/domain/failure_class.go` (mới nếu TASK-REQ-029-01 chưa tạo), `internal/usecase/list_execution_records.go` (mới), `internal/adapter/grpc/server_execution_record.go` (mới), `internal/adapter/grpc/server.go` (sửa), `backend-go/proto/orca/task/v1/task.proto` (sửa), `cmd/server/main.go` (sửa wiring), và các `_test.go`
**Depends on:** TASK-REQ-029-01 (`ExecutionRecord`, repository)
**Status:** [ ] TODO

---

## Context

Đã đọc ngày 2026-10-06:
- `task.proto` dòng 317 đến 327: `TaskServiceExecuteRequest{task_id=1, request_id=2, prompt=3}`. Trường kế tiếp là `4`. `buf breaking` (FILE) cho phép thêm trường và RPC.
- `Server` của `task-service` (`internal/adapter/grpc/server.go`) nhận use case qua constructor (mẫu `server_task_source.go` cho tệp handler riêng); `cmd/server/main.go:254` dựng `simpleExecutor`, `:326` dựng `executeTaskUC`.
- Agent CR-REQ-033 mục 2.5: khối `ORCA_RESULT_BEGIN <nonce>` ... `ORCA_RESULT_END <nonce>`; agent mới trả `parsed: {ok:true,value}` hoặc `{ok:false,code,detail}` với `code` thuộc `RESULT_BLOCK_MISSING|RESULT_BLOCK_INVALID_JSON|RESULT_BLOCK_TOO_LARGE|RESULT_BLOCK_NOT_OBJECT`; agent **không** kiểm schema. Agent cũ (protocol 1) và bundle build từ `desktop/` không có `parsed`, nên `task-service` phải tự tìm khối từ `stdout`. Thuật toán: cặp BEGIN/END **cuối cùng** có đúng `nonce`; phần giữa phải là một đối tượng JSON, tối đa 256 KiB.
- Schema nội dung do CR-REQ-029 mục 2.5 định nghĩa: `{"schema_version":1,"status":"done|blocked|failed|needs_info","summary":"...","files_changed":[...],"checks_run":[{"id":"c1","exit":0}],"outputs":{...},"questions":[...],"notes":"..."}`; giới hạn `summary` ≤ 2 KB, mỗi danh sách ≤ 200, `outputs` ≤ 16 KB.
- `domain` của `task-service` là Go thuần (stdlib), không import `common`: giữ nguyên quy ước (`domain/task.go`, `domain/progress.go`).

## Việc cần làm

1. `execution_result.go`, kiểu:
   ```go
   type ResultStatus string // ResultDone|ResultBlocked|ResultFailed|ResultNeedsInfo
   type ExecutionResult struct {
       SchemaVersion int; Status ResultStatus; Summary string; FilesChanged []string
       ChecksRun []CheckRun; Outputs map[string]json.RawMessage; Questions []string; Notes string
   }
   type CheckRun struct{ ID string; Exit int }
   type AgentParsed struct{ OK bool; Value json.RawMessage; Code, Detail string } // bản sao phía domain, không import adapter
   type ParsedExecution struct {
       Status ParseStatus; Code string; Result *ExecutionResult; Raw []byte // Raw: JSON đã kiểm, để lưu vào cột result
   }
   func ParseExecutionResult(stdout, nonce string, parsed *AgentParsed) ParsedExecution
   func FindResultBlock(stdout, nonce string) (body []byte, code string)
   func (r ExecutionResult) Validate() error
   ```
2. `FindResultBlock`: duyệt từ cuối:
   - tìm `ORCA_RESULT_END <nonce>` cuối cùng rồi `ORCA_RESULT_BEGIN <nonce>` đứng trước nó gần nhất
   - chỉ khớp khi dòng chứa **đúng** chuỗi `BEGIN␣<nonce>` (không tiền tố/hậu tố khác)
   - `nonce` rỗng hoặc không khớp `^[A-Za-z0-9]{16,64}$` thì trả `RESULT_BLOCK_MISSING` (không tìm)
   - phần giữa quá 256 KiB trả `RESULT_TOO_LARGE`
   - chuỗi BEGIN không có nonce đúng (chèn từ nội dung Request) bị bỏ qua.
3. `ParseExecutionResult`: nếu `parsed != nil` thì: `OK=false` ánh xạ `Code` sang `ParseStatusMissing` (với `RESULT_BLOCK_MISSING`) hoặc `ParseStatusInvalid` (các mã còn lại):
   - `OK=true` dùng `Value`. Nếu `parsed == nil` thì `FindResultBlock(stdout, nonce)`. Sau đó `json.Unmarshal` vào `ExecutionResult` (từ chối khoá lạ ở cấp gốc bằng `Decoder.DisallowUnknownFields`), rồi `Validate`: `schema_version == 1`
   - `status` thuộc tập
   - `summary` ≤ 2048 byte
   - `len(files_changed)`, `len(checks_run)`, `len(questions)` ≤ 200
   - tổng `outputs` ≤ 16384 byte
   - `checks_run[].id` không rỗng
   - `status in (needs_info, blocked)` thì `questions` có ít nhất một phần tử khi `needs_info`. Lỗi schema trả `ParseStatusInvalid` mã `RESULT_SCHEMA_INVALID`.
4. Hàm thuần `OutputsByName(r ExecutionResult, declared map[string]string) (map[string]json.RawMessage, []string)`: chỉ giữ output có tên khai báo trong TaskSpec (`declared` là tên → kiểu), trả danh sách tên khai mà thiếu:
   - kiểu `file_list` phải là mảng chuỗi, `number` là số, `json` là bất kỳ JSON hợp lệ, `text` là chuỗi, `api_schema` là đối tượng. Dùng ở `ListExecutionRecords`? Không: chỉ cung cấp cho `request-service` dùng (task 05, 06)
   - đặt ở đây vì cùng schema. (Nếu muốn tránh hai bản, `request-service` có bản sao riêng theo quy tắc không import chéo; golden chung ở task 08.)
5. Proto `task.proto`: thêm `string result_nonce = 4;` vào `TaskServiceExecuteRequest` kèm comment ngắn (lý do: nonce do `request-service` sinh mỗi lần thử để chống chèn khối kết quả). Thêm:
   ```proto
   message ExecutionRecord { string id=1; string task_id=2; string execution_link_id=3; int32 attempt=4; string spec_digest=5;
     string packet_digest=6; string template_version=7; string parse_status=8; string failure_class=9;
     string result_json=10; string changes_json=11; string stdout_tail=12; google.protobuf.Timestamp created_at=13; }
   message ListExecutionRecordsRequest { repeated string task_ids=1; bool latest_only=2; int32 limit=3; }
   message ListExecutionRecordsResponse { repeated ExecutionRecord records=1; }
   rpc ListExecutionRecords(ListExecutionRecordsRequest) returns (ListExecutionRecordsResponse);
   ```
   Chạy `make proto` (hoặc lệnh `buf generate` của repo) và commit mã sinh nếu repo commit `proto/gen`.
6. `usecase/list_execution_records.go`: `ListExecutionRecords.Execute(ctx, in) ([]domain.ExecutionRecord, error)`: `tenant.RequireTenantID`:
   - kiểm `len(task_ids)` trong `[1, 200]` (`TASK_EXECUTION_RECORD_BAD_REQUEST`, `InvalidArgument`)
   - chỉ trả bản ghi của task mà người gọi có quyền đọc **hoặc** là service nội bộ: dùng `common/internalcaller` nếu `ReportTaskExecutionResult` đã dùng (kiểm trong `server.go`), nếu không thì kiểm `ResolvePermission(read)` cho từng task (tối đa 200 lần: chấp nhận, gom bằng một truy vấn nếu `ResolvePermission` có bản hàng loạt; ghi lại quyết định vào PR).
7. Handler `server_execution_record.go`: ánh xạ domain sang proto (`result_json = string(rec.Result)`); `failure_class` rỗng khi NULL.
8. Wiring: tạo `usecase.NewListExecutionRecords(repo, resolvePermissionUC)` trong `main.go`, truyền vào `grpc.New(...)`/struct `Server`; sửa mọi nơi gọi constructor (kể cả `server_test.go`).

## Kiểm thử

- `TestFindResultBlock_Table`: nonce đúng, nonce sai, nhiều khối (lấy khối cuối), BEGIN không có END, END trước BEGIN, nonce rỗng, chuỗi BEGIN giả không nonce trong nội dung Request, khối lồng nhau, Unicode tiếng Việt, khối đúng 256 KiB và 256 KiB + 1.
- `TestParseExecutionResult_UsesAgentParsedWhenPresent`, `_FallsBackToStdoutWhenNoParsed`, `_AgentCodeMissingMapsToMissing`, `_InvalidJSON`, `_SchemaInvalid` (từng giới hạn), `_UnknownTopLevelKeyRejected`, `_NeedsInfoWithoutQuestionsInvalid`.
- `TestOutputsByName_TypeChecks`.
- `TestListExecutionRecords_BadRequest`, `_TenantIsolated`, `_LatestOnly`, `_PermissionDenied` (fake `ResolvePermission`).
- Handler: `TestServer_ListExecutionRecords_MapsFields` (kiểu `server_test.go`).
- Hợp đồng: `cd /opt/repos/orca/backend-go/proto && buf lint && buf breaking --against '../../.git#branch=main'` chạy trực tiếp (không `make proto-lint`: `|| true` nuốt lỗi).
- Lệnh: `cd /opt/repos/orca/backend-go && go test ./services/task-service/internal/domain/... ./services/task-service/internal/usecase/... ./services/task-service/internal/adapter/grpc/...`.

## Tiêu chí hoàn thành

- [ ] Khối có nonce sai hoặc không có nonce không bao giờ được chấp nhận; chuỗi BEGIN giả trong nội dung bị bỏ qua.
- [ ] Mọi giới hạn kích thước của CR mục 2.5 có test biên.
- [ ] `buf lint` và `buf breaking` xanh với `result_nonce` và RPC mới.
- [ ] Không import `common` hay adapter trong `internal/domain`.
- [ ] Hàm phân tích không panic với `stdout` rỗng, nhị phân, hoặc 50 MB (chạy bằng `testing.B`/fuzz ngắn `FuzzFindResultBlock` ít nhất 30 giây cục bộ).

## Rủi ro và lưu ý

- `DisallowUnknownFields` ở cấp gốc có thể làm agent thêm khoá mới (phiên bản sau) bị từ chối; chấp nhận cho v1 (nghiêm ngặt), tăng `schema_version` khi mở rộng.
- Agent cũ in toàn bộ stdout (có thể rất lớn); tìm khối từ cuối chuỗi để không quét 50 MB hai lần, và cắt `stdout` ở bước gọi (task 03) trước khi truyền vào.
- Phân quyền theo từng task trong `ListExecutionRecords` là điểm tốn kém; nếu `request-service` luôn là người gọi duy nhất, cân nhắc chỉ cho phép người gọi nội bộ (quyết định ghi trong PR, chưa chốt ở solution).
