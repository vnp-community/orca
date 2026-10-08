# TC-CR-003 — Gửi Feedback về Agent

**BL Reference:** BL-CR-03  
**Priority:** P1  
**Type:** Integration  
**Actor:** Maya, Alex

---

## TC-CR-003-01: Gửi annotations → Agent prompt

**Priority:** P1

### Steps
1. 3 annotations đã tạo
2. `review.sendFeedbackToAgent { worktreeId, sessionId }`

### Expected Results
- Annotations compiled thành structured prompt
- Prompt injected vào agent: `agent.sendInput { ptyId, data: '<feedback prompt>' }`
- Agent processes feedback

---

## TC-CR-003-02: Feedback format

**Priority:** P1

### Expected Results
- Format:
  ```
  Code review feedback:
  - auth.ts:42: Consider using async/await here
  - auth.ts:58: Missing error handling
  Please address these issues.
  ```

