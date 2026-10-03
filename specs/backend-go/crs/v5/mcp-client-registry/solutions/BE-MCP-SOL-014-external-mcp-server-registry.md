# BE-MCP-SOL-014: Registry MCP server ngoài + cấp cấu hình MCP cho agent do Orca chạy

> **✅ Implemented (unit/integration tests) — see Gaps.** Phụ thuộc cứng: BE-MCP-SOL-001/002 (scaffold `mcp-service`, config, migration `0001`), BE-MCP-SOL-006 (mint token MCP ngắn hạn), BE-MCP-SOL-013 (kill switch/audit). Khuyến nghị làm sau BE-013 (theo CR).

**CR:** [CR-MCP-014](../../../../../../docs/crs/v5/mcp-client-registry/CR-MCP-014-external-mcp-server-registry.md)
**Service:** `mcp-service` (chính), `api-gateway` (kênh WS), `credential-broker-service` (category mới), `infra-fleet-service` (điểm hook spawn), `agent/` (**chỉ ghi phụ thuộc**, không sửa ở đây)
**Hợp đồng:** [CONTRACT-mcp-ui-api.md](../../CONTRACT-mcp-ui-api.md) §1 `McpExternalServer`, §2.2 `mcp.externalServer.*`, §2.3 lỗi · **FE:** [FE-MCP-SOL-011](../../../../../frontend/crs/v5/mcp-client-registry/solutions/FE-MCP-SOL-011-external-mcp-servers-ui.md)
**TDD tham chiếu:** arch/06 (secrets), arch/07 (security/audit/OPA), arch/05 (tenant/RLS/outbox), `credential-broker-service.md` §2/§3/§9, `tenant-service.md` (`mcp.servers`), `infra-fleet-service.md`

---

## 1. Trạng thái hiện tại (re-verify bằng mã thật, 2026-10-01)

| Khẳng định (CR / TDD) | Thực tế | Hệ quả |
|---|---|---|
| `mcp-service` tồn tại để đặt registry | **Chưa có** — `ls backend-go/services` không có `mcp-service` | Solution viết theo layout arch/03; đường dẫn dưới đây là *kế hoạch* |
| `mergeMCPServers` ở `profile_resolution.go:231-282` chỉ gộp theo `name`, user thắng | Đúng. Được gọi từ `ResolveProfile` (dòng 109); `ResolveProfile` chỉ được gọi bởi `usecase/get_resolved_profile.go:74` | Không thể lọc bên trong domain (pure, không I/O) — lọc ở nơi tiêu thụ (mục 2.H) |
| Profile chỉ chứa tên server | **Sai một phần**: `McpServerConfig` phía FE (`types/profile-types.ts`) và dữ liệu profile cho phép `command`, `args`, `env` **inline**; `mergeMCPServers` giữ nguyên entry | Entry inline có thể mang secret/lệnh. Registry coi profile chỉ là **danh sách tên**; mọi `command/env/url` trong profile bị bỏ qua khi cấp cho agent |
| `GetResolvedProfileResponse` có nguồn từng field | Chỉ có `resolved_settings_json` (`tenant.proto:179`); `Sources` có trong domain nhưng **không** ra proto | UI không biết server đến từ lớp profile nào (FE-011 ghi "tương lai") |
| Broker giữ secret, trả plaintext cho caller được phép | Proto thật: `WriteCredential/ResolveCredentialByOwner/RevokeCredentialByOwner/GetCredentialMetadataByOwner` có sẵn. Enum thật khác TDD (`CREDENTIAL_CATEGORY_SCM_OAUTH=1 … DEV_SERVER_AGENT_TOKEN=6`) | Thêm `CREDENTIAL_CATEGORY_MCP_EXTERNAL_SECRET = 7` |
| Broker giải mã envelope của browser | **Không**: `write_credential.go` xử lý `EncryptedEnvelope` là *opaque bytes*, Transit-encrypt rồi lưu; `Resolve*` trả lại đúng bytes đó. Khoá client (`lib/credential-crypto.ts`) dẫn xuất từ `sessionToken` mà auth slice **không có** (fallback `'fallback-dev-token'`) | Server không bao giờ giải mã được phong bì client ⇒ **D1 (đã chốt 2026-10-01): không dùng phong bì client cho secret MCP**; UI gửi plaintext một lần qua WS/TLS, `mcp-service` ghi vào broker (broker Transit-encrypt bytes nhận được — đúng nguyên tắc broker TDD §9 "plaintext chỉ trong bộ nhớ trong thời gian request"). Xem 2.B và R1 |
| Allow-list caller của broker | `requestingService(ctx)` chỉ đọc metadata; không thấy `ErrUnauthorizedCategory` trong mã (chưa xác minh enforcement) | Phải thêm allow-list `category → {caller}` (2.B) |
| Migration broker cho phép category mới | **Đã xác minh (2026-10-01):** `migrations/postgres/0001_init.up.sql:19` có `CHECK (category IN ('scm_oauth','issue_tracker_oauth','ai_provider_key','ssh','service_secret'))`; `0002_config_json` và `0003_owner_id_text` không đụng category; `dev_server_agent_token` cũng không có trong CHECK. Bản `migrations/mysql/0001_init.up.sql` có `credential_metadata_category_check` tương tự | **Phải** có migration `0004` sửa CHECK ở cả `postgres/` và `mysql/` của broker (thêm `'mcp_external_secret'`, kèm `dev_server_agent_token` nếu mã dùng) — nếu không `WriteCredential` category 7 sẽ vi phạm CHECK |
| Guard SSRF có sẵn | `workflow-service/.../webhook.go` `checkTarget`: resolve rồi kiểm, nhưng `http.Client` **resolve lại** khi dial (TOCTOU/rebinding), thiếu CGNAT `100.64/10` | **Không** tái dùng nguyên xi; viết `ssrfpolicy` pin bằng `Dialer.Control` (2.C) |
| Agent CLI có trong `agent/` | `agent/src/relay/agent-binary-specs.ts`: `claude`, `codex`, `gemini`, `opencode`, `ollama` (không MCP). `agent-spawn-env.ts` `buildAgentEnv` có `extraEnv` nhưng `AgentSpawnRequest` **không** có trường MCP | Cần thay đổi phía agent (D2). `agent/src/shared/mcp-config.ts` chỉ *đọc/tóm tắt* `.mcp.json`/Cursor/Claude, không ghi |
| Điểm spawn backend | `infra-fleet-service` `usecase/start_agent_session.go` → `devserveragent.SpawnAgent` → `agent.spawn` (params cố định, không MCP) | Hook ở đây (2.G) |
| `mcp_depth` | Chỉ xuất hiện trong CR-013/CR-014; chưa có mã | Định nghĩa claim ở 2.F, đồng bộ BE-006/013 |

