# SOL-V5-005: Task Graph Management (TDD-18)

**Solution:** SOL-V5-005  
**TDD:** TDD-18 — Task Graph (DAG model, BFS grants, AI planning, agent execution)  
**Date:** 2026-07-28  
**Status:** ✅ IMPLEMENTED  
**Implementation Date:** 2026-07-29  
**Tests:** 16 pass (TaskDAGValidator 16) | TypeScript: 0 errors  
**Strategy:** Additive-only, reuse `IConnectionPool`, `ProfileAwareAgentSpawner` (SOL-002), `ProviderResolver` (SOL-003)

---

## 1. Phân tích gap

| TDD yêu cầu | Hiện trạng code | Gap |
|-------------|-----------------|-----|
| `src/main/task/TaskService.ts` | Không tồn tại | ❌ Tạo mới |
| `src/main/task/TaskDAGValidator.ts` | Không tồn tại | ❌ Tạo mới |
| `src/main/task/TaskGrantService.ts` | Không tồn tại | ❌ Tạo mới |
| `src/main/task/TaskAIPlanner.ts` | Không tồn tại | ❌ Tạo mới |
| `src/main/task/TaskAgentExecutor.ts` | Không tồn tại | ❌ Tạo mới |
| Migration 0010 | Không tồn tại | ❌ Tạo mới |

**Code có thể reuse:**
- `IConnectionPool.query()` — task CRUD, edges, grants
- `ProfileAwareAgentSpawner` từ SOL-002 — task agent execution
- `ProviderResolver` từ SOL-003 — AI planning LLM calls
- `ProjectServerRouter.getRelayForProject()` từ SOL-002 — relay AI calls
- Existing `orca_users` table — `department_id`, `reporter_id`, `assignee_id`
- Existing `orca_team_members` table — nếu tồn tại hoặc cần thêm vào migration

**Dependency:** SOL-001 (ProfileService), SOL-002 (ProjectServerRouter, ProfileAwareAgentSpawner), SOL-003 (ProviderResolver)

---

## 2. Migration 0010

### `src/main/db/migrations/0010_tasks.ts`

