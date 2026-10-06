# BE-REQ-SOL-028: Clarification, trạng thái `awaiting_information`, Definition of Ready và Decision

> **📋 Proposed.** Chưa triển khai, chưa chạy test nào. Phụ thuộc [BE-REQ-SOL-027](./BE-REQ-SOL-027-artifact-schema-ontology-and-task-specs.md) (`ValidateRequestContent`, `AppendRequestRevision`), [BE-REQ-SOL-003](../../request-lifecycle/solutions/BE-REQ-SOL-003-request-state-machine-and-flow-registry.md), [BE-REQ-SOL-005](../../request-lifecycle/solutions/BE-REQ-SOL-005-request-classification-and-type-change.md), [BE-REQ-SOL-007](../../solution-analysis/solutions/BE-REQ-SOL-007-solution-generation-options-and-selection.md), [BE-REQ-SOL-009](../../approval/solutions/BE-REQ-SOL-009-generic-approval-domain-and-api.md), [BE-REQ-SOL-010](../../approval/solutions/BE-REQ-SOL-010-approval-authorization-notification-expiry.md).

**CR:** [CR-REQ-028](../../../../../../docs/crs/v6/request-artifact-model/CR-REQ-028-clarification-and-decision-records.md)
**Service:** `request-service` (`internal/domain`, `internal/usecase`, `internal/adapter/{postgres,mysql,grpc,eventbus}`, `migrations/*`) · `notification-service` (hai hàng cấu hình) · `proto`
**TDD tham chiếu:** [`arch/03`](../../../../tdd/architecture/03-clean-architecture-guidelines.md), [`arch/05`](../../../../tdd/architecture/05-data-architecture.md) (hai dialect, chỉ mục duy nhất một phần), [`arch/08`](../../../../tdd/architecture/08-inter-service-communication.md) (consumer bền, giao lặp), [`services/notification-service.md`](../../../../tdd/services/notification-service.md)

---

## 1. Trạng thái hiện tại (re-verify)

Đã đọc (2026-10-06): CR-REQ-028, README v6 mục 3.3, 3.7, 8, `BE-REQ-SOL-002/003/005/007/009/010`, `TASK-REQ-003-02/-03` (16 trigger, `NextStatus`, `TransitionInput`), `TASK-REQ-005-05` (`ConfirmRequestType`), `notification-service/internal/adapter/eventbus/consumer.go` (`SubjectBinding{StreamName, Subject, Durable}`, `Subjects`), `internal/domain/notification_event.go` (`subjectRules`, `defaultRule`, `EventPayload` đọc `user_ids`), `common/tenant` (`UserID`, `Role`). `request-service` chưa có mã. Solution cho CR-REQ-006 (`ReturnToBacklog`) chưa tồn tại lúc soạn; chỉ có CR.

### Correction relative to CR-REQ-028