### Quyết định khác/thêm so với CR gốc

1. **Không sửa `mergeMCPServers` và không sửa `tenant-service`.** CR cho phép "bước lọc ở nơi gọi"; ta chọn nơi gọi là `mcp-service.ResolveAgentMcpConfig` (nó tự gọi `TenantService.GetResolvedProfile`, parse `mcp.servers[].name`, rồi đối chiếu registry). Lệnh phải chạy trước khi quyết định đụng tenant-service: `gitnexus impact({target:"mergeMCPServers", direction:"upstream"})` và `impact({target:"ResolveProfile"})` — **chưa chạy**, rủi ro dự kiến LOW (1 caller production) nhưng chỉ cần nếu sau này buộc phải sửa.
2. **stdio không bao giờ chạy trong pod `mcp-service`.** Chỉ chạy trên host của agent (dev server) do chính CLI agent spawn. Vì vậy `probe` stdio **không** liệt kê tool (v1): trả `{tools: [], digest: <specDigest>}`, rug-pull của stdio dựa trên `spec_digest` (command+args+tên env). Probe tool của stdio cần sandbox runner — ngoài phạm vi.
3. **Reject = `disabled`.** `status` CONTRACT chỉ có 3 giá trị; `review{reject}` đặt `disabled` + `reviewed_by`. Bật lại = `review{approve}`.
4. **Fail-closed khi tool đổi**: server `approved` nhưng `last_probe_digest != approved_digest` (⇒ `toolsChanged=true`) **không** được cấp cho agent cho tới khi duyệt lại.
5. **Fail-open có kiểm soát ở spawn**: `mcp-service` lỗi/timeout ⇒ spawn agent không MCP + metric/log; riêng `MCP_KILL_SWITCH_ACTIVE` ⇒ không cấp gì.

## 2. Giải pháp

### A. Dữ liệu — migration `mcp-service` (số thứ tự chưa chốt; sau `0001` của BE-002)

