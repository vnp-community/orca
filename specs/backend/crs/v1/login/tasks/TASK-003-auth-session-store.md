# TASK-003: Tạo file `src/main/auth/auth-session-store.ts`

> **Status:** ✅ DONE (2026-07-24)
> **Tests:** 17/17 pass (`auth-session-store.test.ts`)

**Phase:** 1 — Auth Foundation
**Solution:** [SOL-LG-001](../solutions/SOL-LG-001-auth-session.md) §4.2
**Depends on:** TASK-001 (migration), TASK-002 (auth-types)
**Blocks:** TASK-005 (test), TASK-008 (auth-manager)

---

## Mục tiêu

Tạo `AuthSessionStore` — lớp quản lý CRUD sessions trong bảng `orca_sessions`.

---

## File cần tạo

**Path:** `src/main/auth/auth-session-store.ts`

---

## Nội dung

```typescript
// src/main/auth/auth-session-store.ts
import { randomBytes } from 'node:crypto'
import type { IDatabase } from '../db/types'
import type { OrcaSession, CreateSessionInput } from './auth-types'
import { SESSION_TTL_MS } from './auth-types'

export class AuthSessionStore {
  constructor(private readonly db: IDatabase) {}

  async createSession(input: CreateSessionInput): Promise<OrcaSession> {
    const sessionId = randomBytes(32).toString('hex')
    const now       = Date.now()
    const expiresAt = now + SESSION_TTL_MS

    this.db.prepare(`
      INSERT INTO orca_sessions
        (session_id, user_id, created_at, expires_at, last_seen_at, ip_address, user_agent)
      VALUES (?, ?, ?, ?, NULL, ?, ?)
    `).run(sessionId, input.userId, now, expiresAt, input.ipAddress ?? null, input.userAgent ?? null)

    return {
      sessionId, userId: input.userId, userEmail: input.userEmail,
      role: input.role, createdAt: now, expiresAt,
      lastSeenAt: null, ipAddress: input.ipAddress, userAgent: input.userAgent
    }
  }

  getSession(sessionId: string): OrcaSession | null {
    const row = this.db.prepare(`
      SELECT s.session_id, s.user_id, s.created_at, s.expires_at, s.last_seen_at,
             s.ip_address, s.user_agent, u.email AS user_email, u.role
      FROM orca_sessions s
      JOIN orca_users u ON u.id = s.user_id
      WHERE s.session_id = ?
    `).get(sessionId) as any
    return row ? this.rowToSession(row) : null
  }

  validateSession(sessionId: string): OrcaSession | null {
    const session = this.getSession(sessionId)
    if (!session) return null
    if (session.expiresAt < Date.now()) {
      this.revokeSession(sessionId)
      return null
    }
    this.db.prepare(`UPDATE orca_sessions SET last_seen_at = ? WHERE session_id = ?`)
      .run(Date.now(), sessionId)
    return session
  }

  revokeSession(sessionId: string): void {
    this.db.prepare(`DELETE FROM orca_sessions WHERE session_id = ?`).run(sessionId)
  }

  revokeAllUserSessions(userId: string): number {
    const result = this.db.prepare(`DELETE FROM orca_sessions WHERE user_id = ?`).run(userId)
    return (result as any).changes ?? 0
  }

  listUserSessions(userId: string): OrcaSession[] {
    const rows = this.db.prepare(`
      SELECT s.session_id, s.user_id, s.created_at, s.expires_at, s.last_seen_at,
             s.ip_address, s.user_agent, u.email AS user_email, u.role
      FROM orca_sessions s
      JOIN orca_users u ON u.id = s.user_id
      WHERE s.user_id = ? AND s.expires_at > ?
      ORDER BY s.created_at DESC
    `).all(userId, Date.now()) as any[]
    return rows.map(r => this.rowToSession(r))
  }

  cleanupExpired(): number {
    const result = this.db.prepare(`DELETE FROM orca_sessions WHERE expires_at < ?`).run(Date.now())
    return (result as any).changes ?? 0
  }

  private rowToSession(row: any): OrcaSession {
    return {
      sessionId:  row.session_id,
      userId:     row.user_id,
      userEmail:  row.user_email ?? row.email,
      role:       row.role,
      createdAt:  row.created_at,
      expiresAt:  row.expires_at,
      lastSeenAt: row.last_seen_at ?? null,
      ipAddress:  row.ip_address  ?? null,
      userAgent:  row.user_agent  ?? null
    }
  }
}
```

---

## Acceptance Criteria

- [x] File tồn tại, TypeScript compile sạch
- [x] `createSession()` insert vào `orca_sessions`, return `OrcaSession` với đúng TTL (8h)
- [x] `validateSession()` trả về null nếu expired và xoá session đó
- [x] `validateSession()` cập nhật `last_seen_at` khi valid
- [x] `revokeAllUserSessions()` trả về số lượng sessions bị xoá
- [x] `cleanupExpired()` chỉ xoá sessions có `expires_at < now()`
