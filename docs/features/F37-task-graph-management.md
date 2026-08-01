# F37 — Task Graph Management System

| Trường | Giá trị |
|--------|---------|
| **ID** | F37 |
| **Tên** | Task Graph Management System |
| **Ưu tiên** | P0 |
| **Trạng thái** | 🚧 Phát triển |
| **Phiên bản** | v5.0+ |
| **ADR References** | ADR-010 |
| **HLD References** | C2.16, C3.11c |

---

## Mô tả

Orca hỗ trợ quản lý tác vụ theo **mô hình đồ thị (graph)** — mỗi task có thể có quan hệ cha-con (phân rã), quan hệ phụ thuộc (depends-on), và được AI hỗ trợ planning/decomposition. Mỗi task chứa đủ thông tin quản trị và **prompt field** để người dùng đưa ra yêu cầu cho AI agent thực thi. Toàn bộ task/task tree có thể được chia sẻ và phân quyền theo cấp company, team, hoặc cá nhân.

---

## Vấn đề cần giải quyết

- Các tính năng automation hiện tại không có cơ chế quản lý tác vụ có cấu trúc
- Tác vụ phức tạp không thể chia nhỏ có cấu trúc và track progress
- Không có AI planning: người dùng phải tự tay tạo từng subtask
- Không có cơ chế chia sẻ task tree để cộng tác
- Prompt cho agent bị rời rạc, không gắn liền với task context

---

## Data Model — Task Graph

### Task Node

```typescript
interface OrcaTask {
  // Identity
  id: string
  title: string
  description?: string          // Markdown mô tả chi tiết
  
  // Graph structure
  parentId?: string             // null = root task
  dependsOn: string[]           // taskIds phải hoàn thành trước
  subTaskIds: string[]          // children (computed)

  // Classification
  type: 'epic' | 'story' | 'task' | 'subtask' | 'bug' | 'spike'
  status: 'backlog' | 'todo' | 'in_progress' | 'blocked' | 'review' | 'done' | 'cancelled'
  priority: 'low' | 'medium' | 'high' | 'critical'
  labels: string[]

  // Assignment & Planning
  projectId?: string
  assigneeId?: string
  reporterId: string
  dueDate?: number              // timestamp
  estimatedHours?: number
  actualHours?: number

  // AI Fields
  promptTemplate?: string       // prompt template cho AI agent
  aiPlanJson?: string           // AI-generated breakdown (JSON)
  aiContext?: string            // additional context injected vào agent

  // Access Control
  visibility: 'private' | 'team' | 'company'
  ownerId: string
  grants: TaskGrant[]

  // Execution tracking
  worktreeId?: string           // linked worktree
  agentSessionId?: string       // running agent session
  workflowExecutionId?: string  // linked workflow

  // Audit
  createdBy: string
  createdAt: number
  updatedAt: number
}

interface TaskGrant {
  scope: 'company' | 'team' | 'user'
  teamId?: string
  userId?: string
  permission: 'view' | 'comment' | 'edit' | 'execute' | 'manage'
  grantedBy: string
  grantedAt: number
  expiresAt?: number           // optional: time-limited grant
}
```

### Graph Edge Types

```
Task Graph = DAG (Directed Acyclic Graph)

Edge type 1: Parent-Child (decomposition)
  Epic → Story → Task → Subtask
  "Build Auth System"
      ├── "Design DB schema"
      ├── "Implement login API"
      │       ├── "JWT generation"
      │       └── "bcrypt password hash"
      └── "Write integration tests"

Edge type 2: Dependency (depends-on)
  "Deploy to staging" ──depends_on──► "All tests pass"
  "Write docs"        ──depends_on──► "API finalized"

Both edges can coexist on the same task graph.
```

---

## Tính năng chi tiết

### 1. Task Graph View (UI)

