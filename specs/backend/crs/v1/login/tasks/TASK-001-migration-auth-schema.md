# TASK-001: Tạo migration auth schema

> **Status:** ✅ DONE (2026-07-24)
> **Actual file:** `src/main/db/migrations/0005_add_auth_schema.ts` (version 5 — vì 0004 đã tồn tại)

**Phase:** 1 — Auth Foundation
**Solution:** [SOL-LG-001](../solutions/SOL-LG-001-auth-session.md) §5
**Depends on:** (không có)
**Blocks:** TASK-002, TASK-003, TASK-004

---

## Mục tiêu

Tạo migration 0004 thêm 4 bảng vào SQLite: `orca_users`, `orca_sessions`, `orca_audit_log`, `orca_access_policies`.

---

## File cần tạo

**Path:** `src/main/db/migrations/0004_add_auth_schema.ts`

---

## Nội dung

```typescript
// src/main/db/migrations/0004_add_auth_schema.ts
import type { Migration } from './types'

export const migration_0004: Migration = {
  id: 4,
  name: '0004_add_auth_schema',

  up(db) {
    db.exec(`
      CREATE TABLE IF NOT EXISTS orca_users (
        id               TEXT PRIMARY KEY,
        email            TEXT UNIQUE NOT NULL,
        name             TEXT NOT NULL,
        password_hash    TEXT,
        role             TEXT NOT NULL DEFAULT 'developer',
        provider         TEXT NOT NULL DEFAULT 'none',
        provider_user_id TEXT,
        avatar_url       TEXT,
        teams            TEXT NOT NULL DEFAULT '[]',
        projects         TEXT NOT NULL DEFAULT '[]',
        created_at       INTEGER NOT NULL,
        last_login_at    INTEGER,
        is_active        INTEGER NOT NULL DEFAULT 1
      );

      CREATE TABLE IF NOT EXISTS orca_sessions (
        session_id    TEXT PRIMARY KEY,
        user_id       TEXT NOT NULL REFERENCES orca_users(id) ON DELETE CASCADE,
        created_at    INTEGER NOT NULL,
        expires_at    INTEGER NOT NULL,
        last_seen_at  INTEGER,
        ip_address    TEXT,
        user_agent    TEXT
      );
      CREATE INDEX IF NOT EXISTS idx_sessions_user    ON orca_sessions(user_id);
      CREATE INDEX IF NOT EXISTS idx_sessions_expires ON orca_sessions(expires_at);

      CREATE TABLE IF NOT EXISTS orca_audit_log (
        id          INTEGER PRIMARY KEY AUTOINCREMENT,
        created_at  INTEGER NOT NULL,
        user_id     TEXT,
        user_email  TEXT,
        action      TEXT NOT NULL,
        detail      TEXT,
        ip_address  TEXT
      );
      CREATE INDEX IF NOT EXISTS idx_audit_user   ON orca_audit_log(user_id, created_at DESC);
      CREATE INDEX IF NOT EXISTS idx_audit_action ON orca_audit_log(action, created_at DESC);

      CREATE TABLE IF NOT EXISTS orca_access_policies (
        id                    TEXT PRIMARY KEY,
        name                  TEXT NOT NULL,
        teams                 TEXT NOT NULL DEFAULT '[]',
        roles                 TEXT NOT NULL DEFAULT '[]',
        users                 TEXT NOT NULL DEFAULT '[]',
        allowed_servers       TEXT NOT NULL DEFAULT '"*"',
        allowed_projects      TEXT NOT NULL DEFAULT '"*"',
        agent_trust           TEXT NOT NULL DEFAULT 'standard',
        can_create_worktrees  INTEGER NOT NULL DEFAULT 1,
        can_delete_worktrees  INTEGER NOT NULL DEFAULT 1,
        can_access_production INTEGER NOT NULL DEFAULT 0,
        created_at            INTEGER NOT NULL,
        updated_at            INTEGER NOT NULL
      );
    `)
  },

  down(db) {
    db.exec(`
      DROP TABLE IF EXISTS orca_access_policies;
      DROP TABLE IF EXISTS orca_audit_log;
      DROP TABLE IF EXISTS orca_sessions;
      DROP TABLE IF EXISTS orca_users;
    `)
  }
}
```

---

## File cần sửa

**Path:** `src/main/db/migrations/index.ts`

Thêm import và register migration 0004 vào `ALL_MIGRATIONS` array:

```typescript
import { migration_0004 } from './0004_add_auth_schema'

export const ALL_MIGRATIONS: Migration[] = [
  migration_0001,
  migration_0002,
  migration_0003,
  migration_0004,   // ← THÊM
]
```

---

## Acceptance Criteria

- [x] File `src/main/db/migrations/0005_add_auth_schema.ts` tồn tại (version 5)
- [x] `up()` tạo đủ 4 bảng: `orca_users`, `orca_sessions`, `orca_audit_log`, `orca_access_policies`
- [x] `down()` xoá sạch 4 bảng theo thứ tự ngược (FKs trước)
- [x] `migration0005AddAuthSchema` được export và thêm vào `ALL_MIGRATIONS` trong `index.ts`
- [x] Chạy `up()` trên SQLite `:memory:` không throw (17 tests pass)
