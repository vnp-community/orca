# TC-PI-003 — Cập nhật Trạng thái Issue sau Merge

**BL Reference:** BL-PI-03  
**Priority:** P2

---

## TC-PI-003-01: Auto-close GitHub issue sau PR merge

**Priority:** P2

### Steps
1. Worktree linked to issue #42
2. Worktree merged
3. `integration.updateIssueStatus { worktreeId, status: 'closed' }`

### Expected Results
- GitHub API: `PATCH /repos/.../issues/42 { state: 'closed' }`

