# TC-PI-001 — Import GitHub/GitLab Issues

**BL Reference:** BL-PI-01  
**Priority:** P1  
**Type:** Integration  
**Actor:** Maya, Alex

---

## TC-PI-001-01: Import GitHub issues

**Priority:** P1

### Steps
1. GitHub token configured
2. `integration.importIssues { provider: 'github', repo: 'org/repo', labels: ['bug'] }`

### Expected Results
- GitHub API: `GET /repos/org/repo/issues?labels=bug`
- Issues imported và stored
- Each issue: `{ id, title, body, labels, assignees, number }`

---

## TC-PI-001-02: Import Linear tasks

**Priority:** P1

### Steps
1. Linear token configured
2. `integration.importIssues { provider: 'linear', teamId: '...' }`

### Expected Results
- Linear API called
- Tasks imported

---

## TC-PI-001-03: Pagination — nhiều issues

**Priority:** P1

### Steps
1. Repo có 150 issues
2. Import all

### Expected Results
- Pagination handled (page 1, 2, 3...)
- All 150 issues imported

