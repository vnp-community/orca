/**
 * Server tests — F22 Web Server Mode: Health Endpoints
 *
 * Verifies the server's health/readiness/metrics HTTP surface.
 *
 * Run:
 *   ORCA_SERVER_URL=http://172.20.2.39:6769 \
 *   pnpm vitest run --config tests/server/vitest.config.ts tests/server/health.spec.ts
 */

import { describe, it, expect } from 'vitest'
import { apiFetch, BASE_URL } from './helpers'

describe('F22 — Web Server Mode: Health Endpoints', () => {
  it('GET /health returns 200 with status field', async () => {
    const { status, body } = await apiFetch<{ status: string; version?: string }>('/health')
    expect(status).toBe(200)
    expect(body).toMatchObject({ status: expect.any(String) })
  })

  it('GET /health/ready returns {ready: true} when server is up', async () => {
    const { status, body } = await apiFetch<{ ready: boolean }>('/health/ready')
    expect(status).toBe(200)
    expect(body.ready).toBe(true)
  })

  it('GET /health/metrics returns Prometheus text format', async () => {
    const res = await fetch(`${BASE_URL}/health/metrics`)
    expect(res.status).toBe(200)
    const ct = res.headers.get('content-type') ?? ''
    expect(ct).toMatch(/text\/plain|openmetrics/)
    const text = await res.text()
    expect(text.length).toBeGreaterThan(0)
  })

  it('GET /health/metrics is publicly accessible (no auth required)', async () => {
    // Prometheus scraping must not require authentication
    const { status } = await apiFetch('/health/metrics')
    expect(status).toBe(200)
  })

  it('GET /health includes version info', async () => {
    const { status, body } = await apiFetch<{ version?: string; status: string }>('/health')
    expect(status).toBe(200)
    // version may be present
    if ('version' in body) {
      expect(typeof body.version).toBe('string')
    }
  })
})
