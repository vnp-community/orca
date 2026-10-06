# BE-REQ-SOL-010: Chính sách quyền duyệt, thông báo và hết hạn của Approval

> **📋 Proposed** (chưa triển khai). Thiết kế thực thi cho CR-REQ-010: `request-service` (mới) và sửa nhỏ `notification-service`.

**CR:** [CR-REQ-010](../../../../../../docs/crs/v6/approval/CR-REQ-010-approval-authorization-notification-expiry.md)
**Service:** `request-service` (mới) · `notification-service` (sửa `consumer.go`, `notification_event.go`) · gọi `tenant-service`, `auth-service`
**TDD tham chiếu:** [`arch/03`](../../../../tdd/architecture/03-clean-architecture-guidelines.md), [`arch/05`](../../../../tdd/architecture/05-data-architecture.md), [`arch/07`](../../../../tdd/architecture/07-security-architecture.md) (phân quyền, tách nhiệm vụ), [`arch/08`](../../../../tdd/architecture/08-inter-service-communication.md) (stream, subject), [`arch/09`](../../../../tdd/architecture/09-observability-reliability.md), [`services/notification-service`](../../../../tdd/services/notification-service.md), [`services/auth-service`](../../../../tdd/services/auth-service.md), [`services/tenant-service`](../../../../tdd/services/tenant-service.md)

---

## 1. Trạng thái hiện tại (re-verify)

Đã đọc: CR-REQ-010, `notification-service/internal/adapter/eventbus/consumer.go` (`SubjectBinding{StreamName, Subject, Durable}`, danh sách `Subjects`), `internal/domain/notification_event.go` (`subjectRule{Type, Title, Body, Severity, Channels, DeepLink, Locked}`, `subjectRules`, `defaultRule`, `EventPayload` đọc `user_ids`, `ErrNoRecipients`), `proto/orca/tenant/v1/tenant.proto` (`ListTeamMembers{team_id}` dòng 29/208, `ListTeamsForUser{user_id}` dòng 35), `proto/orca/auth/v1/auth.proto` (`ListUsers{tenant_id, page_token, page_size}` dòng 46/305), `task-service/internal/adapter/grpcclient/team_scope_resolver.go` (mẫu: `ListTeamsForUser` không nhận `tenant_id`), `mcp-service/internal/usecase/approvals.go` (`ExpireApprovals`). `notification-service` có thư mục `migrations/{mysql,postgres}` nhưng không cần migration cho việc này (bảng subject cố định trong mã).

**Correction relative to CR-REQ-010**
1. **`Durable` có sẵn.** `SubjectBinding.Durable` cho consumer có tên (một con trỏ chung), dùng cho sự kiện mất là hại; `processed_events` dedupe giao lặp. CR chỉ nêu thêm binding; solution này đặt `approval.requested` là `Durable: "notification-service-request-approval-requested"` để thông báo không mất khi service xuống. `approval.decided` để ephemeral như các task event (mất thì chỉ chậm hiển thị, trạng thái nằm ở DB). Cần xác nhận với chủ `notification-service`.
2. **Rule `Locked` có sẵn.** `Locked: true` bỏ qua title/body/deep_link của payload. Vì CR muốn `request-service` đặt `payload.title` ("Plan rejected"), rule của hai subject này **không** Locked; đổi lại `request-service` phải chỉ đưa vào `title`, `body` chuỗi ngắn tự dựng từ trường whitelist (không `comment`, không nội dung Plan).
3. **Số migration:** `approval_policies` và `approval_approvers` đặt trong migration `NNNN_approval_policies`; số đọc lại ở `request-service/migrations/*` lúc làm (đi sau `NNNN_approvals` của SOL-009). `notification-service` không có migration mới.
4. `ListUsers` nhận `tenant_id` trong request nhưng quy ước repo là lấy tenant từ ngữ cảnh; làm task 04 phải xác nhận trường nào được tôn trọng (chưa kiểm chứng) và không tin `tenant_id` do client truyền.

## 2. Giải pháp

### A. Cây thư mục

