# TC-WT-002 — Fan-out Prompt tới Nhiều Worktrees

**BL Reference:** BL-WT-02  
**Flow Reference:** docs/flows/logic/worktree-management.md#BL-WT-02  
**Priority:** P0  
**Type:** Integration + Performance  
**Actor:** Alex

---

## Preconditions
- User Alex đã login, project tồn tại
- Dev Server online với relay connected
- Disk space đủ cho N worktrees

---

## TC-WT-002-01: Fan-out N=3 worktrees — Happy Path

**Priority:** P0

### Test Data
| Field | Value |
|-------|-------|
| n | 3 |
| prompt | "Implement user authentication with JWT" |
| baseRef | "main" |
| agentType | "claude" |

### Steps
1. RPC: `worktree.fanOut { projectId, prompt, n: 3, baseRef: 'main', agentType: 'claude' }`
2. Theo dõi relay calls
3. Kiểm tra DB
4. Kiểm tra WS events

### Expected Results
- 3x `relay.call('git.worktree.add')` (parallel)
- 3x `relay.call('pty.create')`
- 3x `relay.call('agent.spawn', { prompt })`
- DB: 3 rows trong `orca_worktrees` status='ready'
- WS: 3 events `worktree:created` + 3 events `agent:started`

### Assertions
```
result = await rpc.call('worktree.fanOut', { n: 3, prompt, baseRef: 'main' })

// Parallel creation
assert spyRelay.callCount('git.worktree.add') === 3
assert spyRelay.callCount('pty.create') === 3
assert spyRelay.callCount('agent.spawn') === 3

// Each agent.spawn has same prompt
spyRelay.calls('agent.spawn').forEach(call => {
  assert call.args.prompt === prompt
})

// DB
assert db.worktrees.count({ projectId }) === 3
db.worktrees.list({ projectId }).forEach(wt => {
  assert wt.status === 'ready'
})

// Events
events = await wsEvents.collect('worktree:created', { count: 3, timeout: 30000 })
assert events.length === 3
```

---

## TC-WT-002-02: Fan-out — Boundary values (N=1, N=5, N=10)

**Priority:** P1

### Steps (N=1)
1. `worktree.fanOut { n: 1, ... }`
2. Verify: 1 worktree created, 1 agent started

### Steps (N=5)
1. `worktree.fanOut { n: 5, ... }`
2. Verify: 5 worktrees, all parallel

### Steps (N=10)
1. `worktree.fanOut { n: 10, ... }`
2. Verify: 10 worktrees created

### Assertions
```
[1, 5, 10].forEach(async n => {
  result = await rpc.call('worktree.fanOut', { n, ... })
  assert db.worktrees.count({ fanOutId: result.fanOutId }) === n
})
```

---

## TC-WT-002-03: Fan-out — N=0 (invalid)

**Priority:** P1

### Steps
1. `worktree.fanOut { n: 0, ... }`

### Expected Results
- Error: `{ code: 'INVALID_N', message: 'n must be >= 1' }`

---

## TC-WT-002-04: Fan-out — N=11 (exceeds max)

**Priority:** P1

### Steps
1. `worktree.fanOut { n: 11, ... }`

### Expected Results
- Error: `{ code: 'N_EXCEEDS_MAXIMUM', max: 10 }`

---

## TC-WT-002-05: Fan-out — Partial failure (1 worktree fails)

**Priority:** P1

### Steps
1. Mock: `git.worktree.add` fails for worktree #2 (out of 3)
2. `worktree.fanOut { n: 3, ... }`

### Expected Results
- `Promise.allSettled` — các worktrees còn lại vẫn succeed
- Response: `{ succeeded: 2, failed: 1, failedIndexes: [1] }`
- DB: 2 worktrees created, 0 rollback of successful ones

### Assertions
```
mockRelayFailForIndex('git.worktree.add', 1)
result = await rpc.call('worktree.fanOut', { n: 3, ... })
assert result.succeeded === 2
assert result.failed === 1
assert db.worktrees.count({ projectId, status: 'ready' }) === 2
```

---

## TC-WT-002-06: Fan-out — Tất cả worktrees có unique paths

**Priority:** P0

### Steps
1. `worktree.fanOut { n: 5, ... }`
2. Kiểm tra paths

### Expected Results
- 5 paths đều unique
- Không có conflict

### Assertions
```
result = await rpc.call('worktree.fanOut', { n: 5, ... })
worktrees = db.worktrees.list({ projectId })
paths = worktrees.map(wt => wt.path)
assert new Set(paths).size === 5 // all unique
```

---

## TC-WT-002-07: Fan-out notification khi hoàn thành

**Priority:** P1

### Steps
1. `worktree.fanOut { n: 3, ... }`
2. Đợi tất cả agent complete
3. Kiểm tra notification

### Expected Results
- Notification: "3 agents completed fan-out task"
- Mobile push notification nếu Sam paired

---

## TC-WT-002-08: Performance — Fan-out N=5 < 60s

**Priority:** P1  
**Type:** Performance

### Steps
1. Measure time: `worktree.fanOut { n: 5 }` → tất cả `agent:started`

### Expected Results
- Total time < 60s

### Assertions
```
startTime = Date.now()
await rpc.call('worktree.fanOut', { n: 5, ... })
events = await wsEvents.collect('agent:started', { count: 5 })
duration = Date.now() - startTime
assert duration < 60000
```

---

*TC-WT-002 — Orca v5.0 — 2026-08-01*