```sql
-- backend-go/services/mcp-service/migrations/postgres/00NN_external_servers.up.sql
CREATE TABLE mcp.external_servers (
  id               UUID PRIMARY KEY,
  tenant_id        UUID NOT NULL,
  scope            TEXT NOT NULL CHECK (scope IN ('tenant','team','user')),
  scope_id         TEXT NOT NULL,                 -- tenant: tenant_id; team/user: id logic (không FK xuyên DB)
  name             TEXT NOT NULL CHECK (name ~ '^[a-z0-9][a-z0-9_-]{0,62}$'),
  transport        TEXT NOT NULL CHECK (transport IN ('http','stdio')),
  url              TEXT, command TEXT, args JSONB NOT NULL DEFAULT '[]',
  status           TEXT NOT NULL DEFAULT 'pending_review'
                   CHECK (status IN ('pending_review','approved','disabled')),
  spec_digest      TEXT NOT NULL,                 -- sha256(transport,url|command,args,tên env/header) — đổi => duyệt lại
  last_probe_digest TEXT, last_probe_at TIMESTAMPTZ,
  approved_digest  TEXT, approved_tools JSONB,    -- snapshot {name,description} để FE diff (text KHÔNG tin cậy)
  health_ok BOOLEAN, health_checked_at TIMESTAMPTZ, health_error TEXT,
  created_by TEXT NOT NULL, reviewed_by TEXT, reviewed_at TIMESTAMPTZ,
  version INT NOT NULL DEFAULT 1, created_at TIMESTAMPTZ NOT NULL, updated_at TIMESTAMPTZ NOT NULL,
  CONSTRAINT http_has_url   CHECK (transport <> 'http'  OR url IS NOT NULL),
  CONSTRAINT stdio_has_cmd  CHECK (transport <> 'stdio' OR command IS NOT NULL),
  UNIQUE (tenant_id, scope, scope_id, name)
);
-- Tham chiếu secret: CHỈ tên + con trỏ; KHÔNG có cột giá trị (arch/06).
CREATE TABLE mcp.external_server_secret_refs (
  server_id UUID NOT NULL REFERENCES mcp.external_servers(id) ON DELETE CASCADE,
  tenant_id UUID NOT NULL, kind TEXT NOT NULL CHECK (kind IN ('env','header')), name TEXT NOT NULL,
  broker_owner_id TEXT,                           -- NULL = đã khai tên nhưng chưa set (hasSecret=false)
  set_by TEXT, set_at TIMESTAMPTZ, PRIMARY KEY (server_id, kind, name)
);
CREATE TABLE mcp.external_server_tools_history (   -- lịch sử digest (rug-pull forensic)
  id UUID PRIMARY KEY, tenant_id UUID NOT NULL, server_id UUID NOT NULL REFERENCES mcp.external_servers(id) ON DELETE CASCADE,
  digest TEXT NOT NULL, tools JSONB NOT NULL, observed_at TIMESTAMPTZ NOT NULL,
  source TEXT NOT NULL CHECK (source IN ('probe','health')), decision TEXT CHECK (decision IN ('approved','rejected')), decided_by TEXT);
-- RLS: ALTER TABLE ... ENABLE ROW LEVEL SECURITY; POLICY USING (tenant_id = current_setting('app.tenant_id')::uuid) cho cả 3 bảng (arch/05).
```
`McpExternalServer` ánh xạ: `toolsDigest = approved_digest ?? last_probe_digest`; `toolsChanged = approved_digest IS NOT NULL AND last_probe_digest IS DISTINCT FROM approved_digest`; `envRefs/headerRefs.hasSecret = broker_owner_id IS NOT NULL`. `args/url` có thể chứa secret nhúng → validator từ chối `url` có userinfo/query chứa `token|key|secret` (`MCP_SERVER_INVALID`) và args khớp mẫu secret (reuse regex của `agent/src/shared/mcp-config.ts` `SENSITIVE_ENV_*` ở dạng port Go).

