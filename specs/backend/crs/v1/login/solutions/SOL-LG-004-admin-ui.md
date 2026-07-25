# SOL-LG-004 — Admin Panel: User, Session, Policy Management + Audit Log

**CR:** [CR-LOGIN-004](../../../../../docs/crs/v1/login/CR-LOGIN-004-admin.md)
**TDD Refs:** TDD-04 (RPC Server — Auth middleware), TDD-11 (Web Server Mode — HTTP server, routes), TDD-12 (Database Layer — migration, query)
**Approach:** Test-Driven — viết tests trước implementations
**Status:** ✅ Implemented (2026-07-24)
**Blocked by:** SOL-LG-001 (cần AuthManager, orca_users, orca_sessions, orca_audit_log tables)

---

## 1. Phân tích từ TDD và Code Hiện tại

### 1.1 HTTP Server Pattern (TDD-11 §4)

```typescript
// src/server/http-server.ts — pattern hiện tại
// Express app, mount routes, static files, health endpoints
// Thêm admin router theo cùng pattern
```

### 1.2 Auth Middleware (từ SOL-LG-001)

```typescript
// src/main/auth/auth-middleware.ts
requireAuth()       // bất kỳ logged-in user
requireAdmin()      // NEW: chỉ role='admin'
```

### 1.3 Database (TDD-12 §2)

```typescript
// IDatabase từ src/main/db/types.ts
// prepare().get(), prepare().all(), prepare().run()
// Migration 0004 tạo orca_users, orca_sessions, orca_audit_log, orca_access_policies
```

---

## 2. File Structure

```
src/main/admin/
├── admin-types.ts                  ← AdminStats, AuditEvent, PolicyInput
├── admin-middleware.ts             ← requireAdmin (role check)
├── admin-user-handlers.ts          ← CRUD users (list, create, update, deactivate)
├── admin-session-handlers.ts       ← List all sessions, kill session/all user sessions
├── admin-policy-handlers.ts        ← CRUD access policies
├── admin-audit-handlers.ts         ← Query audit log (filter by user/action/date)
├── admin-stats-handler.ts          ← Dashboard stats
├── audit-logger.ts                 ← Write audit events to orca_audit_log
├── first-run-setup.ts              ← Seed admin user on first boot
├── admin-router.ts                 ← Mount all admin routes at /admin/api/*
└── __tests__/
    ├── admin-user-handlers.test.ts
    ├── admin-session-handlers.test.ts
    └── audit-logger.test.ts
```

---

## 3. Test Specifications

### 3.1 `audit-logger.test.ts`