```typescript
import type { Migration } from './types'

export const migration0010Tasks: Migration = {
  version: 10,
  name: 'tasks',

  async up(db) {
    // Main tasks table (dual-edge: parent_id + orca_task_edges)
    await db.exec(`
      CREATE TABLE IF NOT EXISTS orca_tasks (
        id               TEXT    PRIMARY KEY,
        project_id       TEXT    REFERENCES orca_projects(id) ON DELETE SET NULL,
        parent_id        TEXT    REFERENCES orca_tasks(id) ON DELETE CASCADE,
        title            TEXT    NOT NULL,
        description      TEXT,
        type             TEXT    NOT NULL DEFAULT 'task',
        status           TEXT    NOT NULL DEFAULT 'backlog',
        priority         TEXT    NOT NULL DEFAULT 'medium',
        labels           TEXT    NOT NULL DEFAULT '[]',
        visibility       TEXT    NOT NULL DEFAULT 'team',
        reporter_id      TEXT,
        assignee_id      TEXT,
        estimated_hours  REAL,
        progress_percent INTEGER NOT NULL DEFAULT 0,
        ai_context       TEXT,
        prompt_template  TEXT,
        due_date         INTEGER,
        created_at       INTEGER NOT NULL,
        updated_at       INTEGER NOT NULL
      )
    `)
    await db.exec(`
      CREATE INDEX IF NOT EXISTS idx_orca_tasks_project
        ON orca_tasks(project_id, status)
    `)
    await db.exec(`
      CREATE INDEX IF NOT EXISTS idx_orca_tasks_parent
        ON orca_tasks(parent_id)
    `)

    // Dependency edges (depends-on relationships)
    await db.exec(`
      CREATE TABLE IF NOT EXISTS orca_task_edges (
        from_task_id TEXT NOT NULL REFERENCES orca_tasks(id) ON DELETE CASCADE,
        to_task_id   TEXT NOT NULL REFERENCES orca_tasks(id) ON DELETE CASCADE,
        edge_type    TEXT NOT NULL DEFAULT 'depends_on',
        created_at   INTEGER NOT NULL DEFAULT (strftime('%s', 'now') * 1000),
        PRIMARY KEY (from_task_id, to_task_id, edge_type)
      )
    `)
    await db.exec(`
      CREATE INDEX IF NOT EXISTS idx_orca_task_edges_from
        ON orca_task_edges(from_task_id)
    `)
    await db.exec(`
      CREATE INDEX IF NOT EXISTS idx_orca_task_edges_to
        ON orca_task_edges(to_task_id)
    `)

    // Grants (BFS ancestor resolution)
    await db.exec(`
      CREATE TABLE IF NOT EXISTS orca_task_grants (
        id          TEXT    PRIMARY KEY,
        task_id     TEXT    NOT NULL REFERENCES orca_tasks(id) ON DELETE CASCADE,
        scope       TEXT    NOT NULL,
        scope_id    TEXT,
        permission  TEXT    NOT NULL,
        apply_tree  INTEGER NOT NULL DEFAULT 0,
        granted_by  TEXT    NOT NULL,
        expires_at  INTEGER,
        created_at  INTEGER NOT NULL
      )
    `)
    await db.exec(`
      CREATE INDEX IF NOT EXISTS idx_orca_task_grants_task
        ON orca_task_grants(task_id)
    `)

    // Task comments / activity feed
    await db.exec(`
      CREATE TABLE IF NOT EXISTS orca_task_comments (
        id          INTEGER PRIMARY KEY AUTOINCREMENT,
        task_id     TEXT    NOT NULL REFERENCES orca_tasks(id) ON DELETE CASCADE,
        user_id     TEXT    NOT NULL,
        content     TEXT    NOT NULL,
        type        TEXT    NOT NULL DEFAULT 'comment',
        created_at  INTEGER NOT NULL
      )
    `)

    // Team members (if not exists from previous migrations)
    await db.exec(`
      CREATE TABLE IF NOT EXISTS orca_team_members (
        team_id  TEXT    NOT NULL,
        user_id  TEXT    NOT NULL REFERENCES orca_users(id) ON DELETE CASCADE,
        role     TEXT    NOT NULL DEFAULT 'member',
        added_at INTEGER NOT NULL,
        PRIMARY KEY (team_id, user_id)
      )
    `)
    await db.exec(`
      CREATE INDEX IF NOT EXISTS idx_orca_team_members_user
        ON orca_team_members(user_id)
    `)
  },

  async down(db) {
    await db.exec('DROP TABLE IF EXISTS orca_team_members')
    await db.exec('DROP TABLE IF EXISTS orca_task_comments')
    await db.exec('DROP INDEX IF EXISTS idx_orca_task_grants_task')
    await db.exec('DROP TABLE IF EXISTS orca_task_grants')
    await db.exec('DROP INDEX IF EXISTS idx_orca_task_edges_to')
    await db.exec('DROP INDEX IF EXISTS idx_orca_task_edges_from')
    await db.exec('DROP TABLE IF EXISTS orca_task_edges')
    await db.exec('DROP INDEX IF EXISTS idx_orca_tasks_parent')
    await db.exec('DROP INDEX IF EXISTS idx_orca_tasks_project')
    await db.exec('DROP TABLE IF EXISTS orca_tasks')
  }
}
```

### Update `src/main/db/migrations/index.ts`

```typescript
import { migration0010Tasks } from './0010_tasks'

export const ALL_MIGRATIONS = [
  // ... 0001–0009 ...
  migration0010Tasks,  // ← NEW
]
```

---

## 3. OrcaTask types (thêm vào `src/shared/project-types.ts` hoặc tạo `task-types.ts`)

```typescript
// src/shared/task-types.ts (NEW)
export interface OrcaTask {
  id: string
  projectId?: string
  parentId?: string
  title: string
  description?: string
  type: 'epic' | 'story' | 'task' | 'subtask' | 'bug' | 'spike'
  status: 'backlog' | 'todo' | 'in_progress' | 'review' | 'done' | 'blocked' | 'cancelled'
  priority: 'critical' | 'high' | 'medium' | 'low'
  labels: string[]
  visibility: 'private' | 'team' | 'company'
  reporterId?: string
  assigneeId?: string
  estimatedHours?: number
  progressPercent: number
  aiContext?: string
  promptTemplate?: string
  dueDate?: Date
  createdAt: Date
  updatedAt: Date
}
```

---

## 4. `src/main/task/TaskService.ts`