```
Graph View / Tree View / Board View (togglable)

Graph View:
┌──────────────────────────────────────────────────────────────┐
│  [Epic] Build Auth System                          [●running] │
│       │                                                       │
│  ┌────┴──────────────────────────────┐                       │
│  │                                   │                       │
│  [Story] DB Schema     [Story] Login API        [Story] Tests│
│  ✅ Done               ● In Progress            ⏸ Blocked    │
│                              │                               │
│                    ┌─────────┴──────────┐                    │
│               [Task] JWT          [Task] bcrypt               │
│               ✅ Done             ● Running 🤖               │
│                                                              │
│         ──depends──► [Task] API Finalized (for docs task)   │
└──────────────────────────────────────────────────────────────┘
```

### 2. Task Detail Panel

```
Task: "Implement bcrypt password hash"
┌──────────────────────────────────────────────────────────────┐
│ Title: Implement bcrypt password hash        [● In Progress] │
│ Type: Task    Priority: High    Due: 2026-08-01              │
│ Assignee: @nguyen.van.a   Reporter: @lead.dev                │
│ Project: vnp-blc-backend → Server: dev-alpha                 │
│ Labels: [backend] [security] [auth]                          │
│                                                              │
│ Description:                                                 │
│ Implement bcrypt hashing for user passwords with 12 rounds.  │
│ Use the existing AuthManager.hashPassword() pattern.         │
│                                                              │
│ Depends on: [✅ JWT generation]                              │
│ Blocking: [⏸ Integration tests]                              │
│ Parent: [Login API Story]                                    │
│                                                              │
│ ── AI Prompt ─────────────────────────────────────────────── │
│ Implement bcrypt password hashing in src/auth/auth-manager.ts│
│ Use 12 rounds. Add unit tests in src/auth/auth.test.ts       │
│ Follow the existing hashPassword() signature.                │
│ {{task.description}} {{task.aiContext}}                      │
│                                                              │
│ [▶ Run Agent]  [📋 Copy Prompt]  [🔗 Create Worktree]        │
│                                                              │
│ ── Subtasks (2) ─────────────────────────────────────────── │
│ + Add Subtask   [AI: Suggest subtasks]                       │
│                                                              │
│ ── Activity ──────────────────────────────────────────────── │
│ 14:23  Agent started — claude-opus-4-5                       │
│ 14:25  Agent: "Reading src/auth/auth-manager.ts..."          │
│ 14:28  Agent: "Writing bcrypt implementation..."             │
└──────────────────────────────────────────────────────────────┘
```

### 3. AI-Assisted Planning & Decomposition

**Auto-decompose:**

```
User: "Build a complete user authentication system"
      + [AI: Plan this task]
    │
    ▼
AI (Claude) receives:
  - Task description
  - Project context (vnp-blc-backend, tech stack from repo analysis)
  - Existing code structure (from dev server)
  - Team velocity/capacity (from recent tasks)
    │
    ▼
AI returns:
  {
    "breakdown": [
      { "title": "DB Schema: users, sessions, tokens", "type": "story", "estimate": 2 },
      { "title": "AuthManager: hash, verify, token gen", "type": "story", "estimate": 4 },
      { "title": "Auth Routes: /login, /logout, /refresh", "type": "story", "estimate": 3 },
      { "title": "Integration Tests", "type": "story", "estimate": 2 },
    ],
    "dependencies": [
      { "from": "Auth Routes", "to": "DB Schema", "type": "depends_on" },
      { "from": "Integration Tests", "to": "Auth Routes", "type": "depends_on" }
    ],
    "totalEstimate": 11,
    "criticalPath": ["DB Schema", "AuthManager", "Auth Routes", "Integration Tests"]
  }
    │
    ▼
User reviews → Accept / Edit / Reject subtask suggestions
→ Accepted tasks inserted into graph
```

**AI Prompt Generation:**

```
User: "Generate agent prompt for this task"
    │
AI generates from task context:
  title + description + aiContext + project tech stack + coding patterns
    │
Output: ready-to-use prompt that user can edit before running agent
```

### 4. Task Sharing & Grants

**Grant Levels:**

| Permission | Mô tả |
|-----------|-------|
| `view` | Đọc task, xem comments, subtasks |
| `comment` | view + thêm comments |
| `edit` | comment + sửa title/desc/status/labels |
| `execute` | edit + Run Agent, create worktree |
| `manage` | execute + grant others, delete, share |

**Sharing Task Tree:**

