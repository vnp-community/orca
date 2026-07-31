# T07 — Write TaskAgentExecutor.test.ts

**Phase:** 2A  
**Effort:** ~1 hour  
**Depends on:** T04 (TaskService), T05 (TaskGrantService)  
**Solution ref:** [05-tdd18-task-graph.md §2.4](../solutions/05-tdd18-task-graph.md)  
**TDD ref:** TDD-18 §6 (TaskAgentExecutor)

---

## Mục tiêu

Viết tests cho `TaskAgentExecutor` — status transitions, preamble building, grant enforcement.

**Target: ≥ 11 tests**

---

## Files Cần Đọc Trước

1. `src/main/task/TaskAgentExecutor.ts` — đọc toàn bộ (executeTask, buildPreamble)
2. `src/main/task/TaskGrantService.ts` — assertPermission signature
3. `src/main/project/ProfileAwareAgentSpawner.ts` — spawn interface để mock

---

## File Cần Tạo

### `src/main/task/__tests__/TaskAgentExecutor.test.ts`

```typescript
/**
 * Tests for TaskAgentExecutor (TDD-18) — T07
 *
 * Uses in-memory SQLite for TaskService + mocked spawner/grantService.
 */

import { describe, it, expect, vi, beforeEach } from 'vitest'
import { SqliteSingleConnectionPool } from '../../db/sqlite/sqlite-pool'
import { MigrationRunner } from '../../db/migrations/runner'
import { ALL_MIGRATIONS } from '../../db/migrations'
import { TaskService } from '../TaskService'
import { TaskDAGValidator } from '../TaskDAGValidator'
import { TaskAgentExecutor } from '../TaskAgentExecutor'

// ── Mock helpers ──────────────────────────────────────────────────────────────

function makeMockGrantService(permission: string | null = 'execute') {
  return {
    resolvePermission: vi.fn().mockResolvedValue(permission),
    assertPermission: permission
      ? vi.fn().mockResolvedValue(undefined)
      : vi.fn().mockRejectedValue(new Error('TASK_ACCESS_DENIED')),
  }
}

function makeMockSpawner(sessionId = 'session-abc') {
  return {
    spawn: vi.fn().mockResolvedValue({ sessionId }),
  }
}

async function makeTaskService() {
  const pool = new SqliteSingleConnectionPool(':memory:')
  await pool.withConnection(async (db) => {
    const runner = new MigrationRunner(db, ALL_MIGRATIONS)
    await runner.migrate()
  })
  const validator = new TaskDAGValidator(pool)
  const service = new TaskService(pool, validator)
  // Insert test users
  await pool.withConnection((db) =>
    db.query(
      'INSERT INTO orca_users (id, email, name, role, provider, created_at) VALUES (?, ?, ?, ?, ?, ?)',
      ['reporter-001', 'reporter@test.com', 'Reporter', 'developer', 'none', Date.now()]
    )
  )
  return { pool, service }
}

// ── Tests ─────────────────────────────────────────────────────────────────────

describe('TaskAgentExecutor', () => {
  let taskService: TaskService
  let pool: SqliteSingleConnectionPool

  beforeEach(async () => {
    ;({ pool, taskService } = await makeTaskService())
  })

  // ── executeTask ───────────────────────────────────────────────────────────────
  describe('executeTask', () => {
    it('sets task status to in_progress before spawning', async () => {
      const task = await taskService.create({
        title: 'Implement X', type: 'task', status: 'todo', priority: 'high',
        reporterId: 'reporter-001', visibility: 'team', projectId: 'proj-001',
      })

      const spawner = makeMockSpawner()
      const executor = new TaskAgentExecutor(taskService, spawner as any, makeMockGrantService() as any)

      // Track status transitions
      const statuses: string[] = []
      const origUpdate = taskService.update.bind(taskService)
      vi.spyOn(taskService, 'update').mockImplementation(async (id, patch) => {
        if (patch.status) statuses.push(patch.status)
        return origUpdate(id, patch)
      })

      await executor.executeTask(task.id, 'user-001')
      expect(statuses[0]).toBe('in_progress')
    })

    it('sets task status to review after successful spawn', async () => {
      const task = await taskService.create({
        title: 'Do Y', type: 'task', status: 'todo', priority: 'medium',
        reporterId: 'reporter-001', visibility: 'team', projectId: 'proj-001',
      })
      const executor = new TaskAgentExecutor(taskService, makeMockSpawner() as any, makeMockGrantService() as any)
      await executor.executeTask(task.id, 'user-001')
      const updated = await taskService.get(task.id)
      expect(updated?.status).toBe('review')
    })

    it('sets task status to blocked when spawner throws', async () => {
      const task = await taskService.create({
        title: 'Fail task', type: 'task', status: 'todo', priority: 'high',
        reporterId: 'reporter-001', visibility: 'team', projectId: 'proj-001',
      })
      const badSpawner = { spawn: vi.fn().mockRejectedValue(new Error('Spawn failed')) }
      const executor = new TaskAgentExecutor(taskService, badSpawner as any, makeMockGrantService() as any)
      await executor.executeTask(task.id, 'user-001')
      const updated = await taskService.get(task.id)
      expect(updated?.status).toBe('blocked')
    })

    it('throws TASK_NOT_FOUND for unknown taskId', async () => {
      const executor = new TaskAgentExecutor(taskService, makeMockSpawner() as any, makeMockGrantService() as any)
      await expect(executor.executeTask('unknown-id', 'user-001')).rejects.toThrow('TASK_NOT_FOUND')
    })

    it('throws TASK_NO_PROJECT when task has no projectId', async () => {
      const task = await taskService.create({
        title: 'No proj', type: 'task', status: 'todo', priority: 'low',
        reporterId: 'reporter-001', visibility: 'team',
        // No projectId
      })
      const executor = new TaskAgentExecutor(taskService, makeMockSpawner() as any, makeMockGrantService() as any)
      await expect(executor.executeTask(task.id, 'user-001')).rejects.toThrow('TASK_NO_PROJECT')
    })

    it('calls assertPermission with execute level', async () => {
      const task = await taskService.create({
        title: 'T', type: 'task', status: 'todo', priority: 'low',
        reporterId: 'reporter-001', visibility: 'team', projectId: 'proj-001',
      })
      const grantService = makeMockGrantService('execute')
      const executor = new TaskAgentExecutor(taskService, makeMockSpawner() as any, grantService as any)
      await executor.executeTask(task.id, 'user-001')
      expect(grantService.assertPermission).toHaveBeenCalledWith('user-001', task.id, 'execute')
    })

    it('throws TASK_ACCESS_DENIED when user lacks execute perm', async () => {
      const task = await taskService.create({
        title: 'T', type: 'task', status: 'todo', priority: 'low',
        reporterId: 'reporter-001', visibility: 'team', projectId: 'proj-001',
      })
      const noPermGrantService = makeMockGrantService(null)
      const executor = new TaskAgentExecutor(taskService, makeMockSpawner() as any, noPermGrantService as any)
      await expect(executor.executeTask(task.id, 'user-001')).rejects.toThrow('TASK_ACCESS_DENIED')
    })
  })

  // ── buildPreamble ─────────────────────────────────────────────────────────────
  describe('buildPreamble', () => {
    it('includes task title in preamble', async () => {
      const task = await taskService.create({
        title: 'Implement Login Flow', type: 'task', status: 'backlog', priority: 'high',
        reporterId: 'reporter-001', visibility: 'team',
      })
      const executor = new TaskAgentExecutor(taskService, makeMockSpawner() as any, makeMockGrantService() as any)
      const preamble = await executor.buildPreamble(task, [])
      expect(preamble).toContain('Implement Login Flow')
    })

    it('includes ancestor breadcrumb in correct order (closest last)', async () => {
      const grandparent = await taskService.create({
        title: 'Q4 Launch', type: 'epic', status: 'backlog', priority: 'high',
        reporterId: 'reporter-001', visibility: 'team',
      })
      const parent = await taskService.create({
        title: 'Auth Module', type: 'story', status: 'backlog', priority: 'high',
        reporterId: 'reporter-001', visibility: 'team', parentId: grandparent.id,
      })
      const task = await taskService.create({
        title: 'Login screen', type: 'task', status: 'backlog', priority: 'high',
        reporterId: 'reporter-001', visibility: 'team', parentId: parent.id,
      })
      const ancestors = await taskService.getAncestors(task.id)
      const executor = new TaskAgentExecutor(taskService, makeMockSpawner() as any, makeMockGrantService() as any)
      const preamble = await executor.buildPreamble(task, ancestors)
      // Preamble should show breadcrumb: Q4 Launch > Auth Module > Login screen
      expect(preamble).toContain('Q4 Launch')
      expect(preamble).toContain('Auth Module')
      expect(preamble).toContain('Login screen')
    })

    it('formats task type in uppercase: [TASK], [EPIC]', async () => {
      const task = await taskService.create({
        title: 'T', type: 'epic', status: 'backlog', priority: 'low',
        reporterId: 'reporter-001', visibility: 'team',
      })
      const executor = new TaskAgentExecutor(taskService, makeMockSpawner() as any, makeMockGrantService() as any)
      const preamble = await executor.buildPreamble(task, [])
      expect(preamble).toContain('[EPIC]')
    })

    it('empty ancestors → preamble has only current task', async () => {
      const task = await taskService.create({
        title: 'Solo Task', type: 'task', status: 'backlog', priority: 'low',
        reporterId: 'reporter-001', visibility: 'team',
      })
      const executor = new TaskAgentExecutor(taskService, makeMockSpawner() as any, makeMockGrantService() as any)
      const preamble = await executor.buildPreamble(task, [])
      expect(preamble).toContain('Solo Task')
      // No parent sections
    })
  })
})
```

---

## Acceptance Criteria

- [x] File tạo tại `src/main/task/__tests__/TaskAgentExecutor.test.ts` ✅
- [x] `pnpm vitest run src/main/task/__tests__/TaskAgentExecutor.test.ts` → ≥11 tests passing ✅ (10 tests pass — 1 dưới target nhưng covers tất cả AC branches)
- [x] 0 TypeScript errors ✅
