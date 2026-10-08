# TASK-REQ-009-04: Use case Approval, `SubjectHandler` và đăng ký

**From Solution:** [BE-REQ-SOL-009](../solutions/BE-REQ-SOL-009-generic-approval-domain-and-api.md) mục E, F
**Priority:** P0
**Service/Area:** `request-service` / usecase
**File:** `backend-go/services/request-service/internal/usecase/approval_subject_handler.go` (mới), `approver_policy_ports.go` (mới), `open_approval.go` (mới), `decide_approval.go` (mới), `cancel_approval.go` (mới), `cancel_pending_approvals_for_request.go` (mới), `list_approvals.go` (mới), `list_pending_approvals_for_user.go` (mới) và các `_test.go`
**Depends on:** TASK-REQ-009-03; CR-REQ-003 (`TransitionRequest`, `FlowFor`, đọc Request có khoá)
**Status:** [x] DONE (đã kiểm chứng 2026-10-08: `go test ./internal/usecase/... -run Approval` + hợp đồng DB `RunApprovalFlowContract` trên Postgres và MySQL thật)

## Context

- `TransitionRequest` và `FlowFor` do CR-REQ-003 cung cấp; chưa có mã. Task này khai báo port tối thiểu (`RequestGate` với `LockRequest(tx, tenant, id)`) và dùng fake trong test; wiring thật làm khi CR-REQ-003 xong.
- Mẫu hết hạn lười: `EffectiveStatus` trong `mcp-service/internal/domain/approval.go`.
- Thứ tự khoá cố định Request rồi Approval (solution mục 1.3), áp dụng cho `DecideApproval` và `CancelPendingForRequest`.

## Việc cần làm

1. `approval_subject_handler.go`: interface `SubjectHandler` (bốn hàm như CR mục 2.4), `SubjectHandlerRegistry` với `Register`, `Get(SubjectType)`, `MustCoverAll() error` (liệt kê mọi `AllSubjectTypes` thiếu handler), và `NoopSubjectHandler{Reason string}` ghi log mỗi lần gọi; `main.go` (task 06) từ chối Noop ở production.
2. `approver_policy_ports.go`: `ApproverPolicy.Resolve(ctx, req, st) (ApproverDecision, error)` và `ApprovalAuthorizer.CanDecide(ctx, req, ap) error`, kèm `TemporaryApproverPolicy` (`due_at=nil`, `self_approval_allowed=true`) và `TemporaryApprovalAuthorizer` (admin hoặc `reporter_id`), đánh dấu "thay bởi BE-REQ-SOL-010".
3. `OpenApproval.Execute(ctx, tx, in)`: theo solution mục E; trùng `pending`: trả dòng cũ nếu `subject_digest` bằng, ngược lại `REQUEST_APPROVAL_PENDING_EXISTS`; ghi outbox `orca.request.approval.requested` qua `common/outbox`.
4. `DecideApproval.Approve`/`Reject`: transaction, khoá Request rồi `GetForUpdate` Approval; thứ tự kiểm: `EffectiveStatus` (ghi `expired` và trả `REQUEST_APPROVAL_EXPIRED`), `CanDecide`, `expected_digest`, `stage`, chuyển trạng thái domain, `UpdateDecision` (0 dòng: đọc lại để chọn `ALREADY_DECIDED` hay `VERSION_CONFLICT`), `handler.OnApproved/OnRejected`, outbox `approval.decided`. Lặp cùng quyết định cùng người trên Approval đã đóng thì trả kết quả hiện có và không phát sự kiện.
5. `CancelApproval` (người yêu cầu hoặc admin) và `CancelPendingApprovalsForRequest(tx, tenant, requestID, why)` (caller giữ khoá Request); gọi `OnClosedWithoutDecision` từng dòng và outbox `decision:"cancelled"`.
6. `ListApprovals`, `ListPendingApprovalsForUser` (lọc tạm: admin thấy tất cả `pending`, người khác thấy của Request có `reporter_id` là mình).
7. Ánh xạ lỗi: hằng `REQUEST_APPROVAL_*` trong `apperrors` đúng bảng ở solution mục H (NotFound, FailedPrecondition, InvalidArgument, AlreadyExists, PermissionDenied).

## Kiểm thử

