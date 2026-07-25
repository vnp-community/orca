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
    await pool.destroy()
  })

  describe('check()', () => {
    it('returns healthy for working pool', async () => {
      const result = await monitor.check()
      expect(result.status).toBe('healthy')
      expect(result.dialect).toBe('sqlite')
      expect(result.latencyMs).toBeGreaterThanOrEqual(0)
      expect(result.checkedAt).toBeTruthy()
    })

    it('includes poolStats in result', async () => {
      const result = await monitor.check()
      expect(result.poolStats).toBeDefined()
      expect(typeof result.poolStats!.total).toBe('number')
    })

    it('returns unhealthy when pool throws', async () => {
      const failPool = {
        withConnection: async () => {
          throw new Error('Connection refused')
        },
        stats: () => ({ total: 0, idle: 0, acquired: 0, waiting: 0 }),
        acquire: async () => { throw new Error() },
        release: () => {},
        withTransaction: async () => { throw new Error() },
        drain: async () => {},
        destroy: async () => {}
      }
      const failMonitor = new DatabaseHealthMonitor(failPool as any, 'mysql')
      const result = await failMonitor.check()
      expect(result.status).toBe('unhealthy')
      expect(result.lastError).toContain('Connection refused')
    })

    it('sets checkedAt to ISO 8601 timestamp', async () => {
      const result = await monitor.check()
      expect(() => new Date(result.checkedAt)).not.toThrow()
      expect(new Date(result.checkedAt).toISOString()).toBe(result.checkedAt)
    })
  })

  describe('getLastCheck()', () => {
    it('returns null before any check', () => {
      expect(monitor.getLastCheck()).toBeNull()
    })

    it('returns last result after check()', async () => {
      await monitor.check()
      const last = monitor.getLastCheck()
      expect(last).not.toBeNull()
      expect(last!.status).toBe('healthy')
    })
  })

  describe('onStatusChange()', () => {
    it('calls handler on first status change', async () => {
      const handler = vi.fn()
      monitor.onStatusChange(handler)
      await monitor.check()
      expect(handler).toHaveBeenCalledOnce()
    })

    it('does NOT call handler if status same as before', async () => {
      const handler = vi.fn()
      monitor.onStatusChange(handler)
      await monitor.check()
      await monitor.check()
      // Both healthy — handler should only be called once (first status change from null)
      expect(handler).toHaveBeenCalledTimes(1)
    })

    it('returns unsubscribe function that stops calls', async () => {
      const handler = vi.fn()
      const unsub = monitor.onStatusChange(handler)
      unsub()
      await monitor.check()
      expect(handler).not.toHaveBeenCalled()
    })
  })

  describe('startPeriodicCheck() / stopPeriodicCheck()', () => {
    it('stopPeriodicCheck() does not throw if not started', () => {
      expect(() => monitor.stopPeriodicCheck()).not.toThrow()
    })

    it('startPeriodicCheck() triggers immediate check', async () => {
      const handler = vi.fn()
      monitor.onStatusChange(handler)
      monitor.startPeriodicCheck(60_000) // long interval — only fires immediately
      // Give a tick for the immediate check to complete
      await new Promise((r) => setTimeout(r, 50))
      monitor.stopPeriodicCheck()
      expect(handler).toHaveBeenCalledOnce()
    })

    it('calling startPeriodicCheck() twice is idempotent', () => {
      monitor.startPeriodicCheck(60_000)
      expect(() => monitor.startPeriodicCheck(60_000)).not.toThrow()
      monitor.stopPeriodicCheck()
    })
  })
})
