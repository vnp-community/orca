# Luồng Dữ liệu — Task Graph Management

**Domain:** Task Graph Management  
**Nghiệp vụ:** BL-TG-01 → BL-TG-04  
**Kiến trúc tham chiếu:** HLD v1 — Task Graph Service (C3.11c), ADR-010, F37

---

## Thành phần tham gia

| Thành phần | Layer | Vai trò |
|------------|-------|---------|
| Developer/Lead Browser | UI | Task graph UI, kanban board |
| Orca Web Server | Backend | /api/tasks REST API |
| TaskService | Business Logic | CRUD tasks, progress aggregation |
| TaskDAGValidator | Business Logic | Cycle detection, dependency blocking |
| TaskGraphBuilder | Business Logic | BFS subtree traversal + access filter |
| TaskAIPlanner | Business Logic | AI decomposition (subtask + dependency) |
| TaskGrantService | Business Logic | Permission resolution (5-level) |
| TaskAgentExecutor | Business Logic | Build preamble + spawn agent |
| Server Database | Persistence | orca_tasks, orca_task_edges, orca_task_grants |
| Dev Server (relay) | Remote | Agent execution |

---

## BL-TG-01 — Task Graph CRUD & Structural Management

```
Developer/Lead
    │
    ▼
CREATE TASK:
[Browser] Tasks → "New Task" / "+ Add Subtask"
    Input: { title, type, priority, parentId?, projectId?, description }
    │ POST /api/tasks
    ▼
[TaskService.create()]
    ├─ Validate: title non-empty (max 500), type valid enum
    ├─ IF parentId: check user có quyền view parent (TaskGrantService)
    ├─ IF projectId: check user là project member
    ├─ INSERT orca_tasks { id, title, type, status, priority,
    │     parentId, projectId, owner_id=userId, reporter_id=userId }  ← DB
    ├─ IF parentId: auto-update parent progress (recursive)
    └─ emit: task:created { taskId }

ADD DEPENDENCY (Edge):
[Browser] drag task A "depends on" task B
    │ POST /api/tasks/:fromId/edges
    Body: { toId, edgeType: 'depends_on' | 'blocks' | 'relates_to' }
    ▼
[TaskDAGValidator.addEdge()]
    ├─ Check cycle: BFS từ toId → nếu đến fromId → REJECT (cycle)
    ├─ INSERT orca_task_edges { from_task_id, to_task_id, edge_type }  ← DB
    └─ IF 'depends_on': check to task status
        IF not done: UPDATE from task status → 'blocked'

UPDATE STATUS:
    PATCH /api/tasks/:id { status: 'in_progress' | 'done' | ... }
    ├─ UPDATE orca_tasks SET status=?, updated_at=?  ← DB
    ├─ IF status='done': check dependents → unblock them
    │   SELECT from_task_id FROM orca_task_edges WHERE to_task_id=? AND edge_type='depends_on'
    │   → check all deps done → UPDATE status='todo' for unblocked tasks
    └─ Recursive progress update for parent tasks

Luồng:
User → POST /api/tasks → TaskService → cycle check → Server DB (INSERT/UPDATE)
                       → TaskDAGValidator (dependency edges)
                       → auto-unblock downstream tasks
```

---

## BL-TG-02 — AI-Assisted Task Planning & Decomposition

```
Lead/Developer
    │
    ▼
[Browser] Task detail → "AI: Decompose" button
    │ POST /api/tasks/:id/ai-plan
    ▼
[TaskAIPlanner.decompose(taskId, userId)]
    ├─ Load task: SELECT * FROM orca_tasks WHERE id=?  ← DB
    ├─ Load context: parent task, project info, existing subtasks
    ├─ Build AI prompt:
    │   "Decompose this task into subtasks with dependencies:\n"
    │   "Task: <title>\nDescription: <description>\n"
    │   "Context: Project <name>, existing subtasks: <list>"
    ├─ ProfileAwareAgentSpawner: get provider + spawn agent
    │   (or use inline AI call via provider API directly)
    ├─ Agent output: JSON plan:
    │   { subtasks: [
    │       { title, type, estimatedHours, dependencies: [siblingId] },
    │       ...
    │     ],
    │     dependencyGraph: { A → [B, C], B → [D] }
    │   }
    ├─ UPDATE orca_tasks SET ai_plan_json=?  ← DB
    └─ Return plan for user review
    │
    ▼
[Browser] "AI Plan" preview panel:
    - Suggested subtasks list (user can edit before apply)
    - Dependency graph visualization
    │ User approves → POST /api/tasks/:id/ai-plan/apply
    ▼
[TaskAIPlanner.applyPlan()]
    ├─ FOR each subtask in plan:
    │   INSERT orca_tasks (subtask, parentId=taskId)
    ├─ FOR each dependency:
    │   INSERT orca_task_edges
    │   TaskDAGValidator.validate() (no cycles check)
    └─ emit: task:planApplied { taskId, subtaskCount }

Luồng:
User → POST /api/tasks/:id/ai-plan → TaskAIPlanner
     → Server DB (load task context)
     → AI call (agent spawn or direct API)
     → parse JSON plan
     → Server DB (UPDATE ai_plan_json)
     → Browser (preview plan)
User approves → POST /apply → Server DB (INSERT subtasks + edges)
```

---

## BL-TG-03 — Task Access Control & Sharing