### B. Secret — chỉ qua `credential-broker-service`
- Proto broker (`proto/orca/credentialbroker/v1/credentialbroker.proto`): thêm `CREDENTIAL_CATEGORY_MCP_EXTERNAL_SECRET = 7;`; domain `CategoryMcpExternalSecret = "mcp_external_secret"` (`Engine()` → KV2); migration broker `0004` (cả `postgres/` và `mysql/`; **cần thiết** — CHECK hiện chỉ có 5 giá trị, đã xác minh) sửa CHECK thêm `'mcp_external_secret'`; allow-list `category→caller` mới: `mcp_external_secret → {mcp-service}` (thêm vào `requestingService` check; test deny cho caller khác).
- Quy ước `owner_id` = `mcp:<serverId>:<kind>:<name>` (đọc/ghi/xoá bằng `…ByOwner`, không cần lưu `credential_id`). `setSecret` ⇒ `WriteCredential{tenant_id, owner_id, category=7, encrypted_envelope=<plaintext bytes>}` (trường tên `encrypted_envelope` nhưng usecase coi là bytes mờ rồi Transit-encrypt; **chưa xác minh** có cần thêm cờ `payload_encoding` như broker TDD mô tả — kiểm khi cài đặt) ⇒ cập nhật `broker_owner_id`. `delete`/`rotate` ⇒ `RevokeCredentialByOwner`. `config_json` **để trống** (được trả nguyên văn bởi metadata read).
- `mcp-service` DB, log, event, response: không bao giờ có giá trị (plaintext chỉ nằm trong bộ nhớ suốt request rồi zero hoá). Chỉ đọc plaintext qua `ResolveCredentialByOwner` trong (i) probe/health có header xác thực, (ii) `ResolveAgentMcpConfig`; buffer `[]byte` zero hoá sau dùng; type `SecretValue` có `String()/LogValue()` trả `"[redacted]"`.
- **D1 (đã chốt, thay cho "phụ thuộc chặn" cũ):** secret MCP **không** dùng phong bì client (`encryptCredential`/`encryptedBlob`/`iv`). Luồng: UI ⇒ `mcp.externalServer.setSecret{serverId, kind, name, value}` (plaintext **một lần**, qua WebSocket/TLS đã xác thực) ⇒ api-gateway chuyển tiếp gRPC (mTLS) ⇒ `mcp-service` gọi broker `WriteCredential` với plaintext (category `mcp_external_secret`) ⇒ broker mã hoá bằng Vault Transit, lưu ciphertext. Plaintext không bao giờ được lưu hay log ở gateway/mcp-service. Khi spawn agent, `mcp-service` đọc qua `ResolveCredentialByOwner` và chỉ đặt vào env tiến trình con. Không còn trạng thái `secret_unresolvable` do thiếu giải mã (giữ cho trường hợp broker lỗi/ khoá bị thu hồi ⇒ fail-closed, bỏ qua server đó).
- **Che `value` bắt buộc (CONTRACT C11):** (i) log WS/request/response của `wscompat` không in `args` của kênh `mcp.externalServer.setSecret` (allow-list kênh nhạy cảm → ghi `args:\"[redacted]\"`); (ii) trace capture/trace store (SSE trace của gateway) không lưu args kênh này; (iii) `mcpChannelError` chỉ dùng `status.Convert(err).Message()` tĩnh, **không** nối args hay giá trị; (iv) span OTel không gắn args (cùng allow-list với BE-015); (v) response chỉ `{hasSecret:true}` — **không bao giờ echo `value`**; (vi) lỗi validate (vd rỗng/quá dài) chỉ nêu tên field. Test bắt buộc: `TestSetSecret_DoesNotLogValue` (bắt toàn bộ slog + trace store sau một lời gọi, khẳng định không chứa giá trị mẫu), `TestSetSecret_ErrorDoesNotEchoValue`, `TestSetSecret_ResponseHasNoValue`, `TestTraceStore_RedactsSetSecretArgs`. (chưa xác minh `Registry.Dispatch`/trace middleware hiện có log args hay không — phải đọc `registry.go`/trace routes trước khi code; nếu có thì sửa tại đó.)
- **Ngoài phạm vi / follow-up riêng:** `CredentialInput` của AI Provider (client envelope với khoá dự phòng `'fallback-dev-token'`) là điểm yếu **có sẵn**, không bị đụng ở đây. FE không được tuyên bố mã hoá đầu cuối; copy: "Sent over TLS and encrypted at rest by the server".

### C. Prober an toàn SSRF — `internal/adapter/mcpprober/` + `internal/domain/ssrf_policy.go`
```go
// domain: thuần stdlib. Chặn theo LOẠI địa chỉ, không theo tên metadata cụ thể.
func IsBlockedIP(ip netip.Addr) bool {
    ip = ip.Unmap()                     // ::ffff:127.0.0.1 -> 127.0.0.1 (tránh lách)
    return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
        ip.IsMulticast() || ip.IsUnspecified() || cgnat.Contains(ip) /*100.64/10 gồm 100.100.100.200*/ ||
        reserved.Contains(ip) /*0/8,192.0.0.0/24,198.18/15,240/4,64:ff9b::/96,2002::/16 (unwrap v4 nhúng)*/
}
```
`169.254.169.254` bị chặn bởi `IsLinkLocalUnicast`; `fd00:ec2::254` bởi `IsPrivate`. Quy tắc URL (`ValidateExternalURL`): scheme `https` (ngoại lệ chỉ host:port nằm trong `MCP_EXTERNAL_HTTP_ALLOWLIST`, mặc định rỗng, dùng cho dev); cấm userinfo, cấm IP-literal, port ∈ `MCP_EXTERNAL_ALLOWED_PORTS` (mặc định `443`); lỗi ⇒ `MCP_SERVER_SSRF_BLOCKED`.
Adapter: `http.Transport{Proxy: nil, DialContext: (&net.Dialer{Timeout: 5s, Control: func(_, addr string, _ syscall.RawConn) error { /* parse addr thật đã resolve → IsBlockedIP ⇒ lỗi */ }}).DialContext, TLSClientConfig{MinVersion: TLS12}}`; **không follow redirect** (`CheckRedirect` trả lỗi); kiểm tại **thời điểm kết nối** nên rebinding giữa lần resolve và dial vô hiệu (khác `webhook.go`). Giới hạn: tổng 15s (`context.WithTimeout`, lý do ghi ở code), body ≤ 1 MiB/response (`io.LimitReader`), ≤ 200 tool, ≤ 20 trang `tools/list`, mô tả lưu ≤ 4 KiB (digest tính trên bản đầy đủ). Header gửi đi **chỉ** từ `headerRefs` đã khai; cấm `Host`, `Cookie`, `Proxy-*`, `Content-Length`; **không bao giờ** lấy `Authorization` từ identity/ctx của caller (cấm token passthrough — CR-005; test `TestProber_NeverForwardsCallerToken`).
Giao thức: `initialize` (`protocolVersion` lấy từ `McpServerInfo.protocolVersions`) → `notifications/initialized` → `tools/list` (xử lý JSON hoặc `text/event-stream` có giới hạn), huỷ session (`DELETE`) cuối. Kết quả → `ToolsDigest`.

