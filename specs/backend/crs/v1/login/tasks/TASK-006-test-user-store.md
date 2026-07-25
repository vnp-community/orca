# TASK-006: Tạo file `src/main/auth/__tests__/auth-user-store.test.ts`

> **Status:** ✅ DONE (2026-07-24)


**Phase:** 1 — Auth Foundation
**Solution:** [SOL-LG-001](../solutions/SOL-LG-001-auth-session.md) §3.2
**Depends on:** TASK-001, TASK-002, TASK-004
**Blocks:** (không)

---

## Mục tiêu

Viết test suite cho `AuthUserStore` — bao gồm bcrypt hash, SSO upsert, deactivate.

---

## File cần tạo

**Path:** `src/main/auth/__tests__/auth-user-store.test.ts`

---

## Nội dung

```typescript
// src/main/auth/__tests__/auth-user-store.test.ts
import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { mkdtempSync, rmSync } from 'node:fs'
import { join } from 'node:path'
import { tmpdir } from 'node:os'
import { SqliteAdapter } from '../../db/sqlite/sqlite-adapter'
import { runMigrations } from '../../db/migrations/runner'
import { AuthUserStore } from '../auth-user-store'

describe('AuthUserStore', () => {
  let tmpDir: string
  let db: SqliteAdapter
  let store: AuthUserStore

  beforeEach(async () => {
    tmpDir = mkdtempSync(join(tmpdir(), 'orca-auth-user-test-'))
    db = new SqliteAdapter(join(tmpDir, 'test.db'))
    await runMigrations(db)
    store = new AuthUserStore(db)
  })

  afterEach(() => {
    db.close()
    rmSync(tmpDir, { recursive: true, force: true })
  })

  describe('createLocalUser', () => {
    it('hashes password with bcrypt before storing', async () => {
      const user = await store.createLocalUser({
        email: 'alice@test.com', name: 'Alice', password: 'secret123', role: 'developer'
      })
      const raw = db.prepare('SELECT password_hash FROM orca_users WHERE id = ?').get(user.id) as any
      expect(raw.password_hash).not.toBe('secret123')
      expect(raw.password_hash).toMatch(/^\$2[ab]\$/)  // bcrypt prefix
    })

    it('returns user without password_hash field', async () => {
      const user = await store.createLocalUser({
        email: 'bob@test.com', name: 'Bob', password: 'pw', role: 'developer'
      })
      expect((user as any).password_hash).toBeUndefined()
      expect(user.id).toBeTruthy()
      expect(user.email).toBe('bob@test.com')
    })

    it('throws on duplicate email', async () => {
      await store.createLocalUser({ email: 'dup@test.com', name: 'A', password: 'pw', role: 'developer' })
      await expect(
        store.createLocalUser({ email: 'dup@test.com', name: 'B', password: 'pw', role: 'developer' })
      ).rejects.toThrow()
    })
  })

  describe('verifyPassword', () => {
    it('returns user on correct password', async () => {
      await store.createLocalUser({ email: 'v@test.com', name: 'V', password: 'correct', role: 'developer' })
      const user = await store.verifyPassword('v@test.com', 'correct')
      expect(user).not.toBeNull()
      expect(user!.email).toBe('v@test.com')
      expect((user as any).password_hash).toBeUndefined()
    })

    it('returns null on wrong password', async () => {
      await store.createLocalUser({ email: 'w@test.com', name: 'W', password: 'right', role: 'developer' })
      expect(await store.verifyPassword('w@test.com', 'wrong')).toBeNull()
    })

    it('returns null for unknown email', async () => {
      expect(await store.verifyPassword('unknown@test.com', 'pw')).toBeNull()
    })

    it('returns null for deactivated user even with correct password', async () => {
      const user = await store.createLocalUser({ email: 'x@test.com', name: 'X', password: 'pw', role: 'developer' })
      store.deactivateUser(user.id)
      expect(await store.verifyPassword('x@test.com', 'pw')).toBeNull()
    })
  })

  describe('upsertSsoUser', () => {
    it('creates new user on first SSO login', async () => {
      const user = await store.upsertSsoUser({
        email: 'sso@github.com', name: 'SSO User',
        provider: 'github', providerUserId: 'gh-123'
      })
      expect(user.id).toBeTruthy()
      expect(user.provider).toBe('github')
      expect(user.role).toBe('developer')
    })

    it('updates existing user name on subsequent SSO login', async () => {
      await store.upsertSsoUser({ email: 'sso2@github.com', name: 'Old Name', provider: 'github', providerUserId: 'gh-456' })
      await store.upsertSsoUser({ email: 'sso2@github.com', name: 'New Name', provider: 'github', providerUserId: 'gh-456' })

      const users = store.listUsers()
      const found = users.find(u => u.email === 'sso2@github.com')
      expect(found!.name).toBe('New Name')
    })

    it('does not create duplicate user for same provider+providerUserId', async () => {
      await store.upsertSsoUser({ email: 'no-dup@g.com', name: 'A', provider: 'google', providerUserId: 'g-999' })
      await store.upsertSsoUser({ email: 'no-dup@g.com', name: 'A', provider: 'google', providerUserId: 'g-999' })
      const count = (db.prepare(`SELECT COUNT(*) as n FROM orca_users WHERE email = 'no-dup@g.com'`).get() as any).n
      expect(count).toBe(1)
    })
  })

  describe('deactivateUser', () => {
    it('sets is_active = 0 without deleting the row', async () => {
      const user = await store.createLocalUser({ email: 'del@test.com', name: 'D', password: 'pw', role: 'developer' })
      store.deactivateUser(user.id)
      const row = db.prepare('SELECT is_active FROM orca_users WHERE id = ?').get(user.id) as any
      expect(row.is_active).toBe(0)
    })

    it('user still appears in listUsers() but not listActiveUsers()', async () => {
      const user = await store.createLocalUser({ email: 'ia@test.com', name: 'I', password: 'pw', role: 'developer' })
      store.deactivateUser(user.id)
      expect(store.listUsers().some(u => u.id === user.id)).toBe(true)
      expect(store.listActiveUsers().some(u => u.id === user.id)).toBe(false)
    })
  })

  describe('countAdmins', () => {
    it('returns 0 when no admins', async () => {
      expect(store.countAdmins()).toBe(0)
    })

    it('counts only active admins', async () => {
      const admin = await store.createLocalUser({ email: 'a@t.com', name: 'A', password: 'pw', role: 'admin' })
      expect(store.countAdmins()).toBe(1)
      store.deactivateUser(admin.id)
      expect(store.countAdmins()).toBe(0)
    })
  })
})
```

---

## Cách chạy test

```bash
pnpm test src/main/auth/__tests__/auth-user-store.test.ts
```

---

## Acceptance Criteria

- [x] Test file tồn tại
- [x] Tất cả describe blocks pass
- [x] `createLocalUser` test xác nhận bcrypt hash
- [x] `verifyPassword` test xác nhận deactivated user bị từ chối
- [x] `upsertSsoUser` không tạo duplicate row
- [x] Tổng số test cases ≥ 12
