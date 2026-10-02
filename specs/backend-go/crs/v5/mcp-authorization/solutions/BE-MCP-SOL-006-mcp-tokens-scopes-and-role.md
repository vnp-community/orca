# BE-MCP-SOL-006: Token MCP — audience hai chiều, mô hình scope, PAT và `Role` hiện hành cho nhánh Bearer của MCP

> **🔲 Designed — chưa implement.** Gồm 2 nhịp PR tách rời (nền tảng trước OAuth, PAT sau) — xem [README](./README.md) §3. Bám [CONTRACT-mcp-ui-api.md](../../CONTRACT-mcp-ui-api.md).

**CR:** [CR-MCP-006](../../../../../../docs/crs/v5/mcp-authorization/CR-MCP-006-mcp-tokens-scopes-and-role.md)
**Service:** `auth-service` (bảng `mcp_tokens`, RPC `IssueMcpToken/ListMcpTokens/RevokeMcpToken/ResolveMcpPrincipal`), `mcp-service` (policy PAT), `api-gateway` (`AuthValidator.ValidateMCP`, REST + 3 kênh WS), `common/` (`jwtauth.Claims`, package mới `mcpscope`)
**TDD tham chiếu:** [`auth-service.md`](../../../../tdd/services/auth-service.md) §3, §5, §9 · [`api-gateway.md`](../../../../tdd/services/api-gateway.md) §9 · `architecture/07` bảng AuthN
**CONTRACT được hiện thực:** §2.1 `mcp.token.list`, `mcp.token.create` (secret trả đúng một lần — C6), `mcp.token.revoke`; §3 `POST/GET/DELETE /v1/auth/mcp-tokens`; mã lỗi `MCP_TOKEN_TOO_LONG`, `MCP_TOKEN_LIMIT`, `MCP_SCOPE_NOT_ALLOWED`, `MCP_SCOPE_INVALID`, `MCP_NOT_FOUND`, `MCP_DISABLED`; kiểu `McpToken`, `McpScopeId`.

---

## 1. Trạng thái hiện tại (re-verify — chi tiết ở [README](./README.md) §2)

- `auth_cli_token_routes.go:13` `cliTokenAudience = "orca-cli"` cố định, không nhận từ request — đúng CR. Handler `IssueServiceToken` gắn `UserId: identity.UserID`.
- **`Identity.Role` ở nhánh Bearer: lệch so với CR.** Mã hiện tại *đã* sao chép `claims.Role` vào `Identity` (`api-gateway/internal/usecase/validate_identity.go`: `Role: claims.Role`, từ CR-RBAC-002) và `jwtauth.Claims` đã có `Role`. Nhưng **không đường phát hành JWT nào đặt `Role`**: `IssueServiceToken` (`issue_service_token.go:88`) và `CompleteDevicePairing.issueDeviceTokens` (`complete_device_pairing.go:125`) đều dựng `Claims` không có `Role` ⇒ `claims.Role` luôn rỗng trong thực tế. Comment cũ ở `wscompat/registry.go:29-35` ("never by the bearer-JWT path") đã lỗi thời nhưng hệ quả CR nêu vẫn đúng: Bearer ⇒ `Role == ""`. Theo `common/tenant.Role` (fail-closed) `""` = "không biết".
- `jwtauth.VerifyWithKey` chỉ kiểm `exp/iat/iss` — **không kiểm `aud`**; `AuthValidator.Validate` cũng không ⇒ hôm nay token `aud=orca-cli` (hay bất kỳ `aud`) đều qua REST/WS, và nếu một token `aud=<MCP>` được cấp thì REST/WS cũng nhận. Cần chặn hai chiều (§D).
- Thu hồi: `RevocationClient.IsRevoked` = 1 RPC sống mỗi request, không cache (comment nêu rõ); `issued_service_tokens` **không có `tenant_id`** và `DeactivateUser` giữ nguyên token cũ ("past sessions kept") ⇒ user bị vô hiệu hoá vẫn dùng được JWT còn hạn. MCP không được thừa hưởng hai điểm này.
- Chưa có khái niệm scope; chưa có PAT. Không tìm thấy cấu hình secret scanner (`gitleaks`/`trufflehog`) trong `.github/` ⇒ tiêu chí "secret scanner không báo" của CR **chưa xác minh được** — xem §F.
- User thuộc đúng một tenant (`auth.users.tenant_id`, unique `(tenant_id,email)`); "chọn tenant lúc consent/tạo PAT" của CR-D thực tế = tenant của phiên đăng nhập (cookie), không có lựa chọn.

