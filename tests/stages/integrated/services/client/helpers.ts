/**
 * Shared helpers for client integration tests.
 *
 * Client tests focus on the web UI/SPA behaviour from the browser's perspective:
 *   - auth-api-client.ts calls
 *   - WebSocket RPC client connectivity (see rpc-client.ts)
 *   - Connection status provider behaviour
 */

import { RpcSession } from './rpc-client'

export const BASE_URL = process.env['ORCA_SERVER_URL'] ?? 'http://172.20.2.39:6769'
// Why: the browser SPA's RPC connection is proxied on the SAME port as HTTP —
// any WS upgrade path other than /agent (backend/src/server/index.ts's
// httpServer 'upgrade' handler → WsSessionRouter,
// backend/src/main/session/ws-session-router.ts) resolves the user from the
// request's Cookie header and proxies to that user's OrcaRuntimeRpcServer.
// A separate port (ORCA_PORT, default 6768) exists for the E2EE/device-token
// surface mobile pairing and remote-environment connections use
// (backend/src/main/runtime/rpc/ws-transport.ts) — cookie auth doesn't apply
// there, so it's the wrong port for this cookie-authenticated test client.
export const WS_BASE_URL = BASE_URL.replace(/^http/, 'ws')
export const RPC_WS_URL = `${WS_BASE_URL}/ws`
export const ADMIN_EMAIL = process.env['ORCA_ADMIN_EMAIL'] ?? 'admin@b15.openledger.vn'
export const ADMIN_PASSWORD = process.env['ORCA_ADMIN_PASSWORD'] ?? 'Orca@Adm1n#2025'

// ─── RPC Client helpers ────────────────────────────────────────────────────────

/**
 * Open a cookie-authenticated RPC session against the backend's RPC method
 * registry — the same registry specs/frontend/api/rpc-catalog.md and
 * specs/frontend/api/mobile-rpc-catalog.md document. Callers own the returned
 * session's lifecycle and must call `.close()` (e.g. in `afterAll`).
 */
export async function connectRpc(cookie: string): Promise<RpcSession> {
  return RpcSession.connect(RPC_WS_URL, cookie)
}

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
  if (!res.ok) {
    throw new Error(`Login failed: ${res.status}`)
  }
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
  if (!res.ok) {
    return null
  }
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

// Why 'admin' | 'user', not 'developer': backend-go's Role enum is 2-valued
// (ROLE_ADMIN/ROLE_USER, auth_admin_routes.go's parseRole/roleToString) — a
// deliberate simplification from the old TS backend's 3-role model
// (admin/lead/developer). Passing 'developer' here silently parses to
// ROLE_UNSPECIFIED server-side instead of erroring, which is why this
// default was wrong for a long time without a test ever catching it.
export async function adminCreateUser(
  adminCookie: string,
  email: string,
  password: string,
  role: 'admin' | 'user' = 'user'
): Promise<{ id: string; email: string; role: string }> {
  const res = await fetch(`${BASE_URL}/admin/api/users`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Cookie: adminCookie },
    body: JSON.stringify({ email, password, role })
  })
  return (await res.json()) as { id: string; email: string; role: string }
}

// Why "delete": DELETE /admin/api/users/:id is a soft-delete
// (handleDeactivateUser sets is_active=false, never a hard row delete —
// see admin_routes.go's doc comment) — kept as test cleanup because it's
// sufficient to stop the throwaway account from logging in again; it does
// not remove the row.
export async function adminDeleteUser(adminCookie: string, userId: string): Promise<void> {
  await fetch(`${BASE_URL}/admin/api/users/${userId}`, {
    method: 'DELETE',
    headers: { Cookie: adminCookie }
  })
}
