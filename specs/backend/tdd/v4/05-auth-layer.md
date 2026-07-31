# TDD-BE-05: Auth Layer

**Version:** 4.0  
**Date:** 2026-07-28  
**Source:** `src/main/auth/`

---

## 1. Module Map

| File | Role |
|------|------|
| `auth-types.ts` | `OrcaSession`, `OrcaSessionUser`, `SESSION_TTL_MS` |
| `auth-session-store.ts` | CRUD sessions trong `orca_sessions` table |
| `auth-user-store.ts` | bcrypt hash/verify, upsertSsoUser(), deactivate |
| `auth-local-handler.ts` | email/password login với format validation |
| `auth-manager.ts` | Facade: validateRequest(), login(), logout(), cleanup |
| `auth-middleware.ts` | Express middleware, requireAuth() guard |
| `auth-router.ts` | HTTP routes: POST /auth/local, /logout, GET /auth/me |

---

## 2. Core Types

```typescript
// Session TTL: 8 giờ
export const SESSION_TTL_MS = 8 * 60 * 60 * 1000

export type OrcaSession = {
  sessionId:  string        // 64-hex (32 random bytes) — HttpOnly cookie
  userId:     string
  userEmail:  string
  role:       OrcaUser['role']
  createdAt:  number        // Unix ms
  expiresAt:  number        // createdAt + SESSION_TTL_MS
  lastSeenAt: number | null
  ipAddress:  string | null
  userAgent:  string | null
}

export type OrcaSessionUser = Pick<OrcaUser, 'id' | 'email' | 'name' | 'role' | 'provider'>

export type LocalLoginResult =
  | { success: true;  sessionId: string; user: OrcaSessionUser }
  | { success: false; error: 'invalid_credentials' | 'account_disabled' | 'validation_error'; detail?: string }
```

---

## 3. AuthManager (Facade)

```typescript
class AuthManager {
  // Validate session token from cookie/header → return session or null
  async validateRequest(req: IncomingMessage): Promise<OrcaSession | null>

  // Login với email + password → LocalLoginResult
  async login(input: LocalLoginInput, meta: { ip: string; userAgent: string }): Promise<LocalLoginResult>

  // Logout — xóa session khỏi DB
  async logout(sessionId: string): Promise<void>

  // Get current user info
  async getMe(sessionId: string): Promise<OrcaSessionUser | null>

  // Scheduled cleanup (mỗi 30 phút) — xóa expired sessions
  private cleanupExpiredSessions(): void
}
```

---

## 4. AuthRouter (Express routes)

```
POST /auth/local
  Body: { email: string, password: string }
  → AuthLocalHandler.login()
  → Set-Cookie: orca_session=<sessionId>; HttpOnly; SameSite=Strict; Max-Age=28800
  Response: { user: OrcaSessionUser }

POST /auth/logout
  → AuthManager.logout(sessionId from cookie)
  → Clear cookie
  Response: { ok: true }

GET /auth/me
  → AuthManager.validateRequest() → getMe()
  Response: OrcaSessionUser | 401

GET /auth/sso/:provider
  → Redirect to SSO provider (GitHub/Google/Keycloak OAuth2)
  [Phase 3 — deferred]
```

---

## 5. AuthMiddleware

```typescript
// Gắn vào mọi request TRƯỚC route handlers
// Không block request — chỉ populate req.orcaSession nếu hợp lệ
function authMiddleware(req, res, next) {
  const sessionId = parseCookie(req.headers.cookie)?.orca_session
  if (sessionId) {
    const session = await authManager.validateRequest(req)
    if (session) req.orcaSession = session
  }
  next()
}

// Guard dùng trong protected routes
function requireAuth(req, res, next) {
  if (!req.orcaSession) return res.status(401).json({ error: 'unauthenticated' })
  next()
}
```

---

## 6. AuthSessionStore (CRUD)

```typescript
class AuthSessionStore {
  createSession(input: CreateSessionInput): OrcaSession
  getSession(sessionId: string): OrcaSession | null
  touchSession(sessionId: string, now: number): void   // update lastSeenAt
  deleteSession(sessionId: string): void
  deleteUserSessions(userId: string): void
  cleanupExpired(now: number): number                   // return deleted count
  listActiveSessions(): OrcaSession[]
}
```

---

## 7. AuthUserStore

```typescript
class AuthUserStore {
  createLocalUser(input: LocalUserInput): OrcaUser    // bcrypt hash password
  verifyPassword(email: string, password: string): Promise<OrcaUser | null>
  upsertSsoUser(input: SsoUserInput): OrcaUser        // create or update SSO user
  deactivateUser(userId: string): void
  getUserById(userId: string): OrcaUser | null
  listUsers(): OrcaUser[]
}
```

**Bcrypt**: cost factor 12, `bcrypt.hash()` / `bcrypt.compare()`

---

## 8. Security Model

- Session token: 64-char hex (32 bytes crypto.randomBytes)
- Cookie: `HttpOnly; SameSite=Strict; Max-Age=28800` (8h)
- Session stored in `auth.db` (SQLite riêng, KHÔNG dùng chung với app DB)
- Cleanup expired: mỗi 30 phút (background timer, `unref()` để không block exit)
- Email validation: regex `^[^\s@]+@[^\s@]+\.[^\s@]+$`
- No rate limiting at this layer (để reverse proxy xử lý)

---

## 9. SSO Providers (Phase 3 — deferred)

```typescript
export type SsoProvider = 'github' | 'google' | 'keycloak'

export type SsoUserInput = {
  email:          string
  name:           string
  provider:       SsoProvider
  providerUserId: string
  avatarUrl?:     string
}
```

---

## 10. Tests (40 tests)

| File | Tests |
|------|-------|
| `auth-session-store.test.ts` | 12 |
| `auth-user-store.test.ts` | 10 |
| `auth-local-handler.test.ts` | 8 |
| `auth-manager.test.ts` | 10 |