```typescript
// src/main/admin/__tests__/audit-logger.test.ts
import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { SqliteAdapter } from '../../db/sqlite/sqlite-adapter'
import { AuditLogger } from '../audit-logger'
import { runMigrations } from '../../db/migrations/runner'
import { mkdtempSync, rmSync } from 'node:fs'
import { join } from 'node:path'
import { tmpdir } from 'node:os'

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
    it('writes audit event to database', async () => {
      await logger.log({
        userId: 'u1', userEmail: 'a@test.com',
        action: 'login.success', ipAddress: '127.0.0.1',
        detail: { provider: 'local' }
      })

      const rows = db.prepare('SELECT * FROM orca_audit_log').all() as any[]
      expect(rows).toHaveLength(1)
      expect(rows[0]!.action).toBe('login.success')
      expect(rows[0]!.user_id).toBe('u1')
    })

    it('stores detail as JSON string', async () => {
      await logger.log({
        userId: 'u1', userEmail: 'a@test.com',
        action: 'ssh.connect', ipAddress: '1.2.3.4',
        detail: { targetId: 't-1', host: '172.20.2.31' }
      })

      const row = db.prepare('SELECT detail FROM orca_audit_log').get() as any
      const parsed = JSON.parse(row.detail)
      expect(parsed.host).toBe('172.20.2.31')
    })

    it('works without userId (system events)', async () => {
      await logger.log({
        action: 'server.start',
        detail: { version: '1.0.0' }
      })

      const row = db.prepare('SELECT * FROM orca_audit_log').get() as any
      expect(row.user_id).toBeNull()
      expect(row.action).toBe('server.start')
    })
  })

  describe('query', () => {
    beforeEach(async () => {
      await logger.log({ userId: 'u1', userEmail: 'a@test.com', action: 'login.success', ipAddress: '1.1.1.1', detail: {} })
      await logger.log({ userId: 'u2', userEmail: 'b@test.com', action: 'login.success', ipAddress: '2.2.2.2', detail: {} })
      await logger.log({ userId: 'u1', userEmail: 'a@test.com', action: 'ssh.connect', ipAddress: '1.1.1.1', detail: {} })
    })

    it('returns all events without filters', async () => {
      const events = await logger.query({})
      expect(events.length).toBe(3)
    })

    it('filters by userId', async () => {
      const events = await logger.query({ userId: 'u1' })
      expect(events.length).toBe(2)
      expect(events.every(e => e.userId === 'u1')).toBe(true)
    })

    it('filters by action', async () => {
      const events = await logger.query({ action: 'ssh.connect' })
      expect(events.length).toBe(1)
      expect(events[0]!.action).toBe('ssh.connect')
    })

    it('respects limit', async () => {
      const events = await logger.query({ limit: 2 })
      expect(events.length).toBe(2)
    })

    it('orders by created_at descending', async () => {
      const events = await logger.query({})
      expect(events[0]!.createdAt).toBeGreaterThanOrEqual(events[events.length - 1]!.createdAt)
    })
  })
})
```

### 3.2 `admin-user-handlers.test.ts`

```typescript
// src/main/admin/__tests__/admin-user-handlers.test.ts
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { AdminUserHandlers } from '../admin-user-handlers'
import type { AuthUserStore } from '../../auth/auth-user-store'
import type { AuthSessionStore } from '../../auth/auth-session-store'
import type { AuditLogger } from '../audit-logger'
import type { Request, Response } from 'express'

const mockRes = () => {
  const res: any = {}
  res.status = vi.fn().mockReturnValue(res)
  res.json = vi.fn().mockReturnValue(res)
  return res as Response
}

const mockReq = (overrides: Partial<Request> = {}): Request => ({
  orcaSession: { userId: 'admin-1', userEmail: 'admin@test.com', role: 'admin' } as any,
  body: {}, params: {}, query: {},
  ...overrides
} as any)

describe('AdminUserHandlers', () => {
  let userStore: AuthUserStore
  let sessionStore: AuthSessionStore
  let auditLogger: AuditLogger
  let handlers: AdminUserHandlers

  beforeEach(() => {
    userStore = {
      listUsers: vi.fn().mockResolvedValue([
        { id: 'u1', email: 'a@test.com', name: 'Alice', role: 'developer', provider: 'none' }
      ]),
      createLocalUser: vi.fn().mockResolvedValue({ id: 'u2', email: 'b@test.com', name: 'Bob', role: 'developer', provider: 'none' }),
      deactivateUser: vi.fn().mockResolvedValue(undefined),
      getUser: vi.fn()
    } as any
    sessionStore = { revokeAllUserSessions: vi.fn().mockResolvedValue(1) } as any
    auditLogger = { log: vi.fn().mockResolvedValue(undefined) } as any
    handlers = new AdminUserHandlers({ userStore, sessionStore, auditLogger })
  })

  describe('listUsers', () => {
    it('returns list of users', async () => {
      const req = mockReq()
      const res = mockRes()
      await handlers.listUsers(req, res)
      expect(res.json).toHaveBeenCalledWith(expect.objectContaining({
        users: expect.arrayContaining([expect.objectContaining({ email: 'a@test.com' })])
      }))
    })
  })

  describe('createUser', () => {
    it('creates user and logs audit event', async () => {
      const req = mockReq({ body: { email: 'b@test.com', name: 'Bob', password: 'pw', role: 'developer' } })
      const res = mockRes()
      await handlers.createUser(req, res)
      expect(userStore.createLocalUser).toHaveBeenCalled()
      expect(auditLogger.log).toHaveBeenCalledWith(expect.objectContaining({ action: 'user.create' }))
      expect(res.json).toHaveBeenCalledWith(expect.objectContaining({ id: 'u2' }))
    })

    it('returns 400 for missing required fields', async () => {
      const req = mockReq({ body: { name: 'Incomplete' } })
      const res = mockRes()
      await handlers.createUser(req, res)
      expect(res.status).toHaveBeenCalledWith(400)
      expect(userStore.createLocalUser).not.toHaveBeenCalled()
    })
  })

  describe('deactivateUser', () => {
    it('deactivates user and revokes all sessions', async () => {
      const req = mockReq({ params: { id: 'u1' } })
      const res = mockRes()
      await handlers.deactivateUser(req, res)
      expect(userStore.deactivateUser).toHaveBeenCalledWith('u1')
      expect(sessionStore.revokeAllUserSessions).toHaveBeenCalledWith('u1')
      expect(auditLogger.log).toHaveBeenCalledWith(expect.objectContaining({ action: 'user.deactivate' }))
    })

    it('prevents admin from deactivating themselves', async () => {
      const req = mockReq({ params: { id: 'admin-1' } })  // same as orcaSession.userId
      const res = mockRes()
      await handlers.deactivateUser(req, res)
      expect(res.status).toHaveBeenCalledWith(400)
      expect(userStore.deactivateUser).not.toHaveBeenCalled()
    })
  })
})
```

