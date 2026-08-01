# BL-TG-04 — Task Prompt → Agent Execution

| Trường | Giá trị |
|--------|---------|
| **Mã** | BL-TG-04 |
| **Tên** | Task Prompt → Agent Execution |
| **Domain** | Task Graph |
| **Actor** | Developer, Lead |
| **Priority** | P0 |

---

## Mô tả

Từ Task Detail, người dùng có thể khởi động AI agent trực tiếp. Hệ thống inject đầy đủ task context (metadata, prompt template, ancestor context) vào agent session. Agent chạy trên Dev Server gắn với project của task.

---

## Luồng: Run Agent từ Task

```
User → Task Detail → [▶ Run Agent]
    │
    ├── Pre-check:
    │   - task.projectId required → project → devServerId
    │   - user có permission 'execute' trên task
    │   - dev server online (FleetHealthMonitor check)
    │
    ├── Worktree selection/creation:
    │   IF task.worktreeId exists: use existing worktree
    │   ELSE: prompt user "Create new worktree?" → yes → git worktree add
    │
    ├── Resolve AI Provider:
    │   ProviderResolver.resolve({
    │     devServerId: project.devServerId,
    │     provider: profile.agent.preferredModel derived provider,
    │     projectId: task.projectId,
    │     userId
    │   })
    │
    ├── Build Task Context Preamble:
    │   """
    │   # Task Context
    │   Task ID: {{task.id}}
    │   Task: {{task.title}}
    │   Type: {{task.type}} | Priority: {{task.priority}}
    │   Status: {{task.status}}
    │   Project: {{project.name}}
    │   Branch: {{worktree.branch}}
    │
    │   ## Description
    │   {{task.description}}
    │
    │   ## Additional Context
    │   {{task.aiContext}}
    │
    │   ## Parent Task Context (if subtask)
    │   {{parent.title}}: {{parent.description}}
    │
    │   ## Dependencies (completed)
    │   {{#each completedDeps}}
    │   - ✅ {{title}}: {{summary_output}}
    │   {{/each}}
    │
    │   ## Your Task
    │   {{task.promptTemplate}}
    │   """
    │
    ├── Spawn Agent:
    │   relay.call('pty.spawn', {
    │     cmd: resolveAgentBinary(provider.model),
    │     args: buildAgentArgs(profile.agent.trustPreset),
    │     cwd: worktree.path,
    │     env: {
    │       ...profile.shell.envVars,
    │       PATH: pathAdditions + $PATH,
    │       GH_CONFIG_DIR: perUserDir,
    │       ANTHROPIC_MODEL: provider.model,
    │       ORCA_TASK_ID: task.id,
    │       ORCA_PROJECT_ID: task.projectId,
    │     },
    │     initFile: taskContextPreamble  // injected before user prompt
    │   })
    │
    ├── Link session to task:
    │   UPDATE orca_tasks SET agent_session_id=sessionId, worktree_id=worktreeId
    │
    ├── Stream PTY output → WebSocket → Task Activity Feed
    │   { type: 'task.agent_output', taskId, line: '...' }
    │
    └── On agent complete:
        UPDATE orca_tasks SET
          status = 'review',        -- auto-advance to review
          actual_hours = elapsed,
          agent_session_id = null
        Emit { type: 'task.agent_completed', taskId }
        → check if parent task should update progress
```

---

## Task Activity Feed

```typescript
// Stream tất cả events liên quan đến task vào Activity Feed:
type TaskActivityEvent =
  | { type: 'status_changed'; from: string; to: string; by: string }
  | { type: 'agent_started'; model: string; worktree: string }
  | { type: 'agent_output'; line: string }
  | { type: 'agent_completed'; duration: number }
  | { type: 'comment_added'; author: string; content: string }
  | { type: 'grant_added'; grantee: string; permission: string }
  | { type: 'subtask_done'; subtaskTitle: string }
  | { type: 'estimate_updated'; old: number; new: number }
```

---

## Multi-task Agent Session (Batch Execution)

```
Lead → select multiple tasks (Ctrl+click) → [Run All with Agent]
    │
    ├── Group tasks by devServer (batch per server)
    │
    ├── Per server group:
    │   Execute tasks in topological order (respect dependencies)
    │   Parallel where no deps between them (up to concurrency limit)
    │
    └── Each task execution = full BL-TG-04 flow
        outputs of earlier tasks available as {{outputs.<taskId>.*}}
        in later task's promptTemplate
```

---

## Tiêu chí chấp nhận

- [ ] "Run Agent" từ Task Detail: permission check + server check
- [ ] Worktree: reuse existing or create new (user chooses)
- [ ] Task context preamble: title, desc, aiContext, parent, completed deps
- [ ] promptTemplate interpolation: {{task.*}}, {{project.*}}, {{worktree.*}}
- [ ] Agent spawn với profile env + ORCA_TASK_ID env
- [ ] Link agent_session_id + worktree_id to task
- [ ] Stream PTY output to Task Activity Feed (WebSocket)
- [ ] Auto-advance task.status to 'review' on agent complete
- [ ] Batch execution: topological order + parallel where possible
- [ ] `actual_hours` auto-calculated from session duration
