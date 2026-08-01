# Task → Agent → Git → PR: End-to-End Flow — F37 Task Graph Management

> **Scope**: Luồng đầy đủ từ tạo task → AI decompose → agent thực thi → git commit → tạo PR
>
> **Key files**:
> - [`src/main/task/task-service.ts`](../../src/main/task/task-service.ts) — TaskService CRUD
> - [`src/main/task/task-dag-validator.ts`](../../src/main/task/task-dag-validator.ts) — Cycle detection, dependency management
> - [`src/main/task/task-ai-planner.ts`](../../src/main/task/task-ai-planner.ts) — AI decomposition (subtasks + dependency graph)
> - [`src/main/task/task-grant-service.ts`](../../src/main/task/task-grant-service.ts) — Grant resolution (owner>admin>user>team>company)
> - [`src/main/task/task-agent-executor.ts`](../../src/main/task/task-agent-executor.ts) — Build preamble + spawn agent + stream
> - **Feature**: [F37 Task Graph Management](../features/F37-task-graph-management.md)
> - **Business Logic**: [BL-TG-01](../logic/task-graph/BL-TG-01-task-graph-crud.md), [BL-TG-02](../logic/task-graph/BL-TG-02-ai-task-planning.md), [BL-TG-03](../logic/task-graph/BL-TG-03-task-access-control.md), [BL-TG-04](../logic/task-graph/BL-TG-04-task-agent-execution.md)

---

## 1. Tổng quan — Task Graph Lifecycle

```
Lead tạo Epic Task
    │
    ├── [Optional] AI Decompose → Subtasks + Dependency Graph
    │
    ├── Developer nhận Task assignment
    │
    ├── [Run Agent] → TaskAgentExecutor
    │   ├── Build context preamble
    │   ├── ProfileAwareAgentSpawner.spawn()
    │   └── Stream output → ActivityFeed
    │
    ├── Agent completes → WorkspaceEvent 'agent.complete'
    │   ├── GitPanel auto-refresh
    │   └── ExplorerPanel refresh decorations
    │
    ├── Developer: [Stage] → [AI: Generate commit msg] → [Commit & Push]
    │
    ├── Developer: [Create PR]
    │   → gh CLI on dev server → PR URL
    │
    └── Task.prUrl = PR.url
        Task.status = 'review'
```

---

## 2. Task Data Model

```typescript
// orca_tasks table (Migration 0010)
interface OrcaTask {
  id:           string
  projectId:    string
  parentId?:    string      // null for root tasks
  title:        string
  description?: string
  status:       'todo' | 'in_progress' | 'review' | 'done' | 'blocked'
  priority:     'p0' | 'p1' | 'p2' | 'p3'
  assigneeId?:  string
  reporterId:   string
  worktreeId?:  string      // linked worktree
  prUrl?:       string      // GitHub/GitLab PR URL
  estimate?:    number      // hours
  actualHours?: number
  tags:         string[]    // JSON array
  createdAt:    number
  updatedAt:    number
}

// orca_task_edges (dependencies)
interface TaskEdge {
  fromTaskId: string   // predecessor
  toTaskId:   string   // successor (blocked by fromTask)
  type: 'blocks' | 'relates_to'
}

// orca_task_grants (RBAC)
interface TaskGrant {
  taskId:   string
  grantee:  string    // userId | teamId | 'company' | 'admin'
  role:     'owner' | 'admin' | 'user' | 'viewer'
  applyTree: boolean  // apply_tree: inherit to subtasks
}

// orca_task_comments
interface TaskComment {
  id:        string
  taskId:    string
  authorId:  string
  content:   string
  isInternal: boolean  // false = visible to all; true = team-only
  agentSessionId?: string  // nếu comment từ agent output
  createdAt: number
}
```

---

## 3. Flow: Tạo Task + AI Decompose

### 3.1 Lead tạo Task

```
Lead mở Task Board → [+ New Task]
    │
    ▼ RPC: tasks.create({
    │   projectId: 'proj-abc',
    │   title: 'Implement blockchain transaction validation',
    │   description: '...',
    │   priority: 'p0',
    │ })
    │
    ├── TaskService.create(input)
    │   → id = generateTaskId()
    │   → INSERT orca_tasks (id, projectId, title, status='todo', ...)
    │   → TaskGrantService.grantOwner(id, lead.userId)
    │   → Audit log: task.create
    │
    └── return OrcaTask { id, status: 'todo', ... }
```

