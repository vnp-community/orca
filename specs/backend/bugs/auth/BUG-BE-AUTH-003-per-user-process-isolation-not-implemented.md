# BUG-BE-AUTH-003: `SessionManager` fork/per-user child process model không được implement — tất cả users dùng chung process

**Status:** ✅ FIXED — 2026-08-01  
**Task:** Verified — Already Implemented  
**Note:** session/session-manager.ts fully implements per-user fork model with Unix socket at users/<userId>/orca.sock. Bug was based on outdated analysis.  

## Mức độ: 🔴 HIGH (Architecture Gap)

## Tóm tắt

HLD (BL-AUTH-02, BL-AUTH-03) mô tả session isolation model:
```
[WsSessionRouter.route()]
    ├─ Lookup SessionManager: childProcess[userId]?
    │   IF not found: fork new child process
    │       child = fork('orca-user-worker', [], {
    │           env: { USER_ID: userId, DATA_PATH: ~/.orca/users/<userId>/ }
    │       })
    │   Open Unix Socket: ~/.orca/users/<userId>/orca.sock
    └─ Proxy WebSocket ↔ Child Process (Unix Socket)
    
Per-User Data:
~/.orca/users/<userId>/
    orca.sock      ← Unix socket
    orca.db        ← Per-user SQLite
    credentials.enc ← AES-256-GCM tokens
    worktrees/     ← Git worktrees
```

Nhưng `src/main/session/` và `src/main/auth/` không có `SessionManager` hay child process forking:

```typescript
// auth-session-store.ts (thực tế):
// Stores session tokens trong shared DB
// Không fork child process per user
// Không có Unix Socket per user
// Không có per-user SQLite
```

## File liên quan

- [`src/main/auth/auth-manager.ts`](file:///Users/binhnt/Work/blockchain/vnp-blc/orca/src/main/auth/auth-manager.ts) — Không có SessionManager, child process logic
- [`src/main/session/`](file:///Users/binhnt/Work/blockchain/vnp-blc/orca/src/main/session) — cần kiểm tra xem có implement chưa

## Ảnh hưởng

1. **Isolation Breach**: Tất cả users có thể share cùng runtime state nếu không có per-user isolation.
2. Một user crash → potentially ảnh hưởng đến tất cả users khác.
3. Per-user credential file (`credentials.enc`) không được scoped theo user.
4. Worktree paths không được scoped theo `~/.orca/users/<userId>/worktrees/`.

## Liên quan đến luồng

- **BL-AUTH-02**: Session Management & Isolation — per-user forking missing.
- **BL-AUTH-03**: Per-User Process Sandbox — sandbox model not implemented.
