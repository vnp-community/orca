# CR-REQ-010 — Chính sách quyền duyệt, thông báo và hết hạn của Approval

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-REQ-010 |
| **Tên** | Ai được duyệt theo `subject_type`/loại/size, tách nhiệm vụ, thông báo qua `notification-service`, hết hạn theo `due_at` |
| **Loại** | Feature |
| **Priority** | 🟠 P1 |
| **Effort** | Medium |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-05 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | CR-REQ-009 (bảng `approvals`, port `ApproverPolicy`, `ApprovalAuthorizer`), CR-REQ-003 (`TransitionRequest`), CR-REQ-006 (`ReturnToBacklog`) |
| **Mở khoá** | CR-REQ-014 (chính sách `hotfix`/`security`/`ops_request`), CR-REQ-022 (hộp duyệt), CR-REQ-024 (audit) |
| **Tác động** | `request-service` (domain, usecase, adapter, migration hai dialect); `notification-service` (`internal/adapter/eventbus/consumer.go`, `internal/domain/notification_event.go`); `tenant-service` và `auth-service` chỉ được gọi, không đổi |

## 1. Bối cảnh và vấn đề

1. CR-REQ-009 cho Approval vòng đời nhưng chỉ có người duyệt tạm: `role=admin` hoặc `reporter_id`. Chưa có chính sách theo `subject_type`, loại Request, `size`, `urgency`.
2. Mô hình quyền hiện có không đủ cho việc này, khi đọc code:
   - `auth-service` chỉ có vai trò toàn cục `user|admin` (`internal/domain/user.go`: "fine-grained authorization is OPA's job"). `tenant.Role(ctx)` cung cấp vai trò đó; rỗng thì coi là không phải admin.
   - Team nằm ở `tenant-service` (`ListTeamsForUser`, `ListTeamMembers`); không có vai trò theo team.
   - `Grant` của `task-service` (`domain/grant.go`) cấp theo Task (owner/admin/user/team/company) và kế thừa theo cây; không có Grant cho Request, và mức grant không mô tả "được duyệt".
   - OPA đã được dùng cho mức grant (`backend-go/policy/orca-authz/task_grant.rego`, `admin.rego`).
3. `DecisionGate.Resolve` không kiểm quyền người giải (`orchestration-service/internal/usecase/resolve_gate.go`); không lặp lại ở đây.
4. `notification-service` chỉ gửi cho người có tên trong payload (`user_id`/`user_ids`); payload không có người nhận thì bỏ qua (`ErrNoRecipients`). Nó không biết team hay vai trò, nên `request-service` phải giải ra danh sách người.
5. Hết hạn đã có tiền lệ ở `mcp-service` (`ExpireApprovals`, so sánh-và-ghi theo `now`), nhưng Approval của Request đóng hạn thì Request phải đi tiếp (trả backlog), không chỉ đổi cờ.

## 2. Giải pháp đề xuất

### 2.1 Phạm vi sở hữu

CR này sở hữu: chính sách người duyệt, tách nhiệm vụ (người yêu cầu không tự duyệt khi bật), thông báo, hết hạn và nhắc. Không sở hữu: bảng `approvals`, máy trạng thái, `ApprovalService` (CR-REQ-009).

### 2.2 Người duyệt (principal)

Một chính sách cho ra tập principal; người gọi hợp lệ nếu khớp ít nhất một và không bị loại bởi tách nhiệm vụ.

| Principal | Khớp khi | Nguồn |
|---|---|---|
| `user:<id>` | `caller == id` | `tenant.UserID(ctx)` |
| `team:<id>` | caller là thành viên | `tenant-service.ListTeamsForUser` (như `TeamScopeResolver` của task-service; RPC không nhận `tenant_id`, lấy từ ngữ cảnh) |
| `role:admin` | `tenant.Role(ctx) == "admin"` | JWT/phiên qua gateway |
| `reporter` | `caller == requests.reporter_id` | bảng `requests` |

