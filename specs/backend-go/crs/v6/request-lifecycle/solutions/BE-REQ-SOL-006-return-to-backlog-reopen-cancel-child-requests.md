# BE-REQ-SOL-006: Trả về Request backlog, mở lại, hủy, Request con

> **📋 Proposed.** Chưa triển khai, chưa chạy test nào. Phụ thuộc [BE-REQ-SOL-005](./BE-REQ-SOL-005-request-classification-and-type-change.md), [BE-REQ-SOL-004](./BE-REQ-SOL-004-request-intake-from-sources.md), [BE-REQ-SOL-003](./BE-REQ-SOL-003-request-state-machine-and-flow-registry.md).

**CR:** [CR-REQ-006](../../../../../../docs/crs/v6/request-lifecycle/CR-REQ-006-return-to-backlog-reopen-cancel-child-requests.md)
**Service:** `request-service` (use case, migration, adapter gRPC) · `proto`
**TDD tham chiếu:** [`arch/03`](../../../../tdd/architecture/03-clean-architecture-guidelines.md), [`arch/05`](../../../../tdd/architecture/05-data-architecture.md), [`arch/08`](../../../../tdd/architecture/08-inter-service-communication.md), [`services/notification-service.md`](../../../../tdd/services/notification-service.md) (người dùng sự kiện `returned`)

---

## 1. Trạng thái hiện tại (re-verify)

Đã đọc: `task-service/internal/usecase/report_execution_result.go` (mẫu thao tác idempotent), `task-service/migrations/postgres/0012_task_sources.up.sql` (RLS, khoá duy nhất), `common/outbox/outbox.go`, `common/apperrors/apperrors.go`; các solution SOL-003 đến 005 đã định nghĩa `TransitionRequest`, `CreateWithinTx`, cổng `ApprovalCanceller`, `ExecutionGuard`. `request-service` chưa có mã (đang là thiết kế). Cột `requests.returned_from_stage` và `return_reason` đã có từ SOL-002; `returned_category` và bảng `request_return_history` chưa.

### Correction relative to CR-REQ-006

| # | CR nói | Hệ quả | Xử lý |
|---|--------|--------|-------|
| C1 | Migration `0004_request_return_history` | SOL-005 đã dùng `0004` cho `classification_attempts`; `0003` là `source_hints` | Migration của solution này là `0005_request_return_history` nếu các migration trước đã có; **đọc thư mục để chốt số** |
| C2 | CHECK `(status='request_backlog') = (returned_category IS NOT NULL)` thêm sau | `TransitionRequest` (SOL-003) đã chuyển được sang `request_backlog` mà không có `category`: mọi dòng backlog sẵn có và mọi chuyển giữa hai task sẽ vi phạm CHECK | Migration backfill `returned_category='other'` cho dòng `request_backlog` có sẵn **trước** khi thêm CHECK; `TransitionInput.Category` bắt buộc ngay từ task 02, hai task merge cùng đợt (cờ tính năng tắt nên không có dữ liệu thật) |
| C3 | `TransitionRequest(return_to_backlog)` chỉ cần `stage`, `reason` | Cần ghi `returned_category` trong cùng `Update` | Mở rộng `TransitionInput.Category` và cập nhật test của SOL-003; test kiến trúc (`status_write_guard_test`) thêm `.ReturnedCategory` |
| C4 | `ReopenRequest` về `classifying` kích hoạt AI | Consumer của SOL-005 lọc `to=classifying`; `classification_attempts` cộng dồn nên Request mở lại sau 5 lần AI sẽ không còn đề xuất AI | Mở lại đặt `classification_attempts = 0` (quyết định của người dùng mở lại, ghi `reopened` vào lịch sử; xem Q4) |
| C5 | Khoá idempotency của con `(tenant, provider, 'user:<actor>', client_request_id)` | Cùng người dùng dùng cùng `client_request_id` ở hai cha khác nhau sẽ trả nhầm con của cha kia | Khoá dùng `client_request_id` có tiền tố cha: `<parent_id>:<client_request_id>` |
| C6 | Hủy "từ mọi trạng thái trừ `completed`, `cancelled`", gọi lặp thành công | `TransitionRequest` không có trigger từ trạng thái cuối, nên gọi lặp sẽ lỗi `REQUEST_TRANSITION_NOT_ALLOWED` | Use case kiểm `status == cancelled` trước và trả thành công `Applied=false` |

## 2. Giải pháp

