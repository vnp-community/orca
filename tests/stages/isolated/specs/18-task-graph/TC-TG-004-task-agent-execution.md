# TC-TG-004 — Task Prompt → Agent Execution

**BL Reference:** BL-TG-04  
**Flow Reference:** docs/flows/logic/task-graph.md  
**Priority:** P0  
**Type:** Integration  
**Actor:** Developer, Lead

---

## TC-TG-004-01: Run Agent từ Task — Full preamble injection

**Priority:** P0

### Preconditions
- Task: `{ title: 'Fix login bug', description: 'JWT refresh fails', type: 'bug', aiContext: 'Check auth-manager.ts' }`
- Parent task: `{ title: 'Auth System' }`
- worktreeId linked to task

### Steps
1. `POST /api/tasks/:id/execute { worktreeId }`

### Expected Results
- Agent spawned với preamble:
  ```
  You are working on Task #<id>: Fix login bug
  Type: bug | Priority: high
  Description: JWT refresh fails
  AI Context: Check auth-manager.ts
  Parent task: Auth System
  ```
- `orca_tasks.status` → 'in_progress'
- `orca_tasks.agent_session_id` set

### Assertions
```
await rpc.call('task.runAgent', { taskId, worktreeId })
spawnCall = capturedRelayCall('agent.spawn')
preamble = spawnCall.args.initialPrompt
assert preamble.includes('Fix login bug')
assert preamble.includes('JWT refresh fails')
assert preamble.includes('Check auth-manager.ts')
assert preamble.includes('Auth System')  // parent

task = await getTask(taskId)
assert task.status === 'in_progress'
assert task.agentSessionId !== null
```

---

## TC-TG-004-02: Blocking deps check — BLOCKED_BY_DEPS error

**Priority:** P0

### Preconditions
- Task B depends_on Task A
- Task A status = 'in_progress' (not done)
- User tries to run agent on Task B

### Steps
1. `POST /api/tasks/B.id/execute`

### Expected Results
- Error: `{ code: 'BLOCKED_BY_DEPS', blockedBy: ['<A.id>'] }`
- No agent spawned
- Task B status unchanged

### Assertions
```
// A is not done
mockTask(A.id, { status: 'in_progress' })
result = await api.post('/api/tasks/' + B.id + '/execute').catch(e => e)
assert result.status === 403
assert result.body.code === 'BLOCKED_BY_DEPS'
assert result.body.blockedBy.includes(A.id)

// No spawn call
assert capturedRelayCall('agent.spawn') === undefined
```

---

## TC-TG-004-03: Require 'execute' grant to run agent

**Priority:** P0

### Steps
1. User B có grant 'edit' (not execute)
2. User B: `POST /api/tasks/:id/execute`

### Expected Results
- Error: `{ code: 'FORBIDDEN', required: 'execute', current: 'edit' }`

### Assertions
```
loginAs(userB)  // userB has 'edit' grant
result = await api.post('/api/tasks/' + taskId + '/execute').catch(e => e)
assert result.status === 403
assert result.body.code === 'FORBIDDEN'
assert result.body.required === 'execute'
```

---

## TC-TG-004-04: Agent output → Task activity feed stream

**Priority:** P0

### Steps
1. Run agent from task
2. Agent produces output lines
3. Monitor SSE stream

### Expected Results
- Events: `task:agentStarted`, `task:agentOutput`, `task:agentCompleted`
- `task:agentOutput.line` = each output line
- Activity feed in UI shows agent log

### Assertions
```
events = []
subscribeSSE('/api/tasks/' + taskId + '/events', e => events.push(e))

await rpc.call('task.runAgent', { taskId, worktreeId })
simulateAgentOutput('Reading auth-manager.ts...')
simulateAgentOutput('Fixing JWT refresh...')
simulateAgentComplete()

await waitFor(() => events.some(e => e.type === 'task:agentCompleted'))
assert events.some(e => e.type === 'task:agentStarted')
assert events.some(e => e.type === 'task:agentOutput' && e.line.includes('Reading'))
assert events.some(e => e.type === 'task:agentCompleted')
```

---

## TC-TG-004-05: Agent complete → Task status → 'review'

**Priority:** P0

### Steps
1. Run agent from task
2. Agent completes successfully (exit code 0)

### Expected Results
- Task status → 'review' (auto, not 'done' yet)
- DB: `orca_tasks.status = 'review'`
- Emit: `task:agentCompleted`

### Assertions
```
await rpc.call('task.runAgent', { taskId, worktreeId })
simulateAgentComplete(exitCode: 0)
await delay(100)

task = await getTask(taskId)
assert task.status === 'review'  // not 'done' — needs review step
```

---

## TC-TG-004-06: Profile-aware agent execution — Provider from resolved profile

**Priority:** P0

### Preconditions
- User profile: `{ agentModel: 'claude-opus-4-5', provider: 'anthropic' }`
- Task project on dev-alpha server

### Steps
1. Run agent from task

### Expected Results
- ProfileResolver called for current user
- Provider resolved: anthropic claude-opus-4-5
- Agent spawned with:
  ```
  env: { ANTHROPIC_API_KEY: '<decrypted-key>', ... }
  cmd: 'claude'
  ```

### Assertions
```
await rpc.call('task.runAgent', { taskId, worktreeId })
spawnCall = capturedRelayCall('agent.spawn')
assert spawnCall.args.cmd.includes('claude')
assert spawnCall.args.env.ANTHROPIC_API_KEY !== undefined
assert !spawnCall.args.env.ANTHROPIC_API_KEY.includes('<encrypted>')  // decrypted
```

---

## TC-TG-004-07: Multiple agents on same task — Concurrent

**Priority:** P1

### Steps
1. Run agent 1 on task-X (worktree-1)
2. Run agent 2 on task-X (worktree-2)

### Expected Results
- Both agents spawn successfully
- Both activity feeds linked to task-X
- Task shows: "2 agents running"
- Each agent's output tagged with its sessionId

### Assertions
```
session1 = await rpc.call('task.runAgent', { taskId, worktreeId: wt1.id })
session2 = await rpc.call('task.runAgent', { taskId, worktreeId: wt2.id })
assert session1.sessionId !== session2.sessionId

task = await getTask(taskId)
assert task.activeAgentSessions.length === 2
```

---

## TC-TG-004-08: Task agent execution — worktreeId optional (uses project default)

**Priority:** P1

### Preconditions
- Task linked to project-123
- project-123 has default worktree (main branch)

### Steps
1. `POST /api/tasks/:id/execute` without worktreeId

### Expected Results
- Agent spawns on project's default dev server
- cwd = project's repo path (main branch)

---

*TC-TG-004 — Orca v5.0 — Updated 2026-08-01*
