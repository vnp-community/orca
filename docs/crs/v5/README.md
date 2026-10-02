# Change Requests v5 — MCP (Model Context Protocol) cho `backend-go`

> **Mục tiêu:** để một AI agent (Claude Desktop, Claude Code, Cursor, agent do chính Orca khởi chạy...) tương tác
> với Orca **như một người dùng thao tác qua giao diện** — tạo/chọn project, mở worktree, chạy terminal, giao task
> cho agent, review PR, vận hành workflow/automation — thông qua một **MCP server đầy đủ theo chuẩn** và một
> **service MCP** quản lý trạng thái/chính sách đi kèm.
>
> **Phạm vi:** `backend-go/` (api-gateway + service mới `mcp-service`). Không thay đổi `agent/` hay `backend/` (TS cũ).

## 1. Hiện trạng đã xác nhận (khảo sát mã nguồn 2026-10-01)

| Hạng mục | Trạng thái thật | Bằng chứng |
|----------|-----------------|------------|
| Service MCP trong `backend-go` | ❌ Không có (18 service, không service nào tên/chức năng MCP) | `backend-go/go.work` |
| Thư viện MCP (SDK) trong các `go.mod` | ❌ Không có | grep `modelcontextprotocol`/`mcp-go` → 0 kết quả |
| `tools/list`, `tools/call` | ⚠️ Chỉ trong `agent/src/relay/agent-rpc-dispatch-misc.ts`, 9 tool nội bộ (`gh`, `git`, `shell`...), **không có** `initialize`/handshake/transport MCP chuẩn | `agent-tool-registry.ts` |
| `mcp.servers` trong profile | ⚠️ Chỉ **gộp cấu hình** (dedupe theo `name`), không có ai chạy/đăng ký các server đó ở backend-go | `tenant-service/internal/domain/profile_resolution.go:237` |
| Bề mặt chức năng mà UI dùng | ✅ Có sẵn: `wscompat.Registry` ánh xạ `channel → handler`, UI gọi qua `GET /ws`. Comment trong `registry.go` ghi 262 method / 36 namespace; đếm thô ~400 chuỗi `ns.method` trong `channels*.go` | `api-gateway/internal/adapter/wscompat/registry.go` |
| Danh sách channel có thể liệt kê (introspection) | ❌ Các map `handlers`... là field private, không thấy method liệt kê trong phần mã đã đọc | `registry.go:46-62` |
| Schema đầu vào của channel | ❌ `ChannelHandler` nhận `[]json.RawMessage` theo vị trí, không có schema | `registry.go:44` |
| Authn | ✅ Cookie `orca_session` + Bearer JWT RS256 (JWKS) + CLI token (`aud` cố định `orca-cli`) | `api-gateway/README.md`, `auth_cli_token_routes.go:13` |
| `Identity.Role` | ⚠️ **Chỉ** được điền ở nhánh cookie; nhánh Bearer JWT để trống → các channel admin fail-closed | `registry.go:29-35` |
| Authz (OPA) trước routing | ❌ README api-gateway tự ghi "No OPA authorization check ahead of routing" | `api-gateway/README.md` "Still not production-safe" |
| Audit log | ✅ `auth-service` có `AppendAuditEntry`/`QueryAuditLog` | `proto/orca/auth/v1` |
| Rate limit theo tenant | ✅ In-memory token bucket, áp trước mọi route | `usecase/rate_limit.go` |

**Kết luận:** *chức năng* (tầng handler) đã có gần đủ cho UI; thứ thiếu là **giao thức MCP, lớp mô tả tool (schema), xác thực/ủy quyền phù hợp với agent, quản trị an toàn, và trạng thái lưu bền**. Vì vậy các CR dưới đây **tái sử dụng handler hiện có** thay vì viết lại ~400 thao tác.

## 2. Quyết định kiến trúc (áp dụng cho mọi CR v5)