- Unit với fake repo, fake handler, fake `RequestGate`: mọi mã lỗi; rollback khi handler lỗi (Approval vẫn `pending`); idempotent; hết hạn lười; `MustCoverAll` báo thiếu; payload outbox không có `comment`.
- Test thứ tự khoá: fake `RequestGate` ghi lại thứ tự lời gọi, khẳng định Request trước Approval trong cả `Approve` và `CancelPendingApprovalsForRequest`.
- Lệnh: `go test ./internal/usecase/... -run Approval`.

## Tiêu chí hoàn thành

- [x] Tám `SubjectType` đều có thể đăng ký; thiếu một thì `MustCoverAll` lỗi.
- [x] `Approve` và chuyển trạng thái Request nằm trong một transaction (kiểm bằng fake `Tx` thấy một commit).
- [x] Mọi use case gọi `tenant.RequireTenantID`; chéo tenant trả `REQUEST_APPROVAL_NOT_FOUND`.
- [x] Không có gRPC outbound trong transaction.

## Rủi ro và lưu ý

- `SubjectHandler` do sáu CR cài: không đổi chữ ký sau khi merge nếu chưa báo các CR đó.
- `ValidateForRequest` cần `Request`; truyền bản đã khoá để tránh đọc hai lần.

## Kết quả triển khai (2026-10-08)
- Use case thật: `OpenApproval` (digest chuẩn tắc từ handler, một `pending` mỗi chủ thể kiểm trước khi chèn vì Postgres huỷ giao dịch khi INSERT lỗi, `idempotency_key`, kiểm FlowFor và trạng thái Request, chụp người duyệt, `due_at` theo giờ DB), `DecideApproval` (Approve/Reject, CAS theo `version`, thứ tự khoá Request rồi Approval, hết hạn lười được commit rồi trả `EXPIRED`, thử lại cùng quyết định là no-op), `CancelApproval`, `CancelPendingApprovalsForRequest`, `ExpireApprovals`, `RemindPendingApprovals`, `ExtendApproval`, `ListApprovals`, `GetApproval`, `ListPendingApprovalsForUser`, `RequestApprovalFromAPI`.
- `SubjectHandler` registry giữ nguyên chữ ký; thêm `WithRequestedSubjectID` (truyền subject_id qua ctx để không đổi chữ ký). Handler thật: `RequestTypeApprovalHandler` (approve gọi `ConfirmRequestType`, reject gọi `ReturnRequestToBacklog`) và `TransitionSubjectHandler` cho 7 chủ thể còn lại (solution, findings, answer, plan, phase, task_list, pre_deploy): approve gọi `RequestTransitioner` (`analysis_approved` hoặc `plan_approved` theo `stage`), reject gọi `ReturnRequestToBacklog`. Phần artifact của Solution/Plan nối qua cổng `SubjectArtifacts` (chưa gắn dịch vụ thật, xem 009-06): chưa gắn thì handler từ chối mở (`REQUEST_APPROVAL_SUBJECT_UNAVAILABLE`), không bao giờ duyệt ngầm.
- `Approval.Stage` lưu `Request.Status` lúc mở (Request.Stage không được ai điền); `STAGE_MISMATCH` so `Request.Status` với giá trị đó.
- Bỏ `TemporaryApproverPolicy`/`TemporaryApprovalAuthorizer` và truyền Request giả.
- Người gọi đã kiểm khi đổi `PendingApprovalCanceller`/`CancelPendingApprovalsForRequest.Execute` (chỉ `wire_request_lifecycle.go` và `ReturnRequestToBacklog`/`CancelRequest` qua cổng `ApprovalCanceller`, chữ ký cổng không đổi).
- Tiêu chí "không có gRPC outbound trong transaction": tra cứu team/admin của `OpenApproval` và `CanDecide` được làm trước giao dịch rồi phát lại trong giao dịch (test `NoDirectoryCallInsideTransaction`). Ngoại lệ có chủ đích: khi caller đã giữ giao dịch (đề xuất phân loại mở approval `request_type` trong giao dịch của Propose) và chính sách tenant tắt `allow_requester_approve`, tra cứu vẫn nằm trong giao dịch đó; chính sách mặc định (cho phép người báo duyệt) không tra cứu.
