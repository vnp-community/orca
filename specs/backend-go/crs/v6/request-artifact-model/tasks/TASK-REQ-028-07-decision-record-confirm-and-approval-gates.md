# TASK-REQ-028-07: `RecordDecision`, `ConfirmDecision` và chặn duyệt Solution, Plan khi Decision chưa `effective`

**From Solution:** BE-REQ-SOL-028
**Priority:** P0
**Service:** `request-service`
**File:** `internal/usecase/record_decision.go`, `internal/usecase/confirm_decision.go`, `internal/usecase/decision_gate.go`, `internal/usecase/choose_solution_option.go` (sửa, của SOL-007), `internal/usecase/solution_approval_handler.go` (sửa), `internal/usecase/plan_subject_handler.go` (sửa, của SOL-012), `internal/usecase/ports.go` (sửa) và test
**Depends on:** TASK-REQ-028-02, 028-03, TASK-REQ-007-06 (`ChooseSolutionOption`, handler), TASK-REQ-012-06 (handler `plan`/`task_list`), TASK-REQ-027-07 (`OpenQuestion`, `Assumption` có cấu trúc), TASK-REQ-010-02 (`ApprovalAuthorization.Decide`, nếu đã có)
**Status:** [ ] TODO (một phần: phía Solution xong và kiểm chứng 2026-10-08 trên Postgres và MySQL qua `Approve` thật; còn cổng Plan (`CheckPlan`) vì handler Plan chưa tồn tại)

---

## Context

CR-REQ-028 mục 2.5, 2.8. Hiện `ChooseSolutionOption` (SOL-007 mục E) chỉ: Solution phải `proposed`; đổi `option_id` thành chỉ số; `UPDATE solutions SET chosen_option=?, version=version+1 WHERE ... AND status='proposed' AND version=?`; cùng transaction `ApprovalRepository.UpdatePendingDigest("solution", id, DigestOptions(options, &idx))`; lặp cùng `option_id` là no-op. Không ghi ai chọn, vì sao, chọn lại bao nhiêu lần, không có xác nhận lần hai cho lựa chọn rủi ro cao (`breaking_change`).

`SolutionApprovalHandler.ValidateForRequest` hiện: trả `DigestOptions(options, chosen)`; `kind=solution` mà `chosen_option` NULL thì `REQUEST_SOLUTION_OPTION_NOT_CHOSEN`. Task này thêm hai kiểm tra: Decision `effective` với `subject_digest` khớp, và không còn `open_question.blocking` hoặc `assumption.needs_confirmation` chưa có câu trả lời ghi nhận.

Decision chỉ áp dụng cho `diagnosis`? Không: `diagnosis`, `findings`, `answer` không có phương án nên **không** cần Decision (CR 2.8 bước 5). `kind=solution` mới có.

Quyền: người chọn phải qua `ApprovalAuthorizer.CanDecide` của Approval `pending` của chủ thể (cùng tập principal của SOL-010). Nếu `approvals.self_approval_allowed=false` và người gọi là `reporter_id` thì `REQUEST_DECISION_SELF_CHOICE_FORBIDDEN`, kể cả admin.

## Việc cần làm

1. `ports.go`: `DecisionRecorder{Record(ctx, tx, in RecordDecisionInput) (domain.Decision, error)}` để `ChooseSolutionOption` gọi trong transaction của nó;
   - `OptionLoader` dùng sẵn `SolutionRepository`.
2. `record_decision.go`: `RecordDecisionInput{Solution domain.Solution; OptionID, ChooserID, Rationale string; Options []domain.SolutionOption; RecommendedOptionID, RecommendationReason string; SubjectDigest string}`;
   - `RecordDecision.Execute(ctx, tx, in)`: `GetLiveBySubject(solution_option, solution.ID)`
   - chưa có thì tạo (`Seq = NextSeq`, `Question` mặc định "Chọn phương án cho REQ-<n>", `Options` rút gọn `{id,label,summary,risk}` từ `SolutionOptions`, `RecommendedOptionID` từ `recommendation.option_id`)
   - `Choose(optionID, chooserID, rationale, DecisionRisk.Assess(option, REQUEST_DECISION_HIGH_RISK_SERVICES), digest)`
   - ghi `decision_history` (`chosen` lần đầu, `rechosen` các lần sau)
   - outbox `orca.request.decision.recorded` `{decision_id, request_id, display_id, subject_id, risk_level, status}` (không chứa `rationale`).
