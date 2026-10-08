# TC-WT-003 — Xóa Worktree An Toàn

**BL Reference:** BL-WT-03  
**Flow Reference:** docs/flows/logic/worktree-management.md#BL-WT-03  
**Priority:** P0  
**Type:** Integration  
**Actor:** Alex, Maya, Carlos

---

## TC-WT-003-01: Safety check — Worktree sạch (clean)

**Priority:** P0

### Preconditions
- Worktree đang có git status sạch (no uncommitted changes)
- Không có agent đang chạy

### Steps
1. RPC: `worktree.checkSafety { worktreeId }`

### Expected Results
- Response: `{ safe: true, uncommittedChanges: 0, agentRunning: false }`

### Assertions
```
result = await rpc.call('worktree.checkSafety', { worktreeId })
assert result.safe === true
assert result.uncommittedChanges === 0
assert result.agentRunning === false
```

---

## TC-WT-003-02: Safety check — Có uncommitted changes

**Priority:** P0

### Preconditions
- Worktree có 3 modified files (via git status --porcelain)

### Steps
1. `worktree.checkSafety { worktreeId }`

### Expected Results
- Response: `{ safe: false, uncommittedChanges: 3, warning: 'Uncommitted changes will be lost' }`

### Assertions
```
result = await rpc.call('worktree.checkSafety', { worktreeId })
assert result.safe === false
assert result.uncommittedChanges === 3
assert result.warning !== undefined
```

---

## TC-WT-003-03: Safety check — Agent đang chạy

**Priority:** P0

### Preconditions
- Agent đang chạy trong worktree

### Steps
1. `worktree.checkSafety { worktreeId }`

### Expected Results
- Response: `{ safe: false, agentRunning: true }`

---

## TC-WT-003-04: Xóa worktree clean — Happy Path

**Priority:** P0

### Preconditions
- Worktree clean, không có agent, không có PTY

### Steps
1. `worktree.delete { worktreeId, force: false }`
2. Kiểm tra relay calls
3. Kiểm tra DB

### Expected Results
- `relay.call('git.worktree.remove')` được gọi
- DB: `orca_worktrees` row bị DELETE
- DB: `orca_terminal_sessions` rows cho worktree bị DELETE
- WS event: `worktree:deleted { worktreeId }`

### Assertions
```
await rpc.call('worktree.delete', { worktreeId, force: false })
assert spyRelay.calledWith('git.worktree.remove', { worktreePath: wt.path })
assert db.worktrees.find({ id: worktreeId }) === null
event = await wsEvents.next('worktree:deleted')
assert event.worktreeId === worktreeId
```

---

## TC-WT-003-05: Xóa worktree — Kill agent trước

**Priority:** P0

### Preconditions
- Agent đang chạy, PTY active

### Steps
1. Safety check → `{ safe: false, agentRunning: true }`
2. User confirm → `worktree.delete { worktreeId, force: true }`

### Expected Results
- `relay.call('agent.kill', { ptyId })` được gọi TRƯỚC
- `relay.call('pty.destroy', { ptyId })` được gọi
- `relay.call('git.worktree.remove', ...)` được gọi sau
- Correct ordering: kill → destroy → remove

### Assertions
```
calls = spyRelay.allCalls()
agentKillIdx = calls.findIndex(c => c.method === 'agent.kill')
ptyDestroyIdx = calls.findIndex(c => c.method === 'pty.destroy')
gitRemoveIdx = calls.findIndex(c => c.method === 'git.worktree.remove')
assert agentKillIdx < ptyDestroyIdx
assert ptyDestroyIdx < gitRemoveIdx
```

---

## TC-WT-003-06: Xóa worktree — Uncommitted changes + force

**Priority:** P1

### Steps
1. Safety check → `{ safe: false, uncommittedChanges: 5 }`
2. User confirm force delete → `worktree.delete { force: true }`

### Expected Results
- `git.worktree.remove { ..., force: true }` được gọi
- Worktree bị xóa kể cả khi có uncommitted changes

### Assertions
```
await rpc.call('worktree.delete', { worktreeId, force: true })
assert spyRelay.calledWith('git.worktree.remove', { force: true })
assert db.worktrees.find({ id: worktreeId }) === null
```

---

## TC-WT-003-07: Xóa worktree — Không có quyền xóa của người khác

**Priority:** P0  
**Security:** User A không được xóa worktree của User B

### Steps
1. User A login
2. Thử `worktree.delete { worktreeId: userBWorktreeId }`

### Expected Results
- Error: `{ code: 'FORBIDDEN' }`
- Worktree của User B không bị xóa

### Assertions
```
loginAsUserA()
result = await rpc.call('worktree.delete', { worktreeId: userBWorktreeId }).catch(e => e)
assert result.code === 'FORBIDDEN'
assert db.worktrees.find({ id: userBWorktreeId }) !== null // unchanged
```

---

*TC-WT-003 — Orca v5.0 — 2026-08-01*
