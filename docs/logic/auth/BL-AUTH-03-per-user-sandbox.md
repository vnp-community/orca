# BL-AUTH-03: Per-User Process Sandbox

**Domain:** Authentication & User Management  
**Priority:** P0  
**Actor chính:** Mọi user (Web Server Mode)  
**Tham chiếu:** FR-12, UR-111, F24

---

## Mô tả

Mỗi user được cấp phát một Node.js child process riêng biệt thông qua `fork()`. Data path và Unix socket hoàn toàn cô lập. Một user process crash không ảnh hưởng user khác.

## Process Architecture

```
Orca Web Server (main)
     │
     ├── SessionManager
     │    ├── userId-A → ChildProcess(A) ← Unix socket A
     │    ├── userId-B → ChildProcess(B) ← Unix socket B
     │    └── userId-C → ChildProcess(C) ← Unix socket C
     │
     └── WsSessionRouter
          ├── WS conn (userA) ↔ proxy ↔ ChildProcess(A)
          └── WS conn (userB) ↔ proxy ↔ ChildProcess(B)
```

## Per-User Data Layout

```
~/.orca/users/<userId>/
├── orca.sock          # Unix domain socket
├── orca.db            # Per-user SQLite (worktrees, sessions, settings)
├── credentials.enc    # AES-256-GCM encrypted API tokens
└── worktrees/         # Git worktrees for this user
    ├── wt-abc/
    └── wt-def/
```

## Lifecycle Events

| Event | Trigger | Action |
|-------|---------|--------|
| User logs in (first WS) | WsSessionRouter.route() | fork() new child process |
| User idle 4h | SessionManager timer | graceful kill + cleanup |
| Child crashes | process 'exit' event | respawn (max 3 attempts) |
| Admin kills session | DELETE /admin/api/sessions/:id | SIGTERM child process |
| User logs out | POST /auth/logout | session delete + idle child |

## Isolation Guarantees

- SSH connection store: isolated per userId in child process
- File system: child process chroot-like scoping via `userDataPath`
- Memory: separate heap (fork creates separate V8 heap)
- Database: separate SQLite file (or separate DB connection in server DB)

## Source References

- `src/main/session/session-manager.ts`
- `src/main/session/ws-session-router.ts`
- `src/main/server/node-adapter.ts` — bootstrapWebApp()