3. `choose_solution_option.go` (sửa): thêm tham số `Rationale string` vào `ChooseSolutionInput` (proto `ChooseSolutionOptionRequest.rationale`, số trường kiểm thật);
   - sau khi cập nhật `solutions.chosen_option` và `UpdatePendingDigest` trong cùng transaction, gọi `DecisionRecorder.Record`
   - lỗi `REQUEST_DECISION_RATIONALE_REQUIRED` làm cả lệnh rollback.
   - Quyền: gọi `ApprovalAuthorizer.CanDecide(approval pending của Solution)` trước khi ghi
   - `REQUEST_DECISION_SELF_CHOICE_FORBIDDEN` khi `!self_approval_allowed` và người gọi là reporter.
   - Phản hồi thêm `decision_status` và `requires_confirmation` (bool) để UI biết cần bước hai.
4. `confirm_decision.go`: `ConfirmDecisionInput{DecisionID, ConfirmationText string; ExpectedVersion int64; ActorID string}`;
   - `Execute`: `RequireTenantID`
   - `Get` Decision
   - `status` phải `chosen` (đã `effective` thì trả thành công idempotent nếu cùng người xác nhận; `open` thì `REQUEST_DECISION_NOT_EFFECTIVE`? dùng mã `REQUEST_DECISION_STATE_INVALID`)
   - danh tính máy bị `REQUEST_DECISION_AGENT_FORBIDDEN`
   - người xác nhận là `chooser_id` hoặc `role=admin`
   - `d.Confirm(by, text)` (so khớp `NormalizeTitle`)
   - CAS `Update`
   - `AppendHistory(confirmed)`
   - outbox `orca.request.decision.confirmed`.
   - Tool MCP `decision_confirm` **không** đăng ký (CR 2.8): ghi vào README task cho CR-REQ-017.
5. `decision_gate.go`: `DecisionGate.Check(ctx, solution domain.Solution, digest string) error`: Solution `kind=solution`: Decision sống của Solution phải `effective` và `subject_digest == digest` (`REQUEST_DECISION_NOT_EFFECTIVE`, FailedPrecondition, thông điệp nêu cần xác nhận ở bước hai);
   - `diagnosis|findings|answer`: bỏ qua.
   - Dùng bởi cả handler `solution` và handler `plan`/`task_list` (Plan sinh từ Solution đó): Plan chỉ yêu cầu Solution gốc có Decision `effective`.
6. `solution_approval_handler.go` (sửa): `ValidateForRequest` gọi `DecisionGate.Check` và `BlockingItems(solution)`: danh sách `open_questions[blocking=true]` mà chưa có Clarification `answered` có `source=solution_open_question`, `source_ref=solution.ID` và `question_key="open_question:<Q-id>"` thì `REQUEST_SOLUTION_BLOCKING_QUESTIONS` (chi tiết: các `Q-id`).
   - `plan_subject_handler.go` (sửa): `needs_confirmation=true` chưa có câu trả lời thì `REQUEST_PLAN_UNCONFIRMED_ASSUMPTIONS`.
7. Hết hiệu lực: Solution `superseded` (đổi loại, sinh lại, trả lời Clarification) thì Decision sống `superseded` cùng transaction: gọi `SupersedeLiveBySubject` trong `SolutionRepository.Supersede...` hook hoặc ở chỗ gọi (đã làm ở 028-05 cho trả lời Clarification; thêm ở `GenerateSolution` sinh lại và `ChangeRequestType` nếu chúng `Supersede` Solution), kèm `decision_history(superseded)`.
8. `RequestClarification(source=solution_open_question|plan_assumption)` đã có ở 028-04;
   - ở đây chỉ **kiểm** chặn duyệt.
   - Chọn lại khi Approval còn `pending`: `RecordDecision` đưa Decision về `open`/`chosen` mới và xoá `confirmed_*`, ghi `rechosen`.

## Kiểm thử

