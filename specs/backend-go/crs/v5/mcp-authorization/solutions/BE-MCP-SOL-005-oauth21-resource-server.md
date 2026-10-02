# BE-MCP-SOL-005: OAuth 2.1 — `auth-service` là Authorization Server, `mcp-service` giữ consent/grant, `api-gateway` là Resource Server edge

> **🔲 Designed — chưa implement.** Thứ tự PR và phụ thuộc chéo với BE-MCP-SOL-006: xem [README](./README.md) §3. Mọi thay đổi bám [CONTRACT-mcp-ui-api.md](../../CONTRACT-mcp-ui-api.md).

**CR:** [CR-MCP-005](../../../../../../docs/crs/v5/mcp-authorization/CR-MCP-005-oauth21-resource-server.md)
**Service:** `auth-service` (nhóm RPC `OAuth*`, 6 bảng mới), `mcp-service` (consent/grant, gRPC), `api-gateway` (`/.well-known/*`, `/oauth/*`, `adapter/mcpauth`, 8 kênh WS)
**TDD tham chiếu:** [`auth-service.md`](../../../../tdd/services/auth-service.md) §3, §4, §5, §7, §9 · [`api-gateway.md`](../../../../tdd/services/api-gateway.md) §3, §6, §9 · `architecture/05` (tenant, outbox) · `architecture/07` (AuthN, audit)
**CONTRACT được hiện thực:** §2.1 `mcp.consent.get`, `mcp.consent.decide`, `mcp.grant.list`, `mcp.grant.revoke`; §2.2 `mcp.admin.client.list`, `mcp.admin.client.setStatus`, `mcp.admin.grant.list`, `mcp.admin.grant.revoke`; §3 `/.well-known/*`, `POST /oauth/{register,token,revoke}`, `GET /oauth/authorize`; mã lỗi `MCP_CONSENT_NOT_FOUND`, `MCP_CONSENT_EXPIRED`, `MCP_SCOPE_INVALID`, `MCP_SCOPE_NOT_ALLOWED`, `MCP_NOT_ADMIN`, `MCP_NOT_FOUND`, `MCP_DISABLED`.

---

## 1. Trạng thái hiện tại (re-verify — chi tiết ở [README](./README.md) §2)

Đã đọc mã thật (không suy từ CR):

- `auth-service` **chưa có** AS: `proto/orca/auth/v1/auth.proto` chỉ có `Login/Logout/ValidateSession/IssueServiceToken/GetJWKS/IsServiceTokenRevoked/ListCliTokens/RevokeCliToken/StartSsoLogin/CompleteSsoLogin/RefreshSession/...`. Thư mục `internal/adapter/oauth` hiện là **client phía SSO** (đổi code lấy token của GitHub/Google/OIDC), không phải AS — tên dễ nhầm, code mới đặt ở `usecase/oauth_*.go` + `adapter/postgres/oauth_repository.go`, **không** thêm vào `adapter/oauth`.
- Ký JWT: `usecase.TokenSigner` (Vault Transit, khoá `jwt-signing`) + `jwtauth.Claims` (`common/jwtauth/jwtauth.go`) **chưa có** `scope/client_id/grant_id`; `jwtauth.Issuer = "auth-service"` cố định và `VerifyWithKey` chỉ kiểm `exp/iat/iss`, **không kiểm `aud`**.
- Mẫu có sẵn để tái dùng: `RefreshSession` (`usecase/refresh_session.go`) đã có rotation + reuse detection cho phiên trình duyệt; `IssueServiceToken`/`IsServiceTokenRevoked` cho CLI; `domain.HashSessionToken` (SHA-256) dùng chung.
- Gateway: `authMiddleware` thử cookie (`CookieSessionValidator`) rồi `AuthValidator.Validate`; cookie `orca_session` đặt `SameSite=Strict` (`auth_routes.go setSessionCookie`). Có tiền lệ route **không** qua `authMiddleware` (`mountUnauthenticatedPairingRoutes`, `mountPushRoutes`) với rate-limit riêng khoá theo IP.
- SSO: `handleSsoStart` → IdP → `handleSsoCallback` đặt cookie rồi `302 /` — **không có `return_to`** ở đâu trong backend hay frontend (`grep return_to` = 0).
- nginx dev (`deploy/dev/docker/nginx/orca.conf`) chỉ proxy các đường dẫn liệt kê (`/auth/...`, `/ws`, `/v1/`, `/admin/api/`); mọi thứ khác rơi vào SPA fallback `web-index.html` ⇒ `/oauth/*`, `/.well-known/*`, `/mcp` hiện **trả HTML của SPA**.
- `mcp-service` chưa tồn tại trong `backend-go/services/` (BE-MCP-SOL-001/002 scaffold). Tên package proto `orca.mcp.v1` / service `McpService` là **giả định** (chưa xác minh — BE-001 chốt).

### Quyết định khác/thêm so với CR gốc

