# BUG-FE-TM-002: Browser không gọi `terminal.snapshot.save` khi đóng tab — thiếu Scrollback Persistence (BL-TM-03)

## Mức độ: HIGH

## Tóm tắt

HLD (BL-TM-03) mô tả Scrollback Persistence:
```
Khi tab đóng → callRuntime('terminal.snapshot.save', { handle })
    → Backend lưu vào terminal_scrollback_snapshots (SQLite, max 50MB)
Khi mở lại → callRuntime('terminal.snapshot.restore', { handle })
    → Browser restore output + cursor position + attributes
```

Khi rà soát `remote-runtime-pty-transport.ts`, **không tìm thấy** bất kỳ lời gọi `terminal.snapshot.save` hay `terminal.snapshot.restore` nào.

## File liên quan

- [`src/renderer/src/components/terminal-pane/remote-runtime-pty-transport.ts`](file:///Users/binhnt/Work/blockchain/vnp-blc/orca/src/renderer/src/components/terminal-pane/remote-runtime-pty-transport.ts) — `disconnect()` method (Lines 730-748)

## Code thực tế

```typescript
// Lines 730-748 — disconnect() không có snapshot save
disconnect() {
  inputBatcher.flush()
  inputBatcher.clear()
  viewportBatcher.flush()
  outputProcessor.clearAccumulatedState()
  if (!connected && !handle) {
    return
  }
  connected = false
  clearPendingViewportClaim()
  const id = remotePtyId
  closeMultiplexedStream()
  handle = null
  remotePtyId = null
  storedCallbacks.onDisconnect?.()
  if (id) {
    onPtyExit?.(id)
  }
  // ← THIẾU: callRuntime('terminal.snapshot.save', { handle })
}
```

Tương tự, không có `terminal.snapshot.restore` khi `connect()` hoặc `attach()`.

## Ảnh hưởng

1. **BL-TM-03 không hoạt động**: Terminal history bị mất hoàn toàn khi đóng tab.
2. Chỉ có relay-side replay buffer (100KB rolling) được dùng khi `pty.attach` — nhưng khi PTY exit và user mở lại worktree, không có gì để restore.
3. `terminal_scrollback_snapshots` table (nếu tồn tại trong SQLite) không bao giờ được populate.

## Cách fix đề xuất

Trong `disconnect()`:
```typescript
async disconnect() {
  // ... existing code

  // Save scrollback snapshot trước khi disconnect
  if (handle) {
    // Serialize xterm.js state
    const snapshot = serializeTerminal() // xterm serialize addon
    try {
      await callRuntime('terminal.snapshot.save', { 
        terminal: handle, 
        data: snapshot 
      })
    } catch {
      // Best-effort — không block disconnect
    }
  }
  
  // ... rest of disconnect
}
```

## Liên quan đến luồng

- **BL-TM-03**: Scrollback Persistence — hoàn toàn chưa implement ở Browser layer.
- **BR-TM-11**: Restore output + cursor position + text attributes — không hoạt động.
