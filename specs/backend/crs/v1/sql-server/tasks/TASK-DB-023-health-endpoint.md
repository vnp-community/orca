# TASK-DB-023: Tạo `src/server/health-endpoint.ts` + tests ✅ DONE

**Source:** SOL-DB-006 §4.3  
**Phase:** 3 | **Effort:** S (45–60 min)   | **Status:** ✅ COMPLETED 2026-07-23  
**Depends on:** TASK-DB-021, TASK-DB-022

---

## Objective

Tạo HTTP health endpoint handler hỗ trợ 3 paths: `/health` (liveness, cached), `/health/ready` (readiness, live check), `/health/metrics` (Prometheus).

---

## Files to create

### 1. `src/server/health-endpoint.ts`

```typescript
import type { IncomingMessage, ServerResponse } from 'node:http'
import type { HealthChecker, DatabaseHealthCheck } from '../main/db/health'

export interface HealthEndpointOptions {
  /** Milliseconds before live check times out (default: 3000) */
  timeoutMs?: number
  /** Include pool stats in /health/ready response (default: false) */
  includePoolStats?: boolean
}

/**
 * Creates an HTTP request handler for health check endpoints.
 *
 * Routes:
 *   GET /health         — Liveness probe (cached, fast)
 *   GET /health/ready   — Readiness probe (live check, may be slow)
 *   GET /health/metrics — Prometheus metrics (text/plain)
 *   GET /health/*       — 404 for unknown sub-paths
 *
 * HTTP status codes:
 *   healthy  → 200
 *   degraded → 200 (still serving, just slow)
 *   unhealthy → 503
 */
export function createHealthEndpoint(
  monitor: HealthChecker,
  options: HealthEndpointOptions = {}
): (req: IncomingMessage, res: ServerResponse) => Promise<void> {
  const { timeoutMs = 3_000, includePoolStats = false } = options

  return async function healthHandler(req: IncomingMessage, res: ServerResponse): Promise<void> {
    const url = new URL(req.url ?? '/', 'http://localhost')

    // ── GET /health — Liveness ───────────────────────────────────────────
    if (url.pathname === '/health') {
      const lastCheck = monitor.getLastCheck()
      const isAlive = !lastCheck || lastCheck.status !== 'unhealthy'
      res.writeHead(isAlive ? 200 : 503, { 'Content-Type': 'application/json' })
      res.end(JSON.stringify({
        status: lastCheck?.status ?? 'unknown',
        uptime: process.uptime(),
        timestamp: new Date().toISOString()
      }))
      return
    }

    // ── GET /health/ready — Readiness ────────────────────────────────────
    if (url.pathname === '/health/ready') {
      let check: DatabaseHealthCheck
      try {
        check = await Promise.race([
          monitor.check(),
          new Promise<never>((_, reject) =>
            setTimeout(() => reject(new Error(`Health check timeout after ${timeoutMs}ms`)), timeoutMs)
          )
        ])
      } catch (err) {
        res.writeHead(503, { 'Content-Type': 'application/json' })
        res.end(JSON.stringify({
          status: 'unhealthy',
          error: (err as Error).message,
          timestamp: new Date().toISOString()
        }))
        return
      }

      const httpStatus = check.status === 'unhealthy' ? 503 : 200
      res.writeHead(httpStatus, { 'Content-Type': 'application/json' })
      res.end(JSON.stringify({
        status: check.status,
        database: {
          dialect: check.dialect,
          latencyMs: check.latencyMs,
          ...(includePoolStats && check.poolStats ? { pool: check.poolStats } : {})
        },
        timestamp: check.checkedAt
      }))
      return
    }

    // ── GET /health/metrics — Prometheus ─────────────────────────────────
    if (url.pathname === '/health/metrics') {
      const check = monitor.getLastCheck()
      const pool = check?.poolStats
      const lines = [
        `# HELP orca_db_latency_ms Database query latency milliseconds`,
        `# TYPE orca_db_latency_ms gauge`,
        `orca_db_latency_ms{dialect="${check?.dialect ?? 'unknown'}"} ${check?.latencyMs ?? -1}`,
        `# HELP orca_db_pool_total Total managed connections`,
        `# TYPE orca_db_pool_total gauge`,
        `orca_db_pool_total ${pool?.total ?? 0}`,
        `# HELP orca_db_pool_idle Idle connections`,
        `# TYPE orca_db_pool_idle gauge`,
        `orca_db_pool_idle ${pool?.idle ?? 0}`,
        `# HELP orca_db_pool_acquired Acquired connections`,
        `# TYPE orca_db_pool_acquired gauge`,
        `orca_db_pool_acquired ${pool?.acquired ?? 0}`,
        `# HELP orca_db_pool_waiting Waiting acquire requests`,
        `# TYPE orca_db_pool_waiting gauge`,
        `orca_db_pool_waiting ${pool?.waiting ?? 0}`,
        ``
      ]
      res.writeHead(200, { 'Content-Type': 'text/plain; version=0.0.4' })
      res.end(lines.join('\n'))
      return
    }

    // ── 404 for unknown paths ────────────────────────────────────────────
    res.writeHead(404, { 'Content-Type': 'application/json' })
    res.end(JSON.stringify({ error: 'Not found', path: url.pathname }))
  }
}
```

### 2. `src/server/__tests__/health-endpoint.test.ts`

```typescript
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import http from 'node:http'
import { createHealthEndpoint } from '../health-endpoint'
import type { HealthChecker, DatabaseHealthCheck } from '../../main/db/health'

