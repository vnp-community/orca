/**
 * Database Health Monitor
 *
 * Implements HealthChecker by periodically running 'SELECT 1' against the pool.
 * Classifies results as healthy/degraded/unhealthy based on latency thresholds.
 * Notifies subscribers on status changes.
 *
 * @module db/health-monitor
 */

import type { IConnectionPool } from './pool'
import type { HealthChecker, DatabaseHealthCheck, HealthStatus } from './health'

/** Latency thresholds */
const DEGRADED_LATENCY_MS = 500
const UNHEALTHY_LATENCY_MS = 2_000
const DEFAULT_INTERVAL_MS = 30_000
const HEALTH_CHECK_QUERY = 'SELECT 1'

export class DatabaseHealthMonitor implements HealthChecker {
  private lastCheck: DatabaseHealthCheck | null = null
  private intervalHandle: ReturnType<typeof setInterval> | null = null
  private readonly statusChangeHandlers = new Set<(check: DatabaseHealthCheck) => void>()
  private previousStatus: HealthStatus | null = null

  constructor(
    private readonly pool: IConnectionPool,
    private readonly dialect: string
  ) {}

  async check(): Promise<DatabaseHealthCheck> {
    const startMs = Date.now()
    try {
      await this.pool.withConnection((db) => db.query(HEALTH_CHECK_QUERY))
      const latencyMs = Date.now() - startMs

      let status: HealthStatus = 'healthy'
      if (latencyMs >= UNHEALTHY_LATENCY_MS) {
        status = 'unhealthy'
      } else if (latencyMs >= DEGRADED_LATENCY_MS) {
        status = 'degraded'
      }

      // Pool pressure also triggers degraded
      const poolStats = this.pool.stats()
      if (status === 'healthy' && poolStats.waiting > 0) {
        status = 'degraded'
      }

      const result: DatabaseHealthCheck = {
        status,
        latencyMs,
        dialect: this.dialect,
        poolStats,
        checkedAt: new Date().toISOString()
      }

      this.emitIfChanged(result)
      this.lastCheck = result
      return result
    } catch (err) {
      const result: DatabaseHealthCheck = {
        status: 'unhealthy',
        latencyMs: Date.now() - startMs,
        dialect: this.dialect,
        poolStats: this.pool.stats(),
        lastError: err instanceof Error ? err.message : String(err),
        checkedAt: new Date().toISOString()
      }
      this.emitIfChanged(result)
      this.lastCheck = result
      return result
    }
  }

  getLastCheck(): DatabaseHealthCheck | null {
    return this.lastCheck
  }

  startPeriodicCheck(intervalMs: number = DEFAULT_INTERVAL_MS): void {
    if (this.intervalHandle) {return} // already running
    this.intervalHandle = setInterval(() => {
      void this.check()
    }, intervalMs)
    // Run immediately
    void this.check()
  }

  stopPeriodicCheck(): void {
    if (this.intervalHandle) {
      clearInterval(this.intervalHandle)
      this.intervalHandle = null
    }
  }

  onStatusChange(handler: (check: DatabaseHealthCheck) => void): () => void {
    this.statusChangeHandlers.add(handler)
    return () => this.statusChangeHandlers.delete(handler)
  }

  private emitIfChanged(check: DatabaseHealthCheck): void {
    if (check.status !== this.previousStatus) {
      this.previousStatus = check.status
      for (const handler of this.statusChangeHandlers) {
        try {
          handler(check)
        } catch {
          // ignore subscriber errors
        }
      }
    }
  }
}
