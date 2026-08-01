# TASK-FE-TM-002-B: Restore scrollback snapshot sau reconnect (TM-002)

**Domain:** terminal-management.  
**Solution Ref:** SOL-FE-TM-002 Phần 2  
**Bug:** BUG-FE-TM-002  
**Priority:** 🟠 P1  
**Estimated:** 30 phút  
**Status:** ✅ DONE — Implemented via serializeBuffer in disconnect()

---

## Mục tiêu

Sau khi `terminal.create` thành công, gọi `terminal.snapshot.restore` để khôi phục scrollback buffer vào xterm.js terminal.

---

## Files cần sửa

- `src/renderer/src/components/terminal-pane/remote-runtime-pty-transport.ts`

---

## Các bước thực thi

Sau khi `callRuntime('terminal.create', ...)` trả về `remotePtyId`, thêm restore:

```typescript
// Sau dòng: remotePtyId = result.ptyId (hoặc tương đương)
// === Snapshot restore (BUG-FE-TM-002 fix) ===
try {
  const snapshot = await callRuntime<{ data: string | null }>(
    'terminal.snapshot.restore',
    { handle: remotePtyId }
  )
  if (snapshot.data && term && !term.element?.dataset.snapshotRestored) {
    // Write snapshot data vào terminal
    term.write(snapshot.data)
    // Scroll to bottom sau khi restore
    term.scrollToBottom()
    if (term.element) {
      term.element.dataset.snapshotRestored = 'true'
    }
  }
} catch (err) {
  // Non-fatal: snapshot không tồn tại → skip
  console.debug('[PTY] No snapshot to restore:', err)
}
```

**Lưu ý:** Cần access `term` (Terminal instance) từ closure. Kiểm tra cách hiện tại transport access xterm instance.

```bash
grep -n "term\.\|terminal\." \
  src/renderer/src/components/terminal-pane/remote-runtime-pty-transport.ts | head -20
```

---

## Verify

```bash
grep -n "terminal.snapshot.restore\|scrollToBottom" \
  src/renderer/src/components/terminal-pane/remote-runtime-pty-transport.ts
```

## Depends on
TASK-FE-TM-002-A

## Blocking
TASK-FE-TM-002-C
