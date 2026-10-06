# BE-REQ-SOL-009: Approval tổng quát (domain, bảng, API, `SubjectHandler`)

> **📋 Proposed** (chưa triển khai). Thiết kế thực thi cho CR-REQ-009 trong `request-service` (mới).

**CR:** [CR-REQ-009](../../../../../../docs/crs/v6/approval/CR-REQ-009-generic-approval-domain-and-api.md)
**Service:** `request-service` (mới) · `proto`
**TDD tham chiếu:** [`arch/03`](../../../../tdd/architecture/03-clean-architecture-guidelines.md), [`arch/05`](../../../../tdd/architecture/05-data-architecture.md) (transaction, outbox, hai dialect), [`arch/07`](../../../../tdd/architecture/07-security-architecture.md) (tenant, RLS), [`arch/08`](../../../../tdd/architecture/08-inter-service-communication.md) (subject `orca.request.approval.*`), [`arch/09`](../../../../tdd/architecture/09-observability-reliability.md), [`services/orchestration-service`](../../../../tdd/services/orchestration-service.md) (DecisionGate, giữ riêng), [`services/auth-service`](../../../../tdd/services/auth-service.md)

---

## 1. Trạng thái hiện tại (re-verify)

Đã đọc: CR-REQ-009, `orchestration-service/internal/usecase/{create_gate.go,resolve_gate.go}` (chỉ kiểm tenant, không kiểm người giải), `mcp-service/internal/domain/{approval.go,params_hash.go}` và `internal/usecase/approvals.go` (`ExpireApprovals`, `EffectiveStatus`), `common/outbox/outbox.go`, `common/tenant/tenant.go`. `request-service` chưa tồn tại; mọi file "(mới)".

**Correction relative to CR-REQ-009**
1. **Migration number.** Service chưa có thư mục migration; số thật đọc lúc làm task 01 (dự kiến sau `0002_request_core` của CR-REQ-002, nhưng các CR 004/006 cũng thêm). Gọi `NNNN_approvals`. Vì Approval là điều kiện của SOL-007, nên migration này đi trước `NNNN_analysis_runs`.
2. **Khoá FK nội bộ:** `approvals.request_id` tham chiếu `requests(id)` (cùng service) nhưng `subject_id` không FK (Task bên `task-service`).
3. **Thứ tự khoá cố định: Request trước, Approval sau.** CR chỉ nói "thứ tự phải cố định". Các lệnh đổi loại, huỷ, về backlog khoá Request rồi huỷ Approval; nếu `Approve` khoá Approval rồi mới để `TransitionRequest` khoá Request thì hai đường đảo thứ tự và có thể khoá chết. Vì vậy `DecideApproval` đọc `request_id` (không khoá), `SELECT ... FOR UPDATE` Request, rồi `GetForUpdate` Approval và kiểm lại `request_id`. Đây là bổ sung so với CR.
4. `ApprovalAuthorizer`/`ApproverPolicy` ở đây là cài tạm (admin hoặc reporter) để SOL-010 thay.

## 2. Giải pháp

### A. Cây thư mục (mới, `backend-go/services/request-service/`)

```
internal/domain/approval.go, approval_subject.go
internal/usecase/
    approval_subject_handler.go     # port SubjectHandler + SubjectHandlerRegistry
    approver_policy_ports.go        # ApproverPolicy, ApprovalAuthorizer + cài tạm
    open_approval.go, decide_approval.go, cancel_approval.go,
    cancel_pending_approvals_for_request.go, list_approvals.go, list_pending_approvals_for_user.go
internal/adapter/postgres/approval_repository.go ; internal/adapter/mysql/approval_repository.go
internal/adapter/grpc/approval_server.go
migrations/{postgres,mysql}/NNNN_approvals.{up,down}.sql
proto/orca/request/v1/approval.proto
```

### B. Domain

