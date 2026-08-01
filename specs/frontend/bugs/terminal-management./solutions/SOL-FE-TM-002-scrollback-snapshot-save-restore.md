# SOL-FE-TM-002: Implement Scrollback Snapshot Save/Restore (`terminal.snapshot.save/restore`)

## Bug Reference
- **Bug:** BUG-FE-TM-002
- **Mức độ:** HIGH
- **File:** `src/renderer/src/components/terminal-pane/remote-runtime-pty-transport.ts` Lines 730-748
- **TDD Reference:** TDD-FE-04 §3.4 PtyConnection (Session snapshot), §2 xterm.js Integration (SerializeAddon)

---

## Root Cause

`disconnect()` method không có `terminal.snapshot.save` call:

```typescript
// Lines 730-748 — THIẾU snapshot save
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
  // ← THIẾU: callRuntime('terminal.snapshot.save', { handle, data })
}
```

---

## Giải pháp

### Phần 1: Serialize terminal state trước khi disconnect

`TDD-FE-04 §2` cho thấy `SerializeAddon` đã được load:
```typescript
const serialize = new SerializeAddon()
term.loadAddon(serialize)
```

`disconnect()` cần truy cập SerializeAddon để serialize state.

**File:** `src/renderer/src/components/terminal-pane/remote-runtime-pty-transport.ts`

```typescript
// Thêm reference tới xterm SerializeAddon (truyền vào qua factory/options):
interface RemoteRuntimePtyTransportOptions {
  // ... existing options ...
  /** SerializeAddon instance for snapshot (BL-TM-03) */
  serializeAddon?: SerializeAddon
}

// Trong factory function, store reference:
let serializeAddon: SerializeAddon | undefined = options.serializeAddon
```

### Phần 2: Async `disconnect()` với snapshot save

```typescript
// MODIFY disconnect() — thêm async snapshot save (best-effort)
async disconnect() {
  inputBatcher.flush()
  inputBatcher.clear()
  viewportBatcher.flush()
  outputProcessor.clearAccumulatedState()

  // === BUG-FE-TM-002 FIX: Save scrollback snapshot TRƯỚC khi disconnect ===
  if (handle && serializeAddon) {
    try {
      // Serialize xterm.js state (TDD-FE-04 §2 — SerializeAddon)
      const serialized = serializeAddon.serialize({
        rows: 1000,       // last 1000 lines of scrollback
        excludeAltBuffer: true,
      })

      if (serialized && serialized.length > 0) {
        // Best-effort: không block disconnect nếu save fails
        await Promise.race([
          callRuntime('terminal.snapshot.save', {
            terminal: handle,
            data: serialized,
            encoding: 'utf-8',
          }),
          // Timeout 3s — nếu quá lâu thì skip
          new Promise((_, reject) => setTimeout(() => reject(new Error('snapshot timeout')), 3000)),
        ])
      }
    } catch {
      // Best-effort — không block disconnect
      // (snapshot lỗi không phải lỗi nghiêm trọng)
    }
  }
  // === END FIX ===

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
}
```

### Phần 3: Restore snapshot khi `connect()` hoặc `attach()`

```typescript
// Thêm vào connect() sau khi PTY ready — restore snapshot (BL-TM-03 §BR-TM-11)
async function tryRestoreSnapshot(terminalHandle: string): Promise<void> {
  if (!serializeAddon) return
  try {
    const response = await callRuntime<{ data: string | null }>(
      'terminal.snapshot.restore',
      { terminal: terminalHandle }
    )
    if (response.data) {
      // Write serialized data back to xterm.js
      // (xterm supports writing ANSI sequences directly)
      storedCallbacks.onRestoreSnapshot?.(response.data)
    }
  } catch {
    // No snapshot or restore failed — start fresh
  }
}

// Gọi trong connect() sau khi connected:
// await tryRestoreSnapshot(handle)
// Gọi trong attach() sau khi reattached:
// await tryRestoreSnapshot(handle)
```

### Phần 4: Callback trong TerminalPane để apply snapshot

**File:** `src/renderer/src/components/terminal-pane/TerminalPane.tsx` (MODIFY)

