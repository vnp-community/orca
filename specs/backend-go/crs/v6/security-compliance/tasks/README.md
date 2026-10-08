# security-compliance: tasks (backend-go, v6)

> 🚧 7/9 task DONE (035-01, 03 đến 08, kiểm chứng 2026-10-08); 035-02 một phần; 035-09 một phần (xem file task). Phần `request-service` đã đối chiếu với code thật.

## Solution → Task

| Solution | Task | Tên | Priority | Phụ thuộc | Trạng thái |
|---|---|---|---|---|---|
| [BE-REQ-SOL-035](../solutions/BE-REQ-SOL-035-security-and-compliance-baseline.md) | [TASK-REQ-035-01](./TASK-REQ-035-01-secretscan-package.md) | Gói `common/secretscan` và vector chung | P0 | không | ✅ DONE 2026-10-07 |
| | [TASK-REQ-035-02](./TASK-REQ-035-02-actor-type-metadata-and-gateway.md) | `x-orca-actor-type`, token nội bộ gateway | P0 | TASK-REQ-016-01 | 🚧 Một phần |
| | [TASK-REQ-035-03](./TASK-REQ-035-03-request-rego-policy.md) | `request.rego`, ảnh container | P0 | SOL-001 | ✅ DONE 2026-10-08 |
| | [TASK-REQ-035-04](./TASK-REQ-035-04-interceptor-chain-and-rpc-catalog.md) | Chuỗi interceptor, danh mục RPC | P0 | 035-02, 035-03, SOL-002, TASK-REQ-025-02 | ✅ DONE 2026-10-08 |
| | [TASK-REQ-035-05](./TASK-REQ-035-05-rls-tenant-isolation-and-sql-guard.md) | RLS thật, test quét SQL MySQL | P0 | SOL-001, 002 | ✅ DONE 2026-10-08 |
| | [TASK-REQ-035-06](./TASK-REQ-035-06-security-migration-and-audit-outbox.md) | Migration `security_compliance`, audit outbox | P0 | TASK-REQ-024-01, 02, 025-01, 035-05 | ✅ DONE 2026-10-08 (thiếu ai.*, approve pre_deploy) |
| | [TASK-REQ-035-07](./TASK-REQ-035-07-rate-limit-and-webhook-replay.md) | Rate limit, trần đồng thời, chống phát lại webhook | P1 | 035-04, 035-06, TASK-REQ-004-08 | ✅ DONE 2026-10-08 |
| | [TASK-REQ-035-08](./TASK-REQ-035-08-retention-erase-export.md) | Lưu giữ, xoá, xuất | P1 | 035-01, 035-04, 035-06 | ✅ DONE 2026-10-08 (thiếu trace/ledger) |
| | [TASK-REQ-035-09](./TASK-REQ-035-09-secret-redaction-application-and-supply-chain.md) | Áp dụng `secretscan`, `env` agent, supply chain | P0 | 035-01, 035-04, 035-06, SOL-004, BE-REQ-SOL-034 | 🚧 Một phần |

## Thứ tự phụ thuộc

```
035-01 ────────────────────────────────────────────────┬─▶ 035-08 ─▶ 035-09
035-02 ─┐                                                │              ▲
035-03 ─┴─▶ 035-04 ─▶ 035-07                             │              │
035-05 (RLS) ──────────────────▲                         │              │
024-01, 024-02, 025-01 ─▶ 035-06 ───────────────────────┘──────────────┘
```

## Ghi chú

- Làm sớm (độc lập hoặc nền tảng cho CR khác): 035-01 (CR 031, 034, 008 dùng), 035-05 (mọi repository), 035-02 và 035-03.
- Số migration `NNNN` ở 035-06 (và 035-05 nếu cần): `ls backend-go/services/request-service/migrations/{postgres,mysql}` lúc làm; hai dialect cùng số; chờ TASK-REQ-025-01.
- Chạy `gitnexus_impact` trước khi sửa `AttachIdentity`, `Executor.CallTool`, `tenant.*`, `auditclient.Client`, `CreateRequest` use case (quy ước repo); `grpcmw`/`tenant` dùng chung 16 service, báo mức rủi ro (CRITICAL) trong PR và chỉ thêm, không đổi hành vi.
- Lệnh chung: `cd backend-go && go build ./... && go test ./common/... ./services/request-service/... ./services/api-gateway/... && opa test policy/orca-authz -v && go test -tags=integration ./services/request-service/internal/adapter/...` (cần Docker cho tích hợp). Chưa chạy.
- Điều kiện GA (CR mục 2.9, không có task mã): review thiết kế bảo mật độc lập, kiểm thử thâm nhập các điểm T1, T3, T4, T5, quét ảnh; kết quả ghi vào tài liệu của CR-REQ-025.
- Số liệu (hạn mức tốc độ, trần đồng thời, thời hạn lưu giữ, cache vai trò 30 giây, cửa sổ 5 phút) là đề xuất chưa đo.
