# TASK-HLD-011: Tạo `AdminPolicyHandlers` (CRUD Access Policies) và mount route

**Priority:** 🔴 CRITICAL — Toàn bộ backend API cho Access Policies không tồn tại
**Effort:** ~45 phút
**Status:** ✅ DONE — 2026-08-09 (file mới `admin-policy-handlers.ts` tạo đúng theo solution; mount 4 route trong `admin-router.ts`. Phải sửa thêm 5 chỗ `string|string[]` type mismatch không có trong solution gốc — do `req.params` trong codebase này type rộng hơn dự kiến (dùng `String(id)` cast, nhất quán với cách khác trong repo) và `PolicyInput.allowedServers: string|string[]` khác `PolicyResponse.allowedServers: '*'|string[]` (widen-back cast). Còn đúng 3 lỗi `POLICY_CREATE/UPDATE/DELETE` — theo đúng dự kiến, chờ TASK-HLD-012.)
**Bug refs:** BUG-BE-HLD-007
**Solution ref:** [SOLUTION-admin-panel-exact.md](../solutions/SOLUTION-admin-panel-exact.md)
**Depends on:** (none)

---

## Mục tiêu

Tạo file mới `backend/src/main/admin/admin-policy-handlers.ts` chứa class `AdminPolicyHandlers` với đầy đủ list/create/update/delete cho bảng `orca_access_policies`, và mount 4 route `/admin/api/policies*` trong `admin-router.ts`. Đây là phần API còn thiếu hoàn toàn khiến trang PoliciesPage trong Admin SPA không có backend để gọi.

Lưu ý bối cảnh kỹ thuật: codebase dùng lớp `IDatabase`/`ISyncDatabase` (`backend/src/main/db/types.ts`), **không** dùng `IConnectionPool.executeSync()/execute()`. Vì chưa có store riêng cho `orca_access_policies`, `AdminPolicyHandlers` dùng trực tiếp `ISyncDatabase` (đồng bộ, không `await`) — đúng cách `AdminStatsHandler` và `backend/src/main/admin/audit-logger.ts` đã làm.

## File cần sửa/tạo

- `backend/src/main/admin/admin-policy-handlers.ts` — **file mới**
- `backend/src/main/admin/admin-router.ts` — thêm import `AdminPolicyHandlers`, field `policyHandlers` trong `deps`, 4 route `/policies*`

## Thay đổi cụ thể

### 1. File mới: `backend/src/main/admin/admin-policy-handlers.ts`

Bám đúng pattern DI + response shape của `admin-user-handlers.ts` (constructor nhận `deps` object, mỗi route là 1 arrow-function field để giữ đúng `this` khi Express gọi không bind).

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

`PolicyInput` type (`admin-types.ts` dòng 41–52) đã tồn tại đúng shape cần — **không cần đổi**, dùng nguyên trạng làm request body type cho `createPolicy`/`updatePolicy`. `OrcaAccessPolicy` (`backend/src/shared/rbac-types.ts:32-50`) là shape mà `rowToPolicy()` phải khớp — response của các handler trên trả về object đúng shape này (cộng thêm `createdAt`/`updatedAt`).

Lưu ý: task này **không** thêm 3 hằng số `AUDIT_ACTIONS.POLICY_CREATE/UPDATE/DELETE` mà file trên import — việc đó và việc instantiate `AdminPolicyHandlers` trong `http-server.ts` thuộc TASK-HLD-012 (phụ thuộc bởi task đó). Nếu build task này độc lập, TypeScript sẽ báo lỗi thiếu `AUDIT_ACTIONS.POLICY_*` cho tới khi TASK-HLD-012 áp dụng.

### 2. `backend/src/main/admin/admin-router.ts`

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

Fix — thêm import + field deps + block route mới (KHÔNG xoá gì, chỉ chèn thêm):

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

**Lưu ý:** Sau task này, `createAdminRouter({...})` yêu cầu field `policyHandlers` bắt buộc trong `deps` — mọi call site (hiện chỉ có `http-server.ts`) sẽ báo lỗi TypeScript thiếu field cho tới khi TASK-HLD-012 wiring xong. Đây là phụ thuộc bắt buộc, xem TASK-HLD-012.

## Verification

```bash
# File mới tồn tại và export đúng class
grep -n "export class AdminPolicyHandlers" backend/src/main/admin/admin-policy-handlers.ts

# Route đã mount
grep -n "policies" backend/src/main/admin/admin-router.ts

# tsc sẽ báo lỗi thiếu field policyHandlers trong http-server.ts + thiếu AUDIT_ACTIONS.POLICY_*
# cho tới khi TASK-HLD-012 hoàn thành — đây là kỳ vọng đúng, không phải lỗi của task này.
pnpm tsc --noEmit || true
```

Test cần viết (mới) `backend/src/main/admin/admin-policy-handlers.test.ts`:

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
