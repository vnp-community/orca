# TC-SSH-004 — Auto Port Forwarding

**BL Reference:** BL-SSH-04  
**Priority:** P1  
**Type:** Integration  
**Actor:** Carlos, QA

---

## TC-SSH-004-01: Port forwarding — local:3000 → remote:3000

**Priority:** P1

### Steps
1. SSH connected to remote
2. `ssh.forwardPort { localPort: 3000, remotePort: 3000 }`
3. Test connection on localhost:3000

### Expected Results
- Local port 3000 forwards to remote port 3000
- `curl localhost:3000` reaches remote app

### Assertions
```
await rpc.call('ssh.forwardPort', { localPort: 3000, remotePort: 3000 })
response = await fetch('http://localhost:3000')
assert response.status === 200 // remote app responds
```

---

## TC-SSH-004-02: Auto port forwarding từ Orca config

**Priority:** P1

### Steps
1. Project config: `portForwarding: [{ local: 3000, remote: 3000 }]`
2. SSH connect → ports auto-forwarded

### Expected Results
- Auto-forward applied trên connect

---

## TC-SSH-004-03: Port conflict — local port in use

**Priority:** P1

### Steps
1. `ssh.forwardPort { localPort: 80, remotePort: 3000 }`
2. Local port 80 đã bị occupied

### Expected Results
- Error: `{ code: 'PORT_IN_USE', port: 80 }`