`request-service` (mới):
```
internal/domain/approval_policy.go, approval_authorization.go
internal/usecase/
    resolve_approver_policy.go, authorize_approval_decision.go, manage_approval_policies.go
    expand_approval_recipients.go, publish_approval_notifications.go
    expire_approvals.go, remind_pending_approvals.go
    approver_policy_ports.go (sửa: bỏ cài tạm của SOL-009)
internal/adapter/postgres|mysql/approval_policy_repository.go, approval_approver_repository.go
internal/adapter/grpcclient/team_membership_resolver.go, admin_directory_resolver.go
internal/adapter/grpc/approval_policy_admin_server.go   # nếu câu hỏi mở 2 được chấp nhận
migrations/{postgres,mysql}/NNNN_approval_policies.{up,down}.sql
```
`notification-service` (sửa): `internal/adapter/eventbus/consumer.go` (+2 binding), `internal/domain/notification_event.go` (+2 rule), kèm `consumer_test.go`, `notification_event_test.go`.

### B. Domain thuần

```go
type PrincipalKind string // user|team|role|reporter
type Principal struct { Kind PrincipalKind; ID string }
type ApprovalPolicy struct {
    ID, TenantID string; ProjectID, RequestType, Size, Urgency *string
    Approvers []Principal; AllowRequesterApprove bool; DueAfter *time.Duration
    Priority int; Enabled bool; Version int64
}
func (p ApprovalPolicy) Specificity() int          // số trường project/type/size/urgency khác nil
func SelectPolicy(candidates []ApprovalPolicy, ctx PolicyContext) (ApprovalPolicy, bool) // cụ thể hơn thắng, rồi priority, rồi created_at
func DefaultPolicy(st SubjectType, ctx PolicyContext) ApprovalPolicy // bảng 2.5 của CR

type DecisionActor struct { UserID, Role string; TeamIDs []string; IsMachine bool }
func (a ApprovalAuthorization) Decide(actor DecisionActor, ap Approval, approvers []Principal, reporterID string) error
```
Thứ tự kiểm trong `Decide` đúng CR 2.4: có `UserID`; `IsMachine` thì `REQUEST_APPROVAL_AGENT_FORBIDDEN`; khớp principal hoặc `role:admin` (`REQUEST_APPROVAL_NOT_APPROVER`); tách nhiệm vụ khi `!SelfApprovalAllowed` và `UserID==reporterID` (hoặc `==requested_by` khi là người, trừ `system`) thì `REQUEST_APPROVAL_SELF_APPROVAL_FORBIDDEN`, kể cả admin. `IsMachine` hiện luôn false vì chưa có cơ chế nhận biết (câu hỏi mở 6); CR-REQ-017 không đăng ký tool `approval_approve`/`approval_reject` trong khi chờ.

### C. Migration `NNNN_approval_policies`

`approval_policies` theo CR 2.3 (cùng CHECK `subject_type` với `approvals`, CHECK 11 loại, `size`, `urgency`; `approvers` là `JSONB`/`JSON`), và:

```sql
CREATE TABLE approval_approvers (            -- cả hai dialect; UUID vs CHAR(36)
  approval_id UUID NOT NULL REFERENCES approvals(id), tenant_id UUID NOT NULL,
  principal_kind TEXT NOT NULL CHECK (principal_kind IN ('user','team','role','reporter')),
  principal_id TEXT NOT NULL,                -- 'reporter' dùng chuỗi rỗng
  PRIMARY KEY (approval_id, principal_kind, principal_id));
CREATE INDEX approval_approvers_by_principal ON approval_approvers (tenant_id, principal_kind, principal_id);
```
MySQL: `principal_id VARCHAR(64) NOT NULL DEFAULT ''` (khoá chính không nhận NULL). `approval_policies.project_id/request_type/size/urgency` NULL nghĩa là khớp mọi giá trị; chọn ứng viên bằng `WHERE (project_id IS NULL OR project_id=?) AND (request_type IS NULL OR request_type=?) ...` giống nhau hai DB, sắp xếp phần còn lại ở Go.

### D. Use case

