# TASK-REQ-028-04: `RequestClarification`, kiểm tra sẵn sàng trong `ConfirmRequestType` và `WaiveReadiness`

**From Solution:** BE-REQ-SOL-028
**Priority:** P0
**Service:** `request-service`
**File:** `internal/usecase/request_clarification.go`, `internal/usecase/waive_readiness.go`, `internal/usecase/get_request_readiness.go`, `internal/usecase/confirm_request_type.go` (sửa, của SOL-005), `internal/usecase/ports.go` (sửa) và test
**Depends on:** TASK-REQ-028-02, 028-03, TASK-REQ-027-05 (`AppendWithinTx`), TASK-REQ-005-05 (`ConfirmRequestType`), TASK-REQ-009-04 (`CancelPendingForRequest`), TASK-REQ-010-03 (`ResolveApproverPolicy`, nếu đã có; nếu chưa, dùng cài tạm)
**Status:** [x] DONE

---

## Context

CR-REQ-028 mục 2.1, 2.3, 2.5, 2.6. Trích từ TASK-REQ-005-05: `ConfirmRequestType.Execute` hiện (a) kiểm hợp lệ, (b) trong `InTx` đặt `Type, Size, Urgency`, `TypeSource`, ghi `request_type_history` khi khác đề xuất AI, (c) `repo.Update` CAS, (d) `approvals.Approve`, (e) `transition.Execute(type_confirmed, ExpectedFrom=awaiting_type_confirmation)`, (f) outbox `type_confirmed`. Task này chèn bước kiểm sẵn sàng giữa (c) và (e) và đổi trigger khi chưa sẵn sàng (SOL-028 C5): vẫn ghi `type`, `size`, `urgency`, approval `request_type`, outbox `type_confirmed` (người đã xác nhận loại, không phải xác nhận lại sau khi trả lời).

`RequestClarification` có **ba nguồn gọi**: (1) nội bộ từ `ConfirmRequestType` (`readiness`), (2) RPC người dùng từ người duyệt hoặc admin (`solution_open_question`, `plan_assumption`, `manual`), (3) nội bộ từ CR-REQ-013/029 (`task_blocked`, chỉ khi Request `executing`). Mọi nguồn dùng cùng use case lõi `CreateWithinTx` để vào `awaiting_information` đúng một cách.

## Việc cần làm

1. `ports.go`: `ClarificationRecipientResolver{Resolve(ctx, req domain.Request, source domain.ClarificationSource) ([]domain.Assignee, error)}` (mặc định `reporter`; mở rộng `team:` và `role:admin` thuộc task 028-06, ở đây dùng cài tạm trả `[{reporter}]`);
   - `ClarificationDeadlinePolicy{DueAt(source, urgency, now) time.Time}` (cài tạm bọc `domain.DefaultDue`).
2. `request_clarification.go`: `RequestClarificationInput{RequestID string; Source domain.ClarificationSource; SourceRef string; Questions []QuestionInput; Assignees []domain.Assignee; ActorID string; ActorKind domain.ActorKind; ExpectedVersion int64}`;
   - `RequestClarification.Execute(ctx, in) (domain.Clarification, error)` mở `InTx` và gọi `CreateWithinTx`.
   - `CreateWithinTx(ctx, in)`: `RequireTenantID`
   - `Get` Request
   - kiểm `status` thuộc sáu trạng thái nguồn (`REQUEST_CLARIFICATION_STATE_NOT_ALLOWED`, kèm `status`)
   - kiểm quyền theo nguồn (`readiness`/`task_blocked` chỉ do hệ thống gọi, tức `ActorKind=system`; `solution_open_question`/`plan_assumption`/`manual` do người có quyền ghi mức Request hoặc admin: tạm `reporter_id` hoặc `role=admin`, ghi chú Q1)
   - `ValidateQuestion` từng câu (1..20 câu, `QuestionKey` duy nhất)
   - `ResumeStatusFor(flow, source, status)`
   - tạo `Clarification` (`Round` bằng `max(round)+1` của chuỗi Clarification trước của cùng `source` và `source_ref`, mặc định 1; `AskedRequestRevision = request.ContentRevision`; `DueAt` từ chính sách)
   - `CancelPendingForRequest(why="information_required")` khi trạng thái nguồn có Approval `pending`
   - `repo.Insert`
   - `TransitionRequest(information_required, ExpectedFrom=status hiện tại, ActorKind, ActorID, Reason="clarification:<seq>")`
   - outbox `orca.request.clarification.requested` `{clarification_id, request_id, display_id, source, resume_status, due_at, round}` (không có nội dung câu hỏi) cùng transaction.
3. Giao lặp: nếu đã có Clarification `open` cùng `source` và `source_ref` và danh sách `question_key` giống nhau thì trả Clarification đó (không tạo thứ hai);
   - khác thì `REQUEST_CLARIFICATION_STATE_NOT_ALLOWED`.
