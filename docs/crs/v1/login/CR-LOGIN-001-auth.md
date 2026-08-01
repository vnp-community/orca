# CR-LOGIN-001 — Authentication: Login / SSO bên cạnh PairCode

| Field | Value |
|-------|-------|
| **CR ID** | CR-LOGIN-001 |
| **Tên** | Authentication: Login / SSO bên cạnh PairCode |
| **Ưu tiên** | P0 — Prerequisite cho tất cả CR còn lại |
| **Effort** | L (2–3 sprints) |
| **Blocked by** | — |
| **Blocks** | CR-LOGIN-002, CR-LOGIN-004 |
| **Status** | ✅ Phase 1 Done (2026-07-24) — SSO OAuth & OIDC deferred Phase 2/3 |

---

## 1. Vấn đề hiện tại

### Cơ chế xác thực hiện tại

```typescript
// src/shared/rbac-types.ts — đã có type nhưng chưa implement
export type OrcaIdentityProvider = 'github' | 'google' | 'keycloak' | 'none'
export type OrcaSsoConfig = {
  provider: OrcaIdentityProvider
  clientId: string
  discoveryUrl?: string   // OIDC
  allowedOrg?: string     // GitHub org
  allowedDomain?: string  // Google domain
}
export type OrcaUser = {
  id: string; email: string; name: string
  role: 'developer' | 'lead' | 'admin'
  provider: OrcaIdentityProvider
  // ...
}
```

Types đã được thiết kế sẵn trong `src/shared/rbac-types.ts` nhưng **chưa có implementation**:
- Không có HTTP login endpoint
- Không có OAuth2/OIDC callback handler
- Không có session/JWT management
- PairCode là cơ chế duy nhất — không có user identity

### Impact

- Developer phải xin PairCode từ admin mỗi lần → friction cao
- Không có danh tính người dùng → không thể sandbox, không audit log
- PairCode share dễ bị leak

---

## 2. Giải pháp đề xuất

### 2.1 Web Login Flow (Username/Password)

**Cho môi trường nội bộ không có IdP bên ngoài:**

```
Browser                    Orca HTTP (:6769)           UserStore (SQLite)
   │                              │                           │
   │── GET /login ───────────────►│                           │
   │◄─ HTML login form ───────────│                           │
   │                              │                           │
   │── POST /auth/local ──────────►│                          │
   │   { email, password }        │── bcrypt.verify ─────────►│
   │                              │◄─ user record ────────────│
   │◄─ Set-Cookie: session_id ────│                           │
   │   302 → /                    │── create session ─────────►│
```

**Session token:**
```typescript
type OrcaSession = {
  sessionId: string       // random 32 bytes hex
  userId:    string
  userEmail: string
  role:      OrcaUser['role']
  createdAt: number
  expiresAt: number       // default: 8 hours
  lastSeenAt: number
}
```

### 2.2 SSO Flow (GitHub / Google / Keycloak OIDC)

```
Browser                  Orca HTTP              IdP (GitHub/Google/Keycloak)
   │                         │                            │
   │── GET /auth/sso/github ►│                            │
   │◄─ 302 → GitHub OAuth ───│                            │
   │                         │                            │
   │── OAuth redirect ───────────────────────────────────►│
   │◄─ code + state ─────────────────────────────────────│
   │                         │                            │
   │── GET /auth/callback ───►│                            │
   │   ?code=...&state=...   │── exchange code ───────────►│
   │                         │◄─ access_token + id_token ─│
   │                         │── validate + upsert user   │
   │◄─ Set-Cookie: session ──│                            │
   │   302 → /               │                            │
```

**OIDC (Keycloak / custom):**
```typescript
// Discovery URL tự động lấy jwks_uri, issuer, token_endpoint
const oidcConfig = await fetchOidcDiscovery(config.discoveryUrl)
// Verify JWT với jwks
const claims = await verifyIdToken(idToken, oidcConfig.jwksUri)
```

### 2.3 Hybrid: PairCode vẫn hoạt động song song

PairCode vẫn được giữ cho:
- Orca Desktop app (Electron) kết nối local
- CI/automation scripts
- Offline scenarios

Sau khi login, web client nhận thêm `sessionToken` bên cạnh `deviceToken`:

