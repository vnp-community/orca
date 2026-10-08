# TC-INT-003 — Preflight Status Merge

**BL Reference:** BL-INT-03  
**Priority:** P1

---

## TC-INT-003-01: Merge relay CLI + local API checks

### Steps
1. GitHub: relay check (`gh auth status`) → authenticated
2. Bitbucket: local API check (token stored) → valid

### Expected Results
- `mergePreflightStatuses()` combines both
- Priority: relay CLI > local API
- Output: `{ github: 'ok', bitbucket: 'ok' }`

---

## TC-INT-003-02: Relay check fails — Fallback to local

### Steps
1. GitHub relay check fails (SSH offline)
2. Local API check: token stored → valid

### Expected Results
- Fallback: use local API check result
- Output: `{ github: 'fallback-ok' }`

