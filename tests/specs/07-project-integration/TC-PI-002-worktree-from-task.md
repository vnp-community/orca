# TC-PI-002 — Tạo Worktree từ Issue/Task

**BL Reference:** BL-PI-02  
**Priority:** P1  
**Type:** Integration

---

## TC-PI-002-01: Create worktree từ GitHub issue #42

**Priority:** P1

### Steps
1. Issue #42: "Fix login bug"
2. `integration.createWorktreeFromIssue { issueId: 42, provider: 'github' }`

### Expected Results
- Branch name convention: `fix/issue-42-fix-login-bug`
- Worktree created với này branch
- Issue #42 linked to worktreeId in DB

---

## TC-PI-002-02: Branch naming — Slug từ issue title

**Priority:** P1

| Issue Title | Expected Branch |
|-------------|----------------|
| "Fix login bug" | `fix/issue-42-fix-login-bug` |
| "Add user profile" | `feat/issue-99-add-user-profile` |
| "URGENT: prod down!" | `fix/issue-1-urgent-prod-down` |