### Quyết định khác/thêm so với CR gốc

1. **Không sửa ngữ nghĩa `Role` của `Validate` (REST/WS/mobile/CLI).** Thêm đường riêng `AuthValidator.ValidateMCP` lấy `Role` hiện hành từ `auth-service` (CR-C "áp riêng cho MCP trước"). Mở rộng cho Bearer chung là việc khác, cần impact + ADR.
2. **PAT là JWT RS256** (cùng khoá Transit, cùng verifier) với `aud=<resourceUrl>`, kèm tiền tố nhận diện `omp_` (chỉ để secret scanner/người dùng nhận ra; verifier bóc tiền tố). `token_sha256` được lưu theo CR (tra cứu hỗ trợ/đối soát, không dùng để xác thực — xác thực bằng chữ ký + `jti`).
3. **Đường tạo PAT đi qua `mcp-service`**, không gọi thẳng `auth-service` từ gateway: `maxTokenDays` và hạn mức PAT/user là policy tenant (BE-MCP-SOL-012) do `mcp-service` giữ; `auth-service` không gọi `mcp-service` (quy tắc §7) nên chỉ enforce **trần cứng** (90 ngày, trần scope theo role đọc từ `auth.users`) làm lớp thứ hai.
4. **Scope hiệu lực** tại gateway = `scope token ∩ CeilingForRole(role hiện hành)`; giao thêm **policy tenant** (BE-012) do `mcp-service` tính khi quyết định tool — gateway không biết policy tenant (T1/T2: gateway không chứa nghiệp vụ).
5. Lỗi hạ tầng khi tra `ResolveMcpPrincipal` ⇒ **503** (không phải 401) tại `/mcp` để client không rơi vào vòng lặp đăng nhập lại; vẫn fail-closed (không cho đi tiếp).

---

## 2. Giải pháp

### A. `common/jwtauth` — thêm claim (additive, `omitempty`)

`backend-go/common/jwtauth/jwtauth.go` (MODIFY):

```go
// MCP-only claims (BE-MCP-SOL-006): absent on every pre-existing token, so verifiers of other audiences are unaffected.
Scope    string `json:"scope,omitempty"`     // space-delimited, RFC 8693 style
ClientID string `json:"client_id,omitempty"` // OAuth access token only
GrantID  string `json:"grant_id,omitempty"`  // OAuth access token only (logical FK -> mcp.grants)
FamilyID string `json:"fid,omitempty"`       // OAuth access token only (refresh family)
TokenUse string `json:"token_use,omitempty"` // "mcp_oauth" | "mcp_pat"
```

Không đặt `Role` vào token MCP (role đọc sống, §C). `go.work` có nhiều module cùng dùng `common` ⇒ chạy `go build ./...` toàn workspace.

### B. Package `backend-go/common/mcpscope` (tên theo miền, không `utils`)

```go
package mcpscope

const (Read = "orca:read"; Write = "orca:write"; Exec = "orca:exec"; Admin = "orca:admin")

func All() []string                          // thứ tự ổn định, dùng cho metadata + consent
func Parse(space string) ([]string, error)   // dedupe; scope lạ ⇒ ErrUnknownScope (map -> MCP_SCOPE_INVALID)
func Format(scopes []string) string
func Intersect(a, b []string) []string
func CeilingForRole(role string) []string    // "admin"=All; "user"={Read,Write,Exec}; ""/lạ => nil (fail-closed)
func Implies(granted []string, required string) bool // "orca:git:write" được ngầm định bởi "orca:write"; coarse scope không suy ra lẫn nhau
```