```
Task Owner / Lead
    │
    ▼
GRANT ACCESS:
[Browser] Task → "Share" → add user/team/company
    │ POST /api/tasks/:id/grants
    Body: { scope: 'user' | 'team' | 'company',
            userId?, teamId?, permission: 'view'|'comment'|'edit'|'execute'|'manage',
            applyTree: true, expiresAt? }
    ▼
[TaskGrantService.grant()]
    ├─ Check granter has 'manage' permission on task
    ├─ INSERT orca_task_grants { id, task_id, scope, user_id?, team_id?,
    │     permission, apply_tree, granted_by, granted_at, expires_at }  ← DB
    └─ IF apply_tree=true: propagate to all subtasks (recursive BFS)

PERMISSION CHECK:
[TaskGrantService.hasPermission(userId, taskId, permission)]
    Priority resolution (5-level):
    1. owner → always has 'manage'
    2. admin → always has 'manage'
    3. user grant → SELECT WHERE task_id=? AND scope='user' AND user_id=?
    4. team grant → SELECT WHERE task_id=? AND scope='team' AND team_id IN (user's teams)
    5. company grant → SELECT WHERE task_id=? AND scope='company'
    → First match wins (most specific)
    → IF none: check parent task grant WITH apply_tree=true

Luồng:
Owner/Lead → POST /api/tasks/:id/grants → TaskGrantService
           → Server DB (INSERT grant)
           → IF apply_tree: BFS subtask propagation → INSERT grants × N

Permission check (every API call):
Request → TaskGrantService.hasPermission() → Server DB (multi-level SELECT)
        → allow/deny
```

---

## BL-TG-04 — Task Prompt → Agent Execution

```
Developer/Lead
    │
    ▼
[Browser] Task detail → "Run Agent" button
    │ POST /api/tasks/:id/execute
    ▼
[TaskAgentExecutor.execute(taskId, userId)]
    │
    ├─ Permission check: hasPermission(userId, taskId, 'execute')
    │
    ├─ Check blocking deps:
    │   SELECT from orca_task_edges WHERE from_task_id=taskId AND edge_type='depends_on'
    │   → check all to_task status = 'done'
    │   IF any not done: return error 'BLOCKED_BY_DEPS'
    │
    ├─ Load task + context:
    │   task = SELECT * FROM orca_tasks WHERE id=?
    │   parent = SELECT * (if parentId exists)
    │   project = SELECT * FROM orca_projects WHERE id=?
    │
    ├─ Build agent preamble:
    │   "You are working on Task #<id>: <title>\n"
    │   "Type: <type> | Priority: <priority>\n"
    │   "Description: <description>\n"
    │   "AI Context: <ai_context>\n"
    │   IF parentId: "Parent task: <parent.title>\n"
    │   IF prompt_template: <resolved prompt template>
    │
    ├─ ProfileAwareAgentSpawner.spawn(userId, worktreeId):
    │   ProfileResolver → ResolvedProfile (BL-PRF-02)
    │   AIProviderResolver → ProviderConfig (BL-AIP-02)
    │   relay.call('agent.spawn', {
    │     cmd: providerConfig.agentCommand,
    │     env: { ...profile.envVars, [providerConfig.apiKeyEnvVar]: '<key>' },
    │     cwd: worktreePath,
    │     initialPrompt: preamble
    │   })
    │
    ├─ UPDATE orca_tasks SET status='in_progress', agent_session_id=?  ← DB
    │
    ├─ Stream agent events → Browser (SSE):
    │   task:agentStarted, task:agentOutput, task:agentCompleted
    │
    └─ ON agent:complete:
        UPDATE orca_tasks SET status='review'  ← DB
        emit: task:agentCompleted

Luồng:
User → POST /api/tasks/:id/execute → TaskAgentExecutor
     → Server DB (check deps + load context)
     → ProfileResolver + AIProviderResolver
     → relay.call('agent.spawn') → Dev Server → PTY → Agent
     → Server DB (UPDATE status)
     → SSE events → Browser (real-time output)
```

---

## Sơ đồ tổng quan — Task Graph

```
┌──────────────────┐  HTTP/SSE  ┌────────────────────────────────────────────┐
│  Browser         │◄──────────►│  Orca Web Server                           │
│  Task graph UI   │            │  TaskService (CRUD + progress)             │
│  Kanban board    │            │  TaskDAGValidator (cycle detect)           │
│  AI plan preview │            │  TaskGraphBuilder (BFS + access filter)    │
│  Execution feed  │            │  TaskAIPlanner (decomposition)             │
└──────────────────┘            │  TaskGrantService (5-level permission)     │
                                │  TaskAgentExecutor (spawn + stream)        │
                                └──────────┬─────────────────────────────────┘
                                           │
                                ┌──────────▼─────────────────────────────────┐
                                │  Server Database                            │
                                │  orca_tasks (DAG nodes)                    │
                                │  orca_task_edges (DAG edges)               │
                                │  orca_task_grants (ACL)                    │
                                │  orca_task_comments                        │
                                └──────────┬─────────────────────────────────┘
                                           │
                   ┌───────────────────────┤
                   │                       │
          ┌────────▼──────────┐   ┌────────▼──────────────────────┐
          │  ProfileResolver  │   │  relay.call('agent.spawn')    │
          │  AIProviderResolver│  │  → Dev Server → PTY → Agent   │
          └───────────────────┘   └───────────────────────────────┘

Task state machine:
backlog → todo → in_progress → review → done
                 ↑ blocked (unresolved deps)
```