- `TestRecordDecision_FirstChoose_NormalRisk_Effective`
- `TestRecordDecision_BreakingChange_StaysChosen`
- `TestRecordDecision_NotRecommendedWithoutRationale_Rejected`
- `TestRecordDecision_RechooseWritesHistoryAndClearsConfirmation`
- `TestRecordDecision_PayloadHasNoRationale`.
- `TestChooseSolutionOption_RecordsDecisionSameTx` (lỗi rationale làm rollback cả `solutions.chosen_option`)
- `TestChooseSolutionOption_SelfChoiceForbidden_EvenAdmin`
- `TestChooseSolutionOption_NotAuthorizedPrincipal_Rejected`
- `TestChooseSolutionOption_SameOptionTwice_NoOp`.
- `TestConfirmDecision_ExactTitleVietnameseNFCCaseInsensitive`
- `TestConfirmDecision_Mismatch`
- `TestConfirmDecision_MachineForbidden`
- `TestConfirmDecision_OnlyChooserOrAdmin`
- `TestConfirmDecision_IdempotentWhenAlreadyEffective`
- `TestConfirmDecision_StaleVersion`.
- `TestSolutionHandler_Validate_DecisionNotEffective_Blocks`
- `TestSolutionHandler_DigestMismatchAfterRechoose_Blocks`
- `TestSolutionHandler_BlockingQuestionUnanswered_Blocks`
- `TestSolutionHandler_BlockingQuestionAnsweredViaClarification_Passes`
- `TestSolutionHandler_DiagnosisKindSkipsDecision`.
- `TestPlanHandler_UnconfirmedAssumption_Blocks`
- `TestPlanHandler_DecisionOfSourceSolutionRequired`.
- `TestDecisionSupersededWhenSolutionSuperseded`.
- Integration hai dialect: `Record` đồng thời hai lần (chỉ mục sống: một Decision), CAS `ConfirmDecision`, transaction đa bảng (`solutions`, `approvals.subject_digest`, `decisions`, `decision_history`, outbox) rollback.
- Lệnh: `cd /opt/repos/orca/backend-go && go test ./services/request-service/internal/usecase/... -run 'RecordDecision|ConfirmDecision|ChooseSolutionOption|DecisionGate|SolutionHandler|PlanHandler' && go test -tags=integration ./services/request-service/... -run 'Decision'`.

## Tiêu chí hoàn thành

- [x] Chọn phương án khác đề xuất mà thiếu `rationale` bị `REQUEST_DECISION_RATIONALE_REQUIRED`, không để lại thay đổi nào. (use case, ở mức giao dịch DB)
- [x] Phương án `breaking_change` ở `chosen` chặn `Approve` bằng `REQUEST_DECISION_NOT_EFFECTIVE` đến khi `ConfirmDecision` đúng tên phương án. (`TestSolutionHandler_HighRiskPickBlocksApprovalUntilConfirmed` qua handler thật với kho giả; DB thật hai dialect chỉ ở mức repository/use case, chưa qua `Approve` RPC)
- [x] Chọn lại khi Approval `pending`: ghi `decision_history`, xoá xác nhận cũ, digest Approval cập nhật. (hai nửa kiểm riêng: `RecordDecision` chọn lại ghi `rechosen`; `TestChooseSolutionOption_RecordsChoiceAndRefreshesTheDigest` + `SolutionFlowGenerateChooseApprove` kiểm digest Approval; chưa có một kịch bản DB liền mạch)
- [x] Solution `superseded` kéo theo Decision `superseded`. (tạo lại Solution kèm góp ý và Persist: `SolutionContract/SolutionArtifactsDecisionsAndGates` hai dialect; nhánh `type_changed` của handler có mã nhưng chưa có test riêng)
- [ ] `open_question.blocking` và `assumption.needs_confirmation` chưa có câu trả lời chặn duyệt với mã đúng. (`open_question.blocking` xong: `REQUEST_SOLUTION_BLOCKING_QUESTIONS` qua `Approve` thật hai dialect; `assumption.needs_confirmation` của Plan chưa nối vì chưa có handler Plan)
- [x] `diagnosis`, `findings`, `answer` duyệt không cần Decision. (`TestSolutionHandler_DiagnosisNeedsNoDecisionEvenWithGatesWired`; findings/answer dùng handler không có cổng)

## Rủi ro và lưu ý