Quy tắc: `orca:exec` **không** kéo theo `orca:write` (và ngược lại); `orca:admin` chỉ dành cho role `admin` và **không** kéo theo ba scope còn lại — token admin muốn đọc phải xin `orca:read`. Descriptor (label, description, `risk`) là dữ liệu của `mcp-service` (catalog tool, BE-MCP-SOL-007) — package này chỉ giữ ID + trần role + phép giao để gateway/auth/mcp dùng chung một luật. Test bảng ở `mcpscope/scopes_test.go` gồm ma trận role × scope.

### C. Xác thực MCP và `Role` hiện hành

**auth-service** — RPC mới (proto `auth.proto`, additive):

```proto
rpc ResolveMcpPrincipal(ResolveMcpPrincipalRequest) returns (ResolveMcpPrincipalResponse);
message ResolveMcpPrincipalRequest {
  string jti = 1; string user_id = 2; string tenant_id = 3;
  string token_use = 4;            // "mcp_oauth" | "mcp_pat"
  string family_id = 5; string grant_id = 6; string client_id = 7;   // oauth
}
message ResolveMcpPrincipalResponse {
  bool active = 1;
  string inactive_reason = 2;      // revoked|expired|user_inactive|client_blocked|grant_revoked|unknown_token
  string role = 3;                 // "admin" | "user" — đọc từ auth.users tại thời điểm gọi
}
```

`usecase/resolve_mcp_principal.go`: một lượt truy vấn kiểm — user `is_active` và `tenant_id` khớp; với `mcp_pat`: hàng `mcp_tokens` tồn tại, `revoked_at IS NULL`, `expires_at > now()`; với `mcp_oauth`: `oauth_token_families.revoked_at IS NULL`, `grant_id ∉ oauth_grant_revocations`, `oauth_client_tenant_status.status <> 'blocked'` (BE-005 §C). Với PAT lần dùng đầu: `UPDATE ... SET first_used_at = now() WHERE jti=$1 AND first_used_at IS NULL` — nếu 1 hàng bị đổi ⇒ audit `mcp_token.first_used` (kèm `client_ip` từ metadata). `last_used_at` chỉ cập nhật khi cũ hơn 5 phút (tránh ghi mỗi request). RPC **có tác dụng phụ ghi** nhưng an toàn khi gọi lặp ⇒ gateway không retry tự động.

**api-gateway** — file (đường dẫn thật đã kiểm):

| File | Thay đổi |
|---|---|
| `internal/usecase/validate_mcp_principal.go` | NEW — `McpPrincipal`, lỗi `ErrAudienceMismatch`, `ErrPrincipalInactive`, `ErrPrincipalLookupFailed`, `func (v *AuthValidator) ValidateMCP(r *http.Request, resourceURL string) (McpPrincipal, error)` |
| `internal/usecase/validate_identity.go` | MODIFY — thêm field `RejectAudiences []string` (gán sau khi dựng, giống `Revocation` — tránh đổi chữ ký `NewAuthValidator` đang có nhiều call site); trong `Validate`, sau `VerifyWithKey` từ chối nếu `aud` chứa phần tử nào của `RejectAudiences`. **Không** đụng nhánh `Role` |
| `internal/usecase/ports.go` | MODIFY — port `McpPrincipalResolver` |
| `internal/adapter/authclient/mcp_principal_resolver.go` | NEW — gọi `ResolveMcpPrincipal`, cache TTL 30s theo `jti` (positive), 5s (negative), `singleflight` chống bão cache, nhận `Invalidate(jti)` |
| `cmd/server/main.go` | MODIFY — `authValidator.RejectAudiences = []string{resourceURL}`; `authValidator.McpPrincipals = authclient.NewMcpPrincipalResolver(authClient, 30*time.Second)` |

