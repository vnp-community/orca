# TC-PI-004 — Submit PR Review lên GitHub

**BL Reference:** BL-PI-04  
**Priority:** P1

---

## TC-PI-004-01: Approve PR

**Priority:** P1

### Steps
1. `review.submitPRReview { worktreeId, prNumber: 42, action: 'approve' }`

### Expected Results
- GitHub API: `POST /repos/.../pulls/42/reviews { event: 'APPROVE' }`

---

## TC-PI-004-02: Request changes với comments

**Priority:** P1

### Steps
1. `review.submitPRReview { action: 'request_changes', body: '...', comments: [...] }`

### Expected Results
- GitHub API called với comments array