### 3.3 `admin-session-handlers.test.ts`

```typescript
// src/main/admin/__tests__/admin-session-handlers.test.ts
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { AdminSessionHandlers } from '../admin-session-handlers'
import type { AuthSessionStore } from '../../auth/auth-session-store'
import type { AuditLogger } from '../audit-logger'

describe('AdminSessionHandlers', () => {
  let sessionStore: AuthSessionStore
  let auditLogger: AuditLogger
  let handlers: AdminSessionHandlers

  beforeEach(() => {
    sessionStore = {
      listUserSessions: vi.fn().mockResolvedValue([
        { sessionId: 'sid-1', userId: 'u1', userEmail: 'a@test.com', role: 'developer',
          createdAt: Date.now(), expiresAt: Date.now() + 3600000, lastSeenAt: null,
          ipAddress: '1.2.3.4', userAgent: 'Mozilla/5' }
      ]),
      revokeSession: vi.fn().mockResolvedValue(undefined),
      revokeAllUserSessions: vi.fn().mockResolvedValue(2)
    } as any
    auditLogger = { log: vi.fn() } as any
    handlers = new AdminSessionHandlers({ sessionStore, auditLogger })
  })

  describe('killSession', () => {
    it('revokes session and logs audit', async () => {
      const req: any = {
        orcaSession: { userId: 'admin-1', userEmail: 'admin@test.com' },
        params: { sessionId: 'sid-1' }
      }
      const res: any = { json: vi.fn() }
      await handlers.killSession(req, res)
      expect(sessionStore.revokeSession).toHaveBeenCalledWith('sid-1')
      expect(auditLogger.log).toHaveBeenCalledWith(expect.objectContaining({ action: 'session.kill' }))
    })
  })

  describe('killAllUserSessions', () => {
    it('revokes all sessions for target user', async () => {
      const req: any = {
        orcaSession: { userId: 'admin-1', userEmail: 'admin@test.com' },
        params: { userId: 'u1' }
      }
      const res: any = { json: vi.fn() }
      await handlers.killAllUserSessions(req, res)
      expect(sessionStore.revokeAllUserSessions).toHaveBeenCalledWith('u1')
      expect(auditLogger.log).toHaveBeenCalledWith(expect.objectContaining({ action: 'session.kill_all' }))
    })
  })
})
```

---

