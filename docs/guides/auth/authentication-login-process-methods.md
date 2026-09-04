# Quy trình & Phương pháp Xác thực Đăng nhập vào Orca

**Tổng hợp từ:** `docs/flows/logic/auth.md`, `docs/flows/code/auth/*.md`, `docs/logic/auth/BL-AUTH-0*.md`,
`docs/features/F22/F23/F24-*.md`, `docs/crs/v1/login/CR-LOGIN-00*.md`, `docs/hld/v1/security.md`
(bản cập nhật 2026-08-14, đã đối chiếu code).

> **Lưu ý về độ tin cậy tài liệu:** Orca có 2 thế hệ tài liệu auth chồng lên nhau — (1) tài liệu mô
> tả **PairCode + E2EE** như cơ chế "duy nhất" (viết trước khi F23 ra đời, ví dụ
> `docs/flows/code/auth/account-management.md` mục 9 "Chưa có: User accounts, SSO, Persistent
> sessions"), và (2) tài liệu mô tả **Local Login email/password** sau khi CR-LOGIN-001 (F23)
> triển khai Phase 1 ngày 2026-07-24. Cả hai cơ chế **cùng tồn tại** trong code — không cái nào thay
> thế cái nào (xem bảng "Hai auth mode song song" ở mục 4). `docs/hld/v1/security.md` là bản đối
> chiếu code mới nhất (2026-08-14) và đáng tin cậy nhất cho phần đã ✅ implemented; các mục 🚧/❌
> trong đó là thiết kế đề xuất, **chưa có trong code**.
>
> **Toàn bộ tài liệu này mô tả backend Node/TS cũ** (`backend/`). Kể từ khi hệ thống chuyển sang
> **backend-go** (`backend-go/services/auth-service` + `api-gateway`, deploy thật tại
> `b15.openledger.vn`), **SSO/OAuth2 KHÔNG còn là "⏳ Deferred Phase 2, endpoint 501"** như mục 5/10/11
> bên dưới còn ghi (đúng cho backend TS, sai cho backend-go) — GitHub/Google/OIDC đã triển khai thật
> qua CR-LOGIN-001. Xem [`google-sso-setup.md`](./google-sso-setup.md) (hướng dẫn setup Google SSO)
> và `backend-go/services/auth-service/README.md`'s mục SSO (chi tiết implementation) — đừng dựa vào
> các dòng "Chưa dùng được"/"501" bên dưới cho backend-go.

---

## 1. Tổng quan — các cơ chế xác thực song song

Orca không có một cổng đăng nhập duy nhất. Cơ chế xác thực phụ thuộc vào **client** và **mode** server đang chạy:

| # | Cơ chế | Dùng cho | Trạng thái |
|---|--------|----------|------------|
| 1 | **PairCode + E2EE** (device pairing) | Desktop (Electron), CLI, Mobile app kết nối vào 1 Orca runtime | ✅ Implemented — cơ chế gốc, không có user identity |
| 2 | **Local Login (email + password)** | Web browser khi Orca chạy Web Server Mode multi-user (`ORCA_MULTI_USER=1`) | ✅ Implemented Phase 1 (2026-07-24) |
| 3 | **SSO / OAuth2 (GitHub, Google)** | Web browser, tổ chức có IdP ngoài | ⏳ Deferred Phase 2 — endpoint trả `501` |
| 4 | **OIDC (Keycloak/generic)** | Web browser, enterprise SSO | ⏳ Deferred Phase 3 — chưa verify JWT/JWKS |
| 5 | **Agent WebSocket Auth** | Dev Server Agent tự kết nối vào Orca (không phải người dùng login) | ✅ Implemented — token tĩnh, không phải "login" theo nghĩa người dùng |
| 6 | **Gateway↔Agent Signed Context (HMAC)** | Orca Server ký context RPC gửi cho Agent | 🚧 Proposed — chưa có trong code (ADR-015/017/018/019) |

Mục 2–4 là các cơ chế **login của người dùng cuối**; mục 5–6 là auth **giữa các thành phần hệ thống** (đưa vào tài liệu này để tránh nhầm lẫn phạm vi).

---

## 2. Cơ chế 1 — PairCode + E2EE (Desktop/CLI/Mobile)

**Nguồn:** `docs/flows/code/auth/authentication.md`, `docs/flows/code/auth/account-management.md`

### 2.1 Bản chất

Đây là cơ chế xác thực **device-centric**, không có khái niệm "user account". Mỗi lần pairing tạo ra một `DeviceEntry` — một cặp credential giữa client và server. Không có login/SSO ở tầng này.

### 2.2 PairingOffer — credential được cấp phát

```typescript
export type PairingOffer = {
  v: 2
  endpoint: string        // "wss://b15.openledger.vn"
  deviceToken: string     // 48 ký tự hex (24 random bytes)
  publicKeyB64: string    // Curve25519 public key của server, base64
  scope?: 'mobile' | 'runtime'   // 'runtime' = CLI/web, 'mobile' = điện thoại
}
```

Encode thành URL: `orca://pair?code=<base64url(JSON)>` (web client cũng chấp nhận bare base64 hoặc `https://.../#pairing=<base64>`).

### 2.3 Quy trình bắt tay E2EE (state machine)

```
Client (Browser/Mobile/CLI)         Orca Server
             │                              │
             │── WS connect ───────────────►│  state = awaiting_hello
             │── e2ee_hello {publicKeyB64} ►│  ECDH: sharedKey = X25519(serverSecret, clientPub)
             │                              │  state → awaiting_auth (timeout 10s)
             │◄─ e2ee_ready (plaintext) ────│
             │── e2ee_auth (encrypted) ────►│  { deviceToken }
             │                              │  decrypt → validateToken(deviceToken)
             │                              │  FAIL → close(4003, 'auth failed')
             │                              │  OK   → state = ready
             │  ── mọi RPC sau đây đều encrypted (XChaCha20-Poly1305) ──
             │── { authToken, method, ... }►│  decrypt → handle → encrypt reply
             │◄─ { result/error } ──────────│
```

- **Key exchange:** Curve25519 ECDH (X25519)
- **Encryption:** XChaCha20-Poly1305 (AEAD)
- **Nonce:** random 24 byte/message
- **Timeout handshake:** 10s; đóng kết nối sau 5 lần decrypt fail liên tục

### 2.4 Ba loại authToken (sau khi channel `ready`)

| Loại | Nguồn | Scope | TTL | Lưu trữ |
|------|-------|-------|-----|---------|
| `runtime authToken` | `randomBytes(24)` khi khởi động | Toàn quyền (mọi method) | Vòng đời process | In-memory |
| `deviceToken` | `DeviceRegistry`, 48-hex | Theo từng device | Vĩnh viễn (đến khi revoke) | `orca-devices.json` (chmod 600) |
| `ScopedPairingToken` | `DeviceRegistry.generateScopedToken()` | RBAC-limited | 24h | In-memory (mất khi restart) |

### 2.5 Vòng đời Device: tạo → pairing → active → revoke

```
[1. Tạo]    createPairingOffer() → getOrCreatePendingDevice() → PairCode (QR/URL)
[2. Pairing] Client dùng PairCode → E2EE handshake → e2ee_auth{deviceToken}
             → validateToken() OK → updateLastSeen() → ACTIVE
[3. Active]  Mỗi lần reconnect: E2EE auth lại với cùng deviceToken
[4. Revoke]  removeDevice(deviceId) → xoá khỏi orca-devices.json
             → terminateClientConnections(token) → mọi WS đang dùng token bị đóng (4401)
```

### 2.6 Unix Socket Auth (local Electron/CLI)

Trên local, `authToken` lấy từ file `orca-runtime.json` (`{ socketPath, authToken }`, permission 600) — bảo vệ bằng file permission OS, **không** phải mã hoá tầng ứng dụng (yếu hơn E2EE có chủ đích, vì socket local đã có OS bảo vệ).

---

## 3. Cơ chế 2 — Local Login Email/Password (Web Server Mode, Multi-User)

**Nguồn:** F23, CR-LOGIN-001, `docs/flows/code/auth/multi-user-session.md`, `docs/logic/auth/BL-AUTH-01/02`

### 3.1 Điều kiện kích hoạt

- Server chạy với `ORCA_MULTI_USER=1`
- Database đã apply migration 0004/0005 (`orca_users`, `orca_sessions`/`orca_workspace_sessions` tồn tại)
- Nếu `ORCA_MULTI_USER=0` → route `/login`, `/auth/*` trả 404, dùng PairCode như bình thường (single-user)

### 3.2 Quy trình đăng nhập chi tiết

```
Browser              Express /auth/local        AuthManager         DB (orca_users/orca_sessions)
  │ GET /login              │                        │                       │
  │◄── HTML login form ─────│                        │                       │
  │ POST /auth/local        │                        │                       │
  │ { email, password } ───►│                        │                       │
  │                         │── login(email, pw) ───►│                       │
  │                         │                        │ SELECT * FROM orca_users
  │                         │                        │ WHERE email=? AND is_active=1
  │                         │                        │──────────────────────►│
  │                         │                        │◄───────────────────────│
  │                         │                        │ bcrypt.compare(pw, hash, 12 rounds)
  │                         │                        │ [~100-300ms, timing-safe]
  │                         │                        │ FAIL → { error: 'invalid_credentials' } (401)
  │                         │                        │ OK   → token = randomBytes(32).hex (64 ký tự)
  │                         │                        │        INSERT orca_sessions
  │                         │                        │        (token, userId, expires_at=now+8h)
  │                         │                        │──────────────────────►│
  │◄── Set-Cookie: orca_session=<token>; HttpOnly; Secure; SameSite=Strict/Lax; Max-Age=28800
  │◄── 200 { id, email, name, role }
  │
  │ WS ws://:6768/ + Cookie: orca_session=<token>
  │ ─────────────────────────────────────────────► WsSessionRouter.onConnection()
  │                                                  → validateSession(token) (JOIN orca_users)
  │                                                  → getOrCreateUserRuntime(userId) [fork/reuse]
  │◄════════════ JSON-RPC (scope theo userId) ══════► OrcaRuntimeRpcServer (isolated)
```

**Không phân biệt lý do lỗi:** "email không tồn tại" và "sai password" trả cùng một lỗi
`401 invalid_credentials` (chống dò email hợp lệ).

### 3.3 Database schema

```sql
CREATE TABLE orca_users (
  id            TEXT PRIMARY KEY,
  email         TEXT UNIQUE NOT NULL,
  name          TEXT NOT NULL,
  role          TEXT DEFAULT 'developer',   -- developer | lead | admin
  password_hash TEXT,                        -- bcrypt 12 rounds; NULL nếu SSO-only
  provider      TEXT DEFAULT 'none',         -- none | github | google | keycloak
  provider_user_id TEXT,
  is_active     INTEGER DEFAULT 1,
  department_id TEXT,                        -- [v5.0]
  profile_json  TEXT DEFAULT '{}',           -- [v5.0]
  created_at    INTEGER, updated_at INTEGER, last_login_at INTEGER
);

CREATE TABLE orca_sessions (            -- (tên bảng thực tế: orca_workspace_sessions ở migration 0003)
  token/session_id TEXT PRIMARY KEY,    -- randomBytes(32).hex = 64 ký tự
  user_id       TEXT REFERENCES orca_users(id) ON DELETE CASCADE,
  device_name   TEXT,
  expires_at    INTEGER NOT NULL,       -- created_at + 8h
  last_seen_at  INTEGER,
  created_at    INTEGER NOT NULL
);
-- Index: idx_sessions_user(user_id), idx_sessions_expires(expires_at)

CREATE TABLE orca_audit_log (           -- append-only, không UPDATE/DELETE
  id TEXT PRIMARY KEY, actor_id TEXT REFERENCES orca_users(id),
  action TEXT NOT NULL, target TEXT, metadata TEXT DEFAULT '{}',
  timestamp INTEGER NOT NULL, ip TEXT
);
```

### 3.4 Session validation middleware (`requireAuth`)

Chạy trên **mọi** HTTP request và mọi WS upgrade khi multi-user mode bật:

```
1. Lấy cookie orca_session từ request
2. SELECT session JOIN orca_users WHERE token=? AND expires_at>now AND is_active=1
3. Nếu invalid → 401 Unauthorized (HTTP) hoặc WS close(4401)
4. Sliding expiry: UPDATE last_seen_at=now, expires_at=now+8h  (mỗi request "nới" TTL thêm)
5. Inject req.userId, req.userRole vào request/context tiếp theo
```

`requireAdmin()` = `requireAuth()` + kiểm tra `role === 'admin'`, áp cho toàn bộ `/admin/api/*`.

### 3.5 Vòng đời session — login/logout/expiry/revoke

```
[Login]   POST /auth/local → bcrypt verify → INSERT orca_sessions → Set-Cookie 8h
[Active]  Mỗi request → validateSession() → sliding expiry (+8h mỗi lần)
[Logout]  POST /auth/logout → DELETE orca_sessions WHERE token=? → Set-Cookie Max-Age=0
                            → đóng các WS connection đang mở của user đó
[Expiry]  Background job mỗi 1h (hoặc 30 phút tuỳ tài liệu) → DELETE WHERE expires_at<now
[Admin]   DELETE /admin/api/sessions/:id → xoá session + đóng WS + audit log 'session.revoke'
```

### 3.6 Bảng lỗi

| Tình huống | HTTP Code | Response |
|---|---|---|
| Email không tồn tại | 401 | `{ error: "invalid_credentials" }` |
| Password sai | 401 | `{ error: "invalid_credentials" }` |
| Account bị deactivate | 403 | `{ error: "account_inactive" }` |
| `ORCA_MULTI_USER=0` | 404 | — |
| Vượt rate limit (thiết kế, **chưa implement**) | 429 | `{ error: "too_many_attempts" }` |

### 3.7 `GET /auth/me` và `POST /auth/logout`

- `GET /auth/me` → `{ id, email, name, role, provider }` (thông tin user hiện tại từ session)
- `POST /auth/logout` → revoke session, clear cookie

---

## 4. Hai auth mode song song (không loại trừ nhau)

| Mode | Transport | Cơ chế xác thực | TTL |
|---|---|---|---|
| **Desktop / Web Pairing** (mục 2) | WebSocket E2EE | `deviceToken` (48-hex) trong `e2ee_auth` | Vĩnh viễn đến khi revoke |
| **Web Server Mode multi-user** (mục 3) | HTTP cookie + WS | `orca_session` cookie (64-hex) | 8h sliding |

Sau khi login qua web, client có **cả hai** credential song song:

```typescript
type AuthenticatedContext = {
  deviceToken:  string   // E2EE pairing (giữ nguyên, không đổi)
  sessionToken: string   // HTTP session cookie (mới, từ login)
  userId, userEmail, role: ...
}
```

---

## 5. Cơ chế 3 & 4 — SSO / OIDC (⏳ Deferred, chưa hoàn thiện)

Thiết kế đã có type/route stub nhưng **chưa hoạt động**:

```
GET /auth/sso/:provider   → redirect đến IdP (GitHub/Google/Keycloak)   [Phase 2 — hiện trả 501]
GET /auth/callback        → exchange code → upsert user → session       [Phase 2 — chưa làm]
OIDC (Keycloak/generic)   → verify JWT qua JWKS endpoint                [Phase 3 — chưa làm]
```

```typescript
export type OrcaIdentityProvider = 'github' | 'google' | 'keycloak' | 'none'
export type OrcaSsoConfig = {
  provider: OrcaIdentityProvider
  clientId: string
  discoveryUrl?: string   // OIDC — tự fetch jwks_uri, issuer, token_endpoint
  allowedOrg?: string     // GitHub org whitelist
  allowedDomain?: string  // Google domain whitelist
}
```

Config dự kiến qua env: `AUTH_MODE=local|sso|both`, `SSO_GITHUB_CLIENT_ID/SECRET`, `SSO_GOOGLE_*`,
`SSO_OIDC_DISCOVERY_URL`. PairCode vẫn hoạt động song song kể cả khi SSO bật (dùng cho Desktop app,
CI/automation, offline).

---

## 6. Per-User Sandbox sau khi Login (F24)

**Nguồn:** `docs/flows/code/auth/multi-user-session.md` mục 4

Sau khi `orca_session` hợp lệ, `WsSessionRouter` route WS connection sang một **runtime cô lập theo userId**:

```typescript
class WsSessionRouter {
  private userRuntimes = new Map<string, OrcaRuntimeRpcServer>()

  onConnection(ws, req) {
    const user = validateSession(extractCookie(req, 'orca_session'))
    if (!user) return ws.close(4401, 'Unauthorized')
    getOrCreateUserRuntime(user.id).handleWebSocketConnection(ws, user)
  }
}
```

Mỗi user có: PTY sessions riêng, worktrees theo project họ là thành viên, file path riêng
(`~/.orca/users/<userId>/`). Dùng chung: DB pool (lọc theo userId), RelayConnectionPool (SSH,
gắn userId vào context), FleetHealthMonitor, AgentWebSocketServer.

---

## 7. Phân quyền sau đăng nhập (RBAC)

| Role | Cấp độ |
|---|---|
| `developer` | Mặc định |
| `lead` | Team lead |
| `admin` | Toàn quyền, truy cập `/admin/api/*` |

**Guard functions:**
```
requireAuth()          → validate cookie → inject userId, role
requireAdmin()         → requireAuth() + role === 'admin' → 403 nếu không
requireOwnerOrAdmin()  → dùng ở RPC layer (project) — check owner hoặc admin
```

**⚠️ Gap đã biết (theo `docs/hld/v1/security.md` §8.3):** RBAC hiện **phân mảnh trên 4 cơ chế độc
lập, không đồng nhất** — HTTP `requireAdmin` middleware, RPC `requireAdmin`, RPC
`requireOwnerOrAdmin`, và `resolveUserPermissions()` ở fleet level (`shared/rbac-types.ts`).
Không có một hàm `hasPermission(role, resource, action)` làm nguồn chân lý duy nhất (BUG-BE-HLD-003,
còn mở). Một lỗ hổng bypass quyền tại RPC layer (`profile.updateCompany`, `project` ownership...) đã
được vá ngày 2026-08-09 (BUG-BE-HLD-001/002) — các guard RPC trước đó chỉ check "đã login", không
check đúng role admin.

---

## 8. Audit Log

Mọi hành động quan trọng ghi vào `orca_audit_log` (append-only, không có API xoá):

```sql
INSERT orca_audit_log { id, actor_id, action, target_type, target_id, metadata, ip_address, created_at }
```

Actions tiêu biểu: `login.success`, `login.fail`, `logout`, `user.create`, `user.update`,
`user.deactivate`, `session.kill`/`session.revoke`, `server.connect/disconnect`,
`agent.spawn/stop`, `worktree.create/delete`, `ai_provider.credential.write`.

Truy vấn (admin only): `GET /admin/api/audit?action=&from=&page=`; export CSV:
`GET /admin/api/audit/export?format=csv`.

---

## 9. Cơ chế phụ trợ (không phải "login" người dùng)

Hai mục dưới đây **không phải** quy trình login của người dùng cuối, nhưng thuộc "auth" nói chung
và hay bị nhầm lẫn phạm vi — liệt kê ở đây để phân biệt rõ:

### 9.1 Agent WebSocket Auth (✅ implemented) — dev-server agent tự xác thực với Orca

```
Orca → HTTP Upgrade ws://agent:6799/orca-relay, Header: Authorization: Bearer <agentToken>
Agent → HTTP Upgrade ws://orca:6769/agent, gửi trước: agent.handshake { agentToken, capabilities }
        Orca validate agentToken (SHA-256 hash lookup, in-memory) → OK / AuthFailed + close(1008)
```

Issuance: self-service `POST /api/agent-token` với `Authorization: Bearer <ORCA_AGENT_API_SECRET>`
(một secret tĩnh dùng chung, **không phải** credential cấp riêng từng kết nối qua Admin UI). Token
dạng đoán được (`agt-<devServerId>-<timestamp>`), không có bảng DB, không có API revoke từng token —
chỉ có thể vô hiệu hoá toàn bộ bằng cách xoay `ORCA_AGENT_API_SECRET`. Coi secret này như credential
root.

### 9.2 Gateway ↔ Agent Signed Execution Context (🚧 Proposed — CHƯA có trong code)

`docs/adrs/v1/ADR-015`, `docs/adrs/v2/ADR-017/018/019` mô tả một cơ chế HMAC-SHA256 signed context,
TTL 30s, cho mỗi RPC Gateway gửi Agent. **Xác nhận trực tiếp qua code** (`agent/src/relay/context.ts`):
không tồn tại `ContextVerifier`, `SignedExecutionContext`, hay `_ctx` nào trong `agent/src`. Đây là
thiết kế mục tiêu, chưa triển khai — cơ chế thật sự đang chạy là mục 9.1 ở trên.

---

## 10. Bảo mật — tóm tắt & các lỗ hổng đã biết

| Property | Cơ chế | Mức độ |
|---|---|---|
| Password hashing | bcrypt 12 rounds | Mạnh |
| Session token | `randomBytes(32)` = 64-hex (256-bit entropy) | Mạnh |
| Cookie | `HttpOnly; SameSite=Strict/Lax; Secure` | Mạnh |
| Session TTL | 8h sliding window | Vừa |
| E2EE (PairCode) | Curve25519 ECDH + XChaCha20-Poly1305 | Mạnh (nhưng không Forward Secrecy — server keypair persistent) |
| User isolation | Fork runtime riêng theo userId | Mạnh |
| Audit trail | append-only `orca_audit_log` | Có |
| **Rate limiting login** | ❌ Chưa implement (thiết kế: 10 lần/phút/IP) | Thiếu |
| **2FA** | ❌ Chưa implement (`profile.security.require2FA` tồn tại field nhưng backend không đọc) | Thiếu |
| **SSO/OIDC** | ⏳ Deferred Phase 2/3, endpoint stub 501 | Chưa xong |
| **RBAC thống nhất** | ❌ Phân mảnh 4 cơ chế, không có `hasPermission()` chung | Gap đang mở |
| **Agent token revoke từng cái** | ❌ Không có, chỉ xoay được secret toàn hệ thống | Gap đang mở |
| **Replay prevention (E2EE)** | ❌ Không có nonce tracking | Thiếu |

---

## 11. Bảng tổng hợp nhanh: "Tôi login bằng gì?"

| Tôi là... | Tôi login bằng... | File/RPC liên quan |
|---|---|---|
| Người dùng Desktop app (Electron), local, single-user | Không cần login — chỉ chạy app | — |
| Người dùng CLI/Web pairing vào 1 Orca runtime | Quét/nhập PairCode → E2EE handshake tự động | `pairing.ts`, `e2ee-channel.ts` |
| Người dùng Mobile app | Quét QR (chứa PairingOffer) | `mobile.getPairingQR`, `device-registry.ts` |
| Người dùng Web browser, Orca chạy `ORCA_MULTI_USER=1` | Form email/password tại `/login` → `POST /auth/local` | `auth-router.ts`, `auth-manager.ts` |
| Người dùng muốn dùng GitHub/Google/Keycloak để login | **Chưa dùng được** — deferred Phase 2/3 | `CR-LOGIN-001` §7 |
| Admin quản lý user/session | `/admin/api/*` sau khi đã login với role `admin` | `admin-routes.ts` |
| Dev Server Agent kết nối vào Orca | `ORCA_AGENT_API_SECRET` → `agent.handshake` RPC | §9.1 ở trên |

---

## 12. Nguồn tài liệu

| File | Nội dung |
|---|---|
| [`docs/flows/logic/auth.md`](../../flows/logic/auth.md) | Luồng dữ liệu tổng quan BL-AUTH-01→05 |
| [`docs/flows/code/auth/authentication.md`](../../flows/code/auth/authentication.md) | PairCode + E2EE handshake chi tiết |
| [`docs/flows/code/auth/session-management.md`](../../flows/code/auth/session-management.md) | WS connection/tab/device session lifecycle |
| [`docs/flows/code/auth/multi-user-session.md`](../../flows/code/auth/multi-user-session.md) | Local login + per-user sandbox chi tiết |
| [`docs/flows/code/auth/account-management.md`](../../flows/code/auth/account-management.md) | DeviceRegistry, ScopedPairingToken |
| [`docs/logic/auth/BL-AUTH-01-local-login.md`](../../logic/auth/BL-AUTH-01-local-login.md) | Business logic — local login |
| [`docs/logic/auth/BL-AUTH-02-session-management.md`](../../logic/auth/BL-AUTH-02-session-management.md) | Business logic — session |
| [`docs/logic/auth/BL-AUTH-03-per-user-sandbox.md`](../../logic/auth/BL-AUTH-03-per-user-sandbox.md) | Business logic — sandbox |
| [`docs/logic/auth/BL-AUTH-04-admin-user-crud.md`](../../logic/auth/BL-AUTH-04-admin-user-crud.md) | Business logic — admin CRUD |
| [`docs/logic/auth/BL-AUTH-05-audit-log.md`](../../logic/auth/BL-AUTH-05-audit-log.md) | Business logic — audit log |
| [`docs/features/F22-web-server-mode.md`](../../features/F22-web-server-mode.md) | Feature spec — Web Server Mode |
| [`docs/features/F23-multi-user-auth.md`](../../features/F23-multi-user-auth.md) | Feature spec — Multi-User Auth |
| [`docs/features/F24-per-user-sandbox.md`](../../features/F24-per-user-sandbox.md) | Feature spec — Per-User Sandbox |
| [`docs/crs/v1/login/CR-LOGIN-001-auth.md`](../../crs/v1/login/CR-LOGIN-001-auth.md) | Change request — Login/SSO, implementation status |
| [`docs/crs/v1/login/README.md`](../../crs/v1/login/README.md) | Tổng quan 4 CR Login/Sandbox/SSH/Admin |
| [`docs/hld/v1/security.md`](../../hld/v1/security.md) | Security architecture, đối chiếu code 2026-08-14 |
| [`docs/adrs/v1/ADR-015-signed-execution-context-gateway-agent.md`](../../adrs/v1/ADR-015-signed-execution-context-gateway-agent.md) | Thiết kế đề xuất (chưa implement) Gateway↔Agent |
