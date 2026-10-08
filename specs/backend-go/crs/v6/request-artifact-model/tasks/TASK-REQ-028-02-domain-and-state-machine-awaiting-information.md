# TASK-REQ-028-02: Domain Clarification, Decision, sẵn sàng và mở rộng máy trạng thái (12 trạng thái, 18 trigger)

**From Solution:** BE-REQ-SOL-028
**Priority:** P0
**Service:** `request-service`
**File:** `internal/domain/request_status.go` (sửa), `internal/domain/request_trigger.go` (sửa), `internal/domain/request_transition.go` (sửa), `internal/domain/clarification.go`, `internal/domain/clarification_question.go`, `internal/domain/readiness_policy.go`, `internal/domain/question_builder.go`, `internal/domain/decision.go`, `internal/domain/decision_risk.go`, `internal/domain/clarification_errors.go` và test (mới trừ các file sửa)
**Depends on:** TASK-REQ-003-02 (trigger, `NextStatus`), TASK-REQ-027-05 (`ValidateRequestContent`, `RequiredFields`, `RequestContent`), TASK-REQ-002-02
**Status:** [x] DONE (đã kiểm chứng 2026-10-08: `go test ./internal/domain`)

---

## Context

CR-REQ-028 mục 2.1 đến 2.4, 2.8. Hiện (TASK-REQ-003-02): 11 trạng thái, 16 trigger, `NextStatus(flow, size, from, trigger) (RequestStatus, error)` và test table-driven phủ ma trận 11 nhân 16; `type_change` có 6 nguồn, `return_to_backlog` 7 nguồn, `cancel` mọi trạng thái trừ cuối. Task này **mở rộng** chứ không thay: đích của `information_provided` phụ thuộc `ResumeStatus` nên không nhét được vào `NextStatus` mà không đổi chữ ký (SOL-028 C2). Quyết định: giữ `NextStatus`, thêm `NextStatusWithResume`.

Toàn bộ là mã thuần, không I/O, để bảng test bao phủ hết. Tên tệp theo khái niệm (`clarification.go`, không `helpers`).

## Việc cần làm

1. `request_status.go`: thêm `StatusAwaitingInformation RequestStatus = "awaiting_information"`;
   - `AllRequestStatuses()` thành 12 giá trị
   - `IsTerminal()` không đổi
   - `IsHumanWaiting()` (mới, trả đúng cho các trạng thái chờ người: `awaiting_type_confirmation`, `awaiting_analysis_approval`, `awaiting_plan_approval`, `awaiting_information`) nếu SOL-015/frontend cần (kiểm xem đã có).
2. `request_trigger.go`: thêm `TriggerInformationRequired = "information_required"`, `TriggerInformationProvided = "information_provided"`;
   - `AllTriggers()` thành 18.
3. `request_transition.go`: `NextStatus` cho `information_required` từ sáu trạng thái nguồn (`awaiting_type_confirmation`, `analyzing`, `awaiting_analysis_approval`, `planning`, `awaiting_plan_approval`, `executing`) thành `awaiting_information`;
   - `information_provided` trong `NextStatus` trả `REQUEST_TRANSITION_NOT_ALLOWED` (cần `resume`)
   - thêm `awaiting_information` vào danh sách nguồn của `return_to_backlog` (8 nguồn), `type_change` (7 nguồn, đích `awaiting_type_confirmation`), `cancel` (đã là mọi trạng thái không cuối: tự động).
   - `NextStatusWithResume(flow, size, from, trigger, resume RequestStatus) (RequestStatus, error)`: cho `information_provided` yêu cầu `from == awaiting_information` và `resume` thuộc `{analyzing, planning, executing}` (`REQUEST_RESUME_STATUS_INVALID` nếu khác), các trigger khác ủy thác `NextStatus`.
   - `HappyPath` (TASK-REQ-003-02) **không** đổi: `awaiting_information` không nằm trên đường chuẩn.
