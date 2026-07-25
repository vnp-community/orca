# TASK-026: Tạo `src/main/admin/admin-session-handlers.ts` + test

> **Status:** ✅ DONE (2026-07-24)


**Phase:** 4 — Admin Panel
**Solution:** [SOL-LG-004](../solutions/SOL-LG-004-admin-ui.md) §4.5, §3.3
**Depends on:** TASK-022, TASK-023, TASK-003 (session-store)
**Blocks:** TASK-031 (admin-router)

---

## Mục tiêu

Tạo `AdminSessionHandlers` — kill session, kill all user sessions. Test với mocked stores.

---

## File 1: `src/main/admin/admin-session-handlers.ts`

```typescript
// src/main/admin/admin-session-handlers.ts
import type { Request, Response } from 'express'
import type { AuthSessionStore } from '../auth/auth-session-store'
import type { AuditLogger } from './audit-logger'
import { AUDIT_ACTIONS } from './admin-types'

export class AdminSessionHandlers {
  constructor(private readonly deps: {
    sessionStore: AuthSessionStore
    auditLogger:  AuditLogger
  }) {}

  listAllSessions = (_req: Request, res: Response): void => {
    // TODO: thêm AuthSessionStore.listAllActiveSessions() trong iteration sau
    res.json({ sessions: [], total: 0, note: 'Full listing not yet implemented' })
  }

  killSession = (req: Request, res: Response): void => {
    const { sessionId } = req.params

    this.deps.sessionStore.revokeSession(sessionId!)
    this.deps.auditLogger.log({
      userId:    req.orcaSession!.userId,
      userEmail: req.orcaSession!.userEmail,
      action:    AUDIT_ACTIONS.SESSION_KILL,
      ipAddress: req.ip,
      detail:    { targetSessionId: sessionId }
    })

    res.json({ ok: true })
  }

  killAllUserSessions = (req: Request, res: Response): void => {
    const { userId } = req.params

    const revokedCount = this.deps.sessionStore.revokeAllUserSessions(userId!)
    this.deps.auditLogger.log({
      userId:    req.orcaSession!.userId,
      userEmail: req.orcaSession!.userEmail,
      action:    AUDIT_ACTIONS.SESSION_KILL_ALL,
      ipAddress: req.ip,
      detail:    { targetUserId: userId, revokedCount }
    })

    res.json({ ok: true, revokedCount })
  }
}
```

---

## File 2: `src/main/admin/__tests__/admin-session-handlers.test.ts`

```typescript
// src/main/admin/__tests__/admin-session-handlers.test.ts
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { AdminSessionHandlers } from '../admin-session-handlers'
import type { AuthSessionStore } from '../../auth/auth-session-store'
import type { AuditLogger } from '../audit-logger'
import type { Request, Response } from 'express'

function mockRes() {
  const res: any = {}
  res.status = vi.fn().mockReturnValue(res)
  res.json   = vi.fn().mockReturnValue(res)
  return res as Response
}

describe('AdminSessionHandlers', () => {
  let sessionStore: AuthSessionStore
  let auditLogger: AuditLogger
  let handlers: AdminSessionHandlers
  const adminSession = { userId: 'admin-1', userEmail: 'admin@test.com', role: 'admin' as const }

  beforeEach(() => {
    sessionStore = {
      revokeSession:         vi.fn(),
      revokeAllUserSessions: vi.fn().mockReturnValue(3)
    } as any
    auditLogger = { log: vi.fn() } as any
    handlers = new AdminSessionHandlers({ sessionStore, auditLogger })
  })

  describe('killSession', () => {
    it('revokes the specified session', () => {
      const req: any = { orcaSession: adminSession, params: { sessionId: 'sid-abc' }, ip: '127.0.0.1' }
      handlers.killSession(req, mockRes())
      expect(sessionStore.revokeSession).toHaveBeenCalledWith('sid-abc')
    })

    it('logs audit event with targetSessionId', () => {
      const req: any = { orcaSession: adminSession, params: { sessionId: 'sid-xyz' }, ip: '1.2.3.4' }
      handlers.killSession(req, mockRes())
      expect(auditLogger.log).toHaveBeenCalledWith(expect.objectContaining({
        action: 'session.kill',
        detail: expect.objectContaining({ targetSessionId: 'sid-xyz' })
      }))
    })

    it('returns { ok: true }', () => {
      const req: any = { orcaSession: adminSession, params: { sessionId: 'sid-1' }, ip: '127.0.0.1' }
      const res = mockRes()
      handlers.killSession(req, res)
      expect(res.json).toHaveBeenCalledWith({ ok: true })
    })
  })

  describe('killAllUserSessions', () => {
    it('revokes all sessions for target user', () => {
      const req: any = { orcaSession: adminSession, params: { userId: 'user-target' }, ip: '127.0.0.1' }
      handlers.killAllUserSessions(req, mockRes())
      expect(sessionStore.revokeAllUserSessions).toHaveBeenCalledWith('user-target')
    })

    it('logs audit event with revokedCount', () => {
      const req: any = { orcaSession: adminSession, params: { userId: 'user-t' }, ip: '127.0.0.1' }
      handlers.killAllUserSessions(req, mockRes())
      expect(auditLogger.log).toHaveBeenCalledWith(expect.objectContaining({
        action: 'session.kill_all',
        detail: expect.objectContaining({ revokedCount: 3 })
      }))
    })

    it('returns { ok: true, revokedCount }', () => {
      const req: any = { orcaSession: adminSession, params: { userId: 'u1' }, ip: '127.0.0.1' }
      const res = mockRes()
      handlers.killAllUserSessions(req, res)
      expect(res.json).toHaveBeenCalledWith({ ok: true, revokedCount: 3 })
    })
  })
})
```

---

## Cách chạy test

```bash
pnpm test src/main/admin/__tests__/admin-session-handlers.test.ts
```

---

## Acceptance Criteria

- [x] `admin-session-handlers.ts` tồn tại, TypeScript compile sạch
- [x] `killSession` gọi `revokeSession` + `auditLogger.log`
- [x] `killAllUserSessions` gọi `revokeAllUserSessions` + `auditLogger.log`
- [x] `listAllSessions` trả về `{ sessions: [], total: 0 }` (stub chấp nhận được)
- [x] Tất cả test cases pass (≥ 6 cases)
