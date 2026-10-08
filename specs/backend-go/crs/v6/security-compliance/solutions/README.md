# backend-go Solutions: Security and Compliance (v6)

**CRs:** [docs/crs/v6/security-compliance](../../../../../../docs/crs/v6/security-compliance/README.md)
**Hợp đồng chung:** [docs/crs/v6/README.md](../../../../../../docs/crs/v6/README.md) (mục 8 thắng mục 3)
**TDD tham chiếu:** [`arch/03`](../../../../tdd/architecture/03-clean-architecture-guidelines.md), [`arch/05`](../../../../tdd/architecture/05-data-architecture.md), [`arch/06`](../../../../tdd/architecture/06-secrets-vault-architecture.md), [`arch/07`](../../../../tdd/architecture/07-security-architecture.md), [`arch/08`](../../../../tdd/architecture/08-inter-service-communication.md), [`arch/09`](../../../../tdd/architecture/09-observability-reliability.md), [`api-gateway`](../../../../tdd/services/api-gateway.md), [`auth-service`](../../../../tdd/services/auth-service.md)

> 🚧 Đang triển khai: 1/9 task xong (035-01), 035-02 một phần; còn lại chưa làm. P0: chặn rollout (CR-REQ-025). `request-service` chưa có trên đĩa; mọi file của nó là "(mới)".

## Bảng CR, Solution, Task

| CR | Solution | Service / Area | Effort | Task |
|----|----------|----------------|--------|------|
| [CR-REQ-035](../../../../../../docs/crs/v6/security-compliance/CR-REQ-035-security-and-compliance-baseline.md) | [BE-REQ-SOL-035](./BE-REQ-SOL-035-security-and-compliance-baseline.md) | `request-service`, `common` (`secretscan`, `grpcmw`, `tenant`, `auditclient`), `policy/orca-authz`, `api-gateway`, `ci` | Large | `TASK-REQ-035-01` đến `-09` (🚧 7/9 xong, 2 một phần) |

## Re-verify trước khi thiết kế (đối chiếu CR với mã thật, 2026-10-06)

| Khẳng định của CR | Kết quả khi đọc mã | Lệch? |
|---|---|---|
| Gateway không kiểm quyền trước định tuyến; `TenantExtractionInterceptor` tin metadata | Đúng (`grpcmw.go`, `dial.go`: `insecure.NewCredentials()`) | Không |
| `internalcaller.Guard` nhận khoá | `Guard(expectedToken, fullMethods...)`: **một** token mỗi thể hiện; token rỗng từ chối hết | Cần hai thể hiện (C2) |
| Thêm `MetadataActorType` vào `grpcmw`, gateway điền | `AttachIdentity` gọi ở ~20 file; `grpcmw` dùng chung 16 service (blast radius CRITICAL) | **Lệch** ⇒ chỉ thêm hằng, đọc actor từ ctx (C1) |
| Tool MCP đi qua đường nhận biết được | `Executor.CallTool` dựng `Identity` từ `Principal`: điểm duy nhất cho tool | Gắn `agent` ở đây |
| `AppendDetailed` nhận `Metadata map[string]any` | TASK-REQ-024-01 định nghĩa `Entry.MetadataJSON string` | Một nơi định nghĩa: 024-01 (C4) |
| `request_audit_outbox` giao qua `common/outbox` | `common/outbox` chỉ đẩy lên NATS, không gọi RPC | **Lệch** ⇒ bộ giao riêng (C5) |
| RLS `mcp-service` chạy thật, `task-service` im lặng | Đúng (`tenant_tx.go`; `share_link.go` ghi "never SET") | Không |
| `mcp-service/internal/domain/secret_redactor.go` có bộ mẫu | Có 9 mẫu, nằm trong `internal/` | Gói mới, vector chung (C8) |
| `reporter_id` băm không đảo ngược | Dùng để cấp quyền reporter | HMAC UUID (C6) |
| `ExportTenantRequests` truyền | `ChainUnary` chỉ unary | Thêm chuỗi stream (C3) |

## Thứ tự thực thi và phụ thuộc

```
035-01 (secretscan) ───────────────────────────────▶ 035-09 (áp dụng, CI)
035-02 (actor type, gateway) ─┐
035-03 (request.rego) ────────┴─▶ 035-04 (interceptor, catalog) ─▶ 035-07 (rate limit, webhook)
035-05 (RLS, test quét) ──────────────────────────────────────────────▲
TASK-REQ-024-01, 02 ─▶ 035-06 (migration, audit outbox) ─▶ 035-08 (retention, erase, export) ─▶ 035-09
```

## Quyết định chung của feature

| # | Quyết định | Lý do |
|---|-----------|-------|
| X1 | Thi hành quyền bằng interceptor ở `request-service` | Gateway không có OPA |
| X2 | Hai `Guard`, hai token (`GATEWAY_INTERNAL_TOKEN`, `SERVICE_INTERNAL_TOKEN`) | Tách cổng công khai và nội bộ; fail closed |
| X3 | Ma trận quyền Rego, tập người duyệt Go | Dữ liệu nhỏ đúng chỗ OPA; tập người duyệt phụ thuộc dữ liệu |
| X4 | `agent` không duyệt, không chạy, không quản trị, kể cả khi người dùng phía sau là admin | Cổng tồn tại để người kiểm soát agent |
| X5 | `FORCE RLS` + `set_config` mỗi giao dịch; MySQL dùng test quét AST | Không có RLS ở MySQL |
| X6 | Audit không chứa nội dung; hành động đặc biệt đi qua bảng outbox riêng | Không mất, không lộ |
| X7 | Ẩn danh hoá thay vì xoá hàng; `ErasableColumns` + test quét lược đồ | Giữ thống kê; cột mới không lọt |

## Điều còn mở

- Bảng nhóm hành động cần chủ sản phẩm xác nhận (Q1); `viewer` chưa có ở `project-service` (Q3).
- Webhook chạy với tư cách `user` đã cấu hình (Q2); đổi chuỗi ký webhook phá vỡ người gửi cũ (cần cờ chuyển tiếp).
- Xoá nội dung ở `task-service`, sao lưu và phục hồi thảm hoạ chưa có chủ (Q4).
- Chưa có mTLS; `internalcaller` là lớp duy nhất giữa gateway và `request-service`.