4. `clarification.go`: `ClarificationStatus` (`open|answered|expired|cancelled`), `ClarificationSource` (`readiness|solution_open_question|plan_assumption|task_blocked|manual`), `Clarification{ID, TenantID, RequestID string; Seq int; Source ClarificationSource; SourceRef string; Status ClarificationStatus; ResumeStatus RequestStatus; Round, AskedRequestRevision int; AnsweredRequestRevision *int; DueAt time.Time; RemindedAt *time.Time; CancelReason, CreatedBy string; CreatedAt time.Time; AnsweredAt *time.Time; Version int64; Questions []ClarificationQuestion; Assignees []Assignee}`;
   - `DisplayID() string` (`CLR-<reqnum>.<seq>`, cần `reqNum` truyền vào)
   - phương thức chuyển trạng thái `Answer(at)`, `Expire(at)`, `Cancel(reason, at)` (chỉ từ `open`, nếu không `REQUEST_CLARIFICATION_NOT_OPEN`)
   - `IsExpired(now)`
   - `ResumeStatusFor(flow FlowDefinition, source ClarificationSource, from RequestStatus) (RequestStatus, error)` đúng bảng SOL-028 mục B
   - `DefaultDue(source ClarificationSource, urgency Urgency, now time.Time) time.Time` (readiness 7 ngày, urgent 24 giờ; `solution_open_question`/`plan_assumption` 72 giờ, urgent 8 giờ; `task_blocked` 24 giờ; số đề xuất chưa kiểm chứng, đọc ghi đè từ chính sách khi SOL-010 có).
5. `clarification_question.go`: `QuestionKind` năm giá trị;
   - `ClarificationQuestion{ID string; Seq int; QuestionKey string; Kind QuestionKind; Prompt, Reason string; Options []QuestionOption; SuggestedDefault json.RawMessage; Required bool; TargetPath string; Answer json.RawMessage; AnswerSource string; AnsweredBy string; AnsweredAt *time.Time}`
   - `ValidateQuestion(q) error` (`Prompt` ≤ 1000, `Reason` bắt buộc ≤ 500, `Options` bắt buộc cho hai kiểu chọn)
   - `ValidateAnswer(q ClarificationQuestion, value json.RawMessage) error` (`single_choice` thuộc `Options`, `multi_choice` tập con không rỗng, `boolean`, `text` ≤ 4000 ký tự theo rune, `file` là `{filename, mime, size, text}` với `text` ≤ 64 KB; sai thì `REQUEST_CLARIFICATION_INVALID_ANSWER`)
   - `ApplyAnswers(content RequestContent, qs []ClarificationQuestion) (RequestContent, error)`: theo `TargetPath` dạng `type_fields.severity`, `acceptance_criteria` (thêm AC mới qua `AcceptanceCriteria.Add`), `body`
   - `target_path` không hỗ trợ thì `REQUEST_CLARIFICATION_INVALID_ANSWER`, không đoán.
6. `readiness_policy.go`: `ReadinessReport{Ready bool; Missing []MissingField}`, `MissingField{Path, Rule string; Blocking bool}`;
   - `ReadinessPolicy.Evaluate(t RequestType, c RequestContent) ReadinessReport` gọi `ValidateRequestContent(t, c, ValidationLevelReady)` và `RequiredFields(t)` của SOL-027
   - chỉ `Blocking` quyết định `Ready=false`.
   - `question_builder.go`: `QuestionBuilder.Build(report ReadinessReport, hints ReadinessHints) []ClarificationQuestion` theo bảng mẫu: `type_fields.repro_steps` kiểu `text`
   - `severity`, `exploitability` kiểu `single_choice` với tập từ `RequiredFields`
   - khoá `[]` kiểu `text` nhiều dòng
   - cờ kiểu `boolean`
   - `acceptance_criteria` kiểu `text` kèm `SuggestedDefault`
   - `Reason` luôn điền ("Loại bug cần bước tái hiện để chẩn đoán").
   - Thứ tự câu hỏi xác định (theo `RequiredFields`).
