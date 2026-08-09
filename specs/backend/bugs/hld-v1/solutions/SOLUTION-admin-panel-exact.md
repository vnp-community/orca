# SOLUTION: admin-panel — Code-Level Exact Fixes

**Source-verified:** ✅ Dựa trên source code thực tế (đọc trực tiếp qua CodeGraph + Read, không suy đoán)
**Files nguồn đã đọc:**
`backend/src/main/admin/admin-session-handlers.ts`, `backend/src/main/admin/admin-router.ts`,
`backend/src/main/admin/admin-audit-handlers.ts`, `backend/src/main/admin/admin-user-handlers.ts`,
`backend/src/main/admin/admin-types.ts`, `backend/src/main/admin/admin-middleware.ts`,
`backend/src/main/admin/admin-stats-handler.ts`, `backend/src/main/admin/audit-logger.ts`,
`backend/src/main/auth/auth-session-store.ts`, `backend/src/main/auth/auth-user-store.ts`,
`backend/src/main/auth/auth-types.ts`, `backend/src/main/db/types.ts`,
`backend/src/main/db/migrations/0005_add_auth_schema.ts`, `backend/src/server/http-server.ts`,
`backend/src/shared/rbac-types.ts`

**Bug tickets:** [BUG-BE-HLD-006](../BUG-BE-HLD-006-admin-sessions-list-stub-empty.md), [BUG-BE-HLD-007](../BUG-BE-HLD-007-admin-access-policies-api-missing.md)

---

## Bối cảnh kỹ thuật quan trọng (đọc trước khi áp fix)

Codebase **không** dùng `IConnectionPool.executeSync()/execute()` cho admin panel như TDD-BE-10 mô tả tổng quát.
Thực tế có 2 lớp trừu tượng DB tồn tại song song trong `backend/src/main/`:

1. **`IDatabase`/`ISyncDatabase`** (`backend/src/main/db/types.ts`) — lớp đang được **admin panel** và **auth** dùng thật:
   - `db.prepare(sql)` trả `IStatement | Promise<IStatement>` (sync cho SQLite, async cho MySQL/Postgres) → luôn `await`.
   - `IStatement.run()/get()/all()` — dùng để INSERT/UPDATE/DELETE/SELECT.
   - `AuthSessionStore`, `AuthUserStore` dùng `IDatabase` (luôn `await this.db.prepare(...)`, tương thích mọi dialect).
   - `AdminStatsHandler`, `backend/src/main/admin/audit-logger.ts` dùng `ISyncDatabase` trực tiếp (đồng bộ, không `await`) vì chưa có store riêng.
2. **`IConnectionPool`** (`withConnection()`) — chỉ dùng trong `backend/src/main/auth/audit-logger.ts` (một class `AuditLogger` **khác**, dùng cho login/logout audit trail, KHÔNG phải class `AuditLogger` mà admin panel dùng).

→ Khi fix 2 bug này, **bám theo lớp `IDatabase`/`ISyncDatabase` đang có trong `admin/` và `auth/`**, không dùng `IConnectionPool` (đó là dependency của một subsystem khác).

Cũng lưu ý: `AdminSessionHandlers` hiện tại inject `sessionStore: AuthSessionStore` (không có `db` trực tiếp) — nên cách sạch nhất để implement `listAllSessions()` là **thêm method mới vào `AuthSessionStore`** (giống cách `listUserSessions()` đã có), thay vì bơm thêm `ISyncDatabase` riêng vào `AdminSessionHandlers` và viết SQL trùng lặp trong handler. Cách này giữ đúng ranh giới DI hiện có (`admin-user-handlers.ts` cũng chỉ gọi `userStore.listUsers()`, không tự viết SQL).

---

## BUG-BE-HLD-006: `listAllSessions()` là stub rỗng

### File 1/2 — Thêm method mới vào `AuthSessionStore`

