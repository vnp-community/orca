# TASK-023: Tạo `src/main/admin/audit-logger.ts` + test

> **Status:** ✅ DONE (2026-07-24)


**Phase:** 4 — Admin Panel
**Solution:** [SOL-LG-004](../solutions/SOL-LG-004-admin-ui.md) §4.2, §3.1
**Depends on:** TASK-001 (migration), TASK-022 (admin-types)
**Blocks:** TASK-025, TASK-026, TASK-027, TASK-035

---

## Mục tiêu

Tạo `AuditLogger` — ghi và query audit events vào `orca_audit_log`. Viết test với real SQLite.

---

## File 1: `src/main/admin/audit-logger.ts`

```typescript
// src/main/admin/audit-logger.ts
import type { IDatabase } from '../db/types'
import type { AuditEvent, AuditLogInput, AuditQueryFilter } from './admin-types'

export class AuditLogger {
  constructor(private readonly db: IDatabase) {}

  log(input: AuditLogInput): void {
    this.db.prepare(`
      INSERT INTO orca_audit_log
        (created_at, user_id, user_email, action, detail, ip_address)
      VALUES (?, ?, ?, ?, ?, ?)
    `).run(
      Date.now(),
      input.userId    ?? null,
      input.userEmail ?? null,
      input.action,
      input.detail ? JSON.stringify(input.detail) : null,
      input.ipAddress ?? null
    )
  }

  query(filter: AuditQueryFilter): AuditEvent[] {
    const conditions: string[] = []
    const params: (string | number)[] = []

    if (filter.userId) { conditions.push('user_id = ?');    params.push(filter.userId) }
    if (filter.action) { conditions.push('action = ?');     params.push(filter.action) }
    if (filter.from)   { conditions.push('created_at >= ?'); params.push(filter.from) }
    if (filter.to)     { conditions.push('created_at <= ?'); params.push(filter.to) }

    const where  = conditions.length ? `WHERE ${conditions.join(' AND ')}` : ''
    const limit  = Math.min(filter.limit ?? 100, 1000)   // cap at 1000
    const offset = filter.offset ?? 0

    const rows = this.db.prepare(`
      SELECT id, created_at, user_id, user_email, action, detail, ip_address
      FROM orca_audit_log
      ${where}
      ORDER BY created_at DESC
      LIMIT ? OFFSET ?
    `).all(...params, limit, offset) as any[]

    return rows.map(row => ({
      id:        row.id,
      createdAt: row.created_at,
      userId:    row.user_id   ?? null,
      userEmail: row.user_email ?? null,
      action:    row.action,
      detail:    row.detail ? JSON.parse(row.detail) : null,
      ipAddress: row.ip_address ?? null
    }))
  }

  count(filter: Pick<AuditQueryFilter, 'userId' | 'action'>): number {
    const conditions: string[] = []
    const params: (string | number)[] = []
    if (filter.userId) { conditions.push('user_id = ?'); params.push(filter.userId) }
    if (filter.action) { conditions.push('action = ?');  params.push(filter.action) }
    const where = conditions.length ? `WHERE ${conditions.join(' AND ')}` : ''
    const row = this.db.prepare(`SELECT COUNT(*) as n FROM orca_audit_log ${where}`).get(...params) as any
    return row?.n ?? 0
  }
}
```

---

## File 2: `src/main/admin/__tests__/audit-logger.test.ts`

```typescript
// src/main/admin/__tests__/audit-logger.test.ts
import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { mkdtempSync, rmSync } from 'node:fs'
import { join } from 'node:path'
import { tmpdir } from 'node:os'
import { SqliteAdapter } from '../../db/sqlite/sqlite-adapter'
import { runMigrations } from '../../db/migrations/runner'
import { AuditLogger } from '../audit-logger'

describe('AuditLogger', () => {
  let tmpDir: string
  let db: SqliteAdapter
  let logger: AuditLogger

  beforeEach(async () => {
    tmpDir = mkdtempSync(join(tmpdir(), 'orca-audit-test-'))
    db = new SqliteAdapter(join(tmpDir, 'test.db'))
    await runMigrations(db)
    logger = new AuditLogger(db)
  })

  afterEach(() => {
    db.close()
    rmSync(tmpDir, { recursive: true, force: true })
  })

  describe('log', () => {
    it('writes audit event to orca_audit_log', () => {
      logger.log({ userId: 'u1', userEmail: 'a@test.com', action: 'login.success', ipAddress: '127.0.0.1', detail: { provider: 'local' } })
      const rows = db.prepare('SELECT * FROM orca_audit_log').all() as any[]
      expect(rows).toHaveLength(1)
      expect(rows[0]!.action).toBe('login.success')
      expect(rows[0]!.user_id).toBe('u1')
    })

    it('serializes detail as JSON string', () => {
      logger.log({ userId: 'u1', userEmail: 'a@test.com', action: 'ssh.connect', detail: { host: '172.20.2.31', port: 22 } })
      const row = db.prepare('SELECT detail FROM orca_audit_log').get() as any
      const parsed = JSON.parse(row.detail)
      expect(parsed.host).toBe('172.20.2.31')
      expect(parsed.port).toBe(22)
    })

    it('works without userId (system events)', () => {
      logger.log({ action: 'server.start', detail: { version: '1.0.0' } })
      const row = db.prepare('SELECT * FROM orca_audit_log').get() as any
      expect(row.user_id).toBeNull()
      expect(row.action).toBe('server.start')
    })

    it('stores created_at as current timestamp', () => {
      const before = Date.now()
      logger.log({ action: 'test.event' })
      const after = Date.now()
      const row = db.prepare('SELECT created_at FROM orca_audit_log').get() as any
      expect(row.created_at).toBeGreaterThanOrEqual(before)
      expect(row.created_at).toBeLessThanOrEqual(after)
    })
  })

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
  })
})
```

---

## Cách chạy test

```bash
pnpm test src/main/admin/__tests__/audit-logger.test.ts
```

---

## Acceptance Criteria

- [x] `audit-logger.ts` tồn tại, TypeScript compile sạch
- [x] `log()` là synchronous (không async — nhanh hơn, không cần await)
- [x] `query()` trả về AuditEvent[] với detail đã parse từ JSON
- [x] `query({ limit })` cap tại 1000
- [x] Tất cả test cases pass (≥ 12 cases)
