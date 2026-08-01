# SOL-TM-003: Terminal Session Persistence — Lưu sessionId vào DB khi tạo PTY

## Bug Reference
- **Bug:** BUG-TM-003 [FRONTEND]
- **Mức độ:** 🟡 MEDIUM
- **TDD Reference:** TDD-FE-04 §3.4 PtyConnection (Session snapshot, reattach), TDD-FE-03 (Runtime Client Layer)

---

## Root Cause

`remote-runtime-pty-transport.ts` chỉ lưu state in-memory:
```typescript
let remotePtyId: string | null = null  // lost on reload
```

Khi browser reload hoặc WebSocket reconnect:
1. `remotePtyId` bị mất → không thể reattach
2. PTY process tiếp tục chạy → resource leak
3. `orca_terminal_sessions` không bao giờ được INSERT → BL-TM-03 không hoạt động

Root cause sâu hơn: Orca Server cần INSERT `orca_terminal_sessions` sau khi `relay.call('pty.create')` thành công. Fix cần ở cả **Frontend** và **Orca Server**.

---

## Giải pháp

### Part A — Orca Server: INSERT `orca_terminal_sessions`

**File:** `src/main/workspace/WorkspaceService.ts` (MODIFY)

```typescript
// Trong handler terminal.create (hoặc tương đương):
async function handleTerminalCreate(opts: TerminalCreateOptions): Promise<TerminalCreateResult> {
  // 1. Relay call → Dev Server → spawn PTY
  const ptyResult = await relay.call('pty.create', {
    worktreeId: opts.worktreeId,
    command: opts.command ?? '/bin/bash',
    cwd: opts.cwd ?? worktreePath,
    cols: opts.cols ?? 120,
    rows: opts.rows ?? 40,
  })

  // 2. INSERT session vào DB (FIX cho BUG-TM-003)
  const sessionId = generateId()
  await db.run(`
    INSERT INTO orca_terminal_sessions
      (session_id, pty_id, user_id, project_id, worktree_id, created_at)
    VALUES (?, ?, ?, ?, ?, ?)
  `, [sessionId, ptyResult.ptyId, opts.userId, opts.projectId, opts.worktreeId, Date.now()])

  // 3. Return BOTH ptyId AND sessionId → browser persist sessionId
  return {
    ptyId: ptyResult.ptyId,
    sessionId,               // ← NEW: browser sẽ lưu cái này
    handle: ptyResult.handle,
  }
}
```

---

### Part B — Frontend: Persist `sessionId` và dùng để reattach

**File:** `src/renderer/src/components/terminal-pane/remote-runtime-pty-transport.ts` (MODIFY)

```typescript
// 1. Thêm sessionId vào state (Lines 83-88 khu vực)
let connected = false
let destroyed = false
let handle: string | null = null
let remotePtyId: string | null = null
let sessionId: string | null = null  // ← NEW: persist across reconnects
```

#### Sau khi `terminal.create` thành công, lưu sessionId:

```typescript
// Trong connect() function, sau khi callRuntime('terminal.create'):
const created = await callRuntime<{ terminal: RuntimeTerminalCreate; sessionId: string }>(
  'terminal.create',
  { /* params */ }
)
handle = created.terminal.handle
remotePtyId = created.terminal.ptyId
sessionId = created.sessionId  // ← NEW: lưu sessionId

// Persist vào sessionStorage để survive page reload:
if (sessionId) {
  sessionStorage.setItem(`orca_terminal_session_${remotePtyId}`, sessionId)
}
```

#### Restore sessionId sau reconnect:

```typescript
// Trong attach() hoặc reconnect logic:
async function reconnectToExistingSession(storedPtyId: string) {
  const storedSessionId = sessionStorage.getItem(`orca_terminal_session_${storedPtyId}`)
  if (!storedSessionId) {
    // No stored session → create new
    return await connect()
  }

  try {
    // Try reattach with existing sessionId
    const result = await callRuntime<{ terminal: RuntimeTerminalCreate }>(
      'terminal.attach',
      { sessionId: storedSessionId }
    )
    handle = result.terminal.handle
    remotePtyId = storedPtyId
    sessionId = storedSessionId
    connected = true
  } catch (err) {
    // Session expired → clean up and create new
    sessionStorage.removeItem(`orca_terminal_session_${storedPtyId}`)
    await connect()
  }
}
```

---

### Part C — Orca Server: `terminal.attach` handler

**File:** `src/main/workspace/WorkspaceService.ts` (MODIFY)

