# TASK-REQ-010-04: Mở rộng người nhận và làm giàu payload thông báo

**From Solution:** [BE-REQ-SOL-010](../solutions/BE-REQ-SOL-010-approval-authorization-notification-expiry.md) mục E
**Priority:** P1
**Service/Area:** `request-service` / usecase, adapter, outbox relay
**File:** `internal/usecase/expand_approval_recipients.go` (mới), `publish_approval_notifications.go` (mới), `internal/adapter/grpcclient/admin_directory_resolver.go` (mới), `internal/config/config.go` (sửa), và `_test.go`
**Depends on:** TASK-REQ-010-03; CR-REQ-001 (bộ phát outbox)
**Status:** [x] DONE (đã kiểm chứng 2026-10-08: `go test ./internal/usecase/... ./internal/adapter/eventbus/... -run "ApprovalNotification|ApprovalRecipients"`)

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

- [x] Payload `approval.requested` chứa `user_ids` không rỗng khi có người duyệt.
- [x] Không có `comment` trong bất kỳ payload nào.
- [x] Mất `auth-service` không làm `OpenApproval` thất bại.
- [x] Trần người nhận cấu hình được.

## Rủi ro và lưu ý

- Tenant lớn: `ListUsers` phân trang tốn kém; cache và trần là giải pháp tạm.
- Bước làm giàu nằm trên đường publish; nếu relay chung với sự kiện khác, chỉ áp cho subject `orca.request.approval.*`.

## Kết quả triển khai (2026-10-08)
- `PublishApprovalNotifications.Enrich` (thay `ProcessAndPublish`, hết ép kiểu không kiểm) làm giàu `approval.requested`/`decided` thành payload khớp golden của notification-service (sao chép vào `internal/usecase/testdata/approval_notification/`, không import chéo): `user_ids`, `title`, `body`, `deep_link` dùng `request_id` thật; xoá `comment` nếu lọt vào. Nhắc dùng `reason:"reminder"`. Id sự kiện giữ nguyên.
- Nối vào relay qua `adapter/eventbus.ApprovalNotificationStore` (bọc `outbox.Store`, chỉ chạm subject `orca.request.approval.*`): lỗi tra cứu thư mục dừng lô trước dòng đó (giữ thứ tự, relay thử lại), payload hỏng thì phát nguyên.
- `AdminDirectoryResolver` thật qua `auth-service.ListUsers` (lọc `ROLE_ADMIN` còn hoạt động, phân trang, cache 60 giây, `tenant_id` lấy từ ctx đã xác thực). Đã đọc handler `ListUsers`: server dùng `req.TenantId` do client gửi (tin tưởng gọi nội bộ), nên request-service chỉ gửi tenant của ctx. Chưa chạy với auth-service thật. Trần người nhận cấu hình `REQUEST_APPROVAL_NOTIFY_MAX_RECIPIENTS` (mặc định 50).
- Producer ghi thêm `reporter_id`, `self_approval_allowed`, `request_number`, `requested_by` vào payload sự kiện gốc để làm giàu không cần đọc DB.
