# CONTRACT — Hợp đồng API giữa backend-go và frontend cho MCP (v5)

> **Nguồn sự thật duy nhất** cho mọi thứ frontend gọi/nhận từ backend-go trong feature MCP.
> Mọi `BE-MCP-SOL-*` (specs/backend-go/crs/v5) **phải hiện thực** đúng hợp đồng này; mọi `FE-MCP-SOL-*`
> (specs/frontend/crs/v5) **chỉ được dùng** những gì có ở đây. Cần đổi ⇒ sửa file này trước, rồi cập nhật cả hai phía.
> CR gốc: [docs/crs/v5](../../../../docs/crs/v5/README.md).

## 0. Nguyên tắc tương thích

| # | Nguyên tắc | Lý do (bằng chứng) |
|---|-----------|--------------------|
| C1 | UI quản lý MCP đi qua **kênh WS RPC `mcp.*`** đăng ký trong `wscompat.Registry` (file mới `channels_mcp.go`), **không** thêm REST cho UI | UI hiện gọi qua `callRuntimeResult(method, params)` → `Registry.Dispatch`; admin UI dùng `admin.*` theo cách này (frontend `web-preload-api.ts` `createAdminApi`) |
| C2 | Danh tính lấy từ session cookie (`Identity{TenantID,UserID,Role}`), **không bao giờ** từ tham số | `registry.go` Identity; quy ước "never trust identity fields from the body" |
| C3 | Kênh `mcp.admin.*` yêu cầu `Identity.Role == "admin"`; sai ⇒ lỗi `MCP_NOT_ADMIN` | `requireAdmin` hiện có (`INFRA_NOT_ADMIN`); `Role` chỉ có ở nhánh cookie ⇒ UI luôn có Role |
| C4 | Lỗi trả dạng message `"<CODE>: <thông điệp ngắn>"` (CODE = `MCP_*` UPPER_SNAKE). Frontend tách theo `^([A-Z0-9_]+): `. **Backend phải bọc** mọi lỗi gRPC của kênh `mcp.*` bằng `mcpChannelError` (lấy `status.Convert(err).Message()`), vì `Registry.Dispatch` trả lỗi gRPC thô và WS gửi `err.Error()` nguyên văn (`rpc error: code = … desc = CODE: msg`). Parser FE chấp nhận cả hai dạng để phòng thủ | `session_dialect.go:94` gửi `err.Error()`; `ChannelHandler` lỗi → `ErrorMessage.Message`; FE `callRuntimeResult` throw `Error(message)` |
| C5 | Kênh push dùng `RegisterStream`/`StreamChannelHandler` (`mcp.events.subscribe`) | `registry.go` `StreamHandler`/`push_bridge.go`; FE dùng `client.subscribe(...)` + hàm teardown (mẫu `nativeChat.subscribe`) |
| C6 | **Không** trả secret/token/giá trị env ở bất kỳ kênh nào, trừ `secret` của PAT **một lần duy nhất** khi tạo | CR-006, CR-014 |
| C7 | Thời gian là RFC 3339 UTC (`string`); id là UUID/`string`; phân trang bằng `cursor` opaque + `limit` (mặc định 50, tối đa 200) | quy ước proto hiện có (`cursor`,`limit`) |
| C8 | Tất cả kênh `mcp.*` ẩn hoàn toàn khi `MCP_ENABLED=false`: trả lỗi `MCP_DISABLED`; `mcp.server.info` vẫn trả `{enabled:false}` | CR-002 |
| C10 | **Mỗi kênh `mcp.*` nhận đúng MỘT object JSON ở `args[0]`** (không tham số vị trí thứ hai). Ví dụ `mcp.session.close` ⇒ `{sessionId}`; `mcp.consent.decide` ⇒ `{requestId, decision, scopes}`; `mcp.approval.decide` ⇒ `{approvalId, decision, paramsHash, note?}`. Các bảng §2 liệt kê field của object đó | web client chỉ chuyển `args[0]` (`wscompat/session_dialect.go`); khớp `admin.deletePolicy` |
| C11 | **Kênh có secret trong tham số** (hiện: `mcp.externalServer.setSecret`) phải được **che `value`** ở mọi nơi ghi nhận request/response: log WS, middleware, trace/span, trace store, thông điệp lỗi (`mcpChannelError` và trace store **không** echo args). Response không bao giờ chứa lại giá trị | D1; BE-MCP-SOL-014 §B/§H |
| C9 | Thêm field mới vào kiểu dưới đây chỉ được **additive** (client cũ bỏ qua field lạ) | tương thích tiến hoá |

