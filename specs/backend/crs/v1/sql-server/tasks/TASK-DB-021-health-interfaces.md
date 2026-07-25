# TASK-DB-021: Tạo `src/main/db/health.ts` — HealthChecker interface ✅ DONE

**Source:** SOL-DB-006 §4.1  
**Phase:** 3 | **Effort:** XS (< 15 min)   | **Status:** ✅ COMPLETED 2026-07-23  
**Depends on:** TASK-DB-008

---

## Objective

Tạo `src/main/db/health.ts` — định nghĩa `DatabaseHealthCheck`, `HealthStatus`, `HealthChecker` interface.

---

## Files to create

### `src/main/db/health.ts`

```typescript
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
 * 'healthy'  — queries complete within acceptable latency
 * 'degraded' — slow (>500ms) or pool has waiting requests
 * 'unhealthy' — connection fails or timeout (>2000ms)
 */
export interface DatabaseHealthCheck {
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
export interface HealthChecker {
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
```

---

## Verification

```bash
npx tsc --noEmit 2>&1 | grep "db/health" | head -5
```

Expected: Zero errors.

---

## Done criteria

- [x] `src/main/db/health.ts` tồn tại
- [x] Export `HealthStatus`, `DatabaseHealthCheck`, `HealthChecker`
- [x] `HealthStatus` = `'healthy' | 'degraded' | 'unhealthy'`
- [x] `DatabaseHealthCheck.poolStats` là optional
- [x] `HealthChecker.onStatusChange()` returns unsubscribe function
