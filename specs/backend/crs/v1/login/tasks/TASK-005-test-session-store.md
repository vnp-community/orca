# TASK-005: Tạo file `src/main/auth/__tests__/auth-session-store.test.ts`

> **Status:** ✅ DONE (2026-07-24) — 17/17 tests pass

**Phase:** 1 — Auth Foundation
**Solution:** [SOL-LG-001](../solutions/SOL-LG-001-auth-session.md) §3.1
**Depends on:** TASK-001, TASK-002, TASK-003
**Blocks:** (không)

---

## Mục tiêu

Viết test suite cho `AuthSessionStore` — chạy được với `pnpm test` mà không cần Electron.

---

## File cần tạo

**Path:** `src/main/auth/__tests__/auth-session-store.test.ts`

---

## Nội dung (copy từ SOL-LG-001 §3.1, đầy đủ)

```typescript
// src/main/auth/__tests__/auth-session-store.test.ts
import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { mkdtempSync, rmSync } from 'node:fs'
import { join } from 'node:path'
import { tmpdir } from 'node:os'
import { SqliteAdapter } from '../../db/sqlite/sqlite-adapter'
import { runMigrations } from '../../db/migrations/runner'
import { AuthSessionStore } from '../auth-session-store'

describe('AuthSessionStore', () => {
  let tmpDir: string
  let db: SqliteAdapter
  let store: AuthSessionStore

  beforeEach(async () => {
    tmpDir = mkdtempSync(join(tmpdir(), 'orca-auth-session-test-'))
    db = new SqliteAdapter(join(tmpDir, 'test.db'))
    await runMigrations(db)
    store = new AuthSessionStore(db)
  })

  afterEach(() => {
    db.close()
    rmSync(tmpDir, { recursive: true, force: true })
  })

  // Seed helper
  async function seedUser(id = 'user-1', email = 'a@test.com', role = 'developer') {
    db.prepare(`
      INSERT INTO orca_users (id, email, name, role, provider, created_at, is_active)
      VALUES (?, ?, 'Test User', ?, 'none', ?, 1)
    `).run(id, email, role, Date.now())
  }

  describe('createSession', () => {
    it('creates session with correct 8h TTL', async () => {
      await seedUser()
      const session = await store.createSession({
        userId: 'user-1', userEmail: 'a@test.com', role: 'developer',
        ipAddress: '127.0.0.1', userAgent: 'test-agent'
      })
      expect(session.sessionId).toHaveLength(64)
      expect(session.expiresAt - session.createdAt).toBe(8 * 60 * 60 * 1000)
      expect(session.lastSeenAt).toBeNull()
    })

    it('persists session to SQLite', async () => {
      await seedUser()
      const created = await store.createSession({
        userId: 'user-1', userEmail: 'a@test.com', role: 'developer',
        ipAddress: '127.0.0.1', userAgent: 'test'
      })
      const found = store.getSession(created.sessionId)
      expect(found).not.toBeNull()
      expect(found!.userId).toBe('user-1')
    })
  })

  describe('validateSession', () => {
    it('returns session for valid non-expired sessionId', async () => {
      await seedUser()
      const created = await store.createSession({
        userId: 'user-1', userEmail: 'a@test.com', role: 'developer',
        ipAddress: '127.0.0.1', userAgent: 'test'
      })
      const session = store.validateSession(created.sessionId)
      expect(session).not.toBeNull()
      expect(session!.userId).toBe('user-1')
    })

    it('returns null for expired session', async () => {
      await seedUser()
      const pastExpiry = Date.now() - 1000
      db.prepare(
        `INSERT INTO orca_sessions VALUES ('expired-sid','user-1',${Date.now()},${pastExpiry},NULL,NULL,NULL)`
      ).run()
      const session = store.validateSession('expired-sid')
      expect(session).toBeNull()
    })

    it('returns null for unknown sessionId', async () => {
      expect(store.validateSession('non-existent')).toBeNull()
    })

    it('updates lastSeenAt on valid session', async () => {
      await seedUser()
      const created = await store.createSession({
        userId: 'user-1', userEmail: 'a@test.com', role: 'developer',
        ipAddress: '127.0.0.1', userAgent: 'test'
      })
      const before = Date.now()
      store.validateSession(created.sessionId)
      const updated = store.getSession(created.sessionId)
      expect(updated!.lastSeenAt).toBeGreaterThanOrEqual(before)
    })

    it('deletes expired session on validation attempt', async () => {
      await seedUser()
      const pastExpiry = Date.now() - 1000
      db.prepare(
        `INSERT INTO orca_sessions VALUES ('ex-sid','user-1',${Date.now()},${pastExpiry},NULL,NULL,NULL)`
      ).run()
      store.validateSession('ex-sid')
      expect(store.getSession('ex-sid')).toBeNull()
    })
  })

  describe('revokeSession', () => {
    it('deletes session from store', async () => {
      await seedUser()
      const session = await store.createSession({
        userId: 'user-1', userEmail: 'a@test.com', role: 'developer',
        ipAddress: '127.0.0.1', userAgent: 'test'
      })
      store.revokeSession(session.sessionId)
      expect(store.getSession(session.sessionId)).toBeNull()
    })

    it('is idempotent for non-existent session', () => {
      expect(() => store.revokeSession('non-existent')).not.toThrow()
    })
  })

  describe('revokeAllUserSessions', () => {
    it('deletes all sessions for user, leaves others intact', async () => {
      await seedUser('user-1', 'a@test.com')
      await seedUser('user-2', 'b@test.com')
      await store.createSession({ userId: 'user-1', userEmail: 'a@test.com', role: 'developer', ipAddress: '1.1.1.1', userAgent: 'ua' })
      await store.createSession({ userId: 'user-1', userEmail: 'a@test.com', role: 'developer', ipAddress: '1.1.1.2', userAgent: 'ua' })
      const s2 = await store.createSession({ userId: 'user-2', userEmail: 'b@test.com', role: 'developer', ipAddress: '2.2.2.2', userAgent: 'ua' })

      const count = store.revokeAllUserSessions('user-1')
      expect(count).toBe(2)
      expect(store.listUserSessions('user-1')).toHaveLength(0)
      expect(store.listUserSessions('user-2')).toHaveLength(1)
    })
  })

  describe('cleanupExpired', () => {
    it('removes expired sessions without touching active ones', async () => {
      await seedUser()
      const active = await store.createSession({
        userId: 'user-1', userEmail: 'a@test.com', role: 'developer',
        ipAddress: '1.2.3.4', userAgent: 'ua'
      })
      const pastExpiry = Date.now() - 1000
      db.prepare(`INSERT INTO orca_sessions VALUES ('exp1','user-1',${Date.now()},${pastExpiry},NULL,NULL,NULL)`).run()
      db.prepare(`INSERT INTO orca_sessions VALUES ('exp2','user-1',${Date.now()},${pastExpiry},NULL,NULL,NULL)`).run()

      const removed = store.cleanupExpired()
      expect(removed).toBe(2)
      expect(store.getSession(active.sessionId)).not.toBeNull()
      expect(store.getSession('exp1')).toBeNull()
    })
  })
})
```

---

## Cách chạy test

```bash
pnpm test src/main/auth/__tests__/auth-session-store.test.ts
```

---

## Acceptance Criteria

- [x] Test file tồn tại
- [x] Tất cả describe blocks pass: `createSession`, `validateSession`, `revokeSession`, `revokeAllUserSessions`, `cleanupExpired`
- [x] Tổng số test cases ≥ 10
- [x] Không dùng Electron, không mock IDatabase (dùng real SQLite `:memory:`)