```typescript
// Handler cho 'terminal.attach' RPC từ Browser:
async function handleTerminalAttach(opts: { sessionId: string }): Promise<TerminalCreateResult> {
  // 1. Lookup session trong DB
  const session = await db.get(`
    SELECT * FROM orca_terminal_sessions WHERE session_id = ?
  `, [opts.sessionId])

  if (!session) {
    throw { code: 'SESSION_NOT_FOUND', message: 'Terminal session not found or expired' }
  }

  // 2. Check PTY còn alive trên Dev Server
  try {
    const attachResult = await relay.call('pty.attach', { ptyId: session.pty_id })
    // Update last_seen
    await db.run(
      'UPDATE orca_terminal_sessions SET last_seen_at = ? WHERE session_id = ?',
      [Date.now(), opts.sessionId]
    )
    return {
      ptyId: session.pty_id,
      sessionId: opts.sessionId,
      handle: attachResult.handle,
    }
  } catch {
    // PTY dead → cleanup DB record
    await db.run('DELETE FROM orca_terminal_sessions WHERE session_id = ?', [opts.sessionId])
    throw { code: 'PTY_NOT_ALIVE', message: 'PTY process no longer running on dev server' }
  }
}
```

---

### Part D — Database Schema

**File:** DB migration (SQLite schema):

```sql
-- Thêm table orca_terminal_sessions nếu chưa có:
CREATE TABLE IF NOT EXISTS orca_terminal_sessions (
  session_id   TEXT PRIMARY KEY,
  pty_id       TEXT NOT NULL,
  user_id      TEXT NOT NULL,
  project_id   TEXT NOT NULL,
  worktree_id  TEXT,
  created_at   INTEGER NOT NULL,
  last_seen_at INTEGER,
  snapshot_id  TEXT   -- FK → terminal_scrollback_snapshots (BL-TM-03)
);

CREATE INDEX IF NOT EXISTS idx_tm_sessions_pty
  ON orca_terminal_sessions (pty_id);

CREATE INDEX IF NOT EXISTS idx_tm_sessions_user
  ON orca_terminal_sessions (user_id);
```

---

### Part E — Orphan PTY Cleanup

Để tránh resource leak, Orca Server cần cleanup orphan PTYs:

**File:** `src/main/workspace/WorkspaceService.ts` (MODIFY)

```typescript
// Scheduled cleanup: mỗi 5 phút, check sessions không có last_seen trong 30 phút
setInterval(async () => {
  const staleSessions = await db.all(`
    SELECT * FROM orca_terminal_sessions
    WHERE last_seen_at < ?
  `, [Date.now() - 30 * 60 * 1000])

  for (const session of staleSessions) {
    try {
      await relay.call('pty.kill', { ptyId: session.pty_id })
    } catch {} // PTY might already be dead
    await db.run('DELETE FROM orca_terminal_sessions WHERE session_id = ?', [session.session_id])
  }
}, 5 * 60 * 1000)
```

---

## Files cần tạo/sửa

| File | Action | Phần |
|------|--------|------|
| `src/renderer/src/components/terminal-pane/remote-runtime-pty-transport.ts` | MODIFY | Part B |
| `src/main/workspace/WorkspaceService.ts` | MODIFY | Part A, C, E |
| DB migration file | CREATE | Part D |

---

## Flow sau khi fix

```
Browser
  │ callRuntime('terminal.create', {...})
  │
Orca Server
  │ relay.call('pty.create') → Dev Server
  │ INSERT orca_terminal_sessions { sessionId, ptyId, userId }
  │ return { ptyId, sessionId }
  │
Browser
  │ sessionStorage.setItem(`orca_terminal_session_${ptyId}`, sessionId)
  │
  [Browser reload]
  │
Browser
  │ Read sessionStorage → find stored sessionId
  │ callRuntime('terminal.attach', { sessionId })
  │
Orca Server
  │ SELECT orca_terminal_sessions WHERE session_id = ?
  │ relay.call('pty.attach', { ptyId }) → reattach to live PTY
  │ return { handle } → Browser reconnects
```

---

## Liên quan

- **BL-TM-01**: `terminal.create` → INSERT session ✅ fixed
- **BL-TM-03**: Scrollback Persistence — unblocked (sessionId available)
- **TDD-FE-04**: §3.4 PtyConnection — Session snapshot, reattach
- **BUG-FE-TM-001**: Timeout issue (độc lập, fix riêng)
- **BUG-FE-TM-002**: Scrollback snapshot save/restore (phụ thuộc vào BUG-TM-003 fix này)