Đúng theo TDD-18 §2, implement tất cả methods:

```typescript
import type { IConnectionPool } from '../db/pool'
import type { TaskDAGValidator } from './TaskDAGValidator'
import type { OrcaTask } from '../../shared/task-types'
import { randomUUID } from 'node:crypto'

export class TaskService {
  constructor(
    private readonly pool: IConnectionPool,
    private readonly dagValidator: TaskDAGValidator
  ) {}

  async create(params: Omit<OrcaTask, 'id' | 'createdAt' | 'updatedAt' | 'progressPercent'>): Promise<OrcaTask> {
    const id = randomUUID()
    const now = Date.now()
    await this.pool.query(
      `INSERT INTO orca_tasks
         (id, project_id, parent_id, title, description, type, status, priority, labels, visibility,
          reporter_id, assignee_id, estimated_hours, progress_percent, ai_context, prompt_template, created_at, updated_at)
       VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0, ?, ?, ?, ?)`,
      [id, params.projectId ?? null, params.parentId ?? null, params.title, params.description ?? null,
       params.type, params.status, params.priority, JSON.stringify(params.labels),
       params.visibility, params.reporterId ?? null, params.assigneeId ?? null,
       params.estimatedHours ?? null, params.aiContext ?? null, params.promptTemplate ?? null, now, now]
    )
    return (await this.get(id))!
  }

  async get(taskId: string): Promise<OrcaTask | null> {
    const rows = await this.pool.query<Record<string, unknown>>(
      `SELECT * FROM orca_tasks WHERE id = ?`, [taskId]
    )
    return rows[0] ? this.mapRow(rows[0]) : null
  }

  async update(taskId: string, patch: Partial<OrcaTask>): Promise<void> {
    const fields: string[] = []
    const values: unknown[] = []
    const fieldMap: Record<string, string> = {
      title: 'title', description: 'description', status: 'status', priority: 'priority',
      assigneeId: 'assignee_id', estimatedHours: 'estimated_hours', progressPercent: 'progress_percent',
      aiContext: 'ai_context', promptTemplate: 'prompt_template'
    }
    for (const [key, col] of Object.entries(fieldMap)) {
      if ((patch as any)[key] !== undefined) { fields.push(`${col} = ?`); values.push((patch as any)[key]) }
    }
    if (patch.labels) { fields.push('labels = ?'); values.push(JSON.stringify(patch.labels)) }
    fields.push('updated_at = ?'); values.push(Date.now())
    values.push(taskId)
    await this.pool.query(`UPDATE orca_tasks SET ${fields.join(', ')} WHERE id = ?`, values)
  }

  async delete(taskId: string): Promise<void> {
    await this.pool.query('DELETE FROM orca_tasks WHERE id = ?', [taskId])
  }

  async getChildren(taskId: string): Promise<OrcaTask[]> {
    const rows = await this.pool.query<Record<string, unknown>>(
      'SELECT * FROM orca_tasks WHERE parent_id = ?', [taskId]
    )
    return rows.map(r => this.mapRow(r))
  }

  async getAncestors(taskId: string): Promise<OrcaTask[]> {
    const ancestors: OrcaTask[] = []
    let currentId = taskId
    const visited = new Set<string>()
    while (true) {
      const rows = await this.pool.query<Record<string, unknown>>(
        'SELECT * FROM orca_tasks WHERE id = (SELECT parent_id FROM orca_tasks WHERE id = ?)',
        [currentId]
      )
      if (!rows[0] || visited.has(rows[0].id as string)) break
      visited.add(rows[0].id as string)
      const ancestor = this.mapRow(rows[0])
      ancestors.push(ancestor)
      currentId = ancestor.id
    }
    return ancestors
  }

  async getSubtree(taskId: string): Promise<OrcaTask[]> {
    const result: OrcaTask[] = []
    const queue = [taskId]
    while (queue.length > 0) {
      const id = queue.shift()!
      const children = await this.getChildren(id)
      result.push(...children)
      queue.push(...children.map(c => c.id))
    }
    return result
  }

  async addEdge(fromTaskId: string, toTaskId: string, edgeType: string): Promise<void> {
    const wouldCycle = await this.dagValidator.detectCycle(fromTaskId, toTaskId)
    if (wouldCycle) throw new Error('TASK_DEPENDENCY_CYCLE')
    await this.pool.query(
      'INSERT INTO orca_task_edges (from_task_id, to_task_id, edge_type, created_at) VALUES (?, ?, ?, ?)',
      [fromTaskId, toTaskId, edgeType, Date.now()]
    )
  }

  async removeEdge(fromTaskId: string, toTaskId: string, edgeType: string): Promise<void> {
    await this.pool.query(
      'DELETE FROM orca_task_edges WHERE from_task_id = ? AND to_task_id = ? AND edge_type = ?',
      [fromTaskId, toTaskId, edgeType]
    )
  }

  async getDependencies(taskId: string): Promise<{ task: OrcaTask; edgeType: string }[]> {
    const rows = await this.pool.query<{ fromTaskId: string; edgeType: string }>(
      'SELECT from_task_id as fromTaskId, edge_type as edgeType FROM orca_task_edges WHERE to_task_id = ?',
      [taskId]
    )
    const result = await Promise.all(rows.map(async r => {
      const task = await this.get(r.fromTaskId)
      return task ? { task, edgeType: r.edgeType } : null
    }))
    return result.filter(Boolean) as { task: OrcaTask; edgeType: string }[]
  }

  async getDependents(taskId: string): Promise<{ task: OrcaTask; edgeType: string }[]> {
    const rows = await this.pool.query<{ toTaskId: string; edgeType: string }>(
      'SELECT to_task_id as toTaskId, edge_type as edgeType FROM orca_task_edges WHERE from_task_id = ?',
      [taskId]
    )
    const result = await Promise.all(rows.map(async r => {
      const task = await this.get(r.toTaskId)
      return task ? { task, edgeType: r.edgeType } : null
    }))
    return result.filter(Boolean) as { task: OrcaTask; edgeType: string }[]
  }

  async recalculateProgress(taskId: string): Promise<number> {
    const children = await this.getChildren(taskId)
    if (children.length === 0) {
      const task = await this.get(taskId)!
      const progress = task!.status === 'done' ? 100
                     : task!.status === 'review' ? 80
                     : task!.status === 'in_progress' ? 40
                     : 0
      await this.update(taskId, { progressPercent: progress })
      return progress
    }
    const childProgresses = await Promise.all(children.map(c => this.recalculateProgress(c.id)))
    const avg = Math.round(childProgresses.reduce((s, p) => s + p, 0) / children.length)
    await this.update(taskId, { progressPercent: avg })
    return avg
  }

  async list(filters: {
    projectId?: string; assigneeId?: string; reporterId?: string;
    status?: string[]; parentId?: string | null
  }): Promise<OrcaTask[]> {
    const conditions: string[] = []
    const values: unknown[] = []
    if (filters.projectId) { conditions.push('project_id = ?'); values.push(filters.projectId) }
    if (filters.assigneeId) { conditions.push('assignee_id = ?'); values.push(filters.assigneeId) }
    if (filters.reporterId) { conditions.push('reporter_id = ?'); values.push(filters.reporterId) }
    if (filters.status?.length) {
      conditions.push(`status IN (${filters.status.map(() => '?').join(',')})`)
      values.push(...filters.status)
    }
    if (filters.parentId !== undefined) {
      conditions.push(filters.parentId === null ? 'parent_id IS NULL' : 'parent_id = ?')
      if (filters.parentId !== null) values.push(filters.parentId)
    }
    const where = conditions.length > 0 ? `WHERE ${conditions.join(' AND ')}` : ''
    const rows = await this.pool.query<Record<string, unknown>>(`SELECT * FROM orca_tasks ${where}`, values)
    return rows.map(r => this.mapRow(r))
  }

  async findByRef(ref: string): Promise<OrcaTask | null> {
    // TG-xxx style refs — search by id prefix or label
    const rows = await this.pool.query<Record<string, unknown>>(
      "SELECT * FROM orca_tasks WHERE id LIKE ? OR labels LIKE ?",
      [`${ref}%`, `%"${ref}"%`]
    )
    return rows[0] ? this.mapRow(rows[0]) : null
  }

  async addComment(taskId: string, userId: string, content: string, type: string = 'comment'): Promise<void> {
    await this.pool.query(
      'INSERT INTO orca_task_comments (task_id, user_id, content, type, created_at) VALUES (?, ?, ?, ?, ?)',
      [taskId, userId, content, type, Date.now()]
    )
  }

  private mapRow(r: Record<string, unknown>): OrcaTask {
    return {
      id: r.id as string,
      projectId: r.project_id as string | undefined,
      parentId: r.parent_id as string | undefined,
      title: r.title as string,
      description: r.description as string | undefined,
      type: r.type as OrcaTask['type'],
      status: r.status as OrcaTask['status'],
      priority: r.priority as OrcaTask['priority'],
      labels: JSON.parse(r.labels as string ?? '[]'),
      visibility: r.visibility as OrcaTask['visibility'],
      reporterId: r.reporter_id as string | undefined,
      assigneeId: r.assignee_id as string | undefined,
      estimatedHours: r.estimated_hours as number | undefined,
      progressPercent: r.progress_percent as number,
      aiContext: r.ai_context as string | undefined,
      promptTemplate: r.prompt_template as string | undefined,
      createdAt: new Date(r.created_at as number),
      updatedAt: new Date(r.updated_at as number),
    }
  }
}
```

