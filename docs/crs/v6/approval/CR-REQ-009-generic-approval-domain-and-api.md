# CR-REQ-009 — Approval tổng quát: domain, bảng, API và vòng đời

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-REQ-009 |
| **Tên** | Thực thể `Approval` tổng quát, máy trạng thái, `ApprovalService` và cơ chế `SubjectHandler` kích hoạt chuyển trạng thái Request |
| **Loại** | Feature |
| **Priority** | 🔴 P0 |
| **Effort** | Large |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-05 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | CR-REQ-001 (service, outbox), CR-REQ-002 (bảng `requests`), CR-REQ-003 (use case chuyển trạng thái Request) |
| **Mở khoá** | CR-REQ-010 (chính sách quyền, thông báo, hết hạn), CR-REQ-005, 007, 008, 012, 013, 014 (đăng ký `SubjectHandler`), CR-REQ-016, 022 |
| **Tác động** | `request-service` (domain, usecase, adapter postgres/mysql, grpc, migration, proto). Không đổi service khác; `DecisionGate` của `orchestration-service` giữ nguyên (README O4) |

## 1. Bối cảnh và vấn đề

1. Mọi cấp của luồng Request cần một cổng người duyệt: loại, Solution, Plan, Phase, danh sách task, trước deploy (README 3.4). Hiện không có thực thể tổng quát nào.
2. Ba thứ gần giống đã có nhưng không dùng được:
   - `DecisionGate` (`orchestration-service/internal/domain/orchestration.go`): gắn với `OrchestrationTask` và `DispatchContext`; trạng thái `pending|resolved|timeout`; `Resolve` nhận chuỗi tự do `OutcomeJSON`; không kiểm quyền người giải (`resolve_gate.go` chỉ kiểm tenant); không có UI. Là câu hỏi của agent điều phối, không phải cổng duyệt. README O4: không hợp nhất trong v6.
   - `Approval` của `mcp-service` (`internal/domain/approval.go`): duyệt lời gọi tool của AI agent, trạng thái `pending|approved|denied|expired|cancelled`, thuộc `UserID` duy nhất. Khác miền (không có Request, không có người duyệt theo vai trò). Có hai thứ đáng học: `ParamsHash` chống đổi nội dung sau khi hiển thị, và `ExpireApprovals` đóng hạn bằng so sánh-và-ghi.
   - `Approval` của `workflow-service`: duyệt template, không phải cổng của bước chạy (nghiên cứu mục 2.3).
3. Cần một nơi duy nhất ghi: ai yêu cầu duyệt cái gì, ai quyết định, khi nào, lý do, và hiệu ứng lên Request, để audit và hộp duyệt (CR-REQ-022) đọc.

## 2. Giải pháp đề xuất

### 2.1 Phạm vi sở hữu

CR này sở hữu: bảng `approvals`, máy trạng thái Approval, `ApprovalService` (README 3.6), việc `Approve`/`Reject` kích hoạt chuyển trạng thái Request (qua use case `TransitionRequest` của CR-REQ-003, chỉ gọi), và sự kiện `approval.requested`, `approval.decided`. Không sở hữu: ai được duyệt, tách nhiệm vụ, thông báo, hết hạn (CR-REQ-010); máy trạng thái Request (CR-REQ-003); nội dung chủ thể (Solution: 007/008; Plan, Phase: 011/012; `request_type`: 005).

### 2.2 Máy trạng thái

```
pending ──Approve──▶ approved
pending ──Reject───▶ rejected      (comment bắt buộc)
pending ──Cancel───▶ cancelled     (người yêu cầu, hoặc hệ thống khi Request đổi loại, bị hủy, hoặc về backlog)
pending ──hết hạn──▶ expired       (CR-REQ-010)
```
Bốn trạng thái cuối là kết thúc, không có cạnh ra. Mọi chuyển trạng thái là so sánh-và-ghi: `UPDATE approvals SET status=?, ..., version=version+1 WHERE id=? AND tenant_id=? AND status='pending' AND version=?`; 0 dòng bị ảnh hưởng thì đọc lại để phân biệt "đã quyết định" với "xung đột phiên bản". Domain `Approval` có phương thức `Approve`, `Reject`, `Cancel`, `Expire` trả lỗi miền khi trạng thái không phải `pending`.