1. **CR gốc tự mâu thuẫn** (mục B: `/authorize,/token,/register` ở `auth-service`; mục C: client lưu ở `mcp-service.oauth_clients`). Theo CONTRACT/T3: `oauth_clients` ở **auth-service**; `mcp-service` chỉ giữ **grant + consent request**. Danh sách client cho UI do `mcp-service` ghép (gọi auth-service + đếm grant của chính nó) để gateway không phải ghép dữ liệu liên service.
2. **DCR diễn ra trước khi biết tenant** (`POST /oauth/register` ẩn danh). Vì vậy `auth.oauth_clients` là **registry toàn deployment, không có `tenant_id`** (ngoại lệ có chủ đích của quy ước `tenant_id NOT NULL`: bảng chỉ chứa danh tính app, không dữ liệu tenant). Trạng thái cho phép/chặn là **theo tenant** ở bảng `auth.oauth_client_tenant_status` (có `tenant_id` + RLS). `dcrEnabled` của tenant (BE-MCP-SOL-012) quyết định trạng thái khởi tạo của client DCR trong tenant đó: `true` → `allowed`, `false` → `pending` (admin phải duyệt, đúng ngữ nghĩa "allow-list" của CR). Cờ cấp deployment `OAUTH_DCR_ENABLED` tắt hẳn endpoint `/oauth/register`.
3. **Mã authorization không do gateway xin trực tiếp.** Chỉ `mcp-service` (sau khi consent được quyết) gọi `OAuthIssueAuthCode` — đảm bảo không có đường nào cấp code mà không qua grant. Hệ quả hướng phụ thuộc: `mcp-service → auth-service` (đúng quy tắc `auth-service.md` §7: auth-service **không** gọi mcp-service ở đường login lõi; `OAuthExchangeToken` và `ResolveMcpPrincipal` chỉ đọc bảng của chính auth-service).
4. **Auto-approve:** nếu client không mới, tenant cho phép, và grant `active` đã bao phủ mọi scope yêu cầu ⇒ `mcp-service` cấp code ngay (bỏ qua màn consent). Mở rộng scope hoặc client mới ⇒ luôn hỏi lại (đúng CR-D).
5. **Thu hồi ≤ 60s** không dùng "gọi live mỗi request" như `RevocationClient` hiện tại (mỗi request 1 RPC, không cache) mà dùng `ResolveMcpPrincipal` (BE-MCP-SOL-006 §C) + cache 30s ⇒ trần lệch 30s.
6. **SSO/OIDC: không thêm đường đăng nhập thứ hai.** `/oauth/authorize` chỉ kiểm cookie; chưa đăng nhập thì `302 /login?return_to=...` và để SPA dùng `/auth/local` hoặc `/auth/sso/*` hiện có. Vì cookie là `SameSite=Strict` (redirect từ IdP là chuỗi cross-site nên cookie **không** đi kèm lần điều hướng đầu), việc quay lại `return_to` phải do **JS của SPA** thực hiện (điều hướng cùng site) — xem §H. **Không đổi `handleSsoCallback`** (vẫn `302 /`); FE giữ `return_to` trong `sessionStorage` qua vòng IdP.
7. **Chưa làm:** Client ID Metadata Documents, client confidential (secret qua Vault/`credential-broker-service`), custom URI scheme — ghi vào backlog (§Không thuộc phạm vi). Metadata **không** quảng bá những tính năng này.

---

## 2. Giải pháp

### A. Cấu hình (env, tiền tố theo T8; đối chiếu BE-MCP-SOL-002)

| Service | Biến | Mặc định | Ý nghĩa |
|---|---|---|---|
| api-gateway | `MCP_PUBLIC_BASE_URL` | `PUBLIC_BASE_URL` | origin công khai; `resource = <base>/mcp`, `issuer = <base>` (không bao giờ lấy từ `Host`/`X-Forwarded-Host` — cùng lý do `handleSsoStart`) |
| auth-service | `OAUTH_RESOURCE_URL` | — (bắt buộc khi bật AS) | `aud` duy nhất được cấp; so khớp chính xác với tham số `resource` |
| auth-service | `OAUTH_ACCESS_TOKEN_TTL` / `OAUTH_REFRESH_TOKEN_TTL` / `OAUTH_AUTH_CODE_TTL` | `10m` / `30d` / `60s` | CR: access ≤ 15 phút |
| auth-service | `OAUTH_DCR_ENABLED`, `OAUTH_DCR_MAX_CLIENTS` | `true`, `1000` | trần chống DCR spam |
| mcp-service | `MCP_CONSENT_TTL` | `10m` | hạn `consent_requests` |

### B. Proto — `backend-go/proto/orca/auth/v1/auth.proto` (additive; `make proto-gen && make proto-lint`)