| # | CR nói | Solution khác hoặc mã thật | Xử lý |
|---|--------|----------------------------|-------|
| C1 | Request có 12 trạng thái | SOL-002 mục A: CHECK `status` Postgres inline (tên mặc định `requests_status_check`) với 11 giá trị; TASK-REQ-003-02 test ma trận 11 nhân 16 | Migration đổi CHECK (Postgres `DROP CONSTRAINT` + `ADD`, MySQL `DROP CHECK` + `ADD`, tên lấy từ `pg_constraint`/`information_schema`); test ma trận thành 12 nhân 18 |
| C2 | `information_provided` đến `resume_status` | `domain.NextStatus(flow, size, from, trigger)` (TASK-REQ-003-02) không có tham số đích động | Thêm `NextStatusWithResume(flow, size, from, trigger, resume RequestStatus)` bọc `NextStatus` cho hai trigger mới; `NextStatus` cũ trả `REQUEST_TRANSITION_NOT_ALLOWED` cho hai trigger đó khi thiếu `resume`, giữ nguyên chữ ký để không vỡ test 003-02 |
| C3 | `TransitionInput.ResumeStatus` | `TransitionInput` (SOL-003 mục D) chưa có | Thêm `ResumeStatus RequestStatus` và `Category ReturnCategory` (SOL-003 C5 chờ CR-REQ-006); `TransitionRequest.once` kiểm `ResumeStatus` thuộc `{analyzing, planning, executing}` (`REQUEST_RESUME_STATUS_INVALID`) |
| C4 | `return_to_backlog` ghi `category=missing_info` | Cột `returned_category` thuộc CR-REQ-006, chưa có trong SOL-002 | Task phụ thuộc TASK-REQ-006-xx; nếu cột chưa có, dùng `return_reason` cố định `clarification_expired` và ghi nợ kỹ thuật |
| C5 | `ConfirmRequestType` dùng `information_required` thay `type_confirmed` | TASK-REQ-005-05 gọi `transition.Execute(type_confirmed, ExpectedFrom=awaiting_type_confirmation)` và outbox `type_confirmed` | Sửa chỗ này: nếu `ReadinessPolicy.Evaluate` không đạt thì `information_required` và vẫn ghi `type_confirmed` (outbox) **và** `request_type` Approval `approved` (người đã xác nhận loại) |
| C6 | Hủy Approval `pending` bằng `CancelPendingForRequest` lý do `information_required` | SOL-009 mục D: chữ ký `CancelPendingForRequest(ctx, tx, tenantID, requestID, why, now)`; `OnClosedWithoutDecision(why)` của handler | Dùng nguyên chữ ký; thêm `information_required` vào tập `why` hợp lệ; handler `solution` giữ `proposed` khi `why=information_required` (Solution bị `superseded` ở `AnswerClarification`, không ở đây) |
| C7 | Chỉ mục duy nhất một phần cho Clarification `open` | Postgres hỗ trợ; MySQL không | Cột sinh `open_key` như `pending_key` của SOL-009 (đã dùng ở `approvals`); `decisions` dùng `live_key` tương tự |
| C8 | Hai hàng thông báo trong `notification-service` | TASK-REQ-010-05 (approval) sửa cùng hai tệp `consumer.go`, `notification_event.go` | Task 028-06 làm sau hoặc cùng PR với TASK-REQ-010-05; không tạo PR song song sửa cùng khối |
| C9 | `chosen_option_id` (chuỗi `opt-N`) của Decision | `solutions.chosen_option` là chỉ số nguyên 0-based (SOL-007, README mục 8 điểm 5) | Decision lưu id `opt-N` (đọc được); `ChooseSolutionOption` đổi sang chỉ số khi ghi vào `solutions` như đã làm; một hàm `OptionIndexByID` duy nhất ở `domain` |

## 2. Giải pháp

### A. Cây thư mục (mới trong `request-service/` trừ khi ghi "sửa")

```
internal/domain/request_trigger.go (sửa)           # + information_required, information_provided (18 trigger)
internal/domain/request_transition.go (sửa)        # NextStatusWithResume, nguồn mới của return_to_backlog/type_change/cancel
internal/domain/request_status.go (sửa)            # + awaiting_information (12 giá trị)
internal/domain/clarification.go                   # Clarification, ClarificationStatus, ClarificationSource, ResumeStatusFor
internal/domain/clarification_question.go          # QuestionKind, ClarificationQuestion, ValidateAnswer, ApplyAnswers
internal/domain/readiness_policy.go                # ReadinessPolicy.Evaluate, ReadinessReport
internal/domain/question_builder.go                # QuestionBuilder.Build
internal/domain/decision.go                        # Decision, DecisionStatus, DecisionHistory
internal/domain/decision_risk.go                   # DecisionRisk.Assess
internal/domain/clarification_errors.go            # REQUEST_CLARIFICATION_*, REQUEST_READINESS_*, REQUEST_DECISION_*
internal/usecase/request_clarification.go
internal/usecase/answer_clarification.go
internal/usecase/cancel_clarification.go
internal/usecase/expire_clarifications.go
internal/usecase/waive_readiness.go
internal/usecase/record_decision.go
internal/usecase/confirm_decision.go
internal/usecase/transition_request.go (sửa)       # ResumeStatus, Category
internal/usecase/confirm_request_type.go (sửa)     # gọi readiness
internal/usecase/choose_solution_option.go (sửa)   # gọi RecordDecision
internal/usecase/{solution,plan}_*_handler.go (sửa)# chặn duyệt (blocking question, Decision chưa effective)
internal/adapter/{postgres,mysql}/clarification_repository.go, decision_repository.go
internal/adapter/eventbus/clarification_resume_consumer.go
internal/adapter/grpc/clarification_server.go, decision_server.go
migrations/{postgres,mysql}/NNNN_clarifications_decisions.{up,down}.sql
proto/orca/request/v1/{clarification,decision}.proto
notification-service: internal/adapter/eventbus/consumer.go (sửa), internal/domain/notification_event.go (sửa)
```

