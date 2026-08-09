/**
 * Health Endpoint Handler
 *
 * Creates an HTTP request handler (compatible with node:http) exposing:
 *   GET /health         — cached status (no live DB query)
 *   GET /health/ready   — live DB check with timeout
 *   GET /health/metrics — Prometheus text format
 *
 * Integrate by passing to http.createServer() or mounting on an existing router.
 *
 * @module server/health-endpoint
 */

import type { IncomingMessage, ServerResponse } from 'node:http'
import type { HealthChecker, DatabaseHealthCheck } from '../main/db/health'

export type HealthEndpointOptions = {
  /** Whether to include pool stats in responses */
  includePoolStats?: boolean
  /** Timeout in ms for /health/ready live check (default: 5000) */
  timeoutMs?: number
}

type RequestHandler = (req: IncomingMessage, res: ServerResponse) => void

function respond(res: ServerResponse, statusCode: number, body: unknown, contentType = 'application/json'): void {
  const bodyStr = typeof body === 'string' ? body : JSON.stringify(body)
  res.writeHead(statusCode, {
    'Content-Type': contentType,
    'Content-Length': Buffer.byteLength(bodyStr),
    'Cache-Control': 'no-cache, no-store'
  })
  res.end(bodyStr)
}

function buildResponse(check: DatabaseHealthCheck, options: HealthEndpointOptions) {
  const db: Record<string, unknown> = {
    status: check.status,
    dialect: check.dialect,
    latencyMs: check.latencyMs,
    checkedAt: check.checkedAt
  }
  if (check.lastError) {db['lastError'] = check.lastError}
  if (options.includePoolStats && check.poolStats) {db['pool'] = check.poolStats}
  return db
}

function toPrometheus(check: DatabaseHealthCheck | null): string {
  if (!check) {
    return [
      '# HELP orca_db_latency_ms Database query latency in milliseconds',
      '# TYPE orca_db_latency_ms gauge',
      'orca_db_latency_ms 0',
      '# HELP orca_db_pool_total Total connections managed by pool',
      '# TYPE orca_db_pool_total gauge',
      'orca_db_pool_total 0',
      '# HELP orca_db_pool_idle Idle connections in pool',
      '# TYPE orca_db_pool_idle gauge',
      'orca_db_pool_idle 0',
      '# HELP orca_db_pool_acquired Acquired connections in pool',
      '# TYPE orca_db_pool_acquired gauge',
      'orca_db_pool_acquired 0',
      ''
    ].join('\n')
  }

  const pool = check.poolStats
  return [
    `# HELP orca_db_latency_ms Database query latency in milliseconds`,
    `# TYPE orca_db_latency_ms gauge`,
    `orca_db_latency_ms ${check.latencyMs}`,
    `# HELP orca_db_pool_total Total connections managed by pool`,
    `# TYPE orca_db_pool_total gauge`,
    `orca_db_pool_total ${pool?.total ?? 0}`,
    `# HELP orca_db_pool_idle Idle connections in pool`,
    `# TYPE orca_db_pool_idle gauge`,
    `orca_db_pool_idle ${pool?.idle ?? 0}`,
    `# HELP orca_db_pool_acquired Acquired connections in pool`,
    `# TYPE orca_db_pool_acquired gauge`,
    `orca_db_pool_acquired ${pool?.acquired ?? 0}`,
    ''
  ].join('\n')
}

/**
 * Create a health endpoint handler.
 *
 * @param monitor - HealthChecker implementation
 * @param options - Endpoint configuration
 * @returns HTTP request handler
 */
export function createHealthEndpoint(
  monitor: HealthChecker,
  options: HealthEndpointOptions = {}
): RequestHandler {
  const timeoutMs = options.timeoutMs ?? 5_000
  const startedAt = Date.now()

  return async (req: IncomingMessage, res: ServerResponse): Promise<void> => {
    const url = req.url ?? '/'

    // GET /health — cached, no live query
    if (url === '/health' || url === '/health/') {
      const lastCheck = monitor.getLastCheck()
      const status = lastCheck?.status ?? 'healthy'
      const httpStatus = status === 'unhealthy' ? 503 : 200
      respond(res, httpStatus, {
        status,
        uptime: Math.floor((Date.now() - startedAt) / 1000),
        database: lastCheck ? buildResponse(lastCheck, options) : null
      })
      return
    }

    // GET /health/ready — live check with timeout
    if (url === '/health/ready' || url === '/health/ready/') {
      try {
        let timedOut = false
        const checkPromise = monitor.check()
        const timeoutPromise = new Promise<never>((_, reject) =>
          setTimeout(() => {
            timedOut = true
            reject(new Error('Health check timeout'))
          }, timeoutMs)
        )

        const check = await Promise.race([checkPromise, timeoutPromise])
        const httpStatus = check.status === 'unhealthy' ? 503 : 200
        respond(res, httpStatus, {
          status: check.status,
          uptime: Math.floor((Date.now() - startedAt) / 1000),
          database: buildResponse(check, options)
        })
      } catch (err) {
        const message = err instanceof Error ? err.message : 'Health check failed'
        respond(res, 503, {
          status: 'unhealthy',
          error: message,
          uptime: Math.floor((Date.now() - startedAt) / 1000)
        })
      }
      return
    }

    // GET /health/metrics — Prometheus format
    if (url === '/health/metrics' || url === '/health/metrics/') {
      const lastCheck = monitor.getLastCheck()
      respond(res, 200, toPrometheus(lastCheck), 'text/plain; version=0.0.4; charset=utf-8')
      return
    }

    // Unknown path
    respond(res, 404, { error: 'Not found', path: url })
  }
}
