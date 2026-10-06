# backend-go Solutions: Request Lifecycle (v6)

**CRs:** [docs/crs/v6/request-lifecycle](../../../../../../docs/crs/v6/request-lifecycle/README.md)
**Hợp đồng chung:** [docs/crs/v6/README.md](../../../../../../docs/crs/v6/README.md) (mục 8 thắng mục 3)
**TDD tham chiếu:** [`arch/03`](../../../../tdd/architecture/03-clean-architecture-guidelines.md), [`arch/05`](../../../../tdd/architecture/05-data-architecture.md), [`arch/08`](../../../../tdd/architecture/08-inter-service-communication.md), [`arch/09`](../../../../tdd/architecture/09-observability-reliability.md), [`services/task-service.md`](../../../../tdd/services/task-service.md), [`services/api-gateway.md`](../../../../tdd/services/api-gateway.md)

> 📋 Proposed. Chưa triển khai, chưa chạy test nào. Nền tảng: [request-service-foundation](../../request-service-foundation/solutions/README.md).

## Bảng CR, Solution, Task

| CR | Solution | Service / Area | Effort | Task |
|----|----------|----------------|--------|------|
| [CR-REQ-003](../../../../../../docs/crs/v6/request-lifecycle/CR-REQ-003-request-state-machine-and-flow-registry.md) | [BE-REQ-SOL-003](./BE-REQ-SOL-003-request-state-machine-and-flow-registry.md) | `request-service` (domain, `TransitionRequest`, `GetRequestFlow`) | Large | `TASK-REQ-003-01` đến `-06` |
| [CR-REQ-004](../../../../../../docs/crs/v6/request-lifecycle/CR-REQ-004-request-intake-from-sources.md) | [BE-REQ-SOL-004](./BE-REQ-SOL-004-request-intake-from-sources.md) | `request-service`, `proto`, `api-gateway` (route webhook) | Large | `TASK-REQ-004-01` đến `-08` |
| [CR-REQ-005](../../../../../../docs/crs/v6/request-lifecycle/CR-REQ-005-request-classification-and-type-change.md) | [BE-REQ-SOL-005](./BE-REQ-SOL-005-request-classification-and-type-change.md) | `request-service` (classifier, consumer, confirm, change type), `proto` | Large | `TASK-REQ-005-01` đến `-08` |
| [CR-REQ-006](../../../../../../docs/crs/v6/request-lifecycle/CR-REQ-006-return-to-backlog-reopen-cancel-child-requests.md) | [BE-REQ-SOL-006](./BE-REQ-SOL-006-return-to-backlog-reopen-cancel-child-requests.md) | `request-service` (return, reopen, cancel, spawn child), `proto` | Medium | `TASK-REQ-006-01` đến `-06` |

## Re-verify trước khi thiết kế (đối chiếu CR với mã thật, 2026-10-06)

| Khẳng định của CR | Kết quả khi đọc mã | Lệch? |
|---|---|---|
| Retry CAS 3 lần trong một `InTx` | `CreateRequest` (CR-REQ-004) gọi `TransitionRequest` trong giao dịch của nó; REPEATABLE READ (MySQL) giữ snapshot cũ | **Lệch** ⇒ retry chỉ ở lớp ngoài cùng (`TxRunner.InTransaction`), SOL-003 C1 |
| Test kiến trúc grep `SET status` | `Update` tổng quát ghi mọi cột | **Lệch** ⇒ test `go/parser` (SOL-003 C2) |
| Consumer "dedup bằng `processed_events`" | Đánh dấu trước làm mất sự kiện khi AI lỗi, sau làm AI chạy hai lần | **Lệch** ⇒ `MarkProcessed` cùng giao dịch ghi kết quả (SOL-005 C3) |
| `ResolveProvider` rồi `Relay` theo `connectionID` | `connectionID=projectID` luôn trượt (BUG-025 trong `task-service`); phải `RelayByDevServer` | **Lệch** ⇒ `AIConnectionResolver` (SOL-005 C1) |
| 5 lần AI đếm bằng lịch sử | Thất bại không ghi lịch sử nên không bị đếm | **Lệch** ⇒ cột `classification_attempts` (SOL-005 C2, cần xác nhận) |
| Webhook xác thực bí mật `(tenant, source_name)` | Route không mang tenant, ngoài nhóm JWT | **Lệch** ⇒ header `X-Orca-Tenant-Id` (SOL-004 C1) |
| `now()` cho `created_at` outbox | Hai sự kiện cùng giao dịch cùng `created_at`, relay sắp sai thứ tự | **Lệch** ⇒ cột `seq` ở `0001` (SOL-001 D2) |
| Migration `0004_request_return_history` | `0003` (source_hints) và `0004` (classification_attempts) đứng trước | **Lệch** ⇒ `0005`; mọi task tự đọc thư mục để chốt số |
| CHECK `(status='request_backlog') = (returned_category IS NOT NULL)` | `TransitionRequest` chưa có `category`, dòng sẵn có vi phạm | **Lệch** ⇒ backfill `other`, `TransitionInput.Category` (SOL-006 C2, C3) |

