# TC-MB-003 — Remote Dispatch từ Mobile

**BL Reference:** BL-MB-03  
**Priority:** P1  
**Type:** Integration + Security  
**Actor:** Sam

---

## TC-MB-003-01: Gửi instruction từ mobile — Stop agent

**Priority:** P1

### Steps
1. Sam nhận push: agent đang running
2. Sam tap "Stop Agent" trên mobile app
3. Mobile send dispatch: `{ command: 'stop_agent', sessionId }` (E2E encrypted)
4. Orca Server decrypt và execute

### Expected Results
- Orca Server decrypt dispatch payload
- `agent.stop { sessionId, force: false }` được gọi
- Agent bị dừng
- Mobile nhận confirmation

### Assertions
```
mobile.send(encryptDispatch({ command: 'stop_agent', sessionId }))
await delay(2000)
session = db.sessions.find({ id: sessionId })
assert session.status === 'stopped'
```

---

## TC-MB-003-02: Gửi instruction — Follow-up prompt

**Priority:** P1

### Steps
1. Mobile send: `{ command: 'send_prompt', sessionId, prompt: 'Now add unit tests' }`
2. Verify agent nhận prompt

### Expected Results
- Prompt sent to agent PTY input
- Agent responds to prompt

---

## TC-MB-003-03: Dispatch với invalid session

**Priority:** P1

### Steps
1. Mobile send dispatch với `sessionId: 'nonexistent'`

### Expected Results
- Error response: `{ error: 'SESSION_NOT_FOUND' }`
- Mobile nhận error notification

---

## TC-MB-003-04: Dispatch rejected — unauthorized

**Priority:** P0  
**Security:** Replay attack prevention

### Steps
1. Intercept encrypted dispatch message
2. Replay same message sau 60s

### Expected Results
- Replay rejected (nonce already used hoặc timestamp expired)