7. `decision.go`: `DecisionStatus` (`open|chosen|effective|superseded`), `DecisionSubjectKind`, `Decision{...cột theo SOL-028 mục C}`, `DecisionOption{ID, Label, Summary string; Risk DecisionRiskInfo}`;
   - `(d *Decision) Choose(optionID, chooserID, rationale string, risk RiskLevel, digest string) error` (khác `recommended` mà `rationale` rỗng thì `REQUEST_DECISION_RATIONALE_REQUIRED`; `normal` thì `effective`, `high` thì `chosen`; chọn lại xoá `confirmed_*`)
   - `(d *Decision) Confirm(by, text string) error` (`REQUEST_DECISION_CONFIRMATION_MISMATCH` khi `NormalizeTitle(text) != NormalizeTitle(title phương án chọn)`)
   - `Supersede()`
   - `NormalizeTitle(s)` (NFC, cắt khoảng trắng đầu cuối, gộp khoảng trắng giữa, `strings.ToLower`).
   - `decision_risk.go`: `DecisionRisk.Assess(opt SolutionOption, serviceThreshold int) RiskLevel` (high khi `breaking_change`, hoặc có `risks[].severity=high`, hoặc số `affected_areas` kind `service` ≥ ngưỡng)
   - `OptionIndexByID(opts []SolutionOption, id string) (int, bool)` (C9).
8. `clarification_errors.go`: constructor `REQUEST_CLARIFICATION_NOT_FOUND` (NotFound), `_NOT_OPEN`, `_EXPIRED`, `_ALREADY_ANSWERED`, `_INCOMPLETE`, `_NOT_ASSIGNEE` (PermissionDenied), `_INVALID_ANSWER` (InvalidArgument), `_STATE_NOT_ALLOWED`, `_VERSION_CONFLICT`, `REQUEST_READINESS_WAIVE_FORBIDDEN`, `REQUEST_RESUME_STATUS_INVALID`, `REQUEST_DECISION_RATIONALE_REQUIRED` (InvalidArgument), `_NOT_EFFECTIVE`, `_CONFIRMATION_MISMATCH`, `_AGENT_FORBIDDEN`, `_SELF_CHOICE_FORBIDDEN`, `REQUEST_SOLUTION_BLOCKING_QUESTIONS`, `REQUEST_PLAN_UNCONFIRMED_ASSUMPTIONS` (FailedPrecondition trừ khi ghi khác).

## Kiểm thử

- `TestNextStatus_FullMatrix_12x18` (mọi cặp `RequestStatus` x `Trigger`; kỳ vọng sinh từ một bảng nguồn duy nhất trong test; fail nếu cặp ngoài bảng không trả `REQUEST_TRANSITION_NOT_ALLOWED`); `TestNextStatusWithResume_OnlyThreeResumeValues`
- `TestNextStatus_FromWrongState`
- `TestNextStatus_DelegatesOtherTriggers`.
- `TestTypeChange_FromAwaitingInformation`
- `TestReturnToBacklog_FromAwaitingInformation`
- `TestCancel_FromAwaitingInformation`.
- `TestResumeStatusFor_Table` (mọi nguồn, flow có/không `AnalysisKind`).
- `TestClarification_StateMachine`
- `TestDefaultDue_Table`
- `TestValidateAnswer_AllKinds` (kể cả `file` quá 64 KB, `text` 4001 ký tự tiếng Việt có dấu theo rune)
- `TestApplyAnswers_TargetPaths`
- `TestApplyAnswers_UnsupportedPath`.
- `TestReadinessPolicy_AllElevenTypes`
- `TestQuestionBuilder_DeterministicOrderAndReasons`.
- `TestDecision_Choose_RationaleRequiredWhenNotRecommended`
- `TestDecision_HighRiskStaysChosen`
- `TestDecision_RechooseClearsConfirmation`
- `TestDecision_Confirm_VietnameseNFCAndCase`
- `TestDecisionRisk_Assess_Table`
- `TestOptionIndexByID`.
- Lệnh: `cd /opt/repos/orca/backend-go && go test ./services/request-service/internal/domain/... -run 'NextStatus|Resume|Clarification|Readiness|QuestionBuilder|Decision|ApplyAnswers|ValidateAnswer'`.

