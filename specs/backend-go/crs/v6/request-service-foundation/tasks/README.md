# backend-go Tasks: Request Service Foundation (v6)

Task thực thi của hai solution trong [`../solutions/`](../solutions/README.md). Trạng thái cập nhật 2026-10-07 sau khi triển khai đợt R1a (xem [IMPLEMENTATION-NOTES](../IMPLEMENTATION-NOTES.md)): 11/13 task DONE, 2 task còn TODO (001-02, 001-06). Mỗi task nêu file và lệnh test; số dòng trích dẫn từ lần đọc code 2026-10-06.

## Bảng Solution, Task

| Solution | Task | Nội dung | Priority | Trạng thái |
|----------|------|----------|----------|------------|
| BE-REQ-SOL-001 | [TASK-REQ-001-01](./TASK-REQ-001-01-module-config-workspace-registration.md) | `go.mod`, `config`, `go.work`, `Makefile`, DB init | P0 | DONE |
| | [TASK-REQ-001-02](./TASK-REQ-001-02-proto-request-and-approval.md) | Proto `orca.request.v1`, sinh stub | P0 | TODO (nợ buf lint approval.proto) |
| | [TASK-REQ-001-03](./TASK-REQ-001-03-migration-0001-init-two-dialects.md) | Migration `0001_init` hai dialect, RLS thật | P0 | DONE |
| | [TASK-REQ-001-04](./TASK-REQ-001-04-tx-runner-and-outbox-adapters.md) | `TxRunner` (lồng được), `OutboxWriter`, `outbox.Store` hai dialect | P0 | DONE |
| | [TASK-REQ-001-05](./TASK-REQ-001-05-main-grpc-server-health.md) | `main.go`, gRPC khung, health, relay, `EnsureStream` | P0 | DONE |
| | [TASK-REQ-001-06](./TASK-REQ-001-06-deploy-dev-and-ci-workflow.md) | Dockerfile, compose, `migrate.sh`, `build-local.sh`, CI | P1 | TODO (chưa chạy compose, migrate.sh, CI) |
| BE-REQ-SOL-002 | [TASK-REQ-002-01](./TASK-REQ-002-01-migration-0002-request-core.md) | Migration `0002_request_core` hai dialect | P0 | DONE |
| | [TASK-REQ-002-02](./TASK-REQ-002-02-domain-entities-and-errors.md) | Domain, hằng, mã lỗi | P0 | DONE |
| | [TASK-REQ-002-03](./TASK-REQ-002-03-repository-ports-and-list-filter.md) | Cổng repository, `ListFilter`, token keyset | P0 | DONE |
| | [TASK-REQ-002-04](./TASK-REQ-002-04-postgres-repositories.md) | Repository Postgres | P0 | DONE |
| | [TASK-REQ-002-05](./TASK-REQ-002-05-mysql-repositories.md) | Repository MySQL | P0 | DONE |
| | [TASK-REQ-002-06](./TASK-REQ-002-06-repository-contract-suite.md) | Bộ test hợp đồng chung, test schema | P0 | DONE |
| | [TASK-REQ-002-07](./TASK-REQ-002-07-wire-get-and-list-requests-rpc.md) | `GetRequest`, `ListRequests` thật | P1 | DONE |

## Thứ tự phụ thuộc

```
TASK-REQ-001-01 (module, config, workspace)
   ├──▶ TASK-REQ-001-02 (proto) ───────────────┐
   ├──▶ TASK-REQ-001-03 (migration 0001) ──┐   │
   │                                       ▼   │
   │                        TASK-REQ-001-04 (tx, outbox) ──┐
   │                                                       ▼
   └───────────────────────────────▶ TASK-REQ-001-05 (main, gRPC, health) ──▶ TASK-REQ-001-06 (deploy, CI)

TASK-REQ-001-03 ─▶ TASK-REQ-002-01 (migration 0002)
TASK-REQ-001-01 ─▶ TASK-REQ-002-02 (domain) ─▶ TASK-REQ-002-03 (ports)   (ports cần cả 001-04)
TASK-REQ-002-01 + 002-02 + 002-03 ─┬─▶ TASK-REQ-002-04 (Postgres) ─┐
                                   └─▶ TASK-REQ-002-05 (MySQL)  ───┴─▶ TASK-REQ-002-06 (contract suite)
TASK-REQ-001-05 + 002-04/05 ─▶ TASK-REQ-002-07 (GetRequest, ListRequests)
```

Có thể làm song song: 001-02 với 001-03; 002-04 với 002-05; 002-02 với 002-01.

## Ghi chú

- **Số migration:** `0001` và `0002` là số kế tiếp thật vì `request-service` chưa có thư mục migrations. Nếu PR của CR khác đã thêm file vào thư mục đó, đọc `ls migrations/postgres` trước và đổi số.
- **RLS:** task 001-03 và 001-04 theo mẫu `mcp-service`, không theo `task-service` (RLS ở đó không chạy). Test cách ly tenant phải dùng role `NOSUPERUSER NOBYPASSRLS`.
- **Tx lồng:** `InTx` tham gia giao dịch của ctx. Retry CAS của `TransitionRequest` (CR-REQ-003) chỉ hợp lệ khi use case là lớp ngoài cùng.
- **MySQL:** không có `RETURNING` và RLS; mọi truy vấn có `tenant_id`; `CHECK` cần 8.0.16.
- **Sau feature này:** [request-lifecycle](../../request-lifecycle/tasks/README.md) bắt đầu bằng TASK-REQ-003-01.
