/**
 * AuditLogger Unit Tests
 *
 * Uses real SQLite in-memory DB with all migrations applied.
 */

import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { SqliteAdapter } from '../../db/sqlite/sqlite-adapter'
import { MigrationRunner } from '../../db/migrations/runner'
import { ALL_MIGRATIONS } from '../../db/migrations/index'
import { AuditLogger } from '../audit-logger'

describe('AuditLogger', () => {
  let db: SqliteAdapter
  let logger: AuditLogger

  beforeEach(async () => {
    db = new SqliteAdapter(':memory:')
    const runner = new MigrationRunner(db, ALL_MIGRATIONS)
    await runner.migrate()
    logger = new AuditLogger(db)
  })

  afterEach(() => db.close())

  // ── log ────────────────────────────────────────────────────────────────────

  describe('log', () => {
    it('writes audit event to orca_audit_log', () => {
      logger.log({ userId: 'u1', userEmail: 'a@test.com', action: 'login.success', ipAddress: '127.0.0.1', detail: { provider: 'local' } })
      const rows = db.prepare('SELECT * FROM orca_audit_log').all() as Record<string, unknown>[]
      expect(rows).toHaveLength(1)
      expect(rows[0]!['action']).toBe('login.success')
      expect(rows[0]!['user_id']).toBe('u1')
    })

    it('serializes detail as JSON string in DB', () => {
      logger.log({ action: 'ssh.connect', detail: { host: '172.20.2.31', port: 22 } })
      const row = db.prepare('SELECT detail FROM orca_audit_log').get() as Record<string, unknown>
      const parsed = JSON.parse(row['detail'] as string) as { host: string; port: number }
      expect(parsed.host).toBe('172.20.2.31')
      expect(parsed.port).toBe(22)
    })

    it('works without userId (system events)', () => {
      logger.log({ action: 'server.start', detail: { version: '1.0.0' } })
      const row = db.prepare('SELECT * FROM orca_audit_log').get() as Record<string, unknown>
      expect(row['user_id']).toBeNull()
      expect(row['action']).toBe('server.start')
    })

    it('stores created_at as current timestamp (ms)', () => {
      const before = Date.now()
      logger.log({ action: 'test.event' })
      const after = Date.now()
      const row = db.prepare('SELECT created_at FROM orca_audit_log').get() as Record<string, unknown>
      expect(row['created_at']).toBeGreaterThanOrEqual(before)
      expect(row['created_at']).toBeLessThanOrEqual(after)
    })

    it('stores null for detail when not provided', () => {
      logger.log({ action: 'logout' })
      const row = db.prepare('SELECT detail FROM orca_audit_log').get() as Record<string, unknown>
      expect(row['detail']).toBeNull()
    })
  })

  // ── query ──────────────────────────────────────────────────────────────────

  describe('query', () => {
    beforeEach(() => {
      logger.log({ userId: 'u1', userEmail: 'a@test.com', action: 'login.success', ipAddress: '1.1.1.1', detail: {} })
      logger.log({ userId: 'u2', userEmail: 'b@test.com', action: 'login.success', ipAddress: '2.2.2.2', detail: {} })
      logger.log({ userId: 'u1', userEmail: 'a@test.com', action: 'ssh.connect',   ipAddress: '1.1.1.1', detail: {} })
    })

    it('returns all events without filter', () => {
      expect(logger.query({})).toHaveLength(3)
    })

    it('filters by userId', () => {
      const events = logger.query({ userId: 'u1' })
      expect(events).toHaveLength(2)
      expect(events.every(e => e.userId === 'u1')).toBe(true)
    })

    it('filters by action', () => {
      const events = logger.query({ action: 'ssh.connect' })
      expect(events).toHaveLength(1)
      expect(events[0]!.action).toBe('ssh.connect')
    })

    it('combines userId + action filters', () => {
      const events = logger.query({ userId: 'u1', action: 'login.success' })
      expect(events).toHaveLength(1)
    })

    it('respects limit', () => {
      expect(logger.query({ limit: 1 })).toHaveLength(1)
    })

    it('caps limit at 1000', () => {
      // Even if 9999 is passed, it should not exceed 1000
      const events = logger.query({ limit: 9999 })
      expect(events.length).toBeLessThanOrEqual(1000)
    })

    it('orders by created_at DESC (most recent first)', () => {
      const events = logger.query({})
      for (let i = 0; i < events.length - 1; i++) {
        expect(events[i]!.createdAt).toBeGreaterThanOrEqual(events[i + 1]!.createdAt)
      }
    })

    it('parses detail from JSON string', () => {
      logger.log({ action: 'test.detail', detail: { key: 'value' } })
      const events = logger.query({ action: 'test.detail' })
      expect(events[0]!.detail).toEqual({ key: 'value' })
    })

    it('maps row fields to AuditEvent correctly', () => {
      const events = logger.query({ action: 'ssh.connect' })
      const e = events[0]!
      expect(typeof e.id).toBe('number')
      expect(typeof e.createdAt).toBe('number')
      expect(e.userId).toBe('u1')
      expect(e.userEmail).toBe('a@test.com')
      expect(e.ipAddress).toBe('1.1.1.1')
    })
  })

  // ── count ──────────────────────────────────────────────────────────────────

  describe('count', () => {
    beforeEach(() => {
      logger.log({ userId: 'u1', action: 'login.success' })
      logger.log({ userId: 'u1', action: 'login.success' })
      logger.log({ userId: 'u2', action: 'login.failure' })
    })

    it('counts all events without filter', () => {
      expect(logger.count({})).toBe(3)
    })

    it('counts by userId', () => {
      expect(logger.count({ userId: 'u1' })).toBe(2)
    })

    it('counts by action', () => {
      expect(logger.count({ action: 'login.failure' })).toBe(1)
    })

    it('combines userId + action in count', () => {
      expect(logger.count({ userId: 'u1', action: 'login.success' })).toBe(2)
      expect(logger.count({ userId: 'u2', action: 'login.success' })).toBe(0)
    })
  })
})
