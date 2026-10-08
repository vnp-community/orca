# TASK-REQ-010-03: Repository chính sách và `ResolveApproverPolicy`, `AuthorizeApprovalDecision`

**From Solution:** [BE-REQ-SOL-010](../solutions/BE-REQ-SOL-010-approval-authorization-notification-expiry.md) mục D
**Priority:** P1
**Service/Area:** `request-service` / adapter, usecase
**File:** `internal/adapter/{postgres,mysql}/approval_policy_repository.go` (mới), `approval_approver_repository.go` (mới), `internal/adapter/grpcclient/team_membership_resolver.go` (mới), `internal/usecase/resolve_approver_policy.go` (mới), `authorize_approval_decision.go` (mới), `internal/usecase/open_approval.go` (sửa), `list_pending_approvals_for_user.go` (sửa), `approver_policy_ports.go` (sửa: bỏ cài tạm)
**Depends on:** TASK-REQ-010-01, TASK-REQ-010-02, TASK-REQ-009-04
**Status:** [x] DONE (đã kiểm chứng 2026-10-08: `go test ./internal/usecase/... -run Approval`; hợp đồng `RunApprovalPolicyContract` và `RunApprovalFlowContract` trên Postgres và MySQL thật)

## Context

- Mẫu client team: `task-service/internal/adapter/grpcclient/team_scope_resolver.go`: `ListTeamsForUser{user_id}` không nhận `tenant_id`, tenant lấy từ metadata (`withTenantMetadata`). `ListTeamMembers{team_id}` cũng không có `tenant_id` (tenant.proto dòng 208).
- `request-service` phải dựng `withTenantMetadata` tương đương (chưa có; sao chép khuôn, không import chéo service).
- `OpenApproval` hiện dùng `TemporaryApproverPolicy` (task 009-04); task này thay.

## Việc cần làm

1. Repository chính sách: `ListEnabledCandidates(ctx, tenantID, subjectType, projectID, requestType, size, urgency)` bằng `WHERE ... (col IS NULL OR col=?)` cùng câu hai dialect; `Upsert`, `Delete`, `NowDB(ctx)` (`SELECT now()` / `SELECT CURRENT_TIMESTAMP(6)`).
2. Repository người duyệt: `InsertSnapshot(tx, approvalID, tenantID, []Principal)`, `ListForApproval`.
3. `TeamMembershipResolver`: `TeamsForUser(ctx, userID)` và `MembersOfTeam(ctx, teamID)`; cache trong tiến trình 60 giây theo `(tenant, teamID)`.
4. `ResolveApproverPolicy.Resolve`: ứng viên, `SelectPolicy`, không có thì `DefaultPolicy`; `due_at = NowDB + DueAfter` (giờ DB).
5. Sửa `OpenApproval`: chụp `approval_approvers` cùng transaction; nếu `!self_approval_allowed`, mở rộng `team:` thành người và loại `reporter_id` rồi gọi `EligibleApprovers`; rỗng thì `REQUEST_APPROVAL_NO_ELIGIBLE_APPROVER` và không chèn.
6. `AuthorizeApprovalDecision.CanDecide`: nạp `DecisionActor` (`tenant.UserID`, `tenant.Role`, `TeamsForUser`), đọc người duyệt đã chụp, gọi `Decide`; ánh xạ lỗi miền sang `REQUEST_APPROVAL_NOT_APPROVER`, `..._SELF_APPROVAL_FORBIDDEN`, `..._AGENT_FORBIDDEN`, `..._FORBIDDEN`.
7. `ListPendingApprovalsForUser` thay bộ lọc tạm: nạp principal của caller một lần; một SQL join `approval_approvers` (nhánh user/team/role) hợp nhánh `reporter` join `requests.reporter_id`; loại dòng caller bị tách nhiệm vụ; phân trang `(created_at, id)`.
8. Xoá `TemporaryApproverPolicy` và `TemporaryApprovalAuthorizer` hoặc đánh dấu chỉ dùng trong test.

## Kiểm thử

- Unit (fake team resolver, clock): mở rộng team, `NO_ELIGIBLE_APPROVER`, người duyệt chụp không đổi khi chính sách đổi sau, từng nhánh lỗi `CanDecide`.
- Integration hai DB: `ListPendingForUser` trả cùng kết quả trên bộ dữ liệu mẫu (user, team, admin, reporter); chọn chính sách theo độ cụ thể.
- Lệnh: `go test ./internal/usecase/... ./internal/adapter/... -run "Policy|ApproverSnapshot|PendingForUser"`.

## Tiêu chí hoàn thành

- [x] Chính sách đổi sau khi mở không đổi người duyệt của Approval `pending`.
- [x] `ListPendingForUser` giống nhau trên Postgres và MySQL.
- [x] `NO_ELIGIBLE_APPROVER` không tạo dòng `approvals`.
- [x] Không còn cài tạm trong đường chạy production.

## Rủi ro và lưu ý

- Chưa kiểm chứng: `ListTeamMembers` có phân trang không (message chỉ có `team_id`); team lớn có thể trả nhiều người.
- Nạp principal mỗi lần `Approve` tốn một lời gọi tenant-service; cache 60 giây chấp nhận độ trễ khi người rời team.

## Kết quả triển khai (2026-10-08)
- Repository chính sách hai dialect thật: `ListEnabledCandidates`, `Get`, `List` (phân trang), `Upsert` (tạo khi version 0, cập nhật có khoá lạc quan), `Delete`, đều `scoped` theo tenant; repository người duyệt chụp/đọc snapshot. `ResolveApproverPolicy` lấy `size`, `urgency`, `project`, `type` thật từ Request (hết hằng `"S"`/`"normal"`).
- `TeamMembershipResolver` thật (`grpcclient`): `ListTeamsForUser`/`ListTeamMembers`, tenant qua metadata, cache 60 giây theo (tenant, khoá), lỗi thành `REQUEST_APPROVAL_DIRECTORY_UNAVAILABLE` (không cache). Khi chưa cấu hình `TENANT_SERVICE_ADDR` tra cứu từ chối (fail closed), không trả "không ai". Chưa chạy với tenant-service thật (test dùng client giả); chưa kiểm chứng `ListTeamMembers` có phân trang không.
- Thứ tự ưu tiên trong `CanDecide`: admin hoặc snapshot; team chỉ tra khi có principal team và caller không phải admin/người được nêu tên. `ListPendingForUser` một SQL, giống nhau trên hai DB (`ListPendingForUserMatchesPrincipals`).
- Không còn cài tạm trong đường chạy production.
