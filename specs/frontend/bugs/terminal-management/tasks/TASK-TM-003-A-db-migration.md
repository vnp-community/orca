# TASK-TM-003-A: Tạo bảng `orca_terminal_sessions` và migration (TM-003)

**Domain:** terminal-management  
**Solution Ref:** SOL-TM-003 Phần 1  
**Bug:** BUG-TM-003  
**Priority:** 🔴 P0  
**Estimated:** 20 phút  
**Status:** ✅ DONE — Implemented

---

## Mục tiêu

Tạo DB migration thêm bảng `orca_terminal_sessions` để lưu trữ terminal session state persistent.

---

## Files cần tạo

- **TẠO MỚI:** Migration file trong `src/main/db/migrations/` (đặt tên theo convention hiện có, ví dụ `0011_add_terminal_sessions.sql`)

---

## Các bước thực thi

### Bước 1: Tìm migration convention hiện có

```bash
ls src/main/db/migrations/
# Xem pattern đặt tên
```

### Bước 2: Tạo migration file

```sql
-- Migration: Add orca_terminal_sessions table (TM-003)
CREATE TABLE IF NOT EXISTS orca_terminal_sessions (
  id           TEXT PRIMARY KEY,           -- UUID
  worktree_id  TEXT NOT NULL,
  terminal_id  TEXT NOT NULL,              -- xterm terminal identifier
  snapshot     TEXT,                       -- serialized xterm state (base64)
  cwd          TEXT,                       -- last working directory
  title        TEXT,                       -- terminal tab title
  cols         INTEGER NOT NULL DEFAULT 80,
  rows         INTEGER NOT NULL DEFAULT 24,
  created_at   INTEGER NOT NULL,
  updated_at   INTEGER NOT NULL,
  UNIQUE(worktree_id, terminal_id)
);

CREATE INDEX IF NOT EXISTS idx_terminal_sessions_worktree
  ON orca_terminal_sessions(worktree_id);
```

### Bước 3: Thêm migration vào migration runner

```bash
grep -rn "runMigrations\|migration" src/main/db/ | head -10
# Tìm nơi migrate được gọi và thêm migration mới
```

---

## Verify

```bash
grep -rn "orca_terminal_sessions" src/main/db/
```

## Depends on
Không có

## Blocking
TASK-TM-003-B (TerminalSessionService)