`role:admin` luôn được tính vào mọi Approval, trừ khi bị tách nhiệm vụ loại. Vai trò tùy chỉnh (ví dụ "security lead") được mô hình bằng `team:<id>`, vì vai trò mịn chưa tồn tại (xem mục 7).

### 2.3 Bảng chính sách `approval_policies` (mới, migration của CR này)

| Cột | Postgres (schema `request`) | MySQL |
|---|---|---|
| `id`, `tenant_id` | `UUID` | `CHAR(36)` |
| `project_id` | `UUID NULL` (NULL = mặc định của tenant) | `CHAR(36) NULL` |
| `subject_type` | `TEXT NOT NULL` (cùng CHECK với `approvals`) | `VARCHAR(20)` |
| `request_type` | `TEXT NULL` (NULL = mọi loại; CHECK 11 loại) | `VARCHAR(20) NULL` |
| `size` | `TEXT NULL CHECK IN ('S','M','L')` | `VARCHAR(1) NULL` |
| `urgency` | `TEXT NULL CHECK IN ('normal','urgent')` | `VARCHAR(10) NULL` |
| `approvers` | `JSONB NOT NULL` (mảng principal, ví dụ `["reporter","team:..."]`) | `JSON NOT NULL` |
| `allow_requester_approve` | `BOOLEAN NOT NULL` | `TINYINT(1)` |
| `due_after_seconds` | `INT NULL` (NULL = không hết hạn) | `INT NULL` |
| `priority` | `INT NOT NULL DEFAULT 0` | |
| `enabled`, `version`, `created_by`, `created_at`, `updated_at` | | |

Chọn chính sách: lọc các dòng `enabled` khớp (`NULL` khớp mọi giá trị); xếp theo số trường cụ thể khớp (`project_id`, `request_type`, `size`, `urgency` không NULL) giảm dần, rồi `priority` giảm dần, rồi `created_at`. Không có dòng nào khớp thì dùng mặc định dựng sẵn (2.5). Chính sách chỉ áp cho Approval tạo sau đó; Approval `pending` giữ tập người duyệt đã chụp.

Danh sách người duyệt của từng Approval được chụp ở bảng `approval_approvers` (mới): `approval_id`, `tenant_id`, `principal_kind` (`user|team|role|reporter`), `principal_id`, khóa chính `(approval_id, principal_kind, principal_id)`, chỉ mục `(tenant_id, principal_kind, principal_id)`. Lý do tách bảng thay vì cột JSON: `ListPendingForUser` cần join portable giữa hai DB; `JSON_OVERLAPS`/`@>` khác nhau giữa MySQL và Postgres.

CRUD chính sách: RPC quản trị `ApprovalPolicyAdminService` (`List`, `Upsert`, `Delete`), chỉ `role=admin`. README 3.6 không liệt kê RPC này (xem mục 7); nếu bị từ chối, v1 chỉ dùng mặc định và bảng được nạp bằng migration/seed.

### 2.4 Quyền và tách nhiệm vụ (`ApprovalAuthorizer.CanDecide`)

Thứ tự kiểm (domain thuần `ApprovalAuthorization.Decide`, không gọi mạng; dữ liệu principal của caller nạp trước):
1. Tenant khớp; caller có `UserID`. Không có thì `REQUEST_APPROVAL_FORBIDDEN`.
2. **Danh tính không phải người:** lời gọi mang nguồn MCP/dịch vụ (token máy) bị từ chối `REQUEST_APPROVAL_AGENT_FORBIDDEN`. Cơ chế nhận biết nguồn chưa có (CR-REQ-009 câu 5); cho đến khi có, CR-REQ-017 không đăng ký tool `approval_approve`/`approval_reject`. Lý do: agent tự duyệt kế hoạch của chính nó làm cổng vô nghĩa.
3. Khớp ít nhất một principal trong `approval_approvers` (cộng `role:admin`). Không thì `REQUEST_APPROVAL_NOT_APPROVER`.
4. **Tách nhiệm vụ:** nếu `approvals.self_approval_allowed=false` và `caller == requests.reporter_id` (hoặc `== approvals.requested_by` khi là người) thì `REQUEST_APPROVAL_SELF_APPROVAL_FORBIDDEN`, kể cả khi caller là admin. "Người yêu cầu" là người báo cáo Request, không phải `requested_by=system` của cổng do AI mở.
5. Còn lại: được phép.