### 2.3 Bảng `approvals`

README 3.5 liệt kê trường chính; CR này định nghĩa DDL đầy đủ và thêm 5 cột mà README chưa có (đánh dấu **mới**, xem mục 7).

| Cột | Postgres (schema `request`) | MySQL | Ghi chú |
|---|---|---|---|
| `id` | `UUID PK` | `CHAR(36) PK` | |
| `tenant_id` | `UUID NOT NULL` | `CHAR(36) NOT NULL` | RLS `tenant_isolation` (như `task.task_sources`) |
| `request_id` | `UUID NOT NULL` | `CHAR(36) NOT NULL` | FK nội bộ tới `requests(id)` |
| `subject_type` | `TEXT CHECK IN ('request_type','solution','findings','answer','plan','phase','task_list','pre_deploy')` | `VARCHAR(20)` + CHECK | khớp nghiên cứu 4.2 mục 3 |
| `subject_id` | `TEXT NOT NULL` | `VARCHAR(64) NOT NULL` | `solution`/`findings`/`answer`: `solutions.id`; `plan`/`phase`: id Task bên `task-service` (không FK); `task_list`: id Task `plan`; `request_type`, `pre_deploy`: id Request |
| `stage` | `TEXT NOT NULL` | `VARCHAR(40) NOT NULL` | trạng thái Request mà cổng này chặn: `awaiting_type_confirmation`, `awaiting_analysis_approval`, `awaiting_plan_approval` (gồm `pre_deploy` của `hotfix`/`security`, theo CR-REQ-003), hoặc `executing` (`phase`, `pre_deploy` của `ops_request`); README chưa định nghĩa `stage`, xem mục 7 |
| `status` | `TEXT CHECK IN ('pending','approved','rejected','cancelled','expired')` | tương tự | |
| `requested_by` | `TEXT NOT NULL` | `VARCHAR(64) NOT NULL` | id người dùng, hoặc `system` khi AI/hệ thống mở cổng |
| `decided_by` | `TEXT NULL` | `VARCHAR(64) NULL` | `system` khi hết hạn hoặc tự hủy |
| `decided_at` | `TIMESTAMPTZ NULL` | `TIMESTAMP(6) NULL` | đồng hồ DB |
| `comment` | `TEXT NOT NULL DEFAULT ''` | `TEXT` | tối đa 2000 ký tự |
| `due_at` | `TIMESTAMPTZ NULL` | `TIMESTAMP(6) NULL` | CR-REQ-010 đặt giá trị |
| `version` | `BIGINT NOT NULL DEFAULT 1` | | khóa lạc quan |
| `subject_digest` **mới** | `TEXT NOT NULL` | `CHAR(64) NOT NULL` | sha256 nội dung chủ thể lúc mở; chống duyệt nội dung đã đổi |
| `self_approval_allowed` **mới** | `BOOLEAN NOT NULL DEFAULT TRUE` | `TINYINT(1)` | CR-REQ-010 điền |
| `idempotency_key` **mới** | `TEXT NULL` | `VARCHAR(128) NULL` | |
| `reminded_at` **mới** | `TIMESTAMPTZ NULL` | `TIMESTAMP(6) NULL` | CR-REQ-010 |
| `created_at`, `updated_at` **mới** | `TIMESTAMPTZ NOT NULL DEFAULT now()` | `TIMESTAMP(6) ... DEFAULT CURRENT_TIMESTAMP(6)` | |