### A. Migration `0005_request_return_history` (hai dialect; số do `ls` quyết định)

```sql
-- Postgres
ALTER TABLE request.requests ADD COLUMN returned_category TEXT
  CHECK (returned_category IN ('missing_info','infeasible','blocked_dependency','rejected','other'));
UPDATE request.requests SET returned_category = 'other' WHERE status = 'request_backlog';
ALTER TABLE request.requests ADD CONSTRAINT requests_backlog_category
  CHECK ((status = 'request_backlog') = (returned_category IS NOT NULL));
CREATE TABLE request.request_return_history (
  id UUID PRIMARY KEY, tenant_id UUID NOT NULL, request_id UUID NOT NULL,
  action TEXT NOT NULL CHECK (action IN ('returned','reopened','cancelled')),
  stage TEXT CHECK (stage IN ('classification','analysis','plan','phase','task')),
  category TEXT CHECK (category IN ('missing_info','infeasible','blocked_dependency','rejected','other')),
  reason TEXT NOT NULL DEFAULT '', actor_id UUID,
  actor_kind TEXT NOT NULL CHECK (actor_kind IN ('ai','user','system')),
  at TIMESTAMPTZ NOT NULL DEFAULT now());
CREATE INDEX idx_request_return_history ON request.request_return_history (tenant_id, request_id, at);
-- RLS ENABLE + FORCE + tenant_isolation (NULLIF), như 0001
```
MySQL: cột `returned_category VARCHAR(30) NULL`, cùng `UPDATE`, `ADD CONSTRAINT ... CHECK` (8.0.16+), bảng `CHAR(36)`, `TIMESTAMP(6)`. Bảng chỉ thêm. Down: bỏ ràng buộc, bảng, cột. Proto: `Request.returned_category = 25`.

### B. Domain

`ReturnCategory` (5 giá trị, `ParseReturnCategory`), `ReturnAction`, `ReturnHistoryEntry`; `ActorKind` thêm `system`; `Request.ReturnedCategory`. Lỗi: `REQUEST_RETURN_STAGE_INVALID`, `REQUEST_RETURN_CATEGORY_INVALID`, `REQUEST_RETURN_BLOCKED_ACTIVE_EXECUTION`, `REQUEST_CANCEL_BLOCKED_ACTIVE_EXECUTION`, `REQUEST_REOPEN_NOT_ALLOWED`, `REQUEST_CANCEL_NOT_ALLOWED`, `REQUEST_PARENT_NOT_FOUND`, `REQUEST_CHILD_NOT_ALLOWED`, `REQUEST_CHILD_LIMIT`, `REQUEST_CHILD_DEPTH_EXCEEDED`, `REQUEST_CLIENT_REQUEST_ID_REQUIRED`. Bảng stage hợp lệ (`domain/request_return_stage.go`): `classifying`, `awaiting_type_confirmation` → `classification`; `analyzing`, `awaiting_analysis_approval` → `analysis`; `planning`, `awaiting_plan_approval` → `plan`; `executing` → `phase` (chỉ khi `FlowFor(type).PhasesFor(size)`) hoặc `task`.

### C. Mở rộng `TransitionRequest`

`TransitionInput` thêm `Category domain.ReturnCategory`. Khi đích là `request_backlog`: `Category` bắt buộc (`REQUEST_RETURN_CATEGORY_INVALID`), ghi `ReturnedCategory`; rời `request_backlog` xoá. Payload `status_changed` thêm `category`.

### D. `ReturnRequestToBacklog` (`return_request_to_backlog.go`)

```go
type ReturnInput struct { RequestID string; Stage domain.ReturnStage; Category domain.ReturnCategory
    Reason string; ExpectedVersion int64; ActorID string; ActorKind domain.ActorKind }
func (uc *ReturnRequestToBacklog) Execute(ctx context.Context, in ReturnInput) (domain.Request, error)
```
`InTx`: đọc; đã `request_backlog` cùng `returned_from_stage` thì thành công không ghi gì (giao lặp), khác stage thì `REQUEST_TRANSITION_NOT_ALLOWED`; kiểm `Stage` khớp `status` (`REQUEST_RETURN_STAGE_INVALID`); `Reason` (`REQUEST_REASON_REQUIRED`); `Category`; từ `executing` mà `ExecutionGuard` báo còn Task chạy: `REQUEST_RETURN_BLOCKED_ACTIVE_EXECUTION`. Chọn trigger: `Category=rejected` và `status=awaiting_analysis_approval` thì `analysis_rejected`; `awaiting_plan_approval` thì `plan_rejected`; còn lại `return_to_backlog`. Rồi `TransitionRequest.Execute` (lồng), `ReturnHistory.Append(returned)`, `ApprovalCanceller.CancelPending(requestID, "returned")`, outbox `orca.request.request.returned` `{request_id, stage, category, reason, actor_id}`. CR-REQ-007, 009 (từ chối Solution/Plan) và CR-REQ-013 (lỗi thực thi, `actor_kind=system`) gọi use case này.