Khi mở Approval với tách nhiệm vụ bật mà sau khi loại người yêu cầu tập người duyệt rỗng (tenant chỉ có một admin), `OpenApproval` thất bại `REQUEST_APPROVAL_NO_ELIGIBLE_APPROVER` (FailedPrecondition) thay vì tạo cổng không ai duyệt được. Request giữ trạng thái cũ, admin sửa chính sách rồi thử lại. Kiểm lúc mở dùng `team:` đã mở rộng thành người (`ListTeamMembers`); thành viên thay đổi sau đó vẫn xét lại lúc quyết định.

### 2.5 Mặc định dựng sẵn (đề xuất, cần xác nhận)

| `subject_type` | Người duyệt | Tự duyệt | Hạn (normal / urgent) |
|---|---|---|---|
| `request_type` | `reporter`, `role:admin` | cho phép; **không** cho phép với `hotfix`, `security` (README 3.4: bắt buộc người, xác nhận mức độ) | 72 giờ / 4 giờ |
| `solution` | `reporter`, `role:admin` | cho phép; không cho phép khi `size=L` | 72 giờ / 8 giờ |
| `findings`, `answer` | `reporter`, `role:admin` | cho phép | 7 ngày / 24 giờ |
| `plan`, `task_list` | `reporter`, `role:admin` | cho phép; không cho phép khi `size=L` | 72 giờ / 8 giờ |
| `phase` | `reporter`, `role:admin` | cho phép | 72 giờ / 8 giờ |
| `pre_deploy` | `role:admin` | **không** cho phép | 24 giờ / 2 giờ |

Con số hạn và các cờ là mặc định đề xuất, không phải số liệu đã đo. Mặc định tách nhiệm vụ tắt ở các loại rủi ro thấp để người làm một mình không tự khóa; khóa lại ở `pre_deploy`, `hotfix`, `security`, và size `L`. Tenant một người dùng gặp `REQUEST_APPROVAL_NO_ELIGIBLE_APPROVER` ở các trường hợp đó và phải tạo chính sách nới lỏng (ví dụ thêm `team:` hoặc `allow_requester_approve=true`).

### 2.6 Thông báo qua `notification-service`

Cơ chế thật của `notification-service` (đã đọc code): `internal/adapter/eventbus/consumer.go` có danh sách `Subjects` (`StreamName` + `Subject`); `internal/domain/notification_event.go` có bảng `subjectRules` (`Type`, `Title`, `Body`, `Severity`, `Channels` ws/push); `EventPayload` đọc `user_id`/`user_ids`, `title`, `body`, `deep_link`; thiếu người nhận thì bỏ qua; có dedup theo `EventID`. Kiểu mới cần thêm hai dòng ở mỗi bảng, không đổi lược đồ.

Thay đổi ở `notification-service` (nhỏ):
- `consumer.go`: thêm `{StreamName: "REQUEST", Subject: "orca.request.approval.requested"}` và `{... "orca.request.approval.decided"}`. Tên stream do `request-service` tạo khi publisher khởi động (CR-REQ-001); cần khớp.
- `notification_event.go`, `subjectRules`:
  - `orca.request.approval.requested`: `Type: "request.approval_requested"`, `Title: "Approval needed"`, `Severity: warning`, `Channels: ws, push`.
  - `orca.request.approval.decided`: `Type: "request.approval_decided"`, `Title: "Approval decided"`, `Severity: info`, `Channels: ws, push`. Quy tắc tĩnh theo subject nên không phân biệt được mức độ theo `decision`; tiêu đề cụ thể ("Plan rejected") do `request-service` đặt vào `payload.title`.
