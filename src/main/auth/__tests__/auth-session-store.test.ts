/**
 * AuthSessionStore Unit Tests
 *
 * Tests run against a real SQLite in-memory database (no mocks for IDatabase).
 * Covers: createSession, validateSession, revokeSession, revokeAllUserSessions, cleanupExpired.
 */

import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { SqliteAdapter } from '../../db/sqlite/sqlite-adapter'
import { MigrationRunner } from '../../db/migrations/runner'
import { ALL_MIGRATIONS } from '../../db/migrations/index'
import { AuthSessionStore } from '../auth-session-store'

describe('AuthSessionStore', () => {
  let db: SqliteAdapter
  let store: AuthSessionStore

  beforeEach(async () => {
    db = new SqliteAdapter(':memory:')
    const runner = new MigrationRunner(db, ALL_MIGRATIONS)
    await runner.migrate()
    store = new AuthSessionStore(db)
  })

  afterEach(() => db.close())

  // ── Test Helpers ───────────────────────────────────────────────────────────

  async function seedUser(
    id = 'user-1',
    email = 'alice@test.com',
    role = 'developer'
  ) {
    const stmt = await db.prepare(`
      INSERT INTO orca_users (id, email, name, role, provider, created_at, is_active)
      VALUES (?, ?, 'Test User', ?, 'none', ?, 1)
    `)
    await stmt.run(id, email, role, Date.now())
    return { id, email, role }
  }

  async function createSession(userId = 'user-1', email = 'alice@test.com') {
    return store.createSession({
      userId, userEmail: email, role: 'developer',
      ipAddress: '127.0.0.1', userAgent: 'vitest'
    })
  }

  // ── createSession ──────────────────────────────────────────────────────────

  describe('createSession', () => {
    it('returns OrcaSession with a 64-hex sessionId', async () => {
      await seedUser()
      const session = await createSession()
      expect(session.sessionId).toHaveLength(64)
      expect(session.sessionId).toMatch(/^[a-f0-9]+$/)
    })

    it('sets expiresAt 8 hours after createdAt', async () => {
      await seedUser()
      const session = await createSession()
      expect(session.expiresAt - session.createdAt).toBe(8 * 60 * 60 * 1000)
    })

    it('sets lastSeenAt to null', async () => {
      await seedUser()
      const session = await createSession()
      expect(session.lastSeenAt).toBeNull()
    })

    it('persists session to SQLite', async () => {
      await seedUser()
      const created = await createSession()
      const found = await store.getSession(created.sessionId)
      expect(found).not.toBeNull()
      expect(found!.userId).toBe('user-1')
    })

    it('generates unique sessionIds for repeated calls', async () => {
      await seedUser()
      const s1 = await createSession()
      const s2 = await createSession()
      expect(s1.sessionId).not.toBe(s2.sessionId)
    })
  })

  // ── validateSession ────────────────────────────────────────────────────────

  describe('validateSession', () => {
    it('returns session for a valid non-expired sessionId', async () => {
      await seedUser()
      const created = await createSession()
      const session = await store.validateSession(created.sessionId)
      expect(session).not.toBeNull()
      expect(session!.userId).toBe('user-1')
    })

    it('returns null for unknown sessionId', async () => {
      const session = await store.validateSession('non-existent-id')
      expect(session).toBeNull()
    })

    it('returns null for an expired session', async () => {
      await seedUser()
      // Insert an already-expired session directly
      const expiredStmt = await db.prepare(
        `INSERT INTO orca_sessions VALUES ('exp-sid','user-1',?,?,NULL,'127.0.0.1','ua')`
      )
      const past = Date.now() - 1000
      await expiredStmt.run(past, past)

      const session = await store.validateSession('exp-sid')
      expect(session).toBeNull()
    })

    it('deletes expired session on validation attempt', async () => {
      await seedUser()
      const expiredStmt = await db.prepare(
        `INSERT INTO orca_sessions VALUES ('del-sid','user-1',?,?,NULL,'127.0.0.1','ua')`
      )
      const past = Date.now() - 1000
      await expiredStmt.run(past, past)

      await store.validateSession('del-sid')
      const row = await store.getSession('del-sid')
      expect(row).toBeNull()
    })

    it('updates lastSeenAt on a valid session', async () => {
      await seedUser()
      const created = await createSession()
      const before = Date.now()
      await store.validateSession(created.sessionId)
      const updated = await store.getSession(created.sessionId)
      expect(updated!.lastSeenAt).toBeGreaterThanOrEqual(before)
    })
  })

  // ── revokeSession ──────────────────────────────────────────────────────────

  describe('revokeSession', () => {
    it('deletes the session from the store', async () => {
      await seedUser()
      const session = await createSession()
      await store.revokeSession(session.sessionId)
      expect(await store.getSession(session.sessionId)).toBeNull()
    })

    it('is idempotent for non-existent sessionId', async () => {
      await expect(store.revokeSession('ghost-id')).resolves.toBeUndefined()
    })
  })

  // ── revokeAllUserSessions ──────────────────────────────────────────────────

  describe('revokeAllUserSessions', () => {
    it('deletes all sessions for user and returns count', async () => {
      await seedUser('user-a', 'a@test.com')
      await store.createSession({ userId: 'user-a', userEmail: 'a@test.com', role: 'developer', ipAddress: '1.1.1.1', userAgent: 'ua' })
      await store.createSession({ userId: 'user-a', userEmail: 'a@test.com', role: 'developer', ipAddress: '1.1.1.2', userAgent: 'ua' })

      const count = await store.revokeAllUserSessions('user-a')
      expect(count).toBe(2)
    })

    it('leaves other users sessions intact', async () => {
      await seedUser('user-a', 'a@test.com')
      await seedUser('user-b', 'b@test.com')
      await store.createSession({ userId: 'user-a', userEmail: 'a@test.com', role: 'developer', ipAddress: '1.1.1.1', userAgent: 'ua' })
      const s2 = await store.createSession({ userId: 'user-b', userEmail: 'b@test.com', role: 'developer', ipAddress: '2.2.2.2', userAgent: 'ua' })

      await store.revokeAllUserSessions('user-a')

      const remaining = await store.listUserSessions('user-b')
      expect(remaining).toHaveLength(1)
      expect(remaining[0]!.sessionId).toBe(s2.sessionId)
    })

    it('returns 0 for user with no sessions', async () => {
      await seedUser()
      const count = await store.revokeAllUserSessions('user-1')
      expect(count).toBe(0)
    })
  })

  // ── cleanupExpired ─────────────────────────────────────────────────────────

  describe('cleanupExpired', () => {
    it('removes expired sessions and leaves active ones intact', async () => {
      await seedUser()
      const active = await createSession()

      // Insert 2 expired sessions
      const expStmt = await db.prepare(
        `INSERT INTO orca_sessions VALUES (?, 'user-1', ?, ?, NULL, NULL, NULL)`
      )
      const past = Date.now() - 1000
      await expStmt.run('exp-1', past, past)
      await expStmt.run('exp-2', past, past)

      const removed = await store.cleanupExpired()
      expect(removed).toBe(2)
      expect(await store.getSession(active.sessionId)).not.toBeNull()
      expect(await store.getSession('exp-1')).toBeNull()
      expect(await store.getSession('exp-2')).toBeNull()
    })

    it('returns 0 when there are no expired sessions', async () => {
      await seedUser()
      await createSession()
      const removed = await store.cleanupExpired()
      expect(removed).toBe(0)
    })
  })
})
