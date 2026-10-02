# backend-go Solutions — MCP Service Foundation (v5)

**CRs:** [docs/crs/v5/mcp-service-foundation](../../../../../../docs/crs/v5/mcp-service-foundation/README.md)
**Hợp đồng FE:** [CONTRACT-mcp-ui-api.md](../../CONTRACT-mcp-ui-api.md) · **Quy ước chung + bảng T1..T8:** [crs/v5/README.md](../../README.md)
**TDD tham chiếu:** [`arch/02`](../../../../tdd/architecture/02-microservices-decomposition.md), [`arch/03`](../../../../tdd/architecture/03-clean-architecture-guidelines.md), [`arch/05`](../../../../tdd/architecture/05-data-architecture.md), [`api-gateway.md`](../../../../tdd/services/api-gateway.md) §3, §6, §9

## Re-verify trước khi thiết kế (đối chiếu CR với mã thật, 2026-10-01)

| Khẳng định của CR | Kết quả khi đọc mã | Lệch? |
|---|---|---|
| `go.work` có 18 service, không có MCP | Đúng: 18 thư mục `backend-go/services/*` (kể cả `issue-status-sync`); `go.work` còn có `./common`, `./proto`, `./cmd/orca-cli`. Không có `mcp-service`, không có `modelcontextprotocol` trong bất kỳ `go.mod` | Không |
| Thêm vào `Makefile` "dòng ~10" | `SERVICES :=` ở dòng 6–10 chỉ có **17** tên — **thiếu `issue-status-sync`** (có trong `go.work`) ⇒ `make build/test/lint` hiện không chạm module đó | **Lệch có sẵn** (không thuộc phạm vi, ghi nhận; SOL-001 chỉ thêm `mcp-service`) |
| `usage-service` có `migrations/postgres` + `migrations/mysql`, "xác nhận chuẩn hiện hành" | Đúng, và **cả 16 service có DB đều có thư mục `mysql/`**; `arch/04` ghi multi-dialect đã được xác nhận (CR-DB-001, Option B). README v5 ghi "Postgres-only mặc định cho service mới" — **mâu thuẫn** với TDD hiện hành | **Lệch** ⇒ quyết định ở SOL-001 (§Quyết định 1) |
| RLS "như `usage-service`" | `usage.sessions` có `ENABLE ROW LEVEL SECURITY` + policy `app.tenant_id`, **nhưng** không đâu gọi `SET LOCAL app.tenant_id` và pool kết nối bằng owner nên RLS **không bao giờ kích hoạt** (chính `repository_test.go:130` của usage-service ghi điều này) | **Lệch** ⇒ SOL-001 phải làm RLS thật (`FORCE` + `set_config` + role không-superuser) để đạt acceptance "tenant A không đọc được B" |
| Docker/compose: "thêm DB `mcp` vào `deploy/postgres-init-databases.sh` và `docker-compose.yml`" | `backend-go/docker-compose.yml` chỉ chạy hạ tầng (postgres/vault/nats) và mount script init; **stack dev thật** (service + nginx) ở `/opt/repos/orca/deploy/dev/docker-compose.yml` (mỗi service là binary `./bin/<svc>/orca` mount vào image chung), cùng `deploy/dev/scripts/{build-local,migrate}.sh` có danh sách service riêng | Bổ sung: CR thiếu các file `deploy/dev/*` |
| `router.go` mount `/ws` + nhóm `authed`, không có path MCP | Đúng (`router.go:107-216`); không có `NotFound` tuỳ biến nên path lạ ⇒ chi 404 mặc định | Không |
| api-gateway "owns no database" | Đúng (README api-gateway) | Không |
| `wsbridge` đặt `InsecureSkipVerify: true` cho WS upgrade | Đúng (`wsbridge/handler.go:80`) **và `wscompat/handler.go:91` cũng vậy** — tức `/ws` (kênh mà UI dùng cho `mcp.consent.decide`, `mcp.approval.decide`, `mcp.token.create`) xác thực bằng cookie nhưng **không kiểm Origin** ⇒ rủi ro cross-site WebSocket hijacking cho các kênh MCP có tác dụng phụ | **Mở rộng rủi ro** ⇒ SOL-002 §D.3 |
| `http.Server` công khai của gateway | `publicServer := &http.Server{Addr, Handler}` **không có timeout nào** (`main.go:467`) ⇒ CR-002 mục D ("timeout đọc header") cần sửa thật ở đây | Xác nhận gap |

## Solutions

| Solution | CR | Service / Area | Effort | Status |
|---|---|---|---|---|
| [BE-MCP-SOL-001](./BE-MCP-SOL-001-scaffold-mcp-service.md) | CR-MCP-001 | `mcp-service` (mới), `proto`, `go.work`, `deploy/*`, CI | Medium | ✅ Implemented (unit/integration tests) — see service README for gaps |
| [BE-MCP-SOL-002](./BE-MCP-SOL-002-gateway-mcp-endpoint-wiring.md) | CR-MCP-002 | `api-gateway` (`adapter/mcpserver`, `wscompat/channels_mcp*.go`, config, nginx) | Small | ✅ Implemented (unit/integration tests) — see service README for gaps |

## Thứ tự thực thi & phụ thuộc

```
SOL-001 (mcp-service + proto GetServerInfo + RLS thật)
   └─▶ SOL-002 (gateway: route, Origin/body/cookie guard, kênh mcp.* khung + gating MCP_DISABLED/MCP_NOT_ADMIN, nginx)
          └─▶ BE-MCP-SOL-003 (transport + mcp.server.info)  ← cắm vào seam do SOL-002 để lại
```

- SOL-002 chỉ cần *proto đã sinh* của SOL-001 (client `mcpv1.McpServiceClient`); có thể làm song song nếu SOL-001 chốt `mcp.proto` trước (§2.C của SOL-001).
- **Mốc an toàn:** `MCP_ENABLED` mặc định `false` ở cả hai solution; không route nào của `/mcp` được mount khi cờ tắt.

## Quyết định đã chốt (2026-10-01) & điều còn mở trước khi code

1. **Đã chốt (D2):** `mcp-service` **Postgres-only tạm thời** (SOL-001 §Quyết định 1); không có thư mục `mysql/`, fail-fast với DSN khác. Hỗ trợ đa dialect về sau sẽ cần `migrations/mysql`, `adapter/mysql`, thay các câu SQL đặc thù Postgres (`FOR UPDATE SKIP LOCKED`, RLS, JSONB) và CI matrix như `backend-go-usage-service.yml`.
2. **Đã chốt (D3):** allow-list Origin (`WS_ALLOWED_ORIGINS`, `MCP_ALLOWED_ORIGINS`) được phê duyệt và đang hiện thực — xem SOL-002 §D.3.
3. **Đã chốt (D6):** `MCP_TENANT_DEFAULT_ENABLED` (mặc định `true`) cho tenant mới; `MCP_ENABLED` vẫn là công tắc tổng mặc định `false` — xem SOL-002 §A, SOL-012.
4. Bản Go: `go.work` ghi `go 1.26.0`, các `go.mod` service ghi `go 1.25.0`, `Dockerfile` dùng `golang:1.25-bookworm` — module mới theo **1.25.0** (khớp `usage-service`).