Ràng buộc và chỉ mục:
- **Một Approval `pending` cho mỗi chủ thể:** Postgres `CREATE UNIQUE INDEX ... (tenant_id, subject_type, subject_id) WHERE status='pending'`. MySQL không có partial index: cột sinh `pending_key VARCHAR(120) GENERATED ALWAYS AS (IF(status='pending', CONCAT(subject_type,':',subject_id), NULL)) STORED` kèm `UNIQUE KEY (tenant_id, pending_key)` (cùng thủ thuật `project_key` ở migration MySQL 0012 của task-service).
- `UNIQUE (tenant_id, idempotency_key)` khi khác NULL.
- Chỉ mục đọc: `(tenant_id, request_id, created_at)`, `(tenant_id, status, due_at)` (quét hết hạn), `(tenant_id, status, created_at)` (hộp duyệt).
- Một Approval cũ ở trạng thái cuối không bị xóa; chủ thể mới (đã sửa) tạo dòng mới.

### 2.4 Cơ chế `SubjectHandler`

Cổng mở cho nhiều loại chủ thể, mỗi loại do CR sở hữu nội dung cài đặt. Port trong `usecase/approval_subject_handler.go` (mới):

```go
type SubjectHandler interface {
    // Gọi khi mở: chủ thể tồn tại, thuộc Request, ở trạng thái duyệt được; trả digest.
    ValidateForRequest(ctx context.Context, req Request, subjectID string) (digest string, err error)
    // Ba hàm sau chạy TRONG transaction của quyết định; chỉ ghi DB của request-service.
    OnApproved(ctx context.Context, tx Tx, a Approval) error
    OnRejected(ctx context.Context, tx Tx, a Approval) error
    OnClosedWithoutDecision(ctx context.Context, tx Tx, a Approval, why string) error // cancelled|expired
}
```
Đăng ký theo `subject_type` lúc khởi động; thiếu handler cho một `subject_type` là lỗi khởi động. Chủ sở hữu:

| `subject_type` | CR cài `SubjectHandler` |
|---|---|
| `request_type` | CR-REQ-005 (`ConfirmRequestType` gọi `Approve` trên Approval này, một đường duy nhất) |
| `solution` (kind `solution` và `diagnosis`) | CR-REQ-007 |
| `findings`, `answer` | CR-REQ-008 |
| `plan`, `task_list` | CR-REQ-012 |
| `phase` | CR-REQ-012/013 |
| `pre_deploy` | CR-REQ-014 |

Quy tắc: handler trong transaction chỉ ghi DB của `request-service` và chuyển trạng thái Request qua `TransitionRequest` (CR-REQ-003). Hiệu ứng sang `task-service` (kích hoạt Phase, cho phép Task chạy) đi bằng sự kiện outbox `approval.decided` và consumer idempotent của CR-REQ-012/013, không gọi gRPC trong transaction.

### 2.5 Use case

