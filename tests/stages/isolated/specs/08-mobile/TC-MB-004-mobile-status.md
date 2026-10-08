# TC-MB-004 — Xem Agent Status từ Mobile

**BL Reference:** BL-MB-04  
**Priority:** P1  
**Type:** Integration  
**Actor:** Sam, Carlos

---

## TC-MB-004-01: Xem danh sách agents đang chạy

**Priority:** P1

### Steps
1. Mobile connect (paired)
2. Request: `{ command: 'get_agent_status' }`
3. Orca Server trả danh sách agents

### Expected Results
- Response: list agents với status, worktreeName, agentType, startedAt

---

## TC-MB-004-02: Real-time status updates từ mobile

**Priority:** P1

### Steps
1. Mobile subscribe (paired)
2. Agent status changes (running → completed)
3. Verify mobile nhận update

### Expected Results
- Mobile nhận `agent:statusChanged` events via E2E WS

---

## TC-MB-004-03: Mobile view khi không có agent running

**Priority:** P1

### Steps
1. Không có agent nào đang chạy
2. Mobile request status

### Expected Results
- Empty list: `{ agents: [] }`
- UI: "No agents currently running"

