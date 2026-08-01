# F23 — Multi-User Login & Authentication

| Trường | Giá trị |
|--------|---------|
| **ID** | F23 |
| **Tên** | Multi-User Login & Authentication |
| **Ưu tiên** | P0 |
| **Trạng thái** | ✅ Phát hành (Phase 1) |
| **CRs** | [login/CR-LOGIN-001](../crs/v1/login/CR-LOGIN-001-auth.md) |
| **TDD** | [TDD-04: Auth Layer](../specs/backend/tdd/04-rpc-server.md) |
| **Phiên bản** | v4.0+ |
| **ADR References** | ADR-003 |
| **HLD References** | C3.1, C2 |

---

## Mô tả

Orca hỗ trợ **multi-user authentication** khi chạy ở server mode (`ORCA_MULTI_USER=1`). Mỗi user đăng nhập với email + password, nhận HTTP cookie session, và được cô lập hoàn toàn với các user khác.

---

## Vấn đề cần giải quyết

Trước đây Orca dùng **PairCode only** — một mã QR/base64 phải share thủ công. Không thể:
- Onboard nhiều developer vào cùng Orca instance
- Kiểm soát ai có quyền truy cập
- Audit hoạt động của từng user

---

## Tính năng chi tiết

### Local Email/Password Login
- `POST /auth/local` — validate email + bcrypt password, trả `Set-Cookie: orca_session`
- Session cookie: `HttpOnly; Secure; SameSite=Lax`, TTL 8 giờ
- `GET /auth/me` — trả `{id, email, name, role, provider}`
- `POST /auth/logout` — revoke session + clear cookie

### Session Management
- `OrcaSession` — persist trong SQLite (`orca_sessions` table)
- `last_seen_at` cập nhật mỗi request
- Auto-cleanup sessions expired mỗi 30 phút
- `AuthManager.validateRequest()` — validate cookie per-request

### SSO Stub (Phase 2)
- `GET /auth/sso/:provider` — 501 stub (GitHub, Google, Keycloak)
- Full OAuth2/OIDC flow → Phase 2/3

### PairCode Backward Compat
- PairCode + E2EE flow giữ nguyên hoàn toàn
- Khi `ORCA_MULTI_USER=0` → single-user mode không cần login

### Login UI
- `LoginPage.tsx` — email/password form + SSO buttons
- `LoginForm.tsx` — validation, loading state, error display
- `SsoButton.tsx` — per-provider buttons (khi được config)
- `PairCodeFallback.tsx` — backward compat với PairCode

---

## Database Schema

```sql
CREATE TABLE orca_users (
  id TEXT PRIMARY KEY, email TEXT UNIQUE NOT NULL,
  role TEXT DEFAULT 'developer',  -- 'admin' | 'developer'
  password_hash TEXT,             -- bcrypt 12 rounds
  provider TEXT DEFAULT 'none',   -- 'none'|'github'|'google'|'keycloak'
  is_active INTEGER DEFAULT 1, ...
);
CREATE TABLE orca_sessions (
  session_id TEXT PRIMARY KEY,    -- randomBytes(32).hex = 64 chars
  user_id TEXT REFERENCES orca_users(id) ON DELETE CASCADE,
  expires_at INTEGER NOT NULL,    -- created_at + 8h
  last_seen_at INTEGER, ...
);
```

---

## Tiêu chí chấp nhận

- [x] `POST /auth/local` xác thực email+bcrypt password, trả về Set-Cookie session
- [x] Session middleware block unauthenticated requests khi MULTI_USER mode
- [x] PairCode flow vẫn hoạt động song song (backward compat)
- [x] Session expire sau 8h, `last_seen_at` update mỗi request
- [x] `/auth/me` trả về `{id, email, name, role, provider}`
- [x] Login page render đúng với email/password form + SSO buttons
- [ ] SSO GitHub/Google redirect — DEFERRED Phase 2
- [ ] OIDC/Keycloak JWT verification — DEFERRED Phase 3

---

## Yêu cầu kỹ thuật

| Component | File |
|-----------|------|
| Auth types | `src/main/auth/auth-types.ts` |
| Auth session store | `src/main/auth/auth-session-store.ts` |
| Auth user store | `src/main/auth/auth-user-store.ts` |
| Auth local handler | `src/main/auth/auth-local-handler.ts` |
| Auth manager | `src/main/auth/auth-manager.ts` |
| Auth middleware | `src/main/auth/auth-middleware.ts` |
| Auth router | `src/main/auth/auth-router.ts` |
| DB migration | `src/main/db/migrations/0005_add_auth_schema.ts` |
| Login UI | `src/renderer/src/web/login/LoginPage.tsx` |
| Auth API client | `src/renderer/src/auth/auth-api-client.ts` |

**Tests:** 40 backend + 17 frontend = **57 tests**