```go
func (v *AuthValidator) ValidateMCP(r *http.Request, resourceURL string) (McpPrincipal, error) {
    raw := strings.TrimPrefix(bearerToken(r), "omp_")          // CHỈ Authorization: Bearer — cookieToken() cố tình không gọi
    if raw == "" { return McpPrincipal{}, ErrNoCredential }
    kid, err := jwtauth.KeyID(raw);            if err != nil { return McpPrincipal{}, ErrMalformedToken }
    key, err := v.jwks.PublicKey(r.Context(), kid); if err != nil { return McpPrincipal{}, ErrKeyLookupFailed }
    c, err := jwtauth.VerifyWithKey(key, raw); if err != nil { return McpPrincipal{}, ErrSignatureVerificationFailed }
    if len(c.Audience) != 1 || strings.TrimRight(c.Audience[0], "/") != strings.TrimRight(resourceURL, "/") {
        return McpPrincipal{}, ErrAudienceMismatch              // aud=orca-cli, aud thiếu, aud nhiều giá trị đều rơi vào đây
    }
    if c.TenantID == "" || c.Subject == "" || (c.TokenUse != "mcp_oauth" && c.TokenUse != "mcp_pat") {
        return McpPrincipal{}, ErrMissingIdentityClaims
    }
    res, err := v.McpPrincipals.Resolve(r.Context(), ResolveInput{JTI: c.ID, UserID: c.Subject, TenantID: c.TenantID,
        TokenUse: c.TokenUse, FamilyID: c.FamilyID, GrantID: c.GrantID, ClientID: c.ClientID})
    if err != nil { return McpPrincipal{}, ErrPrincipalLookupFailed }     // middleware -> 503
    if !res.Active || res.Role == "" { return McpPrincipal{}, ErrPrincipalInactive }
    scopes, err := mcpscope.Parse(c.Scope); if err != nil { return McpPrincipal{}, ErrMissingIdentityClaims }
    return McpPrincipal{
        Identity: Identity{TenantID: c.TenantID, UserID: c.Subject, Role: res.Role},   // Role sống, không từ claim
        Scopes:   mcpscope.Intersect(scopes, mcpscope.CeilingForRole(res.Role)),
        TokenUse: c.TokenUse, ClientID: c.ClientID, GrantID: c.GrantID, JTI: c.ID,
    }, nil
}
```

Hạ quyền admin→user có hiệu lực ≤ 30s (TTL cache) và đồng thời cắt các scope vượt trần ngay lúc đó. User `deactivate` ⇒ `user_inactive` sau ≤ 30s. Đáp ứng tiêu chí "≤ TTL cache" và "≤ 60s" của CR. Admin thật gọi tool admin được: `Identity.Role=="admin"` đi qua `AttachIdentity` như cookie; user thường gọi tool admin ⇒ do `mcp-service` trả thông điệp rõ (BE-007/012), không phải `INFRA_NOT_ADMIN` mơ hồ.

**Impact trước khi sửa (chưa chạy):**
`gitnexus impact AuthValidator.Validate --direction upstream` (kỳ vọng **HIGH**: gọi từ `authMiddleware` mọi route REST `authed` và `wscompat.Handler`/`wsbridge.Handler` fallback Bearer), `gitnexus impact Identity --direction upstream` (kỳ vọng HIGH — chỉ **thêm** `McpPrincipal` bọc ngoài, không đổi struct), `gitnexus impact Claims --direction upstream` ở `common/jwtauth` (kỳ vọng HIGH, thay đổi additive), `gitnexus impact NewAuthValidator --direction upstream` (kỳ vọng MEDIUM — chữ ký **không** đổi). Báo blast radius vào PR; thay đổi duy nhất chạm hành vi hiện có là `RejectAudiences` (mặc định rỗng ⇒ no-op khi `MCP_ENABLED=false`).

### D. Audience hai chiều (bảng chấp nhận)

| Token | `/mcp` (`ValidateMCP`) | REST `authed` + `/ws` (`Validate`) |
|---|---|---|
| `aud=<resourceUrl>` (OAuth/PAT) | ✅ nếu `Resolve.active` | ❌ `ErrAudienceNotAccepted` ⇒ 401 `UNAUTHENTICATED` |
| `aud=orca-cli` | ❌ `ErrAudienceMismatch` ⇒ 401 | ✅ (như hiện nay) |
| JWT không `aud` (mobile device) | ❌ | ✅ (như hiện nay) |
| cookie `orca_session` | ❌ không bao giờ đọc | ✅ |

Hệ quả bảo mật cần có test: một PAT **không** thể gọi `POST /v1/auth/mcp-tokens` để tự sinh PAT mới (REST từ chối `aud` MCP).

