# TASK-REQ-028-05: `AnswerClarification`, `CancelClarification` và hook huỷ khi đổi loại, huỷ Request, trả backlog

**From Solution:** BE-REQ-SOL-028
**Priority:** P0
**Service:** `request-service`
**File:** `internal/usecase/answer_clarification.go`, `internal/usecase/cancel_clarification.go`, `internal/usecase/clarification_side_effects.go`, `internal/usecase/change_request_type.go` (sửa, của SOL-005), `internal/usecase/cancel_request.go` (sửa, của CR-REQ-006), `internal/usecase/return_to_backlog.go` (sửa, của CR-REQ-006), `internal/usecase/ports.go` (sửa) và test
**Depends on:** TASK-REQ-028-03, 028-04, TASK-REQ-027-05 (`AppendWithinTx`, `ApplyAnswers` dùng `AcceptanceCriteria`), TASK-REQ-007-05 (Solution `superseded`), TASK-REQ-012-05 (Plan thay thế), TASK-REQ-005-06 (`ChangeRequestType`)
**Status:** [ ] TODO

---

## Context

CR-REQ-028 mục 2.4 và bảng 2.1. `AnswerClarification` là lệnh nặng nhất của feature: một transaction chạm `clarifications`, `clarification_questions`, `request_revisions`, `requests`, `solutions`, `decisions`, và outbox. Thứ tự khoá phải trùng thứ tự của `Approve` (SOL-009 mục 1.3: khoá Request rồi mới khoá bản ghi con) để không khoá chết; `CancelPendingForRequest` đã được chốt chạy trong transaction đã khoá Request.

`ApplyAnswers` và `ValidateAnswer` do task 028-02 cung cấp. Giao lặp (at-least-once, người bấm hai lần): `AnswerClarification` lặp cùng nội dung trên Clarification đã `answered` trả kết quả hiện có; nội dung khác thì `REQUEST_CLARIFICATION_ALREADY_ANSWERED`. Kích hoạt lại bước chờ (sinh Solution, `AdvanceExecution`) **không** làm ở đây mà ở consumer của task 028-06 để tách lỗi mạng ra khỏi transaction.

Người trả lời phải thuộc `clarification_assignees` hoặc `role=admin`; danh tính máy bị từ chối (cơ chế nhận biết nguồn máy chưa có, như SOL-010 mục 2.4: hiện `IsMachine` luôn false; hàm kiểm vẫn gọi để sau này bật).

## Việc cần làm

1. `AnswerInput{ClarificationID string; Answers []AnswerItem{QuestionID string; Value json.RawMessage; AcceptDefault bool}; Complete bool; ExpectedVersion int64; ActorID string}`;
   - `AnswerResult{Clarification domain.Clarification; RequestStatus domain.RequestStatus; RequestRevision int; StillMissing bool}`.
2. `AnswerClarification.Execute(ctx, in)`: `RequireTenantID`;
   - `Get` Clarification (`REQUEST_CLARIFICATION_NOT_FOUND`)
   - `Status != open`: nếu `answered` và `in` trùng nội dung đã lưu thì trả bản hiện có, ngược lại `REQUEST_CLARIFICATION_ALREADY_ANSWERED`
   - `expired|cancelled` thì `REQUEST_CLARIFICATION_NOT_OPEN`
   - `now >= DueAt` thì `REQUEST_CLARIFICATION_EXPIRED` (hết hạn lười, chưa quét)
   - người trả lời hợp lệ (`REQUEST_CLARIFICATION_NOT_ASSIGNEE`)
   - `ValidateAnswer` từng câu, `AcceptDefault` chỉ khi có `SuggestedDefault` (`answer_source=default_accepted`).
3. `Complete=false`: một `InTx` cập nhật `UpdateAnswers` (nháp), không đổi Request, không phát sự kiện, trả `StillMissing` theo câu bắt buộc chưa có.
4. `Complete=true` mà thiếu câu bắt buộc: `REQUEST_CLARIFICATION_INCOMPLETE` (kèm danh sách `question_key` thiếu).
5. `Complete=true` đủ: một `InTx`: khoá Request (`repo.Get` + CAS ở bước sau);
   - `ApplyAnswers(ContentFromRequest(r), questions)`
   - `AppendRequestRevision.AppendWithinTx(cause=clarification_answered, ClarificationID, ActorID, ExpectedVersion=r.Version)` (không mở transaction lồng)
   - `MarkAnswered(id, newRevision, now, expectedVersion)`
   - **kiểm sẵn sàng lại** bằng `ReadinessPolicy.Evaluate` trên nội dung mới: thiếu và `Round < MAX_ROUNDS` thì `CreateWithinTx` Clarification mới (`round+1`, `source` cùng, `resume_status` cùng) và **không** chuyển Request (đã `awaiting_information`), outbox `answered` kèm `still_missing=true`
   - thiếu và `Round >= MAX_ROUNDS` thì `TransitionRequest(return_to_backlog, Stage theo ResumeStatus, Category=missing_info, Reason=liệt kê khoá thiếu)`
   - đủ thì tiếp bước 6.
