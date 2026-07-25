/**
 * Server tests — F24 Per-User Process Sandbox + F25 Admin Panel
 *
 * Tests:
 *   - Admin stats, user CRUD, session management, audit log (F25)
 *   - Session isolation between users (F24)
 *
 * Run:
 *   ORCA_SERVER_URL=http://172.20.2.39:6769 \
 *   pnpm vitest run --config tests/server/vitest.config.ts tests/server/admin.spec.ts
 */

import { describe, it, expect, beforeAll, afterAll } from 'vitest'
import {
  apiFetch,
  loginAs,
  loginAsAdmin,
  adminCreateUser,
  adminDeleteUser,
  withTestUser,
  type AdminStats,
  type UserRecord,
  type AuditEntry
} from './helpers'

// ─── F25: Dashboard Stats ─────────────────────────────────────────────────────

describe('F25 — Admin Panel: GET /admin/api/stats', () => {
  let adminCookie: string

  beforeAll(async () => {
    adminCookie = await loginAsAdmin()
  })

  it('returns totalUsers ≥ 1 and activeSessions ≥ 0', async () => {
    const { status, body } = await apiFetch<AdminStats>('/admin/api/stats', {
      cookie: adminCookie
    })
    expect(status).toBe(200)
    expect(body.totalUsers).toBeGreaterThanOrEqual(1)
    expect(body.activeSessions).toBeGreaterThanOrEqual(0)
  })

  it('activeSessions increments after a new login', async () => {
    const { body: before } = await apiFetch<AdminStats>('/admin/api/stats', {
      cookie: adminCookie
    })

    // Create a new session
    await withTestUser(adminCookie, 'stats-check', async (_user, _newCookie) => {
      const { body: after } = await apiFetch<AdminStats>('/admin/api/stats', {
        cookie: adminCookie
      })
      expect(after.activeSessions).toBeGreaterThan(before.activeSessions)
    })
  })
})

// ─── F25: User Management ─────────────────────────────────────────────────────

describe('F25 — Admin Panel: User CRUD', () => {
  let adminCookie: string
  const testEmail = `e2e-crud-${Date.now()}@test.orca.local`
  let createdUser: UserRecord

  beforeAll(async () => {
    adminCookie = await loginAsAdmin()
  })

  afterAll(async () => {
    if (createdUser?.id) {
      await adminDeleteUser(adminCookie, createdUser.id).catch(() => {/* already deleted */})
    }
  })

  it('GET /admin/api/users returns array containing admin', async () => {
    const { status, body } = await apiFetch<UserRecord[]>('/admin/api/users', {
      cookie: adminCookie
    })
    expect(status).toBe(200)
    expect(Array.isArray(body)).toBe(true)
    expect(body.some((u) => u.role === 'admin')).toBe(true)
  })

  it('POST /admin/api/users creates a developer user → 201', async () => {
    const { status, body } = await apiFetch<UserRecord>('/admin/api/users', {
      method: 'POST',
      cookie: adminCookie,
      body: JSON.stringify({ email: testEmail, password: 'CrudTest@2025', role: 'developer' })
    })
    expect(status).toBe(201)
    expect(body).toMatchObject({ id: expect.any(String), email: testEmail, role: 'developer' })
    createdUser = body
  })

  it('created user appears in GET /admin/api/users list', async () => {
    const { body } = await apiFetch<UserRecord[]>('/admin/api/users', { cookie: adminCookie })
    expect(body.some((u) => u.id === createdUser.id)).toBe(true)
  })

  it('POST with duplicate email → 409', async () => {
    const { status } = await apiFetch('/admin/api/users', {
      method: 'POST',
      cookie: adminCookie,
      body: JSON.stringify({ email: testEmail, password: 'AnotherPass@2025', role: 'developer' })
    })
    expect(status).toBe(409)
  })

  it('POST without email → 400', async () => {
    const { status } = await apiFetch('/admin/api/users', {
      method: 'POST',
      cookie: adminCookie,
      body: JSON.stringify({ password: 'pass', role: 'developer' })
    })
    expect(status).toBe(400)
  })

  it('DELETE /admin/api/users/:id deactivates user and blocks login', async () => {
    const { status } = await apiFetch(`/admin/api/users/${createdUser.id}`, {
      method: 'DELETE',
      cookie: adminCookie
    })
    expect([200, 204]).toContain(status)

    // Deactivated user cannot login
    const loginRes = await apiFetch('/auth/local', {
      method: 'POST',
      body: JSON.stringify({ email: testEmail, password: 'CrudTest@2025' })
    })
    expect(loginRes.status).toBe(401)
    createdUser = { ...createdUser, id: '' } // prevent double-cleanup
  })

  it('DELETE non-existent user → 404', async () => {
    const { status } = await apiFetch('/admin/api/users/non-existent-id-xyz', {
      method: 'DELETE',
      cookie: adminCookie
    })
    expect([404, 400]).toContain(status)
  })
})

