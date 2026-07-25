# Backend Solutions — Login (Multi-User Auth, Sandbox, SSH Isolation, Admin)
## Index

**Version:** 1.0
**Date:** 2026-07-24
**CRs:** [docs/crs/v1/login/](../../../../../docs/crs/v1/login/)
**TDD Reference:** [specs/backend/tdd/](../../../tdd/)
**Based on TDD:** TDD-04 (RPC Server), TDD-06 (Persistence), TDD-11 (Web Server Mode), TDD-12 (Database Layer), TDD-05 (SSH Relay)

---

## ✅ Implementation Status

> **HOÀN THÀNH: 2026-07-24**
> 30 tasks thực thi | 134/134 tests pass | 0 TypeScript errors | 163/163 Acceptance Criteria ✅

| Phase | Solution | Tasks | Tests | Status |
|-------|----------|-------|-------|--------|
| Phase 1 | SOL-LG-001 | TASK-001~012 | 40 pass | ✅ Done |
| Phase 2 | SOL-LG-002 | TASK-013~018 | 21 pass | ✅ Done |
| Phase 3 | SOL-LG-003 | TASK-019~021 | 29 pass | ✅ Done |
| Phase 4 | SOL-LG-004 | TASK-022~030 | 44 pass | ✅ Done |
| **Total** | **4 solutions** | **30 tasks** | **134 pass** | **✅ Done** |

---



## Mục tiêu

Bộ solutions này cung cấp **hướng dẫn triển khai chi tiết** (test-driven) cho 4 Change Requests trong `login`, bổ sung:
- Multi-user authentication (local login + SSO) bên cạnh PairCode
- Per-user process isolation (sandbox)
- SSH dev server isolation theo unix account
- Admin panel quản lý users, sessions, policies

### Nguyên tắc thiết kế

1. **Additive Only** — Không phá vỡ PairCode + E2EE hiện tại, thêm auth layer mới bên ngoài
2. **Backward Compat** — PairCode/deviceToken vẫn hoạt động sau khi thêm login
3. **Server Mode Only** — Login/SSO chỉ kích hoạt trong web server mode (không ảnh hưởng Electron desktop)
4. **Interface-driven** — Mọi implementation phải đi kèm interface trước
5. **Test-driven** — Viết test spec trước implementation, mỗi module ≥ 3 test cases
6. **No breaking changes** — `OrcaRuntimeRpcServer` auth flow hiện tại phải giữ nguyên

---

## Danh sách Solutions

| Solution | CR tương ứng | Domain | TDD Reference | Status |
|----------|-------------|--------|--------------|--------|
| [SOL-LG-001](./SOL-LG-001-auth-session.md) | CR-LOGIN-001 | Auth Layer: Login + SSO + Session | TDD-04, TDD-06, TDD-12 | ✅ Implemented (2026-07-24) |
| [SOL-LG-002](./SOL-LG-002-user-sandbox.md) | CR-LOGIN-002 | Per-User Sandbox: Session Manager + Process Fork | TDD-04, TDD-07, TDD-11 | ✅ Implemented (2026-07-24) |
| [SOL-LG-003](./SOL-LG-003-ssh-isolation.md) | CR-LOGIN-003 | SSH Dev Server: Per-User Unix Account | TDD-05, TDD-13 | ✅ Implemented (2026-07-24) |
| [SOL-LG-004](./SOL-LG-004-admin-ui.md) | CR-LOGIN-004 | Admin Panel: User, Session, Policy Management | TDD-04, TDD-11, TDD-12 | ✅ Implemented (2026-07-24) |

---

## Mapping CR → Solution

```
CR-LOGIN-001 (Auth: Login + SSO)        → SOL-LG-001
CR-LOGIN-002 (Per-User Sandbox)         → SOL-LG-002
CR-LOGIN-003 (SSH Dev Isolation)        → SOL-LG-003
CR-LOGIN-004 (Admin UI)                 → SOL-LG-004
```

---

## Dependency thực hiện

```
SOL-LG-001 (Auth/Session) — phải xong trước
    │
    ├──► SOL-LG-002 (Sandbox) — cần userId từ session
    │         │
    │         └──► SOL-LG-003 (SSH Isolation) — cần userId routing
    │
    └──► SOL-LG-004 (Admin) — cần user model từ SOL-LG-001
```

---

## File Structure Mục tiêu

