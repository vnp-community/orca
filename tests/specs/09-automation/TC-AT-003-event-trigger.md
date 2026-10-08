# TC-AT-003 — Event-based Automation Trigger

**BL Reference:** BL-AT-03  
**Priority:** P2

---

## TC-AT-003-01: Git push trigger automation

### Steps
1. Automation: trigger='git.push', action='run_tests'
2. Simulate git push event
3. Verify automation triggered

### Expected Results
- Git push event detected
- Automation runs `run_tests` action

