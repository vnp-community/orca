# T05 — Write TaskGrantService.test.ts

**Phase:** 2A  
**Effort:** ~1 hour  
**Depends on:** T04 (TaskService tests — same DB setup pattern)  
**Solution ref:** [05-tdd18-task-graph.md §2.2](../solutions/05-tdd18-task-graph.md)  
**TDD ref:** TDD-18 §4 (TaskGrantService)

---

## Mục tiêu

Viết test file cho `TaskGrantService` — BFS ancestor grant resolution, cascade, team/company grants.

**Target: ≥ 12 tests**

---

## Files Cần Đọc Trước

1. `src/main/task/TaskGrantService.ts` — đọc toàn bộ
2. `src/main/task/TaskService.ts` — constructor để inject vào GrantService
3. `src/main/db/migrations/0010_tasks.ts` — xem schema orca_task_grants, orca_team_members

---

## File Cần Tạo

### `src/main/task/__tests__/TaskGrantService.test.ts`

```typescript
/**
 * Tests for TaskGrantService (TDD-18) — T05
 *
 * Uses in-memory SQLite + ALL_MIGRATIONS.
 * Tests BFS ancestor grant resolution, cascade, scope matching.
 */

import { describe, it, expect, beforeEach } from 'vitest'
import { SqliteSingleConnectionPool } from '../../db/sqlite/sqlite-pool'
import { MigrationRunner } from '../../db/migrations/runner'
import { ALL_MIGRATIONS } from '../../db/migrations'
import { TaskService } from '../TaskService'
import { TaskDAGValidator } from '../TaskDAGValidator'
import { TaskGrantService } from '../TaskGrantService'
import type { OrcaTask } from '../../../shared/task-types'

// ── Helpers ──────────────────────────────────────────────────────────────────

async function makeServices() {
  const pool = new SqliteSingleConnectionPool(':memory:')
  await pool.withConnection(async (db) => {
    const runner = new MigrationRunner(db, ALL_MIGRATIONS)
    await runner.migrate()
  })
  const validator = new TaskDAGValidator(pool)
  const taskService = new TaskService(pool, validator)
  const grantService = new TaskGrantService(pool, taskService)
  return { pool, taskService, grantService }
}

async function insertUser(pool: SqliteSingleConnectionPool, userId: string): Promise<void> {
  await pool.withConnection((db) =>
    db.query(
      'INSERT INTO orca_users (id, email, name, role, provider, created_at) VALUES (?, ?, ?, ?, ?, ?)',
      [userId, `${userId}@test.com`, userId, 'developer', 'none', Date.now()]
    )
  )
}

async function insertTeamMember(pool: SqliteSingleConnectionPool, userId: string, teamId: string): Promise<void> {
  await pool.withConnection((db) =>
    db.query(
      'INSERT INTO orca_team_members (user_id, team_id) VALUES (?, ?)',
      [userId, teamId]
    )
  )
}

// ── Tests ─────────────────────────────────────────────────────────────────────

describe('TaskGrantService', () => {
  let pool: SqliteSingleConnectionPool
  let taskService: TaskService
  let grantService: TaskGrantService

  beforeEach(async () => {
    ;({ pool, taskService, grantService } = await makeServices())
    await insertUser(pool, 'reporter-001')
    await insertUser(pool, 'assignee-001')
    await insertUser(pool, 'user-001')
    await insertUser(pool, 'user-002')
  })

  // ── Implicit grants (reporter / assignee) ────────────────────────────────────
  describe('implicit grants', () => {
    it('reporter always gets manage permission', async () => {
      const task = await taskService.create({
        title: 'T', type: 'task', status: 'backlog', priority: 'low',
        reporterId: 'reporter-001', visibility: 'team',
      })
      expect(await grantService.resolvePermission('reporter-001', task.id)).toBe('manage')
    })

    it('assignee always gets edit permission', async () => {
      const task = await taskService.create({
        title: 'T', type: 'task', status: 'backlog', priority: 'low',
        reporterId: 'reporter-001', assigneeId: 'assignee-001', visibility: 'team',
      })
      expect(await grantService.resolvePermission('assignee-001', task.id)).toBe('edit')
    })
  })

  // ── Direct grants ────────────────────────────────────────────────────────────
  describe('direct user grant', () => {
    it('returns granted permission for direct user', async () => {
      const task = await taskService.create({
        title: 'T', type: 'task', status: 'backlog', priority: 'low',
        reporterId: 'reporter-001', visibility: 'team',
      })
      await grantService.grantAccess({
        taskId: task.id, scope: 'user', scopeId: 'user-001',
        permission: 'view', grantedBy: 'reporter-001',
      })
      expect(await grantService.resolvePermission('user-001', task.id)).toBe('view')
    })

    it('higher permission wins over lower', async () => {
      const task = await taskService.create({
        title: 'T', type: 'task', status: 'backlog', priority: 'low',
        reporterId: 'reporter-001', visibility: 'team',
      })
      // Grant view directly + execute via company grant
      await grantService.grantAccess({
        taskId: task.id, scope: 'user', scopeId: 'user-001',
        permission: 'view', grantedBy: 'reporter-001',
      })
      await grantService.grantAccess({
        taskId: task.id, scope: 'company',
        permission: 'execute', grantedBy: 'reporter-001',
      })
      // execute (4) > view (1)
      expect(await grantService.resolvePermission('user-001', task.id)).toBe('execute')
    })
  })

  // ── Cascade grants ────────────────────────────────────────────────────────────
  describe('ancestor cascade', () => {
    it('applyTree=true propagates grant to subtask', async () => {
      const parent = await taskService.create({
        title: 'Epic', type: 'epic', status: 'backlog', priority: 'low',
        reporterId: 'reporter-001', visibility: 'team',
      })
      const child = await taskService.create({
        title: 'Task', type: 'task', status: 'backlog', priority: 'low',
        reporterId: 'reporter-001', visibility: 'team', parentId: parent.id,
      })
      await grantService.grantAccess({
        taskId: parent.id, scope: 'user', scopeId: 'user-001',
        permission: 'edit', applyTree: true, grantedBy: 'reporter-001',
      })
      // Grant on parent with applyTree → cascades to child
      expect(await grantService.resolvePermission('user-001', child.id)).toBe('edit')
    })

    it('applyTree=false does NOT propagate to subtask', async () => {
      const parent = await taskService.create({
        title: 'Epic', type: 'epic', status: 'backlog', priority: 'low',
        reporterId: 'reporter-001', visibility: 'team',
      })
      const child = await taskService.create({
        title: 'Task', type: 'task', status: 'backlog', priority: 'low',
        reporterId: 'reporter-001', visibility: 'team', parentId: parent.id,
      })
      await grantService.grantAccess({
        taskId: parent.id, scope: 'user', scopeId: 'user-001',
        permission: 'edit', applyTree: false, grantedBy: 'reporter-001',
      })
      // No grant on child directly
      expect(await grantService.resolvePermission('user-001', child.id)).toBeNull()
    })
  })

  // ── Scope matching ────────────────────────────────────────────────────────────
  describe('team scope', () => {
    it('team grant matches user in team', async () => {
      await insertTeamMember(pool, 'user-001', 'team-alpha')
      const task = await taskService.create({
        title: 'T', type: 'task', status: 'backlog', priority: 'low',
        reporterId: 'reporter-001', visibility: 'team',
      })
      await grantService.grantAccess({
        taskId: task.id, scope: 'team', scopeId: 'team-alpha',
        permission: 'comment', grantedBy: 'reporter-001',
      })
      expect(await grantService.resolvePermission('user-001', task.id)).toBe('comment')
    })

    it('team grant does NOT match user NOT in team', async () => {
      const task = await taskService.create({
        title: 'T', type: 'task', status: 'backlog', priority: 'low',
        reporterId: 'reporter-001', visibility: 'team',
      })
      await grantService.grantAccess({
        taskId: task.id, scope: 'team', scopeId: 'team-alpha',
        permission: 'comment', grantedBy: 'reporter-001',
      })
      expect(await grantService.resolvePermission('user-002', task.id)).toBeNull()
    })
  })

  describe('company scope', () => {
    it('company grant matches any user', async () => {
      const task = await taskService.create({
        title: 'T', type: 'task', status: 'backlog', priority: 'low',
        reporterId: 'reporter-001', visibility: 'company',
      })
      await grantService.grantAccess({
        taskId: task.id, scope: 'company',
        permission: 'view', grantedBy: 'reporter-001',
      })
      expect(await grantService.resolvePermission('user-001', task.id)).toBe('view')
      expect(await grantService.resolvePermission('user-002', task.id)).toBe('view')
    })
  })

  // ── assertPermission ──────────────────────────────────────────────────────────
  describe('assertPermission', () => {
    it('passes when resolved permission >= required', async () => {
      const task = await taskService.create({
        title: 'T', type: 'task', status: 'backlog', priority: 'low',
        reporterId: 'reporter-001', visibility: 'team',
      })
      await grantService.grantAccess({
        taskId: task.id, scope: 'user', scopeId: 'user-001',
        permission: 'edit', grantedBy: 'reporter-001',
      })
      await expect(grantService.assertPermission('user-001', task.id, 'view')).resolves.toBeUndefined()
    })

    it('throws TASK_ACCESS_DENIED when insufficient permission', async () => {
      const task = await taskService.create({
        title: 'T', type: 'task', status: 'backlog', priority: 'low',
        reporterId: 'reporter-001', visibility: 'team',
      })
      await expect(grantService.assertPermission('user-001', task.id, 'view')).rejects.toThrow('TASK_ACCESS_DENIED')
    })
  })

  // ── No grant ──────────────────────────────────────────────────────────────────
  describe('no grant', () => {
    it('returns null when user has no grant at all', async () => {
      const task = await taskService.create({
        title: 'T', type: 'task', status: 'backlog', priority: 'low',
        reporterId: 'reporter-001', visibility: 'team',
      })
      expect(await grantService.resolvePermission('user-001', task.id)).toBeNull()
    })
  })
})
```

---

## Acceptance Criteria

- [x] File tạo tại `src/main/task/__tests__/TaskGrantService.test.ts` ✅
- [x] `pnpm vitest run src/main/task/__tests__/TaskGrantService.test.ts` → ≥12 tests passing ✅ (13 tests pass)
- [x] 0 TypeScript errors ✅
- [x] Không dùng vi.mock() cho pool — dùng in-memory SQLite thực ✅
