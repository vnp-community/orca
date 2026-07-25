# TASK-025: Tạo `src/main/admin/admin-user-handlers.ts` + test

> **Status:** ✅ DONE (2026-07-24)


**Phase:** 4 — Admin Panel
**Solution:** [SOL-LG-004](../solutions/SOL-LG-004-admin-ui.md) §4.4, §3.2
**Depends on:** TASK-022, TASK-023, TASK-004 (user-store), TASK-003 (session-store)
**Blocks:** TASK-031 (admin-router)

---

## Mục tiêu

Tạo `AdminUserHandlers` — CRUD users: list, create, deactivate (+ revoke all sessions). Kèm test với mocked stores.

---

## File 1: `src/main/admin/admin-user-handlers.ts`

```typescript
// src/main/admin/admin-user-handlers.ts
import type { Request, Response } from 'express'
import type { AuthUserStore } from '../auth/auth-user-store'
import type { AuthSessionStore } from '../auth/auth-session-store'
import type { AuditLogger } from './audit-logger'
import { AUDIT_ACTIONS } from './admin-types'

const VALID_ROLES = ['developer', 'lead', 'admin'] as const

export class AdminUserHandlers {
  constructor(private readonly deps: {
    userStore:    AuthUserStore
    sessionStore: AuthSessionStore
    auditLogger:  AuditLogger
  }) {}

  listUsers = async (_req: Request, res: Response): Promise<void> => {
    const users = this.deps.userStore.listUsers()
    res.json({ users, total: users.length })
  }

  createUser = async (req: Request, res: Response): Promise<void> => {
    const { email, name, password, role } = req.body ?? {}

    if (!email || !name || !password || !role) {
      res.status(400).json({ error: 'missing_fields', required: ['email', 'name', 'password', 'role'] })
      return
    }
    if (!VALID_ROLES.includes(role)) {
      res.status(400).json({ error: 'invalid_role', allowed: VALID_ROLES })
      return
    }

    const user = await this.deps.userStore.createLocalUser({ email, name, password, role })

    this.deps.auditLogger.log({
      userId:    req.orcaSession!.userId,
      userEmail: req.orcaSession!.userEmail,
      action:    AUDIT_ACTIONS.USER_CREATE,
      ipAddress: req.ip,
      detail:    { targetEmail: email, role }
    })

    res.status(201).json(user)
  }

  deactivateUser = async (req: Request, res: Response): Promise<void> => {
    const { id } = req.params

    // Prevent self-deactivation
    if (id === req.orcaSession!.userId) {
      res.status(400).json({ error: 'cannot_deactivate_self' })
      return
    }

    this.deps.userStore.deactivateUser(id!)
    const revokedCount = this.deps.sessionStore.revokeAllUserSessions(id!)

    this.deps.auditLogger.log({
      userId:    req.orcaSession!.userId,
      userEmail: req.orcaSession!.userEmail,
      action:    AUDIT_ACTIONS.USER_DEACTIVATE,
      ipAddress: req.ip,
      detail:    { targetUserId: id, revokedSessions: revokedCount }
    })

    res.json({ ok: true, revokedSessions: revokedCount })
  }
}
```

---

## File 2: `src/main/admin/__tests__/admin-user-handlers.test.ts`

