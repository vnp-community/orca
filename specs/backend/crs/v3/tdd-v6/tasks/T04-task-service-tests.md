# T04 — Write TaskService.test.ts

**Phase:** 2A (Task Tests — highest priority gap)  
**Effort:** ~2 hours  
**Depends on:** T03 (shared types), T02 (migrations)  
**Solution ref:** [05-tdd18-task-graph.md §2.1](../solutions/05-tdd18-task-graph.md)  
**TDD ref:** TDD-18 §2 (TaskService)

---

## Mục tiêu

Viết test file đầy đủ cho `TaskService` — CRUD, tree operations, dependency edges, progress calculation.

**Target: ≥ 15 tests**

---

## Files Cần Đọc Trước

1. `src/main/task/TaskService.ts` — đọc toàn bộ implementation
2. `src/main/task/TaskDAGValidator.ts` — xem constructor + detectCycle
3. `src/main/db/migrations/0010_tasks.ts` — xem schema bảng orca_tasks
4. `src/main/project/__tests__/ProjectService.test.ts` — **pattern tái sử dụng** (in-memory SQLite + ALL_MIGRATIONS)
5. `src/main/db/migrations/index.ts` — verify ALL_MIGRATIONS

---

## File Cần Tạo

### `src/main/task/__tests__/TaskService.test.ts`

**Pattern tái sử dụng từ `ProjectService.test.ts`:**