### D. Digest ghim mô tả + phát hiện rug-pull — `internal/domain/tools_digest.go`
`digest = hex(sha256(canonicalJSON(sort_by_name([{name, description, inputSchema}]))))` (JSON khoá sắp xếp, UTF-8 NFC). `Probe` lưu `last_probe_digest` + 1 dòng `tools_history`. `Review{approve, toolsDigest}`: chỉ chấp nhận nếu `toolsDigest == last_probe_digest` hiện lưu (khác ⇒ `MCP_SERVER_DIGEST_MISMATCH`, buộc probe lại) ⇒ ghi `approved_digest/approved_tools/reviewed_by`, `status='approved'`. Health worker (E) phát hiện đổi ⇒ `toolsChanged` + event; **không** tự duyệt lại. Văn bản tool là dữ liệu không tin cậy: không bao giờ đưa vào prompt/log ở mức INFO; chỉ lưu và trả cho UI.

### E. Health check & vòng đời — `internal/usecase/check_external_server_health.go`
Worker chạy mỗi `MCP_EXTERNAL_HEALTH_INTERVAL` (mặc định 5m, leader-lock bằng advisory lock để nhiều replica không probe trùng) cho server `http` `approved`: probe → cập nhật `health_*`, `last_probe_digest`; phát event khi `ok` đổi trạng thái hoặc digest đổi. `upsert` đổi `spec_digest` ⇒ `status` về `pending_review` (giữ `approved_digest` làm lịch sử). `review{reject}` trên server đang chạy ⇒ `disabled` + event; tiến trình đang chạy không bị thu hồi (chỉ lần spawn kế) — ghi nhận giới hạn, thông báo cho infra-fleet là future.

### F. `ResolveAgentMcpConfig` — `internal/usecase/resolve_agent_mcp_config.go`
```proto
// backend-go/proto/orca/mcp/v1/external_server.proto  (cùng package orca.mcp.v1; tenant lấy từ gRPC metadata, KHÔNG có trong body)
service McpRegistryService {
  rpc ListExternalServers(ListExternalServersRequest) returns (ListExternalServersResponse);
  rpc UpsertExternalServer(UpsertExternalServerRequest) returns (UpsertExternalServerResponse);
  rpc SetExternalServerSecret(SetExternalServerSecretRequest) returns (SetExternalServerSecretResponse);
  rpc ProbeExternalServer(ProbeExternalServerRequest) returns (ProbeExternalServerResponse);
  rpc ReviewExternalServer(ReviewExternalServerRequest) returns (ReviewExternalServerResponse);
  rpc DeleteExternalServer(DeleteExternalServerRequest) returns (DeleteExternalServerResponse);
  rpc ResolveAgentMcpConfig(ResolveAgentMcpConfigRequest) returns (ResolveAgentMcpConfigResponse);
}
message ResolveAgentMcpConfigRequest {
  string user_id = 1; string project_id = 2;
  string agent_kind = 3;            // "claude"|"codex"|"gemini"|"opencode" (khớp AGENT_SPECS)
  string host_os = 4;               // "posix"|"windows" — quyết định launcher/đường dẫn
  string host_kind = 5;             // "local"|"relay"|"ssh"
  int32  parent_mcp_depth = 6;      // 0 nếu user bấm spawn từ UI; >0 nếu spawn đến từ tool MCP
  string parent_mcp_session_id = 7;
}
message ResolveAgentMcpConfigResponse {
  repeated ConfigFile files = 1;      // nội dung KHÔNG chứa secret (chỉ placeholder ${ORCA_MCP_*})
  repeated string extra_args = 2;
  repeated EnvVar env = 3;            // GỒM secret: chỉ cho env tiến trình con; không log; có LogValue redacted
  repeated Warning warnings = 4;      // {server_name, reason}
  int32 mcp_depth = 5;                // = parent+1 (đã nhúng vào token)
}
```
Các bước: (1) `RequireTenantID`; kill switch ⇒ trả rỗng + `MCP_KILL_SWITCH_ACTIVE`. (2) `TenantService.GetResolvedProfile(user_id)` ⇒ danh sách tên `mcp.servers[].name` (bỏ mọi field khác); `ListTeamsForUser` ⇒ team ids. (3) Với mỗi tên: tìm theo ưu tiên `user > team > tenant`; chỉ lấy `status='approved' AND NOT toolsChanged AND health không bị chặn SSRF`; lý do bỏ qua ⇒ `Warning{not_in_registry|pending_review|disabled|tools_changed|secret_unresolvable|ssrf_blocked|stdio_unsupported}`. Re-validate SSRF bằng resolve tại thời điểm này (giảm cửa sổ rebinding; **rủi ro dư R3**). (4) `parent_mcp_depth+1 > MCP_MAX_AGENT_DEPTH` (mặc định 2) ⇒ bỏ server `orca`, vẫn cấp server ngoài; else gọi port `AgentTokenIssuer.Issue` (BE-006: `aud=resourceUrl`, `scope=MCP_AGENT_TOKEN_SCOPES` mặc định `orca:read`, `ttl=MCP_AGENT_TOKEN_TTL` mặc định 4h, claim `mcp_depth`, `root_session`, `jti`; tên RPC **chưa chốt** ở BE-006). Token không bao giờ bị lộ ra server ngoài. (5) Resolve secret qua broker (`…ByOwner`), render theo agent: `internal/adapter/agentconfig/{claude,codex,gemini,opencode}.go` — secret luôn là `${ORCA_MCP_<n>}` trong file, giá trị nằm ở `env`.

