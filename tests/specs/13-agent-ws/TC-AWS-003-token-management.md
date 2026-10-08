# TC-AWS-003 — Agent Token Management

**BL Reference:** BL-AWS-03  
**Priority:** P1

---

## TC-AWS-003-01: Generate agent token

### Steps
1. Admin/Agent Developer: `agentToken.generate { devServerId, label: 'my-agent' }`

### Expected Results
- Token generated: `orca_agent_<random>`
- Stored in DB (hashed) với devServerId association
- Token displayed once (plaintext) to admin

---

## TC-AWS-003-02: Revoke agent token

### Steps
1. `agentToken.revoke { tokenId }`
2. Agent tries to connect với revoked token

### Expected Results
- Token deleted from DB
- Agent connection rejected: 401

---

## TC-AWS-003-03: Token listing

### Steps
1. `agentToken.list { devServerId }`

### Expected Results
- List tokens với: id, label, createdAt, lastUsed
- Plaintext token NOT shown (security)