// ─── F25: Session Management ──────────────────────────────────────────────────

describe('F25 — Admin Panel: Session Management', () => {
  let adminCookie: string

  beforeAll(async () => {
    adminCookie = await loginAsAdmin()
  })

  it('DELETE /admin/api/users/:id/sessions kills all sessions for a user', async () => {
    await withTestUser(adminCookie, 'session-kill', async (user, devCookie) => {
      // Verify session valid
      const { status: before } = await apiFetch('/auth/me', { cookie: devCookie })
      expect(before).toBe(200)

      // Admin kills all sessions
      const { status: killStatus } = await apiFetch(
        `/admin/api/users/${user.id}/sessions`,
        { method: 'DELETE', cookie: adminCookie }
      )
      expect([200, 204]).toContain(killStatus)

      // Session should be invalid
      const { status: after } = await apiFetch('/auth/me', { cookie: devCookie })
      expect(after).toBe(401)
    })
  })

  it('killing user A sessions does not invalidate user B sessions', async () => {
    let userACookie = ''
    let userBCookie = ''
    let userAId = ''

    const userAEmail = `e2e-isol-a-${Date.now()}@test.orca.local`
    const userBEmail = `e2e-isol-b-${Date.now()}@test.orca.local`

    const userA = await adminCreateUser(adminCookie, userAEmail, 'PassA@2025')
    const userB = await adminCreateUser(adminCookie, userBEmail, 'PassB@2025')
    userAId = userA.id

    try {
      userACookie = await loginAs(userAEmail, 'PassA@2025')
      userBCookie = await loginAs(userBEmail, 'PassB@2025')

      // Kill user A sessions
      await apiFetch(`/admin/api/users/${userA.id}/sessions`, {
        method: 'DELETE',
        cookie: adminCookie
      })

      // User A session invalid
      const { status: aStatus } = await apiFetch('/auth/me', { cookie: userACookie })
      expect(aStatus).toBe(401)

      // User B session still valid
      const { status: bStatus } = await apiFetch('/auth/me', { cookie: userBCookie })
      expect(bStatus).toBe(200)
    } finally {
      await adminDeleteUser(adminCookie, userA.id)
      await adminDeleteUser(adminCookie, userB.id)
    }
  })
})

// ─── F25: Audit Log ───────────────────────────────────────────────────────────