- `OpenApproval` (nội bộ, dùng bởi 005/007/008/012...): `RequireTenantID`; Request tồn tại và loại có cổng này theo registry (CR-REQ-003; sai thì `REQUEST_APPROVAL_SUBJECT_TYPE_NOT_ALLOWED`); `handler.ValidateForRequest` lấy `subject_digest`; gọi `ApproverPolicy.Resolve` (port; CR-REQ-010 cài, mục 2.7) lấy `due_at`, `self_approval_allowed`; chèn dòng `pending`; outbox `approval.requested`. Nếu đã có `pending` cho chủ thể: trả dòng đó khi `subject_digest` trùng, ngược lại `REQUEST_APPROVAL_PENDING_EXISTS`.
- Cổng cho CR-REQ-005: `OpenApproval` cài `ApprovalRecorder` và `CancelPendingForRequest` cài `ApprovalCanceller` (hai cổng no-op của CR-REQ-005). `request_type` được người sửa rồi xác nhận: `ConfirmRequestType` gửi `expected_digest` của đề xuất người đó đã xem, `OnApproved` áp phần sửa.
- `Approve(approval_id, comment, expected_version, expected_digest)`: transaction, `SELECT ... FOR UPDATE` dòng Approval (hai DB đều hỗ trợ); kiểm `status='pending'` (hết hạn nhưng chưa quét: coi là `expired`, ghi lười và trả `REQUEST_APPROVAL_EXPIRED`, như `EffectiveStatus` của `mcp-service`); `ApprovalAuthorizer.CanDecide` (port; CR-REQ-010); `expected_digest` khớp `subject_digest`; Request đang đúng `stage` (`REQUEST_APPROVAL_STAGE_MISMATCH`); cập nhật Approval; `handler.OnApproved`; outbox `approval.decided` `{decision:"approved"}`. Mọi bước cùng một transaction; lỗi bất kỳ thì rollback.
- `Reject(approval_id, comment, expected_version)`: như trên, `comment` bắt buộc (`REQUEST_APPROVAL_COMMENT_REQUIRED`), `handler.OnRejected`.
- `Cancel(approval_id, reason)`: người yêu cầu, hoặc admin; dùng cùng `OnClosedWithoutDecision`.
- `UpdatePendingDigest(subject_type, subject_id, digest)` (repository): CR-REQ-007 dùng khi người duyệt chọn phương án; chỉ ghi khi còn `pending`.
- `CancelPendingForRequest(request_id, reason)`: do CR-REQ-003/005/006 gọi khi đổi loại, hủy, hoặc về backlog; hủy mọi `pending` của Request trong cùng transaction của lệnh đó.
- Lặp lại cùng quyết định bởi cùng người trên Approval đã đóng: trả kết quả hiện có, không lỗi (idempotent). Quyết định khác hoặc người khác: `REQUEST_APPROVAL_ALREADY_DECIDED`.
- `ListApprovals`, `GetApproval`, `ListPendingForUser`: chỉ đọc. Ở CR này `ListPendingForUser` dùng bộ lọc tạm: admin thấy mọi `pending` trong tenant; người khác thấy `pending` của Request có `reporter_id` là mình. CR-REQ-010 thay bằng lọc theo chính sách.
- Cài tạm cho `ApproverPolicy`/`ApprovalAuthorizer` trong CR này: người duyệt hợp lệ là `role=admin` (`tenant.Role(ctx)`, rỗng coi là không phải admin) hoặc `reporter_id`; `due_at` NULL; `self_approval_allowed=true`.

### 2.6 Proto (`proto/orca/request/v1/approval.proto`, mới)

```proto
enum ApprovalSubjectType { APPROVAL_SUBJECT_TYPE_UNSPECIFIED=0; REQUEST_TYPE=1; SOLUTION=2; FINDINGS=3; ANSWER=4; PLAN=5; PHASE=6; TASK_LIST=7; PRE_DEPLOY=8; }
enum ApprovalStatus { APPROVAL_STATUS_UNSPECIFIED=0; PENDING=1; APPROVED=2; REJECTED=3; CANCELLED=4; EXPIRED=5; }
message Approval { string id=1; string request_id=2; ApprovalSubjectType subject_type=3; string subject_id=4; string stage=5; ApprovalStatus status=6;
  string requested_by=7; string decided_by=8; google.protobuf.Timestamp decided_at=9; string comment=10; google.protobuf.Timestamp due_at=11;
  int64 version=12; string subject_digest=13; google.protobuf.Timestamp created_at=14; }
service ApprovalService {
  rpc RequestApproval(RequestApprovalRequest) returns (Approval);      // {request_id, subject_type, subject_id, comment, idempotency_key}
  rpc Approve(DecideApprovalRequest) returns (DecideApprovalResponse);  // {approval_id, comment, expected_version, expected_digest}
  rpc Reject(DecideApprovalRequest) returns (DecideApprovalResponse);
  rpc Cancel(CancelApprovalRequest) returns (Approval);                 // {approval_id, reason}
  rpc GetApproval(GetApprovalRequest) returns (Approval);
  rpc ListApprovals(ListApprovalsRequest) returns (ListApprovalsResponse);       // {request_id, subject_type, status, page_size, page_token}
  rpc ListPendingForUser(ListPendingForUserRequest) returns (ListPendingForUserResponse); // {subject_type, page_size, page_token}
}
message DecideApprovalResponse { Approval approval = 1; string request_status = 2; } // trạng thái Request sau chuyển
```
`RequestApproval` qua RPC chỉ cho `pre_deploy` và mở lại cổng đã đóng; các `subject_type` khác chỉ mở nội bộ bởi use case sở hữu (tránh người dùng tự tạo cổng giả). Vi phạm: `REQUEST_APPROVAL_SUBJECT_TYPE_NOT_ALLOWED`.

