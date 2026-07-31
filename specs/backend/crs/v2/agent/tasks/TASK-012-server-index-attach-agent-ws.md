# TASK-012: Sửa `src/server/index.ts` — attach AgentWebSocketServer

> **Status:** ✅ DONE (2026-07-26)
> **TypeScript:** 0 errors

**Status:** ✅ DONE  
**Phase:** 4 — direct-websocket mode  
**Solution:** [SOL-AG-004](../solutions/SOL-AG-004-direct-websocket.md) §3.3 (cuối)  
**Depends on:** TASK-011  
**Blocks:** (không có — last task)  

---

## Mục tiêu

Sửa `src/server/index.ts` để:
1. Nhận `agentWsServer` từ `initializeOrcaServices()` result
2. Gọi `agentWsServer.attach(httpServer)` sau khi HTTP server được khởi động
3. Log agent WS endpoint URL

---

## File cần sửa

**Path:** `src/server/index.ts`

---

## Phân tích code hiện tại

```typescript
// src/server/index.ts — khoảng line 73-93
const { shutdown, dbMonitor, pushManager, authManager } = await initializeOrcaServices({
  platform: createNodeAdapter(),
  port: rpcPort
})

if (webRoot) {
  const { startHttpServer } = await import('./http-server')
  httpServer = await startHttpServer(httpPort, webRoot, { dbMonitor, authManager })
  // ...
}

// ...
console.log(`[Orca Server] RPC:     ws://0.0.0.0:${rpcPort}`)
```

---

## Thay đổi cần thực hiện

### 1. Thêm `agentWsServer` vào destructure của `initializeOrcaServices()`

**Tìm dòng:**
```typescript
const { shutdown, dbMonitor, pushManager, authManager } = await initializeOrcaServices({
```

**Thay bằng:**
```typescript
const { shutdown, dbMonitor, pushManager, authManager, agentWsServer } = await initializeOrcaServices({
```

### 2. Attach agentWsServer vào httpServer

**Tìm đoạn sau khi `startHttpServer()` được gọi:**
```typescript
  if (webRoot) {
    const { startHttpServer } = await import('./http-server')
    httpServer = await startHttpServer(httpPort, webRoot, { dbMonitor, authManager })
    registerPushApiRoutes(httpServer, pushManager)
    // ...
  }
```

**Thêm attach sau `registerPushApiRoutes`:**
```typescript
  if (webRoot) {
    const { startHttpServer } = await import('./http-server')
    httpServer = await startHttpServer(httpPort, webRoot, { dbMonitor, authManager })
    registerPushApiRoutes(httpServer, pushManager)

    // Attach AgentWebSocketServer to handle ws://<host>:<httpPort>/agent connections
    // Why: HTTP server (httpPort) hosts the web UI and REST API. We attach agent WS
    // here (not on rpcPort) because rpcPort is dedicated to browser RPC clients.
    agentWsServer.attach(httpServer)
  }
```

### 3. Thêm log line cho agent WS endpoint

**Tìm đoạn log:**
```typescript
console.log(`[Orca Server] RPC:     ws://0.0.0.0:${rpcPort}`)
```

**Thêm sau dòng đó:**
```typescript
console.log(`[Orca Server] RPC:     ws://0.0.0.0:${rpcPort}`)
if (webRoot) {
  console.log(`[Orca Server] Agent WS: ws://0.0.0.0:${httpPort}/agent`)
}
```

---

## Lưu ý quan trọng

**Tại sao attach vào `httpPort` (6769) chứ không phải `rpcPort` (6768)?**

- `rpcPort` (6768): WS-only, dedicated cho browser clients qua `OrcaRuntimeRpcServer`
- `httpPort` (6769): HTTP server chạy Express, phục vụ web UI + REST API + agent WS
- Agent WS được attach vào `httpPort` server vì nó share cùng HTTP upgrade infrastructure

**Nếu httpServer không được tạo (webRoot = null):**
Agent WS cũng không được attach — acceptable vì direct-websocket mode yêu cầu web server mode.

---

## Acceptance Criteria

- [x] `agentWsServer` được destructure từ `initializeOrcaServices()` result
- [x] `agentWsServer.attach(httpServer)` được gọi khi `webRoot` có giá trị
- [x] Log: `[Orca Server] Agent WS: ws://0.0.0.0:<httpPort>/agent`
- [x] Khi chạy server: agent có thể connect tới `ws://host:<httpPort>/agent`
- [x] Các connections tới `ws://host:<rpcPort>/` vẫn hoạt động (browser RPC)
- [x] TypeScript compile không lỗi
- [x] Server start log không bị regression

---

## Verification

Sau khi deploy, test agent connection bằng command sau trên agent machine:

```bash
# Test với curl (WebSocket upgrade)
curl -v -N \
  -H "Connection: Upgrade" \
  -H "Upgrade: websocket" \
  -H "Sec-WebSocket-Key: $(openssl rand -base64 16)" \
  -H "Sec-WebSocket-Version: 13" \
  http://b15.openledger.vn:6769/agent

# Expect: HTTP 101 Switching Protocols
```