- Không trùng với MCP: `mcp.approval` và subject `orca.mcp.approval.requested` đã dùng cho `mcp-service`.

Phía `request-service` (publish):
- `approval.requested`: người nhận = người duyệt đã chụp. Mở rộng `team:` bằng `tenant-service.ListTeamMembers`; `role:admin` bằng `auth-service.ListUsers` lọc `role=admin` (phân trang); `reporter` là `reporter_id`. Loại bỏ người yêu cầu nếu tách nhiệm vụ. Tối đa `REQUEST_APPROVAL_NOTIFY_MAX_RECIPIENTS` (mặc định đề xuất 50) người; vượt thì cắt và log. Kết quả mở rộng được cache theo tenant 60 giây. Bước này chạy ở bộ phát outbox (consumer nội bộ), không ở transaction của `OpenApproval`, và lỗi tra cứu không làm Approval thất bại (chỉ chậm thông báo, retry theo outbox).
- `approval.decided`: người nhận = `reporter_id` và `requested_by` nếu là người, trừ người vừa quyết định.
- Payload: `user_ids`, `title`, `body` ngắn không chứa `comment`, `deep_link` (đề xuất `/?section=requests&request=<id>&approval=<id>`; route do CR-REQ-018/022 chốt). Lý do bỏ `comment`: thông báo được lưu (cùng nguyên tắc của `mcp.approval`).
- Dedup đã có theo `EventID` ở `HandleIncomingEvent`; `request-service` dùng id sự kiện ổn định để lần phát lại không báo hai lần.

### 2.7 Hết hạn và nhắc

Worker `ExpireApprovals` trong `request-service` (mẫu `mcp-service/internal/usecase/approvals.go`), chạy mỗi 60 giây (cấu hình), nhiều replica an toàn vì mỗi lần chuyển là so sánh-và-ghi:
1. Chọn tối đa 100 Approval `status='pending' AND due_at <= <đồng hồ DB>` (`NOW()`/`CURRENT_TIMESTAMP(6)`, không dùng đồng hồ ứng dụng, như lease của CR-TG-008), có khóa dòng `FOR UPDATE SKIP LOCKED` (Postgres; MySQL cần >= 8.0.1 theo CR-DB-002).
2. Mỗi Approval một transaction: `status=expired`, `decided_by='system'`, `decided_at`; `handler.OnClosedWithoutDecision(why="expired")`; outbox `approval.decided` `{decision:"expired"}`.
3. Hệ quả lên Request (chuyển bằng CR-REQ-003/006 `ReturnToBacklog`): Request về `request_backlog`, `return_reason="approval_expired"`, `returned_from_stage` theo chủ thể: `request_type` → `classification`; `solution`/`findings`/`answer` → `analysis`; `plan`/`task_list` → `plan`; `phase` → `phase`; `pre_deploy` → `plan` nếu Request đang `awaiting_plan_approval` (`hotfix`, `security`) hoặc `phase` nếu đang `executing` (`ops_request`). Dùng trigger `return_to_backlog` của CR-REQ-003 với `Stage` và `Reason`. Từ backlog người dùng mở lại (CR-REQ-006). Chưa có RPC gia hạn (mục 7).
4. **Nhắc:** khi đã qua 75% của `(due_at - created_at)` và `reminded_at IS NULL`, phát lại `approval.requested` với `payload.reason="reminder"` và đặt `reminded_at` (so sánh-và-ghi, mỗi Approval một lần). README 3.7 không có sự kiện nhắc riêng (mục 7).
5. Lười hết hạn: `Approve` trên Approval quá `due_at` mà chưa quét trả `REQUEST_APPROVAL_EXPIRED` (CR-REQ-009).
6. Lệch đồng hồ giữa DB và ứng dụng không ảnh hưởng vì mọi so sánh dùng giờ DB.

