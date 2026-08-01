# TC-DB-002 — Inject UI Context vào Agent

**BL Reference:** BL-DB-02  
**Priority:** P1

---

## TC-DB-002-01: Inject captured element vào agent prompt

### Steps
1. Element đã captured (TC-DB-001-01)
2. `design.injectContext { sessionId, elementData }` 
3. Verify agent nhận context

### Expected Results
- Context prepended vào agent prompt:
  ```
  UI Context: HTML=<button class='submit-btn'>...</button>
  CSS=background:#007bff; ...
  [screenshot attached]
  ```
- `agent.sendInput` với context payload

