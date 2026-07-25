/**
 * Shared test helpers for server integration tests.
 *
 * Provides fetch wrappers, auth helpers, and type definitions
 * used across all tests/server/*.spec.ts files.
 */

export const BASE_URL = process.env['ORCA_SERVER_URL'] ?? 'http://172.20.2.39:6769'
export const ADMIN_EMAIL = process.env['ORCA_ADMIN_EMAIL'] ?? 'admin@b15.openledger.vn'
export const ADMIN_PASSWORD = process.env['ORCA_ADMIN_PASSWORD'] ?? 'Orca@Adm1n#2025'

// ─── Types ────────────────────────────────────────────────────────────────────

export interface ApiResponse<T = unknown> {
  status: number
  body: T
  headers: Headers
}

export interface UserRecord {
  id: string
  email: string
  role: 'admin' | 'developer'
  isActive: boolean
}

export interface SessionRecord {
  sessionId: string
  userId: string
  expiresAt: number
  lastSeenAt: number
}

export interface AuditEntry {
  id: string
  action: string
  userId?: string
  metadata?: Record<string, unknown>
  createdAt: number
}

export interface AdminStats {
  totalUsers: number
  activeSessions: number
  pairedDevices?: number
}

// ─── Fetch Helpers ────────────────────────────────────────────────────────────

export async function apiFetch<T = unknown>(
  path: string,
  options: RequestInit & { cookie?: string } = {}
): Promise<ApiResponse<T>> {
  const { cookie, headers: extraHeaders, ...restOpts } = options
  const res = await fetch(`${BASE_URL}${path}`, {
    headers: {
      'Content-Type': 'application/json',
      ...(cookie ? { Cookie: cookie } : {}),
      ...extraHeaders
    },
    ...restOpts
  })

  let body: T
  const ct = res.headers.get('content-type') ?? ''
  if (ct.includes('application/json')) {
    body = (await res.json()) as T
  } else {
    body = (await res.text()) as unknown as T
  }

  return { status: res.status, body, headers: res.headers }
}

// ─── Auth Helpers ─────────────────────────────────────────────────────────────

/**
 * Login and return the session cookie string (e.g. "orca_session=abc123").
 * Throws if login fails.
 */
export async function loginAs(email: string, password: string): Promise<string> {
  const res = await fetch(`${BASE_URL}/auth/local`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email, password })
  })
  if (!res.ok) {
    throw new Error(`Login failed for ${email}: HTTP ${res.status}`)
  }
  const setCookie = res.headers.get('set-cookie') ?? ''
  const match = setCookie.match(/orca_session=[^;]+/)
  if (!match) throw new Error(`No session cookie in login response for ${email}`)
  return match[0]
}

/**
 * Login as admin and return cookie. Throws if ADMIN credentials are wrong.
 */
export async function loginAsAdmin(): Promise<string> {
  return loginAs(ADMIN_EMAIL, ADMIN_PASSWORD)
}

// ─── Admin API Helpers ────────────────────────────────────────────────────────

export async function adminCreateUser(
  adminCookie: string,
  email: string,
  password: string,
  role: 'admin' | 'developer' = 'developer'
): Promise<UserRecord> {
  const { status, body } = await apiFetch<UserRecord>('/admin/api/users', {
    method: 'POST',
    cookie: adminCookie,
    body: JSON.stringify({ email, password, role })
  })
  if (status !== 201) {
    throw new Error(`Failed to create user ${email}: HTTP ${status} — ${JSON.stringify(body)}`)
  }
  return body
}

export async function adminDeleteUser(adminCookie: string, userId: string): Promise<void> {
  await apiFetch(`/admin/api/users/${userId}`, {
    method: 'DELETE',
    cookie: adminCookie
  })
}

export async function adminKillUserSessions(adminCookie: string, userId: string): Promise<void> {
  await apiFetch(`/admin/api/users/${userId}/sessions`, {
    method: 'DELETE',
    cookie: adminCookie
  })
}

/**
 * Create a temp test user, run the callback, and always cleanup afterwards.
 */
export async function withTestUser(
  adminCookie: string,
  suffix: string,
  fn: (user: UserRecord, cookie: string) => Promise<void>
): Promise<void> {
  const email = `e2e-${suffix}-${Date.now()}@test.orca.local`
  const password = 'TestPass@2025!'
  const user = await adminCreateUser(adminCookie, email, password)
  const cookie = await loginAs(email, password)
  try {
    await fn(user, cookie)
  } finally {
    await adminDeleteUser(adminCookie, user.id)
  }
}