| # | Quyết định | Lý do |
|---|-----------|-------|
| D1 | **Transport MCP đặt trong `api-gateway`** (path `/mcp`, thư mục `internal/adapter/mcpserver/`) | api-gateway là listener ngoài duy nhất, đã có authn + rate limit; tránh mở thêm cổng công khai |
| D2 | **Service mới `mcp-service`** (gRPC, Postgres DB riêng `mcp`) giữ trạng thái: OAuth client đã đăng ký, session, grant/consent, policy tool, approval, registry MCP server ngoài | Database-per-service; api-gateway không sở hữu DB ([README api-gateway](../../../backend-go/services/api-gateway/README.md)) |
| D3 | **Thực thi tool = gọi lại `wscompat.Registry.Dispatch` trong tiến trình** với cùng `Identity` | UI parity: cùng handler, cùng kiểm tra quyền, cùng timeout 60s; không phát sinh đường thực thi thứ hai |
| D4 | **Dùng SDK chính thức, đã chốt (2026-10-01):** server dùng Go SDK `github.com/modelcontextprotocol/go-sdk` cho khung JSON-RPC/transport (ADR ghi phiên bản/giấy phép và khoảng trống hook; spike chỉ kiểm hook Streamable HTTP/session-store); **client kiểm thử/tham chiếu trong CI** dùng MCP Python SDK chính thức (PyPI `mcp`, `modelcontextprotocol/python-sdk`, phiên bản ghim ở `backend-go/ci/mcp-conformance/requirements.txt`; API chưa xác minh — kiểm khi cài đặt). Harness Go giữ cho test in-process | Không tự chế lại giao thức; hưởng bản vá spec; client độc lập SDK server bắt lỗi tương thích |
| D5 | **Phiên bản spec:** tối thiểu `2025-06-18`, thương lượng qua `initialize` + header `MCP-Protocol-Version`; đối chiếu bản mới hơn khi triển khai | Spec thay đổi nhanh; không hard-code một phiên bản |
| D6 | **Mặc định an toàn:** tool phá huỷ/nhạy cảm bị chặn hoặc cần phê duyệt; secret không bao giờ ra khỏi server | Agent là "người dùng không đáng tin tuyệt đối" (prompt injection) |
| D7 | Tên tool = tên channel thay `.` bằng `_` (`task.create` → `task_create`) | Một số client MCP giới hạn ký tự `[a-zA-Z0-9_-]{1,64}`; test chống va chạm tên |

### Quyết định đã chốt bổ sung (2026-10-01)

Chi tiết và nơi áp dụng: [specs/backend-go/crs/v5/README.md](../../../specs/backend-go/crs/v5/README.md) mục "Quyết định đã chốt". Không trùng số với D1–D7 ở trên (đây là nhãn của đợt quyết định, ký hiệu `Q-D1..Q-D6` khi trích dẫn từ CR):

| Nhãn | Quyết định | CR chịu ảnh hưởng |
|------|-----------|-------------------|
| Q-D1 | Secret server ngoài: UI gửi plaintext một lần qua WS/TLS (`setSecret{serverId,kind,name,value}`), server ghi vào credential-broker (Vault Transit); không dùng phong bì client; `value` bị che ở log/trace | CR-014 |
| Q-D2 | `mcp-service` Postgres-only tạm thời, fail-fast DSN khác | CR-001 |
| Q-D3 | Allow-list Origin (`WS_ALLOWED_ORIGINS`, `MCP_ALLOWED_ORIGINS`) được phê duyệt, đang hiện thực; rỗng ⇒ hành vi cũ + WARN | CR-002 |
| Q-D4 | Python SDK chính thức làm client tham chiếu trong CI (cùng Go SDK cho server, xem D4 ở trên) | CR-003, CR-015 |
| Q-D5 | Web Push deep link `/?section=mcp&tab=approvals&approval=<id>`; push thật phụ thuộc CR-NOTIF-002 | CR-013 |
| Q-D6 | Tenant mới bật mặc định (`MCP_TENANT_DEFAULT_ENABLED=true`); an toàn vì mặc định rủi ro/hard-deny/scope/kill switch không đổi | CR-002, CR-012, CR-015 |

## 3. Danh sách feature & CR

