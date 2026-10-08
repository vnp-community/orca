# TC-CR-001 — Xem Diff của Agent Changes

**BL Reference:** BL-CR-01  
**Priority:** P1  
**Type:** Integration  
**Actor:** Alex, Maya, QA

---

## TC-CR-001-01: Xem unified diff

**Priority:** P1

### Steps
1. Agent đã thay đổi files trong worktree
2. `review.getDiff { worktreeId, baseRef: 'main' }`

### Expected Results
- `relay.call('git.exec', { args: ['diff', 'main...HEAD'] })`
- Unified diff returned với +/- lines
- Syntax highlighting per file type

---

## TC-CR-001-02: Diff filter — Specific files

**Priority:** P1

### Steps
1. `review.getDiff { worktreeId, files: ['src/auth.ts'] }`

### Expected Results
- Only `auth.ts` diff returned

---

## TC-CR-001-03: Diff statistics — Summary

**Priority:** P1

### Steps
1. `review.getDiffStats { worktreeId }`

### Expected Results
- `{ filesChanged: 5, insertions: 120, deletions: 30 }`