| agent_kind | Phát hành | Ghi chú (**chưa xác minh với phiên bản CLI đang ship — cần spike + golden test**) |
|---|---|---|
| `claude` | file JSON `{"mcpServers":{…}}` + `--mcp-config <path>` | Claude hỗ trợ mở rộng `${VAR}` trong `.mcp.json` |
| `codex` | `-c mcp_servers.<n>.url=…` / `bearer_token_env_var=ORCA_MCP_TOKEN` (không file) | cần xác nhận cờ `-c` và khoá TOML theo phiên bản |
| `gemini` | file settings + env `GEMINI_CLI_SYSTEM_SETTINGS_PATH` | cần xác nhận biến và cú pháp mở rộng env |
| `opencode` | env `OPENCODE_CONFIG_CONTENT` (JSON inline, placeholder `{env:VAR}`) | cần xác nhận |
| khác (`ollama`, 25+ `TuiAgent` ở `tui-agent-config.ts`) | không cấp; `Warning{agent_unsupported}` | mở rộng sau |

Không bao giờ ghi file vào worktree/cwd (tránh bị commit kèm token); agent ghi vào thư mục tạm riêng của task (D2). stdio: chỉ phát hành nếu `host` có launcher (D3) hoặc `MCP_EXTERNAL_STDIO_ALLOW_UNSANDBOXED=true`; command được bọc `orca-mcp-launch -- <cmd> <args…>` (Windows: resolve `npx.cmd` phía agent).

### G. Hook vào spawn (không sửa `agent/` ở solution này)
`infra-fleet-service/internal/usecase/start_agent_session.go` — trước `uc.agent.SpawnAgent`, gọi port mới `AgentMcpConfigResolver` (adapter `grpcclient` → `McpRegistryService.ResolveAgentMcpConfig`, timeout 3s, lỗi ⇒ fail-open + log, trừ kill switch). `SpawnAgentInput` thêm `McpConfig *AgentMcpConfig`; `devserveragent/agent_methods.go` thêm param `mcpConfig` **chỉ khi** `devServer.AgentVersion` ≥ phiên bản agent hỗ trợ (chưa xác định; agent cũ phải không nhận được trường lạ — chưa xác minh agent có bỏ qua param lạ không). `SpawnAgentInput.McpConfig` hiện thực `slog.LogValuer` che `env`. Impact trước khi sửa (**chưa chạy**): `gitnexus impact({target:"StartAgentSession", direction:"upstream"})`, `impact({target:"SpawnAgent"})` — rủi ro dự kiến **MEDIUM** (đường spawn agent trọng yếu; `ResumeAgentSession` cũng đi qua). SSH/relay: cấu hình sinh ở backend, đi cùng `agent.spawn` qua kênh sẵn có (WS/SSH relay) tới host đích; agent ghi file `0600` (Windows: ACL chủ sở hữu) vào thư mục tạm riêng và xoá khi PTY thoát.
- **D2 (agent/, CR riêng):** `AgentSpawnRequest` nhận `mcpConfig{files[],extraArgs[],env{}}`; `agent-spawn-env.ts` hợp nhất `env` vào child (không echo vào log/trace), ghi/xoá file, nối `extraArgs` vào `buildAgentArgs`. **D3:** shim `orca-mcp-launch` (scrub env: chỉ giữ biến khai báo + PATH tối thiểu; tuỳ chọn sandbox bubblewrap/sandbox-exec; Windows job object).

