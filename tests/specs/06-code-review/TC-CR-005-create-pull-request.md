# TC-CR-005 — Tạo Pull Request với AI

**BL Reference:** BL-CR-05  
**Priority:** P1  
**Type:** Integration  
**Actor:** Alex, Maya

---

## TC-CR-005-01: Tạo PR via GitHub CLI

**Priority:** P1

### Steps
1. Branch `feature/auth` với commits
2. `review.createPR { worktreeId, title: 'feat: auth', base: 'main', useAI: true }`

### Expected Results
- AI generates PR description từ diff
- `gh pr create --title '...' --body '<ai-description>'` executed via relay
- PR URL returned

---

## TC-CR-005-02: Tạo PR — Chưa có commits

**Priority:** P1

### Steps
1. Branch không có commits vs base

### Expected Results
- Error: `{ code: 'EMPTY_BRANCH' }`

---

## TC-CR-005-03: Tạo PR via API token (Bitbucket)

**Priority:** P1

### Steps
1. Bitbucket credentials configured
2. `review.createPR { provider: 'bitbucket', ... }`

### Expected Results
- Bitbucket API call via WebCredentialStore
- PR created

