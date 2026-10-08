# TC-PW-001 — Project Workspace Context

**BL Reference:** BL-PW-01  
**Flow Reference:** docs/flows/logic/project-workspace.md  
**Priority:** P0  
**Type:** Integration  
**Actor:** Developer, Lead

---

## TC-PW-001-01: Open project → WorkspaceContext initialized

**Priority:** P0

### Steps
1. User select project-123 from project list
2. `workspace.switch({ projectId: 'project-123' })`

### Expected Results
- WorkspaceContext populated:
  - `project` metadata loaded
  - `relay` connection established (or reused)
  - `profile` resolved (3-layer inheritance)
  - `gitStatus` fetched (initial poll via relay)
  - `worktrees` list fetched
  - `fileTreeRoot` (depth 2) fetched
  - `activeWorkflows` fetched from DB
  - `currentWorktree` = null (main)
- All parallel loads via `Promise.all`

### Assertions
```
await rpc.call('workspace.switch', { projectId: 'project-123' })
ctx = WorkspaceContext.get('project-123')
assert ctx.project.id === 'project-123'
assert ctx.relay !== null
assert ctx.relay.connected === true
assert ctx.profile !== null
assert ctx.gitStatus !== null
assert Array.isArray(ctx.worktrees)
assert ctx.fileTreeRoot !== null
```

---

## TC-PW-001-02: Permission check — Non-member blocked

**Priority:** P0

### Preconditions
- userB is NOT a member of project-123

### Steps
1. Login as userB
2. `workspace.switch({ projectId: 'project-123' })`

### Expected Results
- Error: `{ code: 'FORBIDDEN' }` (403)
- No relay connection established
- No workspace context created

### Assertions
```
loginAs(userB)
result = await rpc.call('workspace.switch', { projectId: 'project-123' }).catch(e => e)
assert result.code === 'FORBIDDEN'
assert WorkspaceContext.get('project-123') === undefined
```

---

## TC-PW-001-03: RelayConnectionPool — Reuse connections same server

**Priority:** P0

### Steps
1. Open project A (devServerId: srv-1)
2. Open project B (devServerId: srv-1) — same server!
3. Verify relay connection reused

### Expected Results
- Only 1 WebSocket connection to srv-1
- Both projects share same relay connection
- SSH connection count: 1 (not 2)

### Assertions
```
await rpc.call('workspace.switch', { projectId: 'proj-A' })
connCountBefore = RelayConnectionPool.connectionCount('srv-1')
await rpc.call('workspace.switch', { projectId: 'proj-B' })
connCountAfter = RelayConnectionPool.connectionCount('srv-1')
assert connCountBefore === connCountAfter  // no new connection
assert connCountAfter === 1  // shared connection
```

---

## TC-PW-001-04: RelayConnectionPool — Cleanup idle > 5min

**Priority:** P1

### Steps
1. Open project, use relay
2. Close project, no activity
3. Advance time 5+ minutes
4. Verify relay connection closed

### Assertions
```
await rpc.call('workspace.switch', { projectId })
await rpc.call('workspace.close', { projectId })
advanceTime(5 * 60 * 1000 + 1000)  // 5 min + 1s
assert RelayConnectionPool.connectionCount('srv-1') === 0
```

---

## TC-PW-001-05: Offline mode — Banner + cached file tree + disable writes

**Priority:** P0

### Steps
1. Open workspace (normal) → file tree cached
2. Simulate relay disconnect (FleetHealthMonitor returns 'unreachable')
3. User tries file operations

### Expected Results
- Offline banner displayed: "Dev server offline — read-only mode"
- File tree: uses cached data (last known state, not empty)
- Write operations (git commit, file write): disabled with "Unavailable offline" message
- Git status poll: paused

### Assertions
```
await rpc.call('workspace.switch', { projectId })
simulateServerDown('srv-1')

state = await rpc.call('workspace.getState', { projectId })
assert state.offlineMode === true
assert state.fileTreeRoot !== null  // cached

// Write disabled
result = await rpc.call('git.commit', { message: 'test' }).catch(e => e)
assert result.code === 'OFFLINE_MODE'
```

---

## TC-PW-001-06: Git status poll — 5s khi tab active, stop khi inactive

**Priority:** P1

### Steps
1. Open workspace
2. Git tab is active (focused)
3. Wait 15s → verify poll count
4. Switch tab to inactive
5. Wait 15s → verify poll count same

### Expected Results
- Active: git status fetched mỗi 5s → at least 3 polls in 15s
- Inactive: poll stops (count unchanged)

---

## TC-PW-001-07: Teardown previous workspace — Keep relay same server

**Priority:** P1

### Steps
1. Switch from project A (srv-1) to project B (srv-1)
2. Both on same dev server

### Expected Results
- Previous workspace torn down: terminal warnings if sessions running
- Git status poll for project A stopped
- Relay connection kept (same server)
- New workspace initialized with project B context

---

## TC-PW-001-08: Cross-panel event bus — agent.complete → all panels refresh

**Priority:** P0

### Steps
1. Workspace open với Explorer, Git, Tasks panels active
2. Agent session completes

### Expected Results
- `agent.complete` event triggers ALL panels to refresh:
  - Git panel: git status re-fetched
  - Explorer: file decorations updated (modified files shown)
  - Tasks panel: task status updated

### Assertions
```
// Setup spies
gitRefreshSpy = jest.spyOn(gitPanel, 'refresh')
explorerRefreshSpy = jest.spyOn(explorerPanel, 'refresh')
tasksRefreshSpy = jest.spyOn(tasksPanel, 'refresh')

// Agent completes
simulateAgentComplete(sessionId)
await delay(100)

assert gitRefreshSpy.called
assert explorerRefreshSpy.called
assert tasksRefreshSpy.called
```

---

*TC-PW-001 — Orca v5.0 — Updated 2026-08-01*
