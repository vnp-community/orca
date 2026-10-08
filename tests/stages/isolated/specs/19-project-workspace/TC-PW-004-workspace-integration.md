# TC-PW-004 — Workspace Integration (Agent + Git + Tasks + Workflows)

**BL Reference:** BL-PW-04  
**Flow Reference:** docs/flows/logic/project-workspace.md  
**Priority:** P0  
**Type:** E2E + Integration  
**Actor:** Developer, Lead

---

## TC-PW-004-01: E2E — Full developer workflow in workspace (Journey 5)

**Priority:** P0  
**Type:** E2E

### Scenario: Alex opens project, runs agent, commits, pushes, creates PR

### Steps
1. `workspace.switch { projectId: 'proj-frontend' }`
2. `worktree.create { baseRef: 'main', name: 'feat-login' }`
3. `task.create { title: 'Implement Login', type: 'story', projectId }`
4. `task.runAgent { taskId, worktreeId }` → agent spawns
5. Monitor agent status → `agentCompleted`
6. Git panel auto-refresh → show modified files
7. `git.stageAll { projectId }`
8. `git.generateCommitMessage { projectId }` → AI message
9. `git.commit { message: '...' }`
10. `git.push { branch: 'feat-login' }`
11. `git.createPR { useAI: true, base: 'main' }`

### Expected Results
- Each step succeeds
- Git panel updates after each git operation
- PR created on GitHub with AI description
- Task status → 'review' after agent completes

### Assertions
```
// Verify end-to-end state
pr = await rpc.call('git.createPR', { projectId, base: 'main', useAI: true })
assert pr.prUrl.includes('github.com/..../pull/')

task = await getTask(taskId)
assert task.status === 'review'
assert task.agentSessionId !== null
```

---

## TC-PW-004-02: Task → Workspace → Worktree — Full context flow

**Priority:** P0

### Scenario: Task linked to project → opens workspace → switches to task's worktree → ready to run agent

### Steps
1. Task has `{ projectId: 'proj-backend', worktreeId: 'wt-auth-bcrypt' }`
2. User clicks [Open Workspace] from task detail
3. `workspace.openTaskContext { taskId }`

### Expected Results
- Workspace switches to proj-backend (BL-PW-01)
- Worktree switches to wt-auth-bcrypt: `relay.call('git.worktree.switch', { worktreeId })`
- Task detail panel renders in workspace
- [Run Agent] button ready

### Assertions
```
await rpc.call('workspace.openTaskContext', { taskId })
ctx = WorkspaceContext.get('proj-backend')
assert ctx.project.id === 'proj-backend'
assert ctx.currentWorktree.id === 'wt-auth-bcrypt'

worktreeCall = capturedRelayCall('git.worktree.switch')
assert worktreeCall.args.worktreeId === 'wt-auth-bcrypt'
```

---

## TC-PW-004-03: Agent complete → Full cross-panel update

**Priority:** P0

### Steps
1. Workspace open (Explorer, Git, Tasks panels active)
2. Agent runs and completes
3. Monitor all panels

### Expected Results
On `agent.complete` event:
- **Git panel**: git status re-fetched → shows modified files from agent
- **Explorer**: file decorations updated (M badges on modified files)
- **Tasks panel**: task status → 'review' + activity feed updated

### Assertions
```
gitRefreshSpy = jest.spyOn(gitPanel, 'refresh')
explorerRefreshSpy = jest.spyOn(explorerPanel, 'refresh')
taskActivitySpy = jest.spyOn(tasksPanel, 'updateActivity')

simulateAgentComplete(sessionId, { exitCode: 0 })
await delay(100)

assert gitRefreshSpy.called
assert explorerRefreshSpy.called
assert taskActivitySpy.calledWith(taskId, 'agentCompleted')
```

---

## TC-PW-004-04: Multiple projects — Isolated contexts

**Priority:** P0

### Steps
1. Open project A (srv-1, repo: /srv/projects/proj-a)
2. Open project B (srv-2, repo: /srv/projects/proj-b)
3. Git operations in A → only affect A's relay/context
4. Git operations in B → only affect B's relay/context

### Expected Results
- No cross-contamination between project contexts
- A's git status not showing B's files
- B's agent session not appearing in A's panel

### Assertions
```
await rpc.call('workspace.switch', { projectId: 'proj-A' })
await rpc.call('workspace.switch', { projectId: 'proj-B' })

statusA = WorkspaceContext.get('proj-A').gitStatus
statusB = WorkspaceContext.get('proj-B').gitStatus
assert statusA.cwd !== statusB.cwd
assert statusA.cwd.includes('proj-a')
assert statusB.cwd.includes('proj-b')
```

---

## TC-PW-004-05: Worktree switcher — Updates all panels

**Priority:** P1

### Steps
1. 3 worktrees exist: [main, feature/auth, feature/ui]
2. Switch from main → feature/auth via worktree selector

### Expected Results
- File explorer updates to feature/auth path
- Git status updates: shows feature/auth branch's changes
- Agent panel shows agents for feature/auth worktree

### Assertions
```
await rpc.call('workspace.switchWorktree', { projectId, worktreeId: authWorktree.id })
status = await rpc.call('git.status', { projectId })
assert status.branch === 'feature/auth'

explorerRoot = WorkspaceContext.get(projectId).fileTreeRoot.path
assert explorerRoot === authWorktree.path
```

---

## TC-PW-004-06: Workflow panel — Quick run + recent executions

**Priority:** P1

### Steps
1. Workflow panel open
2. `POST /api/workflows/execute { templateId: 'quick-feature' }`
3. Monitor recent executions list

### Expected Results
- "Quick Run" starts workflow
- Execution appears in "Recent Executions" list immediately
- Live progress: step statuses update in real-time
- After complete: "DONE" status shown

---

## TC-PW-004-07: Server status indicator — Online/Degraded/Offline

**Priority:** P1

### Steps
1. Dev server health changes: healthy → degraded → offline
2. Monitor ServerStatusBar in workspace

### Expected Results
- healthy: green indicator, all operations available
- degraded: yellow indicator, write operations warn user
- offline: red indicator, offline mode activated (BL-PW-01)

---

## TC-PW-004-08: Task → Agent → Git auto-refresh → PR (Full end-to-end)

**Priority:** P0  
**Type:** E2E

### Scenario: Full flow from task execution to PR creation

### Steps
1. Task 'Implement bcrypt' exists với worktreeId
2. Click [Run Agent] from task detail
3. Agent spawns → works → completes
4. Git panel: shows 3 modified files (auto-refresh)
5. [Stage All] → [AI: Generate message] → [Commit & Push]
6. [Create PR] → PR #42 created
7. Task prUrl updated, status → 'review'

### Expected Results
- Entire flow completes without manual intervention except staging/committing
- Task.prUrl = 'https://github.com/org/repo/pull/42'
- Task.status = 'review'

### Assertions
```
// After full E2E flow
task = await getTask(taskId)
assert task.status === 'review'
assert task.prUrl !== undefined
assert task.prUrl.includes('/pull/')
```

---

*TC-PW-004 — Orca v5.0 — Updated 2026-08-01*