### 2.7 Lỗi, sự kiện, quyền

- Lỗi (`apperrors`): `REQUEST_APPROVAL_NOT_FOUND` (NotFound), `REQUEST_APPROVAL_ALREADY_DECIDED`, `REQUEST_APPROVAL_EXPIRED`, `REQUEST_APPROVAL_VERSION_CONFLICT`, `REQUEST_APPROVAL_DIGEST_MISMATCH`, `REQUEST_APPROVAL_STAGE_MISMATCH`, `REQUEST_APPROVAL_SUBJECT_NOT_FOUND` (đều FailedPrecondition trừ NotFound), `REQUEST_APPROVAL_COMMENT_REQUIRED` (InvalidArgument), `REQUEST_APPROVAL_PENDING_EXISTS` (AlreadyExists), `REQUEST_APPROVAL_SUBJECT_TYPE_NOT_ALLOWED` (FailedPrecondition), `REQUEST_APPROVAL_FORBIDDEN` (PermissionDenied; mã chi tiết do CR-REQ-010).
- Sự kiện (outbox, subject `orca.request.approval.requested`, `orca.request.approval.decided`): `requested` `{approval_id, request_id, subject_type, subject_id, stage, requested_by, due_at}`; `decided` `{approval_id, request_id, subject_type, subject_id, decision: approved|rejected|cancelled|expired, decided_by}`. README chỉ có hai sự kiện nên `decided` dùng `decision` thay cho sự kiện riêng cho hủy và hết hạn. Payload không chứa `comment` (thông báo được lưu).
- Tên không trùng với MCP: subject `orca.mcp.approval.requested` và kiểu `mcp.approval` đã thuộc `mcp-service`; ở đây là `orca.request.approval.*`.
- Quyền: mặc định như 2.5; mọi lệnh ghi qua gateway kiểm quyền (CR-REQ-016).

### 2.8 Tệp sẽ tạo trong `backend-go/services/request-service/` (mới)

`internal/domain/approval.go`, `approval_subject.go`; `internal/usecase/open_approval.go`, `decide_approval.go` (Approve/Reject), `cancel_approval.go`, `cancel_pending_approvals_for_request.go`, `list_approvals.go`, `list_pending_approvals_for_user.go`, `approval_subject_handler.go` (port), `approver_policy_ports.go`; `internal/adapter/postgres/approval_repository.go`, `internal/adapter/mysql/approval_repository.go`; `internal/adapter/grpc/approval_server.go`; migration `NNNN_approvals.{up,down}.sql` cho cả hai dialect; `proto/orca/request/v1/approval.proto`.

## 3. Quyết định thiết kế