```proto
// --- OAuth Authorization Server (BE-MCP-SOL-005). RPC *không* tenant-scoped (Register/Validate/Exchange/Revoke)
// được api-gateway gọi không kèm identity, giống CompleteDevicePairing. RPC có hậu tố "ForTenant"/Grant
// chỉ chấp nhận caller nội bộ là mcp-service (NetworkPolicy + mTLS peer identity — chưa xác minh cơ chế interceptor).
rpc OAuthRegisterClient(OAuthRegisterClientRequest) returns (OAuthRegisterClientResponse);          // RFC 7591
rpc OAuthValidateAuthorizeRequest(OAuthValidateAuthorizeRequestRequest) returns (OAuthAuthorizeRequestInfo);
rpc OAuthIssueAuthCode(OAuthIssueAuthCodeRequest) returns (OAuthIssueAuthCodeResponse);             // chỉ mcp-service
rpc OAuthExchangeToken(OAuthExchangeTokenRequest) returns (OAuthTokenResponse);                     // authorization_code | refresh_token
rpc OAuthRevokeToken(OAuthRevokeTokenRequest) returns (google.protobuf.Empty);                      // RFC 7009
rpc OAuthRevokeGrant(OAuthRevokeGrantRequest) returns (google.protobuf.Empty);                      // chỉ mcp-service, idempotent
rpc OAuthListClientsForTenant(google.protobuf.Empty) returns (OAuthListClientsResponse);            // tenant từ metadata
rpc OAuthSetClientStatus(OAuthSetClientStatusRequest) returns (OAuthClientTenantView);              // chỉ mcp-service (admin đã kiểm)
rpc OAuthEnsureClientForTenant(OAuthEnsureClientForTenantRequest) returns (OAuthClientTenantView);  // tạo hàng status lần đầu thấy client trong tenant

message OAuthValidateAuthorizeRequestRequest {
  string response_type = 1; string client_id = 2; string redirect_uri = 3; string scope = 4;
  string code_challenge = 5; string code_challenge_method = 6; string resource = 7;
}
message OAuthAuthorizeRequestInfo {      // chỉ trả khi client + redirect_uri hợp lệ
  string client_id = 1; string client_name = 2; string client_uri = 3;
  repeated string scopes = 4; string registered_via = 5;
}
message OAuthIssueAuthCodeRequest {      // tenant/user lấy từ metadata (mcp-service chuyển tiếp danh tính của người dùng đã consent)
  string client_id = 1; string redirect_uri = 2; repeated string scopes = 3;
  string code_challenge = 4; string resource = 5; string grant_id = 6;
}
message OAuthIssueAuthCodeResponse { string code = 1; google.protobuf.Timestamp expires_at = 2; }
message OAuthExchangeTokenRequest {
  string grant_type = 1; string code = 2; string redirect_uri = 3; string code_verifier = 4;
  string client_id = 5; string refresh_token = 6; string resource = 7; string scope = 8;
}
message OAuthTokenResponse {
  string access_token = 1; int32 expires_in = 2; string refresh_token = 3; string scope = 4;
}
```

Lỗi RFC 6749 đi qua `apperrors` với `Code` = `OAUTH_INVALID_GRANT`, `OAUTH_INVALID_CLIENT`, `OAUTH_INVALID_REQUEST`, `OAUTH_INVALID_SCOPE`, `OAUTH_INVALID_TARGET`, `OAUTH_UNAUTHORIZED_CLIENT`; gateway dịch sang JSON `{error, error_description}` (§E). Không dùng `MCP_*` ở tầng HTTP OAuth — client MCP chỉ hiểu mã RFC.

### C. `auth-service` — domain, usecase, migration

Domain (`internal/domain/`, chỉ stdlib): `oauth_client.go` (`OAuthClient`, `ValidateRedirectURI`), `oauth_auth_code.go`, `oauth_refresh_token.go` (`TokenFamily`), `oauth_client_tenant_status.go`.

`ValidateRedirectURI` (pure, test bảng): chỉ `https://`; hoặc `http://` với host đúng `127.0.0.1`, `[::1]`, `localhost` (mọi port); cấm fragment, userinfo, wildcard `*`, đường dẫn chứa `..`; ≤ 2048 ký tự; so khớp lúc authorize/token là **so sánh chuỗi chính xác** với phần tử đã đăng ký (riêng loopback cho phép port khác theo RFC 8252 §7.3 — chỉ so scheme+host+path).

Usecase (mỗi file một type `Execute`): `oauth_register_client.go`, `oauth_validate_authorize_request.go`, `oauth_issue_auth_code.go`, `oauth_exchange_token.go` (hai nhánh `exchangeCode`/`rotateRefresh`), `oauth_revoke_token.go`, `oauth_revoke_grant.go`, `oauth_list_clients_for_tenant.go`, `oauth_set_client_status.go`. Port mới trong `usecase/ports.go`: `OAuthRepository` (client, status, code, family, refresh, grant revocation).

Migration `services/auth-service/migrations/postgres/0011_oauth_authorization_server.{up,down}.sql` (+ bản `mysql/0011_*` song song vì service có cả hai adapter — CR-DB-002; MySQL dùng `JSON` thay `TEXT[]`, bỏ RLS):