### E. `ReopenRequest` và `CancelRequest`

`ReopenRequest`: chỉ từ `request_backlog` (`REQUEST_REOPEN_NOT_ALLOWED`); `InTx`: CAS, `ClassificationAttempts=0`, `TransitionRequest(reopen)` về `classifying` (xoá `returned_*`), `ReturnHistory.Append(reopened, reason=note)`; chỉ phát `status_changed` (không có `request.reopened`, README mục 8 điểm 4); `type` cũ giữ làm gợi ý; Solution, Plan, Task giữ nguyên. `completed`, `cancelled` không mở lại (Q3).

`CancelRequest`: từ mọi trạng thái trừ `completed` (`REQUEST_CANCEL_NOT_ALLOWED`), `cancelled` thì thành công `Applied=false` (C6); `Reason` bắt buộc; từ `executing` còn Task chạy: `REQUEST_CANCEL_BLOCKED_ACTIVE_EXECUTION`; `InTx`: `TransitionRequest(cancel)`, `ReturnHistory.Append(cancelled)`, `ApprovalCanceller.CancelPending(requestID, "cancelled")`. Không lan sang Request con. Việc huỷ cây Plan trong `task-service` chưa thuộc solution này (Q5).

### F. `SpawnChildRequest` (`spawn_child_request.go`)

Bảng cha, con (CR mục 2.6): `spawned_by_spike` (cha `spike`, status `awaiting_analysis_approval` hoặc `completed`, `type_hint` `change_request|task`); `spawned_by_question` (tương tự); `followup_hotfix` (cha `hotfix`, `executing` hoặc `completed`, `bug|task`); `escalation` (bất kỳ loại, mọi status trừ `cancelled`, không ràng buộc `type_hint`). Sai tổ hợp: `REQUEST_CHILD_NOT_ALLOWED`.

Bước: `client_request_id` bắt buộc; đọc cha (`REQUEST_PARENT_NOT_FOUND`); kiểm bảng; kiểm số con (`ListChildren` ≥ 50: `REQUEST_CHILD_LIMIT`; giới hạn mềm vì đồng thời có thể vượt vài đơn vị) và độ sâu (đi ngược `ListParents` tối đa 5 bước: con sinh ra ở cấp 6 trả `REQUEST_CHILD_DEPTH_EXCEEDED`); rồi một `InTx`: `CreateRequest.CreateWithinTx` với `ProjectID` của cha, `Source{provider: manual hoặc mcp}`, `ClientRequestID = <parent_id>:<client_request_id>`, `Hints.TypeHint=type_hint`, `AllowTypeHint=true`, `Parent{ParentRequestID, LinkReason}` để outbox `created` mang `parent_request_id`, `link_reason` (cộng thêm); `RequestLinks.Insert(parent, child, reason, created_by)`. Gọi lặp cùng khoá: trả con cũ, `Created=false` (không chèn link lần hai). `hotfix` kết thúc thì Request theo dõi do CR-REQ-014 gọi use case này.

### G. Proto và RPC

Theo CR mục 2.2: `ReturnToBacklog`, `ReopenRequest`, `CancelRequest`, `SpawnChildRequest`, thêm `ListRequestLinks` (README mục 8 điểm 12) với `RequestLink{parent_request_id, child_request_id, reason, created_at}`. Ba lệnh đầu trả `Request` đã cập nhật (`ReturnToBacklogResponse{request}`, `ReopenRequestResponse{request}`, `CancelRequestResponse{request}`). `ListBacklog` thuộc CR-REQ-015.

## 3. Quyết định thiết kế