| # | Quyết định | Lý do |
|---|---|---|
| 1 | Bảng `approvals` mới trong `request-service`, không dùng `DecisionGate` | README D3/O4; `DecisionGate` thiếu kiểm quyền và gắn vào điều phối. |
| 2 | Một cổng mở cho nhiều chủ thể bằng `SubjectHandler` | Mỗi CR sở hữu nội dung chủ thể; Approval chỉ điều phối. |
| 3 | `Approve` và chuyển trạng thái Request cùng một transaction | Không có trạng thái "đã duyệt nhưng Request chưa đi tiếp". |
| 4 | Hiệu ứng sang `task-service` qua outbox | Không gọi gRPC trong transaction; consumer idempotent theo mẫu `ReportTaskExecutionResult`. |
| 5 | `subject_digest` + `expected_digest` | Chống duyệt nội dung đã đổi (tiền lệ `ParamsHash`). |
| 6 | Một `pending` cho mỗi chủ thể, bằng chỉ mục duy nhất | Chặn cổng trùng khi hai tiến trình cùng mở; hai dialect cùng ngữ nghĩa. |
| 7 | Bốn trạng thái cuối, không mở lại; chủ thể mới tạo dòng mới | Lịch sử audit bất biến. |
| 8 | `reject` bắt buộc `comment` | Nghiên cứu 4.2 mục 3; lý do là đầu vào để sinh lại hoặc trả backlog. |
| 9 | `RequestApproval` qua RPC giới hạn | Người dùng không tự tạo cổng `plan` giả. |
| 10 | Không dùng chuỗi tự do như `OutcomeJSON` | Quyết định có kiểu, kiểm được. |

## 4. Tiêu chí chấp nhận

- [ ] Migration `approvals` chạy lên và xuống trên Postgres và MySQL 8.0.1+; CHECK của `subject_type` và `status` từ chối giá trị lạ ở cả hai DB.
- [ ] Hai `OpenApproval` đồng thời cho cùng `(subject_type, subject_id)`: chỉ một dòng `pending` (cả hai DB); lần hai trả dòng cũ khi digest giống, `REQUEST_APPROVAL_PENDING_EXISTS` khi khác.
- [ ] `Approve` trên `pending`: Approval `approved`, `handler.OnApproved` chạy, Request chuyển trạng thái, outbox `approval.decided` có `decision=approved`, tất cả cùng một transaction; ép `OnApproved` lỗi thì Approval vẫn `pending`.
- [ ] `Reject` không có `comment` trả `REQUEST_APPROVAL_COMMENT_REQUIRED`; có `comment` thì Approval `rejected` và `OnRejected` chạy.
- [ ] Hai `Approve` đồng thời: một thành công, một nhận `REQUEST_APPROVAL_ALREADY_DECIDED`; `expected_version` cũ nhận `REQUEST_APPROVAL_VERSION_CONFLICT`.
- [ ] `Approve` lặp bởi cùng người trên Approval đã `approved` trả thành công, không phát thêm sự kiện.
- [ ] Sửa chủ thể sau khi mở làm `Approve` trả `REQUEST_APPROVAL_DIGEST_MISMATCH`.
- [ ] Request đã rời `stage` (ví dụ đổi loại) thì `Approve` trả `REQUEST_APPROVAL_STAGE_MISMATCH`; `CancelPendingForRequest` đưa mọi `pending` của Request sang `cancelled`.
- [ ] Mỗi `subject_type` trong CHECK có `SubjectHandler` đã đăng ký, thiếu thì service không khởi động.
- [ ] Payload `approval.*` không có `comment`; subject đúng `orca.request.approval.requested` và `.decided`.
- [ ] Truy vấn chéo tenant trả `REQUEST_APPROVAL_NOT_FOUND`; mọi use case gọi `tenant.RequireTenantID`.

## 5. Kiểm thử

- **Unit (domain):** bảng chuyển trạng thái (mọi cặp, kể cả bất hợp lệ), `subject_digest`.
- **Unit (usecase, fake repo và handler):** mọi nhánh lỗi mục 2.7, idempotent, rollback khi handler lỗi, lười hết hạn.
- **Integration trên Postgres và MySQL:** một bộ test repository chạy với hai DSN (`common/dbcapability`): chỉ mục `pending` duy nhất, `SELECT ... FOR UPDATE` tuần tự hóa hai `Approve`, transaction với `SubjectHandler` thật của 007 và outbox, phân trang `ListPendingForUser`.
- **Hợp đồng:** `buf breaking` cho `approval.proto`; test mọi `subject_type` có handler; test golden payload `approval.requested`/`decided`; test `notification-service` dịch được payload (CR-REQ-010).
- **Chưa kiểm chứng:** hành vi `FOR UPDATE` dưới tải thật; sự tương thích của `request-service` với mẫu outbox `common/outbox` khi hai replica (CR-REQ-001).