```sql
-- Registry toàn deployment: DCR ẩn danh nên chưa có tenant (ngoại lệ có chủ đích của quy ước tenant_id NOT NULL).
CREATE TABLE auth.oauth_clients (
    client_id       TEXT PRIMARY KEY,                         -- 32 byte ngẫu nhiên base64url
    client_name     TEXT NOT NULL CHECK (char_length(client_name) BETWEEN 1 AND 100),
    client_uri      TEXT,
    redirect_uris   TEXT[] NOT NULL CHECK (cardinality(redirect_uris) BETWEEN 1 AND 5),
    registered_via  TEXT NOT NULL CHECK (registered_via IN ('dcr','admin')),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_used_at    TIMESTAMPTZ
);
CREATE TABLE auth.oauth_client_tenant_status (
    tenant_id   UUID NOT NULL,
    client_id   TEXT NOT NULL REFERENCES auth.oauth_clients(client_id),
    status      TEXT NOT NULL CHECK (status IN ('allowed','blocked','pending')),
    updated_by  UUID, updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, client_id)
);
CREATE TABLE auth.oauth_token_families (                      -- 1 family = 1 lần authorize thành công
    family_id UUID PRIMARY KEY, tenant_id UUID NOT NULL, user_id UUID NOT NULL REFERENCES auth.users(id),
    client_id TEXT NOT NULL REFERENCES auth.oauth_clients(client_id), grant_id UUID NOT NULL,  -- grant_id: logical FK -> mcp.grants (không FK xuyên DB)
    scope TEXT NOT NULL, resource TEXT NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at TIMESTAMPTZ, revoke_reason TEXT      -- 'user_revoked'|'admin_revoked'|'reuse_detected'|'client_blocked'
);
CREATE TABLE auth.oauth_auth_codes (
    code_hash TEXT PRIMARY KEY,                               -- SHA-256 hex, không lưu code thô
    family_id UUID NOT NULL REFERENCES auth.oauth_token_families(family_id), tenant_id UUID NOT NULL,
    redirect_uri TEXT NOT NULL, code_challenge TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL, used_at TIMESTAMPTZ
);
CREATE TABLE auth.oauth_refresh_tokens (
    token_hash TEXT PRIMARY KEY, family_id UUID NOT NULL REFERENCES auth.oauth_token_families(family_id),
    tenant_id UUID NOT NULL, expires_at TIMESTAMPTZ NOT NULL,
    used_at TIMESTAMPTZ,                                      -- đã xoay: dùng lại = reuse
    replaced_by_hash TEXT
);
CREATE TABLE auth.oauth_grant_revocations (grant_id UUID PRIMARY KEY, tenant_id UUID NOT NULL, revoked_at TIMESTAMPTZ NOT NULL DEFAULT now());
CREATE INDEX idx_oauth_families_user ON auth.oauth_token_families (tenant_id, user_id);
CREATE INDEX idx_oauth_codes_expiry ON auth.oauth_auth_codes (expires_at);   -- reaper
-- RLS tenant_isolation cho 5 bảng có tenant_id, đúng khuôn 0001_init (tra cứu theo hash ở /token chạy không có tenant
-- trong context, giống sessions: tenant lấy từ hàng, enforcement chính ở tầng ứng dụng).
```

Luồng nghiệp vụ (bất biến an toàn):

- `OAuthRegisterClient`: tắt nếu `!OAUTH_DCR_ENABLED`; kiểm `OAUTH_DCR_MAX_CLIENTS`; chỉ chấp nhận `token_endpoint_auth_method="none"`, `grant_types ⊆ {authorization_code, refresh_token}`, `response_types=["code"]`; `client_name` bỏ ký tự điều khiển/URL; trả `client_id` + `client_id_issued_at`, **không** secret. Audit `oauth.client_registered`.
- `OAuthValidateAuthorizeRequest`: `response_type=code`; **`code_challenge` bắt buộc, `method=S256`** (từ chối `plain`/thiếu → `OAUTH_INVALID_REQUEST`); độ dài challenge 43–128; `resource` bắt buộc và `== OAUTH_RESOURCE_URL` (sai → `OAUTH_INVALID_TARGET`); `scope` ⊆ `mcpscope.All` (BE-MCP-SOL-006 §B), rỗng ⇒ mặc định `orca:read`. Thứ tự: kiểm `client_id` + `redirect_uri` **trước** — nếu hỏng thì lỗi trả về trực tiếp, **không** redirect (chống open-redirect).
- `OAuthIssueAuthCode`: tạo family (`grant_id`, scope đã consent), sinh code 32 byte, lưu hash, TTL 60s; scope phải ⊆ `mcpscope.CeilingForRole(role)` — role lấy từ `users` (không từ metadata).
- `exchangeCode`: tra theo hash → (a) `used_at != NULL` ⇒ **code replay**: `revokeFamily(reason='reuse_detected')` + audit `oauth.code_replay`, trả `invalid_grant`; (b) hết hạn; (c) `client_id`/`redirect_uri` khớp chính xác; (d) PKCE: `base64url(sha256(code_verifier)) == code_challenge` (`subtle.ConstantTimeCompare`); (e) `resource` nếu gửi phải bằng `family.resource`; (f) client không `blocked`, grant không có trong `oauth_grant_revocations`, user `is_active`; rồi `UPDATE ... SET used_at=now() WHERE code_hash=$1 AND used_at IS NULL` (không có hàng ⇒ thua race ⇒ `invalid_grant`) và phát hành cặp token trong cùng transaction.
- `rotateRefresh`: tra theo hash; `used_at != NULL` ⇒ **reuse** ⇒ thu hồi cả family + audit `oauth.refresh_reuse_detected` (trả `invalid_grant`); hợp lệ ⇒ đánh dấu `used_at`, cấp refresh mới (`replaced_by_hash`), **TTL tuyệt đối không gia hạn quá `family.created_at + OAUTH_REFRESH_TOKEN_TTL`**; scope yêu cầu thêm ⊆ scope family (chỉ thu hẹp được).
- Access token (JWT RS256 qua `TokenSigner` hiện có): `iss=jwtauth.Issuer` (giữ `"auth-service"` để verifier hiện tại dùng được; `issuer` trong metadata là URL công khai — hai khái niệm khác nhau, client không xác minh JWT), `sub=user_id`, `aud=[OAUTH_RESOURCE_URL]`, `tenant_id`, `scope`, `client_id`, `grant_id`, `fid=family_id`, `jti`, `exp=now+TTL`. Thêm field vào `jwtauth.Claims` theo BE-MCP-SOL-006 §A. **Không** đặt `role`.
- `OAuthRevokeToken` (RFC 7009): luôn trả 200 kể cả token lạ; refresh ⇒ thu hồi family; access (parse `fid`) ⇒ thu hồi family.
- `OAuthRevokeGrant(grant_id)`: idempotent — chèn `oauth_grant_revocations`, `UPDATE token_families SET revoked_at=now(), revoke_reason=... WHERE grant_id=$1`. `OAuthSetClientStatus(blocked)`: thu hồi mọi family `client_id` trong tenant với reason `client_blocked`.
- `ResolveMcpPrincipal` (BE-MCP-SOL-006 §C) kiểm: family chưa thu hồi, grant không bị thu hồi, client không `blocked`, user `is_active`.