## Thứ tự thực thi và phụ thuộc

```
BE-REQ-SOL-002 (foundation: schema, repository)
   └─▶ SOL-003 (registry, bảng chuyển, TransitionRequest, GetRequestFlow)
          └─▶ SOL-004 (CreateRequest, nguồn, idempotency, webhook)   [migration 0003]
                 └─▶ SOL-005 (phân loại AI, Confirm, ChangeType)      [migration 0004]
                        └─▶ SOL-006 (return, reopen, cancel, child)   [migration 0005]
```

Số migration kỳ vọng: `0001` init, `0002` request_core (foundation), `0003`, `0004`, `0005` ở đây. Số thật do `ls` quyết định khi viết, vì CR-REQ-007, 009, 013 cũng thêm bảng.

## Quyết định chung của feature

| # | Quyết định | Lý do |
|---|-----------|-------|
| L1 | Chỉ `TransitionRequest` ghi `status`, `returned_from_stage`, `return_reason`, `returned_category`; test `go/parser` bảo vệ | Một nơi giữ bất biến |
| L2 | Registry luồng là mã Go tĩnh trong `internal/domain` | Đổi luồng đi cùng release và test |
| L3 | Mọi lệnh ghi một Request là một `InTx`: kiểm tra, ghi, `version+1`, ghi `outbox_events` | At-least-once an toàn, không sự kiện ma |
| L4 | `InTx` tham gia giao dịch lồng; retry CAS chỉ ở lớp ngoài cùng | Use case gọi use case trong một commit |
| L5 | Giao lặp là bình thường: lệnh đã áp dụng trả thành công | Callback và consumer outbox có thể lặp |
| L6 | Phân loại bắt buộc người xác nhận, kể cả `question`, `task` size S | D5, nghiên cứu mục 7 điểm 3 |
| L7 | Gọi AI luôn ngoài giao dịch; kết quả và `processed_events` ghi trong một giao dịch | Không giữ khoá lâu, không mất, không trùng |
| L8 | Cổng tích hợp mềm (`ApprovalRecorder`, `ApprovalCanceller`, `ExecutionGuard`) có bản no-op tới CR-REQ-009, 011 | Không phụ thuộc cứng |
| L9 | Mọi RPC tự kiểm tenant và user; quyền theo project do gateway; mô hình quyền Request chờ CR-REQ-010 | README v6 mục 8 điểm 13 |
| L10 | Sự kiện `orca.request.request.<event>`; không có `reopened`, `cancelled` riêng | README v6 mục 8 điểm 4 |

## Điều còn mở (tổng hợp)

- **Cổng `phase`/`pre_deploy`** (SOL-003 Q1): chưa xác nhận.
- **Cột `classification_attempts`** (SOL-005 Q1): lệch có chủ ý so với CR.
- **Kiểm HMAC webhook ở gateway hay trong service** (SOL-004 Q2).
- **Khoá idempotency không có `project_id`** (SOL-002 Q2).
- **Tham chiếu tiến:** CR-REQ-028 (`awaiting_information`, clarification) sẽ mở rộng phân loại, tiếp nhận và trạng thái Request; các solution ở đây không phụ thuộc vào nó.
