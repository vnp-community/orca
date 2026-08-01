# ADR-010 — Task Graph as DAG with BFS Access Control

| Trường | Giá trị |
|--------|---------|
| **ID** | ADR-010 |
| **Trạng thái** | 🚧 Proposed |
| **Ngày** | 2026-07-28 |
| **HLD Ref** | C2.16 (Task Graph Service), C3.11c |
| **Code Ref** | Cần tạo: `src/main/task/` |
| **Feature Ref** | F37 |

---

## Bối cảnh

Task management cần hỗ trợ 2 loại relationship:
1. **Parent-child** (decomposition): Epic → Stories → Tasks → Subtasks
2. **Dependency** (depends-on): Task B không thể bắt đầu cho đến khi Task A done

Truyền thống dùng tree (chỉ parent-child) nhưng không model được dependency giữa tasks trong cùng level. Cần **DAG** (directed acyclic graph) cho cả 2 loại.

**Access control challenge:** Task grants phải cascade xuống subtask tree mà không denormalize (không copy grant vào mỗi subtask — quá tốn storage và phức tạp khi revoke).

---

## Quyết định

### Dual-edge DAG Model

```typescript
// orca_tasks table
interface OrcaTask {
  id: string              // UUID
  type: 'epic' | 'story' | 'task' | 'subtask' | 'bug' | 'spike'
  title: string
  description?: string
  status: 'backlog' | 'todo' | 'in_progress' | 'blocked' | 'review' | 'done' | 'cancelled'
  priority: 'critical' | 'high' | 'medium' | 'low'
  parentId?: string       // for tree structure
  assigneeId?: string
  reporterId: string
  projectId?: string
  estimatedHours?: number
  actualHours?: number
  labels: string[]
  promptTemplate?: string     // template for AI agent
  aiContext?: string           // additional AI context
  visibility: 'private' | 'team' | 'company' | 'public'
  progressPercent: number     // computed from subtasks
  createdAt: Date
  updatedAt: Date
}

// orca_task_edges table (dependency edges only, not parent-child)
interface TaskEdge {
  fromTaskId: string
  toTaskId: string
  edgeType: 'depends_on' | 'blocks' | 'relates_to'
}
```

### Cycle Detection (BFS)

```typescript
function detectCycle(tasks: Map<string, string[]>, newEdge: TaskEdge): boolean {
  // BFS from newEdge.toTaskId
  // If we can reach newEdge.fromTaskId → cycle detected
  const visited = new Set<string>()
  const queue = [newEdge.toTaskId]
  while (queue.length) {
    const current = queue.shift()!
    if (current === newEdge.fromTaskId) return true  // cycle!
    if (visited.has(current)) continue
    visited.add(current)
    const deps = tasks.get(current) ?? []
    queue.push(...deps)
  }
  return false
}
```

### Access Control: Grant Resolution (no denormalization)

```typescript
// orca_task_grants table
interface TaskGrant {
  id: string
  taskId: string
  scope: 'user' | 'team' | 'company'
  scopeId?: string          // userId or teamId (null for company-wide)
  permission: 'view' | 'comment' | 'edit' | 'execute' | 'manage'
  applyTree: boolean        // if true: grant cascades to all descendants
  expiresAt?: Date
}

// Grant resolution (at query time, not stored):
function resolveGrant(userId: string, task: OrcaTask): Permission | null {
  // Priority order:
  // 1. task.reporterId === userId → 'manage'
  // 2. Direct user grant on this task
  // 3. User grant with applyTree=true on any ancestor
  // 4. Team grant (user is member) on this task or ancestor
  // 5. Company grant on this task or ancestor
  // 6. null → no access

  // Uses BFS up the parent_id chain to find ancestor grants
}
```

**Why no denormalization?** Khi revoke grant, chỉ cần delete 1 row thay vì update N descendant rows. Trade-off: query phức tạp hơn (walk parent chain), được giải quyết bằng caching.

### Progress Calculation (Recursive)

```typescript
async function calculateProgress(taskId: string): Promise<number> {
  const children = await getDirectChildren(taskId)
  if (children.length === 0) {
    // Leaf task: 100 if done, 0 otherwise
    return task.status === 'done' ? 100 : 0
  }
  const childProgresses = await Promise.all(children.map(c => calculateProgress(c.id)))
  return childProgresses.reduce((sum, p) => sum + p, 0) / children.length
}
```

### AI Planning Integration

```typescript
async function decomposeTask(task: OrcaTask): Promise<SubtaskSuggestion[]> {
  const prompt = buildDecomposePrompt(task)  // title + desc + aiContext + project tech stack
  const provider = await ProviderResolver.resolve({ devServerId: task.projectDevServerId })
  const response = await relay.call('ai.complete', {
    prompt, maxTokens: 1000, accountId: provider.id
  })
  return parseSubtaskSuggestions(response)
  // Returns: [{ title, description, type, estimatedHours, dependsOn }]
}
```

### Agent Execution from Task

```typescript
async function runAgentForTask(task: OrcaTask, worktree: Worktree): Promise<void> {
  // Build task preamble
  const preamble = buildTaskPreamble(task)  // parent context + deps outputs
  const prompt = interpolateTemplate(task.promptTemplate, { task, project, worktree })

  // Spawn agent với task env
  await agentExec({
    worktreePath: worktree.path,
    devServerId: worktree.devServerId,
    env: {
      ORCA_TASK_ID: task.id,
      ORCA_TASK_CONTEXT: preamble,
      ORCA_MODEL: resolvedProfile.agent.preferredModel,
    },
    prompt: preamble + '\n\n' + prompt,
    onComplete: () => {
      TaskService.update(task.id, { status: 'review' })
      recordActualHours(task.id, agentSession.durationMs / 3_600_000)
    }
  })
}
```

---

## Lý do chọn

| Lựa chọn | Đánh giá |
|----------|---------|
| **Dual-edge DAG (parent-child + depends-on)** ✅ | Flexible; models real project structures; no tree-only limitations |
| Tree only (parent-child) | Cannot model cross-branch dependencies |
| Linear backlog | No hierarchy; no dependencies |
| External tools (Jira, Linear) | External dep; no native AI integration |
| Denormalize grants | Simple query but expensive revoke + storage |

---

## Hậu quả

**Tích cực:**
- Modular: parent-child (`parentId`) và dependency (`orca_task_edges`) là separate concerns
- Grant resolution từ DB không duplicate data
- AI decompose tạo entire subtree với dependencies
- Progress calculation tự động từ subtask completion

**Tiêu cực:**
- BFS cycle detection phải chạy trước mỗi `INSERT INTO orca_task_edges`
- Grant resolution walk parent chain → cần index trên `parentId`
- Progress calculation recursive → cần cache hoặc materialized view cho large trees
- `applyTree=true` grant resolution requires full ancestor scan

---

## Trạng thái Implementation

❌ Chưa implement (v5.0 proposed)  
🎯 Migration 0010 (`orca_tasks`, `orca_task_edges`, `orca_task_grants`, `orca_task_comments`)  
🎯 `src/main/task/TaskService.ts`  
🎯 `src/main/task/TaskDAGValidator.ts`  
🎯 `src/main/task/TaskGrantService.ts`  
🎯 `src/main/task/TaskAIPlanner.ts`  
🎯 `src/main/task/TaskAgentExecutor.ts`