## 1. Kiểu dùng chung (TypeScript — FE đặt trong `frontend/src/shared/mcp-types.ts`; BE sinh JSON tương ứng bằng `protojson`/struct tag `json:"camelCase"`)

```ts
type McpRisk = 'read' | 'write_reversible' | 'exec' | 'destructive' | 'admin'
type McpScopeId = 'orca:read' | 'orca:write' | 'orca:exec' | 'orca:admin'   // mở rộng additive
type McpDecision = 'allow' | 'require_approval' | 'deny'

interface McpScopeDescriptor { id: McpScopeId | string; label: string; description: string; risk: McpRisk }
// label/description tiếng Anh từ server; FE ưu tiên khoá i18n `auto.mcp.scope.<id>` rồi rơi về chuỗi server.

interface McpServerInfo {
  enabled: boolean                    // = cờ tiến trình MCP_ENABLED (BE-002). KHÔNG phụ thuộc cài đặt tenant
  tenantEnabled?: boolean             // cài đặt của tenant; tenant mới mặc định theo env MCP_TENANT_DEFAULT_ENABLED của mcp-service (mặc định true, D6). FE hiện UI khi enabled=true && tenantEnabled!==false; hiện công tắc cho admin khi tenantEnabled=false
  resourceUrl: string                 // vd https://orca.example.com/mcp
  protocolVersions: string[]          // vd ['2025-06-18']
  authorizationServer: string         // issuer URL
  scopesSupported: McpScopeDescriptor[]
  dcrEnabled: boolean
  maxTokenDays: number                // trần hạn PAT do tenant đặt
  killSwitch: { active: boolean; reason?: string; at?: string }
}

interface McpGrant {            // consent của (user, client)
  id: string; clientId: string; clientName: string; clientUri?: string
  scopes: McpScopeId[]; createdAt: string; lastUsedAt?: string
  status: 'active' | 'revoked'
  userId?: string; userName?: string  // chỉ có ở kênh admin
}

interface McpSessionView {
  id: string; clientName: string; grantId?: string; tokenId?: string
  createdAt: string; lastSeenAt: string; protocolVersion: string
  activeStreams: number; toolCalls: number
  userId?: string; userName?: string  // chỉ ở kênh admin
}

interface McpConsentRequest {
  requestId: string; clientId: string; clientName: string; clientUri?: string
  redirectHost: string               // chỉ host để hiển thị, không cả URL
  scopes: McpScopeDescriptor[]       // scope client XIN
  alreadyGranted: McpScopeId[]       // scope user đã cấp trước đó cho client này
  tenant: { id: string; name: string }
  isNewClient: boolean; registeredViaDcr: boolean
  expiresAt: string
}

interface McpToken {              // PAT; KHÔNG chứa secret
  id: string; name: string; scopes: McpScopeId[]
  createdAt: string; expiresAt: string; lastUsedAt?: string
  status: 'active' | 'revoked' | 'expired'
  userId?: string; userName?: string  // chỉ ở kênh admin
}

interface McpApproval {
  id: string; createdAt: string; expiresAt: string
  status: 'pending' | 'approved' | 'denied' | 'expired' | 'cancelled'
  tool: { name: string; title: string; risk: McpRisk }
  clientName: string; sessionId: string
  argsPreview: { text: string; redacted: boolean }   // nguyên văn lệnh/đường dẫn, đã che secret
  paramsHash: string                                  // FE phải gửi lại đúng giá trị khi quyết định
  decidedAt?: string; decidedVia?: 'web' | 'mobile' | 'elicitation'
  reasons?: string[]                                  // vì sao cần duyệt (vd 'exec', 'open_world_after_untrusted_read')
}

interface McpToolView {
  name: string; channel: string; title: string; description: string
  namespace: string; risk: McpRisk; requiredScope: McpScopeId
  pack: 1 | 2 | 3 | 4                // đợt CR-008
  hardDenied: boolean                // deny-list cứng, không policy nào mở được
  effective: McpDecision             // quyết định hiệu lực mặc định cho tenant hiện tại
  effectiveSource: 'default' | 'tenant_policy' | 'hard_deny' | 'kill_switch'
  annotations: { readOnly: boolean; destructive: boolean; idempotent: boolean; openWorld: boolean }
}

interface McpToolPolicy {
  id: string; version: number; updatedAt: string; updatedBy: string
  match: { tool?: string; namespace?: string; risk?: McpRisk; clientId?: string; roles?: Array<'admin' | 'user'> }
  decision: McpDecision            // 'allow' bị từ chối bởi server nếu match chạm hard-deny
  note?: string
}

interface McpAuditEntry {
  id: string; at: string; actorType: 'agent'
  userId: string; userName?: string; clientName: string; sessionId: string
  tool: string; risk: McpRisk
  decision: 'allow' | 'deny' | 'approved' | 'denied' | 'expired'
  argsSummary: string                 // đã che secret
  result: 'ok' | 'error' | null; durationMs?: number; approver?: string; traceId?: string
}

interface McpOAuthClient {
  clientId: string; name: string; redirectUris: string[]
  registeredVia: 'dcr' | 'admin'; status: 'allowed' | 'blocked' | 'pending'
  createdAt: string; lastUsedAt?: string; activeGrants: number
}

interface McpPrompt {
  id: string; name: string; description: string; version: number; updatedAt: string
  arguments: Array<{ name: string; description: string; required: boolean }>
  template: string; builtin: boolean   // builtin: chỉ đọc
}

interface McpExternalServer {
  id: string; scope: 'tenant' | 'team' | 'user'; scopeId?: string; name: string
  transport: 'http' | 'stdio'; url?: string; command?: string; args?: string[]
  envRefs: Array<{ name: string; hasSecret: boolean }>      // KHÔNG có giá trị
  headerRefs: Array<{ name: string; hasSecret: boolean }>
  status: 'pending_review' | 'approved' | 'disabled'
  toolsDigest?: string; toolsChanged: boolean
  health?: { ok: boolean; checkedAt: string; error?: string }
  createdBy: string; reviewedBy?: string; updatedAt?: string
}

type McpEvent =                                    // phần tử của luồng mcp.events.subscribe
  | { type: 'approval.requested'; approval: McpApproval }
  | { type: 'approval.resolved'; id: string; status: McpApproval['status'] }
  | { type: 'grant.revoked'; grantId: string }
  | { type: 'session.closed'; sessionId: string }
  | { type: 'killswitch.changed'; active: boolean; reason?: string }
```

