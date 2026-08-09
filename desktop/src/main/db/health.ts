/**
 * Database Health Check Interfaces
 *
 * @module db/health
 */

import type { PoolStats } from './pool'

/** Health status of the database connection */
export type HealthStatus = 'healthy' | 'degraded' | 'unhealthy'

/**
 * Result of a database health check.
 * 'healthy'   — queries complete within acceptable latency (<500ms)
 * 'degraded'  — slow (>=500ms) or pool has waiting requests
 * 'unhealthy' — connection fails or timeout (>=2000ms)
 */
export type DatabaseHealthCheck = {
  status: HealthStatus
  /** Query latency in milliseconds */
  latencyMs: number
  /** Database dialect (sqlite, mysql, postgresql, etc.) */
  dialect: string
  /** Pool stats at time of check (if available) */
  poolStats?: PoolStats
  /** Last error message (only when status = 'unhealthy') */
  lastError?: string
  /** ISO 8601 timestamp of this check */
  checkedAt: string
}

/** Interface for database health monitoring */
export type HealthChecker = {
  /** Run a live health check now */
  check(): Promise<DatabaseHealthCheck>
  /** Return cached result from last check (never queries DB) */
  getLastCheck(): DatabaseHealthCheck | null
  /** Start periodic background checks */
  startPeriodicCheck(intervalMs?: number): void
  /** Stop periodic background checks */
  stopPeriodicCheck(): void
  /**
   * Register a handler called when status changes.
   * Returns an unsubscribe function.
   */
  onStatusChange(handler: (check: DatabaseHealthCheck) => void): () => void
}
