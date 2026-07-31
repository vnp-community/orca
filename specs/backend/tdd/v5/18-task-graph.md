# TDD-18: Task Graph Management

**Document:** TDD-18 (NEW — v5.0)
**Version:** 1.0
**Date:** 2026-07-28
**Domain:** Task Graph — DAG model, grants, AI planning, agent execution
**Feature:** F37
**ADR:** ADR-010
**HLD Ref:** C2.16, C3.11b, C4.9
**Source files (to create):**
- `src/main/task/TaskService.ts`
- `src/main/task/TaskDAGValidator.ts`
- `src/main/task/TaskGrantService.ts`
- `src/main/task/TaskAIPlanner.ts`
- `src/main/task/TaskAgentExecutor.ts`
- `src/main/runtime/rpc/methods/task.ts`
- `src/main/db/migrations/0010_tasks.ts`

> **Status: ❌ TODO** — v5.0 proposed; ADR-010: dual-edge DAG + BFS grant resolution

---

## 1. Mục tiêu

Quản lý tasks theo mô hình graph với:
- **Parent-child** (decomposition): Epic → Stories → Tasks → Subtasks
- **Dependency edges** (depends-on): Task B chờ Task A done
- **Grant cascade**: grants chạy BFS qua subtree
- **AI planning**: AI decompose task thành subtasks
- **Agent execution**: spawn agent với task context

---

## 2. TaskService

```typescript
// src/main/task/TaskService.ts

export class TaskService {
  constructor(
    private readonly pool: IConnectionPool,
    private readonly dagValidator: TaskDAGValidator
  ) {}

  // CRUD
  async create(params: Omit<OrcaTask, 'id'|'createdAt'|'updatedAt'|'progressPercent'>): Promise<OrcaTask>
  async get(taskId: string): Promise<OrcaTask | null>
  async update(taskId: string, patch: Partial<OrcaTask>): Promise<void>
  async delete(taskId: string): Promise<void>

  // Tree operations
  async getChildren(taskId: string): Promise<OrcaTask[]>
  async getAncestors(taskId: string): Promise<OrcaTask[]>   // BFS up parentId chain
  async getSubtree(taskId: string): Promise<OrcaTask[]>     // BFS down

  // Dependency edges
  async addEdge(fromTaskId: string, toTaskId: string, edgeType: string): Promise<void> {
    // Validate no cycle before insert
    const wouldCycle = await this.dagValidator.detectCycle(fromTaskId, toTaskId)
    if (wouldCycle) throw new Error('TASK_DEPENDENCY_CYCLE')
    await this.pool.query(`
      INSERT INTO orca_task_edges (from_task_id, to_task_id, edge_type) VALUES (?, ?, ?)
    `, [fromTaskId, toTaskId, edgeType])
  }
  async removeEdge(fromTaskId: string, toTaskId: string, edgeType: string): Promise<void>
  async getDependencies(taskId: string): Promise<{ task: OrcaTask; edgeType: string }[]>
  async getDependents(taskId: string): Promise<{ task: OrcaTask; edgeType: string }[]>

  // Progress (computed — not stored, calculated from leaf task statuses)
  async recalculateProgress(taskId: string): Promise<number>

  // List with filters
  async list(filters: {
    projectId?: string
    assigneeId?: string
    reporterId?: string
    status?: string[]
    parentId?: string | null
  }): Promise<OrcaTask[]>
}
```

---

## 3. TaskDAGValidator — BFS Cycle Detection

```typescript
// src/main/task/TaskDAGValidator.ts

export class TaskDAGValidator {
  constructor(private readonly pool: IConnectionPool) {}

  /**
   * BFS from `toTaskId` — if we can reach `fromTaskId`, adding this edge would create a cycle.
   * Uses orca_task_edges table for dependency edges.
   */
  async detectCycle(fromTaskId: string, toTaskId: string): Promise<boolean> {
    const visited = new Set<string>()
    const queue = [toTaskId]

    while (queue.length > 0) {
      const current = queue.shift()!
      if (current === fromTaskId) return true  // cycle!
      if (visited.has(current)) continue
      visited.add(current)

      // Load successors (tasks that depend on `current`)
      const successors = await this.pool.query<{ toTaskId: string }>(
        `SELECT to_task_id as toTaskId FROM orca_task_edges WHERE from_task_id = ?`,
        [current]
      )
      queue.push(...successors.map(r => r.toTaskId))
    }

    return false
  }
}
```