### H. Kênh WS — `api-gateway/internal/adapter/wscompat/channels_mcp_external_server.go` (file riêng, tránh xung đột với `channels_mcp.go` của các BE khác; đăng ký bằng `registerMcpExternalServerChannels`)
| Kênh | Quyền | Hành vi chính | Lỗi |
|---|---|---|---|
| `mcp.externalServer.list{scope?}` | login | admin: mọi server của tenant (lọc `scope`); user: chỉ `scope:'user' && scopeId==Identity.UserID` | `MCP_DISABLED` |
| `.upsert` | `scope:'user'`: chủ sở hữu; `tenant/team`: admin | `scopeId` user **do server gán**; bỏ qua `status/toolsDigest/hasSecret` từ client; đổi spec ⇒ `pending_review`; stdio cần cờ bật | `MCP_NOT_ADMIN`, `MCP_SERVER_SSRF_BLOCKED`, `MCP_NOT_FOUND` (+ mã đề xuất `MCP_SERVER_INVALID`, `MCP_SERVER_STDIO_NOT_ALLOWED`, `MCP_SERVER_NAME_CONFLICT`) |
| `.setSecret{serverId,kind,name,value}` | như `upsert` | `value` = plaintext một lần qua WS/TLS (D1; không envelope, không `iv`); **che `value` ở log/trace, không echo** (C11); trả `{hasSecret:true}`; tên phải đã khai trong `envRefs/headerRefs` | `MCP_NOT_FOUND`, `MCP_INVALID_ARGUMENT` |
| `.probe{serverId}` | **admin** | http: probe thật; stdio: spec digest; ghi history | `MCP_SERVER_SSRF_BLOCKED`, `MCP_NOT_FOUND` |
| `.review{serverId,{decision,toolsDigest}}` | **admin**; stdio luôn cần bước này kể cả do admin tạo (tuỳ chọn `MCP_STDIO_FOUR_EYES=true`: `reviewer != created_by`) | xem 2.D | `MCP_SERVER_DIGEST_MISMATCH` (đề xuất), `MCP_NOT_FOUND` |
| `.delete{serverId}` | như `upsert` | xoá + `RevokeCredentialByOwner` mọi ref (best-effort + outbox retry) | `MCP_NOT_FOUND` |
Cross-tenant/không quyền ⇒ luôn `MCP_NOT_FOUND`. Quyền kiểm kép: gateway (`Identity.Role` từ cookie) **và** `mcp-service` OPA in-process (`package orca.authz.mcp_external_server`, `default allow := false`, `mcp_external_server_test.rego`: admin full; user chỉ `scope=="user" && scope_id==input.user_id`; `review/probe` chỉ admin).

### I. Sự kiện & audit (outbox cùng transaction; envelope chuẩn arch/05, `schema_version:1`)
`orca.mcp.externalserver.created|updated|deleted|reviewed|secret_set|tools_changed|health_changed`, `orca.mcp.agentconfig.resolved` (id server, số lượng, depth — **không** token/secret). Tất cả trở thành dòng `audit_log` (auth-service) qua `auditclient`; tên secret có thể ghi, giá trị thì không. Metric `orca_mcp_external_probe_total{result}`, `orca_mcp_external_ssrf_blocked_total`, `orca_mcp_agent_config_resolved_total{outcome}`.

## Hợp đồng với frontend
Hiện thực đúng 6 kênh `mcp.externalServer.*` của CONTRACT §2.2 (kiểu `McpExternalServer`, lỗi `MCP_*`). `list` của user thường chỉ trả server của chính họ; nội dung `tools[].description` trả nguyên văn (UI phải coi là untrusted). **Trạng thái đề xuất đổi CONTRACT** (CR-1 đã thay bằng D1; CR-2..CR-5 đã nằm trong CONTRACT §1/§2.2/§2.3):
- **CR-1** *(đã thay bằng D1; CONTRACT chốt `setSecret{serverId,kind,name,value}` plaintext, không `iv`/`encryptedBlob`, kèm C11)*.
- **CR-2** `probe` kết quả thêm `approvedTools?: [{name,description}]` và `transport?` (additive, C9) để FE diff "kể từ lần duyệt".
- **CR-3** thêm mã `MCP_SERVER_INVALID`, `MCP_SERVER_STDIO_NOT_ALLOWED`, `MCP_SERVER_DIGEST_MISMATCH`, `MCP_SERVER_NAME_CONFLICT` vào §2.3.
- **CR-4** (tuỳ chọn) cho chủ sở hữu được `probe` server `scope:'user'` của mình; hiện CONTRACT chỉ admin ⇒ user không bao giờ xem được tool của server mình thêm.
- **CR-5** (tuỳ chọn) `McpExternalServer.updatedAt?` để hiển thị.

## Sửa TDD kèm theo
T7 (`00-service-catalog`, `arch/02`: registry ngoài thuộc `mcp-service`); **T9 đề xuất mới** cho README v5 §3: `arch/06` thêm hàng "MCP external server secrets → KV v2 qua broker, category `mcp_external_secret`, plaintext chỉ tới `mcp-service` rồi tới env tiến trình agent"; `credential-broker-service.md` §3 enum + bảng "Per category" + §9; `arch/07` (SSRF egress policy, token passthrough ban); `infra-fleet-service.md` (hook `AgentMcpConfigResolver`); `tenant-service.md` §`mcp.servers` (ghi: entry profile chỉ là tham chiếu tên, inline command/env bị bỏ qua khi cấp agent); T8 (metric `orca_mcp_*`, env `MCP_*`).