```
Owner → Task [Epic] → Share → Grant Access
    │
    ├── Scope: company
    │   → ALL users thấy task này + toàn bộ subtree
    │
    ├── Scope: team (Backend Team)
    │   → chỉ Backend Team members thấy
    │
    ├── Scope: user (@nguyen.van.b)
    │   → chỉ 1 user cụ thể
    │
    ├── Apply to: this task only | this task + all subtasks (tree)
    │
    └── Permission: view | comment | edit | execute | manage
    
Access resolves by UNION:
  user sees task IF ANY grant applies:
    1. Direct grant to user
    2. Team grant (user is team member)
    3. Company grant
    4. User is owner
```

---

## Luồng người dùng đầy đủ

```
1. Lead tạo Epic: "Build Auth System" → assigns to project vnp-blc-backend
2. Click "AI: Plan this task" → AI suggests 4 stories + dependencies
3. Lead reviews, adjusts estimates, accepts → subtasks created in graph
4. Each story: Lead assigns to developer, writes aiContext
5. Developer mở Story → Task Detail → click "Run Agent"
   → agent spawn trên dev-alpha (project's server)
   → agent reads promptTemplate + aiContext + task.description
   → executes, streams output to Task Activity feed
6. Developer marks task Done → parent story auto-updates progress
7. Lead shares Epic tree với "company" scope (view) → all devs can see progress
8. External stakeholder: Lead shares link "view" → họ xem task board
```

---

## Tiêu chí chấp nhận

- [ ] Task CRUD với đủ metadata fields
- [ ] Graph model: parent-child (decomposition) + depends-on edges
- [ ] DAG validation: no cycles khi add dependency
- [ ] Graph View UI (zoomable, drag-drop)
- [ ] Tree View UI (collapsible hierarchy)
- [ ] Board View (Kanban-style per status)
- [ ] Task Detail Panel với tất cả fields + prompt section
- [ ] AI decompose: gửi task context → nhận subtask suggestions
- [ ] AI prompt generation từ task context
- [ ] "Run Agent" từ task → spawn agent trên project's dev server + inject task prompt
- [ ] Grant system: company/team/user + 5 permission levels
- [ ] "Apply to subtree" option khi grant
- [ ] Share link (public view)
- [ ] Grant inheritance: task inherits company-scope grants từ cha
- [ ] Progress tracking: task.progress = done_subtasks / total_subtasks
- [ ] Critical path highlighting
- [ ] Estimated vs actual hours tracking

---

## Yêu cầu kỹ thuật

| Component | File |
|-----------|------|
| Task types | `src/shared/task-types.ts` |
| Task service | `src/main/task/task-service.ts` |
| Graph builder | `src/main/task/task-graph-builder.ts` |
| DAG validator | `src/main/task/task-dag-validator.ts` |
| AI planner | `src/main/task/task-ai-planner.ts` |
| Grant service | `src/main/task/task-grant-service.ts` |
| Task executor | `src/main/task/task-agent-executor.ts` |
| DB migration | `src/main/db/migrations/0010_task_graph.ts` |
| RPC methods | `src/main/runtime/rpc/methods/tasks.ts` |
| Graph View UI | `src/renderer/src/components/task/TaskGraphView.tsx` |
| Tree View UI | `src/renderer/src/components/task/TaskTreeView.tsx` |
| Board View UI | `src/renderer/src/components/task/TaskBoardView.tsx` |
| Task Detail | `src/renderer/src/components/task/TaskDetailPanel.tsx` |
| Grant Modal | `src/renderer/src/components/task/TaskGrantModal.tsx` |
| AI Plan Modal | `src/renderer/src/components/task/AIPlanModal.tsx` |

**DB tables:** `orca_tasks`, `orca_task_edges`, `orca_task_grants`, `orca_task_comments`

---

## Metrics

| KPI | Mục tiêu |
|-----|----------|
| Graph load (100 tasks) | < 500ms |
| DAG cycle detection | < 10ms |
| AI decompose (avg task) | < 5s (LLM call) |
| "Run Agent" from task | < 3s to PTY active |
| Grant resolution | < 5ms (cached) |