### B. Máy trạng thái

Thêm `awaiting_information` (12 trạng thái) và hai trigger. Bảng chuyển đúng CR 2.1: `information_required` từ sáu trạng thái (`awaiting_type_confirmation`, `analyzing`, `awaiting_analysis_approval`, `planning`, `awaiting_plan_approval`, `executing`); `information_provided` chỉ từ `awaiting_information` tới `ResumeStatus`; `awaiting_information` được thêm làm nguồn của `return_to_backlog`, `type_change`, `cancel`. Cài bằng `NextStatusWithResume` (C2); ma trận test 12 nhân 18. `TransitionRequest` giữ bất biến: chỉ nó ghi `status`, chuyển trạng thái và outbox cùng transaction, `ExpectedFrom` làm giao lặp thành công; chốt chặn `go/parser` (SOL-003 mục F) cập nhật cho trường mới `ResumeStatus` nếu lưu trên Request (không lưu; `resume_status` nằm ở `clarifications`).

`ResumeStatusFor(flow FlowDefinition, source ClarificationSource, from RequestStatus) (RequestStatus, error)` theo bảng CR 2.1: `readiness` thành `analyzing` nếu `flow.AnalysisKind != none`, ngược lại `planning`; `solution_open_question` thành `analyzing`; `plan_assumption` thành `planning`; `task_blocked` thành `executing`; `manual` theo trạng thái nguồn (`awaiting_type_confirmation` không hợp lệ cho `manual`).

### C. Migration `NNNN_clarifications_decisions`

```sql
-- Postgres
ALTER TABLE request.requests DROP CONSTRAINT requests_status_check;   -- tên thật: kiểm pg_constraint
ALTER TABLE request.requests ADD CONSTRAINT requests_status_check CHECK (status IN (
  'new','classifying','awaiting_type_confirmation','analyzing','awaiting_analysis_approval','planning',
  'awaiting_plan_approval','executing','completed','request_backlog','cancelled','awaiting_information'));
CREATE TABLE request.clarifications (
  id UUID PRIMARY KEY, tenant_id UUID NOT NULL, request_id UUID NOT NULL, seq INT NOT NULL,
  source TEXT NOT NULL CHECK (source IN ('readiness','solution_open_question','plan_assumption','task_blocked','manual')),
  source_ref TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL CHECK (status IN ('open','answered','expired','cancelled')),
  resume_status TEXT NOT NULL CHECK (resume_status IN ('analyzing','planning','executing')),
  round INT NOT NULL DEFAULT 1, asked_request_revision INT NOT NULL, answered_request_revision INT NULL,
  due_at TIMESTAMPTZ NOT NULL, reminded_at TIMESTAMPTZ NULL, cancel_reason TEXT NOT NULL DEFAULT '',
  created_by TEXT NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT now(), answered_at TIMESTAMPTZ NULL,
  version BIGINT NOT NULL DEFAULT 1, UNIQUE (tenant_id, request_id, seq));
CREATE UNIQUE INDEX clarifications_one_open ON request.clarifications (tenant_id, request_id) WHERE status='open';
CREATE INDEX clarifications_expiry_scan ON request.clarifications (tenant_id, status, due_at);
```

