# TC-AT-002 — Chạy Automation theo Schedule

**BL Reference:** BL-AT-02  
**Priority:** P2

---

## TC-AT-002-01: Automation chạy đúng giờ

### Steps
1. Automation: cron='0 2 * * *' (2:00 AM)
2. Advance time to 2:00 AM
3. Verify automation executed

### Expected Results
- `action.execute()` called at 2:00 AM
- Execution logged với timestamp

