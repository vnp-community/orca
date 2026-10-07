# TASK-REQ-005-04: Use case `ProposeRequestClassification` (AI gọi ngoài giao dịch, ghi kết quả atomically)

**From Solution:** BE-REQ-SOL-005
**Priority:** P0
**Service:** `request-service`
**File:** `internal/usecase/propose_request_classification.go`, `internal/usecase/propose_request_classification_test.go`, `internal/usecase/approval_noop.go`, `internal/usecase/execution_guard_noop.go`, `internal/usecase/ports.go` (sửa: `ApprovalRecorder`, `ApprovalCanceller`, `ExecutionGuard`)
**Depends on:** TASK-REQ-005-01, 005-02, 005-03, TASK-REQ-003-03
**Status:** [x] DONE

---

## Context

CR-REQ-005 mục 2.2 (sáu bước), SOL-005 mục B (đã sửa: `MarkProcessed` và `classification_attempts` trong giao dịch). `TransitionRequest.Execute` lồng được. `RequestTypeHistoryRepository.Append`. Sự kiện `orca.request.request.classified` `{request_id, type, size, urgency, confidence, failed}`. Cổng no-op tới khi CR-REQ-009, 011 thay.

## Việc cần làm

1. `ports.go`: `ApprovalRecorder{RequestTypeApproval(ctx, requestID string) error; Approve(ctx, requestID string, actorID string) error}`, `ApprovalCanceller{CancelPending(ctx, requestID, reason string) error}`, `ExecutionGuard{HasActiveExecution(ctx, requestID string) (bool, error)}`. Bản no-op `NoopApprovalRecorder`, `NoopApprovalCanceller`, `NoopExecutionGuard` (trả nil, nil, false).
2. `ProposeRequestClassification{repo, history, processed, classifier, transition, approvals, tx, outbox, clock}`; `Execute(ctx, in ProposeInput) error` với `ProposeInput{RequestID, EventID, Trigger string; Manual bool}` (`Manual` do `ClassifyRequest`).
3. Bước 1 (đọc, không giao dịch): `repo.Get`; `status` ngoài {`classifying`, `awaiting_type_confirmation`} thì trả nil (consumer) hoặc `REQUEST_NOT_CLASSIFIABLE` (`Manual`); `ClassificationAttempts >= MaxClassificationAttempts`: `Manual` trả `REQUEST_CLASSIFICATION_LIMIT`; consumer không gọi AI, đi thẳng nhánh thất bại với lý do "classification limit".
4. Bước 2: `classifier.Classify` (ngoài giao dịch, ctx tenant và user = `reporter_id`).
5. Bước 3 (`tx.InTx`): nếu `EventID != ""` thì `processed.MarkProcessed`; đã xử lý thì thoát. `repo.Get` lại (status có thể đã đổi: không còn trong tập hợp lệ thì thoát). Thành công: đặt trường đề xuất, `type_source=ai`, `ClassificationAttempts+1`, `repo.Update(CAS)`; `history.Append(from=loại trước hoặc rỗng, to, ActorKind=ai)`; nếu đang `classifying` thì `transition.Execute(proposal_ready, ExpectedFrom=classifying, ActorKind=ai)`; `approvals.RequestTypeApproval`; outbox `classified` `failed=false`. Thất bại: nếu chưa có đề xuất trước đó đặt `classification_reason` ngắn (không quá 500 rune: `ErrNoDevServer` -> "no dev server connected", `ErrProposalInvalid` -> "invalid classifier output", timeout -> "classifier timeout"); `ClassificationAttempts+1`; `proposal_ready` nếu `classifying`; outbox `classified` `failed=true`; **không** đặt `type`.
6. `ClassifyRequest` đang `awaiting_type_confirmation` mà thất bại: giữ đề xuất cũ nguyên vẹn (chỉ tăng `attempts`, phát `failed=true`).
7. Hàm `ClassifyNow(ctx, requestID)` bọc `Execute{Manual:true}` và trả `Request` mới cho RPC.

## Kiểm thử

Unit với fake (classifier, repo, history, processed, tx):
- `TestPropose_Success_WritesAllAndTransitions` (type, size, urgency, confidence, reason, `type_source=ai`, một dòng lịch sử ai, status `awaiting_type_confirmation`, một sự kiện `classified`).
- `TestPropose_ClassifierFailsTwice_StillProposalReady` (`type` rỗng, `failed=true`).
- `TestPropose_NoDevServer_FailurePath`.
- `TestPropose_RedeliveryProcessedOnce` (cùng `EventID` hai lần: một đề xuất).
- `TestPropose_StatusMovedMeanwhile_NoWrite` (đổi trạng thái giữa bước 1 và 3).
- `TestPropose_RerunKeepsPreviousProposalOnFailure`.
- `TestPropose_LimitReached_ManualRejected` (lần thứ 6, kể cả khi 5 lần trước thất bại) và `_ConsumerGoesFailurePath`.
- `TestPropose_InjectedContentCannotChangeOutputBeyondEnum` (classifier giả trả `type=rm -rf`: bị loại, nhánh thất bại).
- `TestPropose_ApprovalNoopDoesNotBreakTx`.
Lệnh: `go test ./services/request-service/internal/usecase/... -run Propose`.

## Tiêu chí hoàn thành

- [x] Đề xuất hợp lệ ghi đủ trường, một lịch sử `ai`, một sự kiện `classified`.
- [x] AI thất bại hai lần: `awaiting_type_confirmation`, `type` rỗng, `failed=true`.
- [x] Giao lặp: chỉ một đề xuất.
- [x] Lần AI thứ 6 bị chặn kể cả khi thất bại liên tiếp.
- [x] Không gán `.Status` ngoài `transition_request.go` (test kiến trúc TASK-REQ-003-06 vẫn xanh).

## Rủi ro và lưu ý

- AI chạy ngoài giao dịch nên Request có thể bị sửa giữa chừng (người đổi `reopen` hay `cancel`): bước 3 đọc lại và thoát; kết quả AI bị bỏ (chấp nhận, chi phí đã tiêu).
- `ClassifyRequest` thủ công và consumer chạy đồng thời: CAS ở `Update` quyết định một bên thắng; bên thua nhận `REQUEST_VERSION_CONFLICT` (RPC) hoặc thoát nhẹ (consumer).