## 6. Rủi ro và điểm chưa kiểm chứng

- Tám loại chủ thể, handler do sáu CR cài (005, 007, 008, 012, 013, 014): lệch hợp đồng `SubjectHandler` giữa các CR là rủi ro lớn nhất; cần test hợp đồng chạy chung.
- `request_type` qua cùng Approval làm `ConfirmRequestType` thành vỏ mỏng; CR-REQ-005 phải tuân (câu hỏi 3).
- Giữ transaction dài khi handler gọi `TransitionRequest` (nhiều bảng): cần kiểm khóa chết với `FOR UPDATE` thứ tự Approval rồi Request; thứ tự khóa phải cố định trong cả hai dialect.
- `plan`/`phase` có `subject_id` là id Task của `task-service`; Task bị xóa thì Approval mồ côi. Không có FK chéo service; `ValidateForRequest` chỉ kiểm lúc mở.
- `tenant.Role(ctx)` chỉ có `user|admin` (`auth-service/internal/domain/user.go`); mô hình vai trò mịn hơn chưa tồn tại, dồn sang CR-REQ-010.

## 7. Câu hỏi mở

1. **README thiếu cột:** `subject_digest`, `self_approval_allowed`, `idempotency_key`, `reminded_at`, `created_at`/`updated_at` chưa có ở 3.5. Đề xuất cập nhật README (CR-REQ-002 không sở hữu bảng này nên migration nằm ở CR này).
2. **Nghĩa của `stage`:** README không định nghĩa. CR này chọn "trạng thái Request mà cổng chặn". Cần xác nhận với CR-REQ-003.
3. **`ConfirmRequestType` và Approval `request_type`:** README có cả hai. CR này coi `ConfirmRequestType` là lệnh gọi `Approve` bên trong. CR-REQ-005 cần xác nhận, nếu không sẽ có hai đường xác nhận loại.
4. **Sự kiện `approval.decided` dùng chung cho hủy và hết hạn:** có chấp nhận `decision` thay vì thêm sự kiện riêng vào README 3.7 không?
5. **Phát hiện nguồn gọi MCP:** README 6 yêu cầu tool MCP ghi qua chính sách `mcp-service`. Việc `Approve`/`Reject` do agent MCP gọi cần bị chặn mặc định (CR-REQ-010 mục 2.4); chưa có cơ chế truyền "nguồn gọi" từ `mcp-service` qua gateway tới service (chưa kiểm chứng).

## 8. Tham chiếu

- `/opt/repos/orca/docs/crs/v6/README.md` (3.3 đến 3.7, 6)
- `/opt/repos/orca/docs/research/receive-request/request-pipeline-existing-capabilities-and-build-scope.md` (4.2 mục 3)
- `/opt/repos/orca/backend-go/services/orchestration-service/internal/domain/orchestration.go` (`DecisionGate`)
- `/opt/repos/orca/backend-go/services/orchestration-service/internal/usecase/create_gate.go`, `resolve_gate.go`
- `/opt/repos/orca/backend-go/services/mcp-service/internal/domain/approval.go`, `internal/usecase/approvals.go` (`ExpireApprovals`)
- `/opt/repos/orca/backend-go/services/task-service/internal/usecase/report_execution_result.go` (idempotent callback)
- `/opt/repos/orca/backend-go/services/task-service/migrations/mysql/0012_task_sources.up.sql`, `postgres/0012_task_sources.up.sql`
- `/opt/repos/orca/backend-go/common/tenant/tenant.go`, `common/apperrors/apperrors.go`, `common/outbox/outbox.go`, `common/dbcapability/capability.go`
- `/opt/repos/orca/docs/crs/v6/approval/CR-REQ-010-approval-authorization-notification-expiry.md`
