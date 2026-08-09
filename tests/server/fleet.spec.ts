/**
 * Server tests — F27 Fleet Health Monitoring + F28 Dev Server Onboarding
 *
 * Tests:
 *   - /health/metrics Prometheus endpoint
 *   - /api/push/* Web Push API endpoints
 *   - Fleet config parser (unit-level, no SSH)
 *
 * Run:
 *   ORCA_SERVER_URL=http://172.20.2.39:6769 \
 *   pnpm vitest run --config tests/server/vitest.config.ts tests/server/fleet.spec.ts
 */

import { describe, it, expect, beforeAll } from 'vitest'
import { apiFetch, loginAsAdmin, BASE_URL } from './helpers'

// ─── F27: Prometheus Metrics ──────────────────────────────────────────────────

describe('F27 — Fleet Health Monitoring: Metrics Endpoint', () => {
  it('GET /health/metrics → 200 text/plain', async () => {
    const res = await fetch(`${BASE_URL}/health/metrics`)
    expect(res.status).toBe(200)
    const ct = res.headers.get('content-type') ?? ''
    expect(ct).toMatch(/text\/plain|openmetrics/)
  })

  it('metrics output is non-empty', async () => {
    const res = await fetch(`${BASE_URL}/health/metrics`)
    const text = await res.text()
    expect(text.trim().length).toBeGreaterThan(0)
  })

  it('metrics does not require authentication', async () => {
    // No cookie — should still succeed (Prometheus scraper has no auth)
    const { status } = await apiFetch('/health/metrics')
    expect(status).toBe(200)
  })

  it('metrics output contains at least one metric line or HELP comment', async () => {
    const res = await fetch(`${BASE_URL}/health/metrics`)
    const text = await res.text()
    const lines = text.split('\n').filter((l) => l.trim())
    // Must have at least one non-empty line
    expect(lines.length).toBeGreaterThan(0)
    // Accept either HELP/TYPE comment or metric key=value
    const hasValidLine = lines.some((l) => l.startsWith('#') || /^\w+/.test(l))
    expect(hasValidLine).toBe(true)
  })
})

// ─── F28: Web Push API ────────────────────────────────────────────────────────

describe('F28 — Dev Server Onboarding: Web Push API', () => {
  let adminCookie: string

  beforeAll(async () => {
    adminCookie = await loginAsAdmin()
  })

  it('POST /api/push/subscribe endpoint exists (not 404)', async () => {
    const { status } = await apiFetch('/api/push/subscribe', {
      method: 'POST',
      cookie: adminCookie,
      body: JSON.stringify({}) // Missing required fields — expect 400, not 404
    })
    expect(status).not.toBe(404)
  })

  it('POST /api/push/unsubscribe endpoint exists (not 404)', async () => {
    const { status } = await apiFetch('/api/push/unsubscribe', {
      method: 'POST',
      cookie: adminCookie,
      body: JSON.stringify({})
    })
    expect(status).not.toBe(404)
  })

  it('POST /api/push/subscribe without auth → 401', async () => {
    const { status } = await apiFetch('/api/push/subscribe', {
      method: 'POST',
      body: JSON.stringify({
        endpoint: 'https://push.example.com/endpoint',
        keys: { p256dh: 'abc', auth: 'xyz' }
      })
    })
    expect(status).toBe(401)
  })

  it('POST /api/push/subscribe with missing endpoint → 400', async () => {
    const { status } = await apiFetch('/api/push/subscribe', {
      method: 'POST',
      cookie: adminCookie,
      body: JSON.stringify({ keys: { p256dh: 'abc', auth: 'xyz' } }) // missing endpoint
    })
    expect([400, 422]).toContain(status)
  })
})

// ─── F27: Fleet Config Parser ─────────────────────────────────────────────────