```typescript
// Sau login thành công
type AuthenticatedContext = {
  deviceToken:  string       // E2EE pairing (như hiện tại)
  sessionToken: string       // HTTP session cookie
  userId:       string       // identity từ login/SSO
  userEmail:    string
  role:         OrcaUser['role']
}
```

---

## 3. Các thay đổi cần thực hiện

### 3.1 HTTP Endpoints mới (`:6769`)

| Method | Path | Mô tả |
|--------|------|-------|
| `GET` | `/login` | Web login page (SPA route hoặc server-rendered) |
| `POST` | `/auth/local` | Username/password login |
| `GET` | `/auth/sso/:provider` | Redirect đến IdP (github/google/keycloak) |
| `GET` | `/auth/callback` | OAuth2/OIDC callback handler |
| `POST` | `/auth/logout` | Logout, clear session |
| `GET` | `/auth/me` | Current user info (JSON) |

### 3.2 Files cần tạo mới

```
src/main/auth/
├── auth-router.ts          # Express/Hono router cho /auth/* endpoints
├── auth-session-store.ts   # CRUD session trong SQLite
├── auth-local-handler.ts   # Username/password (bcrypt)
├── auth-oauth-handler.ts   # GitHub, Google OAuth2
├── auth-oidc-handler.ts    # Keycloak / generic OIDC (node-openid-client)
├── auth-middleware.ts      # Express middleware: verify session cookie
└── auth-types.ts           # OrcaSession, AuthContext types
```

### 3.3 Schema database mới

```sql
-- Bảng users (local auth)
CREATE TABLE orca_users (
  id           TEXT PRIMARY KEY,
  email        TEXT UNIQUE NOT NULL,
  name         TEXT NOT NULL,
  password_hash TEXT,          -- null nếu SSO-only
  role         TEXT DEFAULT 'developer',
  provider     TEXT DEFAULT 'none',
  provider_user_id TEXT,
  avatar_url   TEXT,
  teams        TEXT DEFAULT '[]',  -- JSON array
  projects     TEXT DEFAULT '[]',
  created_at   INTEGER NOT NULL,
  last_login_at INTEGER,
  is_active    INTEGER DEFAULT 1
);

-- Bảng sessions
CREATE TABLE orca_sessions (
  session_id   TEXT PRIMARY KEY,
  user_id      TEXT REFERENCES orca_users(id),
  created_at   INTEGER NOT NULL,
  expires_at   INTEGER NOT NULL,
  last_seen_at INTEGER,
  ip_address   TEXT,
  user_agent   TEXT
);
```

### 3.4 Files cần sửa

#### [MODIFY] `src/main/runtime/runtime-rpc.ts`

Thêm `sessionToken` vào `AuthenticatedContext`, xử lý auth middleware cho WS connection khi client login qua web:

```typescript
// Thêm: nhận sessionToken từ WS handshake header/query
// Cookie: session_id=xxx hoặc ?session=xxx trong WS URL
const sessionToken = extractSessionFromWsRequest(req)
if (sessionToken) {
  const session = await authSessionStore.validate(sessionToken)
  if (session) {
    context.userId = session.userId
    context.userEmail = session.userEmail
    context.role = session.role
  }
}
```

#### [MODIFY] `src/renderer/src/web/`

Thêm Login page và SSO buttons vào web SPA:
- `/login` route với form email/password
- "Login with GitHub / Google / Keycloak" buttons
- Session-aware: nếu đã có session → skip pairing form → vào thẳng workspace

#### [MODIFY] `src/main/http-server.ts` (hoặc file serve HTTP)

Mount auth router:
```typescript
app.use('/auth', authRouter)
app.use('/login', serveLoginPage)
```

---

## 4. Config

Thêm vào `ORCA_*` env vars hoặc `orca-config.json`:

```yaml
# .env
AUTH_MODE=local          # local | sso | both (default: both)
AUTH_SESSION_TTL=28800   # seconds (8 hours)

# SSO — GitHub
SSO_GITHUB_CLIENT_ID=xxx
SSO_GITHUB_CLIENT_SECRET=xxx
SSO_GITHUB_ALLOWED_ORG=vnp-blc   # optional: restrict org

# SSO — Google
SSO_GOOGLE_CLIENT_ID=xxx
SSO_GOOGLE_CLIENT_SECRET=xxx
SSO_GOOGLE_ALLOWED_DOMAIN=vnpblockchain.com

# SSO — Keycloak / OIDC
SSO_OIDC_CLIENT_ID=orca
SSO_OIDC_CLIENT_SECRET=xxx
SSO_OIDC_DISCOVERY_URL=https://keycloak.internal/realms/vnp/.well-known/openid-configuration
```

