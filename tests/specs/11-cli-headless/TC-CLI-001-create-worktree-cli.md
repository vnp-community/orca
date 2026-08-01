# TC-CLI-001 — Tạo Worktree qua CLI

**BL Reference:** BL-CLI-01  
**Priority:** P1  
**Actor:** DevOps, Alex

---

## TC-CLI-001-01: `orca worktree create` — Basic

### Steps
1. `orca worktree create --project proj-123 --branch main --name feat-auth`

### Expected Results
- Exit code: 0
- Output: `Created worktree feat-auth at /path/to/worktree`
- Worktree created via API

---

## TC-CLI-001-02: CLI exit codes

| Scenario | Expected Exit Code |
|----------|-------------------|
| Success | 0 |
| Invalid args | 1 |
| Auth failed | 2 |
| Server error | 3 |

---

## TC-CLI-001-03: `orca worktree list`

### Expected Results
- Table output của worktrees
- Columns: Name, Branch, Status, Created