Audit (qua `AuditRepository` nội bộ, như `IssueServiceToken`): `oauth.client_registered`, `oauth.code_issued`, `oauth.token_issued`, `oauth.refresh_rotated`, `oauth.refresh_reuse_detected`, `oauth.code_replay`, `oauth.grant_revoked`, `oauth.client_status_changed`. **Không** ghi code/token/verifier vào audit, log, hay span (thêm `code`, `code_verifier`, `refresh_token`, `access_token` vào danh sách attr bị che của `common/logging` — chưa xác minh tên hàm redaction, kiểm khi cài đặt).

### D. `mcp-service` — consent & grant (gRPC; schema `mcp`, migration `0002_authorization.up.sql` — số thứ tự tuỳ BE-001)

```sql
CREATE TABLE mcp.consent_requests (
    id UUID PRIMARY KEY, tenant_id UUID NOT NULL, user_id UUID NOT NULL,   -- logical FK -> auth.users
    client_id TEXT NOT NULL, client_name TEXT NOT NULL, client_uri TEXT,
    redirect_uri TEXT NOT NULL, scopes TEXT[] NOT NULL, state TEXT,         -- state của client, ≤ 512 ký tự, trả nguyên văn
    code_challenge TEXT NOT NULL, resource TEXT NOT NULL,
    is_new_client BOOLEAN NOT NULL, registered_via_dcr BOOLEAN NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(), expires_at TIMESTAMPTZ NOT NULL,
    decided_at TIMESTAMPTZ, decision TEXT CHECK (decision IN ('approve','deny'))
);
CREATE TABLE mcp.grants (
    id UUID PRIMARY KEY, tenant_id UUID NOT NULL, user_id UUID NOT NULL, client_id TEXT NOT NULL,
    client_name TEXT NOT NULL, client_uri TEXT, scopes TEXT[] NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('active','revoked')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_used_at TIMESTAMPTZ, revoked_at TIMESTAMPTZ, revoked_by UUID,
    revocation_propagated_at TIMESTAMPTZ                      -- NULL & status='revoked' => job reconcile còn phải gọi auth-service
);
CREATE UNIQUE INDEX uq_grants_active ON mcp.grants (tenant_id, user_id, client_id) WHERE status = 'active';
-- + RLS tenant_isolation, outbox_events (theo BE-001)
```

Usecase (`internal/usecase/`): `create_consent_request.go`, `get_consent_request.go`, `decide_consent.go`, `list_grants.go`, `revoke_grant.go`, `list_oauth_clients.go`, `set_oauth_client_status.go`, `reconcile_grant_revocations.go` (job 15s).

- `CreateConsentRequest` (gateway gọi sau khi cookie hợp lệ): gọi `auth.OAuthEnsureClientForTenant` (tạo hàng `oauth_client_tenant_status` — `allowed` nếu tenant `dcrEnabled` hoặc client `admin`, ngược lại `pending`); `blocked`/`pending` ⇒ lỗi `MCP_CLIENT_NOT_ALLOWED` (**mã nội bộ gateway → redirect `error=access_denied`**, không phát ra qua WS nên không nằm trong CONTRACT §2.3); scope yêu cầu ∩ `mcpscope.CeilingForRole`; auto-approve nếu grant active ⊇ scope ⇒ gọi `OAuthIssueAuthCode` và trả `redirect_url`; còn lại ghi `consent_requests` (TTL `MCP_CONSENT_TTL`) và trả `request_id`.
- `DecideConsent`: khoá hàng (`UPDATE ... SET decided_at=now(), decision=$d WHERE id=$1 AND user_id=$caller AND decided_at IS NULL RETURNING`). Không có hàng: phân biệt hết hạn (`expires_at < now()` ⇒ `MCP_CONSENT_EXPIRED`) với còn lại ⇒ `MCP_CONSENT_NOT_FOUND` (gồm cả đã quyết, khác user, khác tenant — không phân biệt, chống dò). `approve`: `scopes` phải ≠ ∅, ⊆ scope client xin, ⊆ trần theo role (sai ⇒ `MCP_SCOPE_INVALID` / `MCP_SCOPE_NOT_ALLOWED`); upsert grant (scope = tập user chọn), outbox `orca.mcp.grant.created|updated`, gọi `auth.OAuthIssueAuthCode`; `redirectUrl = redirect_uri + ?code=…&state=…&iss=<issuer>` (RFC 9207 chống mix-up). `deny`: `redirectUrl = redirect_uri?error=access_denied&state=…&iss=…`, không tạo grant.
- `RevokeGrant` (user: chỉ grant của chính mình; admin: bất kỳ trong tenant — sai chủ ⇒ `MCP_NOT_FOUND`): tx{status='revoked', outbox `orca.mcp.grant.revoked`} rồi gọi `auth.OAuthRevokeGrant`, thành công ⇒ set `revocation_propagated_at`; thất bại ⇒ job `reconcile_grant_revocations` thử lại (lỗi vẫn trả OK cho UI vì trạng thái bền đã ghi). Đây là chỗ kết nối "≤ 60s": 30s cache gateway + vài giây RPC.
- `ListOAuthClients`/`SetOAuthClientStatus` (admin; Rego bên dưới): ghép `OAuthListClientsForTenant` + `COUNT(*) FILTER (status='active')` theo client; `registeredVia` = `registered_via`; `blocked` ⇒ audit + outbox `orca.mcp.client.status_changed`.
- Tên người dùng cho `McpGrant.userName` (kênh admin): `auth.ListTenantMemberDirectory` (RPC có sẵn, không cần quyền admin).

