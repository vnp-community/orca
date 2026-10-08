# TC-TM-003 — Scrollback Persistence

**BL Reference:** BL-TM-03  
**Priority:** P1  
**Type:** Integration  
**Actor:** Alex, Carlos

---

## TC-TM-003-01: Lưu scrollback khi session close

**Priority:** P1

### Steps
1. Tạo terminal, gõ commands, có output
2. Close terminal session
3. Kiểm tra scrollback được lưu vào DB/file

### Expected Results
- Scrollback buffer lưu vào `orca_terminal_sessions.scrollback` hoặc file
- Serialized state bao gồm output history

---

## TC-TM-003-02: Restore scrollback sau restart

**Priority:** P1

### Steps
1. Orca Server restart
2. User reconnect, mở lại terminal
3. Kiểm tra scrollback history visible

### Expected Results
- Previous output visible trong terminal
- Scroll up hiện history cũ

---

## TC-TM-003-03: Scrollback chính xác — không mất ký tự

**Priority:** P1

### Steps
1. Output 1000 dòng text
2. Close và reopen
3. Verify all 1000 lines preserved