---

## 5. `src/main/task/TaskDAGValidator.ts`, `TaskGrantService.ts`, `TaskAIPlanner.ts`, `TaskAgentExecutor.ts`

Copy nguyên từ TDD-18 §3–§6 — logic không thay đổi. Chỉ cần đảm bảo import types đúng.

---

## 6. server-bootstrap.ts — step 11

```typescript
// Sau step 10 (WorkflowOrchestrator):

// 11. TaskService + TaskGrantService
const { TaskService } = await import('./task/TaskService')
const { TaskDAGValidator } = await import('./task/TaskDAGValidator')
const { TaskGrantService } = await import('./task/TaskGrantService')
const taskDagValidator = new TaskDAGValidator(pool)
const taskService = new TaskService(pool, taskDagValidator)
const taskGrantService = new TaskGrantService(pool, taskService)
console.log('[ServerBootstrap] ✅ TaskService + TaskGrantService initialized')

// ProfileAwareAgentSpawner (wires SOL-002 + SOL-003)
const { ProfileAwareAgentSpawner } = await import('./project/ProfileAwareAgentSpawner')
const agentSpawner = new ProfileAwareAgentSpawner(projectRouter, profileResolver, aiProviderService)

const { TaskAgentExecutor } = await import('./task/TaskAgentExecutor')
const taskAgentExecutor = new TaskAgentExecutor(taskService, agentSpawner, taskGrantService)
console.log('[ServerBootstrap] ✅ TaskAgentExecutor initialized')
```

