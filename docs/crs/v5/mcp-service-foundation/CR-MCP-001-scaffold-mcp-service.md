# CR-MCP-001 — Dựng `mcp-service`: nơi giữ trạng thái và chính sách của lớp MCP

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-MCP-001 |
| **Tên** | Tạo service Go mới `mcp-service` (gRPC + DB riêng) theo layout Clean Architecture của `usage-service` |
| **Loại** | Feature (service mới) |
| **Priority** | 🔴 P0 — điều kiện tiên quyết của CR-005/006/010/012/013/014 |
| **Effort** | Medium (4–6 ngày: scaffold, proto, migration, repo, wiring, CI) |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-01 |
| **Trạng thái** | ✅ Đã triển khai (unit/integration test) — xem solution và README service để biết khoảng trống |
| **Tác giả** | Khảo sát trực tiếp `backend-go/` theo yêu cầu hỗ trợ MCP |
| **Phụ thuộc** | Không |
| **Mở khoá** | Toàn bộ feature còn lại của v5 |

---

## Bối cảnh & Vấn đề

1. `backend-go/go.work` liệt kê 18 module service; **không có** service nào cho MCP. Phần duy nhất chạm tới MCP ở backend-go là `tenant-service/internal/domain/profile_resolution.go` (gộp `mcp.servers` trong profile) — chỉ là xử lý cấu hình.
2. Lớp MCP cần **trạng thái bền** mà api-gateway không được phép giữ (README api-gateway: *"owns no database"*): OAuth client đã đăng ký, consent/grant, session, token MCP, chính sách tool, yêu cầu phê duyệt, registry MCP server ngoài.
3. Nếu nhét các thứ này vào `auth-service` hoặc `tenant-service` thì vi phạm ranh giới trách nhiệm (auth = danh tính/phiên; tenant = tổ chức/profile) và làm hai service đó phình thêm theo nhịp thay đổi của một giao thức bên ngoài (spec MCP thay đổi nhanh).

## Giải pháp đề xuất

### A. Module `backend-go/services/mcp-service`

Sao layout của `usage-service` (service tham chiếu theo README):

```
services/mcp-service/
├── cmd/server/main.go          # composition root; GRPC_PORT/HTTP_PORT/DATABASE_DSN theo common/config
├── internal/config/config.go
├── internal/domain/            # entity thuần: OAuthClient, McpSession, Grant, ToolPolicy, ApprovalRequest, ExternalServer
├── internal/usecase/           # ports.go + usecase (mỗi file một usecase, như usage-service)
├── internal/adapter/{grpc,postgres}/            # mysql: không có (D2, Postgres-only tạm thời)
├── migrations/postgres/        # 0001_init.up/down.sql ...
└── deploy/Dockerfile
```

Đăng ký module: thêm `./services/mcp-service` vào `go.work`; thêm vào danh sách service ở `Makefile` (dòng ~10); thêm DB `mcp` vào `deploy/postgres-init-databases.sh` và `docker-compose.yml`.
**Đã chốt (2026-10-01, D2):** `usage-service` có cả `migrations/postgres` lẫn `migrations/mysql` (hướng multi-database, xem `docs/crs/v4/multi-database`), nhưng `mcp-service` **Postgres-only tạm thời**: không tạo `adapter/mysql` hay `migrations/mysql`, fail-fast nếu DSN không phải Postgres. Đa dialect về sau cần thêm adapter + migration MySQL, thay các câu SQL đặc thù Postgres (`FOR UPDATE SKIP LOCKED`, JSONB, RLS) và CI matrix (chi tiết: BE-MCP-SOL-001).

### B. Proto `proto/orca/mcp/v1/mcp.proto`

Chỉ khai báo bề mặt nội bộ (api-gateway ↔ mcp-service), không phải wire protocol MCP:

| RPC (gợi ý) | Dùng bởi CR |
|-------------|-------------|
| `RegisterOAuthClient / GetOAuthClient` | CR-005 |
| `CreateGrant / RevokeGrant / ListGrants` | CR-005, CR-006 |
| `CreateSession / TouchSession / CloseSession` | CR-004 |
| `GetEffectiveToolPolicy / UpsertToolPolicy` | CR-012 |
| `CreateApproval / ResolveApproval / ListPendingApprovals` | CR-013 |
| `ListExternalServers / UpsertExternalServer` | CR-014 |

Chạy `buf lint` + `buf breaking` (đã cấu hình `STANDARD`/`FILE` trong `proto/buf.yaml`); sinh stub vào `proto/gen/go/orca/mcp/v1`.

### C. Lược đồ DB tối thiểu (schema `mcp`, mọi bảng có `tenant_id` + RLS như `usage-service`)

`oauth_clients`, `grants`, `mcp_sessions`, `tool_policies`, `approval_requests`, `external_servers`, `processed_events` (dedup NATS, theo mẫu notification-service), `outbox` (theo mẫu `usage-service` `0002_outbox`). Ràng buộc: khoá ngoại về user/tenant chỉ là **ID tham chiếu** (không FK xuyên DB).

### D. Quy ước bắt buộc của repo

- Không đặt tên file/folder `helpers/utils/common` (AGENTS.md).
- Mọi gRPC call có deadline; lỗi map qua `common/apperrors`.
- Secret (client secret, signing key) đi qua Vault/`credential-broker-service`, **không** lưu rõ trong bảng.
- Không thêm `max-lines` disable.

## Acceptance Criteria

- [ ] `make build`, `make vet`, `make test`, `make lint` xanh với module mới trong `go.work`.
- [ ] `buf lint` và `buf breaking` xanh.
- [ ] `mcp-service` khởi động với `make dev-up`, `/healthz` và gRPC health trả OK; migration `up`/`down` chạy sạch.
- [ ] RLS: test tích hợp chứng minh tenant A không đọc được dòng của tenant B.
- [ ] `README.md` của service ghi rõ phần "real vs stub" như các service khác.

## Rủi ro & Ngoài phạm vi

- **Rủi ro:** scaffold lệch chuẩn so với service khác → dùng `usage-service` làm mẫu, review chéo.
- **Ngoài phạm vi:** logic giao thức MCP (CR-003), cấp token (CR-005), nội dung policy (CR-012).
