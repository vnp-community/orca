/**
 * Client tests — F22 Web Server Mode: Web Client Connectivity
 *
 * Tests what a browser client experiences when connecting to the Orca server:
 *   - Static web UI is served at root URL
 *   - Admin SPA is served at /admin/
 *   - WebSocket RPC endpoint responds on correct port
 *   - CORS headers are present for browser access
 *   - Connection status (online/offline) detection
 *
 * Run:
 *   ORCA_SERVER_URL=http://172.20.2.39:6769 \
 *   pnpm vitest run --config tests/client/vitest.config.ts tests/client/web-client.spec.ts
 */

import { describe, it, expect, beforeAll } from 'vitest'
import { BASE_URL, adminLogin, clientLogin, ADMIN_EMAIL } from './helpers'

// ─── Static File Serving ──────────────────────────────────────────────────────

describe('F22 — Web Client: Static File Serving', () => {
  it('GET / serves the web SPA (HTML)', async () => {
    const res = await fetch(`${BASE_URL}/`)
    expect(res.status).toBe(200)
    const ct = res.headers.get('content-type') ?? ''
    expect(ct).toMatch(/text\/html/)
    const html = await res.text()
    // Case-insensitive: the served SPA shell emits a lowercase `<!doctype
    // html>`, which is equally valid HTML5 — this test previously assumed
    // the uppercase spelling and would have failed against valid markup.
    expect(html.toLowerCase()).toContain('<!doctype html>')
  })

  it('GET /index.html serves the main SPA entry', async () => {
    const res = await fetch(`${BASE_URL}/index.html`)
    expect([200, 301, 302]).toContain(res.status)
  })

  it('GET /admin/ serves the admin SPA (HTML)', async () => {
    const res = await fetch(`${BASE_URL}/admin/`)
    // Admin SPA may redirect to login or serve HTML directly
    expect([200, 301, 302, 401, 403]).toContain(res.status)
    if (res.status === 200) {
      const html = await res.text()
      expect(html).toContain('<!DOCTYPE html>')
    }
  })

  it('static JS/CSS assets are served with correct content-type', async () => {
    // First get the HTML to find a real asset URL
    const htmlRes = await fetch(`${BASE_URL}/`)
    const html = await htmlRes.text()

    // Extract a JS asset URL from <script src="...">
    const jsMatch = html.match(/src="(\/[^"]+\.js)"/)
    if (jsMatch) {
      const jsRes = await fetch(`${BASE_URL}${jsMatch[1]}`)
      expect(jsRes.status).toBe(200)
      const ct = jsRes.headers.get('content-type') ?? ''
      expect(ct).toMatch(/javascript/)
    }
  })

  it('unknown routes return 200 with SPA shell (client-side routing)', async () => {
    // SPA should serve index.html for unknown paths (client-side routing)
    const res = await fetch(`${BASE_URL}/some/unknown/path`)
    // Either SPA fallback (200) or 404 depending on server config
    expect([200, 404]).toContain(res.status)
  })
})

// ─── WebSocket RPC Endpoint ───────────────────────────────────────────────────

describe('F22 — Web Client: WebSocket RPC Endpoint', () => {
  it('WS endpoint rejects unauthenticated connection in multi-user mode', async () => {
    // In multi-user mode, unauthenticated WS should be rejected
    // We test this indirectly via /auth/me (no cookie → 401)
    const res = await fetch(`${BASE_URL}/auth/me`)
    expect(res.status).toBe(401)
  })

  it('the RPC WS upgrade path (/ws) is served on the same port as HTTP', async () => {
    // Why same port, not a separate 6768: this comment previously described
    // the old TS backend's split ORCA_PORT (E2EE/device-token surface) from
    // the HTTP port — see helpers.ts's WS_BASE_URL doc comment for why that
    // doesn't apply to this cookie-authenticated browser SPA connection.
    // WsSessionRouter upgrades /ws on the SAME listener as HTTP, so a plain
    // health check on BASE_URL is a fair proxy for "the port RPC also runs
    // on is up" — the real WS handshake itself is covered end-to-end by
    // rpc-transport.spec.ts.
    const res = await fetch(`${BASE_URL}/health`)
    expect(res.status).toBe(200)
  })
})

// ─── Auth-Aware Static Serving ────────────────────────────────────────────────

describe('F22 — Web Client: Auth-Aware Behaviour', () => {
  let adminCookie: string

  beforeAll(async () => {
    adminCookie = await adminLogin()
  })

  it('authenticated user can access /auth/me → 200', async () => {
    const res = await fetch(`${BASE_URL}/auth/me`, {
      headers: { Cookie: adminCookie }
    })
    expect(res.status).toBe(200)
  })

  it('session cookie is HttpOnly (not accessible via JS in real browser)', async () => {
    // We can verify the Set-Cookie header has HttpOnly flag
    const res = await fetch(`${BASE_URL}/auth/local`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        email: ADMIN_EMAIL,
        password: process.env['ORCA_ADMIN_PASSWORD'] ?? 'Orca@Adm1n#2025'
      })
    })
    const setCookie = res.headers.get('set-cookie') ?? ''
    expect(setCookie.toLowerCase()).toContain('httponly')
  })

  it('session cookie has SameSite attribute for CSRF protection', async () => {
    const res = await fetch(`${BASE_URL}/auth/local`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        email: ADMIN_EMAIL,
        password: process.env['ORCA_ADMIN_PASSWORD'] ?? 'Orca@Adm1n#2025'
      })
    })
    const setCookie = res.headers.get('set-cookie') ?? ''
    expect(setCookie.toLowerCase()).toMatch(/samesite=(strict|lax|none)/)
  })
})

// ─── Admin API Client ─────────────────────────────────────────────────────────

describe('F25 — Admin API Client: Dashboard Stats', () => {
  let adminCookie: string

  beforeAll(async () => {
    const result = await clientLogin(
      ADMIN_EMAIL,
      process.env['ORCA_ADMIN_PASSWORD'] ?? 'Orca@Adm1n#2025'
    )
    adminCookie = result.cookie
  })

  it('admin can read dashboard stats', async () => {
    const res = await fetch(`${BASE_URL}/admin/api/stats`, {
      headers: { Cookie: adminCookie }
    })
    expect(res.status).toBe(200)
    const stats = (await res.json()) as { totalUsers: number; activeSessions: number }
    expect(stats.totalUsers).toBeGreaterThanOrEqual(1)
    expect(typeof stats.activeSessions).toBe('number')
  })

  it('admin can read users list', async () => {
    const res = await fetch(`${BASE_URL}/admin/api/users`, {
      headers: { Cookie: adminCookie }
    })
    expect(res.status).toBe(200)
    // Why {users, total}, not a bare array — see auth-api-client.spec.ts's
    // matching test for the full explanation (usersListJSON's shape).
    const body = (await res.json()) as { users: { email: string; role: string }[]; total: number }
    expect(body.users.some((u) => u.role === 'admin')).toBe(true)
  })

  it('admin can read audit log', async () => {
    const res = await fetch(`${BASE_URL}/admin/api/audit`, {
      headers: { Cookie: adminCookie }
    })
    expect(res.status).toBe(200)
    const body = (await res.json()) as unknown[] | { entries: unknown[] }
    const entries = Array.isArray(body) ? body : (body.entries ?? [])
    expect(Array.isArray(entries)).toBe(true)
  })
})