**File:** [`backend/src/main/auth/auth-session-store.ts`](file:///opt/repos/orca/backend/src/main/auth/auth-session-store.ts)
**Vị trí:** Thêm method mới ngay sau `listUserSessions()` (dòng 106), trước `cleanupExpired()`.

Code thật hiện tại của `listUserSessions()` (dòng 93–106) — copy đúng style JOIN + `rowToSession()` đã có, chỉ khác WHERE clause và thêm pagination:

```typescript
  /** List active (non-expired) sessions for a user. */
  async listUserSessions(userId: string): Promise<OrcaSession[]> {
    const stmt = await this.db.prepare(`
      SELECT
        s.session_id, s.user_id, s.created_at, s.expires_at, s.last_seen_at,
        s.ip_address, s.user_agent, u.email AS user_email, u.role
      FROM orca_sessions s
      JOIN orca_users u ON u.id = s.user_id
      WHERE s.user_id = ? AND s.expires_at > ?
      ORDER BY s.created_at DESC
    `)
    const rows = await stmt.all(userId, Date.now())
    return rows.map((r) => this.rowToSession(r))
  }
```

**Fix — thêm method `listAllActiveSessions()`:**

```typescript
  /**
   * List ALL active (non-expired) sessions across all users, joined with
   * orca_users for userEmail/role display in Admin Panel SessionsPage.
   * FIX BUG-BE-HLD-006: replaces the stub that always returned [].
   * Pagination mirrors admin-audit-handlers.ts (limit capped, default offset 0).
   */
  async listAllActiveSessions(limit: number, offset: number): Promise<{ sessions: OrcaSession[]; total: number }> {
    const now = Date.now()

    const countStmt = await this.db.prepare(
      `SELECT COUNT(*) AS n FROM orca_sessions WHERE expires_at > ?`
    )
    const countRow = await countStmt.get(now) as Record<string, unknown> | undefined
    const total = (countRow?.['n'] as number) ?? 0

    const stmt = await this.db.prepare(`
      SELECT
        s.session_id, s.user_id, s.created_at, s.expires_at, s.last_seen_at,
        s.ip_address, s.user_agent, u.email AS user_email, u.role
      FROM orca_sessions s
      JOIN orca_users u ON u.id = s.user_id
      WHERE s.expires_at > ?
      ORDER BY s.created_at DESC
      LIMIT ? OFFSET ?
    `)
    const rows = await stmt.all(now, limit, offset)

    return { sessions: rows.map((r) => this.rowToSession(r)), total }
  }
```

`rowToSession()` (đã có sẵn, không đổi — dòng 115–127) đã map đúng `user_email`/`role` từ JOIN, nên tái dùng nguyên vẹn.

---

### File 2/2 — `admin-session-handlers.ts`: thay stub bằng gọi store thật

**File:** [`backend/src/main/admin/admin-session-handlers.ts`](file:///opt/repos/orca/backend/src/main/admin/admin-session-handlers.ts)
**Lines:** 18–22

### Code sai thực tế:
```typescript
  /** List all active sessions — stub (full listing requires additional store method) */
  listAllSessions = (_req: Request, res: Response): void => {
    // TODO: implement AuthSessionStore.listAllActiveSessions() in a future iteration
    res.json({ sessions: [], total: 0, note: 'Full listing not yet implemented' })
  }
```

### Fix:
```typescript
  /** List all active (non-expired) sessions across all users. Supports ?limit=&offset= pagination. */
  listAllSessions = async (req: Request, res: Response): Promise<void> => {
    const q = req.query as Record<string, string>
    // FIX BUG-BE-HLD-006: pagination pattern copied from admin-audit-handlers.ts (cap limit at 1000)
    const limit  = q['limit']  ? Math.min(Number(q['limit']), 1000) : 100
    const offset = q['offset'] ? Number(q['offset']) : 0

    const { sessions, total } = await this.deps.sessionStore.listAllActiveSessions(limit, offset)
    res.json({ sessions, total })
  }
```

Không cần đổi import — `AuthSessionStore` đã được inject qua `deps.sessionStore` (dòng 13–16 của file, không đổi). `killSession`/`killAllUserSessions` giữ nguyên.

**Lưu ý wiring:** `admin-router.ts` gọi `router.get('/sessions', deps.sessionHandlers.listAllSessions)` — do handler này đổi từ sync sang `async`, Express tự động xử lý (route handler trả `Promise<void>` là hợp lệ, đúng pattern `listUsers`/`createUser` trong `admin-user-handlers.ts` đã dùng). Không cần đổi `admin-router.ts` cho bug này.

**Response shape mới:** `{ sessions: OrcaSession[], total: number }` — bỏ field `note` (client hiện tại parse `sessions`/`total`, field `note` chỉ tồn tại vì đây là stub nên loại bỏ an toàn).

### Test cần viết (theo đề xuất trong ticket)
`backend/src/main/admin/admin-session-handlers.test.ts` (mới hoặc bổ sung nếu đã tồn tại):
```typescript
it('trả về danh sách session thật khi DB có ≥2 session active', async () => {
  // seed: tạo 2 user + 2 session (expires_at trong tương lai) qua AuthUserStore/AuthSessionStore thật
  // GET /admin/api/sessions (hoặc gọi handler.listAllSessions trực tiếp với req/res mock)
  // assert: res.json được gọi với { sessions: [...2 items], total: 2 }
  // assert: mỗi session có userEmail đúng (từ JOIN orca_users)
})

it('không trả về session đã expired', async () => {
  // seed 1 session active + 1 session expires_at < now()
  // assert total === 1, chỉ session active xuất hiện
})

it('áp dụng limit/offset đúng', async () => {
  // seed 3 session, gọi với ?limit=2&offset=1
  // assert trả về đúng 2 session (item thứ 2 và 3 theo created_at DESC), total vẫn = 3
})
```

---

## BUG-BE-HLD-007: Toàn bộ backend API cho Access Policies không tồn tại

### Bước 1 — Thêm audit action constants vào `admin-types.ts`

**File:** [`backend/src/main/admin/admin-types.ts`](file:///opt/repos/orca/backend/src/main/admin/admin-types.ts)
**Vị trí:** Thêm 3 dòng vào object `AUDIT_ACTIONS` (dòng 59–72), ngay sau `SESSION_KILL_ALL`.

Code thật hiện tại:
```typescript
export const AUDIT_ACTIONS = {
  LOGIN_SUCCESS:    'login.success',
  LOGIN_FAILURE:    'login.failure',
  LOGOUT:           'logout',
  SSO_LOGIN:        'sso.login',
  USER_CREATE:      'user.create',
  USER_DEACTIVATE:  'user.deactivate',
  SESSION_KILL:     'session.kill',
  SESSION_KILL_ALL: 'session.kill_all',
  SSH_CONNECT:      'ssh.connect',
  SSH_DISCONNECT:   'ssh.disconnect',
  SERVER_START:     'server.start',
  SERVER_STOP:      'server.stop',
} as const
```

Fix — thêm (không xoá gì, chỉ chèn thêm 3 field trước dòng đóng `} as const`):
```typescript
export const AUDIT_ACTIONS = {
  LOGIN_SUCCESS:    'login.success',
  LOGIN_FAILURE:    'login.failure',
  LOGOUT:           'logout',
  SSO_LOGIN:        'sso.login',
  USER_CREATE:      'user.create',
  USER_DEACTIVATE:  'user.deactivate',
  SESSION_KILL:     'session.kill',
  SESSION_KILL_ALL: 'session.kill_all',
  SSH_CONNECT:      'ssh.connect',
  SSH_DISCONNECT:   'ssh.disconnect',
  SERVER_START:     'server.start',
  SERVER_STOP:      'server.stop',
  // FIX BUG-BE-HLD-007: audit trail cho Access Policy CRUD
  POLICY_CREATE:    'policy.create',
  POLICY_UPDATE:    'policy.update',
  POLICY_DELETE:    'policy.delete',
} as const
```

`PolicyInput` type (dòng 41–52) đã tồn tại đúng như audit mô tả — **không cần đổi**, dùng nguyên trạng làm request body type cho `createPolicy`/`updatePolicy`.

`OrcaAccessPolicy` (dùng bởi `resolveUserPermissions()`) đã định nghĩa ở `backend/src/shared/rbac-types.ts:32-50` — response của các handler dưới đây trả về object đúng shape này (cộng thêm `createdAt`/`updatedAt`), để bất kỳ call site nào sau này (BUG-BE-HLD-003) có thể lấy list qua `listPolicies()` rồi truyền thẳng vào `resolveUserPermissions(user, policies)` mà không cần convert thêm.

---

### Bước 2 — File MỚI: `admin-policy-handlers.ts`

**File:** `backend/src/main/admin/admin-policy-handlers.ts` (chưa tồn tại — tạo mới, đặt cạnh `admin-user-handlers.ts`)

Bám đúng pattern DI + response shape của `admin-user-handlers.ts` (constructor nhận `deps` object, mỗi route = 1 arrow-function field để giữ đúng `this` khi Express gọi không bind). Vì chưa có store riêng cho `orca_access_policies`, dùng trực tiếp `ISyncDatabase` như `AdminStatsHandler`/`admin/audit-logger.ts` đã làm (xem "Bối cảnh kỹ thuật" ở đầu file).

```typescript
/**
 * Admin Policy Handlers — CRUD cho RBAC Access Policies (orca_access_policies)
 *
 * Handles: list, create, update, delete access policies.
 * Dùng bởi PoliciesPage trong Admin SPA để quản lý OrcaAccessPolicy (rbac-types.ts).
 * Tất cả route yêu cầu requireAdmin middleware (áp dụng qua router.use() trong admin-router.ts).
 *
 * @module main/admin/admin-policy-handlers
 */

import { randomUUID } from 'node:crypto'
import type { Request, Response } from 'express'
import type { ISyncDatabase } from '../db/types'
import type { AuditLogger } from './audit-logger'
import type { PolicyInput } from './admin-types'
import { AUDIT_ACTIONS } from './admin-types'

const VALID_AGENT_TRUST = ['minimal', 'standard', 'full'] as const

/** Shape trả về cho client — tương thích OrcaAccessPolicy (backend/src/shared/rbac-types.ts) + audit timestamps. */
type PolicyResponse = {
  id:                   string
  name:                 string
  teams:                string[]
  roles:                string[]
  users:                string[]
  allowedServers:       '*' | string[]
  allowedProjects:      '*' | string[]
  agentTrust:           'minimal' | 'standard' | 'full'
  canCreateWorktrees:   boolean
  canDeleteWorktrees:   boolean
  canAccessProduction:  boolean
  createdAt:            number
  updatedAt:            number
}

export class AdminPolicyHandlers {
  constructor(private readonly deps: {
    db:          ISyncDatabase
    auditLogger: AuditLogger
  }) {}

  /** GET /admin/api/policies */
  listPolicies = (_req: Request, res: Response): void => {
    const rows = this.deps.db.prepare(`
      SELECT * FROM orca_access_policies
      ORDER BY created_at DESC
    `).all() as Record<string, unknown>[]

    const policies = rows.map((r) => this.rowToPolicy(r))
    res.json({ policies, total: policies.length })
  }

  /** POST /admin/api/policies — Body: PolicyInput */
  createPolicy = (req: Request, res: Response): void => {
    const input = (req.body ?? {}) as PolicyInput

    if (!input.name || typeof input.name !== 'string') {
      res.status(400).json({ error: 'missing_fields', required: ['name'] })
      return
    }
    if (input.agentTrust && !(VALID_AGENT_TRUST as readonly string[]).includes(input.agentTrust)) {
      res.status(400).json({ error: 'invalid_agent_trust', allowed: VALID_AGENT_TRUST })
      return
    }

    const id  = randomUUID()
    const now = Date.now()

    try {
      this.deps.db.prepare(`
        INSERT INTO orca_access_policies
          (id, name, teams, roles, users, allowed_servers, allowed_projects,
           agent_trust, can_create_worktrees, can_delete_worktrees, can_access_production,
           created_at, updated_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
      `).run(
        id,
        input.name,
        JSON.stringify(input.teams ?? []),
        JSON.stringify(input.roles ?? []),
        JSON.stringify(input.users ?? []),
        JSON.stringify(input.allowedServers ?? '*'),
        JSON.stringify(input.allowedProjects ?? '*'),
        input.agentTrust ?? 'standard',
        input.canCreateWorktrees === false ? 0 : 1,
        input.canDeleteWorktrees === false ? 0 : 1,
        input.canAccessProduction === true ? 1 : 0,
        now,
        now
      )
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : 'Unknown error'
      res.status(500).json({ error: 'internal_error', message })
      return
    }

    this.deps.auditLogger.log({
      userId:    req.orcaSession!.userId,
      userEmail: req.orcaSession!.userEmail,
      action:    AUDIT_ACTIONS.POLICY_CREATE,
      ipAddress: req.ip,
      detail:    { policyId: id, name: input.name }
    })

    const row = this.deps.db.prepare(`SELECT * FROM orca_access_policies WHERE id = ?`).get(id) as Record<string, unknown>
    res.status(201).json(this.rowToPolicy(row))
  }

  /** PUT /admin/api/policies/:id — Body: Partial<PolicyInput> */
  updatePolicy = (req: Request, res: Response): void => {
    const { id } = req.params
    const input = (req.body ?? {}) as Partial<PolicyInput>

    const existing = this.deps.db.prepare(`SELECT * FROM orca_access_policies WHERE id = ?`).get(id) as Record<string, unknown> | undefined
    if (!existing) {
      res.status(404).json({ error: 'not_found' })
      return
    }
    if (input.agentTrust && !(VALID_AGENT_TRUST as readonly string[]).includes(input.agentTrust)) {
      res.status(400).json({ error: 'invalid_agent_trust', allowed: VALID_AGENT_TRUST })
      return
    }

    // Partial update — chỉ merge field nào có trong body, giữ nguyên phần còn lại
    const current = this.rowToPolicy(existing)
    const merged: PolicyResponse = {
      ...current,
      name:                input.name                ?? current.name,
      teams:               input.teams                ?? current.teams,
      roles:               input.roles                ?? current.roles,
      users:               input.users                ?? current.users,
      allowedServers:      input.allowedServers        ?? current.allowedServers,
      allowedProjects:     input.allowedProjects       ?? current.allowedProjects,
      agentTrust:          input.agentTrust            ?? current.agentTrust,
      canCreateWorktrees:  input.canCreateWorktrees   ?? current.canCreateWorktrees,
      canDeleteWorktrees:  input.canDeleteWorktrees   ?? current.canDeleteWorktrees,
      canAccessProduction: input.canAccessProduction  ?? current.canAccessProduction,
      updatedAt:           Date.now()
    }

    this.deps.db.prepare(`
      UPDATE orca_access_policies
      SET name = ?, teams = ?, roles = ?, users = ?, allowed_servers = ?, allowed_projects = ?,
          agent_trust = ?, can_create_worktrees = ?, can_delete_worktrees = ?, can_access_production = ?,
          updated_at = ?
      WHERE id = ?
    `).run(
      merged.name,
      JSON.stringify(merged.teams),
      JSON.stringify(merged.roles),
      JSON.stringify(merged.users),
      JSON.stringify(merged.allowedServers),
      JSON.stringify(merged.allowedProjects),
      merged.agentTrust,
      merged.canCreateWorktrees ? 1 : 0,
      merged.canDeleteWorktrees ? 1 : 0,
      merged.canAccessProduction ? 1 : 0,
      merged.updatedAt,
      id
    )

    this.deps.auditLogger.log({
      userId:    req.orcaSession!.userId,
      userEmail: req.orcaSession!.userEmail,
      action:    AUDIT_ACTIONS.POLICY_UPDATE,
      ipAddress: req.ip,
      detail:    { policyId: id, changes: Object.keys(input) }
    })

    res.json(merged)
  }

  /** DELETE /admin/api/policies/:id */
  deletePolicy = (req: Request, res: Response): void => {
    const { id } = req.params

    const existing = this.deps.db.prepare(`SELECT id, name FROM orca_access_policies WHERE id = ?`).get(id) as Record<string, unknown> | undefined
    if (!existing) {
      res.status(404).json({ error: 'not_found' })
      return
    }

    this.deps.db.prepare(`DELETE FROM orca_access_policies WHERE id = ?`).run(id)

    this.deps.auditLogger.log({
      userId:    req.orcaSession!.userId,
      userEmail: req.orcaSession!.userEmail,
      action:    AUDIT_ACTIONS.POLICY_DELETE,
      ipAddress: req.ip,
      detail:    { policyId: id, name: existing['name'] }
    })

    res.json({ ok: true })
  }

  /** Map raw DB row → PolicyResponse (decode JSON columns, coerce INTEGER → boolean). */
  private rowToPolicy(row: Record<string, unknown>): PolicyResponse {
    return {
      id:                  row['id']   as string,
      name:                row['name'] as string,
      teams:               JSON.parse(row['teams']            as string) as string[],
      roles:               JSON.parse(row['roles']             as string) as string[],
      users:               JSON.parse(row['users']             as string) as string[],
      allowedServers:      JSON.parse(row['allowed_servers']   as string) as '*' | string[],
      allowedProjects:     JSON.parse(row['allowed_projects']  as string) as '*' | string[],
      agentTrust:          row['agent_trust'] as PolicyResponse['agentTrust'],
      canCreateWorktrees:  Boolean(row['can_create_worktrees']),
      canDeleteWorktrees:  Boolean(row['can_delete_worktrees']),
      canAccessProduction: Boolean(row['can_access_production']),
      createdAt:           row['created_at'] as number,
      updatedAt:           row['updated_at'] as number
    }
  }
}
```

**Vì sao `allowed_servers`/`allowed_projects` cần `JSON.parse`/`JSON.stringify` (không phải raw string):**
migration `0005_add_auth_schema.ts` khai báo default cột là `'"*"'` (chuỗi JSON hợp lệ chứa giá trị `"*"`, dòng 89–90) — nghĩa là cột này **luôn** lưu JSON-encoded (`'"*"'` cho ký tự đơn `*`, hoặc `'["srv-1","srv-2"]'` cho mảng), khớp với type `OrcaAccessPolicy['allowedServers'] = '*' | string[]`. Lưu raw string `*` (không encode) sẽ làm `JSON.parse` throw khi đọc lại.

---

### Bước 3 — Mount route trong `admin-router.ts`

**File:** [`backend/src/main/admin/admin-router.ts`](file:///opt/repos/orca/backend/src/main/admin/admin-router.ts)

Code thật hiện tại (toàn bộ file, 45 dòng):
```typescript
import { Router } from 'express'
import { requireAdmin } from './admin-middleware'
import type { AdminUserHandlers }    from './admin-user-handlers'
import type { AdminSessionHandlers } from './admin-session-handlers'
import type { AdminStatsHandler }    from './admin-stats-handler'
import type { AdminAuditHandlers }   from './admin-audit-handlers'

export function createAdminRouter(deps: {
  userHandlers:    AdminUserHandlers
  sessionHandlers: AdminSessionHandlers
  statsHandler:    AdminStatsHandler
  auditHandlers:   AdminAuditHandlers
}): Router {
  const router = Router()

  // Guard ALL admin routes — must come before any route handlers
  router.use(requireAdmin)

  // ── Stats Dashboard ──────────────────────────────────────────────
  router.get('/stats', deps.statsHandler.getStats)

  // ── Users ────────────────────────────────────────────────────────
  router.get('/users',        deps.userHandlers.listUsers)
  router.post('/users',       deps.userHandlers.createUser)
  router.delete('/users/:id', deps.userHandlers.deactivateUser)

  // ── Sessions ─────────────────────────────────────────────────────
  router.get('/sessions',                       deps.sessionHandlers.listAllSessions)
  router.delete('/sessions/:sessionId',         deps.sessionHandlers.killSession)
  router.delete('/users/:userId/sessions',      deps.sessionHandlers.killAllUserSessions)

  // ── Audit Log ────────────────────────────────────────────────────
  router.get('/audit', deps.auditHandlers.queryAuditLog)

  return router
}
```

### Fix — thêm import + field deps + block route mới (KHÔNG xoá gì, chỉ chèn thêm):
```typescript
import { Router } from 'express'
import { requireAdmin } from './admin-middleware'
import type { AdminUserHandlers }    from './admin-user-handlers'
import type { AdminSessionHandlers } from './admin-session-handlers'
import type { AdminStatsHandler }    from './admin-stats-handler'
import type { AdminAuditHandlers }   from './admin-audit-handlers'
import type { AdminPolicyHandlers }  from './admin-policy-handlers'   // FIX BUG-BE-HLD-007

export function createAdminRouter(deps: {
  userHandlers:    AdminUserHandlers
  sessionHandlers: AdminSessionHandlers
  statsHandler:    AdminStatsHandler
  auditHandlers:   AdminAuditHandlers
  policyHandlers:  AdminPolicyHandlers   // FIX BUG-BE-HLD-007
}): Router {
  const router = Router()

  // Guard ALL admin routes — must come before any route handlers
  router.use(requireAdmin)

  // ── Stats Dashboard ──────────────────────────────────────────────
  router.get('/stats', deps.statsHandler.getStats)

  // ── Users ────────────────────────────────────────────────────────
  router.get('/users',        deps.userHandlers.listUsers)
  router.post('/users',       deps.userHandlers.createUser)
  router.delete('/users/:id', deps.userHandlers.deactivateUser)

  // ── Sessions ─────────────────────────────────────────────────────
  router.get('/sessions',                       deps.sessionHandlers.listAllSessions)
  router.delete('/sessions/:sessionId',         deps.sessionHandlers.killSession)
  router.delete('/users/:userId/sessions',      deps.sessionHandlers.killAllUserSessions)

  // ── Audit Log ────────────────────────────────────────────────────
  router.get('/audit', deps.auditHandlers.queryAuditLog)

  // ── Access Policies (RBAC) ──────────────────────────────────────── FIX BUG-BE-HLD-007
  router.get('/policies',         deps.policyHandlers.listPolicies)
  router.post('/policies',        deps.policyHandlers.createPolicy)
  router.put('/policies/:id',     deps.policyHandlers.updatePolicy)
  router.delete('/policies/:id',  deps.policyHandlers.deletePolicy)

  return router
}
```

**`requireAdmin` guard:** đã áp dụng cho TOÀN BỘ router qua `router.use(requireAdmin)` (dòng ngay sau `const router = Router()`) — các route `/policies*` mới thêm nằm sau dòng này nên **tự động được guard**, không cần thêm middleware riêng lẻ cho từng route (đúng cách 4 nhóm route hiện có — stats/users/sessions/audit — đang hoạt động).

---

### Bước 4 — Wiring: instantiate `AdminPolicyHandlers` trong `http-server.ts`

Route đã mount nhưng chưa có gì gọi `createAdminRouter({..., policyHandlers: ...})` — nếu bỏ qua bước này, TypeScript sẽ báo lỗi thiếu field bắt buộc `policyHandlers`. Đây là điểm nối bắt buộc để 2 bug thực sự fix xong (không chỉ file tồn tại mà còn phải chạy được).

**File:** [`backend/src/server/http-server.ts`](file:///opt/repos/orca/backend/src/server/http-server.ts)
**Lines liên quan:** import block (dòng 23–28) và `createAdminRouter({...})` call (dòng 99–111).

Code thật hiện tại:
```typescript
import { createAdminRouter }    from '../main/admin/admin-router'
import { AdminUserHandlers }    from '../main/admin/admin-user-handlers'
import { AdminSessionHandlers } from '../main/admin/admin-session-handlers'
import { AdminStatsHandler }    from '../main/admin/admin-stats-handler'
import { AdminAuditHandlers }   from '../main/admin/admin-audit-handlers'
import { AuditLogger }          from '../main/admin/audit-logger'
```
```typescript
      const auditLogger  = new AuditLogger(adminDb)
      const adminRouter  = createAdminRouter({
        userHandlers:    new AdminUserHandlers({
          userStore:    options.authManager.userStore,
          sessionStore: options.authManager.sessionStore,
          auditLogger
        }),
        sessionHandlers: new AdminSessionHandlers({
          sessionStore: options.authManager.sessionStore,
          auditLogger
        }),
        statsHandler:  new AdminStatsHandler(adminDb),
        auditHandlers: new AdminAuditHandlers(auditLogger)
      })
```

### Fix:
```typescript
import { createAdminRouter }    from '../main/admin/admin-router'
import { AdminUserHandlers }    from '../main/admin/admin-user-handlers'
import { AdminSessionHandlers } from '../main/admin/admin-session-handlers'
import { AdminStatsHandler }    from '../main/admin/admin-stats-handler'
import { AdminAuditHandlers }   from '../main/admin/admin-audit-handlers'
import { AdminPolicyHandlers }  from '../main/admin/admin-policy-handlers'   // FIX BUG-BE-HLD-007
import { AuditLogger }          from '../main/admin/audit-logger'
```
```typescript
      const auditLogger  = new AuditLogger(adminDb)
      const adminRouter  = createAdminRouter({
        userHandlers:    new AdminUserHandlers({
          userStore:    options.authManager.userStore,
          sessionStore: options.authManager.sessionStore,
          auditLogger
        }),
        sessionHandlers: new AdminSessionHandlers({
          sessionStore: options.authManager.sessionStore,
          auditLogger
        }),
        statsHandler:  new AdminStatsHandler(adminDb),
        auditHandlers: new AdminAuditHandlers(auditLogger),
        // FIX BUG-BE-HLD-007: adminDb đã là ISyncDatabase — cùng instance AdminStatsHandler đang dùng
        policyHandlers: new AdminPolicyHandlers({ db: adminDb, auditLogger })
      })
```

Không cần thay đổi gì khác trong `http-server.ts` — `adminDb` (kiểu `ISyncDatabase`, dòng 96) đã sẵn có trong scope này.

---

### Bước 5 — Kết nối với `resolveUserPermissions()` (điểm 3 trong đề xuất fix của ticket)

`resolveUserPermissions(user: OrcaUser, policies: OrcaAccessPolicy[])` (`backend/src/shared/rbac-types.ts:73-119`) là hàm **thuần túy** (pure function) — nhận sẵn mảng policy, không tự đọc DB. `rowToPolicy()` ở trên trả về object đúng shape `OrcaAccessPolicy` (id, name, teams, roles, users, allowedServers, allowedProjects, agentTrust, canCreateWorktrees, canDeleteWorktrees, canAccessProduction), nên **round-trip CRUD → effect RBAC thật** giờ khả thi:

```typescript
// Ví dụ call site tương lai (SSH connect gate, KHÔNG thuộc phạm vi bug này — xem BUG-BE-HLD-003):
const rows     = db.prepare('SELECT * FROM orca_access_policies').all()
const policies = rows.map(rowToPolicy) as OrcaAccessPolicy[]   // shape khớp resolveUserPermissions()
const perms    = resolveUserPermissions(currentUser, policies)
```

Việc **enforce** thật sự (áp permissions vào luồng SSH connect/provision) là phạm vi của **BUG-BE-HLD-003** (RBAC fragmented) — doc gốc `F25-admin-panel.md` cũng note rõ phần enforcement SSH là "DEFERRED Phase 3". Fix trong solution này chỉ đảm bảo **CRUD hoạt động và dữ liệu ghi ra đúng shape** để BUG-BE-HLD-003 có nguồn dữ liệu thật để dùng — không tự ý mở rộng sang enforcement (ngoài phạm vi 2 ticket được giao).

### Test cần viết
`backend/src/main/admin/admin-policy-handlers.test.ts` (mới):
```typescript
describe('AdminPolicyHandlers', () => {
  it('createPolicy: tạo policy mới, trả 201 + shape OrcaAccessPolicy', ...)
  it('createPolicy: 400 khi thiếu name', ...)
  it('createPolicy: 400 khi agentTrust không hợp lệ', ...)
  it('listPolicies: trả về policies đã tạo, ORDER BY created_at DESC', ...)
  it('updatePolicy: merge partial body, giữ nguyên field không gửi', ...)
  it('updatePolicy: 404 khi id không tồn tại', ...)
  it('deletePolicy: xoá thành công, trả { ok: true }', ...)
  it('deletePolicy: 404 khi id không tồn tại', ...)
  it('mọi mutation (create/update/delete) đều ghi 1 dòng orca_audit_log đúng action', ...)
  it('allowedServers/allowedProjects roundtrip đúng qua JSON.stringify/parse (cả "*" và mảng)', ...)
})
```

---

## Tóm tắt thay đổi

| Bug | File | Thay đổi |
|-----|------|---------|
| BE-HLD-006 | `backend/src/main/auth/auth-session-store.ts` | Thêm method `listAllActiveSessions(limit, offset)` |
| BE-HLD-006 | `backend/src/main/admin/admin-session-handlers.ts` | `listAllSessions`: stub rỗng → `async`, gọi `sessionStore.listAllActiveSessions()` + pagination từ query string |
| BE-HLD-007 | `backend/src/main/admin/admin-types.ts` | Thêm `POLICY_CREATE`/`POLICY_UPDATE`/`POLICY_DELETE` vào `AUDIT_ACTIONS` |
| BE-HLD-007 | `backend/src/main/admin/admin-policy-handlers.ts` | **File mới** — `AdminPolicyHandlers` (list/create/update/delete cho `orca_access_policies`) |
| BE-HLD-007 | `backend/src/main/admin/admin-router.ts` | Thêm import `AdminPolicyHandlers`, field `policyHandlers` trong `deps`, 4 route `/policies*` (tự động guard qua `router.use(requireAdmin)` đã có) |
| BE-HLD-007 | `backend/src/server/http-server.ts` | Wiring: import + `new AdminPolicyHandlers({ db: adminDb, auditLogger })` trong `createAdminRouter({...})` |