## Tiêu chí hoàn thành

- [x] Ma trận 12 nhân 18 xanh; chỉ các cặp ở bảng CR 2.1 hợp lệ.
- [x] `information_provided` chỉ tới `analyzing|planning|executing` qua `NextStatusWithResume`.
- [x] Test 003-02 cũ (11 nhân 16) vẫn xanh sau khi cập nhật số liệu, không đổi kỳ vọng của các cặp cũ.
- [x] `ApplyAnswers` không bao giờ đoán đường dẫn; `ValidateAnswer` phủ năm kiểu.
- [x] Domain không import gói ngoài stdlib, `x/text`, `common/apperrors`.

## Rủi ro và lưu ý

- Sửa tệp của TASK-REQ-003-02 sau khi đã merge: giữ commit nhỏ, cập nhật `status_write_guard_test.go` nếu thêm trường vào `TransitionInput` (task 028-04).
- `DefaultDue` dùng số đề xuất; khi SOL-010 có bảng chính sách thì đọc từ đó, nên tách hàm để thay được.
- Văn bản câu hỏi tiếng Việt hay tiếng Anh: chưa chốt (i18n ở frontend); `Prompt` hiện sinh bằng tiếng Việt cố định, ghi ở README task.
- `awaiting_information` cần được thêm vào frontend `RequestStatus` và bảng loại trừ của MCP (CR mục 9); không làm ở đây.

## Ghi chú triển khai (rf/art)

- 12 trạng thái, 18 trigger; `NextStatus` giữ chữ ký và từ chối `information_provided`; `NextStatusWithResume` chỉ nhận `analyzing|planning|executing` (`REQUEST_RESUME_STATUS_INVALID`); `StageForStatus` có `awaiting_information` (classification, analysis, plan, [phase], task). Test ma trận cũ (`TestNextStatus_FullMatrix`, nay 12x18 với 44 cặp hợp lệ) đổi số liệu, kỳ vọng các cặp cũ không đổi; thêm test cho từng đích của `information_required`, `NextStatusWithResume` (ba giá trị, sai trạng thái, uỷ quyền các trigger khác).
- `QuestionBuilder.Build(report, hints)` lấy loại Request từ `ReadinessReport.Type` (thêm trường này so với task để giữ chữ ký hai tham số). `Decision` dùng `Principal` sẵn có của approval cho assignee. `NormalizeTitle` của task đổi tên `NormalizeConfirmationText` (đã có `NormalizeTitle` ở `request_source_normalization.go`).
- `DecisionRisk.Assess` nhận `RiskSignals` (suy ra từ JSON của phương án bằng `RiskSignalsFromOption`: `breaking_change`, `risks[].severity=high` hoặc `risk.level=high`, số `affected_areas` kind `service`) thay vì kiểu `SolutionOption`, vì kiểu đó đang được nhánh Solution viết lại. `OptionIndexByID` nhận `[]Option` hiện có.
- Domain của file clarification/decision chỉ import stdlib, `x/text`, `common/apperrors`. (Cả package còn import `santhosh-tekuri/jsonschema` và `yaml.v3` ở các file artifact của 027.)
- README v6 mục 3.3 được sửa thêm `awaiting_information` để `TestREADMEListsSameTypesAndStatuses` (hợp đồng sẵn có) tiếp tục xanh.