MySQL: `open_key VARCHAR(80) GENERATED ALWAYS AS (IF(status='open', request_id, NULL)) STORED` + `UNIQUE KEY clarifications_one_open (tenant_id, open_key)`; `DROP CHECK` rồi `ADD CONSTRAINT` cho `status` (8.0.16). `clarification_questions` (cột như CR 2.2, `UNIQUE (tenant_id, clarification_id, seq)`), `clarification_assignees` (PK `(clarification_id, principal_kind, principal_id)`, MySQL `principal_id VARCHAR(64) NOT NULL DEFAULT ''`), `decisions` (cột như CR 2.8 + `live_key` MySQL cho chỉ mục duy nhất một phần `status IN ('open','chosen','effective')`), `decision_history` (chỉ thêm). Down: chuyển `awaiting_information` về `request_backlog` (`missing_info`) rồi khôi phục CHECK 11 giá trị, xoá bảng. Chỉ dùng khi rollback toàn bộ v6.

### D. Kiểm tra sẵn sàng (Definition of Ready)

`ReadinessPolicy.Evaluate(t RequestType, c RequestContent) ReadinessReport` gọi `ValidateRequestContent(t, c, ready)` của SOL-027; khoá bắt buộc là `blocking`, khoá khuyến nghị (ví dụ `scope_out`) `blocking=false` và không tạo câu hỏi. `QuestionBuilder.Build(report, hints) []ClarificationQuestion` theo bảng mẫu: `repro_steps` kiểu `text`, `severity`/`exploitability` kiểu `single_choice`, các cờ `boolean`, `acceptance_criteria` kiểu `text` có `suggested_default`. Mặc định đề xuất chỉ lấy từ dữ liệu có sẵn (`source_hints`, `urgency`, `size`); soạn nháp AC bằng `ai.complete` là tuỳ chọn sau cờ `REQUEST_READINESS_AI_DRAFT` (mặc định tắt, lỗi thì bỏ qua). `ConfirmRequestType` (SOL-005 sửa): đủ thì `type_confirmed`; thiếu thì `RequestClarification(source=readiness, assignees=[reporter])` với `round` kế tiếp; vượt `REQUEST_CLARIFICATION_MAX_ROUNDS` (mặc định đề xuất 3) thì `return_to_backlog` (`missing_info`). `WaiveReadiness` chỉ `role=admin`, `reason` bắt buộc, cấm `hotfix|security|ops_request` (`REQUEST_READINESS_WAIVE_FORBIDDEN`), ghi revision `cause=edited` có ghi chú waiver.

### E. Trả lời, hết hạn, nhắc

`AnswerClarification` (một transaction): kiểm người trả lời thuộc `clarification_assignees` hoặc `admin` (`REQUEST_CLARIFICATION_NOT_ASSIGNEE`); kiểm kiểu từng câu (`ValidateAnswer`: `single_choice` thuộc `options`, `multi_choice` tập con, `boolean`, `text` ≤ 4000, `file` chỉ `{filename, mime, size, text}` ≤ 64 KB vì chưa có kho tải lên); `complete=false` lưu nháp; `complete=true` mà thiếu câu bắt buộc thì `REQUEST_CLARIFICATION_INCOMPLETE`; đủ thì `ApplyAnswers` (theo `target_path`) → `AppendRequestRevision(cause=clarification_answered)` (dùng `AppendWithinTx` của SOL-027) → Clarification `answered` → kiểm sẵn sàng lại (tạo Clarification `round+1` hoặc về backlog khi vượt vòng) → Solution `proposed`/Plan sinh từ revision cũ thành `superseded` và Decision tương ứng `superseded` (khi `resume_status` là `analyzing`/`planning`) → `TransitionRequest(information_provided, ExpectedFrom=awaiting_information, ResumeStatus)` → outbox `orca.request.clarification.answered`. Consumer bền `clarification_resume_consumer` (khử trùng `processed_events`) đọc `status_changed` có `trigger=information_provided`: `to=analyzing` thì gọi `GenerateSolution` với `feedback="clarification:<id>"`; `to=planning` không tự chạy (người dùng bấm lại); `to=executing` thì `AdvanceExecution` (SOL-013). `ExpireClarifications` (vòng 60 giây, `FOR UPDATE SKIP LOCKED` Postgres, `SELECT ... FOR UPDATE SKIP LOCKED` rồi `UPDATE` MySQL ≥ 8.0.1) đặt `expired` rồi `ReturnToBacklog(stage, missing_info, "clarification_expired")` với `actor_kind=system`; hết hạn lười: `AnswerClarification` quá hạn chưa quét trả `REQUEST_CLARIFICATION_EXPIRED`. Nhắc một lần ở 50% hạn (`reminded_at`).