---

## 4. TaskGrantService — BFS Ancestor Grant Resolution

```typescript
// src/main/task/TaskGrantService.ts

export type TaskPermission = 'view' | 'comment' | 'edit' | 'execute' | 'manage'
const PERMISSION_LEVELS: Record<TaskPermission, number> = {
  view: 1, comment: 2, edit: 3, execute: 4, manage: 5
}

export class TaskGrantService {
  constructor(
    private readonly pool: IConnectionPool,
    private readonly taskService: TaskService
  ) {}

  /**
   * Resolve effective permission for user on task.
   * BFS up parentId chain to find cascade grants.
   * Priority: direct > ancestor
   */
  async resolvePermission(userId: string, taskId: string): Promise<TaskPermission | null> {
    const task = await this.taskService.get(taskId)
    if (!task) return null

    // Reporter always has 'manage'
    if (task.reporterId === userId) return 'manage'
    // Assignee always has 'edit'
    if (task.assigneeId === userId) return 'edit'

    // Walk task + ancestor chain
    const taskChain = [task, ...await this.taskService.getAncestors(taskId)]
    const userTeams = await this.getUserTeams(userId)

    let bestPermission: TaskPermission | null = null

    for (const ancestor of taskChain) {
      const grants = await this.pool.query<{
        scope: string; scopeId: string | null;
        permission: TaskPermission; applyTree: number
      }>(
        `SELECT scope, scope_id as scopeId, permission, apply_tree as applyTree
         FROM orca_task_grants WHERE task_id = ?`,
        [ancestor.id]
      )

      for (const grant of grants) {
        // For ancestor tasks, only cascade grants count
        if (ancestor.id !== taskId && !grant.applyTree) continue
        if (!this.grantMatchesUser(grant, userId, userTeams)) continue

        // Select highest permission
        if (!bestPermission || PERMISSION_LEVELS[grant.permission] > PERMISSION_LEVELS[bestPermission]) {
          bestPermission = grant.permission
        }
      }
    }

    return bestPermission
  }

  async assertPermission(userId: string, taskId: string, required: TaskPermission): Promise<void> {
    const resolved = await this.resolvePermission(userId, taskId)
    if (!resolved || PERMISSION_LEVELS[resolved] < PERMISSION_LEVELS[required]) {
      throw new Error('TASK_ACCESS_DENIED')
    }
  }

  async grantAccess(params: {
    taskId: string; scope: string; scopeId?: string;
    permission: TaskPermission; applyTree?: boolean; grantedBy: string; expiresAt?: Date
  }): Promise<void>

  async revokeAccess(grantId: string): Promise<void>

  private grantMatchesUser(
    grant: { scope: string; scopeId: string | null },
    userId: string,
    userTeams: string[]
  ): boolean {
    if (grant.scope === 'company') return true
    if (grant.scope === 'user') return grant.scopeId === userId
    if (grant.scope === 'team') return userTeams.includes(grant.scopeId!)
    return false
  }

  private async getUserTeams(userId: string): Promise<string[]> {
    const rows = await this.pool.query<{ teamId: string }>(
      `SELECT team_id as teamId FROM orca_team_members WHERE user_id = ?`, [userId]
    )
    return rows.map(r => r.teamId)
  }
}
```

---

## 5. TaskAIPlanner — AI-Powered Task Decomposition