### 2.8 `ListPendingForUser` theo chính sách

Thay bộ lọc tạm của CR-REQ-009: nạp principal của caller một lần (`user`, các `team`, `role:admin`), truy vấn `approvals` join `approval_approvers` ở `(tenant_id, principal_kind, principal_id)`, thêm nhánh `reporter` join `requests.reporter_id`; loại Approval mà caller bị tách nhiệm vụ loại; phân trang theo `(created_at, id)`. Không dùng toán tử JSON nên cùng một câu SQL cho hai DB.

### 2.9 Lỗi và sự kiện

Mã lỗi mới: `REQUEST_APPROVAL_NOT_APPROVER`, `REQUEST_APPROVAL_SELF_APPROVAL_FORBIDDEN`, `REQUEST_APPROVAL_AGENT_FORBIDDEN` (PermissionDenied); `REQUEST_APPROVAL_NO_ELIGIBLE_APPROVER` (FailedPrecondition); `REQUEST_APPROVAL_POLICY_INVALID` (InvalidArgument); `REQUEST_APPROVAL_POLICY_NOT_FOUND` (NotFound). `REQUEST_APPROVAL_FORBIDDEN` của CR-REQ-009 là mã bao trùm. Không thêm sự kiện mới: dùng `approval.requested` và `approval.decided` (README 3.7).

### 2.10 Tệp sẽ tạo (mới)

`request-service`: `internal/domain/approval_policy.go`, `approval_authorization.go`; `internal/usecase/resolve_approver_policy.go`, `authorize_approval_decision.go`, `expire_approvals.go`, `remind_pending_approvals.go`, `expand_approval_recipients.go`, `manage_approval_policies.go`; `internal/adapter/{postgres,mysql}/approval_policy_repository.go`, `approval_approver_repository.go`; `internal/adapter/grpcclient/team_membership_resolver.go`, `admin_directory_resolver.go`; migration `NNNN_approval_policies.{up,down}.sql`. `notification-service`: sửa `consumer.go`, `notification_event.go` và test của chúng.

## 3. Quyết định thiết kế

| # | Quyết định | Lý do |
|---|---|---|
| 1 | Chính sách lưu DB, đánh giá bằng Go thuần | Quy tắc phụ thuộc thuộc tính Request và đội nhóm; OPA hiện chỉ dùng cho mức grant. Dễ test hơn rego khi chưa có nhu cầu đồng bộ bundle. |
| 2 | Chụp người duyệt lúc mở (`approval_approvers`) | Đổi chính sách giữa chừng không làm cổng đang chờ đổi nghĩa; join portable cho hai DB. |
| 3 | `role:admin` luôn được tính | Tránh cổng kẹt khi chính sách sai; admin vẫn chịu tách nhiệm vụ. |
| 4 | Không tách nhiệm vụ mặc định ở loại rủi ro thấp | Người làm một mình không bị khóa; bật tại nơi README yêu cầu người (`hotfix`, `security`) và `pre_deploy`. |
| 5 | Thất bại sớm khi không có người duyệt hợp lệ | Một cổng không ai duyệt được chỉ chờ tới khi hết hạn. |
| 6 | Agent/MCP không duyệt | Cổng tồn tại để người kiểm soát agent. |
| 7 | Mở rộng team/admin thành người ở `request-service` | `notification-service` chỉ biết `user_ids`. |
| 8 | Hết hạn đưa Request về backlog, không tự chuyển tiếp | Không có quyết định thì không đi tiếp; backlog có lý do để mở lại. |
| 9 | Giờ DB cho `due_at` | Nhiều replica, lệch đồng hồ ứng dụng. |
| 10 | Nhắc dùng lại `approval.requested` | README 3.7 chưa có sự kiện nhắc; tránh tự thêm sự kiện ngoài hợp đồng. |

