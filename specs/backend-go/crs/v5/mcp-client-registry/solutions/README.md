# backend-go Solutions — MCP Client Registry (v5)

**CR:** [docs/crs/v5/mcp-client-registry](../../../../../../docs/crs/v5/mcp-client-registry/CR-MCP-014-external-mcp-server-registry.md)
**Hợp đồng:** [CONTRACT-mcp-ui-api.md](../../CONTRACT-mcp-ui-api.md) · **Quy ước + bảng T1..T8:** [../../README.md](../../README.md) · **Phía FE:** [FE-MCP-SOL-011](../../../../../frontend/crs/v5/mcp-client-registry/solutions/FE-MCP-SOL-011-external-mcp-servers-ui.md)

## Solutions

| Solution | CR | Area | Effort | Status |
|---|---|---|---|---|
| [BE-MCP-SOL-014](./BE-MCP-SOL-014-external-mcp-server-registry.md) | CR-MCP-014 | `mcp-service` (registry, prober, `ResolveAgentMcpConfig`), `api-gateway` (`channels_mcp_external_server.go`), `credential-broker-service` (category 7), `infra-fleet-service` (hook spawn); `agent/` chỉ ghi phụ thuộc | Large (8–10 ngày; chưa tính D2/D3 phía `agent/`; D1 đã chốt, xem dưới) | ✅ Implemented (unit/integration tests) — see service README for gaps |

## Re-verify (CR/TDD vs mã thật, 2026-10-01)

| Khẳng định | Kết quả | Lệch? |
|---|---|---|
| `tenant-service/internal/domain/profile_resolution.go:231-282` `mergeMCPServers` chỉ dedupe theo `name`, lớp ưu tiên cao thắng, giữ thứ tự thấy đầu tiên | Đúng; gọi từ `ResolveProfile` (dòng 109); `ResolveProfile` chỉ có 1 caller production `usecase/get_resolved_profile.go:74`; test `profile_resolution_test.go:198-234` | Không lệch |
| Entry profile là tham chiếu server | Entry được giữ **nguyên** (kể cả `command/args/env` inline; FE `types/profile-types.ts` `McpServerConfig` có các field này) | **Lệch/bổ sung**: registry phải coi profile chỉ là danh sách tên |
| Có thể biết lớp profile của từng server | `GetResolvedProfileResponse` chỉ có `resolved_settings_json`; `Sources` không ra proto | Lệch với kỳ vọng "ProfileSourceBadge" — FE ghi là tương lai |
| `agent/src/shared/mcp-config.ts` chỉ đọc/tóm tắt, không phải nguồn sự thật | Đúng: 3 format (`workspace`/`cursor`/`claude`), `maskMcpEnv` che env theo regex; không có đường ghi cấu hình cho agent. `agent-spawn-types.ts` `AgentSpawnRequest` không có trường MCP | Không lệch; xác nhận cần thay đổi phía agent (D2) |
| Agent CLI cần cấp MCP | `agent-binary-specs.ts`: `claude`, `codex`, `gemini`, `opencode`, `ollama`; ngoài ra ~30 `TuiAgent` ở `frontend/src/shared/tui-agent-config.ts` (đường TUI cục bộ, không qua backend) | v1 chỉ 4 CLI đầu |
| Secret chỉ qua Vault/`credential-broker-service`, plaintext chỉ có ở env tiến trình con | Broker có `…ByOwner` đủ dùng, nhưng **coi envelope là opaque**, không giải mã; enum category thật khác TDD; CHECK `0001_init.up.sql` (cả `postgres/` và `mysql/`) chỉ có 5 giá trị ⇒ **cần** migration `0004` thêm `mcp_external_secret` (đã xác minh) | **Lệch lớn → D1 đã chốt (2026-10-01):** không dùng envelope client; UI gửi plaintext qua WS/TLS, `mcp-service` ghi vào broker, broker Transit-encrypt; che `value` ở log/trace (CONTRACT C11) |
| Có guard SSRF tái dùng | `workflow-service/.../webhook.go` có nhưng resolve-rồi-kiểm (TOCTOU) | Viết mới, pin bằng `Dialer.Control` |
| `mcp_depth` | Chỉ có trong docs CR-013/014 | Chưa có mã — claim định nghĩa trong BE-014/BE-006 |
| `mcp-service` | Không tồn tại trong `backend-go/services` | Tất cả đường dẫn là kế hoạch |

## Thứ tự & phụ thuộc

```
BE-MCP-SOL-001/002 (scaffold, config, migration 0001) ──► BE-MCP-SOL-014
BE-MCP-SOL-006 (mint token agent, aud/mcp_depth) ──────► ResolveAgentMcpConfig (mục F)
BE-MCP-SOL-013 (kill switch, audit chuẩn) ─────────────► khuyến nghị làm trước
Song song, ticket ngoài solution:
  (D1 đã chốt: plaintext qua TLS, không còn chặn)   D2 agent/: mcpConfig trong agent.spawn   D3 agent/: orca-mcp-launch (stdio sandbox)
FE-MCP-SOL-011 ◄── CONTRACT §2.2 (chờ CR-1..CR-5 nếu được chấp nhận)
```
Tách PR gợi ý: (1) migration + domain + prober + usecase CRUD/review; (2) kênh gateway + OPA; (3) broker category 7 + allow-list; (4) `ResolveAgentMcpConfig` + renderer 4 CLI (sau golden test/spike); (5) hook `infra-fleet-service` sau cờ `MCP_AGENT_CONFIG_ENABLED`.

## Yêu cầu đổi CONTRACT (trạng thái)
CR-1 *(đã thay bằng D1: `setSecret{serverId,kind,name,value}` plaintext, không `iv`; CONTRACT cập nhật kèm C11)*; CR-2 `probe` thêm `approvedTools?`/`transport?`; CR-3 thêm mã `MCP_SERVER_INVALID`, `MCP_SERVER_STDIO_NOT_ALLOWED`, `MCP_SERVER_DIGEST_MISMATCH`, `MCP_SERVER_NAME_CONFLICT`; CR-4 (tuỳ chọn) owner được `probe` server user-scope của mình; CR-5 (tuỳ chọn) `updatedAt?`. Các CR-2..CR-5 đã nằm trong CONTRACT. **T9** đã ghi trong README v5 §3 (secret MCP qua broker category `mcp_external_secret`).
