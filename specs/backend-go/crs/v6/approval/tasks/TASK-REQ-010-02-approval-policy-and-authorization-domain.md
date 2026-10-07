# TASK-REQ-010-02: Domain chính sách và `ApprovalAuthorization.Decide`

**From Solution:** [BE-REQ-SOL-010](../solutions/BE-REQ-SOL-010-approval-authorization-notification-expiry.md) mục B
**Priority:** P1
**Service/Area:** `request-service` / domain
**File:** `backend-go/services/request-service/internal/domain/approval_policy.go` (mới), `approval_authorization.go` (mới), và `_test.go` tương ứng (mới)
**Depends on:** TASK-REQ-009-02
**Status:** `[x] DONE`

## Context

- Mô hình quyền thật chỉ có `tenant.Role(ctx)` = `user|admin` (`common/tenant/tenant.go`) và team (`tenant-service`, `ListTeamsForUser`). Chưa có vai trò mịn; vai trò tuỳ chỉnh dựng bằng `team:<id>`.
- Domain thuần stdlib; dữ liệu principal của caller được nạp trước ở usecase.
- Bảng mặc định lấy từ CR-REQ-010 mục 2.5 (đề xuất, chưa được chủ sản phẩm xác nhận).

## Việc cần làm

1. `PrincipalKind` (`user|team|role|reporter`), `Principal{Kind, ID}`, `ParsePrincipal("team:abc")` và `String()`; từ chối dạng sai (`REQUEST_APPROVAL_POLICY_INVALID` ở lớp trên).
2. `ApprovalPolicy` và `Specificity()`; `SelectPolicy(candidates, PolicyContext{ProjectID, RequestType, Size, Urgency})`: lọc khớp (nil khớp mọi giá trị), sắp theo `Specificity` giảm dần, rồi `Priority` giảm dần, rồi `CreatedAt` tăng dần.
3. `DefaultPolicy(subjectType, ctx)` đúng bảng 2.5: người duyệt (`reporter`, `role:admin`; `pre_deploy` chỉ `role:admin`), `AllowRequesterApprove` (false cho `pre_deploy`, cho `request_type` khi loại là `hotfix` hoặc `security`, và cho `solution`/`plan`/`task_list` khi `size=L`), `DueAfter` theo `urgency` (72h/4h, 72h/8h, 7d/24h, 24h/2h).
4. `DecisionActor{UserID, Role string; TeamIDs []string; IsMachine bool}` và `ApprovalAuthorization.Decide(actor, ap, approvers, reporterID) error` theo đúng thứ tự năm bước ở CR 2.4; trả lỗi miền `ErrNoUser`, `ErrAgentForbidden`, `ErrNotApprover`, `ErrSelfApprovalForbidden`.
5. `EligibleApprovers(approvers, reporterID, selfAllowed, expandedUsers []string) []string` trả tập người sau khi loại người yêu cầu (dùng cho `NO_ELIGIBLE_APPROVER`).

## Kiểm thử

- `approval_authorization_test.go`: bảng tổ hợp principal (user, team, role, reporter) x `selfAllowed` x caller là admin x `IsMachine`; admin bị tách nhiệm vụ loại khi là reporter; `requested_by=system` không bị coi là người yêu cầu.
- `approval_policy_test.go`: chọn cụ thể hơn, cùng độ cụ thể thì `Priority`, bằng nhau thì `CreatedAt`; `DefaultPolicy` đủ 8 `subject_type` x vài tổ hợp loại, size, urgency.
- Lệnh: `go test ./internal/domain/... -run "Policy|Authorization"`.

## Tiêu chí hoàn thành

- [x] 8 `subject_type` đều có mặc định; test so khớp bảng 2.5.
- [x] `Decide` không gọi mạng, không import ngoài stdlib.
- [x] `hotfix` và `security` mặc định `AllowRequesterApprove=false` ở `request_type`.

## Rủi ro và lưu ý

- Tenant một người dùng bị khoá ở `pre_deploy`, `hotfix`, `security`, size `L`: đó là hành vi có chủ đích; usecase trả lỗi sớm (task 04).
- Các con số hạn là đề xuất; đặt thành hằng có tên, không rải số trong mã.
