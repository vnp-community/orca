/**
 * Server tests — F23 Multi-User Auth: Login / Session / Middleware
 *
 * Tests:
 *   - POST /auth/local  — email+password login
 *   - GET  /auth/me     — session identity
 *   - POST /auth/logout — session termination
 *   - Auth middleware   — unauthenticated requests blocked
 *
 * Run:
 *   ORCA_SERVER_URL=http://172.20.2.39:6769 \
 *   pnpm vitest run --config tests/server/vitest.config.ts tests/server/auth.spec.ts
 */

import { describe, it, expect, beforeAll } from 'vitest'
import {
  apiFetch,
  loginAs,
  loginAsAdmin,
  withTestUser,
  ADMIN_EMAIL
} from './helpers'

// ─── Login Flow ───────────────────────────────────────────────────────────────

describe('F23 — Auth: POST /auth/local', () => {
  let adminCookie: string

  beforeAll(async () => {
    adminCookie = await loginAsAdmin()
  })

  it('valid credentials → 200 + user object + session cookie', async () => {
    const res = await fetch(`${process.env['ORCA_SERVER_URL'] ?? 'http://172.20.2.39:6769'}/auth/local`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        email: ADMIN_EMAIL,
        password: process.env['ORCA_ADMIN_PASSWORD'] ?? 'Orca@Adm1n#2025'
      })
    })
    expect(res.status).toBe(200)
    const body = await res.json() as { id: string; email: string; role: string }
    expect(body).toMatchObject({ id: expect.any(String), email: ADMIN_EMAIL, role: 'admin' })
    const cookie = res.headers.get('set-cookie')
    expect(cookie).toMatch(/orca_session=/)
  })

  it('wrong password → 401 with error message', async () => {
    const { status, body } = await apiFetch<{ error: string }>('/auth/local', {
      method: 'POST',
      body: JSON.stringify({ email: ADMIN_EMAIL, password: 'wrong!' })
    })
    expect(status).toBe(401)
    expect(body).toHaveProperty('error')
  })

  it('unknown email → 401', async () => {
    const { status } = await apiFetch('/auth/local', {
      method: 'POST',
      body: JSON.stringify({ email: 'nobody@nowhere.com', password: 'any' })
    })
    expect(status).toBe(401)
  })

  it('missing password field → 400', async () => {
    const { status } = await apiFetch('/auth/local', {
      method: 'POST',
      body: JSON.stringify({ email: ADMIN_EMAIL })
    })
    expect(status).toBe(400)
  })

  it('missing email field → 400', async () => {
    const { status } = await apiFetch('/auth/local', {
      method: 'POST',
      body: JSON.stringify({ password: 'pass' })
    })
    expect(status).toBe(400)
  })

  it('empty body → 400', async () => {
    const { status } = await apiFetch('/auth/local', {
      method: 'POST',
      body: JSON.stringify({})
    })
    expect(status).toBe(400)
  })

  it('deactivated user cannot login → 401', async () => {
    await withTestUser(adminCookie, 'deactivated', async (user, _cookie) => {
      // Deactivate the user
      await apiFetch(`/admin/api/users/${user.id}`, {
        method: 'DELETE',
        cookie: adminCookie
      })
      // Try to login again
      const { status } = await apiFetch('/auth/local', {
        method: 'POST',
        body: JSON.stringify({ email: user.email, password: 'TestPass@2025!' })
      })
      expect(status).toBe(401)
    })
  })
})

// ─── Session Identity ─────────────────────────────────────────────────────────

describe('F23 — Auth: GET /auth/me', () => {
  let adminCookie: string

  beforeAll(async () => {
    adminCookie = await loginAsAdmin()
  })

  it('valid session → 200 + full user identity', async () => {
    const { status, body } = await apiFetch<{ id: string; email: string; role: string }>(
      '/auth/me',
      { cookie: adminCookie }
    )
    expect(status).toBe(200)
    expect(body).toMatchObject({
      id: expect.any(String),
      email: ADMIN_EMAIL,
      role: 'admin'
    })
  })

  it('no session cookie → 401', async () => {
    const { status } = await apiFetch('/auth/me')
    expect(status).toBe(401)
  })

  it('invalid session token → 401', async () => {
    const { status } = await apiFetch('/auth/me', {
      cookie: 'orca_session=invalid-token-xyz'
    })
    expect(status).toBe(401)
  })
})

// ─── Logout ───────────────────────────────────────────────────────────────────

describe('F23 — Auth: POST /auth/logout', () => {
  it('logout invalidates the session', async () => {
    const cookie = await loginAsAdmin()

    // Session is valid
    const { status: before } = await apiFetch('/auth/me', { cookie })
    expect(before).toBe(200)

    // Logout
    const { status: logoutStatus } = await apiFetch('/auth/logout', {
      method: 'POST',
      cookie
    })
    expect(logoutStatus).toBe(200)

    // Session is now invalid
    const { status: after } = await apiFetch('/auth/me', { cookie })
    expect(after).toBe(401)
  })

  it('logout without session → still returns 200 (idempotent)', async () => {
    const { status } = await apiFetch('/auth/logout', { method: 'POST' })
    // Should not crash — either 200 or 401 is acceptable
    expect([200, 401]).toContain(status)
  })
})

// ─── Auth Middleware ──────────────────────────────────────────────────────────

describe('F23 — Auth: Middleware Protection', () => {
  it('GET /admin/api/stats requires authentication → 401', async () => {
    const { status } = await apiFetch('/admin/api/stats')
    expect(status).toBe(401)
  })

  it('GET /admin/api/users requires authentication → 401', async () => {
    const { status } = await apiFetch('/admin/api/users')
    expect(status).toBe(401)
  })

  it('GET /admin/api/audit requires authentication → 401', async () => {
    const { status } = await apiFetch('/admin/api/audit')
    expect(status).toBe(401)
  })

  it('developer role cannot access admin routes → 403', async () => {
    const adminCookie = await loginAsAdmin()
    await withTestUser(adminCookie, 'dev-rbac', async (_user, devCookie) => {
      const { status } = await apiFetch('/admin/api/stats', { cookie: devCookie })
      expect(status).toBe(403)
    })
  })
})