Rego `policy/orca-authz/mcp_oauth.rego` + `mcp_oauth_test.rego` (bundle `orca-authz`, `make opa-test`):

```rego
package orca.authz.mcp_oauth
import rego.v1
default allow := false
allow if { input.action == "grant.revoke"; input.resource.user_id == input.subject.user_id }
allow if { startswith(input.action, "admin."); input.subject.role == "admin" }   # client.list/setStatus, grant.list/revoke của người khác
```

NATS (outbox, payload có `event_id, tenant_id, occurred_at, schema_version:1`): `orca.mcp.grant.created`, `orca.mcp.grant.updated`, `orca.mcp.grant.revoked {grant_id, user_id, client_id, by}`, `orca.mcp.client.status_changed {client_id, status}`. `notification-service` không đăng ký các subject này; luồng `McpEvent grant.revoked` tới UI do BE-MCP-SOL-013 lấy từ `orca.mcp.grant.revoked`.

### E. `api-gateway` — route công khai (`httpgateway/oauth_routes.go`, `oauth_well_known_routes.go`; mount **ngoài** group `authed`, cạnh `mountUnauthenticatedPairingRoutes`)

| Route | Hành vi |
|---|---|
| `GET /.well-known/oauth-protected-resource` và `…/mcp` (dạng path-insert theo MCP spec) | RFC 9728: `{"resource":"<base>/mcp","authorization_servers":["<base>"],"scopes_supported":[…mcpscope.All],"bearer_methods_supported":["header"]}`; `Cache-Control: public, max-age=300` |
| `GET /.well-known/oauth-authorization-server` | RFC 8414: `issuer`, `authorization_endpoint`, `token_endpoint`, `revocation_endpoint`, `registration_endpoint` (chỉ khi `OAUTH_DCR_ENABLED`), `response_types_supported:["code"]`, `grant_types_supported:["authorization_code","refresh_token"]`, `code_challenge_methods_supported:["S256"]`, `token_endpoint_auth_methods_supported:["none"]`, `scopes_supported`, `authorization_response_iss_parameter_supported:true`. Không có `jwks_uri` (token là opaque với client; JWKS chỉ cho verifier nội bộ) |
| `POST /oauth/register` | JSON ≤ 8 KiB → `OAuthRegisterClient` → `201`; rate-limit khoá `oauth-register:<ip>` (10/giờ, burst 5) theo mẫu `pairingRateLimitMiddleware` |
| `GET /oauth/authorize` | xem dưới |
| `POST /oauth/token` | `application/x-www-form-urlencoded`; rate-limit `oauth-token:<client_id>:<ip>`; `Cache-Control: no-store`, `Pragma: no-cache`; lỗi → `400 {"error","error_description"}` (`invalid_grant`, `invalid_client`, `invalid_target`, …) |
| `POST /oauth/revoke` | → `OAuthRevokeToken`, luôn `200` |

`GET /oauth/authorize` (`handleAuthorize`):

```go
info, err := authClient.OAuthValidateAuthorizeRequest(ctx, fromQuery(r))        // không identity
if err != nil { renderAuthorizeError(w, err); return }                           // client/redirect hỏng: 400 trang lỗi, KHÔNG redirect
id, err := deps.CookieValidator.ValidateCookie(ctx, r)                           // chỉ cookie; Bearer bị bỏ qua ở endpoint này
if err != nil {
    http.Redirect(w, r, "/login?return_to="+url.QueryEscape(r.URL.RequestURI()), http.StatusFound) // chỉ đường dẫn tương đối
    return
}
resp, err := mcpClient.CreateConsentRequest(gatewaygrpc.AttachIdentity(ctx, identity(id)), …)
switch {
case resp.GetRedirectUrl() != "":  http.Redirect(w, r, resp.GetRedirectUrl(), http.StatusFound)        // auto-approve hoặc access_denied
default:                           http.Redirect(w, r, "/oauth/consent?request_id="+resp.GetRequestId(), http.StatusFound)
}
```

Header an toàn cho `/oauth/authorize` và `/oauth/consent`-redirect: `Cache-Control: no-store`, `Referrer-Policy: no-referrer`, `X-Frame-Options: DENY`.

**nginx / ingress (bắt buộc, nếu không toàn bộ trả HTML SPA):** thêm vào `deploy/dev/docker/nginx/orca.conf` (và mọi ingress prod tương đương — chưa tìm thấy cấu hình nginx trong `deploy/prod`, **chưa xác minh**) các block proxy tới `$api_gateway`: `location ^~ /.well-known/`, `location = /mcp` (kèm `proxy_buffering off`, `proxy_read_timeout 3600s` cho SSE), `location ~ ^/oauth/(authorize|register|token|revoke)$`. **`/oauth/consent` KHÔNG proxy** — để rơi vào SPA fallback.

### F. `api-gateway` — `adapter/mcpauth` (bao `/mcp`, do BE-MCP-SOL-003 mount)