### F. Decision

`RecordDecision` chạy trong transaction của `ChooseSolutionOption` (SOL-007 sửa): tạo Decision (`subject_kind=solution_option`) nếu chưa có, ghi `chosen_option_id`, `chooser_id`, `subject_digest` mới. Quy tắc: `chosen != recommended` thì `rationale` không rỗng (`REQUEST_DECISION_RATIONALE_REQUIRED`); `DecisionRisk.Assess(option)` là `high` khi `breaking_change=true`, hoặc có `risks[].severity=high`, hoặc số `affected_areas` kind `service` đạt `REQUEST_DECISION_HIGH_RISK_SERVICES` (mặc định 3); `normal` thì `effective` ngay, `high` thì `chosen` chờ `ConfirmDecision(confirmation_text)` (trùng tiêu đề phương án sau NFC, cắt khoảng trắng, không phân biệt hoa thường; người xác nhận là `chooser_id` hoặc admin; danh tính máy bị từ chối). `SubjectHandler.ValidateForRequest` của `solution` và `plan/task_list` chặn duyệt khi Decision chưa `effective` hoặc `subject_digest` lệch (`REQUEST_DECISION_NOT_EFFECTIVE`), khi còn `open_question.blocking` (`REQUEST_SOLUTION_BLOCKING_QUESTIONS`) hoặc `assumption.needs_confirmation` (`REQUEST_PLAN_UNCONFIRMED_ASSUMPTIONS`) chưa có câu trả lời ghi nhận. Chọn lại khi Approval còn `pending`: ghi `decision_history`, xoá xác nhận cũ.

### G. Proto, lỗi, sự kiện, thông báo, quyền

`clarification.proto`, `decision.proto` như CR 2.9 (RPC: `GetRequestReadiness`, `RequestClarification`, `ListClarifications`, `GetClarification`, `AnswerClarification`, `CancelClarification`, `ListPendingClarificationsForUser`, `WaiveReadiness`, `ListDecisions`, `GetDecision`, `ConfirmDecision`). Sự kiện: `orca.request.clarification.requested|answered|expired|cancelled`, `orca.request.decision.recorded|confirmed`; payload không chứa nội dung câu hỏi hay trả lời. Thông báo: hai binding stream `REQUEST` và hai `subjectRules` (`request.clarification_requested` mức `warning`, kênh ws và push; `request.clarification_expired` mức `warning`); payload do `request-service` đặt (`user_ids` đã mở rộng `reporter`, `team:`, `role:admin`, `title`, `body` ngắn, `deep_link`). Quyền: `ListPendingClarificationsForUser` join `clarification_assignees`; danh tính máy bị từ chối ở `AnswerClarification` và `ConfirmDecision` (cơ chế nhận biết nguồn máy chưa có, như SOL-010).

### H. Bảng chuyển trạng thái và mã lỗi (tóm tắt để cài và kiểm)

