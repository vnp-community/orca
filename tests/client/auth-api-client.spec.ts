/**
 * Client tests — F23 Auth API Client
 *
 * Tests the auth-api-client.ts contract from the browser/web client perspective:
 *   - Login flow (email + password)
 *   - Session identity (/auth/me)
 *   - Logout flow
 *   - Error handling for invalid credentials
 *   - Admin-specific features (admin stats accessible)
 *
 * These simulate what src/renderer/src/auth/auth-api-client.ts does.
 *
 * Run:
 *   ORCA_SERVER_URL=http://172.20.2.39:6769 \
 *   pnpm vitest run --config tests/client/vitest.config.ts tests/client/auth-api-client.spec.ts
 */

import { describe, it, expect, beforeAll, afterAll } from 'vitest'
import {
  clientLogin,
  clientGetMe,
  clientLogout,
  adminLogin,
  adminCreateUser,
  adminDeleteUser,
  ADMIN_EMAIL,
  BASE_URL
} from './helpers'

// ─── Login Flow ───────────────────────────────────────────────────────────────

describe('Auth API Client: Login (POST /auth/local)', () => {
  it('admin login returns user object with role=admin', async () => {
    const { user, cookie } = await clientLogin(
      ADMIN_EMAIL,
      process.env['ORCA_ADMIN_PASSWORD'] ?? 'Orca@Adm1n#2025'
    )
    expect(user.email).toBe(ADMIN_EMAIL)
    expect(user.role).toBe('admin')
    expect(user.id).toBeTruthy()
    expect(cookie).toMatch(/orca_session=/)
  })

  it('login with wrong password throws / returns 401', async () => {
    await expect(clientLogin(ADMIN_EMAIL, 'wrong-password')).rejects.toThrow()
  })

  it('login with unknown email throws / returns 401', async () => {
    await expect(clientLogin('ghost@nowhere.com', 'any')).rejects.toThrow()
  })

  it('developer user login returns role=developer', async () => {
    const adminCookie = await adminLogin()
    const devEmail = `e2e-client-dev-${Date.now()}@test.orca.local`
    const dev = await adminCreateUser(adminCookie, devEmail, 'ClientDev@2025')

    try {
      const { user } = await clientLogin(devEmail, 'ClientDev@2025')
      expect(user.role).toBe('developer')
      expect(user.email).toBe(devEmail)
    } finally {
      await adminDeleteUser(adminCookie, dev.id)
    }
  })
})

// ─── Session Identity ─────────────────────────────────────────────────────────

describe('Auth API Client: Session Identity (GET /auth/me)', () => {
  let cookie: string

  beforeAll(async () => {
    const result = await clientLogin(
      ADMIN_EMAIL,
      process.env['ORCA_ADMIN_PASSWORD'] ?? 'Orca@Adm1n#2025'
    )
    cookie = result.cookie
  })

  it('returns full user identity with valid session', async () => {
    const me = await clientGetMe(cookie)
    expect(me).not.toBeNull()
    expect(me!.email).toBe(ADMIN_EMAIL)
    expect(me!.role).toBe('admin')
  })

  it('returns null (401) when no session', async () => {
    const me = await clientGetMe('')
    expect(me).toBeNull()
  })

  it('returns null (401) with invalid session token', async () => {
    const me = await clientGetMe('orca_session=totally-invalid-token')
    expect(me).toBeNull()
  })
})

// ─── Logout ───────────────────────────────────────────────────────────────────

describe('Auth API Client: Logout (POST /auth/logout)', () => {
  it('logout invalidates session — subsequent /auth/me returns null', async () => {
    const { cookie } = await clientLogin(
      ADMIN_EMAIL,
      process.env['ORCA_ADMIN_PASSWORD'] ?? 'Orca@Adm1n#2025'
    )

    // Verify active
    expect(await clientGetMe(cookie)).not.toBeNull()

    // Logout
    const ok = await clientLogout(cookie)
    expect(ok).toBe(true)

    // Session invalidated
    expect(await clientGetMe(cookie)).toBeNull()
  })
})

// ─── Admin API Client ─────────────────────────────────────────────────────────

describe('Auth API Client: Admin API Access', () => {
  let adminCookie: string

  beforeAll(async () => {
    const { cookie } = await clientLogin(
      ADMIN_EMAIL,
      process.env['ORCA_ADMIN_PASSWORD'] ?? 'Orca@Adm1n#2025'
    )
    adminCookie = cookie
  })

  it('admin can fetch /admin/api/stats', async () => {
    const res = await fetch(`${BASE_URL}/admin/api/stats`, {
      headers: { Cookie: adminCookie }
    })
    expect(res.status).toBe(200)
    const body = await res.json() as { totalUsers: number }
    expect(body.totalUsers).toBeGreaterThanOrEqual(1)
  })

  it('admin can fetch /admin/api/users list', async () => {
    const res = await fetch(`${BASE_URL}/admin/api/users`, {
      headers: { Cookie: adminCookie }
    })
    expect(res.status).toBe(200)
    const users = await res.json() as { email: string }[]
    expect(Array.isArray(users)).toBe(true)
  })

  it('developer cannot access /admin/api/stats → 403', async () => {
    const adminC = await adminLogin()
    const devEmail = `e2e-rbac-${Date.now()}@test.orca.local`
    const dev = await adminCreateUser(adminC, devEmail, 'RbacTest@2025')

    try {
      const { cookie: devCookie } = await clientLogin(devEmail, 'RbacTest@2025')
      const res = await fetch(`${BASE_URL}/admin/api/stats`, {
        headers: { Cookie: devCookie }
      })
      expect(res.status).toBe(403)
    } finally {
      await adminDeleteUser(adminC, dev.id)
    }
  })
})