`mcpauth.Middleware(validator, resourceURL, metadataURL)` bao handler `/mcp`: (1) `AuthValidator.ValidateMCP` (BE-MCP-SOL-006 §C) — **chỉ đọc `Authorization: Bearer`; cookie `orca_session` hiện diện cũng bị bỏ qua và nếu không có Bearer thì `401`** (+ counter `orca_mcp_auth_rejected_total{reason="cookie"}`); (2) mọi lỗi xác thực ⇒ `401` + `WWW-Authenticate: Bearer resource_metadata="<base>/.well-known/oauth-protected-resource/mcp", scope="orca:read"` (+ `error="invalid_token"` khi có token nhưng sai); (3) thiếu scope (do tầng tool, BE-007) ⇒ `403` + `error="insufficient_scope", scope="<scope cần>"`; (4) lưu `McpPrincipal` vào ctx. **Cấm token passthrough:** header `Authorization` không bao giờ được forward; downstream chỉ nhận `AttachIdentity(Identity{TenantID,UserID,Role})` — thêm test khẳng định `outgoing metadata` không chứa token.

### G. `api-gateway` — kênh WS `channels_mcp.go` (file mới do BE-MCP-SOL-003 tạo khung; BE-005 thêm `registerMcpAuthorizationChannels`)

| Kênh | Gọi | Ghi chú |
|---|---|---|
| `mcp.consent.get(requestId)` | `McpService.GetConsentRequest` | trả `McpConsentRequest`; `redirectHost` = chỉ host; `scopes` là `McpScopeDescriptor` do mcp-service dựng; `tenant.name` từ `tenantv1.GetCompany` (lỗi ⇒ dùng id, không fail) |
| `mcp.consent.decide(requestId, {decision, scopes})` | `DecideConsent` | trả `{redirectUrl}` |
| `mcp.grant.list()` / `mcp.grant.revoke(grantId)` | `ListGrants{self}` / `RevokeGrant` | |
| `mcp.admin.client.list()` / `.setStatus(clientId, status)` | `ListOAuthClients` / `SetOAuthClientStatus` | `status` chỉ `allowed|blocked`; `Role != admin` ⇒ `MCP_NOT_ADMIN` |
| `mcp.admin.grant.list({userId?})` / `.revoke(grantId)` | `ListGrants{all}` / `RevokeGrant{admin}` | |

Điểm cần chú ý (đã kiểm): `Registry.Dispatch` trả lỗi gRPC thô cho client qua `writeDialectError` → `err.Error()` = `"rpc error: code = NotFound desc = MCP_CONSENT_NOT_FOUND: …"` làm **hỏng regex `^([A-Z0-9_]+): ` của FE (C4)**. `channels_mcp.go` phải bọc mọi lỗi gRPC bằng `mcpChannelError(err)` = `status.Convert(err).Message()` (đã dạng `"CODE: msg"` nhờ `apperrors.ToGRPCStatus`), có test bảng. Khi `MCP_ENABLED=false` ⇒ `MCP_DISABLED` (C8).

### H. Đăng nhập & `return_to` (SSO/OIDC tái dùng)

Backend không đổi `handleSsoStart/Callback`. Hợp đồng với FE: `GET /oauth/authorize` luôn redirect `/login?return_to=<RequestURI tương đối>`; SPA (FE-MCP-SOL-003 §Bước 4) kiểm hợp lệ `return_to` (bắt đầu `/oauth/authorize?`), giữ qua vòng SSO bằng `sessionStorage`, rồi **tự** `window.location.replace(return_to)` sau khi `fetchCurrentUser()` thành công. Lý do: `SameSite=Strict` ⇒ redirect 302 từ callback IdP không mang cookie tới `/oauth/authorize`; điều hướng do JS cùng site thì có.

---

## Hợp đồng với frontend

| Kênh/Endpoint | Lỗi → UI |
|---|---|
| `mcp.consent.get` | `MCP_CONSENT_NOT_FOUND` (kể cả đã quyết/khác user), `MCP_CONSENT_EXPIRED` ⇒ màn "yêu cầu hết hạn, quay lại ứng dụng và thử lại" |
| `mcp.consent.decide` | + `MCP_SCOPE_INVALID` (không chọn scope / ngoài phạm vi xin), `MCP_SCOPE_NOT_ALLOWED` (vượt quyền role) |
| `mcp.grant.*` | `MCP_NOT_FOUND` |
| `mcp.admin.*` | `MCP_NOT_ADMIN`, `MCP_NOT_FOUND` |
| `GET /oauth/authorize` | `302 /login?return_to=…` hoặc `302 /oauth/consent?request_id=<uuid>`; lỗi client/redirect hỏng: trang lỗi tĩnh 400 (không phải SPA) |

## Sửa TDD kèm theo

- **T3** — `auth-service.md`: §3 thêm nhóm "OAuth Authorization Server" (9 RPC trên) và ghi rõ "`IssueToken/RefreshToken/RevokeToken` vẫn chưa build; OAuth refresh là họ token riêng"; §4 domain `OAuthClient/OAuthTokenFamily/AuthCode`; §5 data (6 bảng + ngoại lệ `oauth_clients` không `tenant_id`); §7 ghi "không gọi mcp-service; `mcp-service → auth-service` là hướng duy nhất, đường login lõi không đổi"; §9 PKCE S256, reuse detection, redirect_uri exact-match, DCR rate-limit.
- **T1** — `api-gateway.md` §3 thêm `/mcp`, `/oauth/*`, `/.well-known/*`; §6 thêm `adapter/mcpauth`, `httpgateway/oauth_*`; §9 liệt kê "cookie bị từ chối tại `/mcp`" và "không token passthrough".
- **T4 (phần OAuth)** — `arch/07` bảng AuthN: thêm dòng "MCP client (OAuth 2.1, `aud`=resource)".
- **T7** — `mcp-service` sở hữu `grants/consent_requests` (đã trong `00-service-catalog.md` do BE-001).