### E. PAT — auth-service

Migration `services/auth-service/migrations/postgres/0012_mcp_tokens.{up,down}.sql` (+ `mysql/0012_*`; đánh số sau `0011` của BE-005 — đối chiếu khi merge):

```sql
CREATE TABLE auth.mcp_tokens (
    jti            TEXT PRIMARY KEY,
    tenant_id      UUID NOT NULL,
    user_id        UUID NOT NULL REFERENCES auth.users (id),
    name           TEXT NOT NULL CHECK (char_length(name) BETWEEN 1 AND 80),
    scope          TEXT NOT NULL,
    token_sha256   TEXT NOT NULL UNIQUE,              -- hex SHA-256 của chuỗi secret đầy đủ ("omp_…"); không lưu secret
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at     TIMESTAMPTZ NOT NULL,
    first_used_at  TIMESTAMPTZ,
    last_used_at   TIMESTAMPTZ,
    revoked_at     TIMESTAMPTZ,
    revoked_by     UUID,
    CONSTRAINT mcp_tokens_max_90d CHECK (expires_at <= created_at + interval '90 days' AND expires_at > created_at)
);
CREATE INDEX idx_mcp_tokens_user ON auth.mcp_tokens (tenant_id, user_id, created_at DESC);
ALTER TABLE auth.mcp_tokens ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON auth.mcp_tokens USING (tenant_id = current_setting('app.tenant_id', true)::uuid);
```

Proto (additive): `IssueMcpToken(IssueMcpTokenRequest{name, repeated scopes, expires_in_days}) → IssueMcpTokenResponse{McpTokenInfo token; string secret}`, `ListMcpTokens(Empty) → {repeated McpTokenInfo}`, `RevokeMcpToken({jti})`. `user_id`/`tenant_id` **chỉ** từ metadata (`tenant.UserID(ctx)`), không có trường trong request — cùng quy ước `createUserRequestBody`. Usecase `issue_mcp_token.go`: kiểm `1 ≤ days ≤ 90` (`AUTH_MCP_TOKEN_TOO_LONG`), `scopes ⊆ CeilingForRole(users.role)` (`AUTH_MCP_SCOPE_NOT_ALLOWED`), `aud = OAUTH_RESOURCE_URL` cố định (không nhận từ request), ký qua `TokenSigner`, ghi hàng + audit `mcp_token.created` (cùng khuôn `IssueServiceToken`: audit best-effort sau khi ghi). `revoke_mcp_token.go`: `UPDATE … WHERE jti=$1 AND user_id=$caller AND revoked_at IS NULL` (không hàng ⇒ `AUTH_MCP_TOKEN_NOT_FOUND`, **cả khi thuộc người khác** — sửa điểm yếu "không verify jti thuộc user" mà `RevokeCliToken` có) + audit `mcp_token.revoked`.

### F. PAT — mcp-service, REST và kênh WS

`mcp-service` usecase `create_mcp_token.go` / `list_mcp_tokens.go` / `revoke_mcp_token.go` (policy; gọi auth-service sau):

1. `MCP_ENABLED`/kill-switch tenant ⇒ `MCP_DISABLED`/`MCP_KILL_SWITCH_ACTIVE`.
2. `mcpscope.Parse` ⇒ lạ ⇒ `MCP_SCOPE_INVALID`; ∅ ⇒ `MCP_SCOPE_INVALID`.
3. `expiresInDays > min(90, tenant.maxTokenDays)` hoặc `< 1` ⇒ **`MCP_TOKEN_TOO_LONG`** (message nêu trần: `"MCP_TOKEN_TOO_LONG: maximum is 30 days for this organization"`).
4. Đếm PAT `active` của user (gọi `ListMcpTokens`) `≥ MCP_PAT_MAX_PER_USER` (mặc định 20) ⇒ **`MCP_TOKEN_LIMIT`**.
5. Gọi `auth.IssueMcpToken`; `AUTH_MCP_SCOPE_NOT_ALLOWED` ⇒ **`MCP_SCOPE_NOT_ALLOWED`**.
6. Outbox `orca.mcp.token.created {token_id, user_id, scopes, expires_at}` (không có secret) / `orca.mcp.token.revoked {token_id, by}`.