- Hai bước (chọn rồi xác nhận) làm UI phức tạp hơn (CR-REQ-020); phản hồi `requires_confirmation` là hợp đồng với frontend.
- `subject_digest` của Decision phải được đồng bộ mỗi khi digest Approval đổi (SOL-009 `UpdatePendingDigest`); lệch làm Decision không bao giờ khớp. Test kiểm điểm nối này.
- Ngưỡng `high` (breaking, severity high, ≥ 3 service) là đề xuất; Q5.
- Danh tính máy chưa nhận biết được (`IsMachine` luôn false): chốt `REQUEST_DECISION_AGENT_FORBIDDEN` hiện chỉ có tác dụng khi cơ chế nhận biết có; CR-REQ-017 không đăng ký tool `decision_confirm` là biện pháp thật.

## Tiến độ (rf/art, 2026-10-08)

Đã làm và kiểm chứng (`go test ./internal/usecase -run 'RecordDecision|ConfirmDecision|ApprovalGates|DecisionQueries'`, `go test -tags integration ./internal/adapter/postgres ./internal/adapter/mysql -run ClarificationContract/Decision`):
- `RecordDecision` (tạo/ghi lựa chọn, rủi ro cao → `chosen`, `rationale` bắt buộc khi khác đề xuất, chọn lại xoá xác nhận và ghi `rechosen`, kiểm tự chọn với cờ `SelfChoiceAllowed`, sự kiện không chứa `rationale`), `ConfirmDecision` (RPC thật, gõ lại tên phương án NFC/không phân biệt hoa thường, chỉ người chọn hoặc admin, máy bị chặn, lặp là no-op, CAS), `ApprovalGates` (`CheckDecision`, `CheckSolution` với câu hỏi `blocking`, `CheckPlan` với giả định `needs_confirmation`; tài liệu `diagnosis|findings|answer` không gọi `CheckSolution`), `DecisionQueries`; nối RPC `ListDecisions`, `GetDecision`, `ConfirmDecision`; `RecordDecision` và `ApprovalGates` dựng sẵn ở `wire_artifact.go` (`artifactWiring.RecordDecision`, `.Gates`).
- Kiểm trên DB thật hai dialect: chọn rủi ro cao → `chosen` → cổng chặn → xác nhận đúng tên → cổng qua; chọn lại ghi đủ lịch sử; refuse chọn thiếu lý do không để lại dòng nào; chỉ mục "một Decision sống" chặn bản thứ hai.

Chưa làm vì phụ thuộc code chưa có trong nhánh: `ChooseSolutionOption` gọi `RecordDecision` trong giao dịch của nó (và `ApprovalAuthorizer.CanDecide`), `SolutionApprovalHandler.ValidateForRequest` và `plan_subject_handler` gọi `ApprovalGates`, Decision `superseded` khi Solution bị thay (Decision đã `superseded` khi trả lời Clarification), đồng bộ `subject_digest` với `UpdatePendingDigest`. Hai tiêu chí còn mở nên giữ `[ ]`: chặn `Approve` thật và `superseded` theo Solution; tiêu chí "`diagnosis|findings|answer` không cần Decision" chỉ ở mức cổng.

## Tiến độ sau hợp nhất (2026-10-08)

Đã nối (nhánh `feat/request-flow-backend` sau khi hợp nhất `rf/art`): `ChooseSolutionOption.WithDecisions` gọi `RecordDecision` trong giao dịch của nó (nhận `rationale`, chính sách tự chọn theo `self_approval_allowed` của Approval đang chờ), trả `decision_status` và `requires_confirmation`; handler `solution` gọi `ApprovalGates.CheckSolution` và kiểm AC cho phương án đã chọn trước khi duyệt; `GenerateSolution` và `AnalysisResultWriter` đóng Decision của Solution bị thay. Kiểm chứng: `go test -race ./internal/usecase`; `go test -tags integration ./internal/adapter/postgres ./internal/adapter/mysql -run SolutionContract` (kịch bản `SolutionArtifactsDecisionsAndGates`: từ chối thiếu `rationale` không để lại Decision, chọn đề xuất thì `effective`, `Approve` bị chặn bởi câu hỏi `blocking`, qua sau khi có câu trả lời, tạo lại Solution đóng Decision).

Còn thiếu: `CheckPlan` ở handler Plan (SOL-012 còn là stub `GeneratePlan/CommitPlan`); `ApprovalAuthorizer.CanDecide` cho người chọn (đang dùng `ApproverAwareAuthorizer` sẵn có).
