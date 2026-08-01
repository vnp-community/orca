# CR-LOGIN-002 — Per-User Sandbox: Isolated Runtime Process

| Field | Value |
|-------|-------|
| **CR ID** | CR-LOGIN-002 |
| **Tên** | Per-User Sandbox: Isolated Runtime Process |
| **Ưu tiên** | P0 |
| **Effort** | XL (3–4 sprints) |
| **Blocked by** | CR-LOGIN-001 (cần userId từ auth) |
| **Blocks** | CR-LOGIN-003 |
| **Status** | ✅ Implemented (2026-07-24) — 8/8 AC done |

---

## 1. Vấn đề hiện tại

Orca Server chạy **1 runtime process** cho tất cả users:

```typescript
// src/main/runtime/runtime-rpc.ts — hiện tại
class OrcaRuntimeRpc {
  private readonly userDataPath: string  // ← shared cho mọi user
  private sshConnections: Map<string, SshConnection>   // shared
  private deviceRegistry: DeviceRegistry               // shared
  // Tất cả WebSocket connections đều gọi vào 1 instance này
}
```

**Hậu quả:**
- User A crash agent → ảnh hưởng user B
- User A có thể đọc terminal output của user B (qua connectionId collision)
- SSH target list của user A visible với user B
- `userDataPath` shared → DB, file, keys đều chung

---

## 2. Giải pháp: Process-per-User (Session Manager Pattern)

### 2.1 Kiến trúc mới

```
Orca Server Process (supervisor / orchestrator)
  │
  │── HTTP :6769 (auth, health, static web)
  │── WS   :6768 (session router)
  │
  │  Sau khi auth:
  │
  ├── fork() → UserProcess [userId=alice]
  │     │ Unix socket: /data/orca/users/alice/orca.sock
  │     │ userDataPath: /data/orca/users/alice/
  │     │ SshConnections: alice's targets only
  │     │ DeviceRegistry: alice's devices only
  │     └── WS proxy: :6768/ws?session=alice → sock
  │
  └── fork() → UserProcess [userId=bob]
        │ Unix socket: /data/orca/users/bob/orca.sock
        │ userDataPath: /data/orca/users/bob/
        │ SshConnections: bob's targets only
        └── WS proxy: :6768/ws?session=bob → sock
```

### 2.2 Session Manager

```typescript
// src/main/session/session-manager.ts [NEW]

type UserProcess = {
  userId:     string
  pid:        number
  socketPath: string
  startedAt:  number
  lastSeenAt: number
  process:    ChildProcess
}

class SessionManager {
  private processes = new Map<string, UserProcess>()

  async getOrSpawnUserProcess(userId: string): Promise<UserProcess> {
    if (this.processes.has(userId)) {
      return this.processes.get(userId)!
    }
    return this.spawnUserProcess(userId)
  }

  private async spawnUserProcess(userId: string): Promise<UserProcess> {
    const userDataPath = path.join(BASE_DATA_PATH, 'users', userId)
    await fs.mkdir(userDataPath, { recursive: true, mode: 0o700 })

    const socketPath = path.join(userDataPath, 'orca.sock')
    const child = fork(USER_PROCESS_ENTRY, [], {
      env: {
        ...process.env,
        ORCA_USER_DATA_PATH: userDataPath,
        ORCA_USER_ID: userId,
        ORCA_SOCKET_PATH: socketPath,
      },
      stdio: ['ignore', 'pipe', 'pipe', 'ipc']
    })

    // Chờ process sẵn sàng (IPC message: { type: 'ready' })
    await waitForReady(child)

    const proc: UserProcess = {
      userId, pid: child.pid!, socketPath,
      startedAt: Date.now(), lastSeenAt: Date.now(), process: child
    }
    this.processes.set(userId, proc)
    return proc
  }

  // Graceful shutdown sau idle timeout (default: 4h)
  private scheduleIdleShutdown(userId: string) { ... }
}
```

### 2.3 WS Session Router (supervisor process)

```typescript
// src/main/session/ws-session-router.ts [NEW]
// Chạy trong supervisor process — proxy WS đến đúng user socket

wss.on('connection', async (ws, req) => {
  // 1. Xác thực session
  const session = await validateSession(req)
  if (!session) { ws.close(4401, 'Unauthorized'); return }

  // 2. Lấy/tạo user process
  const userProc = await sessionManager.getOrSpawnUserProcess(session.userId)

  // 3. Proxy WS → Unix socket của user process
  const upstream = net.createConnection(userProc.socketPath)
  pipeWebSocket(ws, upstream)

  // 4. Track last seen (idle timeout)
  sessionManager.touch(session.userId)
})
```

### 2.4 User Process Entry Point

```typescript
// src/main/session/user-process-entry.ts [NEW]
// File này được fork() bởi SessionManager
// Chạy 1 instance OrcaRuntimeRpc cho 1 user

const userId     = process.env.ORCA_USER_ID!
const dataPath   = process.env.ORCA_USER_DATA_PATH!
const socketPath = process.env.ORCA_SOCKET_PATH!

// Boot runtime (giống hiện tại nhưng isolated)
const runtime = await bootstrapUserRuntime({ userId, dataPath })
const rpc     = new OrcaRuntimeRpc({ userDataPath: dataPath, userId })

// Listen trên Unix socket thay vì TCP port
rpc.listenOnSocket(socketPath)

// Signal supervisor: ready
process.send!({ type: 'ready', socketPath })
```

