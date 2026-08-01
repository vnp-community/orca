# BUG-BE-CLI-001: `orca daemon start` / Unix Socket listener chưa được implement đúng trong `src/main/cli`

**Status:** ✅ FIXED — 2026-08-01  
**Task:** TASK-CLI-001  
**Note:** cli/PtyDaemon.ts: Unix socket JSON-RPC daemon with 0600 perms  

## Mức độ: 🟡 MEDIUM

## Tóm tắt

HLD (BL-CLI-03) mô tả:
```
$ orca daemon start
[CLI] fork Daemon process to background:
    child_process.spawn('orca-daemon', [], { detached: true, stdio: 'ignore' })
    → PID file: ~/.orca/daemon.pid

[Daemon Process]
    ├─ Create Unix Socket: ~/.orca/orca.sock
    ├─ Open SQLite: ~/.orca/orca.db
    ├─ Initialize AutomationEngine + Scheduler
    ├─ Initialize AgentManager
    ├─ Start HTTP server (optional): http://localhost:6770
```

Grep `src/main/cli` không tìm thấy:
```
daemon.pid          → No results
orca.sock Unix Socket (server) → No results  
orca-daemon binary  → No results
```

## File liên quan

- `src/main/cli/` — cần kiểm tra actual implementation

## Ảnh hưởng

1. **BL-CLI-03** headless daemon mode không hoạt động.
2. `$ orca daemon start` → no PID file → cannot check if daemon is running.
3. Unix Socket listener không có → `$ orca worktree create` qua CLI không connect được đến daemon.
4. `$ orca agent stop --worktree <id>` → không có handler.

## Lưu ý

HLD mô tả `$ orca agent stop` gửi `SIGINT` đến PTY (BL-CLI-02). Nhưng SIGINT chỉ gửi Ctrl+C character `\x03` vào PTY stdin — không phải kill signal. Đây cũng là một sai khác nhỏ với implementation thực tế nếu code dùng `process.kill(pid, 'SIGINT')` thay vì write `\x03` vào PTY.

## Liên quan đến luồng

- **BL-CLI-01**: CLI worktree create — daemon socket dependency.
- **BL-CLI-02**: CLI agent management — daemon socket dependency.
- **BL-CLI-03**: Daemon mode — start/stop/status not implemented.