- **`ResolveApproverPolicy.Resolve(ctx, req, subjectType)`** (thay cài tạm): nạp ứng viên enabled của tenant, `SelectPolicy`, không có thì `DefaultPolicy`; trả `{approvers, due_at = now_db + due_after, self_approval_allowed}`. `due_at` dùng giờ DB: repository có `NowDB(ctx)`.
- **`OpenApproval` (SOL-009) mở rộng**: sau khi có chính sách, chụp `approval_approvers` cùng transaction; nếu `!self_approval_allowed`, mở rộng `team:` thành người (`ListTeamMembers` cho từng team; kết quả cache 60s theo tenant) và `role:admin` (`ListUsers` phân trang lọc `role=admin`), loại `reporter_id`; còn rỗng thì `REQUEST_APPROVAL_NO_ELIGIBLE_APPROVER`, không chèn dòng.
- **`AuthorizeApprovalDecision.CanDecide`**: nạp `DecisionActor` (`tenant.UserID`, `tenant.Role`, `ListTeamsForUser`); đọc `approval_approvers`; gọi `ApprovalAuthorization.Decide`.
- **`ListPendingForUser`** thay bộ lọc tạm: một SQL không toán tử JSON, nhánh `user/team/role` join `approval_approvers`, nhánh `reporter` join `requests.reporter_id`, loại dòng caller bị tách nhiệm vụ loại, phân trang `(created_at, id)`.
- **`ManageApprovalPolicies`** (`List`, `Upsert`, `Delete`): chỉ admin; kiểm `approvers` (kind hợp lệ, team tồn tại, tối đa 20 principal), `REQUEST_APPROVAL_POLICY_INVALID`/`..._NOT_FOUND`. Chỉ làm khi câu hỏi mở 2 được chấp nhận; nếu không, bảng nạp bằng seed.

### E. Thông báo

`PublishApprovalNotifications` chạy ở consumer outbox nội bộ của `request-service` (không trong transaction): đọc sự kiện `approval.requested`/`decided` từ `outbox_events`, mở rộng người nhận, rồi phát bản "thông báo" tới subject mà `notification-service` nghe. Do sự kiện domain đã dùng chính subject `orca.request.approval.requested`, cách gọn là: bộ phát outbox của `request-service` làm bước làm giàu **trước khi publish**: payload gắn thêm `user_ids`, `title`, `body`, `deep_link` (`/?section=requests&request=<id>&approval=<id>`, route cuối do CR-REQ-018/022). Cách này không cần subject thứ hai. Lỗi tra cứu chỉ làm retry của outbox, không làm Approval thất bại. Trần `REQUEST_APPROVAL_NOTIFY_MAX_RECIPIENTS`=50 (đề xuất), cắt và log. `approval.decided`: người nhận là `reporter_id` và `requested_by` nếu là người, trừ người vừa quyết định. Id sự kiện ổn định để `processed_events` dedupe. Không đưa `comment` vào payload.

`notification-service`:
```go
{StreamName: "REQUEST", Subject: "orca.request.approval.requested", Durable: "notification-service-request-approval-requested"},
{StreamName: "REQUEST", Subject: "orca.request.approval.decided"},
```
```go
"orca.request.approval.requested": {Type: "request.approval_requested", Title: "Approval needed", Severity: SeverityWarning, Channels: ws+push},
"orca.request.approval.decided":   {Type: "request.approval_decided",   Title: "Approval decided", Severity: SeverityInfo, Channels: ws+push},
```
Tên stream `REQUEST` phải khớp `EnsureStream` của `request-service` (CR-REQ-001); chưa kiểm chứng vì service chưa có. Test hợp đồng: một sự kiện mẫu của `request-service` qua `TranslateEvent` phải ra đúng người nhận và kiểu.

### F. Hết hạn và nhắc

`ExpireApprovals.Execute(ctx, batch=100)` mỗi 60s: `SELECT ... WHERE status='pending' AND due_at <= <giờ DB> ... FOR UPDATE SKIP LOCKED` (Postgres; MySQL ≥ 8.0.1); mỗi Approval một transaction theo thứ tự khoá Request rồi Approval (SOL-009 mục 1.3): `Expire`, `handler.OnClosedWithoutDecision("expired")`, `ReturnToBacklog(trigger=return_to_backlog, reason="approval_expired", stage)`, outbox `decision:"expired"`. Ánh xạ `returned_from_stage` đúng bảng CR 2.7 (`pre_deploy`: `plan` nếu Request ở `awaiting_plan_approval`, `phase` nếu `executing`). `RemindPendingApprovals`: `pending`, `reminded_at IS NULL`, `now_db >= created_at + 0.75*(due_at-created_at)`, so sánh-và-ghi `reminded_at`, phát lại `approval.requested` với `payload.reason="reminder"`. Hai replica an toàn nhờ `SKIP LOCKED` và so sánh-và-ghi.

### G. Lỗi

`REQUEST_APPROVAL_{NOT_APPROVER, SELF_APPROVAL_FORBIDDEN, AGENT_FORBIDDEN, NO_ELIGIBLE_APPROVER, POLICY_INVALID, POLICY_NOT_FOUND}`; `REQUEST_APPROVAL_FORBIDDEN` bao trùm. Không sự kiện mới.

