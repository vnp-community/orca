# Phê duyệt Request

**Cập nhật:** 2026-10-08 · Đối chiếu với: `internal/domain/approval_policy.go`, `approval_policy_validation.go`, `approval_errors.go`,
`approval_subject.go`, `cmd/server/wire_approval.go`, `internal/config/config.go`.

## Cổng nào dùng được

Có 8 loại chủ thể (`subject_type`): `request_type`, `solution`, `findings`, `answer`, `plan`, `phase`, `task_list`, `pre_deploy`.

| Cổng | Tình trạng |
|---|---|
| `request_type` (xác nhận loại) | **có handler thật** |
| `solution`, `findings`, `answer`, `plan`, `phase`, `task_list`, `pre_deploy` | **chưa có**: `approvalSubjectArtifacts()` ở `cmd/server/wire_approval.go` còn rỗng, mở Approval cho các chủ thể này bị từ chối (`REQUEST_APPROVAL_SUBJECT_UNAVAILABLE`). Mở được khi CR-REQ-007, 008, 012, 013, 014 nối handler |

Khung `ApprovalService` (8 RPC), `ApprovalPolicyAdminService` (3 RPC) và bộ quét hết hạn/nhắc là thật.

Lưu ý bật dịch vụ: `REQUEST_APPROVAL_ENABLED` mặc định `true` trong `request-service`, nhưng `deploy/dev/docker-compose.yml` đặt
`${REQUEST_APPROVAL_ENABLED:-false}`, nên trên stack dev mặc định `ApprovalService` đóng cho tới khi đặt biến này `true`.

## RPC phê duyệt

`RequestApproval`, `Approve`, `Reject`, `Cancel`, `GetApproval`, `ListApprovals`, `ListPendingForUser` (hộp duyệt), `ExtendApproval`.
Kênh WS: `approval.request`, `approval.approve`, `approval.reject`, `approval.cancel`, `approval.get`, `approval.list`, `approval.listPending`
và sự kiện `approval.requested`, `approval.decided`, `approval.resolved`. Giao diện hộp duyệt thuộc CR-REQ-022; không khẳng định ở đây.

## Ai được duyệt

Mỗi Approval chọn chính sách khớp nhất theo (project, loại Request, size, urgency): chính sách cụ thể hơn (nhiều điều kiện khớp hơn)
thắng, hoà thì `priority` cao hơn, hoà nữa thì tạo sớm hơn. Không có chính sách khớp thì dùng **mặc định** (`DefaultPolicy`):

| Chủ thể | Người duyệt mặc định | Người yêu cầu tự duyệt |
|---|---|---|
| mọi chủ thể trừ `pre_deploy` | người báo (`reporter`) và `role:admin` | được |
| `pre_deploy` | chỉ `role:admin` | không |
| `request_type` của `hotfix`, `security` | như trên | không |
| `solution`, `plan`, `task_list` khi size L | như trên | không |

Hạn duyệt mặc định (`due_after`): bình thường: `answer`/`pre_deploy` 24 giờ, `findings` 72 giờ, `plan`/`phase`/`task_list` 7 ngày, còn lại 72 giờ.
Khẩn (`urgent`): `answer`/`pre_deploy` 2 giờ, `findings` 8 giờ, `plan`/`phase`/`task_list` 24 giờ, còn lại 4 giờ.

Người duyệt trong chính sách viết dạng `reporter`, `user:<id>`, `team:<id>`, `role:<tên>`. Tra team qua `tenant-service`, tra admin qua
`auth-service`; thiếu `TENANT_SERVICE_ADDR` hoặc `AUTH_SERVICE_ADDR` thì tra cứu tương ứng từ chối (fail closed).

**Agent không duyệt được.** `Approve`, `Reject`, `Cancel` từ chối caller tự động (`actor_type=agent`) với `REQUEST_APPROVAL_AGENT_FORBIDDEN`.

## Quản trị chính sách

`ApprovalPolicyAdminService` (chỉ admin): `ListApprovalPolicies`, `UpsertApprovalPolicy`, `DeleteApprovalPolicy`. Ràng buộc (`Validate`):
`subject_type` hợp lệ; 1 đến 20 người duyệt; principal `user`/`team`/`role` phải có id; loại, size (S/M/L), urgency (normal/urgent) phải
hợp lệ nếu có; `due_after` không âm. Vi phạm trả `REQUEST_APPROVAL_POLICY_INVALID`.

## Hết hạn và nhắc

Bộ quét chạy mỗi `REQUEST_APPROVAL_SWEEP_INTERVAL` (mặc định 1 phút): nhắc người duyệt (tối đa `REQUEST_APPROVAL_NOTIFY_MAX_RECIPIENTS`,
mặc định 50 người nhận) và hết hạn Approval quá `due_after`. Hết hạn ghi audit `approval.expire` và trả Request về backlog. Việc hết hạn vẫn chạy khi
cờ luồng tắt.

## Mã lỗi thường gặp

| Mã | Nghĩa |
|---|---|
| `REQUEST_APPROVAL_NOT_APPROVER` | người gọi không nằm trong danh sách duyệt |
| `REQUEST_APPROVAL_SELF_APPROVAL_FORBIDDEN` | người yêu cầu không được tự duyệt theo chính sách |
| `REQUEST_APPROVAL_AGENT_FORBIDDEN` | caller tự động |
| `REQUEST_APPROVAL_NO_USER` | thiếu người dùng đăng nhập |
| `REQUEST_APPROVAL_ALREADY_DECIDED` | Approval không còn `pending` |
| `REQUEST_APPROVAL_EXPIRED` | đã hết hạn |
| `REQUEST_APPROVAL_VERSION_CONFLICT`, `..._DIGEST_MISMATCH` | nội dung đã đổi từ lúc người duyệt xem; tải lại |
| `REQUEST_APPROVAL_STAGE_MISMATCH` | Request không còn ở giai đoạn cần duyệt |
| `REQUEST_APPROVAL_PENDING_EXISTS` | đã có Approval `pending` cho chủ thể này |
| `REQUEST_APPROVAL_COMMENT_REQUIRED`, `..._COMMENT_TOO_LONG` | bình luận bắt buộc / quá dài |
| `REQUEST_APPROVAL_NO_ELIGIBLE_APPROVER` | không có ai đủ điều kiện duyệt |
| `REQUEST_APPROVAL_SUBJECT_UNAVAILABLE` | chủ thể chưa có dịch vụ nền (xem bảng cổng) |
| `REQUEST_APPROVAL_DIRECTORY_UNAVAILABLE` | không tra được team/admin |
| `REQUEST_FLOW_DISABLED` | cờ luồng tắt; `Approve`, `RequestApproval`, `ExtendApproval` bị chặn, còn `Reject` và `Cancel` vẫn chạy |

Mọi quyết định ghi audit `approval.approve|reject|cancel|expire`; từ chối quyền ghi `outcome=denied`.