---

## 5. Web UI — Login Page

Trang `/login` hiển thị:

```
┌─────────────────────────────────────┐
│           Orca                      │
│  Collaborative Dev Environment      │
├─────────────────────────────────────┤
│                                     │
│  Email:    [_______________________]│
│  Password: [_______________________]│
│           [  Sign In  ]             │
│                                     │
│  ─────────── or ───────────         │
│                                     │
│  [ 🐙 Continue with GitHub ]        │
│  [ 🔵 Continue with Google ]        │
│  [ 🔑 Continue with Keycloak ]      │
│                                     │
│  ─────────── or ───────────         │
│                                     │
│  Pairing URL or Code:               │
│  [_______________________________]  │
│  [       Connect                 ]  │
│                                     │
└─────────────────────────────────────┘
```

---

## 6. Acceptance Criteria

- [x] `POST /auth/local` xác thực email+bcrypt password, trả về Set-Cookie session ✅ `auth-router.ts` L33 + `auth-local-handler.ts`
- [ ] `GET /auth/sso/github` redirect đúng GitHub OAuth URL với state CSRF — **DEFERRED** (Phase 2: stub trả 501)
- [ ] `GET /auth/callback` exchange code → user upsert → session → redirect `/` — **DEFERRED** (Phase 2: OAuth flow)
- [ ] OIDC (Keycloak): verify JWT signature qua JWKS endpoint — **DEFERRED** (Phase 3)
- [x] Session middleware block unauthenticated WS connections khi `AUTH_MODE != none` ✅ `auth-middleware.ts` — `requireAuth` guard
- [x] PairCode flow vẫn hoạt động song song (backward compat) ✅ `runtime-rpc.ts` E2EE/PairCode flow không bị sửa
- [x] Session expire sau TTL, `last_seen_at` update mỗi request ✅ `auth-session-store.ts` L71 — `validateSession()` touches `last_seen_at`
- [x] `/auth/me` trả về `{id, email, name, role, provider}` ✅ `auth-router.ts` L70
- [x] Login page render đúng với các provider được config ✅ `LoginPage.tsx` + `SsoButton.tsx` + `PairCodeFallback.tsx`

---

## 7. Implementation Status

> **✅ PHASE 1 IMPLEMENTED — 2026-07-24**  
> 5/9 AC done | 2 DEFERRED Phase 2 (SSO OAuth) | 1 DEFERRED Phase 3 (OIDC)

| Layer | Files | Status |
|-------|-------|--------|
| Backend: Auth Session Store | `src/main/auth/auth-session-store.ts` | ✅ Done |
| Backend: Auth User Store | `src/main/auth/auth-user-store.ts` | ✅ Done (bcrypt) |
| Backend: Auth Local Handler | `src/main/auth/auth-local-handler.ts` | ✅ Done |
| Backend: Auth Manager | `src/main/auth/auth-manager.ts` | ✅ Done |
| Backend: Auth Middleware | `src/main/auth/auth-middleware.ts` | ✅ Done |
| Backend: Auth Router | `src/main/auth/auth-router.ts` | ✅ Done |
| Backend: DB Migration 0005 | `src/main/db/migrations/0005_add_auth_schema.ts` | ✅ Done |
| Frontend: LoginPage + Form | `src/renderer/src/web/login/LoginPage.tsx` | ✅ Done |
| Frontend: SsoButton + PairCode | `src/renderer/src/web/login/SsoButton.tsx` | ✅ Done |
| Frontend: auth-api-client | `src/renderer/src/auth/auth-api-client.ts` | ✅ Done |
| Frontend: auth-types | `src/renderer/src/auth/auth-types.ts` | ✅ Done |

**Tests:** Backend 40 pass | Frontend 17 pass (login + auth hooks)  
**Deferred:** SSO OAuth redirect, /auth/callback, OIDC JWKS (Phase 2/3)

