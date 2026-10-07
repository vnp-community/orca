# TASK-REQ-010-06: Worker hết hạn và nhắc Approval

**From Solution:** [BE-REQ-SOL-010](../solutions/BE-REQ-SOL-010-approval-authorization-notification-expiry.md) mục F
**Priority:** P1
**Service/Area:** `request-service` / usecase, adapter
**File:** `internal/usecase/expire_approvals.go` (mới), `remind_pending_approvals.go` (mới), `internal/adapter/{postgres,mysql}/approval_repository.go` (sửa: `ClaimDue`, `ClaimDueForReminder`), `cmd/server/main.go` (sửa: khởi chạy vòng), và `_test.go`
**Depends on:** TASK-REQ-010-03, TASK-REQ-010-04; CR-REQ-006 (`ReturnToBacklog`) hoặc `TransitionRequest` của CR-REQ-003
**Status:** `[x] DONE`

## Context

- Mẫu: `mcp-service/internal/usecase/approvals.go` (`ExpireApprovals` so sánh-và-ghi theo `now`) và vòng phục hồi `RunRecoveryLoop` của `task-service/internal/usecase/execution_lease.go`.
- Khác mcp: hết hạn ở đây phải đưa Request về `request_backlog` (không chỉ đổi cờ).
- Thứ tự khoá Request rồi Approval (SOL-009 mục 1.3): worker `SKIP LOCKED` trên Approval chỉ để chọn ứng viên, rồi mỗi ứng viên mở transaction riêng khoá Request trước.
- `FOR UPDATE SKIP LOCKED` cần MySQL >= 8.0.1 (CR-DB-002).

## Việc cần làm

1. Repository: `ClaimDue(ctx, batch)` trả tối đa 100 `(tenantID, approvalID, requestID)` có `status='pending' AND due_at <= <giờ DB>` (Postgres `FOR UPDATE SKIP LOCKED` trong CTE; MySQL `SELECT ... FOR UPDATE SKIP LOCKED` rồi thả khoá; việc ghi thật làm trong transaction của bước 2 bằng so sánh-và-ghi).
2. `ExpireApprovals.Execute`: mỗi ứng viên một transaction: khoá Request, `GetForUpdate`, `Expire`, `UpdateDecision`, `handler.OnClosedWithoutDecision("expired")`, `ReturnToBacklog(stage, reason="approval_expired")` theo bảng ánh xạ chủ thể sang `returned_from_stage` (`request_type`→`classification`; `solution|findings|answer`→`analysis`; `plan|task_list`→`plan`; `phase`→`phase`; `pre_deploy`→`plan` nếu Request ở `awaiting_plan_approval`, `phase` nếu `executing`), outbox `decision:"expired"`.
3. `RemindPendingApprovals.Execute`: chọn `pending`, `reminded_at IS NULL`, `now_db >= created_at + 0.75*(due_at-created_at)`; so sánh-và-ghi `reminded_at` rồi outbox `approval.requested` với `reason:"reminder"`. Mỗi Approval đúng một lần.
4. `RunLoop(ctx, interval)` (60 giây, cấu hình `REQUEST_APPROVAL_SWEEP_INTERVAL`) cho cả hai; khởi chạy ở `main.go` sau khi dịch vụ sẵn sàng, dừng theo `ctx`.
5. Metric đếm số đã hết hạn và đã nhắc (khuôn `common` metric hiện dùng ở service khác; đọc lúc làm).

## Kiểm thử

- Unit (fake clock, fake handler, fake `ReturnToBacklog`): từng chủ thể về đúng `returned_from_stage`; `pre_deploy` hai nhánh; nhắc đúng một lần; handler lỗi thì Approval vẫn `pending` và vòng tiếp tục với ứng viên khác.
- Integration hai DB: hai worker song song không xử lý trùng; hết hạn đua với `Approve` (một thắng, không lỗi 500); nhắc song song chỉ phát một lần.
- Lệnh: `go test ./internal/usecase/... ./internal/adapter/... -run "Expire|Remind"`.

## Tiêu chí hoàn thành

- [x] Approval quá hạn thành `expired` trong một chu kỳ quét; Request `request_backlog` với `return_reason="approval_expired"`.
- [x] Hai replica không xử lý trùng.
- [x] Nhắc gửi đúng một lần ở mốc 75%.
- [x] Mọi so sánh thời gian dùng giờ DB.

## Rủi ro và lưu ý

- Hạn ngắn đưa Request về backlog gây phiền; số hạn là đề xuất.
- Chưa có RPC gia hạn (câu hỏi mở 4): người dùng phải mở lại từ backlog.