Gateway:

- `httpgateway/auth_mcp_token_routes.go` NEW — `mountMcpTokenRoutes(authed, mcpClient)` trong group `authed` (cạnh `mountCliTokenRoutes`), `POST/GET /v1/auth/mcp-tokens`, `DELETE /v1/auth/mcp-tokens/{id}` (`{id}` = `jti`). Body `POST`: `{"name","scopes","expires_in_days"}`; **không** có `user_id`/`audience`. Trả `201 {"token": McpToken(JSON snake_case như cli-tokens), "secret": "omp_…"}`; `Cache-Control: no-store`. Kiểu dữ liệu tương đương `mcp.token.*` (CONTRACT §3). Bearer `aud=orca-cli` của CLI gọi được route này; PAT thì không (§D).
- `wscompat/channels_mcp.go` (MODIFY) thêm `mcp.token.list`, `mcp.token.create({name,scopes,expiresInDays})` → `{token, secret}`, `mcp.token.revoke(tokenId)` → `{ok:true}`. Bọc lỗi bằng `mcpChannelError` (xem BE-005 §G). **Secret chỉ nằm trong ResultMessage của lần gọi này**: không log args/result của `mcp.token.create` (thêm vào danh sách kênh bị che trong trace/log của wscompat — chưa xác minh tên cơ chế; kiểm khi cài đặt), không cache, không phát sự kiện.
- Mapping `McpToken`: `status` = `revoked` nếu `revoked_at`, `expired` nếu `expires_at < now()`, còn lại `active`; `scopes` mảng `McpScopeId`.

Secret scanner: tiền tố `omp_` cho phép thêm rule (regex `omp_[A-Za-z0-9_-]{20,}\.[A-Za-z0-9_-]{20,}\.[A-Za-z0-9_-]{20,}`) vào scanner của CI **khi có** — hiện repo chưa có cấu hình (chưa xác minh); mở task riêng ở BE-MCP-SOL-015.

### G. Audit

| Sự kiện | Nơi ghi | Nội dung (không bao giờ có token) |
|---|---|---|
| `mcp_token.created` | auth-service `AuditRepository` | actor=user, `jti`, scopes, `expires_at` |
| `mcp_token.first_used` | auth-service (trong `ResolveMcpPrincipal`) | `jti`, `client_ip` |
| `mcp_token.revoked` | auth-service | `jti`, `by` |
| `orca.mcp.token.*` | outbox của mcp-service → BE-013 hợp nhất `McpAuditEntry` | như trên |

---

## Hợp đồng với frontend

| Kênh | Lỗi → UI (FE-MCP-SOL-004) |
|---|---|
| `mcp.token.create` | `MCP_TOKEN_TOO_LONG`, `MCP_SCOPE_NOT_ALLOWED`, `MCP_SCOPE_INVALID`, `MCP_TOKEN_LIMIT`, `MCP_KILL_SWITCH_ACTIVE`, `MCP_DISABLED` |
| `mcp.token.list` / `.revoke` | `MCP_NOT_FOUND` (id lạ hoặc của người khác) |

`McpServerInfo.maxTokenDays` (BE-MCP-SOL-003 điền) phải bằng `min(90, tenant.maxTokenDays)` để FE giới hạn ô nhập đúng với kiểm tra phía server.

## Sửa TDD kèm theo

- **T4** — `auth-service.md` §3 (thêm nhóm "MCP tokens": 4 RPC), §5 (bảng `mcp_tokens` + CHECK 90 ngày), §9 (ngoại lệ PAT có tài liệu: hash SHA-256, `jti`, thu hồi tức thì qua `ResolveMcpPrincipal`, audit tạo/thu hồi/dùng đầu, secret hiển thị một lần; verifier MCP **bắt buộc** `aud == resourceUrl`, verifier REST/WS **từ chối** `aud` MCP); `arch/07` bảng AuthN thêm dòng "MCP PAT".
- **T1** — `api-gateway.md` §9: mô tả `ValidateMCP` + cache `ResolveMcpPrincipal` 30s (độ trễ thu hồi tối đa) và quy tắc "gateway không đọc cookie ở `/mcp`".
- **T7** — `mcp-service` giữ policy PAT (`maxTokenDays`, hạn mức).
- Sửa comment lỗi thời `wscompat/registry.go` (Identity.Role) khi cài đặt.

