# TASK-011: Sửa `server-bootstrap.ts` — init AgentWebSocketServer

> **Status:** ✅ DONE (2026-07-26)
> **TypeScript:** 0 errors

**Status:** ✅ DONE  
**Phase:** 4 — direct-websocket mode  
**Solution:** [SOL-AG-004](../solutions/SOL-AG-004-direct-websocket.md) §3.3  
**Depends on:** TASK-008, TASK-010  
**Blocks:** TASK-012  

---

## Mục tiêu

Sửa `src/main/server-bootstrap.ts` để:
1. Init `AgentWebSocketServer` sau DevServerManager
2. Pass nó vào `DevServerManager` constructor
3. Export `agentWsServer` trong `ServerBootstrapResult`
4. Gọi `agentWsServer.stop()` trong `shutdown()`

---

## File cần sửa

**Path:** `src/main/server-bootstrap.ts`

---

## Thay đổi cần thực hiện

### 1. Thêm import `AgentWebSocketServer`

Thêm vào phần imports ở đầu file (sau các imports hiện có):

```typescript
import { AgentWebSocketServer } from './dev-server/agent-ws-server'
```

### 2. Thêm `agentWsServer` vào `ServerBootstrapResult` interface

```typescript
// Tìm interface ServerBootstrapResult (khoảng line 26-43):
export interface ServerBootstrapResult {
  // ... existing fields ...
  /** AgentWebSocketServer for direct-websocket mode — attach to HTTP server in server/index.ts */
  agentWsServer: AgentWebSocketServer     // ← THÊM DÒNG NÀY
  shutdown(): Promise<void>
}
```

### 3. Sửa DevServerManager initialization — thêm agentWsServer (khoảng line 93-102)

**Tìm đoạn:**
```typescript
  // 2a. Initialize DevServerManager
  const { SshConnectionManager } = await import('./ssh/ssh-connection-manager')
  const sshManager = new SshConnectionManager({
    onStateChanged: () => {/* no-op in server bootstrap mode */}
  } as never)
  const devServerManager = new DevServerManager(store, sshManager)
```

**Thay bằng:**
```typescript
  // 2a. Initialize DevServerManager + AgentWebSocketServer
  const { SshConnectionManager } = await import('./ssh/ssh-connection-manager')
  const sshManager = new SshConnectionManager({
    onStateChanged: () => {/* no-op in server bootstrap mode */}
  } as never)

  // Why: AgentWebSocketServer must be created BEFORE DevServerManager so it can
  // be injected into DevServerManager → DevServerRelayBridge for direct-websocket mode.
  const agentWsServer = new AgentWebSocketServer(platform.app.getVersion())

  const devServerManager = new DevServerManager(store, sshManager, agentWsServer)
```

### 4. Sửa return statement — thêm agentWsServer

**Tìm return (khoảng line 314):**
```typescript
  return {
    devServerManager,
    dbMonitor,
    pushManager,
    authManager: authManager!,
    sessionManager,
    async shutdown() {
```

**Thay bằng:**
```typescript
  return {
    devServerManager,
    dbMonitor,
    pushManager,
    authManager: authManager!,
    sessionManager,
    agentWsServer,           // ← THÊM
    async shutdown() {
```

### 5. Thêm `agentWsServer.stop()` vào shutdown() (đầu shutdown, trước authManager)

```typescript
    async shutdown() {
      console.log('[ServerBootstrap] Shutting down...')

      // NEW: stop agent WS server first (clear pending slots, close connections)
      try {
        agentWsServer.stop()
        console.log('[ServerBootstrap] ✅ AgentWebSocketServer stopped')
      } catch (err) {
        console.warn('[ServerBootstrap] AgentWebSocketServer stop error:', err)
      }

      // ... existing shutdown logic (authManager, rpcServer, etc.) ...
```

---

## Acceptance Criteria

- [x] `ServerBootstrapResult.agentWsServer: AgentWebSocketServer` field tồn tại
- [x] `AgentWebSocketServer` được init với `platform.app.getVersion()`
- [x] `DevServerManager` constructor được gọi với `agentWsServer` làm arg thứ 3
- [x] `agentWsServer` được return trong `initializeOrcaServices()` result
- [x] `agentWsServer.stop()` được gọi trong `shutdown()`
- [x] Log line: `[ServerBootstrap] ✅ AgentWebSocketServer stopped`
- [x] TypeScript compile không lỗi
- [x] `initializeOrcaServices()` vẫn hoạt động (không regression)
