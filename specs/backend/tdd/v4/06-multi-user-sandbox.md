# TDD-BE-06: Multi-User Sandbox

**Version:** 4.0  
**Date:** 2026-07-28  
**Source:** `src/main/session/`

---

## 1. Kích hoạt

```
ORCA_MULTI_USER=1 node out/server/index.js
```

Nếu không set: single-user mode, mọi WebSocket kết nối đến `OrcaRuntimeRpcServer` trực tiếp.

---

## 2. Module Map

| File | Role |
|------|------|
| `session-types.ts` | `UserProcess`, `SessionManagerConfig` |
| `session-manager.ts` | Fork per user, track processes, idle GC |
| `ws-session-router.ts` | Proxy WebSocket → per-user Unix socket |
| `user-process-entry.ts` | Fork entry: reads env → starts OrcaRuntimeRpcServer |

---

## 3. Isolation Model

```
Supervisor (server/index.ts)
  ├── HTTP :6769 (shared)
  │   └── WsSessionRouter intercepts WS upgrade
  │       ├── Validate session cookie (AuthManager)
  │       └── Proxy → user-A.sock / user-B.sock
  │
  └── SessionManager
      ├── User A: fork(userProcessEntry, { ORCA_USER_ID: 'uuid-a', ORCA_SOCKET_PATH: '.../users/uuid-a/orca.sock' })
      │   └── OrcaRuntimeRpcServer (Unix socket only)
      └── User B: fork(userProcessEntry, { ORCA_USER_ID: 'uuid-b', ORCA_SOCKET_PATH: '.../users/uuid-b/orca.sock' })
          └── OrcaRuntimeRpcServer (Unix socket only)
```

---

## 4. SessionManager

```typescript
class SessionManager {
  // Trả về process đã tồn tại hoặc fork mới
  async getOrSpawnUserProcess(userId: string): Promise<UserProcess>

  // Spawn mới: mkdir 700, load credentials, fork()
  private async spawnUserProcess(userId: string): Promise<UserProcess>

  // Idle GC: check mỗi 5 phút, kill processes idle > 4h
  private sweepIdleProcesses(): void

  // Broadcast DevServer events từ supervisor đến tất cả user processes
  // (DevServer state owned by supervisor — không per-user)
  private broadcastEvent(event, ...args): void

  // Shutdown: SIGTERM all processes, wait max 5s, then SIGKILL
  async shutdown(): Promise<void>
}
```

**Spawn parameters:**
```typescript
const child = fork(userProcessEntry, [], {
  env: {
    ...process.env,
    ORCA_USER_ID:     userId,
    ORCA_SOCKET_PATH: socketPath,
    // Integration credentials injected here (Bitbucket, Azure DevOps, Gitea)
    ...loadIntegrationCredentials(userId)
  },
  execArgv: ['--max-old-space-size=512']
})
```

---

## 5. WsSessionRouter

```typescript
class WsSessionRouter {
  async handleConnection(ws: WebSocket, req: IncomingMessage): Promise<void> {
    // 1. Validate session cookie
    const session = await authManager.validateRequest(req)
    if (!session) { ws.close(1008, 'Unauthorized'); return }

    // 2. Get or spawn user process
    const proc = await sessionManager.getOrSpawnUserProcess(session.userId)

    // 3. Create proxy WebSocket to user's Unix socket
    const proxyWs = new WebSocket('ws+unix://' + proc.socketPath)

    // 4. Bridge bidirectional: ws ↔ proxyWs
    ws.on('message', (data) => proxyWs.send(data))
    proxyWs.on('message', (data) => ws.send(data))
    ws.on('close', () => proxyWs.close())
    proxyWs.on('close', () => ws.close())
  }
}
```

---

## 6. user-process-entry.ts

```typescript
// Entry point cho child process (fork target)
const userId    = process.env['ORCA_USER_ID']!
const socketPath = process.env['ORCA_SOCKET_PATH']!

// Initialize platform (NodeAdapter với per-user userData path)
const adapter = createNodeAdapter({ userDataPath: join(baseDataPath, 'users', userId) })
setPlatform(adapter)

// Boot OrcaRuntimeRpcServer trên Unix socket
const { initializeOrcaServices } = await import('../server-bootstrap')
await initializeOrcaServices({ platform: adapter, port: 0, isUserProcess: true })
// Port 0 → Unix socket mode (ignores TCP port)

// Listen for DevServer broadcast from supervisor
process.on('message', (msg) => {
  if (msg.type === 'devServer:event') {
    // Relay to RPC clients of this user process
    rpcServer.broadcast(msg.event, ...msg.args)
  }
})
```

---

## 7. `UserProcess` Type

```typescript
export type UserProcess = {
  userId:        string
  process:       ChildProcess
  socketPath:    string
  spawnedAt:     number
  lastSeenAt:    number
  respawnCount:  number
  isReady:       boolean   // true khi process send 'ready' IPC message
}
```

---

## 8. Timers và Limits

| Parameter | Value | Lý do |
|-----------|-------|-------|
| Idle timeout | 4 giờ | Giải phóng RAM |
| Idle check interval | 5 phút | Low overhead |
| Spawn timeout | 30 giây | Nếu process không ready → cleanup |
| Max respawn | 3 lần | Tránh crash loop |
| Max old space | 512MB | Giới hạn RAM per user |

---

## 9. Tests (21 tests)

| File | Tests |
|------|-------|
| `session-manager.test.ts` | 14 |
| `ws-session-router.test.ts` | 7 |