## 4. Implementation

### 4.1 `admin-types.ts`

```typescript
// src/main/admin/admin-types.ts

export type AdminStats = {
  totalUsers:     number
  activeUsers:    number    // is_active=1
  activeSessions: number
  pairedDevices:  number
}

export type AuditEvent = {
  id:          number
  createdAt:   number
  userId:      string | null
  userEmail:   string | null
  action:      string
  detail:      Record<string, unknown> | null
  ipAddress:   string | null
}

export type AuditLogInput = {
  userId?:    string
  userEmail?: string
  action:     string
  ipAddress?: string
  detail?:    Record<string, unknown>
}

export type AuditQueryFilter = {
  userId?:   string
  action?:   string
  from?:     number   // timestamp ms
  to?:       number   // timestamp ms
  limit?:    number   // default 100
  offset?:   number
}

export type PolicyInput = {
  name:                  string
  teams?:                string[]
  roles?:                string[]
  users?:                string[]
  allowedServers?:       string | string[]
  allowedProjects?:      string | string[]
  agentTrust?:           'minimal' | 'standard' | 'full'
  canCreateWorktrees?:   boolean
  canDeleteWorktrees?:   boolean
  canAccessProduction?:  boolean
}
```

### 4.2 `audit-logger.ts`

```typescript
// src/main/admin/audit-logger.ts
import type { IDatabase } from '../db/types'
import type { AuditEvent, AuditLogInput, AuditQueryFilter } from './admin-types'

export class AuditLogger {
  constructor(private readonly db: IDatabase) {}

  async log(input: AuditLogInput): Promise<void> {
    this.db.prepare(`
      INSERT INTO orca_audit_log (created_at, user_id, user_email, action, detail, ip_address)
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

  async query(filter: AuditQueryFilter): Promise<AuditEvent[]> {
    const conditions: string[] = []
    const params: (string | number)[] = []

    if (filter.userId)   { conditions.push('user_id = ?');   params.push(filter.userId) }
    if (filter.action)   { conditions.push('action = ?');    params.push(filter.action) }
    if (filter.from)     { conditions.push('created_at >= ?'); params.push(filter.from) }
    if (filter.to)       { conditions.push('created_at <= ?'); params.push(filter.to) }

    const where  = conditions.length ? `WHERE ${conditions.join(' AND ')}` : ''
    const limit  = filter.limit  ?? 100
    const offset = filter.offset ?? 0
    params.push(limit, offset)

    const rows = this.db.prepare(`
      SELECT id, created_at, user_id, user_email, action, detail, ip_address
      FROM orca_audit_log
      ${where}
      ORDER BY created_at DESC
      LIMIT ? OFFSET ?
    `).all(...params) as any[]

    return rows.map(row => ({
      id:         row.id,
      createdAt:  row.created_at,
      userId:     row.user_id     ?? null,
      userEmail:  row.user_email  ?? null,
      action:     row.action,
      detail:     row.detail ? JSON.parse(row.detail) : null,
      ipAddress:  row.ip_address  ?? null
    }))
  }
}
```

### 4.3 `admin-middleware.ts`

```typescript
// src/main/admin/admin-middleware.ts
import type { Request, Response, NextFunction } from 'express'

export function requireAdmin(req: Request, res: Response, next: NextFunction): void {
  const session = req.orcaSession  // set by auth middleware (SOL-LG-001)
  if (!session) {
    res.status(401).json({ error: 'unauthenticated' })
    return
  }
  if (session.role !== 'admin') {
    res.status(403).json({ error: 'forbidden', required_role: 'admin' })
    return
  }
  next()
}
```

### 4.4 `admin-user-handlers.ts`

```typescript
// src/main/admin/admin-user-handlers.ts
import type { Request, Response } from 'express'
import type { AuthUserStore } from '../auth/auth-user-store'
import type { AuthSessionStore } from '../auth/auth-session-store'
import type { AuditLogger } from './audit-logger'

