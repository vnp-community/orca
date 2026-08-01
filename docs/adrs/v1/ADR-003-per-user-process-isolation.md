# ADR-003 — Per-User Process Isolation via SessionManager

| Trường | Giá trị |
|--------|---------|
| **ID** | ADR-003 |
| **Trạng thái** | ✅ Accepted |
| **Ngày** | 2026-07-28 |
| **HLD Ref** | C2 (Web Server boundary), C3.1 |
| **Code Ref** | `src/main/session/session-manager.ts`, `src/main/session/user-process-entry.ts`, `src/main/session/ws-session-router.ts` |

---

## Bối cảnh

Trong **ORCA_MULTI_USER=1** mode, nhiều developers đăng nhập cùng lúc vào Orca Web Server. Nếu tất cả chạy trong cùng một Node.js process:

1. **Memory isolation fail**: User A có thể access worktrees/state của User B qua shared singletons
2. **Crash blast radius**: Bug trong session của User A crash toàn bộ server
3. **Resource contention**: CPU-intensive agent của User A block event loop của User B
4. **PTY leakage**: Terminal processes không được isolate đúng per-user

---

## Quyết định

Dùng **Node.js `child_process.fork()`** để tạo một process riêng per user session:

```
Main Server Process (HTTP :6769, WS :6768)
├── SessionManager
│   ├── UserProcess[alice]  → fork() → src/main/session/user-process-entry.ts
│   │   └── OrcaRuntime     → Unix socket: users/alice/orca.sock
│   ├── UserProcess[bob]    → fork()
│   │   └── OrcaRuntime     → Unix socket: users/bob/orca.sock
│   └── ...
└── WsSessionRouter: upgrade → route by cookie → correct unix socket
```

### Session lifecycle

```typescript
// session-manager.ts
class SessionManager {
  // fork() user process on first WebSocket connection
  async ensureUserProcess(userId: string): Promise<UserProcess>

  // Auto-cleanup idle processes (DEFAULT: 4h idle timeout)
  private checkIdleProcesses(): void

  // Respawn on crash (max 3 attempts DEFAULT)
  private handleProcessExit(userId: string, code: number): void
}

// Constants từ code:
const DEFAULT_IDLE_TIMEOUT_MS = 4 * 60 * 60 * 1000  // 4 hours
const DEFAULT_MAX_RESPAWN     = 3
const IDLE_CHECK_INTERVAL_MS  = 5 * 60 * 1000
const SPAWN_TIMEOUT_MS        = 30_000               // 30s để fork ready
```

### Giao tiếp

- **Main ↔ UserProcess**: `process.send()` / `process.on('message')` — JSON messages
- **WebSocket → UserProcess**: `WsSessionRouter` proxy qua Unix socket
- **DevServer events**: Main broadcast qua `process.send({ type: 'devServer:event' })` tới tất cả user processes

### Data isolation

```
<userDataPath>/users/<userId>/
├── orca.sock           ← Unix socket for WS proxy
├── credentials/        ← WebCredentialStore per-user
├── worktrees/          ← per-user worktree state
└── session.json        ← session metadata
```

---

## Lý do chọn

| Lựa chọn | Đánh giá |
|----------|---------|
| **child_process.fork() per user** ✅ | True isolation, OS-level resource accounting, crash isolation |
| Worker threads (worker_threads) | Shared memory space, không isolate crash, không đủ isolation |
| Container per user (Docker) | Quá heavy, startup > 10s, infrastructure phức tạp |
| Single process với namespace | JavaScript không có true namespace, shared singletons vẫn leak |
| VM per user (vm.runInContext) | Không isolate I/O, file system, child processes |

---

## Hậu quả

**Tích cực:**
- User A crash không ảnh hưởng User B
- OS-level resource limits per process
- PTY processes inherit correct user env
- `WebCredentialStore` per userId — true isolation

**Tiêu cực:**
- Fork overhead: ~100–300ms per user (có `SPAWN_TIMEOUT_MS = 30s` buffer)
- Memory: ~50–80MB overhead per user process
- DevServer events phải broadcast qua IPC messages
- Shared state (DevServerManager) phải replicate qua IPC — currently broadcast trong `session-manager.ts`

---

## Cấu hình

```bash
ORCA_MULTI_USER=1                 # Enable multi-user mode
ORCA_SESSION_IDLE_TIMEOUT=14400   # Seconds (default 4h)
ORCA_MAX_RESPAWN=3                # Max crash respawn attempts
```

---

## Trạng thái Implementation

✅ SessionManager — fork, idle cleanup, respawn  
✅ WsSessionRouter — route WebSocket upgrades to unix sockets  
✅ user-process-entry.ts — child bootstrap  
✅ DevServer event broadcast  
✅ WebCredentialStore per userId