## 2. Kênh WS RPC (đăng ký ở `api-gateway/internal/adapter/wscompat/channels_mcp.go`)

Cột "BE"/"FE" = solution sở hữu (BE hiện thực; FE tiêu thụ).

### 2.1 Mọi người dùng đã đăng nhập

| Kênh | Tham số | Kết quả | BE | FE |
|------|---------|---------|----|----|
| `mcp.server.info` | — | `McpServerInfo` | BE-MCP-SOL-003 | FE-MCP-SOL-001/002 |
| `mcp.session.list` | — | `McpSessionView[]` (của chính user) | BE-MCP-SOL-004 | FE-MCP-SOL-002 |
| `mcp.session.close` | `{sessionId}` | `{ok:true}` | BE-MCP-SOL-004 | FE-MCP-SOL-002 |
| `mcp.consent.get` | `{requestId}` | `McpConsentRequest` | BE-MCP-SOL-005 | FE-MCP-SOL-003 |
| `mcp.consent.decide` | `{requestId, decision:'approve'\|'deny', scopes: McpScopeId[]}` | `{redirectUrl: string}` | BE-MCP-SOL-005 | FE-MCP-SOL-003 |
| `mcp.grant.list` | — | `McpGrant[]` (của chính user) | BE-MCP-SOL-005 | FE-MCP-SOL-003 |
| `mcp.grant.revoke` | `{grantId}` | `{ok:true}` | BE-MCP-SOL-005 | FE-MCP-SOL-003 |
| `mcp.token.list` | — | `McpToken[]` | BE-MCP-SOL-006 | FE-MCP-SOL-004 |
| `mcp.token.create` | `{name, scopes: McpScopeId[], expiresInDays}` | `{token: McpToken, secret: string}` — **secret chỉ trả lần này** | BE-MCP-SOL-006 | FE-MCP-SOL-004 |
| `mcp.token.revoke` | `{tokenId}` | `{ok:true}` | BE-MCP-SOL-006 | FE-MCP-SOL-004 |
| `mcp.approval.list` | `{status?:'pending'\|'all', cursor?, limit?}` | `{approvals: McpApproval[], nextCursor?: string}` | BE-MCP-SOL-013 | FE-MCP-SOL-009 |
| `mcp.approval.decide` | `{approvalId, decision:'approve'\|'deny', paramsHash, note?}` | `McpApproval` | BE-MCP-SOL-013 | FE-MCP-SOL-009 |
| `mcp.events.subscribe` | — (stream, `RegisterStream`) | luồng `McpEvent` (push key `mcp.event`) | BE-MCP-SOL-013 | FE-MCP-SOL-001/009 |

