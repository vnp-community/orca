# TASK-REQ-028-06: Consumer kích hoạt lại, vòng hết hạn, nhắc và thông báo (`notification-service`)

**From Solution:** BE-REQ-SOL-028
**Priority:** P0
**Service:** `request-service` · `notification-service`
**File:** `request-service/internal/adapter/eventbus/clarification_resume_consumer.go`, `request-service/internal/usecase/resume_after_clarification.go`, `request-service/internal/usecase/expire_clarifications.go`, `request-service/internal/usecase/remind_clarifications.go`, `request-service/internal/usecase/clarification_recipients.go`, `request-service/cmd/server/main.go` (sửa), `notification-service/internal/adapter/eventbus/consumer.go` (sửa), `notification-service/internal/domain/notification_event.go` (sửa) và test
**Depends on:** TASK-REQ-028-03, 028-05, TASK-REQ-007-05 (`GenerateSolution` với `feedback`), TASK-REQ-013-04 (`AdvanceExecution`), TASK-REQ-010-04, 010-05, 010-06 (mở rộng người nhận, hàng thông báo approval, vòng nhắc), TASK-REQ-006-xx (`ReturnToBacklog`)
**Status:** [x] DONE

---

## Context

CR-REQ-028 mục 2.4 bước 5, 2.7. Ba phần việc nền:

1. **Kích hoạt lại sau `information_provided`:** consumer bền của `request-service` đọc `orca.request.request.status_changed` có `trigger=information_provided`, khử trùng bằng `processed_events` (PK `(tenant_id, event_id)` theo BE-REQ-SOL-001 C2; đọc thư mục migrations để biết số). `to=analyzing` thì `GenerateSolution` nội bộ với `feedback="clarification:<id>"` (SOL-007 mục E: đường sinh lại bằng phản hồi chấp nhận `awaiting_analysis_approval` với `feedback`; ở đây Request đang `analyzing`, nên `GenerateSolution` bước 3 chấp nhận trực tiếp); `to=planning` không tự chạy (PROPOSE không lưu gì, người dùng bấm lại ở UI); `to=executing` thì `AdvanceExecution` (SOL-013 mục 2.4, idempotent). Tự chạy hay để người bấm là Q3 của SOL-028 (đề xuất: sinh Solution có, Plan không).
2. **Hết hạn:** vòng quét 60 giây `ExpireClarifications`: `ClaimExpired` (task 028-03), rồi `ReturnToBacklog(stage, category=missing_info, reason="clarification_expired", actor_kind=system)` của CR-REQ-006, stage theo `resume_status`/`source` (`readiness` thành `classification`; `analyzing` thành `analysis`; `planning` thành `plan`; `executing` thành `task`). Vì `ClaimExpired` đặt `expired` rồi mới trả về, thực hiện **cùng một transaction** cho mỗi Clarification: nếu `ReturnToBacklog` lỗi thì Clarification phải trở lại `open` để lần quét sau thử lại (không để `expired` mà Request vẫn `awaiting_information`).
3. **Thông báo:** `notification-service` chỉ gửi cho người có tên trong payload (`EventPayload.user_ids`); `request-service` phải mở rộng `reporter`, `team:<id>` (bằng `tenant-service.ListTeamMembers`) và `role:admin` (bằng `auth-service.ListUsers` lọc `role=admin`) giống TASK-REQ-010-04 (dùng lại `RecipientExpander` nếu đã có, **không viết lần hai**). Đã đọc `notification-service/internal/adapter/eventbus/consumer.go` (danh sách `Subjects`, `SubjectBinding{StreamName, Subject, Durable}`) và `internal/domain/notification_event.go` (`subjectRules`: `Type`, `Title`, `Body`, `Severity`, `Channels`; `defaultRule` cho subject không có luật, tức WS only).

**Xung đột tệp:** TASK-REQ-010-05 sửa chính hai tệp của `notification-service` này (hàng approval). Làm task này **sau** 010-05 hoặc cùng PR để không xung đột.