| Trigger | Từ | Đến | Điều kiện hoặc tác dụng phụ |
|---|---|---|---|
| `information_required` | `awaiting_type_confirmation`, `analyzing`, `awaiting_analysis_approval`, `planning`, `awaiting_plan_approval`, `executing` | `awaiting_information` | tạo Clarification `open` cùng transaction; hủy Approval `pending` (`information_required`) |
| `information_provided` | `awaiting_information` | `analyzing`, `planning` hoặc `executing` | `ResumeStatus` bắt buộc; revision Request mới đã ghi |
| `return_to_backlog` | thêm `awaiting_information` | `request_backlog` | `Category=missing_info`; Clarification `open` thành `expired` hoặc `cancelled` |
| `type_change` | thêm `awaiting_information` | `awaiting_type_confirmation` | Clarification `open` thành `cancelled` (`type_changed`) |
| `cancel` | thêm `awaiting_information` | `cancelled` | Clarification `open` thành `cancelled` (`request_cancelled`) |

| Mã lỗi | Kind | Khi nào |
|---|---|---|
| `REQUEST_CLARIFICATION_NOT_FOUND` | NotFound | id lạ hoặc khác tenant |
| `REQUEST_CLARIFICATION_NOT_ASSIGNEE` | PermissionDenied | người trả lời không thuộc `clarification_assignees` và không phải admin |
| `REQUEST_CLARIFICATION_INVALID_ANSWER` | InvalidArgument | sai kiểu hoặc ngoài `options` |
| `REQUEST_CLARIFICATION_INCOMPLETE` | FailedPrecondition | `complete=true` mà thiếu câu bắt buộc |
| `REQUEST_CLARIFICATION_EXPIRED` | FailedPrecondition | quá `due_at` (hết hạn lười) |
| `REQUEST_CLARIFICATION_ALREADY_ANSWERED` | FailedPrecondition | trả lời lặp với nội dung khác |
| `REQUEST_CLARIFICATION_STATE_NOT_ALLOWED` | FailedPrecondition | trạng thái Request không cho hỏi hoặc đã có Clarification `open` |
| `REQUEST_READINESS_WAIVE_FORBIDDEN` | PermissionDenied | waive với `hotfix`, `security`, `ops_request` hoặc không phải admin |
| `REQUEST_DECISION_RATIONALE_REQUIRED` | InvalidArgument | chọn khác đề xuất mà thiếu lý do |
| `REQUEST_DECISION_NOT_EFFECTIVE` | FailedPrecondition | duyệt khi Decision chưa `effective` hoặc lệch digest |

## 3. Quyết định thiết kế

| # | Quyết định | Lý do |
|---|-----------|-------|
| 1 | Một trạng thái `awaiting_information`, đích quay lại ở `clarifications.resume_status` | Không nhân trạng thái theo nguồn |
| 2 | `NextStatusWithResume` thay vì đổi chữ ký `NextStatus` | Không vỡ test và nơi gọi của SOL-003 |
| 3 | Kiểm sẵn sàng lúc xác nhận loại | Trường bắt buộc phụ thuộc loại |
| 4 | Một Clarification `open` mỗi Request (chỉ mục duy nhất một phần/cột sinh) | Chặn hai nơi cùng giữ Request |
| 5 | Câu trả lời thành revision Request mới | Truy vết, không mất căn cứ Approval |
| 6 | Hết hạn đi qua `ReturnToBacklog` | Một nơi ghi lý do trả về |
| 7 | Decision tách khỏi `chosen_option` và Approval | Lý do, người chọn, lịch sử, xác nhận lần hai |
| 8 | Chặn waive với `hotfix`, `security`, `ops_request` | Thiếu dữ liệu ở ba loại này gây hại thật |

## 4. Phụ thuộc và thứ tự

Cần SOL-027 (task 027-05 `AppendRequestRevision`, `ValidateRequestContent`), SOL-003 (task 003-02, 003-03), SOL-005 (task 005-05), SOL-007 (task 007-06), SOL-009 (009-04 `CancelPendingForRequest`), SOL-010 (task 010-04, 010-05 cho principal và thông báo), CR-REQ-006 (`ReturnToBacklog`). Thứ tự task: 01 migration, 02 domain và máy trạng thái, 03 repository, 04 sẵn sàng và `RequestClarification`, 05 `AnswerClarification` và các hook huỷ, 06 consumer, hết hạn, thông báo, 07 Decision và chặn duyệt, 08 proto, gRPC, tích hợp. Mở khoá: CR-REQ-029 (`needs_info`).

