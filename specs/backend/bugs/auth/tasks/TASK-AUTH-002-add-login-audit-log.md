# TASK-AUTH-002: Thêm Audit Log cho login attempts

**Priority:** 🟠 HIGH — Security compliance, cần audit trail  
**Effort:** ~40 phút  
**Status:** ✅ DONE  
**Bug refs:** BUG-AUTH-001, BUG-BE-AUTH-001  
**Solution ref:** [SOLUTION-auth-exact.md](../solutions/SOLUTION-auth-exact.md)

---

## Mục tiêu

Tạo `AuditLogger` class và gọi nó trong `AuthManager.login()` để ghi lại mọi login attempt (thành công và thất bại).

---

## Bước 1 — Tạo DB migration

Tạo file `src/main/db/migrations/0017_audit_log.ts` (hoặc số thứ tự tiếp theo):

```bash
# Kiểm tra migration hiện tại cao nhất:
ls src/main/db/migrations/ | tail -5
```

```typescript
// src/main/db/migrations/001X_audit_log.ts
import type { IConnection } from '../pool'

export async function up(db: IConnection): Promise<void> {
  await db.query(`
    CREATE TABLE IF NOT EXISTS orca_audit_log (
      id           TEXT    NOT NULL DEFAULT (lower(hex(randomblob(16)))),
      action       TEXT    NOT NULL,
      user_id      TEXT    NOT NULL DEFAULT 'unknown',
      user_email   TEXT    NOT NULL DEFAULT '',
      ip           TEXT    NOT NULL DEFAULT '',
      user_agent   TEXT,
      details_json TEXT    NOT NULL DEFAULT '{}',
      created_at   INTEGER NOT NULL,
      PRIMARY KEY (id)
    )
  `)
  await db.query(`
    CREATE INDEX IF NOT EXISTS idx_audit_log_user_id ON orca_audit_log(user_id)
  `)
  await db.query(`
    CREATE INDEX IF NOT EXISTS idx_audit_log_action ON orca_audit_log(action)
  `)
  await db.query(`
    CREATE INDEX IF NOT EXISTS idx_audit_log_created_at ON orca_audit_log(created_at)
  `)
}

export async function down(db: IConnection): Promise<void> {
  await db.query('DROP TABLE IF EXISTS orca_audit_log')
}
```

---

## Bước 2 — Tạo `src/main/audit/audit-logger.ts` (NEW)

```typescript
// src/main/audit/audit-logger.ts
/**
 * AuditLogger — Write security audit events to orca_audit_log table.
 * @module main/audit/audit-logger
 */

import type { IConnectionPool } from '../db/pool'

export interface AuditEntry {
  action:    string                      // e.g. 'auth.login.success', 'auth.login.failed'
  userId:    string                      // 'unknown' for unauthenticated attempts
  userEmail: string
  ip:        string
  userAgent?: string
  details?:  Record<string, unknown>
}

export class AuditLogger {
  constructor(private readonly pool: IConnectionPool) {}

  async log(entry: AuditEntry): Promise<void> {
    const now = Date.now()
    await this.pool.withConnection((db) =>
      db.query(
        `INSERT INTO orca_audit_log
         (action, user_id, user_email, ip, user_agent, details_json, created_at)
         VALUES (?, ?, ?, ?, ?, ?, ?)`,
        [
          entry.action,
          entry.userId,
          entry.userEmail,
          entry.ip,
          entry.userAgent ?? '',
          JSON.stringify(entry.details ?? {}),
          now,
        ]
      )
    ).catch((err: unknown) => {
      // Audit log write failure must NOT crash the request
      console.error('[AuditLogger] Write failed:', err)
    })
  }
}
```

---

## Bước 3 — Inject AuditLogger vào AuthManager

```bash
# Xem AuthManager constructor:
grep -n "constructor\|auditLogger\|AuditLogger" src/main/auth/auth-manager.ts | head -10
```

Thêm `auditLogger` dependency:
```typescript
// src/main/auth/auth-manager.ts — trong constructor:
import { AuditLogger } from '../audit/audit-logger'

constructor(
  private readonly userStore:    AuthUserStore,
  private readonly sessionStore: AuthSessionStore,
  private readonly auditLogger?: AuditLogger    // optional để không break tests
) {}
```

---

## Bước 4 — Gọi auditLogger trong login()

```typescript
// src/main/auth/auth-manager.ts — trong login():
async login(
  creds: { email: string; password: string },
  ip: string,
  userAgent: string
): Promise<LoginResult> {
  const result = await this.localHandler.login(creds.email, creds.password)

  // Audit log — fire-and-forget (không await để không chậm login response)
  void this.auditLogger?.log({
    action:    result.success ? 'auth.login.success' : 'auth.login.failed',
    userId:    result.success ? result.user.id : 'unknown',
    userEmail: creds.email,
    ip,
    userAgent,
    details:   result.success
      ? { sessionId: result.sessionId }
      : { error: result.error },
  })

  return result
}
```

---

## Bước 5 — Wire AuditLogger vào server-bootstrap

```bash
grep -n "AuthManager\|authManager" src/main/server-bootstrap.ts | head -10
```

```typescript
// src/main/server-bootstrap.ts — trong createServer():
import { AuditLogger } from './audit/audit-logger'

const auditLogger = new AuditLogger(connectionPool)
const authManager = new AuthManager(userStore, sessionStore, auditLogger)
```

---

## Verification

```bash
pnpm tsc --noEmit

# Test: login → check orca_audit_log table:
# SELECT * FROM orca_audit_log ORDER BY created_at DESC LIMIT 5;
# Expected: rows với action='auth.login.success' hoặc 'auth.login.failed'
```