## Việc cần làm

1. `resume_after_clarification.go`: `ResumeAfterClarification.Handle(ctx, ev StatusChangedEvent) error`: `trigger != information_provided` thì bỏ qua;
   - đặt tenant từ `Event.TenantID` (`tenant.WithTenantID`)
   - khử trùng `processed_events` (đánh dấu **sau** khi xử lý thành công, để lỗi mạng được giao lại)
   - nạp Clarification vừa `answered` mới nhất của Request (`ListByRequest`, `status=answered`, `answered_at` lớn nhất) để lấy `id`
   - `to=analyzing`: gọi `GenerateSolution.Execute(ctx, {RequestID, Feedback: "clarification:"+id, IdempotencyKey: "clr-"+id, ActorKind: system})`
   - `to=executing`: `AdvanceExecution.Execute(ctx, requestID)`
   - lỗi `REQUEST_SOLUTION_NO_CONNECTION` (không có dev server) chỉ ghi log và metric, không thử lại vô hạn (người dùng bấm sinh lại).
2. `clarification_resume_consumer.go`: đăng ký consumer bền (`Durable: "request-service-clarification-resume"`) trên subject `orca.request.request.status_changed` của stream `REQUEST`;
   - ack sau khi `Handle` trả nil
   - lỗi tạm thì nak với backoff
   - theo mẫu consumer của TASK-REQ-005-03 (`status_changed` consumer của phân loại) để chỉ có **một** subscriber cho subject này nếu `common/eventbus` cho phép nhiều handler (đọc `common/eventbus` lúc làm; nếu chỉ một handler mỗi durable thì dùng durable riêng).
3. `expire_clarifications.go`: `ExpireClarifications.RunLoop(ctx, interval time.Duration)` (60 giây, `time.Ticker`, dừng khi `ctx` hủy);
   - `RunOnce(ctx, batch int) (int, error)`: `ClaimExpired` + `ReturnToBacklog` trong một `InTx` mỗi Clarification
   - outbox `orca.request.clarification.expired` `{clarification_id, request_id, display_id, source, due_at}`
   - hai instance chạy cùng lúc không xử lý trùng nhờ `SKIP LOCKED`.
4. `remind_clarifications.go`: `RemindClarifications.RunOnce`: `ClaimNeedingReminder` (một lần ở 50% hạn, `reminded_at`), phát `orca.request.clarification.requested` với `reminder=true` (cùng subject để thông báo nhắc, không subject mới) kèm người nhận;
   - dùng chung vòng với expire (một ticker, hai bước) để giảm goroutine.
5. `clarification_recipients.go`: `ClarificationRecipients.Resolve(ctx, c domain.Clarification, req domain.Request) ([]string, error)` gọi `RecipientExpander` (010-04) cho `assignees`;
   - loại trùng, không có ai thì ghi log cảnh báo và để Clarification vẫn tạo (người dùng thấy ở UI)
   - payload thông báo dựng bằng `NotificationPayloadBuilder`: `{user_ids, title, body, deep_link}` với `body` ngắn không chứa nội dung câu hỏi (thông báo được lưu).
6. `notification-service`: thêm vào `Subjects` hai binding `{StreamName: "REQUEST", Subject: "orca.request.clarification.requested", Durable: "notification-service-request-clarification-requested"}` và `{StreamName: "REQUEST", Subject: "orca.request.clarification.expired", Durable: "notification-service-request-clarification-expired"}` (tên stream phải khớp `EnsureStream` của `request-service`, TASK-REQ-001-05);
   - thêm vào `subjectRules` `"orca.request.clarification.requested": {Type: "request_clarification_requested", Title: "Request cần bổ sung thông tin", Severity: SeverityWarning, Channels: [ChannelDeliveryWS, ChannelDeliveryPush]}` và `"orca.request.clarification.expired": {Type: "request_clarification_expired", Title: "Yêu cầu bổ sung thông tin đã hết hạn", Severity: SeverityWarning, Channels: [ChannelDeliveryWS]}`.
   - Giữ nguyên các binding approval của 010-05.