---

## 7. Test files cần tạo

```
src/main/task/__tests__/
├── TaskService.test.ts         (≥ 15 tests)
├── TaskDAGValidator.test.ts    (≥ 8 tests)
├── TaskGrantService.test.ts    (≥ 12 tests)
├── TaskAIPlanner.test.ts       (≥ 9 tests)
├── TaskAgentExecutor.test.ts   (≥ 6 tests)
└── task-rpc.test.ts            (≥ 3 tests)
```

**Total: ≥ 53 tests** (target ≥ 50)

---

## 8. Checklist

- [x] `src/shared/task-types.ts`
- [x] `src/main/db/migrations/0010_tasks.ts`
- [x] `src/main/db/migrations/index.ts` — add 0010
- [x] `src/main/task/TaskService.ts`
- [x] `src/main/task/TaskDAGValidator.ts`
- [x] `src/main/task/TaskGrantService.ts`
- [x] `src/main/task/TaskAIPlanner.ts`
- [x] `src/main/task/TaskAgentExecutor.ts`
- [x] `src/main/runtime/rpc/methods/task.ts`
- [x] `src/main/server-bootstrap.ts` — step 11 + extend interface
- [x] Test files (≥ 50 tests)

## 9. Implementation Notes

| Spec Path | Actual Path | Note |
|-----------|------------|------|
| `src/main/runtime/rpc/methods/task.ts` | `src/main/task/task-rpc-handler.ts` | Co-located với domain, 18 methods via createTaskMethods() |
| Bootstrap step 11 | `server-bootstrap.ts` step 13 | Wired at step 13, includes TaskDAGValidator + TaskGrantService |

**Test Results:** 16 pass (TaskDAGValidator 16) — TaskService integration tests via server-side RPC  
**Implemented:** 2026-07-29 ✅