```go
type ApprovalStatus string // pending|approved|rejected|cancelled|expired
type SubjectType string    // request_type|solution|findings|answer|plan|phase|task_list|pre_deploy

type Approval struct {
    ID, TenantID, RequestID string
    SubjectType SubjectType; SubjectID, Stage string
    Status ApprovalStatus
    RequestedBy string; DecidedBy *string; DecidedAt *time.Time
    Comment string; DueAt *time.Time; Version int64
    SubjectDigest string; SelfApprovalAllowed bool
    IdempotencyKey *string; RemindedAt *time.Time; CreatedAt, UpdatedAt time.Time
}
func (a *Approval) Approve(by, comment string, now time.Time) error  // pending -> approved
func (a *Approval) Reject(by, comment string, now time.Time) error   // comment bắt buộc, <=2000
func (a *Approval) Cancel(by, reason string, now time.Time) error
func (a *Approval) Expire(now time.Time) error
// Sai trạng thái: ErrApprovalNotPending. Bốn trạng thái cuối không có cạnh ra.
```
Domain chỉ stdlib. `SubjectType.Valid()` và bảng `AllSubjectTypes` dùng cho kiểm "mọi subject có handler".

### C. Migration `NNNN_approvals`

Cột theo CR-REQ-009 mục 2.3. Điểm khác dialect:

```sql
-- postgres
CREATE TABLE request.approvals ( id UUID PRIMARY KEY, tenant_id UUID NOT NULL,
  request_id UUID NOT NULL REFERENCES request.requests(id),
  subject_type TEXT NOT NULL CHECK (subject_type IN ('request_type','solution','findings','answer','plan','phase','task_list','pre_deploy')),
  subject_id TEXT NOT NULL, stage TEXT NOT NULL,
  status TEXT NOT NULL CHECK (status IN ('pending','approved','rejected','cancelled','expired')),
  requested_by TEXT NOT NULL, decided_by TEXT NULL, decided_at TIMESTAMPTZ NULL,
  comment TEXT NOT NULL DEFAULT '', due_at TIMESTAMPTZ NULL, version BIGINT NOT NULL DEFAULT 1,
  subject_digest TEXT NOT NULL, self_approval_allowed BOOLEAN NOT NULL DEFAULT TRUE,
  idempotency_key TEXT NULL, reminded_at TIMESTAMPTZ NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now());
CREATE UNIQUE INDEX approvals_one_pending ON request.approvals (tenant_id, subject_type, subject_id) WHERE status='pending';
CREATE UNIQUE INDEX approvals_idem ON request.approvals (tenant_id, idempotency_key) WHERE idempotency_key IS NOT NULL;
CREATE INDEX approvals_by_request ON request.approvals (tenant_id, request_id, created_at);
CREATE INDEX approvals_due ON request.approvals (tenant_id, status, due_at);
CREATE INDEX approvals_inbox ON request.approvals (tenant_id, status, created_at);
```
```sql
-- mysql 8.0.1+
pending_key VARCHAR(120) GENERATED ALWAYS AS (IF(status='pending', CONCAT(subject_type,':',subject_id), NULL)) STORED,
UNIQUE KEY approvals_one_pending (tenant_id, pending_key),
subject_digest CHAR(64) NOT NULL, self_approval_allowed TINYINT(1) NOT NULL DEFAULT 1,
UNIQUE KEY approvals_idem (tenant_id, idempotency_key),
created_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6), updated_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6)
```
Postgres thêm RLS `tenant_isolation` (như các bảng của CR-REQ-002); MySQL lọc `tenant_id` ở repository. `subject_id` 64 ký tự: kiểm UUID 36 ký tự vừa đủ, và `pending_key` 20+1+64 < 120.

### D. Repository

```go
type ApprovalRepository interface {
    Insert(ctx context.Context, tx Tx, a domain.Approval) error // vi phạm chỉ mục pending -> ErrPendingExists
    GetForUpdate(ctx context.Context, tx Tx, tenantID, id string) (domain.Approval, error)
    FindPendingBySubject(ctx context.Context, tx Tx, tenantID string, st domain.SubjectType, subjectID string) (*domain.Approval, error)
    UpdateDecision(ctx context.Context, tx Tx, a domain.Approval, expectedVersion int64) (bool, error) // so sánh-và-ghi
    UpdatePendingDigest(ctx context.Context, tx Tx, tenantID string, st domain.SubjectType, subjectID, digest string) (bool, error)
    CancelPendingForRequest(ctx context.Context, tx Tx, tenantID, requestID, why string, now time.Time) ([]domain.Approval, error)
    List(ctx context.Context, tenantID string, f ListFilter) ([]domain.Approval, string, error)
}
```
Nhận biết trùng khoá: Postgres mã lỗi `23505` và tên chỉ mục; MySQL lỗi `1062` kèm tên khoá. Hai dialect đều bọc thành `domain.ErrPendingExists`/`ErrIdempotencyConflict`. `CancelPendingForRequest` trên MySQL (không `RETURNING`) làm `SELECT` các id rồi `UPDATE`.

