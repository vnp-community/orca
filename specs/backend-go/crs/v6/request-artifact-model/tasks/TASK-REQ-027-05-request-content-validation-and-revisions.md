# TASK-REQ-027-05: Nội dung Request (AC, trường theo loại), `ValidateRequestContent` và `AppendRequestRevision`

**From Solution:** BE-REQ-SOL-027
**Priority:** P0
**Service:** `request-service`
**File:** `internal/domain/acceptance_criteria.go`, `internal/domain/request_content.go`, `internal/domain/request_content_validation.go`, `internal/usecase/append_request_revision.go`, `internal/usecase/edit_request_content.go`, `internal/usecase/request_content_write_guard_test.go`, `internal/usecase/ports.go` (sửa), `internal/adapter/{postgres,mysql}/request_revision_repository.go`, `internal/usecase/create_request.go` (sửa, của SOL-004) và test
**Depends on:** TASK-REQ-027-01 (cột, bảng), TASK-REQ-027-03 (`CanonicalJSON`, `Violation`), TASK-REQ-003-03 (`TransitionRequest`, mẫu CAS và `InTx`), TASK-REQ-004-04 (`CreateRequest`)
**Status:** [ ] TODO

---

## Context

CR-REQ-027 mục 2.4. `requests.body` đóng vai "phát biểu vấn đề"; thêm `acceptance_criteria` (mảng `{id:"AC-1", text(1..500), status:"active|retired", verify_hint:"test|metric|manual|review"}`) và `type_fields` (đối tượng; chứa bộ đếm `ac_next` ở gốc). Hai mức kiểm: `draft` (lúc tạo: chỉ kiểu và kích thước, vì Jira/GitHub không có AC) và `ready` (Definition of Ready, do CR-REQ-028 gọi). `required_fields_by_type` là **một nguồn** cho CR-REQ-028.

Mẫu để theo: `TransitionRequest` (SOL-003 mục D) cho CAS `repo.Update(ctx, r, r.Version)` + outbox cùng transaction, và `status_write_guard_test.go` (SOL-003 mục F, dùng `go/parser`) cho chốt chặn ghi: ở đây chốt chặn cho cột nội dung. `CreateRequest.CreateWithinTx` (SOL-004) là lõi dùng lại bởi `SpawnChildRequest`; nó phải ghi `request_revisions` revision 1 trong cùng transaction.

Bẫy: AC không được tái dùng số khi bị gỡ (đánh dấu `retired`); Solution tham chiếu `REQ-142@r1` vẫn phải đọc được AC cũ, nên `request_revisions.snapshot` giữ AC của từng revision.

## Việc cần làm

1. `acceptance_criteria.go`: `type AcceptanceCriterion struct{ID, Text, Status, VerifyHint string}`;
   - `type AcceptanceCriteria struct{Items []AcceptanceCriterion; Next int}`
   - `(a *AcceptanceCriteria) Add(text, hint string) (AcceptanceCriterion, error)` (id `AC-<Next>`, `Next++`, text 1..500 sau chuẩn hoá NFC và cắt khoảng trắng)
   - `Retire(id string) error` (đổi `status=retired`, **không** xoá, không đổi `Next`)
   - `ActiveIDs() []string`
   - `Find(id string) (AcceptanceCriterion, bool)`.
   - Tối đa 50 AC (đề xuất, `REQUEST_ARTIFACT_LIMIT_EXCEEDED`).
2. `request_content.go`: `RequestContent{Title, Body string; Type RequestType; AcceptanceCriteria AcceptanceCriteria; TypeFields map[string]any}`;
   - `ContentFromRequest(r Request) (RequestContent, error)`
   - `(c RequestContent) Snapshot() ([]byte, error)` (JSON chuẩn tắc, kiểm theo schema `request`)
   - `(c RequestContent) Digest() (string, error)`.