### 3.2 AI Decompose → Subtasks

```
Lead click [AI Decompose]
    │
    ▼ RPC: tasks.aiDecompose({ taskId, depth: 2 })
    │
    ├── TaskAIPlanner.decompose(task)
    │   │
    │   ├── Build prompt:
    │   │   "Decompose this task into subtasks:
    │   │    Title: {task.title}
    │   │    Description: {task.description}
    │   │    Project context: {project.description}
    │   │    Return JSON: { subtasks: [{ title, description, estimate, dependencies }] }"
    │   │
    │   ├── ProviderResolver.resolve(leadId, projectId, 'anthropic')
    │   │   → apiKey from ~/.orca/ai-providers/<id>.enc
    │   │
    │   ├── Call LLM API (anthropic messages.create)
    │   │   → response: {
    │   │       subtasks: [
    │   │         { title: 'Parse transaction input', estimate: 4, dependencies: [] },
    │   │         { title: 'Validate ECDSA signature', estimate: 8, dependencies: [0] },
    │   │         { title: 'Update ledger state', estimate: 6, dependencies: [1] },
    │   │       ]
    │   │     }
    │   │
    │   └── TaskService.batchCreate(subtasks, parentId=taskId)
    │       TaskDAGValidator.validate(allTasks, edges)
    │       → cycle detection (DFS)
    │       → auto-block tasks với unresolved deps
    │
    └── return { taskId, subtasks: OrcaTask[] }
```

---

## 4. Flow: Developer Chạy Agent

### 4.1 Task → Agent

```
Developer mở Task "Validate ECDSA signature"
    │
    ├── Task dependencies resolved? → YES (Parse transaction done)
    │
    ▼ Click [Run Agent]
    │
    ▼ RPC: tasks.runAgent({ taskId })
    │
    ├── TaskGrantService.checkPermission(taskId, userId, 'user')
    │   → resolve chain: owner > admin > user > team > company
    │   → GRANTED
    │
    ├── TaskAgentExecutor.execute(task, userId)
    │   │
    │   ├── Build preamble (context injection):
    │   │   """
    │   │   # Task Context
    │   │   Task: Validate ECDSA signature
    │   │   Project: vnp-blc-backend (repo: /srv/vnp)
    │   │   Description: {task.description}
    │   │   Dependencies done: [Parse transaction input]
    │   │   Worktree: /srv/vnp/worktrees/task-{taskId}
    │   │   Branch: task/{taskId}-ecdsa-validation
    │   │   """
    │   │
    │   ├── ProfileResolver.resolve(userId)
    │   │   → ResolvedProfile { agent.preferredModel: 'claude', trustPreset: 'standard' }
    │   │
    │   ├── ProfileAwareAgentSpawner.spawn({
    │   │     userId, projectId, taskId,
    │   │     worktreePath: '/srv/vnp/worktrees/task-{taskId}',
    │   │     preamble: contextPreamble,
    │   │     profile: resolvedProfile,
    │   │   })
    │   │   → relay.call('pty.spawn', { binary: 'claude', args: [...], env: {...} })
    │   │
    │   └── task.status → 'in_progress'
    │       task.assigneeId → userId
    │       task.worktreeId → created worktree id
    │
    └── Stream: PTY output → ActivityFeed + WorkspaceTerminal
```

### 4.2 Agent Completion → Workspace Events

```
Agent output: "Task complete. Modified 12 files."
Agent exits (OSC state: 'completed')
    │
    ▼ WorkspaceEvent: { type: 'agent.complete', filesChanged: 12 }
    │
    ├── GitPanel.refresh()
    │   → relay.call('git.status', { cwd: worktreePath })
    │   → Shows: 12 modified files (staged/unstaged)
    │
    ├── ExplorerPanel.refreshDecorations()
    │   → Updated file badges (M = modified, A = added)
    │
    └── Notification: "Agent completed: 12 files changed"
        (+ push notification if mobile paired)
```

---

## 5. Flow: Commit + Push + Create PR

### 5.1 Stage + AI Commit Message