| Quyết định | Lý do |
|------------|-------|
| Bảng `request_return_history` | Hai cột trên `requests` chỉ giữ lần cuối; cần người thực hiện và nhiều lần |
| `returned_category` tách khỏi `return_reason` | Nhóm backlog theo lý do cần giá trị có kiểu |
| Backfill trước CHECK | Tránh migration vỡ trên dòng sẵn có |
| Mở lại đặt `attempts=0` | Người dùng chủ động xin phân loại lại; không để Request mở lại kẹt không có đề xuất |
| Con tạo bằng `CreateWithinTx` | Một đường vào, một bộ kiểm tra |
| Khoá con có tiền tố cha | Tránh trả nhầm con giữa các cha |
| Hủy không lan sang con | Con có vòng đời độc lập |

## 4. Phụ thuộc và thứ tự

Cần SOL-003 (`TransitionRequest`), SOL-004 (`CreateWithinTx`), SOL-005 (cổng `ApprovalCanceller`, `ExecutionGuard`, consumer phân loại). Mở khoá CR-REQ-013, 014, 015, 023. Thứ tự task: 01 migration và repository, 02 mở rộng `TransitionRequest` và `ReturnRequestToBacklog` (merge cùng 01), 03 mở lại và huỷ, 04 Request con, 05 proto và gRPC, 06 test tích hợp.

## 5. Kiểm thử

- **Unit:** bảng `stage` theo trạng thái; bảng cha, con, `type_hint`; giới hạn độ sâu và số con (repo giả); chọn trigger theo `Category`; huỷ lặp.
- **Integration, Postgres và MySQL:** migration up/down (có backfill); mọi trạng thái nguồn trả về backlog; CHECK `(status='request_backlog') = (returned_category IS NOT NULL)` giữ ở mọi chuyển; mở lại xoá ba cột và `attempts=0`; 12 `SpawnChildRequest` đồng thời cùng khoá ra một con và một `request_links`; cha có 50 con rồi con thứ 51 bị chặn; chuỗi 6 cấp bị chặn; rollback giữa chừng không để `request_links` mồ côi; cách ly tenant.
- **Hợp đồng:** `buf breaking`; test sự kiện `returned` có đủ trường cho `notification-service` (CR-REQ-010) và `issue-status-sync` (CR-REQ-024).
- **Chưa chạy bất kỳ test nào.**

## 6. Rủi ro và điểm chưa kiểm chứng

- `ExecutionGuard` giả tới CR-REQ-011: chặn trả về, huỷ khi có Task chạy chưa kiểm chứng trên hệ thống thật.
- Người trả về và consumer (từ chối Approval) cạnh tranh: CAS giải quyết, bên thua nhận `REQUEST_VERSION_CONFLICT`; UI phải tải lại.
- Giới hạn 50 con và 5 cấp là số ước lượng; giới hạn con là mềm khi đồng thời.
- Mở lại nhiều lần có thể lặp chi phí AI; chưa đặt giới hạn số lần mở lại (Q4).
- Số migration đụng PR khác (CR-REQ-007, 009 cũng thêm bảng).

## 7. Câu hỏi mở

- **Q1.** `escalation` chưa được README định nghĩa; solution hiểu là "con tạo tay từ Request bất kỳ".
- **Q2.** `ListRequestLinks` đã thêm theo README mục 8 điểm 12; cần kênh WS ở CR-REQ-016.
- **Q3.** Mở lại `completed`, `cancelled`: mặc định không.
- **Q4.** Giới hạn số lần mở lại, hoặc không reset `attempts` khi mở lại.
- **Q5.** Huỷ Request có huỷ cây Plan, Phase, Task trong `task-service` không (cần RPC ở CR-REQ-011 hoặc 013).
- Sẽ được CR-REQ-028 mở rộng: `awaiting_information` có thể thêm một giá trị `category` hoặc trạng thái nguồn trả về; không phụ thuộc ở đây.

## 8. Tham chiếu

- `/opt/repos/orca/docs/crs/v6/README.md` mục 3.3, 3.4, 3.5, 3.7, 8
- `/opt/repos/orca/docs/research/receive-request/request-classification-and-flows.md` mục 4, 5
- `/opt/repos/orca/backend-go/services/task-service/internal/usecase/report_execution_result.go`
- `/opt/repos/orca/backend-go/services/task-service/migrations/postgres/0012_task_sources.up.sql`
- `/opt/repos/orca/backend-go/common/outbox/outbox.go`, `common/apperrors/apperrors.go`
- `/opt/repos/orca/backend-go/services/request-service/internal/usecase/{return_request_to_backlog,reopen_request,cancel_request,spawn_child_request}.go`, `migrations/{postgres,mysql}/0005_request_return_history.*.sql` (mới)