7. `main.go`: đăng ký consumer, `RunLoop` expire/remind sau cờ `REQUEST_CLARIFICATION_WORKERS_ENABLED` (mặc định `true`);
   - tắt khi `request_flow_enabled=false` (CR-REQ-025).
8. Metric: `request_clarification_open` (gauge), `request_clarification_expired_total`, `request_clarification_answer_seconds` (histogram từ `created_at` đến `answered_at`; CR-REQ-024 đặt tên chuẩn, đọc TASK-REQ-024-07 để khớp).

## Kiểm thử

- `TestResume_AnalyzingTriggersGenerateSolutionWithFeedback`
- `TestResume_PlanningDoesNotAutoRun`
- `TestResume_ExecutingCallsAdvanceExecution`
- `TestResume_OtherTriggersIgnored`
- `TestResume_RedeliveryNoSecondRun` (cùng `event_id`)
- `TestResume_TransientErrorNotAckedThenRetries`
- `TestResume_NoConnectionDoesNotLoop`.
- `TestExpire_RunOnce_ReturnsRequestToBacklogWithMissingInfo`
- `TestExpire_StageMapping_AllSources`
- `TestExpire_TwoInstances_NoDoubleProcessing`
- `TestExpire_ReturnToBacklogFailure_KeepsClarificationOpen`
- `TestExpire_AnswerAfterDueBeforeSweep_Rejected` (kiểm tra ở 028-05).
- `TestRemind_OncePerClarification`
- `TestRemind_AtHalfDue`.
- `TestRecipients_ExpandsReporterTeamRole_Dedup`
- `TestRecipients_PayloadHasNoQuestionText`.
- `notification-service`: `TestSubjects_HasClarificationBindings` (kiểm `Durable` và tên stream), `TestTranslateEvent_ClarificationRequested_UsesRuleAndPayloadUserIDs`, `_ClarificationExpired`, `TestApprovalBindingsUnchanged` (hồi quy của 010-05).
- Integration hai dialect: hai worker `ClaimExpired` đồng thời; quét hết hạn với dữ liệu đa tenant; golden payload gửi sang `notification-service`.
- Lệnh: `cd /opt/repos/orca/backend-go && go test ./services/request-service/internal/usecase/... ./services/request-service/internal/adapter/eventbus/... -run 'Resume|Expire|Remind|Recipients' && go test ./services/notification-service/... -run 'Clarification|Subjects|TranslateEvent' && go test -tags=integration ./services/request-service/... -run 'ExpireClarifications'`.

## Tiêu chí hoàn thành

- [x] `information_provided` về `analyzing` tạo đúng một run sinh Solution dù sự kiện giao lặp.
- [x] Quá hạn: Clarification `expired`, Request `request_backlog` (`missing_info`, stage đúng); `ReturnToBacklog` lỗi thì Clarification vẫn `open`.
- [x] Hai instance quét không xử lý trùng.
- [x] Thông báo tới đúng người nhận, không chứa nội dung câu hỏi; hai binding và hai luật có test.
- [x] Hồi quy: binding và luật approval của TASK-REQ-010-05 không đổi.

## Rủi ro và lưu ý

- Tên stream phải khớp chính xác (`REQUEST`) giữa `EnsureStream` của `request-service` và `Subjects` của `notification-service`; sai tên làm thông báo im lặng (không lỗi). Test `Subjects` bắt.
- Người báo cáo từ Jira có thể không có tài khoản Orca: `user_ids` rỗng, Clarification hết hạn không ai thấy; ngoài phạm vi (Q4), nên ghi cảnh báo log.
- `GenerateSolution` tự chạy tốn một lượt gọi model mỗi lần trả lời; người dùng không kiểm soát. Cờ `REQUEST_CLARIFICATION_AUTO_REGENERATE` (mặc định `true`) cho phép tắt.
- Nếu `common/eventbus` không cho hai handler trên cùng subject, cần durable riêng; kiểm lúc làm.