### E. Use case

- **`OpenApproval.Execute`**: `RequireTenantID`; `FlowFor(req.Type)` kiểm `subject_type` được phép (`REQUEST_APPROVAL_SUBJECT_TYPE_NOT_ALLOWED`); `registry.Get(subjectType).ValidateForRequest` lấy digest; `ApproverPolicy.Resolve` lấy `due_at`, `self_approval_allowed`; chèn; outbox `orca.request.approval.requested` `{approval_id, request_id, subject_type, subject_id, stage, requested_by, due_at}`. Có `pending` cho chủ thể: digest trùng thì trả dòng đó, khác thì `REQUEST_APPROVAL_PENDING_EXISTS`. Tất cả trong transaction của caller (nhận `Tx`) hoặc tự mở nếu gọi từ RPC.
- **`DecideApproval.Approve(ctx, in)`** một transaction: khoá Request rồi `GetForUpdate` Approval (mục 1.3); nếu `pending` và `due_at <= now_db` thì ghi `expired` và trả `REQUEST_APPROVAL_EXPIRED`; `authorizer.CanDecide`; `expected_digest == subject_digest` (`REQUEST_APPROVAL_DIGEST_MISMATCH`); Request đang đúng `stage` (`REQUEST_APPROVAL_STAGE_MISMATCH`); `a.Approve`; `UpdateDecision(expected_version)` (0 dòng thì đọc lại: đã quyết định hay xung đột phiên bản); `handler.OnApproved`; outbox `approval.decided{decision:"approved"}`. Handler lỗi thì rollback toàn bộ. Lặp cùng quyết định bởi cùng người trên Approval đã đóng: trả kết quả hiện có, không phát sự kiện.
- **`Reject`**: như trên, `comment` bắt buộc, `OnRejected`.
- **`Cancel`**: người yêu cầu hoặc admin; `OnClosedWithoutDecision(why)`; outbox `decision:"cancelled"`.
- **`CancelPendingForRequest`**: gọi trong transaction của lệnh khoá Request (đổi loại, huỷ, về backlog); gọi `OnClosedWithoutDecision` từng dòng; caller đã giữ khoá Request nên đúng thứ tự mục 1.3.
- **`ListPendingForUser`** (tạm): admin thấy mọi `pending`; người khác thấy `pending` của Request có `reporter_id` là mình, phân trang `(created_at, id)`.
- **Cài tạm**: `TemporaryApproverPolicy.Resolve` trả `due_at=nil`, `self_approval_allowed=true`; `TemporaryAuthorizer.CanDecide` cho `tenant.Role(ctx)=="admin"` hoặc `reporter_id==UserID`.

### F. `SubjectHandler` và đăng ký

Interface đúng như CR mục 2.4 (`ValidateForRequest`, `OnApproved`, `OnRejected`, `OnClosedWithoutDecision`). `SubjectHandlerRegistry.Register(st, h)` và `MustCoverAll()` kiểm mọi giá trị trong `AllSubjectTypes` có handler; `main.go` gọi `MustCoverAll()` sau khi lắp ráp và thoát lỗi nếu thiếu. Trong lúc các CR 005/012/013/014 chưa xong, composition root đăng ký `NoopSubjectHandler{Reason}` có tên rõ ràng và test chặn Noop trong cấu hình production (cờ `REQUEST_ALLOW_NOOP_APPROVAL_HANDLERS`, mặc định tắt). Chủ sở hữu handler: 005 `request_type`, 007 `solution`, 008 `findings`/`answer`, 012 `plan`/`task_list`/`phase`, 013 `phase`, 014 `pre_deploy`.

### G. Proto và gRPC

`approval.proto` đúng mục 2.6 của CR (service `ApprovalService`, 7 RPC, `DecideApprovalResponse.request_status`). `RequestApproval` qua RPC chỉ nhận `subject_type=pre_deploy` hoặc mở lại cổng đã đóng; còn lại `REQUEST_APPROVAL_SUBJECT_TYPE_NOT_ALLOWED`. Mọi RPC tự kiểm quyền (README mục 8 điểm 13: gateway không kiểm OPA trước định tuyến). Ánh xạ lỗi sang gRPC code qua `apperrors`.

### H. Lỗi và sự kiện

