# TASK-REQ-014-02: RPC `RecordRequestCheck` và `ListRequestChecks`

**From Solution:** BE-REQ-SOL-014
**Priority:** P1
**Service:** `request-service`
**File:** `backend-go/proto/orca/request/v1/request_check.proto` (mới), `internal/usecase/record_request_check.go` (mới), `internal/domain/request_check_metrics.go` (mới), `internal/adapter/grpc/server_request_check.go` (mới), `internal/usecase/record_request_check_test.go`, `internal/domain/request_check_metrics_test.go` (mới)
**Depends on:** TASK-REQ-014-01, CR-REQ-003 (trạng thái Request), CR-REQ-010 (quyền)
**Status:** `[x] DONE`

---

## Context

- README v6 mục 8 điều 12 thêm `RecordRequestCheck`, `ListRequestChecks` vào `RequestService`. Tool MCP `request_record_check` thuộc CR-REQ-017; ở đây chỉ làm RPC.
- Người gọi: user (`source=manual`) hoặc agent qua MCP (`source=agent`, `recorded_by` NULL). Gateway **không** kiểm OPA trước định tuyến (README v6 mục 8 điều 13), nên RPC tự kiểm quyền: chủ Request hoặc người có quyền ghi theo CR-REQ-010 (chưa chốt, dùng cổng `RequestAuthorizer`).
- `RecordRequestCheck` chỉ nhận khi Request ở trạng thái hợp lệ cho `kind` (bảng dưới). Sai thì `REQUEST_CHECK_NOT_ALLOWED_NOW`.
- Metrics sai định dạng thì `REQUEST_CHECK_INVALID_METRICS` (InvalidArgument).

## Việc cần làm

1. Proto: 
   ```proto
   rpc RecordRequestCheck(RecordRequestCheckRequest) returns (RecordRequestCheckResponse);
   rpc ListRequestChecks(ListRequestChecksRequest) returns (ListRequestChecksResponse);
   message RecordRequestCheckRequest { string request_id = 1; string kind = 2; string status = 3; string metrics_json = 4; string summary = 5; string task_id = 6; }
   message RequestCheck { string id = 1; string request_id = 2; string kind = 3; string status = 4; string metrics_json = 5; string summary = 6; string source = 7; string task_id = 8; string recorded_by = 9; google.protobuf.Timestamp created_at = 10; }
   ```
2. `request_check_metrics.go`: `ValidateMetrics(kind CheckKind, raw json.RawMessage) error` với schema theo kind: `perf_baseline` (`metrics[]` mỗi phần tử có `name`, `unit`, `direction` ∈ {`lower_is_better`,`higher_is_better`}, `baseline` số, `target_change_percent` số; `method` chuỗi), `perf_after` (`metrics[]` có `name`, `value`), `tests_before` (`total`, `passed`, `failed` nguyên không âm, `command` chuỗi), `tests_after` (thêm `tests_modified` bool), `security_recheck` (bất kỳ object, cần `summary` khác rỗng), `ops_result` (cần `summary` khác rỗng). Trần kích thước `metrics_json` 64 KB.
3. Trạng thái hợp lệ theo kind: `perf_baseline` ở `analyzing`, `awaiting_analysis_approval`, `planning`; `perf_after`, `tests_before`, `tests_after`, `security_recheck`, `ops_result` ở `executing` (các trạng thái khác bị chặn). Đặt bảng này là hằng trong domain, một nguồn.
4. `record_request_check.go`: `RecordRequestCheck.Execute`: `RequireTenantID`, quyền, đọc Request, kiểm trạng thái và loại (`kind` phải thuộc loại Request: ví dụ `perf_*` chỉ cho `performance`; trả `REQUEST_CHECK_NOT_ALLOWED_NOW`), `ValidateMetrics`, `Append`; không sinh sự kiện (chỉ ghi). Trả bản ghi mới.
5. `ListRequestChecks`: trả toàn bộ theo thứ tự thời gian, cộng cờ `effective` cho bản mới nhất mỗi `kind` (tính ở domain).
6. `server_request_check.go`: `recorded_by` từ `tenant.UserID(ctx)`; `source` = `agent` khi lời gọi mang scope của MCP/agent (cơ chế chưa chốt: dùng trường `source` do gateway điền, kiểm lại ở CR-REQ-016/017), mặc định `manual`.

## Kiểm thử

- `TestValidateMetrics_PerKind` (table-driven hợp lệ và không), `_RejectsOversized`.
- `TestRecordRequestCheck_StateNotAllowed`, `_KindNotForType`, `_Persists_WithRecordedBy`, `_AgentSourceNoUser`, `_Forbidden`.
- `TestListRequestChecks_MarksEffective`.
- `cd /opt/repos/orca/backend-go && go test ./services/request-service/... -run 'RequestCheck|ValidateMetrics' -v`.

## Tiêu chí hoàn thành

- [x] `RecordRequestCheck` từ chối khi Request ở trạng thái hoặc loại không phù hợp.
- [x] Bản ghi mới nhất theo `(request_id, kind)` được đánh dấu có hiệu lực; không có đường sửa/xoá.
- [x] Metrics sai định dạng bị `REQUEST_CHECK_INVALID_METRICS`.
- [x] `make proto-lint` xanh.

## Rủi ro và lưu ý

- Quyền ghi ở mức Request chưa chốt (CR-REQ-003/010); không bịa quy tắc, dùng cổng và test với fake.
- Nhận diện `source=agent` phụ thuộc gateway/MCP chưa dựng; ban đầu mọi lời gọi là `manual`.
- Agent có thể ghi số đo sai hoặc bị lừa: không có cách xác minh, chỉ ghi rõ ở tài liệu người dùng.
