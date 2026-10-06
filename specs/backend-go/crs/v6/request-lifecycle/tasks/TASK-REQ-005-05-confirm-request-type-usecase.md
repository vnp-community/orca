# TASK-REQ-005-05: Use case `ConfirmRequestType`

**From Solution:** BE-REQ-SOL-005
**Priority:** P0
**Service:** `request-service`
**File:** `internal/usecase/confirm_request_type.go`, `internal/usecase/confirm_request_type_test.go` (mới)
**Depends on:** TASK-REQ-005-04 (cổng no-op), TASK-REQ-005-01
**Status:** [ ] TODO

---

## Context

CR-REQ-005 mục 2.3. Chỉ người dùng gọi (`ActorKind=user`; tool MCP phải qua chính sách ở CR-REQ-017). Luật xác nhận ở `domain.ValidateConfirmation` (TASK-REQ-005-01). `type_source` cho biết giá trị `type` hiện tại là của AI hay người. `TransitionRequest(type_confirmed)` đưa tới `analyzing` hoặc `planning` theo `FlowFor`.

## Việc cần làm

1. `ConfirmInput{RequestID, Type, Size, Urgency, Reason string; ExpectedVersion int64; ActorID string}`; `ConfirmRequestType{repo, history, transition, approvals, tx, outbox}`; `Execute(ctx, in) (domain.Request, error)`.
2. Parse `Type` (`ParseRequestType`), `Size` (rỗng cho phép), `Urgency` (rỗng nghĩa là giữ giá trị hiện có); `ValidateConfirmation`.
3. `InTx`: `repo.Get`; giao lặp: nếu `status` đã qua `awaiting_type_confirmation` và `r.Type == Type`: trả `r` thành công, không ghi gì; nếu `status != awaiting_type_confirmation` mà khác: `REQUEST_TRANSITION_NOT_ALLOWED`; `ExpectedVersion != 0 && != r.Version` thì `REQUEST_VERSION_CONFLICT`.
4. Đặt `Type`, `Size`, `Urgency`; `TypeSource`: `ai` nếu `r.TypeSource == ai && r.Type == Type` (người chấp nhận đề xuất), ngược lại `human`; nếu khác đề xuất AI (hoặc không có đề xuất) thì `history.Append(from=đề xuất AI hoặc rỗng, to=Type, ActorKind=user, Reason)`; **không** ghi lịch sử khi người chỉ chấp nhận.
5. `repo.Update(CAS)`; `approvals.Approve(ctx, requestID, ActorID)`; `transition.Execute(type_confirmed, ExpectedFrom=awaiting_type_confirmation, ActorKind=user, ActorID)`; outbox `orca.request.request.type_confirmed` `{request_id, type, size, urgency, type_source, actor_id}`.
6. `ActorKind` khác `user` bị từ chối (`REQUEST_ACTOR_NOT_ALLOWED` thêm vào `request_classification_errors.go`, `PermissionDenied`).

## Kiểm thử

- `TestConfirm_AcceptAIProposal_NoHistoryRow_TypeSourceAI`.
- `TestConfirm_Override_HistoryRowAndTypeSourceHuman`.
- `TestConfirm_NoAIProposal_ManualPick_HistoryRow`.
- `TestConfirm_BugWithoutSize_Rejected`, `_RefactorWithoutSize_Rejected`, `_HotfixNormalUrgency_Rejected`, `_SecurityWithoutReason_Rejected`.
- `TestConfirm_GoesToAnalyzingOrPlanning` (từng loại: `task`, `docs`, `ops_request` sang `planning`; còn lại `analyzing`).
- `TestConfirm_IdempotentSecondCall_NoExtraEvent`.
- `TestConfirm_StaleVersion`, `_WrongStatus`, `_NonUserActorRejected`.
- Integration hai dialect: hai `Confirm` đồng thời khác loại: một thắng, bên thua `REQUEST_VERSION_CONFLICT` hoặc `REQUEST_TRANSITION_NOT_ALLOWED` (chạy ở TASK-REQ-005-08).
Lệnh: `go test ./services/request-service/internal/usecase/... -run Confirm`.

## Tiêu chí hoàn thành

- [ ] Confirm hợp lệ vào `analyzing`/`planning` đúng loại.
- [ ] Thiếu `size` với `bug`, `refactor` và `hotfix` + `normal` bị từ chối.
- [ ] Hai lần cùng nội dung: lần hai thành công, không dòng lịch sử, không sự kiện.
- [ ] Chấp nhận đề xuất AI không ghi lịch sử.

## Rủi ro và lưu ý

- `urgency` rỗng giữ giá trị hiện có: tránh ghi đè `urgent` AI đề xuất khi UI không gửi.
- Approval no-op: khi CR-REQ-009 thay, `RequestTypeApproval` và `Approve` phải cùng giao dịch; giữ chữ ký nhận ctx giao dịch.