## Kiểm thử

- Unit: `cd backend-go/common && go test ./jwtauth/... ./mcpscope/...` (ma trận role × scope, `Implies`, `Parse` lạ); `cd backend-go/services/api-gateway && go test ./internal/usecase/... ./internal/adapter/authclient/... ./internal/adapter/httpgateway/... ./internal/adapter/wscompat/...`: bảng `ValidateMCP` (aud đúng/sai/nhiều giá trị/thiếu, cookie-only, token_use lạ, resolver lỗi ⇒ 503, role `""` ⇒ inactive, hạ quyền sau TTL cache với clock giả); `Validate` từ chối `aud` MCP nhưng vẫn nhận `orca-cli`; `POST /v1/auth/mcp-tokens` bằng PAT ⇒ 401.
- `cd backend-go/services/auth-service && go test ./internal/usecase/...`: `IssueMcpToken` (91 ngày, 0 ngày, scope vượt role, `aud` không nhận từ input), `RevokeMcpToken` (jti người khác ⇒ not found), `ResolveMcpPrincipal` (revoked/expired/inactive user/first-use đúng 1 lần ⇒ đúng 1 audit).
- Integration: `go test -tags=integration ./services/auth-service/internal/adapter/postgres/...` (migration up→down→up, CHECK 90 ngày, RLS).
- Tiêu chí CR: PAT tạo→dùng→thu hồi ⇒ `401` ≤ 60s (test e2e với resolver TTL rút ngắn); hết hạn ⇒ `401`; scope `orca:read` không gọi được tool `orca:write` (bảng scope × tool nằm ở BE-007; BE-006 cung cấp `Implies`); không log PAT/JWT (test `slog` handler ghi vào buffer sau khi gọi `mcp.token.create`, khẳng định không chứa `omp_`).
- Proto: `make proto-gen && make proto-lint`.

## Rủi ro & phụ thuộc

- **Thay đổi `common/jwtauth.Claims` chạm mọi module** (additive nên an toàn, nhưng build cả `go.work`).
- Cache 30s đồng nghĩa trần độ trễ thu hồi 30s; ai cần "ngay lập tức" dùng kill-switch (BE-013) hoặc `Invalidate` trên cùng replica.
- `ResolveMcpPrincipal` nằm trên đường nóng của `/mcp` (mỗi `jti` 1 lần/30s) — thêm metric `orca_mcp_principal_resolve_total{result}` và SLO p99 < 20ms như `ValidateSession` (`auth-service.md` §8).
- Phụ thuộc: BE-005 (claim/bảng OAuth, `OAUTH_RESOURCE_URL`), BE-MCP-SOL-002/003 (`MCP_ENABLED`, `maxTokenDays` trong `McpServerInfo`), BE-012 (tenant `maxTokenDays`).

## Không thuộc phạm vi

UI PAT (FE-MCP-SOL-004); bảng scope × tool và scope mịn theo tool (BE-007/008); nội dung policy tenant (BE-012); mở rộng `Role` sống cho Bearer chung của mobile/CLI; kênh admin liệt kê PAT của người khác (CONTRACT không có).

## Liên quan

`backend-go/common/jwtauth/jwtauth.go`, `backend-go/common/mcpscope/` (mới), `backend-go/services/api-gateway/internal/usecase/{validate_identity.go,validate_mcp_principal.go,ports.go}`, `backend-go/services/api-gateway/internal/adapter/{authclient/mcp_principal_resolver.go,httpgateway/auth_mcp_token_routes.go,wscompat/channels_mcp.go}`, `backend-go/services/auth-service/{internal/usecase,migrations/*/0012_mcp_tokens.*}`, `backend-go/services/api-gateway/cmd/server/main.go`. FE: [FE-MCP-SOL-004](../../../../../frontend/crs/v5/mcp-authorization/solutions/FE-MCP-SOL-004-access-tokens-pat-ui.md).
