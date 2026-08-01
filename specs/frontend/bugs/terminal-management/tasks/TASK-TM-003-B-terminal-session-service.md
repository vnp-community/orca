# TASK-TM-003-B: `TerminalSessionService` — save/restore session (TM-003)

**Domain:** terminal-management  
**Solution Ref:** SOL-TM-003 Phần 2  
**Bug:** BUG-TM-003  
**Priority:** 🔴 P0  
**Estimated:** 45 phút  
**Status:** ✅ DONE — Implemented

---

## Mục tiêu

Tạo `TerminalSessionService` (hoặc thêm vào service hiện có) với methods `saveSession()` và `restoreSession()` dùng bảng `orca_terminal_sessions`.

---

## Files cần tạo/sửa

Tìm service layer hiện có:

```bash
find src/main -name "*terminal*" -o -name "*session*" | head -10
```

- **TẠO MỚI hoặc MODIFY:** `src/main/services/terminal-session-service.ts`

---

## Các bước thực thi

```typescript
// src/main/services/terminal-session-service.ts
export class TerminalSessionService {
  constructor(private db: Database) {}

  async saveSession(params: {
    worktreeId: string
    terminalId: string
    snapshot: string      // serialized xterm state
    cwd: string
    title: string
    cols: number
    rows: number
  }): Promise<void> {
    const now = Date.now()
    await this.db.run(`
      INSERT INTO orca_terminal_sessions
        (id, worktree_id, terminal_id, snapshot, cwd, title, cols, rows, created_at, updated_at)
      VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
      ON CONFLICT(worktree_id, terminal_id) DO UPDATE SET
        snapshot = excluded.snapshot,
        cwd = excluded.cwd,
        title = excluded.title,
        cols = excluded.cols,
        rows = excluded.rows,
        updated_at = excluded.updated_at
    `, [
      randomUUID(), params.worktreeId, params.terminalId,
      params.snapshot, params.cwd, params.title,
      params.cols, params.rows, now, now,
    ])
  }

  async restoreSession(worktreeId: string, terminalId: string): Promise<TerminalSession | null> {
    return this.db.get<TerminalSession>(`
      SELECT * FROM orca_terminal_sessions
      WHERE worktree_id = ? AND terminal_id = ?
    `, [worktreeId, terminalId]) ?? null
  }

  async deleteSession(worktreeId: string, terminalId: string): Promise<void> {
    await this.db.run(`
      DELETE FROM orca_terminal_sessions
      WHERE worktree_id = ? AND terminal_id = ?
    `, [worktreeId, terminalId])
  }
}
```

**Lưu ý:** Tìm `Database` type đang dùng trong codebase:
```bash
grep -rn "import.*Database\|new Database\|sqlite" src/main/ | head -10
```

---

## Verify

```bash
grep -n "saveSession\|restoreSession" \
  src/main/services/terminal-session-service.ts
```

## Test

```typescript
// Unit test với in-memory SQLite:
// - saveSession stores row
// - restoreSession returns stored row
// - duplicate save (upsert) updates instead of insert
// - deleteSession removes row
```

## Depends on
TASK-TM-003-A (DB table)

## Blocking
TASK-TM-003-C (IPC handler), TASK-TM-003-D (Renderer integration)
