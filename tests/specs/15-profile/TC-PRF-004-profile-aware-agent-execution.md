# TC-PRF-004 — Profile-Aware Agent Execution Routing

**BL Reference:** BL-PRF-04  
**Priority:** P0  
**Type:** Integration  
**Actor:** Developer, Lead

---

## TC-PRF-004-01: ORCA_PROJECT_ID inject vào agent env

**Priority:** P0

### Steps
1. Start agent cho project-123
2. Verify env

### Expected Results
- `agent.spawn.env.ORCA_PROJECT_ID === 'project-123'`

---

## TC-PRF-004-02: Profile env vars inject

**Priority:** P0

### Steps
1. User profile có `envVars: { MY_TOKEN: 'abc' }`
2. Start agent

### Expected Results
- `agent.spawn.env.MY_TOKEN === 'abc'`

---

## TC-PRF-004-03: Locked model prevention

**Priority:** P0  
**Security:**

### Steps
1. Company locks `gpt-3.5`
2. User tries to start agent với `model: 'gpt-3.5'`

### Expected Results
- Error: `{ code: 'MODEL_LOCKED_BY_POLICY', model: 'gpt-3.5' }`
- Agent NOT spawned

### Assertions
```
result = await ipc.invoke('agent.start', { agentType: 'openai', model: 'gpt-3.5' }).catch(e => e)
assert result.code === 'MODEL_LOCKED_BY_POLICY'
```