```typescript
// src/main/admin/__tests__/admin-user-handlers.test.ts
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { AdminUserHandlers } from '../admin-user-handlers'
import type { AuthUserStore } from '../../auth/auth-user-store'
import type { AuthSessionStore } from '../../auth/auth-session-store'
import type { AuditLogger } from '../audit-logger'
import type { Request, Response } from 'express'

function mockRes() {
  const res: any = {}
  res.status = vi.fn().mockReturnValue(res)
  res.json   = vi.fn().mockReturnValue(res)
  return res as Response
}

function mockReq(overrides: Partial<Request> = {}): Request {
  return {
    orcaSession: { userId: 'admin-1', userEmail: 'admin@test.com', role: 'admin' },
    body: {}, params: {}, query: {}, ip: '127.0.0.1',
    ...overrides
  } as any
}

const mockUser = { id: 'u1', email: 'a@test.com', name: 'Alice', role: 'developer' as const, provider: 'none' as const }

describe('AdminUserHandlers', () => {
  let userStore: AuthUserStore
  let sessionStore: AuthSessionStore
  let auditLogger: AuditLogger
  let handlers: AdminUserHandlers

  beforeEach(() => {
    userStore    = { listUsers: vi.fn().mockReturnValue([mockUser]), createLocalUser: vi.fn().mockResolvedValue(mockUser), deactivateUser: vi.fn() } as any
    sessionStore = { revokeAllUserSessions: vi.fn().mockReturnValue(2) } as any
    auditLogger  = { log: vi.fn() } as any
    handlers = new AdminUserHandlers({ userStore, sessionStore, auditLogger })
  })

  describe('listUsers', () => {
    it('returns user list with total', async () => {
      const res = mockRes()
      await handlers.listUsers(mockReq(), res)
      expect(res.json).toHaveBeenCalledWith(expect.objectContaining({ users: [mockUser], total: 1 }))
    })
  })

  describe('createUser', () => {
    it('creates user and returns 201', async () => {
      const req = mockReq({ body: { email: 'new@test.com', name: 'New', password: 'pw123', role: 'developer' } })
      const res = mockRes()
      await handlers.createUser(req, res)
      expect(userStore.createLocalUser).toHaveBeenCalled()
      expect(res.status).toHaveBeenCalledWith(201)
    })

    it('logs audit event on create', async () => {
      const req = mockReq({ body: { email: 'new2@test.com', name: 'N', password: 'pw', role: 'developer' } })
      await handlers.createUser(req, mockRes())
      expect(auditLogger.log).toHaveBeenCalledWith(expect.objectContaining({ action: 'user.create' }))
    })

    it('returns 400 for missing fields', async () => {
      const req = mockReq({ body: { name: 'No Email' } })
      const res = mockRes()
      await handlers.createUser(req, res)
      expect(res.status).toHaveBeenCalledWith(400)
      expect(userStore.createLocalUser).not.toHaveBeenCalled()
    })

    it('returns 400 for invalid role', async () => {
      const req = mockReq({ body: { email: 'x@test.com', name: 'X', password: 'pw', role: 'superuser' } })
      const res = mockRes()
      await handlers.createUser(req, res)
      expect(res.status).toHaveBeenCalledWith(400)
    })
  })

  describe('deactivateUser', () => {
    it('deactivates user and revokes all sessions', async () => {
      const req = mockReq({ params: { id: 'u-target' } })
      const res = mockRes()
      await handlers.deactivateUser(req, res)
      expect(userStore.deactivateUser).toHaveBeenCalledWith('u-target')
      expect(sessionStore.revokeAllUserSessions).toHaveBeenCalledWith('u-target')
      expect(res.json).toHaveBeenCalledWith(expect.objectContaining({ ok: true }))
    })

    it('logs audit event on deactivate', async () => {
      const req = mockReq({ params: { id: 'u-target' } })
      await handlers.deactivateUser(req, mockRes())
      expect(auditLogger.log).toHaveBeenCalledWith(expect.objectContaining({ action: 'user.deactivate' }))
    })

    it('returns 400 when admin tries to deactivate themselves', async () => {
      const req = mockReq({ params: { id: 'admin-1' } })  // same as orcaSession.userId
      const res = mockRes()
      await handlers.deactivateUser(req, res)
      expect(res.status).toHaveBeenCalledWith(400)
      expect(userStore.deactivateUser).not.toHaveBeenCalled()
    })
  })
})
```

---

## Cách chạy test

```bash
pnpm test src/main/admin/__tests__/admin-user-handlers.test.ts
```

---

## Acceptance Criteria

- [x] `admin-user-handlers.ts` tồn tại, TypeScript compile sạch
- [x] `listUsers` trả về `{ users, total }`
- [x] `createUser` trả về 201 + gọi `auditLogger.log`
- [x] `createUser` trả về 400 nếu thiếu fields hoặc invalid role
- [x] `deactivateUser` gọi cả `deactivateUser` + `revokeAllUserSessions`
- [x] `deactivateUser` trả về 400 nếu admin tự deactivate mình
- [x] Tất cả test cases pass (≥ 8 cases)