## 5. Kiểm thử

- **Unit domain:** ma trận 12 nhân 18 trigger; `ResumeStatusFor`; `ReadinessPolicy` 11 loại; `QuestionBuilder`; `ApplyAnswers` theo `target_path`; `ValidateAnswer` từng kiểu; `DecisionRisk.Assess`; so khớp `confirmation_text` với chữ tiếng Việt (NFC và tổ hợp).
- **Unit usecase (repo, clock giả):** nhánh `AnswerClarification` (nháp, thiếu, sai kiểu, quá hạn), vòng tối đa, waive, chọn lại, chặn duyệt.
- **Integration hai dialect:** chỉ mục `open` duy nhất, `SKIP LOCKED` hai worker, transaction đa bảng rollback, CHECK 12 giá trị lên/xuống.
- **Hợp đồng:** `buf breaking`; test đối chiếu danh sách trạng thái trong mã với README v6 (12 giá trị).
- **Chưa kiểm chứng:** thời gian trả lời thực của người báo cáo; chất lượng nháp AC; trạng thái task quay lại sau `needs_info`.

## 6. Rủi ro và điểm chưa kiểm chứng

- Thêm trạng thái chạm mọi nơi liệt kê trạng thái (backlog, Board, thống kê, frontend `RequestStatus`); dùng `codegraph explore "RequestStatus"` trước khi sửa.
- Người báo cáo Jira có thể không có tài khoản Orca nên Clarification hết hạn; chưa có đường trả lời ngược vào Jira (Q4).
- Hủy Approval `pending` khi vào `awaiting_information` làm mất lựa chọn chưa duyệt; chấp nhận.
- Vòng hỏi nhiều lần làm chậm; 3 vòng và các hạn mặc định (7 ngày, 72 giờ, 24 giờ) chỉ là đề xuất.
- `file` chỉ nhận văn bản, không có ảnh hoặc log nhị phân.

## 7. Câu hỏi mở

1. Quyền hỏi tay (`manual`) ở mức Request chưa chốt (README mục 8).
2. Có thêm kho tệp đính kèm cho `file` không.
3. `information_provided` có tự chạy `GenerateSolution` (đề xuất có) và `GeneratePlan` (đề xuất không).
4. Có đẩy câu hỏi và trả lời ngược lại Jira/GitHub không (CR-REQ-024).
5. Ngưỡng rủi ro `high` đã đủ chưa; có cần "không đảo ngược" khi `rollback` rỗng.
6. Xác nhận lần hai có cần người khác `chooser` ở `security` không.

## 8. Tham chiếu

- `/opt/repos/orca/docs/crs/v6/request-artifact-model/CR-REQ-028-clarification-and-decision-records.md`; `/opt/repos/orca/docs/crs/v6/README.md` mục 3.3, 3.7, 8
- `/opt/repos/orca/backend-go/services/notification-service/internal/adapter/eventbus/consumer.go` (`Subjects`), `internal/domain/notification_event.go` (`subjectRules`, `defaultRule`)
- `/opt/repos/orca/backend-go/services/mcp-service/internal/domain/approval.go` (`ParamsHash`, `EffectiveStatus`)
- `/opt/repos/orca/backend-go/common/tenant/tenant.go`, `common/outbox/outbox.go`, `common/dbcapability/capability.go`
- Solution và task liên quan: `../../request-lifecycle/tasks/TASK-REQ-003-02-*.md`, `TASK-REQ-003-03-*.md`, `TASK-REQ-005-05-*.md`; `../../approval/solutions/BE-REQ-SOL-009-*.md`, `BE-REQ-SOL-010-*.md`; `../../solution-analysis/solutions/BE-REQ-SOL-007-*.md`