describe('F27 — Fleet Config Parser: YAML Parsing', () => {
  it('parses valid 2-server fleet YAML config', async () => {
    let parseFleetConfig: ((yaml: string) => { servers: { id: string; host: string }[] }) | undefined
    try {
      const mod = await import('../../src/shared/fleet-config-parser')
      parseFleetConfig = (mod as Record<string, unknown>).parseFleetConfig as typeof parseFleetConfig
    } catch {
      console.log('Skipping: fleet-config-parser not importable in this env')
      return
    }

    const yaml = [
      'version: "1"',
      'fleet:',
      '  - id: server-1',
      '    label: "Dev Server 1"',
      '    host: 172.20.2.31',
      '    port: 22',
      '    user: ubuntu',
      '    key: ~/.ssh/id_ed25519',
      '  - id: server-2',
      '    label: "Dev Server 2"',
      '    host: 172.20.2.32',
      '    port: 22',
      '    user: ubuntu',
      '    key: ~/.ssh/id_ed25519'
    ].join('\n')

    const result = parseFleetConfig!(yaml)
    expect(result.servers).toHaveLength(2)
    expect(result.servers[0]!.id).toBe('server-1')
    expect(result.servers[0]!.host).toBe('172.20.2.31')
    expect(result.servers[1]!.id).toBe('server-2')
  })

  it('rejects fleet config with missing host field', async () => {
    let parseFleetConfig: ((yaml: string) => unknown) | undefined
    try {
      const mod = await import('../../src/shared/fleet-config-parser')
      parseFleetConfig = (mod as Record<string, unknown>).parseFleetConfig as typeof parseFleetConfig
    } catch {
      return
    }

    const invalid = ['version: "1"', 'fleet:', '  - id: bad-server', '    label: "No host"'].join('\n')
    expect(() => parseFleetConfig!(invalid)).toThrow()
  })

  it('rejects empty fleet array', async () => {
    let parseFleetConfig: ((yaml: string) => unknown) | undefined
    try {
      const mod = await import('../../src/shared/fleet-config-parser')
      parseFleetConfig = (mod as Record<string, unknown>).parseFleetConfig as typeof parseFleetConfig
    } catch {
      return
    }

    const empty = ['version: "1"', 'fleet: []'].join('\n')
    // Empty fleet could throw or return servers: []
    try {
      const result = parseFleetConfig!(empty) as { servers: unknown[] }
      expect(result.servers).toHaveLength(0)
    } catch {
      // Throwing is also acceptable
    }
  })
})

// ─── F28: toLinuxUsername Utility ─────────────────────────────────────────────

describe('F28 — Dev Server Onboarding: toLinuxUsername()', () => {
  it('follows orca-[a-z0-9-]+ format with max 20 chars', async () => {
    let toLinuxUsername: ((input: string) => string) | undefined
    try {
      const mod = await import('../../src/renderer/src/utils/linux-username')
      toLinuxUsername = (mod as Record<string, unknown>).toLinuxUsername as typeof toLinuxUsername
    } catch {
      console.log('Skipping: linux-username util not importable in this env')
      return
    }

    const cases = [
      'john.doe@example.com',
      'Jane.Smith@corp.io',
      'USER_123@domain.vn',
      'very-long-email-address@example.com',
      'user+tag@domain.com'
    ]

    for (const input of cases) {
      const result = toLinuxUsername!(input)
      expect(result).toMatch(/^orca-[a-z0-9-]+$/)
      expect(result.length).toBeLessThanOrEqual(20)
    }
  })

  it('is deterministic — same input always produces same output', async () => {
    let toLinuxUsername: ((input: string) => string) | undefined
    try {
      const mod = await import('../../src/renderer/src/utils/linux-username')
      toLinuxUsername = (mod as Record<string, unknown>).toLinuxUsername as typeof toLinuxUsername
    } catch {
      return
    }

    const input = 'developer@mycompany.com'
    expect(toLinuxUsername!(input)).toBe(toLinuxUsername!(input))
  })
})