6. Làm cũ các thứ sinh từ revision cũ khi `resume_status` là `analyzing` hoặc `planning`: `SolutionRepository.SupersedeByRequest(requestID, statuses=[proposed])` (đã `approved` giữ nguyên, bất biến), Decision sống của các Solution đó `Supersede()` + `decision_history`, Plan chưa duyệt: không xoá (task `plan` còn là tham chiếu; README v6: "Solution, Plan, Task đã có giữ làm tham chiếu"), chỉ `CancelPendingForRequest("information_provided")` nếu còn Approval `pending` (không nên có vì đã hủy lúc vào).
7. `TransitionRequest(information_provided, ExpectedFrom=awaiting_information, ResumeStatus=c.ResumeStatus, ActorKind=user, ActorID)`;
   - outbox `orca.request.clarification.answered` `{clarification_id, request_id, revision, resume_status, still_missing}` cùng transaction.
   - Payload không chứa câu trả lời.
8. `cancel_clarification.go`: `CancelClarification.Execute(ctx, in{ClarificationID, Reason string; ExpectedVersion int64; ActorID})`: chỉ `reporter`, người hỏi (`created_by`) hoặc admin;
   - `Reason` không rỗng
   - `Cancel` Clarification, `TransitionRequest(information_provided hoặc return_to_backlog)`? **Quyết định:** huỷ thủ công đưa Request về `request_backlog` (`missing_info`, `Reason`) vì không còn thông tin để đi tiếp
   - ghi vào README task và Q1.
9. `clarification_side_effects.go`: hàm dùng chung `CancelOpenClarificationTx(ctx, tx, requestID, reason string) (cancelled bool, err error)` cho ba nơi: `ChangeRequestType` (reason `type_changed`, Request đã được `TransitionRequest(type_change)` nên **gọi trước** transition để Clarification `cancelled` cùng transaction), `CancelRequest` (reason `request_cancelled`), `ReturnToBacklog` từ `awaiting_information` (reason `returned_to_backlog`).
   - Sửa ba use case gọi nó khi `status == awaiting_information`
   - không đổi hành vi khác của chúng.
10. Đảm bảo thứ tự khoá nhất quán: mọi đường (Answer, Cancel, Expire, ChangeType, CancelRequest) khoá **Request trước** rồi mới Clarification.

## Kiểm thử

- `TestAnswer_DraftOnlySavesAnswers_NoRequestChange`
- `TestAnswer_CompleteMissingRequired_Incomplete`
- `TestAnswer_InvalidKindValues_InvalidAnswer` (`single_choice` ngoài `options`)
- `TestAnswer_AcceptDefault_SetsAnswerSource`.
- `TestAnswer_Complete_WritesRevisionAnsweredAndTransitions` (revision tăng đúng một, `request_revisions.cause=clarification_answered`, Request vào `resume_status`, một outbox `answered`)
- `TestAnswer_StillMissing_CreatesRound2_RequestStaysAwaitingInformation`
- `TestAnswer_AtMaxRounds_ReturnsToBacklogMissingInfo`.
- `TestAnswer_SupersedesProposedSolutionAndDecision_WhenResumingAnalyzing`
- `TestAnswer_KeepsApprovedSolution`.
- `TestAnswer_RedeliveryIdempotent_SameBodyReturnsExisting`
- `TestAnswer_DifferentBody_AlreadyAnswered`
- `TestAnswer_AfterDueAt_Expired`
- `TestAnswer_NotAssignee_Rejected`
- `TestAnswer_AdminAllowed`.
- `TestCancelClarification_ReturnsRequestToBacklog`
- `TestChangeRequestType_CancelsOpenClarification`
- `TestCancelRequest_CancelsOpenClarification`
- `TestReturnToBacklog_FromAwaitingInformation_CancelsClarification`.
- `TestAnswer_TransactionRollsBackOnTransitionFailure` (fake `TransitionRequest` lỗi: không còn revision, không `answered`).
- `TestLockOrder_RequestBeforeClarification` (test thứ tự gọi trên fake repo ghi lại trình tự).
- Integration hai dialect: đa bảng rollback; `Answer` đồng thời hai lần cùng nội dung (một revision duy nhất); `Answer` và `Expire` đua (một thắng).
- Lệnh: `cd /opt/repos/orca/backend-go && go test ./services/request-service/internal/usecase/... -run 'AnswerClarification|CancelClarification|ChangeRequestType|CancelRequest|ReturnToBacklog' && go test -tags=integration ./services/request-service/... -run 'AnswerClarification|ClarificationRace'`.

## Tiêu chí hoàn thành

- [ ] Trả lời đủ: Clarification `answered`, `content_revision` tăng đúng một, Request vào `resume_status`, `request_revisions` có dòng `clarification_answered`.
- [ ] Solution `proposed` cũ thành `superseded`, Solution `approved` giữ nguyên; Decision tương ứng `superseded`.
- [ ] Giao lặp không tạo revision hay sự kiện thứ hai.
- [ ] `type_change`, `cancel`, `return_to_backlog` từ `awaiting_information` hủy Clarification `open` cùng transaction.
- [ ] Vượt vòng tối đa về `request_backlog` với `missing_info`.
- [ ] Không có lỗi thứ tự khoá trong test đua.

## Rủi ro và lưu ý

- Transaction dài (nhiều bảng) tăng khả năng xung đột CAS; `Execute` ngoài cùng được retry bởi caller chỉ khi không nằm trong transaction khác (SOL-003 C1).
- Quyết định "huỷ thủ công thì về backlog" là suy luận; CR không nói rõ. Cần xác nhận (Q1).
- `SupersedeByRequest` phải bỏ qua Solution `approved` để giữ bất biến (SOL-027); test kiểm.
- Sửa ba use case của CR-REQ-005/006 sau khi đã merge: giữ commit nhỏ, chạy lại test của chúng.