```typescript
// src/main/task/TaskAIPlanner.ts

interface SubtaskSuggestion {
  title: string
  description?: string
  type: OrcaTask['type']
  estimatedHours?: number
  dependsOn?: string[]   // relative indices in suggestions array
  promptTemplate?: string
}

export class TaskAIPlanner {
  constructor(
    private readonly providerResolver: ProviderResolver,
    private readonly taskService: TaskService,
    private readonly router: ProjectServerRouter
  ) {}

  async decomposeTask(
    task: OrcaTask,
    requestedBy: string
  ): Promise<SubtaskSuggestion[]> {
    if (!task.projectId) throw new Error('TASK_NO_PROJECT')

    const project = await this.router.getProject(task.projectId)
    const account = await this.providerResolver.resolve({
      devServerId: project.devServerId,
      projectId: task.projectId,
      userId: requestedBy,
    })

    const prompt = this.buildDecomposePrompt(task)
    const relay = await this.router.getRelayForProject(task.projectId, requestedBy)
    const response = await relay.call('ai.complete', {
      accountId: account.id,
      prompt,
      maxTokens: 1500,
      temperature: 0.3,
    }) as { text: string }

    return this.parseSubtaskSuggestions(response.text)
  }

  async applyDecomposition(
    parentTaskId: string,
    suggestions: SubtaskSuggestion[],
    createdBy: string
  ): Promise<OrcaTask[]> {
    const parent = await this.taskService.get(parentTaskId)!
    const created: OrcaTask[] = []

    for (const suggestion of suggestions) {
      const subtask = await this.taskService.create({
        ...suggestion,
        parentId: parentTaskId,
        projectId: parent.projectId,
        reporterId: createdBy,
        status: 'backlog',
        priority: parent.priority,
        labels: parent.labels,
        visibility: parent.visibility,
      })
      created.push(subtask)
    }

    // Create dependency edges between subtasks
    for (let i = 0; i < suggestions.length; i++) {
      for (const depIdx of suggestions[i].dependsOn ?? []) {
        await this.taskService.addEdge(created[depIdx].id, created[i].id, 'depends_on')
      }
    }

    return created
  }

  private buildDecomposePrompt(task: OrcaTask): string {
    return `You are an expert technical project manager. Decompose the following task into smaller, actionable subtasks.

Task: ${task.title}
Description: ${task.description ?? 'none'}
Additional Context: ${task.aiContext ?? 'none'}

Return a JSON array of subtasks with fields: title, description, type (task|subtask|bug|spike), estimatedHours, dependsOn (array of indices into your response), promptTemplate.

Example:
[
  { "title": "Setup DB migration", "type": "subtask", "estimatedHours": 1, "dependsOn": [] },
  { "title": "Implement API endpoint", "type": "subtask", "estimatedHours": 3, "dependsOn": [0] }
]`
  }

  private parseSubtaskSuggestions(responseText: string): SubtaskSuggestion[] {
    const jsonMatch = responseText.match(/\[[\s\S]*\]/)
    if (!jsonMatch) return []
    try {
      return JSON.parse(jsonMatch[0]) as SubtaskSuggestion[]
    } catch {
      return []
    }
  }
}
```

---

## 6. TaskAgentExecutor

```typescript
// src/main/task/TaskAgentExecutor.ts

export class TaskAgentExecutor {
  constructor(
    private readonly taskService: TaskService,
    private readonly spawner: ProfileAwareAgentSpawner,
    private readonly grantService: TaskGrantService
  ) {}

  async executeTask(taskId: string, requestedBy: string): Promise<void> {
    await this.grantService.assertPermission(requestedBy, taskId, 'execute')
    const task = await this.taskService.get(taskId)
    if (!task) throw new Error('TASK_NOT_FOUND')
    if (!task.projectId) throw new Error('TASK_NO_PROJECT')

    // Build context from parent chain
    const ancestors = await this.taskService.getAncestors(taskId)
    const preamble = this.buildPreamble(task, ancestors)

    // Interpolate prompt template
    const prompt = interpolateTemplate(task.promptTemplate ?? '{{task.title}}\n{{task.description}}', {
      task: { title: task.title, description: task.description ?? '' },
    })

    await this.taskService.update(taskId, { status: 'in_progress' })

    try {
      await this.spawner.spawn({
        projectId: task.projectId,
        userId: requestedBy,
        worktreePath: '.',  // task doesn't specify worktree — uses project default
        prompt: preamble + '\n\n' + prompt,
        taskId: task.id,
      })
      await this.taskService.update(taskId, { status: 'review' })
    } catch (err) {
      await this.taskService.update(taskId, { status: 'blocked' })
      throw err
    }
  }

  private buildPreamble(task: OrcaTask, ancestors: OrcaTask[]): string {
    const breadcrumb = [...ancestors.reverse(), task]
      .map(t => `[${t.type.toUpperCase()}] ${t.title}`)
      .join(' > ')
    return `Context: ${breadcrumb}\n\nYou are working on task: "${task.title}"\n${task.description ?? ''}`
  }
}
```