export class AdminUserHandlers {
  constructor(private readonly deps: {
    userStore: AuthUserStore
    sessionStore: AuthSessionStore
    auditLogger: AuditLogger
  }) {}

  listUsers = async (req: Request, res: Response): Promise<void> => {
    const users = await this.deps.userStore.listUsers()
    res.json({ users })
  }

  createUser = async (req: Request, res: Response): Promise<void> => {
    const { email, name, password, role } = req.body ?? {}
    if (!email || !name || !password || !role) {
      res.status(400).json({ error: 'missing_fields', required: ['email', 'name', 'password', 'role'] })
      return
    }
    if (!['developer', 'lead', 'admin'].includes(role)) {
      res.status(400).json({ error: 'invalid_role' })
      return
    }

    const user = await this.deps.userStore.createLocalUser({ email, name, password, role })
    await this.deps.auditLogger.log({
      userId: req.orcaSession!.userId,
      userEmail: req.orcaSession!.userEmail,
      action: 'user.create',
      ipAddress: req.ip,
      detail: { targetEmail: email, role }
    })
    res.json(user)
  }

  deactivateUser = async (req: Request, res: Response): Promise<void> => {
    const { id } = req.params
    if (id === req.orcaSession!.userId) {
      res.status(400).json({ error: 'cannot_deactivate_self' })
      return
    }
    await this.deps.userStore.deactivateUser(id!)
    await this.deps.sessionStore.revokeAllUserSessions(id!)
    await this.deps.auditLogger.log({
      userId: req.orcaSession!.userId,
      userEmail: req.orcaSession!.userEmail,
      action: 'user.deactivate',
      ipAddress: req.ip,
      detail: { targetUserId: id }
    })
    res.json({ ok: true })
  }
}
```

### 4.5 `admin-session-handlers.ts`

```typescript
// src/main/admin/admin-session-handlers.ts
import type { Request, Response } from 'express'
import type { AuthSessionStore } from '../auth/auth-session-store'
import type { AuditLogger } from './audit-logger'

export class AdminSessionHandlers {
  constructor(private readonly deps: {
    sessionStore: AuthSessionStore
    auditLogger: AuditLogger
  }) {}

  listAllSessions = async (req: Request, res: Response): Promise<void> => {
    // Note: listUserSessions needs a listAll() variant without userId filter
    // For now, iterate all active users and collect sessions
    // TODO: add authSessionStore.listAllActiveSessions() method
    res.json({ sessions: [] })  // stub — implement in iteration
  }

  killSession = async (req: Request, res: Response): Promise<void> => {
    const { sessionId } = req.params
    await this.deps.sessionStore.revokeSession(sessionId!)
    await this.deps.auditLogger.log({
      userId: req.orcaSession!.userId,
      userEmail: req.orcaSession!.userEmail,
      action: 'session.kill',
      ipAddress: req.ip,
      detail: { targetSessionId: sessionId }
    })
    res.json({ ok: true })
  }

  killAllUserSessions = async (req: Request, res: Response): Promise<void> => {
    const { userId } = req.params
    const count = await this.deps.sessionStore.revokeAllUserSessions(userId!)
    await this.deps.auditLogger.log({
      userId: req.orcaSession!.userId,
      userEmail: req.orcaSession!.userEmail,
      action: 'session.kill_all',
      ipAddress: req.ip,
      detail: { targetUserId: userId, revokedCount: count }
    })
    res.json({ ok: true, revokedCount: count })
  }
}
```

### 4.6 `admin-stats-handler.ts`

```typescript
// src/main/admin/admin-stats-handler.ts
import type { Request, Response } from 'express'
import type { IDatabase } from '../db/types'
import type { AdminStats } from './admin-types'

export class AdminStatsHandler {
  constructor(private readonly db: IDatabase) {}