4. `confirm_request_type.go` (sửa): sau bước (c) gọi `ReadinessPolicy.Evaluate(type, ContentFromRequest(r))`;
   - `Ready` thì đi tiếp như cũ
   - không `Ready` thì (i) nếu Request này đã có `round >= REQUEST_CLARIFICATION_MAX_ROUNDS` (mặc định 3, đọc từ cấu hình) thì `TransitionRequest(return_to_backlog, Stage=classification, Category=missing_info, Reason="thiếu: <path,...>")`, (ii) ngược lại `CreateWithinTx(source=readiness, Questions=QuestionBuilder.Build(...), Assignees=[reporter])` thay cho `type_confirmed`.
   - Outbox `type_confirmed` và `approvals.Approve` vẫn chạy ở cả hai nhánh.
5. `get_request_readiness.go`: `GetRequestReadiness.Execute(ctx, requestID) (ReadinessReport, error)`: chỉ đọc, tính lại bằng `ReadinessPolicy` (frontend hiển thị "còn thiếu gì" mà không tạo Clarification).
6. `waive_readiness.go`: `WaiveReadinessInput{RequestID, Reason string; ExpectedVersion int64}`;
   - chỉ `role=admin` (`REQUEST_FORBIDDEN`)
   - `reason` không rỗng (`REQUEST_REASON_REQUIRED`)
   - loại `hotfix|security|ops_request` bị `REQUEST_READINESS_WAIVE_FORBIDDEN`
   - chỉ khi `awaiting_type_confirmation` hoặc `awaiting_information` (từ `readiness`)
   - thực hiện: nếu có Clarification `open` thì `Cancel(reason="waived")`
   - `AppendRequestRevision(cause=edited)` với ghi chú waiver trong `snapshot.meta.waiver`
   - `TransitionRequest(type_confirmed hoặc information_provided với ResumeStatusFor)`
   - outbox `type_confirmed`/`status_changed` có `reason`.
   - Audit qua `auditclient.Append`.
7. Cấu hình: `REQUEST_CLARIFICATION_MAX_ROUNDS` (3), `REQUEST_READINESS_AI_DRAFT` (false); đọc ở `config` (TASK-REQ-001-01), ghi README.
8. Không có nháp AC bằng AI trong task này (cờ tắt); chỗ nối để lại là `QuestionBuilder` nhận `hints.AIDraft` rỗng.

## Kiểm thử

- `TestRequestClarification_FromEachOfSixStates_GoesAwaitingInformation` (bảng sáu nguồn)
- `TestRequestClarification_FromTerminalOrBacklog_Rejected`
- `TestRequestClarification_CancelsPendingApproval_WhenFromAwaitingApproval`
- `TestRequestClarification_PayloadHasNoQuestionText`
- `TestRequestClarification_IdempotentSameKeys`
- `TestRequestClarification_TwoConcurrent_OneWins` (fake repo trả lỗi chỉ mục duy nhất)
- `TestRequestClarification_ActorNotAllowedPerSource`.
- `TestConfirmRequestType_BugMissingReproAndAC_GoesAwaitingInformation` (đủ: một Clarification `open`, câu hỏi cho từng khoá thiếu, có `type_confirmed` và `clarification.requested`, Approval `request_type` đã `approved`)
- `TestConfirmRequestType_ReadyRequest_GoesAnalyzing_AsBefore`
- `TestConfirmRequestType_ExceedsMaxRounds_ReturnsToBacklogMissingInfo`
- `TestConfirmRequestType_NonBlockingMissing_StillReady`.
- `TestWaiveReadiness_AdminOnly`
- `TestWaiveReadiness_ForbiddenForHotfixSecurityOps`
- `TestWaiveReadiness_ReasonRequired`
- `TestWaiveReadiness_CancelsOpenClarification_AndResumes`
- `TestWaiveReadiness_WritesRevisionAndAudit`.
- `TestGetRequestReadiness_ReadOnly_NoWrites`.
- Integration hai dialect cho nhánh đầu cuối `ConfirmRequestType` → `awaiting_information` ở task 028-08.
- Lệnh: `cd /opt/repos/orca/backend-go && go test ./services/request-service/internal/usecase/... -run 'RequestClarification|ConfirmRequestType|WaiveReadiness|GetRequestReadiness'`.

## Tiêu chí hoàn thành

- [x] `bug` thiếu `repro_steps` và AC: Request vào `awaiting_information`, không qua `analyzing`; đủ dữ liệu thì vào `analyzing` như cũ (hồi quy test 005-05).
- [x] Hai `RequestClarification` đồng thời cho một Request: đúng một thành công.
- [x] Approval `pending` bị hủy khi vào `awaiting_information` từ trạng thái duyệt.
- [x] Waive bị cấm cho ba loại an toàn và cho người không phải admin; thành công có dấu vết.
- [x] Payload sự kiện không chứa nội dung câu hỏi.

## Rủi ro và lưu ý

- Sửa `ConfirmRequestType` đã merge: giữ hồi quy 005-05 xanh, và đừng làm vòng hỏi lại trong vòng lặp vô hạn (giới hạn vòng là bảo vệ chính).
- Người gọi nội bộ `task_blocked` chưa tồn tại (CR-REQ-013/029): kiểm thử bằng bên gọi giả; hành vi "task bị chặn quay về `open`" chưa kiểm chứng.
- Quyền hỏi tay ở mức Request chưa chốt (README mục 8); quy tắc tạm có thể quá rộng hoặc hẹp.
- `Reason` của câu hỏi sinh bằng tiếng Việt cố định; nếu cần đa ngôn ngữ, đổi sang mã lý do và để frontend dịch.
