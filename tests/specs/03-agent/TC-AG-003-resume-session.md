# TC-AG-003 — Resume Agent Session

**BL Reference:** BL-AG-03  
**Flow Reference:** docs/flows/logic/agent-orchestration.md#BL-AG-03  
**Priority:** P1  
**Type:** Integration  
**Actor:** Alex, Maya, Carlos

---

## TC-AG-003-01: Resume Claude session — Happy Path

**Priority:** P1

### Preconditions
- Previous session: `sessionId='sess-abc'`, agentType='claude', worktreeId='wt-123'
- `orca_sessions` có record với sessionId

### Steps
1. RPC: `agent.resume { worktreeId: 'wt-123' }`
2. Verify DB lookup
3. Verify resume args

### Expected Results
- DB: SELECT `sessionId, devServerId` từ `orca_sessions WHERE worktreeId=? ORDER BY startedAt DESC`
- Resume args: `['claude', '--resume', 'sess-abc']`
- JSON-RPC: `agent.spawn { agentBinary: 'claude', args: ['--resume', 'sess-abc'], cwd: wt.path }`

### Assertions
```
await ipc.invoke('agent.resume', { worktreeId: 'wt-123' })
spawnCall = spyJsonRpc.lastCall('agent.spawn')
assert spawnCall.args.args.includes('--resume')
assert spawnCall.args.args.includes('sess-abc')
```

---

## TC-AG-003-02: Resume Codex session — Different resume format

**Priority:** P1

### Preconditions
- Previous session: agentType='codex', sessionFilePath='/path/to/session.json'

### Steps
1. `agent.resume { worktreeId }`

### Expected Results
- Resume args: `['codex', '--session-file', sessionFilePath]`

### Assertions
```
spawnCall = spyJsonRpc.lastCall('agent.spawn')
assert spawnCall.args.args.includes('--session-file')
assert spawnCall.args.args.includes(sessionFilePath)
```

---

## TC-AG-003-03: Resume — không có previous session

**Priority:** P1

### Preconditions
- Worktree không có previous session

### Steps
1. `agent.resume { worktreeId: 'wt-fresh' }`

### Expected Results
- Error: `{ code: 'NO_SESSION_TO_RESUME' }`

---

## TC-AG-003-04: Resume — Latest session được chọn

**Priority:** P1

### Preconditions
- Worktree có 3 sessions: sess-old, sess-mid, sess-latest

### Steps
1. `agent.resume { worktreeId }`

### Expected Results
- `sess-latest` được dùng (ORDER BY startedAt DESC LIMIT 1)

### Assertions
```
await ipc.invoke('agent.resume', { worktreeId })
spawnCall = spyJsonRpc.lastCall('agent.spawn')
assert spawnCall.args.args.includes('sess-latest')
```

---

*TC-AG-003 — Orca v5.0 — 2026-08-01*