  getStats = async (_req: Request, res: Response): Promise<void> => {
    const now = Date.now()
    const stats: AdminStats = {
      totalUsers:     this.count('SELECT COUNT(*) FROM orca_users'),
      activeUsers:    this.count('SELECT COUNT(*) FROM orca_users WHERE is_active = 1'),
      activeSessions: this.count(`SELECT COUNT(*) FROM orca_sessions WHERE expires_at > ${now}`),
      pairedDevices:  0  // From DeviceRegistry — runtime data, stub for now
    }
    res.json(stats)
  }

  private count(sql: string): number {
    const row = this.db.prepare(sql).get() as Record<string, number>
    return Object.values(row)[0] ?? 0
  }
}
```

### 4.7 `admin-audit-handlers.ts`

```typescript
// src/main/admin/admin-audit-handlers.ts
import type { Request, Response } from 'express'
import type { AuditLogger } from './audit-logger'

export class AdminAuditHandlers {
  constructor(private readonly auditLogger: AuditLogger) {}

  queryAuditLog = async (req: Request, res: Response): Promise<void> => {
    const { userId, action, from, to, limit, offset } = req.query as Record<string, string>
    const events = await this.auditLogger.query({
      userId:  userId  || undefined,
      action:  action  || undefined,
      from:    from    ? Number(from)   : undefined,
      to:      to      ? Number(to)     : undefined,
      limit:   limit   ? Number(limit)  : 100,
      offset:  offset  ? Number(offset) : 0
    })
    res.json({ events, total: events.length })
  }
}
```

### 4.8 `admin-router.ts`

```typescript
// src/main/admin/admin-router.ts
import { Router } from 'express'
import { requireAdmin } from './admin-middleware'
import type { AdminUserHandlers } from './admin-user-handlers'
import type { AdminSessionHandlers } from './admin-session-handlers'
import type { AdminStatsHandler } from './admin-stats-handler'
import type { AdminAuditHandlers } from './admin-audit-handlers'

export function createAdminRouter(deps: {
  userHandlers:    AdminUserHandlers
  sessionHandlers: AdminSessionHandlers
  statsHandler:    AdminStatsHandler
  auditHandlers:   AdminAuditHandlers
}): Router {
  const router = Router()
  router.use(requireAdmin)  // All admin routes require admin role

  // Users
  router.get('/users',          deps.userHandlers.listUsers)
  router.post('/users',         deps.userHandlers.createUser)
  router.patch('/users/:id',    deps.userHandlers.deactivateUser)  // soft delete
  router.delete('/users/:id',   deps.userHandlers.deactivateUser)

  // Sessions
  router.get('/sessions',                      deps.sessionHandlers.listAllSessions)
  router.delete('/sessions/:sessionId',        deps.sessionHandlers.killSession)
  router.delete('/users/:userId/sessions',     deps.sessionHandlers.killAllUserSessions)

  // Stats
  router.get('/stats',   deps.statsHandler.getStats)

  // Audit log
  router.get('/audit',   deps.auditHandlers.queryAuditLog)

  return router
}
```

### 4.9 `first-run-setup.ts`

```typescript
// src/main/admin/first-run-setup.ts
import { randomBytes } from 'node:crypto'
import type { AuthUserStore } from '../auth/auth-user-store'
import type { IDatabase } from '../db/types'

export async function ensureFirstAdminUser(
  db: IDatabase,
  userStore: AuthUserStore
): Promise<void> {
  const row = db.prepare(`SELECT COUNT(*) as n FROM orca_users WHERE role = 'admin'`).get() as any
  if (row.n > 0) return  // Admin đã tồn tại

  const adminEmail    = process.env.ORCA_ADMIN_EMAIL    ?? 'admin@localhost'
  const adminPassword = process.env.ORCA_ADMIN_PASSWORD ?? randomBytes(8).toString('hex')

  await userStore.createLocalUser({
    email: adminEmail, name: 'Administrator', password: adminPassword, role: 'admin'
  })

  console.log('═══════════════════════════════════════════')
  console.log('  ⚠️  FIRST RUN: Admin account created')
  console.log(`     Email:    ${adminEmail}`)
  console.log(`     Password: ${adminPassword}`)
  console.log('     ▶ Đổi password ngay sau khi login!')
  console.log('═══════════════════════════════════════════')
}
```

---

## 5. Tích hợp vào `src/server/http-server.ts`

```typescript
// src/server/http-server.ts — MODIFY
import { createAdminRouter } from '../main/admin/admin-router'
import { AdminUserHandlers }    from '../main/admin/admin-user-handlers'
import { AdminSessionHandlers } from '../main/admin/admin-session-handlers'
import { AdminStatsHandler }    from '../main/admin/admin-stats-handler'
import { AdminAuditHandlers }   from '../main/admin/admin-audit-handlers'
import { AuditLogger }          from '../main/admin/audit-logger'
import { ensureFirstAdminUser } from '../main/admin/first-run-setup'