| Feature (folder) | CR | Nội dung | Priority | Effort |
|------------------|----|----------|----------|--------|
| [`mcp-service-foundation`](./mcp-service-foundation/README.md) | CR-MCP-001 | Dựng `mcp-service` (proto, DB, migration, wiring) | 🔴 P0 | Medium |
| | CR-MCP-002 | Nối `/mcp` vào api-gateway + cấu hình/TLS/Origin | 🔴 P0 | Small |
| [`mcp-protocol-server`](./mcp-protocol-server/README.md) | CR-MCP-003 | Streamable HTTP + vòng đời `initialize` | 🔴 P0 | Medium |
| | CR-MCP-004 | Session, SSE, resumability, cancel/progress, đa replica | 🟠 P1 | Large |
| [`mcp-authorization`](./mcp-authorization/README.md) | CR-MCP-005 | OAuth 2.1 resource server (RFC 9728/8707, DCR, PKCE) | 🔴 P0 | Large |
| | CR-MCP-006 | PAT/CLI token cho MCP, scope, khôi phục `Role` cho nhánh Bearer | 🔴 P0 | Medium |
| [`mcp-tool-catalog`](./mcp-tool-catalog/README.md) | CR-MCP-007 | Introspection registry + descriptor/schema + parity test | 🔴 P0 | Large |
| | CR-MCP-008 | Tool pack theo domain (UI parity) | 🔴 P0 | XL (chia đợt) |
| | CR-MCP-009 | Tool dài hạn/streaming (terminal, agent run) | 🟠 P1 | Large |
| [`mcp-resources-prompts`](./mcp-resources-prompts/README.md) | CR-MCP-010 | Resources, templates, subscriptions | 🟠 P1 | Medium |
| | CR-MCP-011 | Prompts | 🟡 P2 | Small |
| [`mcp-governance-safety`](./mcp-governance-safety/README.md) | CR-MCP-012 | Policy tool (OPA), annotations, deny-list | 🔴 P0 | Medium |
| | CR-MCP-013 | Phê duyệt (elicitation), audit, kill switch, chống đệ quy | 🔴 P0 | Large |
| [`mcp-client-registry`](./mcp-client-registry/README.md) | CR-MCP-014 | Registry MCP server ngoài + cấp cho agent do Orca chạy | 🟡 P2 | Large |
| [`mcp-quality-rollout`](./mcp-quality-rollout/README.md) | CR-MCP-015 | Conformance, e2e agent, observability, tài liệu, rollout | 🟠 P1 | Medium |

## 4. Thứ tự thực thi

```
CR-001 ─▶ CR-002 ─▶ CR-003 ─┬─▶ CR-005 ─▶ CR-006 ─┐
                            │                      ├─▶ CR-007 ─▶ CR-008 ─▶ CR-009
                            └─▶ CR-012 ────────────┘        │
                                                             ├─▶ CR-010 ─▶ CR-011
CR-004 (song song sau CR-003)     CR-013 (sau CR-012, trước khi bật tool ghi cho production)
CR-014 (độc lập, sau CR-001)      CR-015 (xuyên suốt; gate trước khi GA)
```

**Mốc an toàn bắt buộc:** không bật bất kỳ tool *ghi/phá huỷ* nào (CR-008) ra môi trường có người dùng thật trước khi CR-005, CR-006, CR-012, CR-013 hoàn tất.

## 5. Điều chưa xác minh (cần làm rõ khi triển khai)

- Phiên bản SDK Go chính thức và mức ổn định của Streamable HTTP/OAuth helper tại thời điểm làm CR-003.
- Số lượng channel thực sự "đăng ký" lúc runtime (đếm thô ~400 chuỗi chưa phân biệt channel thật với chuỗi khác) — CR-007 sẽ sinh số liệu chính xác.
- Việc `tasks` primitive (tool dài hạn bất đồng bộ) đã ổn định ở bản spec nào — ảnh hưởng thiết kế CR-009.
- Có cho phép chạy `mcp-service` chung DB cluster với service khác hay cần cluster riêng (theo chuẩn deploy hiện tại).
- API cụ thể của MCP Go SDK và MCP Python SDK (phiên bản, hook session-store/resume, helper OAuth) — kiểm khi cài đặt (spike CR-003, job conformance CR-015).
- `notification-service` chưa có `DeliverPush` (CR-NOTIF-002) nên Web Push của CR-013 chưa gửi được.