```
Developer: [Stage All] → [AI: Generate commit message]
    │
    ├── RPC: git.add({ cwd: worktreePath, files: ['.'] })
    │   → relay: git add .
    │
    ├── RPC: git.generateCommitMessage({ cwd: worktreePath })
    │   → relay: git diff --staged
    │   → LLM call với diff content:
    │     "Generate a conventional commit message for this diff:
    │      {diff output truncated to 4000 chars}"
    │   → response: "feat(crypto): validate ECDSA signature for transactions"
    │
    └── CommitForm: pre-filled message (user can edit)
```

### 5.2 Commit & Push

```
Developer: [Commit & Push]
    │
    ├── RPC: git.commit({
    │     cwd: worktreePath,
    │     message: 'feat(crypto): validate ECDSA signature for transactions',
    │     author: { name: user.name, email: user.email }
    │   })
    │   → relay: git commit -m "..." --author="Name <email>"
    │
    ├── RPC: git.push({
    │     cwd: worktreePath,
    │     remote: 'origin',
    │     branch: 'task/{taskId}-ecdsa-validation'
    │   })
    │   → relay.callStream('git.push', { cwd, remote, branch })
    │   → Stream progress:
    │     "Counting objects: 15, done."
    │     "Writing objects: 100% (15/15), 8.5 KiB"
    │     "Branch 'task/...' set up to track origin"
    │   → { type: 'git.push.done', success: true }
    │
    └── WorkspaceContext.emit('git.push', { branch })
        GitPanel: gitStatus refresh (ahead/behind = 0)
```

### 5.3 Create Pull Request

```
Developer: [Create PR]
    │
    ├── UI: PullRequestForm
    │   └── [AI: Generate PR description]
    │       → relay: git log origin/main..HEAD --format="- %s %b"
    │       → LLM call → PR description draft
    │
    ├── Developer fills: title, reviewers, base branch
    │
    ├── RPC: git.createPR({
    │     cwd: worktreePath,
    │     title: 'feat: ECDSA signature validation',
    │     body: '...',
    │     base: 'main',
    │     reviewers: ['maya@company.com'],
    │   })
    │   → relay: execFile('gh', ['pr', 'create', '--title', ..., '--body', ..., '--reviewer', ...])
    │   → returns: { prUrl: 'https://github.com/org/repo/pull/42' }
    │
    └── TaskService.update(taskId, {
          prUrl: 'https://github.com/org/repo/pull/42',
          status: 'review',
          actualHours: elapsedHours,
        })
        → Task status: 'review'
        → Parent task progress update (aggregate % done)
        → Notify assignee + reporter via WebSocket push
```

---

## 6. Task Grant Resolution

```
Resolution chain (highest priority first):

owner   → creator of the task
admin   → project admin or server admin
user    → explicitly granted user
team    → team member grant (via team membership)
company → company-wide grant (lowest priority)

apply_tree = true: grant inherited by ALL subtasks of this task

Example:
  Task "Implement validation" (lead = owner)
    Grant: { grantee: 'dev-team', role: 'user', apply_tree: true }
    → All devs in dev-team can see + run agent on this task AND all its subtasks
```

---

## 7. DAG Validation

```typescript
// src/main/task/task-dag-validator.ts
class TaskDAGValidator {
  validate(tasks: OrcaTask[], edges: TaskEdge[]): ValidationResult {
    // Build adjacency list
    const adj = new Map<string, string[]>()
    for (const edge of edges) {
      if (!adj.has(edge.fromTaskId)) adj.set(edge.fromTaskId, [])
      adj.get(edge.fromTaskId)!.push(edge.toTaskId)
    }

    // DFS cycle detection
    const visited = new Set<string>()
    const inStack = new Set<string>()

    for (const task of tasks) {
      if (hasCycle(task.id, adj, visited, inStack)) {
        return { valid: false, error: `Circular dependency detected involving task ${task.id}` }
      }
    }

    // Auto-block tasks with unresolved dependencies
    const blockedTasks: string[] = []
    for (const edge of edges) {
      const predecessor = tasks.find(t => t.id === edge.fromTaskId)
      if (predecessor && predecessor.status !== 'done') {
        blockedTasks.push(edge.toTaskId)
      }
    }

    return { valid: true, blockedTasks }
  }
}
```

---

## 8. DB Schema (Migration 0010)

