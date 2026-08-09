/**
 * Shared helpers for client integration tests.
 *
 * Client tests focus on the web UI/SPA behaviour from the browser's perspective:
 *   - auth-api-client.ts calls
 *   - WebSocket RPC client connectivity
 *   - Connection status provider behaviour
 */

export const BASE_URL = process.env['ORCA_SERVER_URL'] ?? 'http://172.20.2.39:6769'
export const WS_BASE_URL = BASE_URL.replace(/^http/, 'ws').replace(':6769', ':6768')
export const ADMIN_EMAIL = process.env['ORCA_ADMIN_EMAIL'] ?? 'admin@b15.openledger.vn'
export const ADMIN_PASSWORD = process.env['ORCA_ADMIN_PASSWORD'] ?? 'Orca@Adm1n#2025'

// ─── Auth API Client helpers ──────────────────────────────────────────────────

/**
 * Simulate what auth-api-client.ts does: POST /auth/local and return cookie.
 */
export async function clientLogin(
  email: string,
  password: string
): Promise<{ cookie: string; user: { id: string; email: string; role: string } }> {
  const res = await fetch(`${BASE_URL}/auth/local`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email, password }),
    credentials: 'include'
  })
  if (!res.ok) {throw new Error(`Login failed: ${res.status}`)}
  const user = (await res.json()) as { id: string; email: string; role: string }
  const setCookie = res.headers.get('set-cookie') ?? ''
  const match = setCookie.match(/orca_session=[^;]+/)
  return { cookie: match ? match[0] : '', user }
}

/**
 * Simulate what auth-api-client.ts does: GET /auth/me.
 */
export async function clientGetMe(
  cookie: string
): Promise<{ id: string; email: string; role: string } | null> {
  const res = await fetch(`${BASE_URL}/auth/me`, {
    headers: { Cookie: cookie }
  })
  if (!res.ok) {return null}
  return (await res.json()) as { id: string; email: string; role: string }
}

/**
 * Simulate what auth-api-client.ts does: POST /auth/logout.
 */
export async function clientLogout(cookie: string): Promise<boolean> {
  const res = await fetch(`${BASE_URL}/auth/logout`, {
    method: 'POST',
    headers: { Cookie: cookie }
  })
  return res.ok
}

// ─── Admin helper (for test setup/teardown) ───────────────────────────────────

export async function adminLogin(): Promise<string> {
  const res = await fetch(`${BASE_URL}/auth/local`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email: ADMIN_EMAIL, password: ADMIN_PASSWORD })
  })
  const cookie = res.headers.get('set-cookie') ?? ''
  const match = cookie.match(/orca_session=[^;]+/)
  return match ? match[0] : ''
}

export async function adminCreateUser(
  adminCookie: string,
  email: string,
  password: string,
  role: 'admin' | 'developer' = 'developer'
): Promise<{ id: string; email: string }> {
  const res = await fetch(`${BASE_URL}/admin/api/users`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Cookie: adminCookie },
    body: JSON.stringify({ email, password, role })
  })
  return (await res.json()) as { id: string; email: string }
}

export async function adminDeleteUser(adminCookie: string, userId: string): Promise<void> {
  await fetch(`${BASE_URL}/admin/api/users/${userId}`, {
    method: 'DELETE',
    headers: { Cookie: adminCookie }
  })
}
