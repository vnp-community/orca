# TASK-REQ-005-01: Domain: đề xuất phân loại, luật xác nhận, đường đổi loại, migration `classification_attempts`

**From Solution:** BE-REQ-SOL-005
**Priority:** P0
**Service:** `request-service`
**File:** `internal/domain/request_classification_proposal.go`, `internal/domain/request_confirmation_rules.go`, `internal/domain/request_type_change_paths.go`, `internal/domain/request_classification_errors.go` và `*_test.go`; `migrations/postgres/0004_request_classification_attempts.{up,down}.sql`, `migrations/mysql/0004_request_classification_attempts.{up,down}.sql` (mới); `internal/domain/request.go`, repository hai dialect (sửa thêm cột)
**Depends on:** TASK-REQ-003-01, TASK-REQ-004-01
**Status:** [ ] TODO

---

## Context

CR-REQ-005 mục 2.2 đến 2.4. `FlowFor(type).PhaseRule` có từ TASK-REQ-003-01. **Số migration:** `ls services/request-service/migrations/postgres`; `0004` đúng nếu `0003_request_source_hints` đã có và chưa ai chiếm `0004`. Cột `classification_attempts` là điểm lệch có chủ ý so với CR (SOL-005 C2, Q1): nếu người duyệt từ chối, bỏ bước 1 và bước 2 (cột) và đếm bằng `request_type_history` như CR.

## Việc cần làm

1. Migration: Postgres `ALTER TABLE request.requests ADD COLUMN classification_attempts INT NOT NULL DEFAULT 0 CHECK (classification_attempts >= 0);` MySQL `ALTER TABLE requests ADD COLUMN classification_attempts INT NOT NULL DEFAULT 0;`; down `DROP COLUMN`. `Request.ClassificationAttempts int`; hai repository ghi/đọc cột (`Create`, `Update`, scan); `ExpectedColumns()` của `contracttest` cập nhật.
2. `request_classification_proposal.go`: `ClassificationProposal{Type RequestType; Size RequestSize; Urgency Urgency; Confidence float64; Reason string}`; `ParseClassificationProposal(raw []byte) (ClassificationProposal, error)`: tìm đối tượng JSON đầu tiên trong chuỗi (agent có thể bọc trong markdown), `json.Decoder` với `DisallowUnknownFields`, kiểm enum, `confidence` trong [0,1] (không NaN), `reason` tối đa 2000 rune; lỗi là `ErrProposalInvalid` (kiểu lỗi trần của domain, có `Reason string` để log, không có mã gRPC).
3. `request_confirmation_rules.go`: `ValidateConfirmation(in ConfirmationInput) error` với `ConfirmationInput{Type RequestType; Size RequestSize; Urgency Urgency; Reason string}`: `type` rỗng thì `REQUEST_TYPE_REQUIRED`; `FlowFor(type).PhaseRule == when_size_L` mà `size` rỗng thì `REQUEST_SIZE_REQUIRED`; `hotfix` mà `urgency != urgent` thì `REQUEST_HOTFIX_REQUIRES_URGENT`; `hotfix`, `security` mà `reason` rỗng sau trim thì `REQUEST_REASON_REQUIRED`.
4. `request_type_change_paths.go`: `ChangeTypeAllowed(from, to RequestType) error`: cùng loại `REQUEST_TYPE_UNCHANGED`; `bug|task|refactor|performance` sang `change_request` ok; `docs` sang `task|change_request` ok; `security` sang `hotfix` ok; `from` thuộc `spike|question|hotfix` trả `REQUEST_TYPE_CHANGE_USE_CHILD`; còn lại `REQUEST_TYPE_CHANGE_NOT_ALLOWED`.
5. `request_classification_errors.go`: constructor cho `REQUEST_NOT_CLASSIFIABLE`, `REQUEST_CLASSIFICATION_LIMIT`, `REQUEST_TYPE_REQUIRED`, `REQUEST_SIZE_REQUIRED`, `REQUEST_HOTFIX_REQUIRES_URGENT`, `REQUEST_TYPE_UNCHANGED`, `REQUEST_TYPE_CHANGE_NOT_ALLOWED`, `REQUEST_TYPE_CHANGE_USE_CHILD`, `REQUEST_TYPE_CHANGE_BLOCKED_ACTIVE_EXECUTION` (kind theo SOL-005 mục G).
6. Hằng `MaxClassificationAttempts = 5`.

## Kiểm thử

- `TestParseClassificationProposal_Valid`, `_MarkdownWrapped`, `_MissingField`, `_UnknownField`, `_EnumOutOfSet` (`hotfix2`), `_ConfidenceOutOfRange` (-0.1, 1.1), `_ReasonTooLong`, `_NotJSON`.
- `TestValidateConfirmation` (bảng: `bug` thiếu size, `refactor` thiếu size, `hotfix` + `normal`, `security` không reason, `task` không cần size).
- `TestChangeTypeAllowed` (ma trận 11 nhân 11 cho các đường ở trên; mọi ô còn lại `NOT_ALLOWED`).
- `TestMigration_0004_UpDownUp` hai dialect, `TestClassificationAttempts_RoundTrip`.
- Lệnh: `go test ./services/request-service/internal/domain/...`; `go test -tags=integration ./services/request-service/internal/adapter/... -run "Migration|Attempts"`.

## Tiêu chí hoàn thành

- [ ] Ma trận đường đổi loại đúng bảng README mục 3.4 và CR mục 2.4.
- [ ] Proposal chặt: mọi trường lạ hoặc ngoài tập bị loại.
- [ ] Migration up/down/up sạch hai dialect.
- [ ] Số migration xác nhận bằng `ls`.

## Rủi ro và lưu ý

- `ParseClassificationProposal` tìm đối tượng JSON trong chuỗi tự do: dùng bộ tách dấu ngoặc cân bằng, không regex tham lam (đầu ra có thể chứa `}` trong chuỗi).
- Cột mới đẩy migration của SOL-006 thành `0005`.