Quy tắc: `mcp.approval.decide` chỉ chấp nhận khi `Identity.UserID` == chủ của approval **và** `paramsHash` khớp; sai ⇒ `MCP_APPROVAL_HASH_MISMATCH`; hết hạn ⇒ `MCP_APPROVAL_EXPIRED`; đã quyết ⇒ `MCP_APPROVAL_ALREADY_DECIDED`. `mcp.token.create` vượt `maxTokenDays` ⇒ `MCP_TOKEN_TOO_LONG`; scope vượt quyền user ⇒ `MCP_SCOPE_NOT_ALLOWED`.

### 2.2 Chỉ admin tenant (`Identity.Role == "admin"`)

| Kênh | Tham số | Kết quả | BE | FE |
|------|---------|---------|----|----|
| `mcp.admin.settings.get` | — | `{enabled, dcrEnabled, maxTokenDays, approvalTtlSeconds, killSwitch}` | BE-MCP-SOL-012 | FE-MCP-SOL-008 |
| `mcp.admin.settings.set` | patch một phần các field trên (trừ `killSwitch`) | settings mới | BE-MCP-SOL-012 | FE-MCP-SOL-008 |
| `mcp.admin.killswitch.set` | `{scope:'tenant'\|'client'\|'grant'\|'session', targetId?, active, reason}` | `{ok:true}` | BE-MCP-SOL-013 | FE-MCP-SOL-008 |
| `mcp.admin.killswitch.list` | — | `Array<{scope, targetId?, reason, at, by}>` (các kill switch đang bật) | BE-MCP-SOL-013 | FE-MCP-SOL-008 |
| `mcp.admin.tool.list` | `{namespace?, risk?}` | `McpToolView[]` | BE-MCP-SOL-007/008 | FE-MCP-SOL-005 |
| `mcp.admin.policy.list` | — | `McpToolPolicy[]` | BE-MCP-SOL-012 | FE-MCP-SOL-008 |
| `mcp.admin.policy.upsert` | `McpToolPolicy` (không `id` ⇒ tạo mới; `version` để chống ghi đè) | `McpToolPolicy` | BE-MCP-SOL-012 | FE-MCP-SOL-008 |
| `mcp.admin.policy.delete` | `{policyId}` | `{ok:true}` | BE-MCP-SOL-012 | FE-MCP-SOL-008 |
| `mcp.admin.policy.explain` | `{tool, userId?, clientId?}` | `{decision: McpDecision, reasons: string[]}` | BE-MCP-SOL-012 | FE-MCP-SOL-008 |
| `mcp.admin.client.list` | — | `McpOAuthClient[]` | BE-MCP-SOL-005 | FE-MCP-SOL-003 |
| `mcp.admin.client.setStatus` | `{clientId, status:'allowed'\|'blocked'}` | `McpOAuthClient` | BE-MCP-SOL-005 | FE-MCP-SOL-003 |
| `mcp.admin.grant.list` | `{userId?}` | `McpGrant[]` (mọi user) | BE-MCP-SOL-005 | FE-MCP-SOL-003 |
| `mcp.admin.grant.revoke` | `{grantId}` | `{ok:true}` | BE-MCP-SOL-005 | FE-MCP-SOL-003 |
| `mcp.admin.session.list` | — | `McpSessionView[]` (mọi user) | BE-MCP-SOL-004 | FE-MCP-SOL-002 |
| `mcp.admin.audit.query` | `{from?, to?, userId?, tool?, decision?, cursor?, limit?}` | `{entries: McpAuditEntry[], nextCursor?}` | BE-MCP-SOL-013 | FE-MCP-SOL-010 |
| `mcp.admin.prompt.list` | — | `McpPrompt[]` (gồm builtin) | BE-MCP-SOL-011 | FE-MCP-SOL-007 |
| `mcp.admin.prompt.upsert` | `McpPrompt` (không builtin) | `McpPrompt` | BE-MCP-SOL-011 | FE-MCP-SOL-007 |
| `mcp.admin.prompt.delete` | `{promptId}` | `{ok:true}` | BE-MCP-SOL-011 | FE-MCP-SOL-007 |
| `mcp.externalServer.list` | `{scope?}` | `McpExternalServer[]` (user thường chỉ thấy phạm vi của mình) | BE-MCP-SOL-014 | FE-MCP-SOL-011 |
| `mcp.externalServer.upsert` | `McpExternalServer` không `*.hasSecret`, `status`, `toolsDigest`... (server tự tính) | `McpExternalServer` | BE-MCP-SOL-014 | FE-MCP-SOL-011 |
| `mcp.externalServer.setSecret` | `{serverId, kind:'env'\|'header', name, value}` — **Đã chốt (D1):** `value` là plaintext gửi **một lần** qua WebSocket/TLS đã xác thực; **không** dùng phong bì client (`encryptCredential`/`encryptedBlob`/`iv`). Server mã hoá at-rest (credential-broker, Vault Transit). `value` bị che ở log/trace (C11); không bao giờ echo | `{hasSecret:true}` (không echo `value`) | BE-MCP-SOL-014 | FE-MCP-SOL-011 |
| `mcp.externalServer.probe` | `{serverId}` (chặn SSRF; chủ sở hữu được probe server `scope:'user'` của mình) | `{transport, tools:[{name,description}], digest, approvedTools?:[{name,description}]}` (`approvedTools` = bản đã duyệt gần nhất để FE diff) | BE-MCP-SOL-014 | FE-MCP-SOL-011 |
| `mcp.externalServer.review` | `{serverId, decision:'approve'\|'reject', toolsDigest}` | `McpExternalServer` | BE-MCP-SOL-014 | FE-MCP-SOL-011 |
| `mcp.externalServer.delete` | `{serverId}` | `{ok:true}` | BE-MCP-SOL-014 | FE-MCP-SOL-011 |

