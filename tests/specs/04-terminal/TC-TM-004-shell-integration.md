# TC-TM-004 — Shell Integration (OSC 133)

**BL Reference:** BL-TM-04  
**Priority:** P1  
**Type:** Integration  
**Actor:** Alex, Maya

---

## TC-TM-004-01: Command tracking — OSC 133 sequences

**Priority:** P1

### Steps
1. Terminal với bash shell, OSC 133 enabled
2. Gõ command `ls -la`
3. Verify tracking

### Expected Results
- `ESC]133;A ST` before prompt → command start marker
- `ESC]133;D;0 ST` after command → exit code 0
- Command tracking: `{ command: 'ls -la', exitCode: 0, startTime, endTime }`

---

## TC-TM-004-02: Current directory tracking

**Priority:** P1

### Steps
1. `cd /tmp` trong terminal
2. Verify current directory update

### Expected Results
- `ESC]7;file:///tmp ST` (OSC 7) → cwd update
- UI status bar shows `/tmp`

---

## TC-TM-004-03: Kitty keyboard protocol — special keys

**Priority:** P1

### Steps
1. Gửi special key sequences (F1, Ctrl+Alt+Del, etc.)
2. Verify correct encoding

