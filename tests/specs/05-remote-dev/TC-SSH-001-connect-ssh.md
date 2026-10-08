# TC-SSH-001 — Kết nối SSH Host

**BL Reference:** BL-SSH-01  
**Flow Reference:** docs/flows/logic/remote-development.md  
**Priority:** P1  
**Type:** Integration  
**Actor:** Carlos, DevOps

---

## TC-SSH-001-01: Kết nối SSH với key authentication

**Priority:** P1

### Steps
1. RPC: `ssh.connect { host: 'dev.example.com', port: 22, username: 'carlos', privateKey: '...' }`
2. Verify SSH handshake

### Expected Results
- SSH connection established
- DB: INSERT `orca_ssh_hosts` với status='connected'
- Event: `ssh:connected { hostId, host, username }`

---

## TC-SSH-001-02: Kết nối SSH với password

**Priority:** P1

### Steps
1. `ssh.connect { host, username, password: '...' }`

### Expected Results
- SSH password auth thành công
- Connection stored với status='connected'

---

## TC-SSH-001-03: SSH key không đúng

**Priority:** P1

### Steps
1. `ssh.connect { ..., privateKey: '<invalid_key>' }`

### Expected Results
- Error: `{ code: 'SSH_AUTH_FAILED' }`

---

## TC-SSH-001-04: Host không reachable

**Priority:** P1

### Steps
1. `ssh.connect { host: '192.168.1.999', ... }`

### Expected Results
- Error: `{ code: 'SSH_UNREACHABLE' }` sau timeout

---

## TC-SSH-001-05: SSH Config file — Host patterns

**Priority:** P1

### Steps
1. SSH config: `Host dev-* User carlos IdentityFile ~/.ssh/id_rsa`
2. Connect với `host: 'dev-server-1'`
3. Verify config applied

### Expected Results
- Username và key được lấy từ SSH config tự động