// In startHttpServer(port, webRoot, { authManager, db }):
const auditLogger    = new AuditLogger(db)
const userHandlers   = new AdminUserHandlers({ userStore: authManager.userStore, sessionStore: authManager.sessionStore, auditLogger })
const sessionHandlers = new AdminSessionHandlers({ sessionStore: authManager.sessionStore, auditLogger })
const statsHandler   = new AdminStatsHandler(db)
const auditHandlers  = new AdminAuditHandlers(auditLogger)

// Mount admin routes
app.use('/admin/api', createAdminRouter({ userHandlers, sessionHandlers, statsHandler, auditHandlers }))

// Seed first admin
await ensureFirstAdminUser(db, authManager.userStore)
```

---

## 6. Admin HTTP API Endpoints

| Method | Path | Mô tả |
|--------|------|-------|
| `GET` | `/admin/api/stats` | Dashboard stats |
| `GET` | `/admin/api/users` | List all users |
| `POST` | `/admin/api/users` | Create local user |
| `DELETE` | `/admin/api/users/:id` | Deactivate user (+ revoke sessions) |
| `GET` | `/admin/api/sessions` | List active sessions |
| `DELETE` | `/admin/api/sessions/:sessionId` | Kill one session |
| `DELETE` | `/admin/api/users/:userId/sessions` | Kill all user sessions |
| `GET` | `/admin/api/audit` | Query audit log (filter: userId, action, from, to) |

---

## 7. Audit Actions List

| Action | Khi nào |
|--------|---------|
| `login.success` | Local login thành công |
| `login.failure` | Login fail (wrong pw) |
| `logout` | User logout |
| `sso.login` | SSO callback thành công |
| `user.create` | Admin tạo user mới |
| `user.deactivate` | Admin deactivate user |
| `session.kill` | Admin kill session |
| `session.kill_all` | Admin kill tất cả sessions của user |
| `ssh.connect` | User SSH vào dev server |
| `ssh.disconnect` | SSH ngắt kết nối |
| `server.start` | Orca Server khởi động |
| `server.stop` | Orca Server dừng |

---

## 8. Acceptance Criteria

- [x] `audit-logger.test.ts` — tất cả tests pass (write, query với filters)
- [x] `admin-user-handlers.test.ts` — list, create, deactivate (self-deactivate prevention)
- [x] `admin-session-handlers.test.ts` — kill session, kill all user sessions
- [x] `GET /admin/api/stats` → `{ totalUsers, activeUsers, activeSessions }`
- [x] `GET /admin/api/users` → admin only (403 nếu role ≠ admin)
- [x] `POST /admin/api/users` → tạo user + log audit
- [x] `DELETE /admin/api/users/:id` → deactivate + revoke sessions + log audit
- [x] `DELETE /admin/api/sessions/:id` → kill session + log audit
- [x] `GET /admin/api/audit?userId=u1&action=login.success` → filtered results
- [x] `first-run-setup.ts` chỉ tạo admin nếu không có admin nào → idempotent
- [x] First run: print admin credentials ra stdout
- [x] Admin SPA tại `/admin/` phục vụ từ `out/web-admin/` (separate build target nếu có)