`externalServer.list/upsert/setSecret/delete` cho `scope:'user'` mở cho user thường với server của chính họ **nhưng** server `stdio` luôn cần admin `review`; các kênh còn lại của mục này yêu cầu admin.

### 2.3 Mã lỗi `MCP_*`

`MCP_DISABLED`, `MCP_NOT_ADMIN`, `MCP_KILL_SWITCH_ACTIVE`, `MCP_CONSENT_NOT_FOUND`, `MCP_CONSENT_EXPIRED`, `MCP_SCOPE_INVALID`, `MCP_SCOPE_NOT_ALLOWED`, `MCP_TOKEN_TOO_LONG`, `MCP_TOKEN_LIMIT`, `MCP_APPROVAL_EXPIRED`, `MCP_APPROVAL_ALREADY_DECIDED`, `MCP_APPROVAL_HASH_MISMATCH`, `MCP_POLICY_HARD_DENY`, `MCP_POLICY_VERSION_CONFLICT`, `MCP_SERVER_SSRF_BLOCKED`, `MCP_SERVER_NOT_APPROVED`, `MCP_NOT_FOUND` (không phân biệt "không tồn tại" và "không có quyền"), `MCP_INVALID_ARGUMENT`, `MCP_INTERNAL`, `MCP_TIMEOUT`, `MCP_UNAVAILABLE` (gRPC Unavailable/DeadlineExceeded từ mcp-service), `MCP_SERVER_INVALID`, `MCP_SERVER_STDIO_NOT_ALLOWED`, `MCP_SERVER_DIGEST_MISMATCH`, `MCP_SERVER_NAME_CONFLICT`, `MCP_PROMPT_INVALID` (message `MCP_PROMPT_INVALID: <field>: <reason>`), `MCP_PROMPT_NAME_CONFLICT`, `MCP_PROMPT_VERSION_CONFLICT`, `MCP_PROMPT_BUILTIN_READONLY`.