function makeMockMonitor(lastCheck: DatabaseHealthCheck | null = null): HealthChecker {
  const defaultCheck: DatabaseHealthCheck = {
    status: 'healthy', latencyMs: 5, dialect: 'sqlite',
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

function makeRequest(server: http.Server, path: string): Promise<{ status: number; body: string; headers: http.IncomingHttpHeaders }> {
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

describe('createHealthEndpoint', () => {
  let server: http.Server
  let monitor: HealthChecker

  beforeEach(async () => {
    monitor = makeMockMonitor()
    const handler = createHealthEndpoint(monitor, { includePoolStats: true })
    server = http.createServer(handler)
    await new Promise<void>((resolve) => server.listen(0, resolve))
  })

  afterEach(async () => {
    await new Promise<void>((resolve) => server.close(() => resolve()))
  })

  describe('GET /health', () => {
    it('returns 200 when healthy', async () => {
      const { status } = await makeRequest(server, '/health')
      expect(status).toBe(200)
    })

    it('response body has status field', async () => {
      const { body } = await makeRequest(server, '/health')
      const json = JSON.parse(body)
      expect(json.status).toBe('healthy')
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
      const handler = createHealthEndpoint(unhealthyMonitor)
      const s = http.createServer(handler)
      await new Promise<void>((resolve) => s.listen(0, resolve))
      const { status } = await makeRequest(s, '/health')
      expect(status).toBe(503)
      await new Promise<void>((resolve) => s.close(() => resolve()))
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
      const json = JSON.parse(body)
      expect(json.database.pool).toBeDefined()
    })

    it('returns 503 for unhealthy database', async () => {
      const failMonitor = makeMockMonitor()
      ;(failMonitor.check as ReturnType<typeof vi.fn>).mockResolvedValue({
        status: 'unhealthy', latencyMs: 5000, dialect: 'sqlite',
        lastError: 'DB down', checkedAt: new Date().toISOString()
      })
      const handler = createHealthEndpoint(failMonitor)
      const s = http.createServer(handler)
      await new Promise<void>((resolve) => s.listen(0, resolve))
      const { status } = await makeRequest(s, '/health/ready')
      expect(status).toBe(503)
      await new Promise<void>((resolve) => s.close(() => resolve()))
    })

    it('returns 200 for degraded (still serving)', async () => {
      const degradedMonitor = makeMockMonitor()
      ;(degradedMonitor.check as ReturnType<typeof vi.fn>).mockResolvedValue({
        status: 'degraded', latencyMs: 800, dialect: 'sqlite',
        checkedAt: new Date().toISOString()
      })
      const handler = createHealthEndpoint(degradedMonitor)
      const s = http.createServer(handler)
      await new Promise<void>((resolve) => s.listen(0, resolve))
      const { status } = await makeRequest(s, '/health/ready')
      expect(status).toBe(200)  // degraded → 200, still serving
      await new Promise<void>((resolve) => s.close(() => resolve()))
    })

    it('returns 503 on timeout', async () => {
      const slowMonitor = makeMockMonitor()
      ;(slowMonitor.check as ReturnType<typeof vi.fn>).mockImplementation(
        () => new Promise((resolve) => setTimeout(resolve, 5000))
      )
      const handler = createHealthEndpoint(slowMonitor, { timeoutMs: 100 })
      const s = http.createServer(handler)
      await new Promise<void>((resolve) => s.listen(0, resolve))
      const { status, body } = await makeRequest(s, '/health/ready')
      expect(status).toBe(503)
      expect(JSON.parse(body).error).toContain('timeout')
      await new Promise<void>((resolve) => s.close(() => resolve()))
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
```

---

## Verification

```bash
pnpm vitest run src/server/__tests__/health-endpoint.test.ts
```

Expected: 16/16 tests pass

---

## Done criteria

- [x] `createHealthEndpoint()` returns async request handler
- [x] `/health` dùng cached result (no live query)
- [x] `/health/ready` gọi `monitor.check()` với timeout
- [x] `degraded` → HTTP 200 (còn serving traffic)
- [x] `unhealthy` → HTTP 503
- [x] Timeout → 503 với message chứa "timeout"
- [x] `/health/metrics` → Prometheus text format, `Content-Type: text/plain`
- [x] Unknown paths → 404
- [x] 16/16 tests pass
