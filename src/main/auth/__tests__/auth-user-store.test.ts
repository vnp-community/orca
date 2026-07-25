/**
 * AuthUserStore Unit Tests
 *
 * Tests run against a real SQLite in-memory database with all migrations applied.
 * Covers: createLocalUser (bcrypt), verifyPassword, upsertSsoUser, deactivateUser, countAdmins.
 */

import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { SqliteAdapter } from '../../db/sqlite/sqlite-adapter'
import { MigrationRunner } from '../../db/migrations/runner'
import { ALL_MIGRATIONS } from '../../db/migrations/index'
import { AuthUserStore } from '../auth-user-store'

describe('AuthUserStore', () => {
  let db: SqliteAdapter
  let store: AuthUserStore

  beforeEach(async () => {
    db = new SqliteAdapter(':memory:')
    const runner = new MigrationRunner(db, ALL_MIGRATIONS)
    await runner.migrate()
    store = new AuthUserStore(db)
  })

  afterEach(() => db.close())

  // ── createLocalUser ────────────────────────────────────────────────────────

  describe('createLocalUser', () => {
    it('hashes password with bcrypt before storing (never plaintext)', async () => {
      const user = await store.createLocalUser({
        email: 'alice@test.com', name: 'Alice', password: 'secret123', role: 'developer'
      })
      const stmt = await db.prepare('SELECT password_hash FROM orca_users WHERE id = ?')
      const raw = await stmt.get(user.id) as Record<string, unknown>
      expect(raw['password_hash']).not.toBe('secret123')
      expect(String(raw['password_hash'])).toMatch(/^\$2[ab]\$/)  // bcrypt prefix
    })

    it('returns OrcaSessionUser without password_hash field', async () => {
      const user = await store.createLocalUser({
        email: 'bob@test.com', name: 'Bob', password: 'pw', role: 'developer'
      })
      expect((user as Record<string, unknown>)['password_hash']).toBeUndefined()
      expect(user.id).toBeTruthy()
      expect(user.email).toBe('bob@test.com')
      expect(user.provider).toBe('none')
    })

    it('throws on duplicate email (UNIQUE constraint)', async () => {
      await store.createLocalUser({ email: 'dup@test.com', name: 'A', password: 'pw', role: 'developer' })
      await expect(
        store.createLocalUser({ email: 'dup@test.com', name: 'B', password: 'pw', role: 'developer' })
      ).rejects.toThrow()
    })

    it('stores user as is_active = 1', async () => {
      const user = await store.createLocalUser({ email: 'active@test.com', name: 'C', password: 'pw', role: 'developer' })
      const stmt = await db.prepare('SELECT is_active FROM orca_users WHERE id = ?')
      const row = await stmt.get(user.id) as Record<string, unknown>
      expect(row['is_active']).toBe(1)
    })
  })

  // ── verifyPassword ─────────────────────────────────────────────────────────

  describe('verifyPassword', () => {
    it('returns OrcaSessionUser on correct password', async () => {
      await store.createLocalUser({ email: 'v@test.com', name: 'V', password: 'correct', role: 'developer' })
      const user = await store.verifyPassword('v@test.com', 'correct')
      expect(user).not.toBeNull()
      expect(user!.email).toBe('v@test.com')
      expect((user as Record<string, unknown>)['password_hash']).toBeUndefined()
    })

    it('returns null on wrong password', async () => {
      await store.createLocalUser({ email: 'w@test.com', name: 'W', password: 'right', role: 'developer' })
      expect(await store.verifyPassword('w@test.com', 'wrong')).toBeNull()
    })

    it('returns null for unknown email', async () => {
      expect(await store.verifyPassword('unknown@test.com', 'pw')).toBeNull()
    })

    it('returns null for deactivated user even with correct password', async () => {
      const user = await store.createLocalUser({ email: 'inactive@test.com', name: 'X', password: 'pw', role: 'developer' })
      await store.deactivateUser(user.id)
      expect(await store.verifyPassword('inactive@test.com', 'pw')).toBeNull()
    })

    it('returns null for SSO user (no password_hash)', async () => {
      // SSO user cannot login with local password
      await store.upsertSsoUser({ email: 'sso@test.com', name: 'SSO', provider: 'github', providerUserId: 'gh-999' })
      expect(await store.verifyPassword('sso@test.com', 'anything')).toBeNull()
    })
  })

  // ── upsertSsoUser ──────────────────────────────────────────────────────────

  describe('upsertSsoUser', () => {
    it('creates new user on first SSO login', async () => {
      const user = await store.upsertSsoUser({
        email: 'sso1@github.com', name: 'SSO User', provider: 'github', providerUserId: 'gh-1'
      })
      expect(user.id).toBeTruthy()
      expect(user.provider).toBe('github')
      expect(user.role).toBe('developer')  // default role for SSO
    })

    it('updates name on subsequent SSO login (same provider+providerUserId)', async () => {
      await store.upsertSsoUser({ email: 'sso2@g.com', name: 'Old Name', provider: 'google', providerUserId: 'g-42' })
      await store.upsertSsoUser({ email: 'sso2@g.com', name: 'New Name', provider: 'google', providerUserId: 'g-42' })

      const users = await store.listUsers()
      const found = users.find(u => u.email === 'sso2@g.com')
      expect(found!.name).toBe('New Name')
    })

    it('does not create duplicate rows for same provider+providerUserId', async () => {
      await store.upsertSsoUser({ email: 'dup-sso@g.com', name: 'A', provider: 'google', providerUserId: 'g-dup' })
      await store.upsertSsoUser({ email: 'dup-sso@g.com', name: 'B', provider: 'google', providerUserId: 'g-dup' })
      const rows = await db.query("SELECT COUNT(*) AS n FROM orca_users WHERE email = 'dup-sso@g.com'")
      expect(rows[0]!['n']).toBe(1)
    })

    it('preserves existing role on update (does not reset to developer)', async () => {
      // First create a user via upsert (gets developer role)
      const first = await store.upsertSsoUser({ email: 'lead@g.com', name: 'Lead', provider: 'github', providerUserId: 'gh-lead' })
      // Manually promote to 'lead'
      const promoteStmt = await db.prepare(`UPDATE orca_users SET role = 'lead' WHERE id = ?`)
      await promoteStmt.run(first.id)
      // Second SSO login should not reset role
      const second = await store.upsertSsoUser({ email: 'lead@g.com', name: 'Lead Updated', provider: 'github', providerUserId: 'gh-lead' })
      expect(second.role).toBe('lead')
    })
  })

  // ── deactivateUser ─────────────────────────────────────────────────────────

  describe('deactivateUser', () => {
    it('sets is_active = 0 without deleting the row', async () => {
      const user = await store.createLocalUser({ email: 'del@test.com', name: 'D', password: 'pw', role: 'developer' })
      await store.deactivateUser(user.id)
      const stmt = await db.prepare('SELECT is_active FROM orca_users WHERE id = ?')
      const row = await stmt.get(user.id) as Record<string, unknown>
      expect(row['is_active']).toBe(0)
    })

    it('user still appears in listUsers() but NOT in listActiveUsers()', async () => {
      const user = await store.createLocalUser({ email: 'ia@test.com', name: 'I', password: 'pw', role: 'developer' })
      await store.deactivateUser(user.id)
      const all    = await store.listUsers()
      const active = await store.listActiveUsers()
      expect(all.some(u => u.id === user.id)).toBe(true)
      expect(active.some(u => u.id === user.id)).toBe(false)
    })

    it('is idempotent (safe to call multiple times)', async () => {
      const user = await store.createLocalUser({ email: 'idem@test.com', name: 'I', password: 'pw', role: 'developer' })
      await store.deactivateUser(user.id)
      await expect(store.deactivateUser(user.id)).resolves.toBeUndefined()
    })
  })

  // ── countAdmins ────────────────────────────────────────────────────────────

  describe('countAdmins', () => {
    it('returns 0 when no admin users exist', async () => {
      expect(await store.countAdmins()).toBe(0)
    })

    it('returns 1 when one active admin exists', async () => {
      await store.createLocalUser({ email: 'admin@test.com', name: 'Admin', password: 'pw', role: 'admin' })
      expect(await store.countAdmins()).toBe(1)
    })

    it('does not count deactivated admins', async () => {
      const admin = await store.createLocalUser({ email: 'ex-admin@test.com', name: 'Ex', password: 'pw', role: 'admin' })
      await store.deactivateUser(admin.id)
      expect(await store.countAdmins()).toBe(0)
    })

    it('does not count non-admin users', async () => {
      await store.createLocalUser({ email: 'dev@test.com', name: 'Dev', password: 'pw', role: 'developer' })
      expect(await store.countAdmins()).toBe(0)
    })
  })

  // ── getUser / listUsers ────────────────────────────────────────────────────

  describe('getUser', () => {
    it('returns user by id', async () => {
      const created = await store.createLocalUser({ email: 'get@test.com', name: 'G', password: 'pw', role: 'developer' })
      const found = await store.getUser(created.id)
      expect(found).not.toBeNull()
      expect(found!.email).toBe('get@test.com')
    })

    it('returns null for unknown id', async () => {
      expect(await store.getUser('non-existent-id')).toBeNull()
    })
  })
})
