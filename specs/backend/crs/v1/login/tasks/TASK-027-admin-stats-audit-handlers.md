# TASK-027: Tạo `src/main/admin/admin-stats-handler.ts` và `admin-audit-handlers.ts`

> **Status:** ✅ DONE (2026-07-24)


**Phase:** 4 — Admin Panel
**Solution:** [SOL-LG-004](../solutions/SOL-LG-004-admin-ui.md) §4.6, §4.7
**Depends on:** TASK-022 (admin-types), TASK-023 (audit-logger), TASK-001 (migration)
**Blocks:** TASK-031 (admin-router)

---

## Mục tiêu

Tạo 2 handler files nhỏ: stats dashboard và audit log query.

---

## File 1: `src/main/admin/admin-stats-handler.ts`

```typescript
// src/main/admin/admin-stats-handler.ts
import type { Request, Response } from 'express'
import type { IDatabase } from '../db/types'
import type { AdminStats } from './admin-types'

export class AdminStatsHandler {
  constructor(private readonly db: IDatabase) {}

  getStats = (_req: Request, res: Response): void => {
    const now = Date.now()

    const stats: AdminStats = {
      totalUsers:     this.countQuery('SELECT COUNT(*) FROM orca_users'),
      activeUsers:    this.countQuery('SELECT COUNT(*) FROM orca_users WHERE is_active = 1'),
      activeSessions: this.countQuery(`SELECT COUNT(*) FROM orca_sessions WHERE expires_at > ${now}`),
      pairedDevices:  0  // Stub — DeviceRegistry data không available tại đây
    }

    res.json(stats)
  }

  private countQuery(sql: string): number {
    const row = this.db.prepare(sql).get() as Record<string, number> | undefined
    if (!row) return 0
    return Object.values(row)[0] ?? 0
  }
}
```

---

## File 2: `src/main/admin/admin-audit-handlers.ts`

```typescript
// src/main/admin/admin-audit-handlers.ts
import type { Request, Response } from 'express'
import type { AuditLogger } from './audit-logger'

export class AdminAuditHandlers {
  constructor(private readonly auditLogger: AuditLogger) {}

  queryAuditLog = (req: Request, res: Response): void => {
    const { userId, action, from, to, limit, offset } = req.query as Record<string, string>

    const events = this.auditLogger.query({
      userId:  userId  || undefined,
      action:  action  || undefined,
      from:    from    ? Number(from)   : undefined,
      to:      to      ? Number(to)     : undefined,
      limit:   limit   ? Math.min(Number(limit), 1000)  : 100,
      offset:  offset  ? Number(offset) : 0
    })

    res.json({ events, total: events.length })
  }
}
```

---

## Acceptance Criteria

### admin-stats-handler.ts
- [x] File tồn tại, TypeScript compile sạch
- [x] `getStats()` trả về `AdminStats` với đúng 4 fields
- [x] `activeSessions` chỉ đếm sessions chưa expired (expires_at > now)
- [x] `pairedDevices` = 0 (stub, không crash)

### admin-audit-handlers.ts
- [x] File tồn tại, TypeScript compile sạch
- [x] `queryAuditLog()` parse `userId`, `action`, `from`, `to`, `limit`, `offset` từ query string
- [x] Cap `limit` tại 1000
- [x] Trả về `{ events, total }` JSON
