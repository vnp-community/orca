# backend-go Solutions: Request Service Foundation (v6)

**CRs:** [docs/crs/v6/request-service-foundation](../../../../../../docs/crs/v6/request-service-foundation/README.md)
**Hợp đồng chung:** [docs/crs/v6/README.md](../../../../../../docs/crs/v6/README.md) (mục 8 thắng mục 3)
**TDD tham chiếu:** [`arch/02`](../../../../tdd/architecture/02-microservices-decomposition.md), [`arch/03`](../../../../tdd/architecture/03-clean-architecture-guidelines.md), [`arch/05`](../../../../tdd/architecture/05-data-architecture.md), [`arch/08`](../../../../tdd/architecture/08-inter-service-communication.md), [`arch/09`](../../../../tdd/architecture/09-observability-reliability.md)

> 📋 Proposed. Chưa triển khai, chưa chạy test nào. Đọc lại code liên quan trước khi sửa.

## Bảng CR, Solution, Task

| CR | Solution | Service / Area | Effort | Task |
|----|----------|----------------|--------|------|
| [CR-REQ-001](../../../../../../docs/crs/v6/request-service-foundation/CR-REQ-001-scaffold-request-service.md) | [BE-REQ-SOL-001](./BE-REQ-SOL-001-scaffold-request-service.md) | `request-service` (mới), `proto`, `go.work`, `deploy/*`, CI | Medium | `TASK-REQ-001-01` đến `-06` |
| [CR-REQ-002](../../../../../../docs/crs/v6/request-service-foundation/CR-REQ-002-request-data-model-and-repositories.md) | [BE-REQ-SOL-002](./BE-REQ-SOL-002-request-data-model-and-repositories.md) | `request-service` (domain, ports, adapter hai dialect, migration `0002`) | Medium | `TASK-REQ-002-01` đến `-07` |

## Re-verify trước khi thiết kế (đối chiếu CR với mã thật, 2026-10-06)

| Khẳng định của CR | Kết quả khi đọc mã | Lệch? |
|---|---|---|
| `task-service` và `notification-service` có `adapter/postgres` và `adapter/mysql` | Đúng (`task-service/internal/adapter/mysql/*`, `notification-service/internal/adapter/mysql/*`, thư mục `migrations/mysql`) | Không |
| `outbox_events` có RLS như `task.task_sources` | RLS của `task-service` không chạy: không có `set_config('app.tenant_id')` nào (`postgres/share_link.go:17`), `FORCE` không bật. `mcp-service` mới làm RLS thật (`tenant_tx.go`, `0001_init.up.sql`) | **Lệch** ⇒ SOL-001 mục 2.D, SOL-002 mục A theo mẫu `mcp-service` |
| Repo lấy executor từ context "mẫu task-service" | `task-service` dùng `RunInTx(fn(ctx, tasks, edges))` với Repository có phạm vi giao dịch và tự ghi nhận không lồng được | **Lệch** ⇒ chọn executor trong ctx, `InTx` tham gia giao dịch khi lồng (cần cho `CreateRequest` gọi `TransitionRequest` cùng commit) |
| `processed_events` khoá theo `event_id` | `notification-service/0002_processed_events` đúng vậy, nhưng README v6 mục 8 buộc mọi bảng có `tenant_id`; `mcp-service` dùng PK `(tenant_id, event_id)` | **Lệch** ⇒ PK kép |
| Chỉ cần tạo DB, compose, `migrate.sh` | Stack dev thật mount binary `./bin/<svc>/orca`; thêm `build-local.sh` dòng 43 | Bổ sung |
| Không nhắc stream JetStream | Cần `pub.EnsureStream(ctx, "REQUEST", []string{"orca.request.>"})` cho consumer CR-REQ-005 | Bổ sung |
| Số migration | `request-service` chưa có thư mục migrations: `0001` (CR-REQ-001), `0002` (CR-REQ-002); các CR sau đọc lại thư mục trước khi đặt số | Không |
| `ListRequestsRequest.status`, `type` | CR ghi số ít; `ListFilter` dùng mảng | **Lệch** ⇒ `repeated string` ngay từ đầu |

## Thứ tự thực thi và phụ thuộc

```
SOL-001 (service khung, proto, 0001, tx/outbox, main, deploy)
   └─▶ SOL-002 (0002_request_core, domain, repository hai dialect, GetRequest/ListRequests thật)
          └─▶ request-lifecycle: BE-REQ-SOL-003 (state machine) ─▶ 004 ─▶ 005 ─▶ 006
```

SOL-002 cần module, `TxRunner`, `OutboxWriter`, `0001` của SOL-001. Mọi solution v6 còn lại cần SOL-002.

## Quyết định chung của feature

| # | Quyết định | Lý do |
|---|-----------|-------|
| F1 | Hai dialect từ đầu, chọn bằng `dbcapability.DetectDialectFromDSN` | README v6 mục 6; `mcp-service` Postgres-only là ngoại lệ, không làm mẫu |
| F2 | Layout theo `notification-service`/`task-service`, không theo `mcp-service`, trừ phần RLS | Hai dialect |
| F3 | Tx: executor trong ctx, `InTx` tham gia khi lồng | Một use case gọi use case khác trong cùng commit |
| F4 | RLS Postgres thật (`FORCE`, `NULLIF`, `set_config` mỗi giao dịch, policy relay); MySQL lọc `tenant_id` ở mọi `WHERE` | Tiêu chí cách ly tenant chỉ đạt khi RLS chạy |
| F5 | Bảng hạ tầng `outbox_events`, `processed_events(tenant_id, event_id)` | README v6 mục 8 điểm 3 |
| F6 | Subject `orca.request.<entity>.<event>`, stream `REQUEST` | README v6 mục 8 điểm 4 |
| F7 | Không dial service khác ở feature này | Test được mà không cần `task-service` |
| F8 | Proto chỉ khai báo RPC có message thật | `buf breaking` không khoá message nửa vời |

## Điều còn mở

- **Khoá idempotency không có `project_id`:** chốt trước khi `0002` merge (SOL-002 Q2).
- **Phiên bản MySQL tối thiểu:** `CHECK` cần 8.0.16 trở lên; TiDB chưa kiểm.
- **`approval.proto` rỗng:** giữ hay dời sang CR-REQ-009 (SOL-001 Q3).
