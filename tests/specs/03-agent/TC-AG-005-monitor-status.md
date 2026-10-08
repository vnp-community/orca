# TC-AG-005 — Monitor Trạng thái Agent Real-time

**BL Reference:** BL-AG-05  
**Flow Reference:** docs/flows/logic/agent-orchestration.md#BL-AG-05  
**Priority:** P0  
**Type:** Integration + Security  
**Actor:** Tất cả users

---

## TC-AG-005-01: OSC 133 status transitions — Full lifecycle

**Priority:** P0

### Steps
1. Agent spawn → status: `'idle'`
2. PTY emit `ESC]133;A ST` → status: `'running'`
3. PTY emit regular text → status unchanged
4. PTY emit `ESC]133;D;0 ST` → status: `'idle'`
5. PTY emit `ESC]133;D;1 ST` → status: `'failed'` (non-zero exit)

### Expected Results
- Correct status transitions at each OSC sequence
- Events emitted correctly

### Assertions
```
// Initial
assert agent.status === 'idle'

// Command started
devServer.emit('agent.output', { ptyId, data: '\x1b]133;A\x07' })
await events.next('agent:statusChanged')
assert agent.status === 'running'

// Command done (success)
devServer.emit('agent.output', { ptyId, data: '\x1b]133;D;0\x07' })
await events.next('agent:statusChanged')
assert agent.status === 'idle'

// Command done (failure)
devServer.emit('agent.output', { ptyId, data: '\x1b]133;D;1\x07' })
await events.next('agent:statusChanged')
assert agent.status === 'failed'
```

---

## TC-AG-005-02: Status "waiting" detection

**Priority:** P1

### Steps
1. PTY emit text: `"Waiting for input..."`

### Expected Results
- Status: `'waiting'`
- Event: `agent:statusChanged { status: 'waiting' }`

---

## TC-AG-005-03: Status "completed" detection

**Priority:** P1

### Steps
1. PTY emit text: `"Task completed successfully"`

### Expected Results
- Status: `'completed'`

---

## TC-AG-005-04: Rate limit pattern detection

**Priority:** P0

### Steps
1. PTY emit text matching RATE_LIMIT_PATTERNS

### Expected Results
- Event: `agent:rateLimited { sessionId, resetAt }`

---

## TC-AG-005-05: Real-time stream — Mobile push

**Priority:** P0

### Preconditions
- Sam's mobile paired via TweetNaCl E2E channel

### Steps
1. Agent status changes to 'completed'
2. Verify mobile receives push

### Expected Results
- Mobile app receives encrypted push: `{ event: 'agent:completed', sessionId }`
- TweetNaCl E2E encryption verified

### Assertions
```
// Mock mobile WS connection
mobileWs = createMockMobileWs(samPairedSession)

// Agent completes
devServer.emit('agent.output', { ptyId, data: 'Task completed' })
await events.next('agent:statusChanged')

// Mobile receives push
push = await mobileWs.next()
decrypted = tweetnacl.box.open(push, nonce, serverPublicKey, samPrivateKey)
payload = JSON.parse(decrypted)
assert payload.event === 'agent:completed'
```

---

## TC-AG-005-06: Multi-agent monitoring — status per sessionId

**Priority:** P1

### Steps
1. 2 agents running (session-X, session-Y)
2. session-X PTY emits completion
3. session-Y PTY emits running

### Expected Results
- session-X: `status='completed'`
- session-Y: `status='running'`
- No cross-contamination

### Assertions
```
// session-X completes
devServer.emit('agent.output', { ptyId: ptyX, data: 'Task completed' })
event = await events.next('agent:statusChanged')
assert event.sessionId === sessionX.id
assert event.status === 'completed'

// session-Y unaffected
assert agent[sessionY.id].status === 'running'
```

---

## TC-AG-005-07: WS persistent connection — Orca connects to Dev Server on startup

**Priority:** P0

### Steps
1. Start Dev Server Agent (service)
2. Dev Server opens WS to Orca: `ws://orca:6768/agent`
3. Verify handshake

### Expected Results
- Dev Server initiates WS connection (client role)
- Handshake: `agent.handshake { agentToken }` → `handshake-ok { sessionId }`
- `AgentConnectionManager` stores connection

### Assertions
```
// Start Dev Server Agent
startDevServerAgent(devServer)
await delay(2000)

// Verify connection registered
conn = AgentConnectionManager.getConnection(devServer.id)
assert conn !== null
assert conn.readyState === WebSocket.OPEN
```

---

*TC-AG-005 — Orca v5.0 — 2026-08-01*