## 4. Tiêu chí chấp nhận

- [ ] Migration `approval_policies` và `approval_approvers` chạy lên và xuống trên Postgres và MySQL 8.0.1+.
- [ ] Chọn chính sách: dòng cụ thể hơn thắng dòng chung; cùng độ cụ thể thì `priority` cao thắng; không khớp thì dùng mặc định bảng 2.5 (test bảng đủ 8 `subject_type`).
- [ ] Người không thuộc principal nào gọi `Approve` nhận `REQUEST_APPROVAL_NOT_APPROVER`; thành viên của `team:<id>` trong chính sách được duyệt.
- [ ] Với `self_approval_allowed=false`, `reporter_id` (kể cả admin) gọi `Approve` nhận `REQUEST_APPROVAL_SELF_APPROVAL_FORBIDDEN`; người khác trong tập thì được.
- [ ] Mở Approval `pre_deploy` khi chỉ người yêu cầu là admin duy nhất trả `REQUEST_APPROVAL_NO_ELIGIBLE_APPROVER` và không tạo dòng `approvals`.
- [ ] `hotfix` và `security` mặc định tạo Approval `request_type` với `self_approval_allowed=false`.
- [ ] Chính sách đổi sau khi mở không làm đổi tập người duyệt của Approval đang `pending`.
- [ ] `ListPendingForUser` cho user chỉ trả Approval mà user duyệt được, và cho cùng kết quả trên Postgres và MySQL với bộ dữ liệu mẫu.
- [ ] `approval.requested` đến `notification-service` tạo `NotificationEvent` kiểu `request.approval_requested` cho đúng người; payload không chứa `comment`; dedup đúng khi phát lại.
- [ ] Approval quá `due_at` được worker chuyển `expired` trong một chu kỳ quét; Request về `request_backlog` với `return_reason="approval_expired"` và `returned_from_stage` đúng bảng 2.7; hai replica chạy song song không xử lý trùng.
- [ ] Nhắc gửi đúng một lần mỗi Approval ở mốc 75%.
- [ ] Lời gọi `Approve` mang nguồn máy/MCP bị từ chối `REQUEST_APPROVAL_AGENT_FORBIDDEN` (khi cơ chế nhận biết có; xem mục 7).

## 5. Kiểm thử

- **Unit (domain):** `ApprovalAuthorization.Decide` bảng đúng/sai đủ tổ hợp (principal x tách nhiệm vụ x admin); chọn chính sách theo độ cụ thể; tính mốc nhắc.
- **Unit (usecase, fake tenant-service/auth-service/clock):** mở rộng người nhận, cắt ở mức tối đa, lỗi tra cứu không làm hỏng Approval, hết hạn từng loại chủ thể, nhắc một lần.
- **Integration trên Postgres và MySQL:** `ListPendingForUser` cùng kết quả; `FOR UPDATE SKIP LOCKED` với hai worker; so sánh-và-ghi hết hạn dưới đua với `Approve`; chụp `approval_approvers` cùng transaction với `OpenApproval`.
- **Hợp đồng với `notification-service`:** test trong `notification_event_test.go` cho hai subject mới (người nhận, kiểu, kênh, `ErrNoRecipients` khi payload rỗng); test golden payload do `request-service` phát phải được `TranslateEvent` chấp nhận.
- **Chưa kiểm chứng:** tên stream `REQUEST` khớp giữa publisher và consumer; hiệu năng `auth-service.ListUsers` khi tenant lớn; hành vi push thật.

## 6. Rủi ro và điểm chưa kiểm chứng

