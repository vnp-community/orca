import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import http from 'node:http'
import { createHealthEndpoint } from '../health-endpoint'
import type { HealthChecker, DatabaseHealthCheck } from '../../main/db/health'

function makeMockMonitor(lastCheck: DatabaseHealthCheck | null = null): HealthChecker {
  const defaultCheck: DatabaseHealthCheck = {
    status: 'healthy',
    latencyMs: 5,
    dialect: 'sqlite',
    checkedAt: new Date().toISOString(),
    poolStats: { total: 1, idle: 1, acquired: 0, waiting: 0 }
  }
  return {
    check: vi.fn().mockResolvedValue(lastCheck ?? defaultCheck),
    getLastCheck: vi.fn().mockReturnValue(lastCheck ?? defaultCheck),
    startPeriodicCheck: vi.fn(),
    stopPeriodicCheck: vi.fn(),
    onStatusChange: vi.fn().mockReturnValue(() => {})
  }
}

function makeRequest(
  server: http.Server,
  path: string
): Promise<{ status: number; body: string; headers: http.IncomingHttpHeaders }> {
  return new Promise((resolve, reject) => {
    const port = (server.address() as { port: number }).port
    const req = http.get(`http://localhost:${port}${path}`, (res) => {
      let body = ''
      res.on('data', (c) => { body += c })
      res.on('end', () => resolve({ status: res.statusCode!, body, headers: res.headers }))
    })
    req.on('error', reject)
  })
}

async function startServer(monitor: HealthChecker, options?: Record<string, unknown>): Promise<http.Server> {
  const handler = createHealthEndpoint(monitor, options as any)
  const server = http.createServer(handler)
  await new Promise<void>((resolve) => server.listen(0, resolve))
  return server
}

async function closeServer(server: http.Server): Promise<void> {
  await new Promise<void>((resolve) => server.close(() => resolve()))
}

describe('createHealthEndpoint', () => {
  let server: http.Server
  let monitor: HealthChecker

  beforeEach(async () => {
    monitor = makeMockMonitor()
    server = await startServer(monitor, { includePoolStats: true })
  })

  afterEach(async () => {
    await closeServer(server)
  })

  describe('GET /health', () => {
    it('returns 200 when healthy', async () => {
      const { status } = await makeRequest(server, '/health')
      expect(status).toBe(200)
    })

    it('response body has status field', async () => {
      const { body } = await makeRequest(server, '/health')
      expect(JSON.parse(body).status).toBe('healthy')
    })

    it('includes uptime in response', async () => {
      const { body } = await makeRequest(server, '/health')
      expect(typeof JSON.parse(body).uptime).toBe('number')
    })

    it('uses getLastCheck() (cached — no live query)', async () => {
      await makeRequest(server, '/health')
      expect((monitor.check as ReturnType<typeof vi.fn>).mock.calls.length).toBe(0)
      expect((monitor.getLastCheck as ReturnType<typeof vi.fn>).mock.calls.length).toBeGreaterThan(0)
    })

    it('returns 503 when last check was unhealthy', async () => {
      const unhealthyMonitor = makeMockMonitor({
        status: 'unhealthy', latencyMs: 5000, dialect: 'sqlite',
        lastError: 'Connection refused', checkedAt: new Date().toISOString()
      })
      const s = await startServer(unhealthyMonitor)
      const { status } = await makeRequest(s, '/health')
      expect(status).toBe(503)
      await closeServer(s)
    })
  })

  describe('GET /health/ready', () => {
    it('returns 200 for healthy check', async () => {
      const { status } = await makeRequest(server, '/health/ready')
      expect(status).toBe(200)
    })

    it('calls monitor.check() (live check)', async () => {
      await makeRequest(server, '/health/ready')
      expect((monitor.check as ReturnType<typeof vi.fn>).mock.calls.length).toBeGreaterThan(0)
    })

    it('includes database.dialect and latencyMs', async () => {
      const { body } = await makeRequest(server, '/health/ready')
      const json = JSON.parse(body)
      expect(json.database.dialect).toBe('sqlite')
      expect(typeof json.database.latencyMs).toBe('number')
    })

    it('includes pool when includePoolStats=true', async () => {
      const { body } = await makeRequest(server, '/health/ready')
      expect(JSON.parse(body).database.pool).toBeDefined()
    })

    it('returns 503 for unhealthy database', async () => {
      const failMonitor = makeMockMonitor()
      ;(failMonitor.check as ReturnType<typeof vi.fn>).mockResolvedValue({
        status: 'unhealthy', latencyMs: 5000, dialect: 'sqlite',
        lastError: 'DB down', checkedAt: new Date().toISOString()
      })
      const s = await startServer(failMonitor)
      const { status } = await makeRequest(s, '/health/ready')
      expect(status).toBe(503)
      await closeServer(s)
    })

    it('returns 200 for degraded (still serving)', async () => {
      const degradedMonitor = makeMockMonitor()
      ;(degradedMonitor.check as ReturnType<typeof vi.fn>).mockResolvedValue({
        status: 'degraded', latencyMs: 800, dialect: 'sqlite',
        checkedAt: new Date().toISOString()
      })
      const s = await startServer(degradedMonitor)
      const { status } = await makeRequest(s, '/health/ready')
      expect(status).toBe(200)
      await closeServer(s)
    })

    it('returns 503 on timeout', async () => {
      const slowMonitor = makeMockMonitor()
      ;(slowMonitor.check as ReturnType<typeof vi.fn>).mockImplementation(
        () => new Promise((resolve) => setTimeout(resolve, 5000))
      )
      const s = await startServer(slowMonitor, { timeoutMs: 100 })
      const { status, body } = await makeRequest(s, '/health/ready')
      expect(status).toBe(503)
      expect(JSON.parse(body).error).toContain('timeout')
      await closeServer(s)
    })
  })

  describe('GET /health/metrics', () => {
    it('returns 200 with Prometheus format', async () => {
      const { status, body, headers } = await makeRequest(server, '/health/metrics')
      expect(status).toBe(200)
      expect(headers['content-type']).toContain('text/plain')
      expect(body).toContain('orca_db_latency_ms')
      expect(body).toContain('orca_db_pool_total')
    })
  })

  describe('unknown paths', () => {
    it('returns 404 for /health/unknown', async () => {
      const { status } = await makeRequest(server, '/health/unknown')
      expect(status).toBe(404)
    })
  })
})