## 3. Quyết định thiết kế

| # | Quyết định | Lý do |
|---|---|---|
| 1 | Chính sách trong DB, đánh giá bằng Go | OPA chỉ dùng cho grant; dễ test |
| 2 | Chụp người duyệt lúc mở | Đổi chính sách giữa chừng không đổi cổng; join portable |
| 3 | Làm giàu payload trước khi publish, không subject thứ hai | `notification-service` chỉ cần `user_ids` |
| 4 | `approval.requested` Durable | Thông báo duyệt mất là cổng kẹt |
| 5 | Hết hạn về backlog, không tự chuyển tiếp | Không có quyết định thì không đi tiếp |
| 6 | Giờ DB cho hạn và nhắc | Nhiều replica |

## 4. Phụ thuộc và thứ tự

Cần SOL-009 (bảng, port, `OpenApproval`), CR-REQ-003 (`TransitionRequest`), CR-REQ-006 (`ReturnToBacklog`; trước khi có, hết hạn gọi `TransitionRequest` trực tiếp với trigger `return_to_backlog`). Mở khoá CR-REQ-014, 022, 024. Ghi tiến (không phụ thuộc): CR bổ sung 026 đến 036 có thể thêm `subject_type` hoặc mức chính sách; bảng chính sách đã có cột `subject_type` nên mở rộng chỉ cần thêm giá trị CHECK.

## 5. Kiểm thử

- Unit domain: `Decide` đủ tổ hợp (principal x tách nhiệm vụ x admin x máy), `SelectPolicy`, `DefaultPolicy` đủ 8 subject, mốc nhắc.
- Unit usecase (fake tenant, auth, clock): mở rộng người nhận, cắt trần, lỗi tra cứu, `NO_ELIGIBLE_APPROVER`, hết hạn từng chủ thể, nhắc một lần.
- Integration hai DB: `ListPendingForUser` cùng kết quả, `SKIP LOCKED` hai worker, đua hết hạn với `Approve`, chụp `approval_approvers` cùng transaction.
- Hợp đồng `notification-service`: `notification_event_test.go` cho hai subject; `consumer_test.go` kiểm binding và `Durable`; golden payload từ `request-service`.
- Chưa kiểm chứng: tên stream `REQUEST`, hiệu năng `ListUsers`, push thật.

## 6. Rủi ro và điểm chưa kiểm chứng

- Mô hình vai trò nghèo (`user|admin` và team); "người có thẩm quyền" phải dựng bằng team.
- Tách nhiệm vụ có thể khoá tenant nhỏ; giảm thiểu bằng lỗi sớm và chính sách ghi đè.
- Mặc định hạn và cờ là con số đề xuất, chưa đo.
- Đổi `notification-service` cần triển khai lại service đó.
- Nhận biết nguồn MCP chưa có.

## 7. Câu hỏi mở

1. Cần vai trò "approver" riêng cho tổ chức không?
2. Chấp nhận `ApprovalPolicyAdminService` (ngoài README 3.6) hay chỉ seed ở v1?
3. Sự kiện nhắc và hết hạn riêng (`reason`, `decision`) có được chấp nhận?
4. Cần RPC gia hạn (`Extend`)?
5. "Người yêu cầu" cho Plan/Phase là người báo cáo (CR chọn) hay người kích hoạt?
6. Cơ chế truyền nguồn gọi MCP (CR-REQ-016/017).
7. Xác nhận `Durable` và việc rule không `Locked` với chủ `notification-service`.

## 8. Tham chiếu

- `/opt/repos/orca/docs/crs/v6/approval/CR-REQ-010-approval-authorization-notification-expiry.md`
- `/opt/repos/orca/backend-go/services/notification-service/internal/adapter/eventbus/consumer.go`
- `/opt/repos/orca/backend-go/services/notification-service/internal/domain/notification_event.go`
- `/opt/repos/orca/backend-go/services/notification-service/internal/usecase/handle_incoming_event.go`
- `/opt/repos/orca/backend-go/services/task-service/internal/adapter/grpcclient/team_scope_resolver.go`
- `/opt/repos/orca/backend-go/services/mcp-service/internal/usecase/approvals.go`
- `/opt/repos/orca/backend-go/proto/orca/tenant/v1/tenant.proto`, `proto/orca/auth/v1/auth.proto`
- `/opt/repos/orca/specs/backend-go/crs/v6/approval/solutions/BE-REQ-SOL-009-generic-approval-domain-and-api.md`