## Kiểm thử
- Unit (`go test ./services/mcp-service/internal/domain/...`): `TestIsBlockedIP` (127.0.0.1, ::1, ::ffff:127.0.0.1, 169.254.169.254, 100.100.100.200, 10/8, fd00:ec2::254, 0.0.0.0, NAT64 nhúng 127.0.0.1); `TestValidateExternalURL` (http, userinfo, IP-literal, port); `TestToolsDigest_StableUnderReorder/ChangesOnDescription`; `TestStdioPolicy` (bare command, pin `pkg@x.y.z`, từ chối `-c`/`--eval`).
- Usecase (fake ports): `TestReview_DigestMismatch`, `TestUpsert_SpecChangeResetsToPending`, `TestUserScope_ForcesOwnScopeId`, `TestResolveAgentMcpConfig_{SkipsPending,SkipsToolsChanged,IgnoresInlineProfileCommand,DepthExceededOmitsOrca,UserBeatsTeamBeatsTenant,SecretOnlyInEnv}`.
- Adapter: `mcpprober` với `httptest` TLS server + resolver giả trả IP chặn ⇒ lỗi tại `Control`; test rebinding (resolver trả IP public lần 1, private lần 2); redirect 302 bị từ chối; body > 1 MiB bị cắt; `TestProber_NeverForwardsCallerToken`.
- Integration `//go:build integration` (`go test -tags integration ./services/mcp-service/...`): migration up→down→up, RLS cross-tenant ⇒ not-found, không cột nào chứa giá trị secret (`information_schema` kiểm tên cột).
- Broker: `go test ./services/credential-broker-service/...` (allow-list category, CHECK mới). Gateway: `go test ./services/api-gateway/internal/adapter/wscompat -run ExternalServer` (không log value, `MCP_NOT_ADMIN`). OPA: `opa test policy/`. Contract: `buf lint && buf breaking --against .git#branch=main` trong `backend-go/proto`. `profile_resolution_test.go` giữ xanh: `go test ./services/tenant-service/internal/domain/...`.
- E2E compose: spawn agent giả với profile `mcp.servers:[{name:"x"}]` — `pending_review` ⇒ không có trong config; sau `review` ⇒ có; đổi mô tả tool ở server giả ⇒ health ⇒ bị loại.

## Rủi ro & phụ thuộc
| # | Rủi ro | Giảm thiểu |
|---|---|---|
| R1 | Plaintext đi qua gateway/mcp-service trong thời gian request (D1 đã chốt) | Chỉ trong bộ nhớ; che `value` ở log/trace/lỗi (C11) + test; TLS/mTLS; zero hoá buffer; broker Transit-encrypt at-rest; điểm yếu `'fallback-dev-token'` của AI-provider là follow-up riêng |
| R2 | stdio = thực thi mã | Mặc định tắt (`MCP_EXTERNAL_STDIO_ENABLED=false`), admin review, allow-list+pin, launcher D3 |
| R3 | Rebinding ở **phía host agent** (agent tự resolve, cert hợp lệ cho domain kẻ tấn công) | Re-resolve lúc spawn + health 5m; khuyến nghị egress policy trên dev server; rủi ro dư được ghi nhận |
| R4 | Đổi `StartAgentSession` ảnh hưởng đường spawn | Fail-open, cờ `MCP_AGENT_CONFIG_ENABLED`, impact trước khi sửa |
| R5 | Cú pháp cấu hình CLI thay đổi theo phiên bản | Golden test + spike; mỗi renderer có cờ tắt riêng |
| Dep | BE-002 (scaffold), BE-006 (mint token), BE-013 (kill switch/audit), D2/D3 phía `agent/` | — |

## Không thuộc phạm vi
Sửa `agent/` (D2/D3); probe tool của stdio; proxy hoá server ngoài qua gateway; OAuth cho server ngoài (chỉ header/env tĩnh); thu hồi tiến trình đang chạy; UI (FE-MCP-SOL-011); cấp cho ~30 `TuiAgent` ngoài 4 CLI ở `AGENT_SPECS`; hiển thị lớp profile của server (cần `Sources` ra proto tenant).

## Liên quan
`backend-go/services/tenant-service/internal/domain/profile_resolution.go:231-282` · `.../usecase/get_resolved_profile.go` · `backend-go/proto/orca/credentialbroker/v1/credentialbroker.proto` · `backend-go/services/credential-broker-service/internal/usecase/{write_credential,resolve_credential_by_owner}.go`, `migrations/postgres/0001_init.up.sql` · `backend-go/services/workflow-service/internal/adapter/stepexecutors/webhook.go` · `backend-go/services/infra-fleet-service/internal/usecase/start_agent_session.go`, `internal/adapter/devserveragent/agent_methods.go` · `backend-go/services/api-gateway/internal/adapter/wscompat/registry.go` · `agent/src/relay/{agent-binary-specs,agent-spawn-env,agent-spawn-types}.ts`, `agent/src/shared/mcp-config.ts`
