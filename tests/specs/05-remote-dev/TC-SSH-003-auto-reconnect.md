# TC-SSH-003 — SSH Auto-Reconnect

**BL Reference:** BL-SSH-03  
**Priority:** P1  
**Type:** Integration  
**Actor:** Carlos

---

## TC-SSH-003-01: Auto-reconnect sau network drop

**Priority:** P1

### Steps
1. SSH connected
2. Simulate network drop: close TCP connection
3. Wait for reconnect attempts

### Expected Results
- SSH client detect disconnect
- Reconnect attempt 1 sau 2s (exponential backoff)
- Reconnect attempt 2 sau 4s
- Reconnect attempt 3 sau 8s
- On success: event `ssh:reconnected { hostId }`

### Assertions
```
connectTimestamps = []
onReconnectAttempt = (attempt) => connectTimestamps.push({ attempt, time: Date.now() })

simulateNetworkDrop(hostId)
await waitForReconnect()

// Verify exponential backoff
gaps = connectTimestamps.map((t, i) => i > 0 ? t.time - connectTimestamps[i-1].time : 0).slice(1)
assert gaps[0] >= 2000 // 2s
assert gaps[1] >= 4000 // 4s
```

---

## TC-SSH-003-02: Max reconnect attempts exceeded

**Priority:** P1

### Steps
1. SSH disconnected
2. Các lần reconnect đều fail
3. Kiểm tra state sau N attempts

### Expected Results
- Sau max attempts: status='unreachable'
- UI: error banner "Cannot reconnect to dev.example.com"
- Không tiếp tục retry