---

## 3. Data Isolation

### 3.1 File System

```
/data/orca/
├── orca-server.db         ← shared (users, sessions, admin)
├── users/
│   ├── alice-uuid/
│   │   ├── orca-data.json           # alice's settings
│   │   ├── orca-devices.json        # alice's paired devices
│   │   ├── orca-e2ee-keypair.json   # alice's E2EE key
│   │   ├── orca-server.db           # alice's SQLite
│   │   ├── orca.sock                # alice's Unix socket
│   │   └── daemon/                  # alice's PTY daemon
│   └── bob-uuid/
│       └── ...
└── shared/
    └── ssh-keys/           # shared fleet SSH key
```

### 3.2 SSH Connection Isolation

Mỗi user process có `SshConnectionStore` riêng:
- Alice chỉ thấy SSH targets của alice
- Bob không thể đọc alice's private keys
- SSH relay session (relay process trên dev server) owned bởi user cụ thể

---

## 4. Process Lifecycle

```
User login                  Supervisor
    │                           │
    │── auth success ──────────►│ getOrSpawnUserProcess(userId)
    │                           │── check existing process?
    │                           │   YES → reuse (touch lastSeenAt)
    │                           │   NO  → fork() new process
    │                           │        wait for IPC 'ready'
    │                           │
    │◄── WS proxied ────────────│ proxy ws → user.sock
    │
    │  [user idle > 4h]
    │                           │── idleShutdown: kill user process
    │                           │   cleanup: rm orca.sock
    │
    │  [user reconnects]
    │                           │── respawn process
    │                           │   restore state từ SQLite
```

**Timeouts:**

| Event | Timeout | Action |
|-------|---------|--------|
| No WS connections | 4 hours | graceful shutdown user process |
| Process crash | immediate | respawn (max 3 retries) |
| No login | 15 minutes | cleanup spawned process |

---

## 5. Files cần tạo/sửa

### Tạo mới

```
src/main/session/
├── session-manager.ts          # Process spawner + lifecycle
├── ws-session-router.ts        # WS proxy (supervisor side)
├── user-process-entry.ts       # Fork entry point
└── session-types.ts            # UserProcess, SessionConfig types
```

### Sửa

| File | Thay đổi |
|------|---------|
| `src/main/index.ts` | Thay đổi startup: supervisor mode vs user-process mode |
| `src/main/runtime/runtime-rpc.ts` | Thêm `userId` context, listen trên Unix socket |
| `src/main/server-bootstrap.ts` | Accept `userDataPath` per-user |
| `deploy/dev/docker-compose.orca.yml` | Tăng ulimit cho nhiều processes |

---

## 6. Resource Limits

Mỗi user process bị giới hạn qua Linux cgroups hoặc `node:child_process` options:

```typescript
fork(USER_PROCESS_ENTRY, [], {
  // Memory limit — prevent OOM killing all users
  // Implement via cgroups v2 hoặc systemd slice (nếu trong container)
  env: {
    ...process.env,
    NODE_OPTIONS: '--max-old-space-size=512'  // 512MB per user process
  }
})
```

---

## 7. Acceptance Criteria

- [x] Mỗi user login tạo 1 isolated Node.js process với `fork()` ✅ `session-manager.ts` — `fork()` per userId
- [x] User process có `userDataPath` riêng biệt `/data/orca/users/{userId}/` ✅ `session-manager.ts` L53 — `join(baseDataPath, 'users', userId)`
- [x] WS connection của user A không thể đọc data của user B ✅ `ws-session-router.ts` — proxy tới Unix socket riêng per userId
- [x] User process tự shutdown sau 4h idle (configurable) ✅ `session-manager.ts` — `DEFAULT_IDLE_TIMEOUT_MS = 4h`
- [x] Process crash tự respawn (max 3 attempts) ✅ `session-manager.ts` — `respawnCount`, `maxRespawnAttempts`
- [x] SSH connection store isolated per user ✅ `ssh-connection-store.ts` — `resolveUserSshTarget(base, session.userId)`
- [x] Supervisor process vẫn healthy khi user process crash ✅ `session-manager.ts` — auto-cleanup on exit, supervisor không crash
- [x] `GET /health/ready` reflect status của supervisor (không phải user process) ✅ `health-endpoint.ts` — checks DB only

---

## 8. Implementation Status

> **✅ IMPLEMENTED — 2026-07-24**  
> 8/8 AC done

| Layer | Files | Status |
|-------|-------|--------|
| SessionManager | `src/main/session/session-manager.ts` | ✅ Done |
| WsSessionRouter | `src/main/session/ws-session-router.ts` | ✅ Done |
| UserProcessEntry | `src/main/session/user-process-entry.ts` | ✅ Done |
| SessionTypes | `src/main/session/session-types.ts` | ✅ Done |

**Tests:** 21 pass | **Env:** `ORCA_MULTI_USER=1` enables per-user isolation
