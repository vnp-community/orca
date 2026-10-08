# TC-TM-002 — Split Terminal

**BL Reference:** BL-TM-02  
**Priority:** P0  
**Type:** Integration  
**Actor:** Alex, Carlos

---

## TC-TM-002-01: Split horizontal — 2 panes

**Priority:** P0

### Steps
1. Tạo terminal session 1
2. `terminal.split { terminalId, direction: 'horizontal' }`
3. Kiểm tra terminal session 2 được tạo

### Expected Results
- New PTY session created on Dev Server
- 2 terminal sessions active
- UI: 2 panes visible side-by-side

---

## TC-TM-002-02: Split vertical

**Priority:** P1

### Steps
1. `terminal.split { direction: 'vertical' }`

### Expected Results
- New PTY session created
- UI: panes stacked vertically

---

## TC-TM-002-03: Multiple splits — nhiều panes

**Priority:** P1

### Steps
1. Tạo terminal
2. Split → 2 panes
3. Split lại → 3 panes
4. Split lại → 4 panes

### Expected Results
- Mỗi lần split tạo thêm 1 PTY session
- 4 sessions active, tất cả independent

---

## TC-TM-002-04: Close one pane — không affect others

**Priority:** P1

### Steps
1. Tạo 3 panes
2. Close pane 2
3. Kiểm tra pane 1 và 3 vẫn active

### Expected Results
- PTY session của pane 2 bị destroy
- Pane 1 và 3: vẫn active, output không bị interrupted