- Mô hình vai trò nghèo (`user|admin` + team): "người có thẩm quyền duyệt" của tổ chức thật (tech lead, security) phải dựng bằng team. Chưa kiểm chứng rằng tenant thực tế tổ chức team như vậy.
- Tách nhiệm vụ có thể khóa tenant nhỏ; giảm thiểu bằng lỗi sớm và chính sách ghi đè, nhưng trải nghiệm chưa thử.
- Mặc định hạn và cờ là con số đề xuất; hết hạn đưa Request về backlog có thể gây phiền nếu hạn quá ngắn.
- Mở rộng admin qua `ListUsers` phân trang tốn kém ở tenant lớn; cache 60 giây và trần 50 người là giải pháp tạm.
- Nhận biết nguồn gọi MCP chưa có; trước khi có, chỉ chặn bằng việc không đăng ký tool duyệt (CR-REQ-017).
- Thông báo được lưu lâu dài; chỉ chứa tiêu đề ngắn, không nội dung Plan/Solution.
- `notification-service` có `Locked` rule và danh sách subject cố định trong mã: đổi cần triển khai lại service đó (phối hợp phát hành).

## 7. Câu hỏi mở

1. **Mô hình vai trò:** có cần vai trò "approver" riêng cho tổ chức (README 7: "quyền duyệt chưa được thiết kế ở mức tổ chức; CR-REQ-010 chỉ đặt mặc định")? CR này chỉ dùng `admin` và team.
2. **RPC quản trị chính sách** không có trong README 3.6; chấp nhận thêm `ApprovalPolicyAdminService` hay chỉ seed mặc định ở v1?
3. **Sự kiện nhắc/hết hạn riêng** (`approval.reminder`) không có trong README 3.7; CR này dùng `reason` trong `approval.requested` và `decision=expired` trong `approval.decided`. Cần xác nhận.
4. **Gia hạn Approval** (RPC `Extend`): README 3.6 không có; hiện hết hạn là về backlog rồi mở lại. Có cần không?
5. **Tách nhiệm vụ cho Plan/Phase:** "người yêu cầu" là người báo cáo Request, hay người kích hoạt sinh Plan? CR này chọn người báo cáo.
6. **Nguồn gọi MCP:** cơ chế truyền từ `mcp-service` qua gateway tới `request-service` chưa tồn tại; thuộc CR-REQ-016/017.
7. Các mặc định hạn (72 giờ, 4 giờ, 8 giờ...) và cờ tách nhiệm vụ ở bảng 2.5 cần chủ sản phẩm xác nhận.

## 8. Tham chiếu

- `/opt/repos/orca/docs/crs/v6/README.md` (3.4, 3.7, 6, 7)
- `/opt/repos/orca/backend-go/services/notification-service/internal/adapter/eventbus/consumer.go` (`Subjects`)
- `/opt/repos/orca/backend-go/services/notification-service/internal/domain/notification_event.go` (`subjectRules`, `EventPayload`, `ErrNoRecipients`)
- `/opt/repos/orca/backend-go/services/notification-service/internal/usecase/handle_incoming_event.go` (dedup, persist, broadcast, push)
- `/opt/repos/orca/backend-go/services/auth-service/internal/domain/user.go` (`Role`)
- `/opt/repos/orca/backend-go/services/tenant-service/internal/domain/team.go`; `proto/orca/tenant/v1/tenant.proto` (`ListTeamsForUser`, `ListTeamMembers`); `proto/orca/auth/v1/auth.proto` (`ListUsers`)
- `/opt/repos/orca/backend-go/services/task-service/internal/domain/grant.go`, `grant_resolution.go`; `internal/adapter/grpcclient/team_scope_resolver.go`
- `/opt/repos/orca/backend-go/policy/orca-authz/task_grant.rego`, `admin.rego`
- `/opt/repos/orca/backend-go/services/mcp-service/internal/usecase/approvals.go` (`ExpireApprovals`)
- `/opt/repos/orca/backend-go/services/orchestration-service/internal/usecase/resolve_gate.go`
- `/opt/repos/orca/backend-go/common/tenant/tenant.go` (`Role`)
- `/opt/repos/orca/docs/crs/v6/approval/CR-REQ-009-generic-approval-domain-and-api.md`
