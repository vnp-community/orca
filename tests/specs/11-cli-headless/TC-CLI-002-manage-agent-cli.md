# TC-CLI-002 — Quản lý Agent qua CLI

**BL Reference:** BL-CLI-02  
**Priority:** P1

---

## TC-CLI-002-01: `orca agent start`

### Steps
1. `orca agent start --worktree wt-123 --agent claude`

### Expected Results
- Agent started
- Output: `Agent claude started in wt-123 (session: sess-abc)`

---

## TC-CLI-002-02: `orca agent stop`

### Steps
1. `orca agent stop --session sess-abc`

### Expected Results
- Agent stopped
- Exit code 0

---

## TC-CLI-002-03: `orca agent status`

### Steps
1. `orca agent status --project proj-123`

### Expected Results
- Table: sessionId, agentType, worktree, status, startedAt

