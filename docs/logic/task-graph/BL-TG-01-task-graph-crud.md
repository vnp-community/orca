# BL-TG-01 — Task Graph CRUD & Structural Management

| Trường | Giá trị |
|--------|---------|
| **Mã** | BL-TG-01 |
| **Tên** | Task Graph CRUD & Structural Management |
| **Domain** | Task Graph |
| **Actor** | Developer, Lead, Admin |
| **Priority** | P0 |

---

## Data Model

```sql
-- Migration 0010

CREATE TABLE orca_tasks (
  id                  TEXT PRIMARY KEY,
  title               TEXT NOT NULL,
  description         TEXT,
  type                TEXT DEFAULT 'task',     -- epic|story|task|subtask|bug|spike
  status              TEXT DEFAULT 'backlog',  -- backlog|todo|in_progress|blocked|review|done|cancelled
  priority            TEXT DEFAULT 'medium',   -- low|medium|high|critical
  labels              TEXT DEFAULT '[]',       -- JSON array
  parent_id           TEXT REFERENCES orca_tasks(id),
  project_id          TEXT REFERENCES orca_projects(id),
  assignee_id         TEXT REFERENCES orca_users(id),
  reporter_id         TEXT REFERENCES orca_users(id) NOT NULL,
  owner_id            TEXT REFERENCES orca_users(id) NOT NULL,
  due_date            INTEGER,
  estimated_hours     REAL,
  actual_hours        REAL,
  prompt_template     TEXT,                    -- AI agent prompt template
  ai_context          TEXT,                    -- extra context injected to agent
  ai_plan_json        TEXT,                    -- AI decomposition result
  visibility          TEXT DEFAULT 'private',  -- private|team|company
  worktree_id         TEXT,
  agent_session_id    TEXT,
  workflow_exec_id    TEXT,
  created_by          TEXT REFERENCES orca_users(id),
  created_at          INTEGER,
  updated_at          INTEGER
);

-- Dependency edges (depends-on, separate from parent-child)
CREATE TABLE orca_task_edges (
  from_task_id  TEXT REFERENCES orca_tasks(id) ON DELETE CASCADE,
  to_task_id    TEXT REFERENCES orca_tasks(id) ON DELETE CASCADE,
  edge_type     TEXT DEFAULT 'depends_on',   -- depends_on | blocks | relates_to
  created_at    INTEGER,
  PRIMARY KEY (from_task_id, to_task_id, edge_type)
);

CREATE TABLE orca_task_grants (
  id          TEXT PRIMARY KEY,
  task_id     TEXT REFERENCES orca_tasks(id) ON DELETE CASCADE,
  scope       TEXT NOT NULL,           -- company|team|user
  team_id     TEXT REFERENCES orca_departments(id),
  user_id     TEXT REFERENCES orca_users(id),
  permission  TEXT NOT NULL,           -- view|comment|edit|execute|manage
  apply_tree  INTEGER DEFAULT 0,       -- 1 = applies to all subtasks too
  granted_by  TEXT REFERENCES orca_users(id),
  granted_at  INTEGER,
  expires_at  INTEGER
);

CREATE TABLE orca_task_comments (
  id          TEXT PRIMARY KEY,
  task_id     TEXT REFERENCES orca_tasks(id) ON DELETE CASCADE,
  author_id   TEXT REFERENCES orca_users(id),
  content     TEXT NOT NULL,           -- Markdown
  created_at  INTEGER,
  updated_at  INTEGER
);

CREATE INDEX idx_tasks_parent ON orca_tasks(parent_id);
CREATE INDEX idx_tasks_project ON orca_tasks(project_id);
CREATE INDEX idx_tasks_assignee ON orca_tasks(assignee_id);
CREATE INDEX idx_task_edges_from ON orca_task_edges(from_task_id);
CREATE INDEX idx_task_edges_to ON orca_task_edges(to_task_id);
CREATE INDEX idx_task_grants_task ON orca_task_grants(task_id);
```

---

## Luồng: Tạo Task

```
User → Tasks → New Task / Add Subtask
    │
    ├── Input: title, type, priority, parentId?, projectId?
    ├── Validate:
    │   - title non-empty (max 500 chars)
    │   - type valid enum
    │   - parentId exists (if set) + user có quyền view parent
    │   - projectId exists (if set) + user là project member
    │
    ├── INSERT orca_tasks
    ├── IF parentId:
    │   - parent.subTaskIds auto-computed (no separate column — query based)
    │
    └── Emit { type: 'task.created', taskId, parentId }
```

---

## Luồng: Add Dependency Edge

```
User → Task A → "Depends on" → select Task B
    │
    ├── Validate:
    │   - Task A và B đều tồn tại
    │   - User có quyền edit Task A
    │   - No cycle check: BFS/DFS từ B, xem có đến A không
    │         IF reachable: reject "Circular dependency detected"
    │
    ├── INSERT orca_task_edges (from=A, to=B, type='depends_on')
    │
    └── Recalculate: IF Task A status='in_progress' AND Task B status != 'done'
                     THEN Task A.status = 'blocked' (auto-block)
```

---

## Luồng: Load Task Graph (subtree)

```typescript
async function loadTaskTree(rootId: string, userId: string): Promise<TaskGraph> {
  // BFS từ root, collect tất cả descendants
  const visited = new Set<string>()
  const queue = [rootId]
  const tasks: OrcaTask[] = []
  const edges: TaskEdge[] = []

  while (queue.length > 0) {
    const batchIds = queue.splice(0, 50)  // batch 50

    // Load tasks + check access
    const batch = await db.tasks.findMany({
      where: { id: { in: batchIds } },
    })

    for (const task of batch) {
      if (!hasTaskAccess(userId, task, 'view')) continue  // skip no-access
      tasks.push(task)
      visited.add(task.id)

      // Queue children
      const children = await db.tasks.findMany({ where: { parent_id: task.id } })
      for (const child of children) {
        if (!visited.has(child.id)) queue.push(child.id)
      }
    }
  }

  // Load all dependency edges within the subtree
  const taskIds = tasks.map(t => t.id)
  const depEdges = await db.taskEdges.findMany({
    where: {
      from_task_id: { in: taskIds },
      to_task_id: { in: taskIds }
    }
  })

  return { root: rootId, tasks, edges: depEdges }
}
```

---

## Progress Calculation

```typescript
function calculateProgress(taskId: string, allTasks: OrcaTask[]): number {
  const subtasks = allTasks.filter(t => t.parentId === taskId)
  if (subtasks.length === 0) return task.status === 'done' ? 100 : 0
  const doneCount = subtasks.filter(t => t.status === 'done').length
  return Math.round((doneCount / subtasks.length) * 100)
}

// Cascade: when leaf task → done, parent progress auto-recalculates
// When parent reaches 100% → parent.status suggestions 'done'
```

---

## Tiêu chí chấp nhận

- [ ] Task CRUD với đầy đủ fields
- [ ] Parent-child relationship (subTaskIds computed from query)
- [ ] Dependency edges (orca_task_edges): add/remove/list
- [ ] Cycle detection trước khi add dependency
- [ ] Auto-block task nếu dependency chưa done
- [ ] `loadTaskTree(rootId)` BFS với access filter
- [ ] `calculateProgress()` từ subtask completion
- [ ] Progress cascade lên parent