describe('F25 — Admin Panel: Audit Log', () => {
  let adminCookie: string

  beforeAll(async () => {
    adminCookie = await loginAsAdmin()
  })

  it('GET /admin/api/audit returns entries array', async () => {
    const { status, body } = await apiFetch<AuditEntry[] | { entries: AuditEntry[] }>(
      '/admin/api/audit',
      { cookie: adminCookie }
    )
    expect(status).toBe(200)
    const entries = Array.isArray(body) ? body : body.entries ?? []
    expect(Array.isArray(entries)).toBe(true)
  })

  it('audit log contains login.success events', async () => {
    const { body } = await apiFetch<AuditEntry[] | { entries: AuditEntry[] }>(
      '/admin/api/audit',
      { cookie: adminCookie }
    )
    const entries = Array.isArray(body) ? body : body.entries ?? []
    const loginEvents = entries.filter((e) => e.action === 'login.success')
    expect(loginEvents.length).toBeGreaterThanOrEqual(1)
  })

  it('GET /admin/api/audit?action=login.success filters correctly', async () => {
    const res = await fetch(
      `${process.env['ORCA_SERVER_URL'] ?? 'http://172.20.2.39:6769'}/admin/api/audit?action=login.success`,
      { headers: { Cookie: adminCookie } }
    )
    expect(res.status).toBe(200)
    const body = await res.json() as AuditEntry[] | { entries: AuditEntry[] }
    const entries = Array.isArray(body) ? body : body.entries ?? []
    for (const entry of entries) {
      expect(entry.action).toBe('login.success')
    }
  })

  it('audit log records user creation actions', async () => {
    const newEmail = `e2e-audit-${Date.now()}@test.orca.local`
    const user = await adminCreateUser(adminCookie, newEmail, 'Audit@2025')

    const { body } = await apiFetch<AuditEntry[] | { entries: AuditEntry[] }>(
      '/admin/api/audit',
      { cookie: adminCookie }
    )
    const entries = Array.isArray(body) ? body : body.entries ?? []
    const createEvents = entries.filter((e) => e.action === 'user.create')
    expect(createEvents.length).toBeGreaterThanOrEqual(1)

    await adminDeleteUser(adminCookie, user.id)
  })
})

// ─── F24: Per-User Process Sandbox Isolation ──────────────────────────────────

describe('F24 — Per-User Sandbox: Data Isolation', () => {
  let adminCookie: string

  beforeAll(async () => {
    adminCookie = await loginAsAdmin()
  })

  it('/auth/me returns correct user for each session', async () => {
    const emailA = `e2e-sandbox-a-${Date.now()}@test.orca.local`
    const emailB = `e2e-sandbox-b-${Date.now()}@test.orca.local`

    const userA = await adminCreateUser(adminCookie, emailA, 'SandboxA@2025')
    const userB = await adminCreateUser(adminCookie, emailB, 'SandboxB@2025')

    try {
      const cookieA = await loginAs(emailA, 'SandboxA@2025')
      const cookieB = await loginAs(emailB, 'SandboxB@2025')

      const { body: meA } = await apiFetch<{ id: string; email: string }>('/auth/me', {
        cookie: cookieA
      })
      const { body: meB } = await apiFetch<{ id: string; email: string }>('/auth/me', {
        cookie: cookieB
      })

      expect(meA.id).toBe(userA.id)
      expect(meB.id).toBe(userB.id)
      expect(meA.id).not.toBe(meB.id)
      expect(meA.email).toBe(emailA)
      expect(meB.email).toBe(emailB)
    } finally {
      await adminDeleteUser(adminCookie, userA.id)
      await adminDeleteUser(adminCookie, userB.id)
    }
  })

  it('deactivating user A does not affect user B login capability', async () => {
    const emailA = `e2e-deact-a-${Date.now()}@test.orca.local`
    const emailB = `e2e-deact-b-${Date.now()}@test.orca.local`

    const userA = await adminCreateUser(adminCookie, emailA, 'DeactA@2025')
    const userB = await adminCreateUser(adminCookie, emailB, 'DeactB@2025')

    try {
      await adminDeleteUser(adminCookie, userA.id)

      // User A login blocked
      const { status: aStatus } = await apiFetch('/auth/local', {
        method: 'POST',
        body: JSON.stringify({ email: emailA, password: 'DeactA@2025' })
      })
      expect(aStatus).toBe(401)

      // User B login still works
      const cookieB = await loginAs(emailB, 'DeactB@2025')
      const { status: bStatus } = await apiFetch('/auth/me', { cookie: cookieB })
      expect(bStatus).toBe(200)
    } finally {
      await adminDeleteUser(adminCookie, userB.id)
    }
  })
})
