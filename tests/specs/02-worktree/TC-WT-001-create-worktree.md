# TC-WT-001 — Tạo Worktree

**BL Reference:** BL-WT-01  
**Flow Reference:** docs/flows/logic/worktree-management.md#BL-WT-01  
**Priority:** P0  
**Type:** Integration  
**Actor:** Alex, Maya, Carlos

---

## Preconditions
- User đã login (session active)
- Project tồn tại với `devServerId` binding hợp lệ
- Dev Server online, relay connected
- Git repo trên Dev Server: `~/projects/test-repo`
- Disk space > 100MB
- User là member của project

---

## TC-WT-001-01: Tạo worktree — Happy Path

**Priority:** P0

### Test Data
| Field | Value |
|-------|-------|
| projectId | `proj-123` |
| baseRef | `main` |
| name | `feature-auth` |
| agentType | `claude` |

### Steps
1. Gửi RPC: `worktree.create { projectId: 'proj-123', baseRef: 'main', name: 'feature-auth', agentType: 'claude' }`
2. Kiểm tra relay calls
3. Kiểm tra DB state
4. Kiểm tra response event

### Expected Results
- `relay.call('git.worktree.add')` được gọi trên Dev Server
- `relay.call('pty.create')` được gọi trên Dev Server
- DB: INSERT vào `orca_worktrees` với status='ready'
- WS event: `worktree:created { worktreeId, path, branch: 'main' }`

### Assertions
```
spyRelay.reset()
result = await rpc.call('worktree.create', { projectId, baseRef: 'main', name: 'feature-auth' })

assert spyRelay.calledWith('git.worktree.add', {
  repoPath: '/home/user/projects/test-repo',
  branch: 'main',
  worktreePath: expect.stringContaining('feature-auth')
})
assert spyRelay.calledWith('pty.create', { cwd: result.path })

wt = db.worktrees.find({ id: result.id })
assert wt.status === 'ready'
assert wt.projectId === 'proj-123'
assert wt.branch === 'main'

event = await wsEvents.next('worktree:created')
assert event.worktreeId === result.id
```

---

## TC-WT-001-02: Tạo worktree — Disk space không đủ

**Priority:** P0

### Steps
1. Mock `relay.call('fs.stat')` trả về `available_bytes: 50 * 1024 * 1024` (50MB)
2. Gửi RPC: `worktree.create { ... }`

### Expected Results
- Error: `{ code: 'INSUFFICIENT_DISK_SPACE', available: 50MB, required: 100MB }`
- Không có git.worktree.add call
- Không có DB insert

### Assertions
```
mockRelay('fs.stat', { available_bytes: 50 * 1024 * 1024 })
result = await rpc.call('worktree.create', { ... }).catch(e => e)
assert result.code === 'INSUFFICIENT_DISK_SPACE'
assert !spyRelay.calledWith('git.worktree.add', anything)
assert db.worktrees.count({ projectId }) === 0
```

---

## TC-WT-001-03: Tạo worktree — baseRef không tồn tại

**Priority:** P1

### Steps
1. Mock `relay.call('git.worktree.add')` throw error: `"branch 'nonexistent' not found"`
2. Gửi RPC: `worktree.create { baseRef: 'nonexistent', ... }`

### Expected Results
- Error trả về cho client
- Không có DB insert

### Assertions
```
mockRelayError('git.worktree.add', 'branch not found')
result = await rpc.call('worktree.create', { baseRef: 'nonexistent', ... }).catch(e => e)
assert result.message.includes('branch')
assert db.worktrees.count({ projectId }) === 0
```

---

## TC-WT-001-04: Tạo worktree — User không phải member

**Priority:** P0

### Steps
1. Login với user không phải member của project
2. Gửi RPC: `worktree.create { projectId: 'proj-123', ... }`

### Expected Results
- Error: `{ code: 'FORBIDDEN', message: 'Not a project member' }`

### Assertions
```
loginAsNonMember()
result = await rpc.call('worktree.create', { projectId }).catch(e => e)
assert result.code === 'FORBIDDEN'
```

---

## TC-WT-001-05: Tạo worktree — Custom path

**Priority:** P1

### Test Data
| Field | Value |
|-------|-------|
| path | `/custom/path/to/worktree` |

### Steps
1. `worktree.create { ..., path: '/custom/path/to/worktree' }`
2. Kiểm tra custom path được dùng

### Expected Results
- `git.worktree.add` được gọi với `worktreePath: '/custom/path/to/worktree'`

---

## TC-WT-001-06: Tạo worktree — Dev Server offline

**Priority:** P1

### Steps
1. Mock relay connection as offline
2. `worktree.create { ... }`

### Expected Results
- Error: `{ code: 'DEV_SERVER_UNREACHABLE' }`

---

## TC-WT-001-07: Performance — Worktree create trong < 30s

**Priority:** P1  
**Type:** Performance

### Steps
1. Measure time: start → `worktree:created` event

### Expected Results
- Total time < 30,000ms

### Assertions
```
startTime = Date.now()
await rpc.call('worktree.create', { ... })
await wsEvents.next('worktree:created')
duration = Date.now() - startTime
assert duration < 30000
```

---

*TC-WT-001 — Orca v5.0 — 2026-08-01*
