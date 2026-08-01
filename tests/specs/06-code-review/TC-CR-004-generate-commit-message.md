# TC-CR-004 — Tạo Commit Message bằng AI

**BL Reference:** BL-CR-04  
**Priority:** P1  
**Type:** Integration  
**Actor:** Alex, Maya

---

## TC-CR-004-01: Generate commit message từ staged diff

**Priority:** P1

### Steps
1. Stage files
2. `git.generateCommitMessage { projectId }`

### Expected Results
- Staged diff sent to AI provider
- Conventional commit message returned: `feat: implement JWT authentication middleware`

---

## TC-CR-004-02: Commit message — Empty staged diff

**Priority:** P1

### Steps
1. No files staged
2. `git.generateCommitMessage`

### Expected Results
- Error: `{ code: 'NOTHING_TO_COMMIT' }`