3. `request_content_validation.go`: bảng `requiredFieldsByType` đúng CR 2.4 (bug: `repro_steps[]`, `actual`, `expected`, `environment`, `severity` thuộc `low|medium|high|critical`; change_request và refactor: `goal`, `value`, `scope_in[]`, `scope_out[]`; security: `affected_components[]`, `exploitability` thuộc `low|medium|high`, `data_exposed`; performance: `metric`, `current_value`, `target_value`, `unit`; ops_request: `target_environment`, `window`, `rollback_plan`; hotfix: `production_impact`, `started_at`; spike: `question`, `time_box_hours`; question: `question`; docs: `audience`, `scope`; task: không có).
   - `RequiredFields(t RequestType) []FieldRule` (xuất ra cho CR-REQ-028 `ReadinessPolicy`)
   - `ValidateRequestContent(t RequestType, c RequestContent, level ValidationLevel) []Violation`.
   - `draft`: kiểu, kích thước, AC hợp lệ
   - `ready`: thêm `body` ≥ 20 ký tự không phải khoảng trắng, ≥ 1 AC `active`, mọi khoá bắt buộc không rỗng (mảng ≥ 1 phần tử, chuỗi sau cắt khoảng trắng không rỗng, enum đúng tập).
   - `Violation.Path` dạng `/type_fields/repro_steps`.
4. `ports.go`: `RequestRevisionRepository{Append(ctx, rev domain.RequestRevision) error; Get(ctx, requestID string, revision int) (domain.RequestRevision, error); List(ctx, requestID string, afterRevision, limit int) ([]domain.RequestRevision, error)}` và `RequestRepository.UpdateContent(ctx, r domain.Request, expectedVersion int64) (domain.Request, error)` (CAS ghi **chỉ** `title, body, type, acceptance_criteria, type_fields, content_schema_version, content_revision, content_digest, version, updated_at`).
5. `append_request_revision.go`: `AppendInput{RequestID string; Content domain.RequestContent; Cause domain.RevisionCause; ActorID string; ActorKind domain.ActorKind; ClarificationID string; ExpectedVersion int64}`;
   - `Execute(ctx, in) (domain.Request, error)`: `RequireTenantID`
   - `Get`
   - `ExpectedVersion` lệch thì `REQUEST_VERSION_CONFLICT`
   - `ValidateRequestContent(..., draft)`
   - một `InTx`: tính `Digest`, `Append` revision `ContentRevision+1`, `UpdateContent` (CAS), `InsertOutboxEvent` `orca.request.request.revised` `{request_id, revision, cause, digest}` (không chứa nội dung)
   - thay đổi trống (digest không đổi) thì trả thành công không ghi gì (idempotent).
   - Có biến thể `AppendWithinTx(ctx, in)` cho `CreateRequest` và CR-REQ-028 gọi trong transaction của họ (không mở transaction lồng).
6. `edit_request_content.go`: `EditInput{RequestID, ActorID string; ExpectedRevision int; Patch ContentPatch}`;
   - chỉ khi `status` thuộc `{new, classifying, awaiting_type_confirmation}` hoặc `request_backlog` (chưa qua `analyzing`, chưa `awaiting_*` duyệt)
   - không thì `REQUEST_CONTENT_NOT_EDITABLE`
   - `ExpectedRevision != ContentRevision` thì `REQUEST_VERSION_CONFLICT`
   - gọi `AppendRequestRevision(cause=edited)`.
7. `create_request.go` (sửa): `CreateRequestInput` thêm `AcceptanceCriteria []AcceptanceCriterionInput` và `TypeFieldsJSON []byte` tuỳ chọn;
   - `CreateWithinTx` gọi `AppendWithinTx(cause=created)` cho revision 1 (cùng transaction, không đốt số khi lỗi).
