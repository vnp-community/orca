# TDD-BE-02: Server Entry Point

**Version:** 4.0  
**Date:** 2026-07-28  
**Source:** `src/server/index.ts`

---

## 1. Startup Sequence

```typescript
// src/server/index.ts — thứ tự QUAN TRỌNG

// STEP 1: NodeAdapter + setPlatform (TRƯỚC mọi import từ src/main/)
const adapter = createNodeAdapter(userDataPath ? { userDataPath } : {})
setPlatform(adapter)

// STEP 2: Parse config
const rpcPort  = parseInt(process.env.ORCA_PORT ?? '6768')
const httpPort = parseInt(process.env.ORCA_HTTP_PORT ?? String(rpcPort + 1))
const webRoot  = process.env.ORCA_WEB_ROOT ?? resolve(__dirname, '..', 'web')

// STEP 3: Initialize all Orca backend services
const { initializeOrcaServices } = await import('../main/server-bootstrap')
const { shutdown, dbMonitor, pushManager, authManager, agentWsServer, devServerManager }
  = await initializeOrcaServices({ platform: adapter, port: rpcPort })

// STEP 4: HTTP Server (if web bundle exists)
if (existsSync(webRoot)) {
  const { startHttpServer } = await import('./http-server')
  const { createAgentTokenApiHandler } = await import('./agent-token-routes')
  httpServer = await startHttpServer(httpPort, webRoot, {
    dbMonitor,
    authManager,
    apiHandler: createAgentTokenApiHandler(agentWsServer, devServerManager)
  })

  // STEP 5: Register Web Push routes
  registerPushApiRoutes(httpServer, pushManager)

  // STEP 6: Attach AgentWebSocketServer to httpServer
  agentWsServer.attach(httpServer)
}

// STEP 7: Multi-user mode (optional)
if (process.env.ORCA_MULTI_USER === '1') {
  const sessionManager = new SessionManager({ baseDataPath, userProcessEntry, devServerManager })
  const wsRouter       = new WsSessionRouter({ sessionManager, authManager })
  httpServer.on('upgrade', (req, socket, head) => {
    // Skip /agent path (handled by agentWsServer.attach)
    if (req.url === AGENT_WS_PATH) return
    wss.handleUpgrade(req, socket, head, (ws) => wsRouter.handleConnection(ws, req))
  })
}
```

---

## 2. Ports & Addresses

| Service | Default Port | Path | Protocol |
|---------|-------------|------|---------|
| OrcaRuntimeRpcServer | 6768 | `/` | WebSocket + JSON |
| HTTP Static Files | 6769 | `/*` | HTTP |
| Agent WebSocket | 6769 | `/agent` | WebSocket (binary frames) |
| Agent Token API | 6769 | `/api/agent-token` | HTTP REST |
| Auth API | 6769 | `/auth/*` | HTTP REST |
| Admin API | 6769 | `/admin/api/*` | HTTP REST |
| Health | 6769 | `/health/*` | HTTP REST |
| Web Push | 6769 | `/push/*` | HTTP REST |

---

## 3. DB Logging at Startup

```typescript
// Log DB configuration tại startup (password masked)
const dbUrl = process.env['ORCA_DB_URL']
if (dbUrl) {
  const config = parseDsn(dbUrl)
  console.log(`[Orca Server] Database: ${formatDsn(config)}`) // password masked
} else {
  console.log('[Orca Server] Database: SQLite (default)')
}
```

---

## 4. Graceful Shutdown

```typescript
const handleShutdown = async (signal: string) => {
  httpServer?.close()
  if (sessionManagerShutdown) await sessionManagerShutdown()
  await shutdown()  // từ ServerBootstrapResult
  process.exit(0)
}

process.on('SIGINT',  () => handleShutdown('SIGINT'))
process.on('SIGTERM', () => handleShutdown('SIGTERM'))
```

**Shutdown order:**
1. HTTP server close (stop accepting new connections)
2. SessionManager shutdown (SIGTERM to all user processes)
3. `ServerBootstrapResult.shutdown()`:
   - AgentWebSocketServer.stop()
   - DevServerManager disconnect all
   - DB connection pool close
   - HealthChecker stop

---

## 5. API-Only Mode (no web bundle)

Nếu `webRoot` không tồn tại:
- Warning logged: "Web bundle not found at: ..."
- Server tiếp tục chạy trong API-only mode
- OrcaRuntimeRpcServer (:6768) vẫn hoạt động
- HTTP server KHÔNG khởi động
- AgentWebSocketServer không được attach
