# TC-AG-001 — Khởi động AI Agent

**BL Reference:** BL-AG-01  
**Flow Reference:** docs/flows/logic/agent-orchestration.md#BL-AG-01  
**Priority:** P0  
**Type:** Integration  
**Actor:** Alex, Maya, Carlos, Sam (trigger từ desktop)

---

## Preconditions
- User đã login, worktree tồn tại
- Dev Server đã connect WebSocket đến Orca Server (persistent WS)
- AgentConnectionManager có connection cho devServerId
- AI provider credentials resolved (mock hoặc real)

---

## TC-AG-001-01: Khởi động Agent — Happy Path

**Priority:** P0

### Test Data
| Field | Value |
|-------|-------|
| worktreeId | `wt-test-123` |
| agentType | `claude` |
| trustPreset | `default` |

### Steps
1. RPC (qua IPC): `agent.start { worktreeId, agentType: 'claude', trustPreset: 'default' }`
2. Verify JSON-RPC call tới Dev Server
3. Verify PTY spawn và output stream
4. Verify DB insert
5. Verify status event

### Expected Results
- `AgentConnectionManager.getConnection(devServerId)` trả về existing WS connection
- JSON-RPC `agent.spawn` gửi tới Dev Server
- Dev Server: `node-pty.spawn('claude', args, { cwd: worktreePath })`
- DB: INSERT `orca_sessions` với `agentType='claude'`, `status='idle'`
- Event: `agent:started { sessionId, status: 'idle' }`

### Assertions
```
spyJsonRpc.reset()
await ipc.invoke('agent.start', { worktreeId, agentType: 'claude', trustPreset: 'default' })

// JSON-RPC call to Dev Server
assert spyJsonRpc.calledWith('agent.spawn', {
  agentBinary: 'claude',
  cwd: worktree.path,
  userId: user.id
})

// DB
session = db.sessions.findLatest({ worktreeId })
assert session.agentType === 'claude'
assert session.status === 'idle'
assert session.devServerId === devServer.id

// Event
event = await events.next('agent:started')
assert event.sessionId === session.id
assert event.status === 'idle'
```

---

## TC-AG-001-02: Profile injection — env vars từ resolved profile

**Priority:** P0

### Preconditions
- User profile có `envVars: { CUSTOM_VAR: 'hello', NODE_ENV: 'development' }`
- Company profile có `envVars: { LOG_LEVEL: 'info' }`

### Steps
1. ProfileResolver.resolve(userId) → merged env vars
2. `agent.start { ... }`
3. Verify env vars trong JSON-RPC spawn call

### Expected Results
- `agent.spawn.env` chứa cả User + Company env vars
- User env > Company env (override)

### Assertions
```
await ipc.invoke('agent.start', { worktreeId, agentType: 'claude' })
spawnCall = spyJsonRpc.lastCall('agent.spawn')
assert spawnCall.args.env.CUSTOM_VAR === 'hello'
assert spawnCall.args.env.NODE_ENV === 'development'
assert spawnCall.args.env.LOG_LEVEL === 'info'
```

---

## TC-AG-001-03: AI Provider Resolution — credentials inject

**Priority:** P0

### Preconditions
- AI Provider account: Anthropic, scope=project, apiKey='sk-ant-...'
- API key stored encrypted trên Dev Server

### Steps
1. `agent.start { agentType: 'claude', ... }`
2. Verify provider resolution

### Expected Results
- `AIProviderResolver.resolve(userId, projectId, devServerId)` returns `{ apiKeyEnvVar: 'ANTHROPIC_API_KEY', ... }`
- API key đọc từ `.enc` file trên Dev Server (không lưu trên Orca Server)
- `ANTHROPIC_API_KEY` inject vào agent env

---

## TC-AG-001-04: Dev Server không có WS connection

**Priority:** P1

### Preconditions
- AgentConnectionManager không có connection cho devServerId

### Steps
1. `agent.start { ... }`

### Expected Results
- Error: `{ code: 'DEV_SERVER_NOT_CONNECTED', devServerId }`

### Assertions
```
AgentConnectionManager.removeConnection(devServerId)
result = await ipc.invoke('agent.start', { worktreeId }).catch(e => e)
assert result.code === 'DEV_SERVER_NOT_CONNECTED'
```

---

## TC-AG-001-05: HMAC handshake — Dev Server verify RpcExecutionContext

**Priority:** P1  
**Security:** Xác thực JSON-RPC request

### Steps
1. `agent.spawn` JSON-RPC được gửi tới Dev Server
2. Dev Server verify HMAC-SHA256 trong RpcExecutionContext header
3. Verify TTL ≤ 30s

### Expected Results
- Request với valid HMAC: accepted, agent spawned
- Request với invalid HMAC: rejected
- Request với TTL > 30s: rejected

---

## TC-AG-001-06: OSC 133 parse — Agent status detection

**Priority:** P0

### Steps
1. Agent spawn thành công
2. PTY output stream về Orca Server qua WS
3. PTY emit: `ESC]133;A ST` (command started)
4. PTY emit: `ESC]133;D;0 ST` (command done, exit 0)

### Expected Results
- Status after `133;A`: `'running'`
- Status after `133;D;0`: `'idle'`
- Events: `agent:statusChanged { sessionId, status: 'running' }` và `agent:statusChanged { sessionId, status: 'idle' }`

### Assertions
```
// Simulate PTY output
devServer.emit('agent.output', { ptyId, data: '\x1b]133;A\x07' })
event1 = await events.next('agent:statusChanged')
assert event1.status === 'running'

devServer.emit('agent.output', { ptyId, data: '\x1b]133;D;0\x07' })
event2 = await events.next('agent:statusChanged')
assert event2.status === 'idle'
```

---

## TC-AG-001-07: Nhiều agents cùng user — isolation

**Priority:** P1

### Steps
1. Tạo 2 worktrees: wt-X, wt-Y
2. Start agent trong wt-X
3. Start agent trong wt-Y
4. Kiểm tra 2 agents độc lập

### Expected Results
- 2 PTY handles riêng biệt: `ptySessionStore[userId+wt-X]` và `ptySessionStore[userId+wt-Y]`
- Output của agent X không mix với output của agent Y

---

## TC-AG-001-08: Performance — Agent spawn < 10s

**Priority:** P1  
**Type:** Performance

### Steps
1. Measure: `agent.start` → `agent:started` event

### Expected Results
- Duration < 10,000ms

---

*TC-AG-001 — Orca v5.0 — 2026-08-01*
