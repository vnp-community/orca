# TC-DB-001 — Capture UI Element

**BL Reference:** BL-DB-01  
**Priority:** P1  
**Actor:** Alex, QA

---

## TC-DB-001-01: Capture UI element — HTML + CSS + screenshot

### Steps
1. Open embedded Chromium browser at `http://localhost:3000`
2. Click vào button `.submit-btn`
3. `design.captureElement { selector: '.submit-btn' }`

### Expected Results
- Response:
  ```json
  {
    "html": "<button class='submit-btn'>Submit</button>",
    "css": { "background-color": "#007bff", "color": "#fff", ... },
    "screenshot": "<base64-png>"
  }
  ```

---

## TC-DB-001-02: Capture non-existent element

### Steps
1. `design.captureElement { selector: '.nonexistent' }`

### Expected Results
- Error: `{ code: 'ELEMENT_NOT_FOUND' }`

