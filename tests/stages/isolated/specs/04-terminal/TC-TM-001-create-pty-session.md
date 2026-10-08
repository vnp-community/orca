# TC-TM-001 — Tạo PTY Session

**BL Reference:** BL-TM-01  
**Flow Reference:** docs/flows/logic/terminal-management.md  
**Priority:** P0  
**Type:** Integration  
**Actor:** Alex, Carlos

---

## TC-TM-001-01: Tạo PTY Session — Happy Path

**Priority:** P0

### Steps
1. RPC: `terminal.create { projectId, worktreeId, shell: '/bin/bash' }`
2. Verify relay call
3. Verify DB insert

### Expected Results
- `relay.call('pty.create', { cwd: worktreePath, shell: '/bin/bash' })` trên Dev Server
- Dev Server: `node-pty.spawn('/bin/bash', [], { cwd })` 
- DB: INSERT `orca_terminal_sessions`
- WS event: `terminal:created { terminalId, ptyId }`

### Assertions
```
result = await rpc.call('terminal.create', { projectId, worktreeId })
assert spyRelay.calledWith('pty.create', { cwd: worktree.path })
session = db.terminalSessions.find({ id: result.terminalId })
assert session !== null
assert session.status === 'active'
```

---

## TC-TM-001-02: Terminal input/output — PTY write

**Priority:** P0

### Steps
1. Tạo PTY session
2. Gửi input: `terminal.write { terminalId, data: 'ls -la\n' }`
3. Verify relay call và output stream

### Expected Results
- `relay.call('pty.write', { ptyId, data: 'ls -la\n' })`
- Output stream về qua WS: `terminal:output { terminalId, data: '<ls output>' }`

---

## TC-TM-001-03: Terminal resize

**Priority:** P1

### Steps
1. `terminal.resize { terminalId, cols: 120, rows: 40 }`

### Expected Results
- `relay.call('pty.resize', { ptyId, cols: 120, rows: 40 })`

---

## TC-TM-001-04: Terminal typing latency < 16ms

**Priority:** P0  
**Type:** Performance

### Steps
1. Measure: keystroke → terminal:output event received

### Expected Results
- P50 latency < 16ms
- P99 latency < 50ms

---

## TC-TM-001-05: PTY session cleanup khi worktree bị xóa

**Priority:** P1

### Steps
1. Tạo worktree với PTY session
2. Xóa worktree
3. Kiểm tra PTY bị destroy

### Expected Results
- `relay.call('pty.destroy', { ptyId })` được gọi
- DB: `orca_terminal_sessions` row deleted