```typescript
/**
 * Tests for TaskService (TDD-18) — T04
 *
 * Uses in-memory SQLite + ALL_MIGRATIONS.
 * Pattern: same as ProjectService.test.ts
 */

import { describe, it, expect, beforeEach } from 'vitest'
import { SqliteSingleConnectionPool } from '../../db/sqlite/sqlite-pool'
import { MigrationRunner } from '../../db/migrations/runner'
import { ALL_MIGRATIONS } from '../../db/migrations'
import { TaskService } from '../TaskService'
import { TaskDAGValidator } from '../TaskDAGValidator'

// ── Helpers ──────────────────────────────────────────────────────────────────

async function makeService(): Promise<{ pool: SqliteSingleConnectionPool; service: TaskService }> {
  const pool = new SqliteSingleConnectionPool(':memory:')
  await pool.withConnection(async (db) => {
    const runner = new MigrationRunner(db, ALL_MIGRATIONS)
    await runner.migrate()
  })
  const validator = new TaskDAGValidator(pool)
  const service = new TaskService(pool, validator)
  return { pool, service }
}

/** Insert a minimal user for FK satisfaction */
async function insertUser(pool: SqliteSingleConnectionPool, userId: string): Promise<void> {
  await pool.withConnection((db) =>
    db.query(
      'INSERT INTO orca_users (id, email, name, role, provider, created_at) VALUES (?, ?, ?, ?, ?, ?)',
      [userId, `${userId}@test.com`, userId, 'developer', 'none', Date.now()]
    )
  )
}

// ── Tests ─────────────────────────────────────────────────────────────────────

describe('TaskService', () => {
  let pool: SqliteSingleConnectionPool
  let service: TaskService

  beforeEach(async () => {
    ;({ pool, service } = await makeService())
    await insertUser(pool, 'user-001')
    await insertUser(pool, 'user-002')
  })

  // ── create ──────────────────────────────────────────────────────────────────
  describe('create', () => {
    it('stores task with correct title and type', async () => {
      const task = await service.create({
        title: 'My Task',
        type: 'task',
        status: 'backlog',
        priority: 'medium',
        reporterId: 'user-001',
        visibility: 'team',
      })
      expect(task.id).toBeDefined()
      expect(task.title).toBe('My Task')
      expect(task.type).toBe('task')
    })

    it('defaults progressPercent to 0', async () => {
      const task = await service.create({
        title: 'Zero progress',
        type: 'task',
        status: 'backlog',
        priority: 'low',
        reporterId: 'user-001',
        visibility: 'team',
      })
      expect(task.progressPercent).toBe(0)
    })

    it('stores parentId when provided', async () => {
      const parent = await service.create({
        title: 'Epic',
        type: 'epic',
        status: 'backlog',
        priority: 'high',
        reporterId: 'user-001',
        visibility: 'team',
      })
      const child = await service.create({
        title: 'Story',
        type: 'story',
        status: 'backlog',
        priority: 'high',
        reporterId: 'user-001',
        visibility: 'team',
        parentId: parent.id,
      })
      expect(child.parentId).toBe(parent.id)
    })
  })

  // ── get / update / delete ────────────────────────────────────────────────────
  describe('get + update + delete', () => {
    it('get returns task by id', async () => {
      const task = await service.create({ title: 'T', type: 'task', status: 'backlog', priority: 'low', reporterId: 'user-001', visibility: 'team' })
      const fetched = await service.get(task.id)
      expect(fetched?.id).toBe(task.id)
    })

    it('get returns null for unknown id', async () => {
      expect(await service.get('nonexistent')).toBeNull()
    })

    it('update changes status', async () => {
      const task = await service.create({ title: 'T', type: 'task', status: 'backlog', priority: 'low', reporterId: 'user-001', visibility: 'team' })
      await service.update(task.id, { status: 'in_progress' })
      const updated = await service.get(task.id)
      expect(updated?.status).toBe('in_progress')
    })

    it('delete removes task', async () => {
      const task = await service.create({ title: 'Del', type: 'task', status: 'backlog', priority: 'low', reporterId: 'user-001', visibility: 'team' })
      await service.delete(task.id)
      expect(await service.get(task.id)).toBeNull()
    })
  })

  // ── tree operations ──────────────────────────────────────────────────────────
  describe('getChildren', () => {
    it('returns direct children only', async () => {
      const parent = await service.create({ title: 'P', type: 'epic', status: 'backlog', priority: 'low', reporterId: 'user-001', visibility: 'team' })
      const child1 = await service.create({ title: 'C1', type: 'task', status: 'backlog', priority: 'low', reporterId: 'user-001', visibility: 'team', parentId: parent.id })
      const child2 = await service.create({ title: 'C2', type: 'task', status: 'backlog', priority: 'low', reporterId: 'user-001', visibility: 'team', parentId: parent.id })
      const children = await service.getChildren(parent.id)
      expect(children.map(c => c.id).sort()).toEqual([child1.id, child2.id].sort())
    })
  })

  describe('getAncestors', () => {
    it('returns 3-level ancestor chain in correct order', async () => {
      const grandparent = await service.create({ title: 'GP', type: 'epic', status: 'backlog', priority: 'low', reporterId: 'user-001', visibility: 'team' })
      const parent = await service.create({ title: 'P', type: 'story', status: 'backlog', priority: 'low', reporterId: 'user-001', visibility: 'team', parentId: grandparent.id })
      const child = await service.create({ title: 'C', type: 'task', status: 'backlog', priority: 'low', reporterId: 'user-001', visibility: 'team', parentId: parent.id })
      const ancestors = await service.getAncestors(child.id)
      expect(ancestors.length).toBe(2)
      // Closest ancestor first
      expect(ancestors[0].id).toBe(parent.id)
      expect(ancestors[1].id).toBe(grandparent.id)
    })

    it('returns empty array for root task', async () => {
      const root = await service.create({ title: 'Root', type: 'epic', status: 'backlog', priority: 'low', reporterId: 'user-001', visibility: 'team' })
      expect(await service.getAncestors(root.id)).toEqual([])
    })
  })

  // ── dependency edges ─────────────────────────────────────────────────────────
  describe('addEdge', () => {
    it('inserts edge when no cycle', async () => {
      const a = await service.create({ title: 'A', type: 'task', status: 'backlog', priority: 'low', reporterId: 'user-001', visibility: 'team' })
      const b = await service.create({ title: 'B', type: 'task', status: 'backlog', priority: 'low', reporterId: 'user-001', visibility: 'team' })
      await expect(service.addEdge(a.id, b.id, 'depends_on')).resolves.toBeUndefined()
    })

    it('throws TASK_DEPENDENCY_CYCLE when adding creates cycle', async () => {
      const a = await service.create({ title: 'A', type: 'task', status: 'backlog', priority: 'low', reporterId: 'user-001', visibility: 'team' })
      const b = await service.create({ title: 'B', type: 'task', status: 'backlog', priority: 'low', reporterId: 'user-001', visibility: 'team' })
      await service.addEdge(a.id, b.id, 'depends_on')
      await expect(service.addEdge(b.id, a.id, 'depends_on')).rejects.toThrow('TASK_DEPENDENCY_CYCLE')
    })
  })

  // ── progress calculation ─────────────────────────────────────────────────────
  describe('recalculateProgress', () => {
    it('leaf task with status done → 100', async () => {
      const task = await service.create({ title: 'Done', type: 'task', status: 'done', priority: 'low', reporterId: 'user-001', visibility: 'team' })
      const progress = await service.recalculateProgress(task.id)
      expect(progress).toBe(100)
    })

    it('leaf task with status in_progress → 40', async () => {
      const task = await service.create({ title: 'WIP', type: 'task', status: 'in_progress', priority: 'low', reporterId: 'user-001', visibility: 'team' })
      expect(await service.recalculateProgress(task.id)).toBe(40)
    })

    it('parent avg of children progress', async () => {
      const parent = await service.create({ title: 'P', type: 'epic', status: 'backlog', priority: 'low', reporterId: 'user-001', visibility: 'team' })
      await service.create({ title: 'C1', type: 'task', status: 'done', priority: 'low', reporterId: 'user-001', visibility: 'team', parentId: parent.id })
      await service.create({ title: 'C2', type: 'task', status: 'backlog', priority: 'low', reporterId: 'user-001', visibility: 'team', parentId: parent.id })
      // done=100, backlog=0 → avg=50
      expect(await service.recalculateProgress(parent.id)).toBe(50)
    })
  })

  // ── list with filters ────────────────────────────────────────────────────────
  describe('list', () => {
    it('filters by assigneeId', async () => {
      await service.create({ title: 'A1', type: 'task', status: 'todo', priority: 'low', reporterId: 'user-001', visibility: 'team', assigneeId: 'user-001' })
      await service.create({ title: 'A2', type: 'task', status: 'todo', priority: 'low', reporterId: 'user-001', visibility: 'team', assigneeId: 'user-002' })
      const results = await service.list({ assigneeId: 'user-001' })
      expect(results.every(t => t.assigneeId === 'user-001')).toBe(true)
    })

    it('filters by status array', async () => {
      await service.create({ title: 'T1', type: 'task', status: 'todo', priority: 'low', reporterId: 'user-001', visibility: 'team' })
      await service.create({ title: 'T2', type: 'task', status: 'done', priority: 'low', reporterId: 'user-001', visibility: 'team' })
      const active = await service.list({ status: ['todo', 'in_progress'] })
      expect(active.every(t => ['todo', 'in_progress'].includes(t.status))).toBe(true)
    })

    it('filters root tasks with parentId = null', async () => {
      const root = await service.create({ title: 'Root', type: 'epic', status: 'backlog', priority: 'low', reporterId: 'user-001', visibility: 'team' })
      await service.create({ title: 'Child', type: 'task', status: 'backlog', priority: 'low', reporterId: 'user-001', visibility: 'team', parentId: root.id })
      const roots = await service.list({ parentId: null })
      expect(roots.some(t => t.id === root.id)).toBe(true)
      expect(roots.every(t => !t.parentId)).toBe(true)
    })
  })
})
```

---

## Acceptance Criteria

- [x] File tạo tại `src/main/task/__tests__/TaskService.test.ts` ✅
- [x] `pnpm vitest run src/main/task/__tests__/TaskService.test.ts` → ≥15 tests passing ✅ (24 tests pass)
- [x] 0 TypeScript errors trong file ✅
- [x] Không sử dụng mock pool — dùng in-memory SQLite thực ✅
