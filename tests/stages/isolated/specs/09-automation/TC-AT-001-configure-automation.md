# TC-AT-001 — Cấu hình Automation

**BL Reference:** BL-AT-01  
**Priority:** P2  
**Actor:** Sam, DevOps

---

## TC-AT-001-01: Tạo cron automation

### Steps
1. `automation.create { name: 'Daily cleanup', schedule: '0 2 * * *', action: 'cleanup_worktrees' }`

### Expected Results
- Automation stored với cron='0 2 * * *'
- Status: 'active'

---

## TC-AT-001-02: Validate cron expression

### Steps
1. `automation.create { schedule: 'invalid-cron', ... }`

### Expected Results
- Error: `{ code: 'INVALID_CRON_EXPRESSION' }`