```typescript
// Trong createRemoteRuntimePtyTransport() options:
onRestoreSnapshot: (data: string) => {
  // Write ANSI data back to xterm terminal
  // SerializeAddon output là ANSI sequences — có thể write trực tiếp
  term.write(data)
},
```

---

## Orca Server: `terminal.snapshot.save` và `terminal.snapshot.restore`

**File:** `src/main/workspace/WorkspaceService.ts` (MODIFY)

```typescript
// Handler: terminal.snapshot.save
async function handleSnapshotSave(opts: {
  terminal: string   // handle
  data: string       // serialized ANSI content
  encoding: string
}): Promise<void> {
  const session = await db.get(
    'SELECT session_id FROM orca_terminal_sessions WHERE pty_id = ?',
    [opts.terminal]
  )
  if (!session) return  // no session → skip silently

  const snapshotId = generateId()
  const compressed = await gzip(Buffer.from(opts.data, opts.encoding))

  // Check size limit: max 50MB (BL-TM-03)
  if (compressed.byteLength > 50 * 1024 * 1024) {
    throw { code: 'SNAPSHOT_TOO_LARGE', message: 'Scrollback snapshot exceeds 50MB limit' }
  }

  await db.run(`
    INSERT OR REPLACE INTO terminal_scrollback_snapshots
      (snapshot_id, session_id, data_compressed, created_at)
    VALUES (?, ?, ?, ?)
  `, [snapshotId, session.session_id, compressed, Date.now()])

  await db.run(
    'UPDATE orca_terminal_sessions SET snapshot_id = ? WHERE session_id = ?',
    [snapshotId, session.session_id]
  )
}

// Handler: terminal.snapshot.restore
async function handleSnapshotRestore(opts: { terminal: string }): Promise<{ data: string | null }> {
  const row = await db.get(`
    SELECT s.data_compressed
    FROM terminal_scrollback_snapshots s
    JOIN orca_terminal_sessions ts ON ts.snapshot_id = s.snapshot_id
    WHERE ts.pty_id = ?
  `, [opts.terminal])

  if (!row) return { data: null }

  const decompressed = await gunzip(row.data_compressed)
  return { data: decompressed.toString('utf-8') }
}
```

---

## Database Schema

```sql
CREATE TABLE IF NOT EXISTS terminal_scrollback_snapshots (
  snapshot_id  TEXT PRIMARY KEY,
  session_id   TEXT NOT NULL,
  data_compressed BLOB NOT NULL,  -- gzip-compressed ANSI content
  created_at   INTEGER NOT NULL,
  FOREIGN KEY (session_id) REFERENCES orca_terminal_sessions(session_id)
);

CREATE INDEX IF NOT EXISTS idx_snapshots_session
  ON terminal_scrollback_snapshots (session_id);
```

---

## Files cần sửa

| File | Action | Change |
|------|--------|--------|
| `src/renderer/src/components/terminal-pane/remote-runtime-pty-transport.ts` | MODIFY | Thêm async snapshot save trong `disconnect()`, restore trong `connect()` |
| `src/renderer/src/components/terminal-pane/TerminalPane.tsx` | MODIFY | Pass `serializeAddon` + handle `onRestoreSnapshot` callback |
| `src/main/workspace/WorkspaceService.ts` | MODIFY | Add handlers `terminal.snapshot.save` và `terminal.snapshot.restore` |
| DB migration | CREATE | `terminal_scrollback_snapshots` table |

---

## Verification

```bash
# Grep verify snapshot save added:
grep -n "snapshot.save" \
  src/renderer/src/components/terminal-pane/remote-runtime-pty-transport.ts

# Grep verify snapshot restore added:
grep -n "snapshot.restore" \
  src/renderer/src/components/terminal-pane/remote-runtime-pty-transport.ts

# Test scenario:
# 1. Open terminal, run some commands
# 2. Close tab (triggers disconnect → snapshot.save)
# 3. Reopen terminal (triggers snapshot.restore)
# 4. Verify: terminal history restored
```

---

## Liên quan

- **BL-TM-03**: Scrollback Persistence ✅ implemented
- **BR-TM-11**: Restore output + cursor position + text attributes ✅ enabled
- **TDD-FE-04**: §2 SerializeAddon, §3.4 PtyConnection snapshot
- **BUG-TM-003**: session persistence (prerequisite cho snapshot_id tracking)