FE gặp mã `MCP_*` chưa biết ⇒ hiển thị thông điệp server (không crash).

## 3. Điểm cuối HTTP (không phải UI RPC)

| Endpoint | Người dùng | Ghi chú |
|----------|-----------|---------|
| `POST/GET/DELETE /mcp` | client MCP | Streamable HTTP (BE-003/004); cookie **bị từ chối** |
| `GET /.well-known/oauth-protected-resource`, `/.well-known/oauth-authorization-server` | client MCP | công khai (BE-005) |
| `POST /oauth/register`, `POST /oauth/token`, `POST /oauth/revoke` | client MCP | BE-005 (auth-service phục vụ qua gateway) |
| `GET /oauth/authorize` | **trình duyệt** | cần cookie `orca_session`; chưa đăng nhập ⇒ `302 /login?return_to=<url authorize>`; đã đăng nhập ⇒ tạo consent request rồi `302 /oauth/consent?request_id=<id>` |
| `POST/GET/DELETE /v1/auth/mcp-tokens` | CLI/script | tương đương `mcp.token.*`, cùng kiểu dữ liệu (BE-006) |

**Hạ tầng:** `/oauth/consent` **không** được proxy tới api-gateway ở nginx/ingress — phải rơi về SPA, kèm `Cache-Control: no-store`, `X-Frame-Options: DENY`, `Referrer-Policy: no-referrer`. Ngược lại `/oauth/*` (trừ `/oauth/consent`), `/.well-known/*`, `/mcp` **phải** được proxy (hiện `deploy/dev/docker/nginx/orca.conf` chỉ proxy danh sách path cố định — BE-MCP-SOL-005 §E). Cookie phiên là `SameSite=Strict` ⇒ `return_to` do SPA giữ trong `sessionStorage` (FE-MCP-SOL-003).

**Origin (D3, đã chốt):** `/ws`, `/mcp` kiểm `Origin` theo `WS_ALLOWED_ORIGINS` / `MCP_ALLOWED_ORIGINS` (mặc định = `WS_ALLOWED_ORIGINS`) khi không rỗng; rỗng ⇒ hành vi cũ của `/ws` kèm WARN khi khởi động (BE-MCP-SOL-002 §D.3). Origin web hợp lệ của deployment phải nằm trong danh sách này.

**Trang SPA mới (FE-MCP-SOL-003):** `/oauth/consent?request_id=…` — gọi `mcp.consent.get` rồi `mcp.consent.decide`, sau đó `window.location.assign(redirectUrl)`. Trang này phải hoạt động ở chế độ web (không Electron) và **không** nằm trong shell Orca đầy đủ (không tải terminal/worktree).

