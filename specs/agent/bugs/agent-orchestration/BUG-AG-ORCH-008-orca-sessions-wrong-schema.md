# BUG-AG-ORCH-008: `orca_sessions` bảng dùng cho HTTP auth sessions, không phải agent sessions

## Mức độ: 🔴 HIGH

## Tóm tắt

HLD (BL-AG-01, BL-AG-02, BL-AG-03) mô tả:
```sql
INSERT orca_sessions { id, worktreeId, agentType, devServerId, startedAt }   -- BL-AG-01
UPDATE orca_sessions SET status='stopped'                                      -- BL-AG-02
SELECT sessionId, devServerId FROM orca_sessions WHERE worktreeId=?            -- BL-AG-03
```

Thực tế trong `src/main/db/migrations/0005_add_auth_schema.ts`:
```sql
CREATE TABLE IF NOT EXISTS orca_sessions (
    session_id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    last_seen_at INTEGER NOT NULL,
    expires_at INTEGER NOT NULL          ← HTTP auth session expiry
)
```

**`orca_sessions` là bảng HTTP authentication sessions (cookie sessions), không phải agent sessions.**  
Không có fields: `worktreeId`, `agentType`, `devServerId`, `ptyId`, `status`.

## Ảnh hưởng

1. **BL-AG-01**: Không thể INSERT agent session sau khi spawn
2. **BL-AG-02**: Không thể UPDATE status → agent stop không được persist
3. **BL-AG-03**: Không thể SELECT `sessionId, devServerId` WHERE `worktreeId` → resume broken
4. Không có cách nào recover agent sessions sau khi Orca server restart

## Schema cần thêm

```sql
-- Cần tạo bảng riêng cho agent sessions
CREATE TABLE IF NOT EXISTS orca_agent_sessions (
    id TEXT PRIMARY KEY,           -- agent session ID
    worktree_id TEXT NOT NULL,
    agent_type TEXT NOT NULL,      -- 'claude' | 'codex' | 'gemini' | 'opencode'
    dev_server_id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    pty_id TEXT,
    status TEXT DEFAULT 'idle',    -- 'idle'|'running'|'stopped'|'error'
    started_at INTEGER NOT NULL,
    stopped_at INTEGER,
    FOREIGN KEY(worktree_id) REFERENCES orca_worktrees(id)
)
```

## Migration cần tạo

File mới: `src/main/db/migrations/XXXX_add_agent_sessions.ts`

## Liên quan đến luồng

- **BL-AG-01**: INSERT agent session — no table
- **BL-AG-02**: UPDATE session status — no table
- **BL-AG-03**: SELECT for resume — wrong table schema

---

## ⏸ Fix Status: DEFERRED

**Reason:** Schema change requires Orca Server DB migration. Deferred — relay does not own the orca_agent_sessions table.
