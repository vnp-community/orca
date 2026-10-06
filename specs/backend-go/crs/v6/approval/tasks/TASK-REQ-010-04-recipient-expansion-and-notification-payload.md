# TASK-REQ-010-04: Mở rộng người nhận và làm giàu payload thông báo

**From Solution:** [BE-REQ-SOL-010](../solutions/BE-REQ-SOL-010-approval-authorization-notification-expiry.md) mục E
**Priority:** P1
**Service/Area:** `request-service` / usecase, adapter, outbox relay
**File:** `internal/usecase/expand_approval_recipients.go` (mới), `publish_approval_notifications.go` (mới), `internal/adapter/grpcclient/admin_directory_resolver.go` (mới), `internal/config/config.go` (sửa), và `_test.go`
**Depends on:** TASK-REQ-010-03; CR-REQ-001 (bộ phát outbox)
**Status:** [ ] TODO

## Context

- `notification-service` chỉ gửi cho `user_id`/`user_ids` trong payload, thiếu thì `ErrNoRecipients` (`internal/domain/notification_event.go`). Nó không biết team hay vai trò.
- `auth-service` `ListUsers{tenant_id, page_token, page_size}` (auth.proto dòng 305). Chưa kiểm chứng: server có tôn trọng `tenant_id` từ request hay chỉ lấy từ ngữ cảnh; làm bước 1 để biết.
- Thông báo được lưu: không đưa `comment`, không đưa nội dung Plan hay Solution.

## Việc cần làm

1. Đọc `auth-service` handler `ListUsers` để xác định nguồn tenant; gọi theo quy ước repo và không tin `tenant_id` do client truyền. Ghi kết quả vào mô tả PR.
2. `AdminDirectoryResolver.ListAdmins(ctx)` phân trang qua `page_token`, lọc `role=admin`, cache 60 giây theo tenant.
3. `ExpandApprovalRecipients.Execute(ctx, approvalID)`: từ `approval_approvers` suy người nhận: `user` giữ nguyên, `team` qua `MembersOfTeam`, `role:admin` qua `ListAdmins`, `reporter` là `reporter_id`; khử trùng; loại người yêu cầu khi `!self_approval_allowed`; cắt ở `REQUEST_APPROVAL_NOTIFY_MAX_RECIPIENTS` (mặc định 50) và log số bị cắt.
4. `PublishApprovalNotifications`: bước làm giàu trước khi publish của bộ phát outbox: với `approval.requested` thêm `user_ids`, `title` ("Approval needed"), `body` ngắn dựng từ `subject_type` và số Request, `deep_link` (`/?section=requests&request=<id>&approval=<id>`); với `approval.decided` người nhận là `reporter_id` và `requested_by` nếu là người, trừ `decided_by`; `title` kiểu "Plan approved/rejected". Nhắc: `payload.reason="reminder"`.
5. Lỗi tra cứu chỉ trả lỗi cho relay để thử lại; không đụng trạng thái Approval.
6. Id sự kiện giữ nguyên khi phát lại để `processed_events` của `notification-service` dedupe.

## Kiểm thử

- Unit (fake tenant, auth): mở rộng đủ bốn loại principal, khử trùng, cắt trần 50, loại người yêu cầu, lỗi tra cứu làm relay retry, payload không có `comment`.
- Golden JSON trong `testdata/approval_notification/*.json` (mới), dùng lại ở task 05.
- Lệnh: `go test ./internal/usecase/... -run "Recipients|ApprovalNotification"`.

## Tiêu chí hoàn thành

- [ ] Payload `approval.requested` chứa `user_ids` không rỗng khi có người duyệt.
- [ ] Không có `comment` trong bất kỳ payload nào.
- [ ] Mất `auth-service` không làm `OpenApproval` thất bại.
- [ ] Trần người nhận cấu hình được.

## Rủi ro và lưu ý

- Tenant lớn: `ListUsers` phân trang tốn kém; cache và trần là giải pháp tạm.
- Bước làm giàu nằm trên đường publish; nếu relay chung với sự kiện khác, chỉ áp cho subject `orca.request.approval.*`.