## 4. Tích hợp thông báo (notification-service, có sẵn)

Khi tạo `McpApproval`, BE publish `orca.mcp.approval.requested` (outbox) ⇒ notification-service tạo `NotificationEvent{type:"mcp.approval", deepLink:"/?section=mcp&tab=approvals&approval=<id>", severity:"warning", channels:[ws,push]}`. **Định dạng deepLink cố định (D5): `/?section=mcp&tab=approvals&approval=<id>`** — Settings **không** có route URL (điều hướng dựa store `openSettingsTarget`), nên SPA parse query khi khởi động rồi gọi `openSettingsTarget`; không dùng `/settings?...`. FE: (a) **đường chính:** hiển thị hộp thoại phê duyệt khi app đang mở (từ `mcp.event`), (b) khi app không focus: service worker (`frontend/src/renderer/public/service-worker.js` — xem FE-MCP-SOL-009) hiển thị Web Push từ payload JSON `{title, body, deepLink, tag}`; `notificationclick` focus client hiện có và `postMessage({type:'orca:navigate', url: deepLink})` hoặc `openWindow(deepLink)`; SPA xử lý `orca:navigate` và deep link lúc khởi động nguội. Hai đường quyết định cùng dẫn tới `mcp.approval.decide`.

**Phụ thuộc thật của đường (b):** `notification-service` **chưa có usecase `DeliverPush`** (CR-NOTIF-002, `docs/crs/v4/notification/CR-NOTIF-002-deliver-push-usecase.md`; README service mục "Known gaps") nên push không bao giờ thực sự được gửi cho tới khi CR đó hoàn tất; trong lúc đó chỉ có đường (a). Ngoài ra `useWebPushSubscription` gọi `/api/vapid-public-key|push-subscribe|push-unsubscribe` (TDD v4/10 ghi `/push/...` là lỗi thời).

## 5. Trường bổ sung vào kênh có sẵn (additive, BE-MCP-SOL-009 ↔ FE-MCP-SOL-006)

Kết quả của các kênh liệt kê/lấy phiên terminal và agent (`terminal.*`, `agent.*`) có thêm trường tuỳ chọn:
`origin?: { type: 'mcp'; clientName: string; mcpSessionId: string; userId: string }` — vắng mặt = phiên do người dùng tạo bằng UI. FE dùng để hiển thị nhãn "Tạo bởi agent" và nút dừng.

## 6. Cờ tính năng & khả dụng

- Backend: `MCP_ENABLED` (BE-002, công tắc tổng cấp tiến trình, mặc định `false` tới gate CR-015) và `MCP_TENANT_DEFAULT_ENABLED` (D6, mặc định `true`: giá trị `enabled` của hàng settings được tạo lười cho tenant mới; admin vẫn tắt được; **không** đổi mặc định rủi ro/hard-deny/scope/kill switch). FE đọc `mcp.server.info.enabled`; `false` ⇒ ẩn toàn bộ UI MCP (không lỗi).
- FE: không thêm cờ build riêng; mọi UI MCP lazy-load. Trong cây hiện tại `ORCA_PLATFORM` được hard-code `'web'`, nên điều kiện thật là `window.api.mcp` tồn tại **và** `McpServerInfo.enabled === true` (FE-MCP-SOL-001 chốt chi tiết). Khi `enabled && tenantEnabled === false` (admin đã tắt, hoặc vận hành đặt mặc định `false`), chỉ admin thấy công tắc bật MCP; tenant mới thường có `tenantEnabled === true` ngay từ đầu.
- i18n: khoá dịch scope = id thay `:` bằng `_` (`auto.mcp.scope.orca_read`) vì i18next coi `:` là ngăn cách namespace.
- Số liệu: registry production hiện có **449 channel / 57 namespace** (đo bằng test dựng `Registry` với 6 lời gọi đăng ký của `cmd/server/main.go`; xem BE-MCP-SOL-007 §2.F) — con số 417/58 trong CR là grep thô, bỏ sót đăng ký qua vòng lặp/helper.
