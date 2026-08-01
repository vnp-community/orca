# TC-SSH-002 — Deploy Orca Relay Binary

**BL Reference:** BL-SSH-02  
**Priority:** P1  
**Type:** Integration  
**Actor:** Carlos, DevOps

---

## TC-SSH-002-01: Auto-deploy relay — Happy Path

**Priority:** P1

### Steps
1. SSH connect thành công
2. Orca detect relay không có trên remote
3. Auto-deploy: upload relay binary via SFTP
4. Execute relay: `./orca-relay --port 6799`

### Expected Results
- SFTP upload relay binary tới `~/orca-relay`
- `chmod +x ~/orca-relay`
- `./orca-relay --port 6799` spawned
- Relay WebSocket: `ws://remote:6799/orca-relay` accessible

---

## TC-SSH-002-02: Relay binary đã tồn tại và up-to-date

**Priority:** P1

### Steps
1. Relay binary đã exist, version match
2. Deploy request

### Expected Results
- SFTP upload SKIP
- Existing relay restarted

---

## TC-SSH-002-03: Relay binary outdated — Update

**Priority:** P1

### Steps
1. Remote có relay binary v1.0, Orca cần v1.1
2. Deploy request

### Expected Results
- Upload new binary (overwrite)
- Restart relay

---

## TC-SSH-002-04: SFTP permission denied

**Priority:** P1

### Steps
1. Remote: `~/orca-relay` directory không có write permission

### Expected Results
- Error: `{ code: 'RELAY_DEPLOY_FAILED', reason: 'SFTP permission denied' }`

