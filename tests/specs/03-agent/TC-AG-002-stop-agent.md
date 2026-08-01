# TC-AG-002 — Dừng Agent

**BL Reference:** BL-AG-02  
**Flow Reference:** docs/flows/logic/agent-orchestration.md#BL-AG-02  
**Priority:** P0  
**Type:** Integration  
**Actor:** Alex, Maya, Carlos, Sam

---

## TC-AG-002-01: Dừng agent gracefully — Ctrl+C

**Priority:** P0

### Preconditions
- Agent đang chạy (status='running'), sessionId active

### Steps
1. RPC: `agent.stop { sessionId, force: false }`
2. Verify JSON-RPC call
3. Wait for exit

### Expected Results
- JSON-RPC: `agent.sendInput { ptyId, data: '\x03' }` (Ctrl+C) gửi tới Dev Server
- Dev Server: `ptyHandle.write('\x03')` → PTY stdin
- Đợi max 10s cho graceful exit
- PTY close event stream về: `agent.exit { ptyId, code }`
- DB: UPDATE `orca_sessions SET status='stopped'`
- Event: `agent:stopped { sessionId }`

### Assertions
```
await ipc.invoke('agent.stop', { sessionId, force: false })
assert spyJsonRpc.calledWith('agent.sendInput', { ptyId, data: '\x03' })
session = db.sessions.find({ id: sessionId })
assert session.status === 'stopped'
event = await events.next('agent:stopped')
assert event.sessionId === sessionId
```

---

## TC-AG-002-02: Graceful timeout → Force kill dialog

**Priority:** P0

### Preconditions
- Agent không respond Ctrl+C trong 10s

### Steps
1. `agent.stop { sessionId, force: false }`
2. Mock: PTY không emit exit trong 10s
3. Verify dialog event

### Expected Results
- Sau 10s timeout: emit `agent:stopTimeout { sessionId }` → UI hiện "Force Kill?" dialog
- User chọn force kill → `agent.stop { sessionId, force: true }`

### Assertions
```
mockPtyIgnoreCtrlC(ptyId)
await ipc.invoke('agent.stop', { sessionId, force: false })
await delay(10000 + 100)
event = await events.next('agent:stopTimeout')
assert event.sessionId === sessionId
```

---

## TC-AG-002-03: Force kill — SIGKILL

**Priority:** P0

### Steps
1. `agent.stop { sessionId, force: true }`

### Expected Results
- JSON-RPC: `agent.kill { ptyId, signal: 'SIGKILL' }` gửi tới Dev Server
- Dev Server: `ptyHandle.kill('SIGKILL')` → immediate terminate
- Dev Server: xóa `ptySessionStore[userId + worktreeId]`

### Assertions
```
await ipc.invoke('agent.stop', { sessionId, force: true })
assert spyJsonRpc.calledWith('agent.kill', { ptyId, signal: 'SIGKILL' })
assert devServer.ptyStore[userId + worktreeId] === undefined
```

---

## TC-AG-002-04: Dừng agent đã stopped rồi

**Priority:** P1

### Steps
1. `agent.stop { sessionId }` lần 1 → success
2. `agent.stop { sessionId }` lần 2

### Expected Results
- Lần 2: Error `{ code: 'AGENT_NOT_RUNNING' }` hoặc no-op

---

## TC-AG-002-05: Dừng agent từ Mobile (Sam)

**Priority:** P1

### Preconditions
- Sam's mobile paired với desktop
- Agent đang chạy, được monitor từ mobile

### Steps
1. Mobile dispatch: `{ command: 'stop_agent', sessionId }`
2. Verify agent bị stop

### Expected Results
- Orca Server nhận dispatch từ mobile qua E2E encrypted channel
- `agent.stop` được gọi
- Mobile nhận confirmation

---

*TC-AG-002 — Orca v5.0 — 2026-08-01*
