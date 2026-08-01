# TASK-FE-TM-002-A: Serialize terminal state trước khi disconnect (TM-002)

**Domain:** terminal-management.  
**Solution Ref:** SOL-FE-TM-002 Phần 1  
**Bug:** BUG-FE-TM-002  
**Priority:** 🔴 P0  
**Estimated:** 45 phút  
**Status:** ✅ DONE — Implemented

---

## Mục tiêu

Trong `disconnect()`, serialize xterm.js state bằng `SerializeAddon` và gọi `terminal.snapshot.save` RPC để lưu lên backend trước khi disconnect.

---

## Files cần sửa

- `src/renderer/src/components/terminal-pane/remote-runtime-pty-transport.ts` (Lines 730–748)

---

## Các bước thực thi

### Bước 1: Thêm `serializeAddon` vào options

```typescript
interface RemoteRuntimePtyTransportOptions {
  // ... existing ...
  serializeAddon?: SerializeAddon  // inject từ TerminalPane
}
```

Lưu reference vào closure:
```typescript
let serializeAddon: SerializeAddon | undefined = opts.serializeAddon
```

### Bước 2: Modify `disconnect()`

Thêm snapshot save **trước** khi `connected = false`:

```typescript
async disconnect() {
  // === Snapshot save (BUG-FE-TM-002 fix) ===
  if (remotePtyId && serializeAddon && connected) {
    try {
      const snapshotData = serializeAddon.serialize()
      // Fire-and-forget: không await để không block disconnect
      callRuntime('terminal.snapshot.save', {
        handle: remotePtyId,
        data: snapshotData,
      }).catch(err => console.warn('[PTY] snapshot save failed:', err))
    } catch (err) {
      console.warn('[PTY] serialize failed:', err)
    }
  }
  // === Existing disconnect logic (không thay đổi) ===
  inputBatcher.flush()
  inputBatcher.clear()
  // ... rest of existing disconnect ...
}
```

---

## Verify

```bash
grep -n "terminal.snapshot.save\|serializeAddon" \
  src/renderer/src/components/terminal-pane/remote-runtime-pty-transport.ts
```

## Depends on
Không có

## Blocking
TASK-FE-TM-002-B (restore), TASK-FE-TM-002-C (TerminalPane integration)