8. `request_content_write_guard_test.go`: `go/parser` quét `internal/usecase` (trừ `*_test.go`), fail nếu có gán `.Title`, `.Body`, `.AcceptanceCriteria`, `.TypeFields`, `.ContentRevision`, `.ContentDigest` ở file ngoài `append_request_revision.go`, `create_request.go` (chỉ trong `NewRequest` của domain) và `edit_request_content.go`.
   - Hai adapter `request_revision_repository.go` (Postgres, MySQL) theo cùng hợp đồng
   - UNIQUE `(tenant_id, request_id, revision)` va chạm thì `REQUEST_VERSION_CONFLICT`.

## Kiểm thử

- `TestRequiredFields_CoversAllElevenTypes` (fail nếu `AllRequestTypes()` có loại mới thiếu dòng)
- `TestValidate_Draft_OnlyTitleAndBodyOK`
- `TestValidate_Ready_MissingKeysPerType` (bảng 11 loại: danh sách khoá thiếu đúng).
- `TestAC_Add_NeverReusesRetiredNumber`
- `TestAC_Retire_KeepsNextCounter`
- `TestAC_TextTooLong`.
- `TestAppendRequestRevision_IncrementsOnce_WritesRowAndEvent`
- `TestAppendRequestRevision_ConcurrentCalls_OneWins` (CAS)
- `TestAppendRequestRevision_NoOpWhenDigestSame`
- `TestAppendRequestRevision_StaleVersion`
- `TestAppendRequestRevision_PayloadHasNoContent`.
- `TestEditRequestContent_OnlyBeforeAnalysis`
- `TestEditRequestContent_StaleRevision`.
- `TestCreateRequest_WritesRevisionOne_SameTx` (lỗi giữa chừng không để `request_revisions` mồ côi, không đốt số).
- `TestContentWriteGuard` (kiến trúc).
- Integration hai dialect: `RequestRevisionRepository` hợp đồng; CAS đồng thời; tiếng Việt qua JSON; snapshot đọc lại đúng AC cũ sau khi AC bị `retired`.
- Lệnh: `cd /opt/repos/orca/backend-go && go test ./services/request-service/internal/domain/... ./services/request-service/internal/usecase/... -run 'AcceptanceCriteria|RequestContent|RequiredFields|AppendRequestRevision|EditRequestContent|ContentWriteGuard|CreateRequest' && go test -tags=integration ./services/request-service/internal/adapter/... -run RequestRevision`.

## Tiêu chí hoàn thành

- [ ] `ready` trả đúng danh sách khoá thiếu cho cả 11 loại; `draft` chấp nhận Request chỉ có `title`, `body`.
- [ ] `AC-n` không tái dùng; revision cũ vẫn đọc được AC cũ.
- [ ] Hai `AppendRequestRevision` đồng thời: đúng một thắng, một sự kiện `revised`.
- [ ] Chỉ ba file được gán cột nội dung (test kiến trúc xanh).
- [ ] `CreateRequest` tạo revision 1 trong cùng transaction.
- [ ] Payload sự kiện không chứa nội dung Request.

## Rủi ro và lưu ý

- `UpdateContent` và `Update` (SOL-002) cùng dùng `version`; `TransitionRequest` đọc `r.Version` rồi `Update` có thể thua CAS khi một `AppendRequestRevision` chen vào; retry của `TransitionRequest` đã xử lý (SOL-003 C1), nhưng CR-REQ-028 `AnswerClarification` gọi cả hai trong một transaction nên phải dùng cùng bản `Request` đã đọc.
- Bảng trường bắt buộc là mã Go tĩnh; đổi cho một loại là đổi hợp đồng với CR-REQ-028 và UI form (CR-REQ-019).
- Quyền ghi mức Request cho `EditRequestContent` chưa chốt (README mục 8): tạm thời reporter hoặc admin, ghi chú trong mã.
- `type` có thể đổi qua `ChangeRequestType` (SOL-005), nhưng cột `type` không nằm trong phạm vi `UpdateContent` trừ `cause=type_changed`: thống nhất với người giữ SOL-005 khi nối (task 027-07).