---

## 7. Progress Calculation

```typescript
// In TaskService — recursive calculation
async recalculateProgress(taskId: string): Promise<number> {
  const children = await this.getChildren(taskId)
  if (children.length === 0) {
    const task = await this.get(taskId)!
    const progress = task.status === 'done' ? 100
                   : task.status === 'review' ? 80
                   : task.status === 'in_progress' ? 40
                   : 0
    await this.update(taskId, { progressPercent: progress })
    return progress
  }

  const childProgresses = await Promise.all(children.map(c => this.recalculateProgress(c.id)))
  const avg = Math.round(childProgresses.reduce((s, p) => s + p, 0) / children.length)
  await this.update(taskId, { progressPercent: avg })
  return avg
}
```

---

## 8. RPC Methods

```typescript
// namespace: 'task'

'task.create'              // (project member) → OrcaTask
'task.get'                 // (view perm) → OrcaTask
'task.update'              // (edit perm) → void
'task.delete'              // (manage perm) → void
'task.list'                // (view perm) → OrcaTask[] with filters
'task.getSubtree'          // (view perm) → OrcaTask[]
'task.addEdge'             // (edit perm) → void
'task.removeEdge'          // (edit perm) → void
'task.getDependencies'     // (view perm) → { task, edgeType }[]
'task.grantAccess'         // (manage perm) → void
'task.revokeAccess'        // (manage perm) → void
'task.decomposeWithAI'     // (edit perm) → SubtaskSuggestion[]
'task.applyDecomposition'  // (edit perm) → OrcaTask[] created subtasks
'task.execute'             // (execute perm) → void (spawn agent)
'task.addComment'          // (comment perm) → void
'task.getComments'         // (view perm) → TaskComment[]
'task.recalculateProgress' // (view perm) → number
```

---

## 9. Test Coverage

```
src/main/task/__tests__/
├── TaskService.test.ts
│   ├── create → stored with correct fields
│   ├── addEdge: valid → inserted
│   ├── addEdge: cycle → TASK_DEPENDENCY_CYCLE
│   ├── getAncestors: 3-level chain
│   └── recalculateProgress: leaf done → 100, parent avg
├── TaskDAGValidator.test.ts
│   ├── detectCycle: simple A→B, adding B→A = cycle
│   ├── detectCycle: diamond (no cycle)
│   └── detectCycle: long chain (no cycle)
├── TaskGrantService.test.ts
│   ├── reporter → manage
│   ├── direct user grant → permission returned
│   ├── ancestor grant applyTree=true → cascades
│   ├── ancestor grant applyTree=false → NOT cascaded
│   ├── team grant → matches if user in team
│   └── company grant → always matches
├── TaskAIPlanner.test.ts
│   ├── buildDecomposePrompt: includes title + context
│   ├── parseSubtaskSuggestions: valid JSON → suggestions
│   ├── parseSubtaskSuggestions: invalid JSON → []
│   └── applyDecomposition: creates subtasks + edges
├── TaskAgentExecutor.test.ts
│   ├── executeTask: status in_progress → spawner called → status review
│   ├── executeTask: spawner throws → status blocked
│   └── buildPreamble: ancestor breadcrumb included
└── task-rpc.test.ts
    ├── task.create (project member OK)
    ├── task.execute (execute perm OK, view perm 403)
    └── task.decomposeWithAI (edit perm OK)
```

**Target:** ≥ 50 tests