```sql
CREATE TABLE orca_tasks (
  id           TEXT PRIMARY KEY,
  project_id   TEXT NOT NULL REFERENCES orca_projects(id),
  parent_id    TEXT REFERENCES orca_tasks(id),
  title        TEXT NOT NULL,
  description  TEXT,
  status       TEXT DEFAULT 'todo',
  priority     TEXT DEFAULT 'p2',
  assignee_id  TEXT REFERENCES orca_users(id),
  reporter_id  TEXT NOT NULL REFERENCES orca_users(id),
  worktree_id  TEXT,
  pr_url       TEXT,
  estimate     REAL,
  actual_hours REAL,
  tags         TEXT DEFAULT '[]',
  created_at   INTEGER NOT NULL,
  updated_at   INTEGER NOT NULL
);
CREATE INDEX idx_tasks_project ON orca_tasks(project_id);
CREATE INDEX idx_tasks_parent  ON orca_tasks(parent_id);
CREATE INDEX idx_tasks_assignee ON orca_tasks(assignee_id);

CREATE TABLE orca_task_edges (
  from_task_id TEXT NOT NULL REFERENCES orca_tasks(id) ON DELETE CASCADE,
  to_task_id   TEXT NOT NULL REFERENCES orca_tasks(id) ON DELETE CASCADE,
  type         TEXT DEFAULT 'blocks',
  PRIMARY KEY (from_task_id, to_task_id)
);
CREATE INDEX idx_task_edges_to ON orca_task_edges(to_task_id);

CREATE TABLE orca_task_grants (
  task_id     TEXT NOT NULL REFERENCES orca_tasks(id) ON DELETE CASCADE,
  grantee     TEXT NOT NULL,   -- userId | teamId | 'company' | 'admin'
  role        TEXT NOT NULL,   -- owner | admin | user | viewer
  apply_tree  INTEGER DEFAULT 0,
  PRIMARY KEY (task_id, grantee)
);

CREATE TABLE orca_task_comments (
  id               TEXT PRIMARY KEY,
  task_id          TEXT NOT NULL REFERENCES orca_tasks(id) ON DELETE CASCADE,
  author_id        TEXT REFERENCES orca_users(id),
  content          TEXT NOT NULL,
  is_internal      INTEGER DEFAULT 0,
  agent_session_id TEXT,
  created_at       INTEGER NOT NULL
);
CREATE INDEX idx_task_comments_task ON orca_task_comments(task_id);
```

---

## 9. RPC Methods — tasks.*

```typescript
'tasks.list'          // (projectId, filters?) → OrcaTask[]
'tasks.get'           // (taskId) → OrcaTask
'tasks.create'        // (input) → OrcaTask
'tasks.update'        // (taskId, fields) → OrcaTask
'tasks.delete'        // (taskId) — soft delete
'tasks.addDependency' // (fromTaskId, toTaskId) → validate DAG
'tasks.removeDependency' // (fromTaskId, toTaskId)
'tasks.grant'         // (taskId, grantee, role, applyTree) — owner/admin
'tasks.revoke'        // (taskId, grantee)
'tasks.aiDecompose'   // (taskId, depth) → subtasks[] (AI generated)
'tasks.runAgent'      // (taskId, userId) — start agent execution
'tasks.stopAgent'     // (taskId) — interrupt agent
'tasks.addComment'    // (taskId, content, isInternal)
'tasks.getComments'   // (taskId) → TaskComment[]
'tasks.getSubtreeProgress' // (taskId) → { total, done, percentage }
```

---

## 10. Cross-References

| Resource | Mô tả |
|---|---|
| [profile-resolution.md](./profile-resolution.md) | Profile inject khi spawn agent |
| [project-workspace-switch.md](./project-workspace-switch.md) | Workspace context cần thiết trước |
| [workflow-orchestration.md](./workflow-orchestration.md) | Task có thể được trigger từ workflow step |
| [remote-git-ui.md](./remote-git-ui.md) | Git operations sau agent complete |
| **HLD C1 Flow 10** | Task → Agent → Git → PR (End-to-end) |
| **HLD C4.9** | Task Graph module detail |
| **HLD C2 Container 16** | Task Graph Service |
| **F37 Task Graph** | Feature spec |
| **BL-TG-01** | Task graph CRUD |
| **BL-TG-02** | AI task planning |
| **BL-TG-03** | Task access control (grant resolution) |
| **BL-TG-04** | Task agent execution |
