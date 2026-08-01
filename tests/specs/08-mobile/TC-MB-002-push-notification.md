# TC-MB-002 — Push Notification khi Agent Complete

**BL Reference:** BL-MB-02  
**Priority:** P0  
**Type:** Integration  
**Actor:** Sam, Carlos (indirect)

---

## TC-MB-002-01: Push notification khi agent complete

**Priority:** P0

### Preconditions
- Sam's mobile paired
- Agent đang chạy

### Steps
1. Agent emit `agent.output` với "Task completed"
2. `AgentHookParser` detect status='completed'
3. Verify push notification sent

### Expected Results
- Orca Server send push via TweetNaCl E2E WS channel
- Mobile nhận notification trong < 5s
- Content: `{ event: 'agent:completed', worktreeName: '...', summary: '...' }`

### Assertions
```
mobileNotifications = []
mobileWs.on('message', msg => mobileNotifications.push(msg))

// Agent completes
simulateAgentCompletion(sessionId)

// Wait for notification
await waitFor(() => mobileNotifications.length > 0, { timeout: 5000 })
notification = decryptMobileNotification(mobileNotifications[0])
assert notification.event === 'agent:completed'
assert notification.worktreeName !== undefined
```

---

## TC-MB-002-02: Push delivery < 5s

**Priority:** P0  
**Type:** Performance

### Steps
1. Agent completes at T=0
2. Measure: T=0 → mobile push received

### Expected Results
- Delivery time < 5,000ms

---

## TC-MB-002-03: Multiple paired devices — All receive push

**Priority:** P1

### Preconditions
- Sam has 2 devices paired (phone + tablet)

### Steps
1. Agent completes
2. Verify both devices receive push

### Expected Results
- Device 1: receives push
- Device 2: receives push

---

## TC-MB-002-04: No paired device — Push silently skipped

**Priority:** P1

### Preconditions
- No mobile devices paired

### Steps
1. Agent completes
2. Verify no error, silent skip

### Expected Results
- System continues normally, no error thrown