```
src/main/
├── auth/                              ← [NEW] SOL-LG-001
│   ├── auth-types.ts                  ← OrcaSession, AuthContext, LocalUser
│   ├── auth-session-store.ts          ← CRUD sessions trong SQLite
│   ├── auth-local-handler.ts          ← Username/bcrypt login
│   ├── auth-oauth-handler.ts          ← GitHub / Google OAuth2
│   ├── auth-oidc-handler.ts           ← Keycloak OIDC (openid-client)
│   ├── auth-middleware.ts             ← Express middleware verify session
│   ├── auth-router.ts                 ← /auth/* HTTP routes
│   └── __tests__/
│       ├── auth-session-store.test.ts
│       ├── auth-local-handler.test.ts
│       └── auth-oauth-handler.test.ts
│
├── session/                           ← [NEW] SOL-LG-002
│   ├── session-types.ts               ← UserProcess, SessionManagerConfig
│   ├── session-manager.ts             ← Process spawner + lifecycle
│   ├── ws-session-router.ts           ← WS proxy: supervisor → user socket
│   ├── user-process-entry.ts          ← Fork entry point per user
│   └── __tests__/
│       ├── session-manager.test.ts
│       └── ws-session-router.test.ts
│
├── ssh/                               ← [MODIFY] SOL-LG-003
│   ├── dev-server-provisioner.ts      ← [NEW] Tạo unix account trên dev server
│   ├── ssh-user-resolver.ts           ← [NEW] Map userId → linux username
│   └── __tests__/
│       ├── dev-server-provisioner.test.ts
│       └── ssh-user-resolver.test.ts
│
├── admin/                             ← [NEW] SOL-LG-004
│   ├── admin-types.ts                 ← AdminStats, AuditEvent
│   ├── admin-router.ts                ← /admin/api/* HTTP routes
│   ├── admin-middleware.ts            ← requireAdmin middleware
│   ├── admin-user-handlers.ts         ← CRUD user handlers
│   ├── admin-session-handlers.ts      ← Session kill handlers
│   ├── admin-policy-handlers.ts       ← Access policy CRUD
│   ├── admin-audit-handlers.ts        ← Audit log query
│   ├── audit-logger.ts                ← Write audit events
│   ├── first-run-setup.ts             ← Seed first admin user
│   └── __tests__/
│       ├── admin-user-handlers.test.ts
│       ├── admin-session-handlers.test.ts
│       └── audit-logger.test.ts
│
db/
└── migrations/
    └── 0004_add_auth_schema.ts        ← [NEW] orca_users, orca_sessions, orca_audit_log, orca_policies
```

---

## Schema Database mới (Migration 0004)

```sql
-- Users (local auth + SSO)
CREATE TABLE orca_users (
  id              TEXT PRIMARY KEY,
  email           TEXT UNIQUE NOT NULL,
  name            TEXT NOT NULL,
  password_hash   TEXT,                 -- null nếu SSO-only
  role            TEXT DEFAULT 'developer',
  provider        TEXT DEFAULT 'none',  -- 'none'|'github'|'google'|'keycloak'
  provider_user_id TEXT,
  avatar_url      TEXT,
  teams           TEXT DEFAULT '[]',    -- JSON array
  projects        TEXT DEFAULT '[]',
  created_at      INTEGER NOT NULL,
  last_login_at   INTEGER,
  is_active       INTEGER DEFAULT 1
);

-- Sessions (HTTP cookie-based)
CREATE TABLE orca_sessions (
  session_id    TEXT PRIMARY KEY,
  user_id       TEXT REFERENCES orca_users(id),
  created_at    INTEGER NOT NULL,
  expires_at    INTEGER NOT NULL,
  last_seen_at  INTEGER,
  ip_address    TEXT,
  user_agent    TEXT
);

-- Audit log
CREATE TABLE orca_audit_log (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  created_at  INTEGER NOT NULL,
  user_id     TEXT,
  user_email  TEXT,
  action      TEXT NOT NULL,
  detail      TEXT,          -- JSON
  ip_address  TEXT
);
CREATE INDEX idx_audit_user   ON orca_audit_log(user_id, created_at DESC);
CREATE INDEX idx_audit_action ON orca_audit_log(action, created_at DESC);

-- Access policies (RBAC)
CREATE TABLE orca_access_policies (
  id                    TEXT PRIMARY KEY,
  name                  TEXT NOT NULL,
  teams                 TEXT DEFAULT '[]',
  roles                 TEXT DEFAULT '[]',
  users                 TEXT DEFAULT '[]',
  allowed_servers       TEXT DEFAULT '"*"',
  allowed_projects      TEXT DEFAULT '"*"',
  agent_trust           TEXT DEFAULT 'standard',
  can_create_worktrees  INTEGER DEFAULT 1,
  can_delete_worktrees  INTEGER DEFAULT 1,
  can_access_production INTEGER DEFAULT 0,
  created_at            INTEGER NOT NULL,
  updated_at            INTEGER NOT NULL
);
```