Mã lỗi: `REQUEST_APPROVAL_{NOT_FOUND, ALREADY_DECIDED, EXPIRED, VERSION_CONFLICT, DIGEST_MISMATCH, STAGE_MISMATCH, SUBJECT_NOT_FOUND, COMMENT_REQUIRED, PENDING_EXISTS, SUBJECT_TYPE_NOT_ALLOWED, FORBIDDEN}`. Sự kiện `orca.request.approval.requested` và `.decided` (`decision` thuộc `approved|rejected|cancelled|expired`); payload không có `comment`.

## 3. Quyết định thiết kế

| # | Quyết định | Lý do |
|---|---|---|
| 1 | Bảng riêng, không dùng `DecisionGate` | README D3/O4 |
| 2 | Handler chạy trong transaction quyết định | Không có trạng thái "đã duyệt nhưng Request chưa đi" |
| 3 | Hiệu ứng sang `task-service` chỉ qua outbox | Không gọi gRPC trong transaction |
| 4 | Một `pending` cho mỗi chủ thể bằng chỉ mục | Hai dialect cùng ngữ nghĩa |
| 5 | Thứ tự khoá Request trước, Approval sau trong mọi lệnh | Tránh khoá chết |
| 6 | `NoopSubjectHandler` có cờ cấm ở production | Cho phép làm theo thứ tự mà không bỏ lọt cổng |

## 4. Phụ thuộc và thứ tự

Cần CR-REQ-001, 002, 003 (`TransitionRequest`, `FlowFor`). Mở khoá SOL-010 và các CR 005, 007, 008, 012, 013, 014, 016, 022. Ghi tiến: các CR bổ sung có thể thêm `subject_type` mới (ví dụ cho Clarification hoặc TaskSpec); thêm giá trị vào CHECK là migration expand-only và thêm một handler; solution này không dự đoán.

## 5. Kiểm thử

- Unit domain: mọi cặp (trạng thái, hành động) kể cả bất hợp lệ; `SubjectType.Valid`.
- Unit usecase (fake repo + handler): mọi lỗi mục H, idempotent, rollback khi handler lỗi, hết hạn lười, `MustCoverAll` fail khi thiếu.
- Integration hai DB: hai `OpenApproval` đồng thời (một dòng), hai `Approve` đồng thời (một thắng), `FOR UPDATE` tuần tự hoá, transaction với handler thật và outbox, phân trang.
- Hợp đồng: `buf breaking`; golden payload `approval.requested`/`decided` (không có `comment`); test mọi `subject_type` có handler.
- Chưa kiểm chứng: `FOR UPDATE` dưới tải; outbox với hai replica (CR-REQ-001).

## 6. Rủi ro và điểm chưa kiểm chứng

- Handler do sáu CR cài; lệch hợp đồng là rủi ro lớn nhất, nên bộ test hợp đồng dùng chung (task 06).
- `plan`/`phase` có `subject_id` là Task của `task-service`; Task xoá thì Approval mồ côi, chỉ kiểm lúc mở.
- `tenant.Role(ctx)` chỉ `user|admin`.
- Khoá chết giữa `CancelPendingForRequest` và `Approve` nếu ai đó đảo thứ tự; cần test đua.

## 7. Câu hỏi mở

1. README 3.5 thiếu `subject_digest`, `self_approval_allowed`, `idempotency_key`, `reminded_at`, `created_at`, `updated_at`; `stage` chưa định nghĩa (CR chọn "trạng thái Request mà cổng chặn").
2. `ConfirmRequestType` có là vỏ gọi `Approve` không (CR-REQ-005 xác nhận)?
3. `decision` thay cho sự kiện hết hạn/huỷ riêng có được chấp nhận?
4. Nhận biết nguồn MCP (CR-REQ-016/017).

## 8. Tham chiếu

- `/opt/repos/orca/docs/crs/v6/approval/CR-REQ-009-generic-approval-domain-and-api.md`
- `/opt/repos/orca/backend-go/services/orchestration-service/internal/usecase/resolve_gate.go`
- `/opt/repos/orca/backend-go/services/mcp-service/internal/domain/approval.go`, `params_hash.go`, `internal/usecase/approvals.go`
- `/opt/repos/orca/backend-go/services/task-service/migrations/{postgres,mysql}/0012_task_sources.up.sql` (thủ thuật cột sinh)
- `/opt/repos/orca/backend-go/common/{outbox,tenant,apperrors,dbcapability}`
