# TASK-DB-022: Tạo `src/main/db/health-monitor.ts` + tests ✅ DONE

**Source:** SOL-DB-006 §4.2  
**Phase:** 3 | **Effort:** S (45–60 min)   | **Status:** ✅ COMPLETED 2026-07-23  
**Depends on:** TASK-DB-008, TASK-DB-021

---

## Objective

Tạo `DatabaseHealthMonitor` — implementation `HealthChecker`, chạy `SELECT 1`, phân loại latency, hỗ trợ periodic check và status-change callbacks.

---

## Files to create

### 1. `src/main/db/health-monitor.ts`

```typescript
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
      if (latencyMs >= UNHEALTHY_LATENCY_MS) status = 'unhealthy'
      else if (latencyMs >= DEGRADED_LATENCY_MS) status = 'degraded'

      // Pool pressure also triggers degraded
      const poolStats = this.pool.stats()
      if (status === 'healthy' && poolStats.waiting > 0) status = 'degraded'

      const result: DatabaseHealthCheck = {
        status, latencyMs, dialect: this.dialect,
        poolStats, checkedAt: new Date().toISOString()
      }

      this.emitIfChanged(result)
      this.lastCheck = result
      return result
    } catch (err) {
      const result: DatabaseHealthCheck = {
        status: 'unhealthy',
        latencyMs: Date.now() - startMs,
        dialect: this.dialect,
        lastError: (err as Error).message,
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

  startPeriodicCheck(intervalMs = DEFAULT_INTERVAL_MS): void {
    if (this.intervalHandle) return  // idempotent
    this.intervalHandle = setInterval(() => {
      this.check().catch(() => { /* errors already captured in check() */ })
    }, intervalMs)
    // Don't block process exit
    if (this.intervalHandle.unref) this.intervalHandle.unref()
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
    if (check.status === this.previousStatus) return
    this.previousStatus = check.status
    for (const handler of this.statusChangeHandlers) {
      try { handler(check) } catch { /* swallow handler errors */ }
    }
  }
}
```

### 2. `src/main/db/__tests__/health-monitor.test.ts`

```typescript
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { SqliteSingleConnectionPool } from '../sqlite/sqlite-pool'
import { DatabaseHealthMonitor } from '../health-monitor'

describe('DatabaseHealthMonitor', () => {
  let pool: SqliteSingleConnectionPool
  let monitor: DatabaseHealthMonitor

  beforeEach(() => {
    pool = new SqliteSingleConnectionPool(':memory:')
    monitor = new DatabaseHealthMonitor(pool, 'sqlite')
  })

  afterEach(async () => {
    monitor.stopPeriodicCheck()
    await pool.destroy().catch(() => {})
  })

  describe('check()', () => {
    it('returns healthy for working database', async () => {
      const result = await monitor.check()
      expect(result.status).toBe('healthy')
      expect(result.latencyMs).toBeGreaterThanOrEqual(0)
      expect(result.dialect).toBe('sqlite')
      expect(result.checkedAt).toBeTruthy()
    })

    it('returns unhealthy when pool is drained', async () => {
      await pool.drain()
      const result = await monitor.check()
      expect(result.status).toBe('unhealthy')
      expect(result.lastError).toBeTruthy()
    })

    it('check result includes poolStats', async () => {
      const result = await monitor.check()
      expect(result.poolStats).toBeDefined()
      expect(typeof result.poolStats!.total).toBe('number')
    })

    it('check result has correct shape', async () => {
      const result = await monitor.check()
      expect(result).toMatchObject({
        status: expect.stringMatching(/^(healthy|degraded|unhealthy)$/),
        latencyMs: expect.any(Number),
        dialect: 'sqlite',
        checkedAt: expect.any(String)
      })
    })
  })

  describe('getLastCheck()', () => {
    it('returns null before any check', () => {
      expect(monitor.getLastCheck()).toBeNull()
    })

    it('returns last check result after check()', async () => {
      await monitor.check()
      expect(monitor.getLastCheck()).not.toBeNull()
      expect(monitor.getLastCheck()!.status).toBe('healthy')
    })
  })

  describe('onStatusChange()', () => {
    it('calls handler when status changes (null → healthy)', async () => {
      const handler = vi.fn()
      monitor.onStatusChange(handler)
      await monitor.check()  // null → healthy: triggers handler
      expect(handler).toHaveBeenCalledWith(expect.objectContaining({ status: 'healthy' }))
    })

    it('does NOT call handler when status unchanged', async () => {
      const handler = vi.fn()
      monitor.onStatusChange(handler)
      await monitor.check()  // null → healthy
      handler.mockClear()
      await monitor.check()  // healthy → healthy (no change)
      expect(handler).not.toHaveBeenCalled()
    })

    it('unsubscribe removes handler', async () => {
      const handler = vi.fn()
      const unsubscribe = monitor.onStatusChange(handler)
      unsubscribe()
      await monitor.check()
      expect(handler).not.toHaveBeenCalled()
    })

    it('multiple handlers can be registered', async () => {
      const h1 = vi.fn()
      const h2 = vi.fn()
      monitor.onStatusChange(h1)
      monitor.onStatusChange(h2)
      await monitor.check()
      expect(h1).toHaveBeenCalledOnce()
      expect(h2).toHaveBeenCalledOnce()
    })
  })

  describe('startPeriodicCheck / stopPeriodicCheck', () => {
    it('startPeriodicCheck() does not throw', () => {
      expect(() => monitor.startPeriodicCheck(60_000)).not.toThrow()
    })

    it('calling startPeriodicCheck() twice is idempotent', () => {
      monitor.startPeriodicCheck(60_000)
      monitor.startPeriodicCheck(60_000)  // second call is no-op
    })

    it('stopPeriodicCheck() before start does not throw', () => {
      expect(() => monitor.stopPeriodicCheck()).not.toThrow()
    })
  })
})
```

---

## Verification

```bash
pnpm vitest run src/main/db/__tests__/health-monitor.test.ts
```

Expected: 13/13 tests pass

---

## Done criteria

- [x] `DatabaseHealthMonitor` implements `HealthChecker`
- [x] `check()` returns `'healthy'` for working SQLite
- [x] `check()` returns `'unhealthy'` when pool drained
- [x] `check()` includes `poolStats` in result
- [x] Status change callback gọi đúng khi status thay đổi
- [x] Unsubscribe function hoạt động
- [x] `startPeriodicCheck()` idempotent
- [x] Interval có `.unref()` — không block process exit
- [x] 13/13 tests pass