## Kiểm thử

- Unit (`cd backend-go/services/auth-service && go test ./internal/domain/... ./internal/usecase/...`): `ValidateRedirectURI` (bảng: wildcard, `http://evil.com`, loopback port, fragment, userinfo, lệch 1 ký tự); PKCE (`plain` bị từ chối, thiếu challenge, verifier sai); `exchangeCode` (replay ⇒ family bị thu hồi, hết hạn, redirect lệch, race `used_at`); `rotateRefresh` (reuse ⇒ cả family + audit); DCR (vượt trần, tên rác, `auth_method` khác `none`).
- Integration (`go test -tags=integration ./services/auth-service/internal/adapter/postgres/... ./services/mcp-service/internal/adapter/postgres/...`): migration up→down→up; `uq_grants_active`; `DecideConsent` song song hai request chỉ một thắng.
- Gateway (`cd backend-go/services/api-gateway && go test ./internal/adapter/httpgateway/... ./internal/adapter/mcpauth/... ./internal/adapter/wscompat/...`): metadata JSON golden; `/oauth/authorize` (thiếu cookie ⇒ 302 login với `return_to` tương đối; `return_to` không chứa host ngoài; redirect_uri hỏng ⇒ 400 không redirect); cookie-only tới `/mcp` ⇒ 401; `mcpChannelError` giữ nguyên `CODE: msg`; test không-passthrough.
- Bảo mật (suite mới `…/oauth_security_test.go`): open-redirect, code replay, mix-up (`iss` có mặt và khớp), DCR spam (429), confused deputy (consent client A không dùng cho client B), thu hồi ⇒ `401` ≤ 60s (test với clock giả: resolver cache 30s).
- Rego: `make opa-test`. Proto: `make proto-gen && make proto-lint` (`buf breaking` chỉ cộng thêm).
- E2E compose (ít kịch bản): MCP Inspector → discovery → DCR → login → consent → `tools/list` (phụ thuộc BE-003/004).

## Rủi ro & phụ thuộc

- **Cơ chế "chỉ mcp-service được gọi" cho `OAuthIssueAuthCode/RevokeGrant/SetClientStatus/EnsureClient`** chưa xác minh trong mã (identity nội bộ hiện là metadata do gateway gắn). Tối thiểu: NetworkPolicy allow-list `mcp-service → auth-service`; tốt hơn: interceptor kiểm peer identity mesh. Ghi vào checklist PR.
- Hạ tầng: thiếu block nginx/ingress ⇒ toàn bộ OAuth trả HTML. Đưa vào cùng PR deploy.
- Thu hồi tin cậy phụ thuộc job reconcile + TTL cache; kill-switch toàn tenant thuộc BE-013.
- Phụ thuộc: BE-MCP-SOL-001/002 (service, proto, `MCP_ENABLED`, `dcrEnabled` từ BE-012), BE-MCP-SOL-003 (mount `/mcp`, `channels_mcp.go`), BE-MCP-SOL-006 (`ValidateMCP`, `ResolveMcpPrincipal`, `mcpscope`, claims).
- Trạng thái `git status` hiện có thay đổi dở ở `common/apperrors/apperrors.go` và nhiều `channels_*.go` (nhánh làm việc khác) — rebase trước khi cài đặt.

## Không thuộc phạm vi

UI consent / connected apps (FE-MCP-SOL-003); PAT (BE-MCP-SOL-006); Client ID Metadata Documents; client confidential; ủy quyền cho IdP ngoài (cần ADR); policy nội dung (BE-012); kill switch (BE-013).

## Liên quan

- Impact (chưa chạy — chạy trước khi sửa, báo blast radius vào PR): `gitnexus impact RefreshSession --direction upstream` (kỳ vọng LOW, chỉ tham khảo mẫu), `gitnexus impact Claims --direction upstream` ở `common/jwtauth` (kỳ vọng **HIGH**: mọi service verify JWT; chỉ thêm field omitempty nên tương thích), `gitnexus impact NewRouter --direction upstream` (kỳ vọng MEDIUM: thêm mount), `gitnexus impact Registry.Register --direction upstream` (LOW). `detect_changes` trước commit.
- Files: `backend-go/proto/orca/auth/v1/auth.proto`, `backend-go/services/auth-service/internal/{domain,usecase,adapter/postgres,adapter/mysql,adapter/grpc}/`, `backend-go/services/auth-service/migrations/{postgres,mysql}/0011_*`, `backend-go/services/api-gateway/internal/adapter/httpgateway/router.go`, `backend-go/services/api-gateway/internal/adapter/wscompat/channels_mcp.go`, `backend-go/policy/orca-authz/mcp_oauth.rego`, `deploy/dev/docker/nginx/orca.conf`.
- FE: [FE-MCP-SOL-003](../../../../../frontend/crs/v5/mcp-authorization/solutions/FE-MCP-SOL-003-consent-page-connected-apps-and-clients.md).
