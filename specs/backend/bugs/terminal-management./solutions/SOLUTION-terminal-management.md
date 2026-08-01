# SOLUTION: Terminal Management Domain — Fix tất cả Bugs

**Domain:** terminal-management (BE) + terminal-management.  
**TDD Reference:** TDD-04 (RPC Server), TDD-11 (Web Server Mode), TDD-05 (SSH Relay)  
**Files cần thay đổi:** `src/main/runtime/orca-runtime.ts`, `src/main/session/ws-session-router.ts`, `src/main/ipc/handlers.ts`  
**Tổng số bugs:** 9 (TM-002, BE-TM-001~006, TRM-BE-001~002)

---

## BUG-TM-002 — Fix workspace init dùng relative path

**Mức độ:** 🟡 MEDIUM  
**Root cause:** `OrcaRuntime.initWorkspace()` sử dụng relative paths → fails khi CWD thay đổi.

### Fix — Convert all paths to absolute

```typescript
// src/main/runtime/orca-runtime.ts

import { resolve, isAbsolute } from 'node:path'

export class OrcaRuntime {
  async initWorkspace(config: WorkspaceConfig): Promise<void> {
    // FIX TM-002: Convert relative → absolute paths
    const absoluteRoot = isAbsolute(config.rootPath)
      ? config.rootPath
      : resolve(process.cwd(), config.rootPath)

    const absoluteDataPath = isAbsolute(config.dataPath ?? '')
      ? config.dataPath!
      : resolve(absoluteRoot, config.dataPath ?? '.orca')

    // Validate absolute paths exist
    if (!existsSync(absoluteRoot)) {
      throw new Error(`Workspace root does not exist: ${absoluteRoot}`)
    }

    await this.setupWorkspaceDirectories(absoluteRoot, absoluteDataPath)
    this.workspaceRoot     = absoluteRoot
    this.workspaceDataPath = absoluteDataPath

    this.log.info(`[Runtime] Workspace initialized: ${absoluteRoot}`)
  }
}
```

---

## BUG-BE-TM-001 — Fix WsSessionRouter không forward binary frames

**Mức độ:** 🔴 HIGH  
**Root cause:** `WsSessionRouter` dùng text mode → binary PTY data (non-UTF8 escape sequences) bị corrupted.

### Fix — Forward binary frames as-is

```typescript
// src/main/session/ws-session-router.ts

private pipeWebSocketToSocket(ws: WebSocket, socket: net.Socket): void {
  ws.on('message', (data, isBinary) => {
    if (isBinary) {
      // FIX BE-TM-001: Binary frames → forward as Buffer (no encoding conversion)
      socket.write(data as Buffer)
    } else {
      // Text frames → JSON-RPC messages
      const text = data.toString('utf-8')
      socket.write(text + '\n')
    }
  })

  socket.on('data', (chunk: Buffer) => {
    // Check if this is wire protocol binary frame or JSON text
    const firstByte = chunk[0]
    if (firstByte === 0x01 || firstByte === 0x02 || firstByte === 0x09) {
      // Binary wire protocol frame → send as binary WS frame
      ws.send(chunk, { binary: true })
    } else {
      // JSON text response
      ws.send(chunk.toString('utf-8'))
    }
  })
}
```

---

## BUG-BE-TM-002 — Fix WsSessionRouter keepalive corrupts Unix socket

**Mức độ:** 🔴 HIGH  
**Root cause:** WebSocket keepalive pong frames được forward vào Unix socket → protocol corruption.

### Fix — Filter keepalive frames, không forward vào Unix socket

```typescript
// src/main/session/ws-session-router.ts

private pipeWebSocketToSocket(ws: WebSocket, socket: net.Socket): void {
  // FIX BE-TM-002: Handle ping/pong without forwarding to Unix socket
  ws.on('ping', () => ws.pong())  // Respond to ping but don't forward
  ws.on('pong', () => {})         // Ignore pong frames

  ws.on('message', (data, isBinary) => {
    // Only forward actual data frames (not control frames)
    if (isBinary) {
      socket.write(data as Buffer)
    } else {
      const text = data.toString('utf-8')
      if (!text.trim()) return  // Skip empty/whitespace frames
      socket.write(text + '\n')
    }
  })
}
```

---

## BUG-BE-TM-003 — Fix session-manager auth token injection diverges from HLD

**Mức độ:** 🟠 HIGH  
**Root cause:** Auth token được inject vào subprocess env theo cách khác với HLD spec.

### Fix — Align với HLD: sử dụng ORCA_SESSION_TOKEN

```typescript
// src/main/session/session-manager.ts

private spawnUserProcess(userId: string, session: OrcaSession): ChildProcess {
  return fork(join(__dirname, 'user-process-entry'), [], {
    env: {
      ...this.safeBaseEnv(),
      ORCA_USER_ID:      userId,
      ORCA_SOCKET_PATH:  this.getSocketPath(userId),
      // FIX BE-TM-003: HLD requires ORCA_SESSION_TOKEN (not ORCA_AUTH_TOKEN)
      ORCA_SESSION_TOKEN: session.id,  // Match HLD spec
      NODE_OPTIONS:      '--max-old-space-size=512',
    },
    stdio: ['ignore', 'inherit', 'inherit', 'ipc'],
  })
}

// user-process-entry.ts:
// Reads ORCA_SESSION_TOKEN để validate requests
const sessionToken = process.env.ORCA_SESSION_TOKEN
if (!sessionToken) {
  console.error('[UserProcess] ORCA_SESSION_TOKEN not set')
  process.exit(1)
}
```

---

## BUG-BE-TM-004 — Fix Agent WS server port mismatch với HLD

**Mức độ:** 🟡 MEDIUM  
**Root cause:** Agent WebSocket server chạy trên port 6767 nhưng HLD chỉ định 6768.

### Fix — Align port với HLD

```typescript
// src/server/index.ts hoặc src/main/server-bootstrap.ts

// HLD: Agent WS server là một endpoint trên chính OrcaRpcServer (port 6768)
// Không cần separate port!

// TRƯỚC (sai):
const agentWsServer = new WebSocketServer({ port: 6767 })  // BUG: separate port

// SAU — mount tại /agent path của main WS server (port 6768):
const mainWsServer = new WebSocketServer({ server: httpServer })  // port 6768

mainWsServer.on('connection', (ws, req) => {
  const url = new URL(req.url!, 'http://localhost')
  
  if (url.pathname === '/agent') {
    // Agent connection (inbound from Dev Server Agent)
    agentWsHandler.handle(ws, req)
  } else {
    // Regular client connection (from browser/desktop)
    clientWsHandler.handle(ws, req)
  }
})
```

---

## BUG-BE-TM-005 — Fix direct-WS missing disconnect handler

**Mức độ:** 🟠 HIGH  
**Root cause:** Khi direct-WebSocket client disconnect, không có cleanup → memory leak + orphaned PTY sessions.

### Fix — Add cleanup on disconnect

```typescript
// src/main/session/ws-session-router.ts

private handleConnection(ws: WebSocket, req: IncomingMessage, session: OrcaSession): void {
  const userProcess = this.sessionManager.getProcess(session.userId)
  if (!userProcess) {
    ws.close(4503, 'User process not available')
    return
  }

  const socket = createConnection(userProcess.socketPath)
  this.pipeWebSocketToSocket(ws, socket)

  // FIX BE-TM-005: Cleanup on disconnect
  ws.on('close', (code, reason) => {
    this.log.info(`[WsRouter] Client disconnected: userId=${session.userId} code=${code}`)
    
    // Close Unix socket connection
    socket.end()

    // Notify user process of disconnect (so it can pause PTY output)
    const msg = JSON.stringify({
      type:    'client.disconnected',
      userId:  session.userId,
      reason:  reason?.toString() ?? '',
    })
    socket.write(msg + '\n')
  })

  ws.on('error', (err) => {
    this.log.warn(`[WsRouter] WebSocket error: userId=${session.userId}`, err)
    socket.destroy()
  })

  socket.on('error', (err) => {
    this.log.warn(`[WsRouter] Socket error: userId=${session.userId}`, err)
    ws.close(1011, 'Internal socket error')
  })
}
```

---

## BUG-BE-TM-006 — Fix terminal.create thiếu RBAC check

**Mức độ:** 🔴 CRITICAL (Security)  
**Root cause:** Bất kỳ authenticated user nào cũng có thể tạo terminal trong project của user khác.

### Fix — Thêm RBAC check trong terminal.create

```typescript
// src/main/ipc/handlers.ts (hoặc src/main/runtime/rpc/methods/terminal.ts)

// TRƯỚC:
handler: async (params, context) => {
  const pty = await context.runtime.createTerminal(params)
  return { ptyId: pty.id }
}

// SAU — với RBAC:
handler: async (params, context) => {
  // FIX BE-TM-006: Check project access before creating terminal
  const project = await context.runtime.getProject(params.projectId)
  if (!project) {
    throw new RpcError(404, `Project not found: ${params.projectId}`)
  }

  // User must be owner or have 'terminal:create' permission
  const hasAccess = await context.rbac.check({
    userId:     context.session.userId,
    resource:   `project:${params.projectId}`,
    permission: 'terminal:create',
  })

  if (!hasAccess) {
    throw new RpcError(403, 'Forbidden: insufficient permissions to create terminal')
  }

  const pty = await context.runtime.createTerminal(params)
  return { ptyId: pty.id, cols: params.cols ?? 80, rows: params.rows ?? 24 }
}

// src/main/auth/rbac.ts (implement hoặc extend):
export class RbacChecker {
  async check(params: {
    userId:     string
    resource:   string
    permission: string
  }): Promise<boolean> {
    // Check project ownership
    if (params.resource.startsWith('project:')) {
      const projectId = params.resource.split(':')[1]
      const project   = await this.projectRepository.findById(projectId)
      if (project?.ownerId === params.userId) return true  // Owner = full access
    }

    // Check access policies
    const policies = await this.policyRepository.getForUser(params.userId)
    return policies.some(p =>
      p.resource === params.resource && p.permissions.includes(params.permission)
    )
  }
}
```

---

## BUG-TRM-BE-001 — Fix auth route mismatch

**Mức độ:** 🔴 HIGH  
**Root cause:** Frontend calls `/rpc/terminal.create` nhưng backend routes it to `/api/terminal.create` → 404.

### Fix — Align route paths

```typescript
// src/server/http-server.ts

// Đảm bảo RPC endpoint được mount đúng:
app.use('/rpc', requireAuth, createRpcRouter(rpcServer))  // Frontend expects /rpc
// KHÔNG mount tại /api/rpc (gây mismatch)

// WebSocket RPC handler tại ws://host:6768/rpc:
mainWsServer.on('connection', (ws, req) => {
  const url = new URL(req.url!, 'http://localhost')
  if (url.pathname === '/rpc' || url.pathname === '/') {
    // Standard RPC connection
    rpcServer.handleConnection(ws, req)
  } else if (url.pathname === '/agent') {
    agentWsHandler.handle(ws, req)
  }
})
```

---

## BUG-TRM-BE-002 — Fix WebSocket auth close với wrong code (4401 vs 401)

**Mức độ:** 🟡 MEDIUM  
**Root cause:** WS close code 4401 không chuẩn → frontend không distinguish auth error vs network error.

### Fix — Use standard WS close codes

```typescript
// src/main/session/ws-session-router.ts

// WS Close Code Standards:
// 4000-4099: Application-specific codes
// 4401 = Unauthorized (used by many WS implementations)
// 4403 = Forbidden

const WS_CLOSE_CODES = {
  UNAUTHORIZED:         4401,  // Missing/invalid auth → redirect to login
  FORBIDDEN:            4403,  // Valid auth but insufficient permission
  USER_PROCESS_ERROR:   4503,  // User process spawn failed (server error)
  SESSION_EXPIRED:      4408,  // Session token expired
} as const

// Apply in WsSessionRouter:
if (!session) {
  ws.close(WS_CLOSE_CODES.UNAUTHORIZED, 'Authentication required')
  return
}

if (!await rbac.canConnect(session.userId)) {
  ws.close(WS_CLOSE_CODES.FORBIDDEN, 'Access denied')
  return
}

// Frontend (TypeScript):
ws.addEventListener('close', (event) => {
  if (event.code === 4401) {
    // Redirect to login page
    window.location.href = '/login'
  } else if (event.code === 4403) {
    showError('Access denied')
  } else if (event.code === 4408) {
    // Session expired — try refresh token
    refreshSession()
  }
})
```

---

## Tóm tắt file changes

| File | Action | Bug |
|------|--------|-----|
| `src/main/runtime/orca-runtime.ts` | Convert relative → absolute paths | TM-002 |
| `src/main/session/ws-session-router.ts` | Forward binary frames as-is | BE-TM-001 |
| `src/main/session/ws-session-router.ts` | Filter ping/pong frames | BE-TM-002 |
| `src/main/session/session-manager.ts` | Use ORCA_SESSION_TOKEN (not ORCA_AUTH_TOKEN) | BE-TM-003 |
| `src/server/http-server.ts` | Agent WS on /agent path (not separate port) | BE-TM-004 |
| `src/main/session/ws-session-router.ts` | Add cleanup on disconnect | BE-TM-005 |
| `src/main/ipc/handlers.ts` | Add RBAC check for terminal.create | BE-TM-006 |
| `src/server/http-server.ts` | Fix RPC route path alignment | TRM-BE-001 |
| `src/main/session/ws-session-router.ts` | Use consistent WS close codes | TRM-BE-002 |

---

## Verification Plan

```bash
pnpm vitest run src/main/session/__tests__/
pnpm vitest run src/main/runtime/__tests__/

# Security test BE-TM-006:
# 1. User A tries to create terminal in User B's project → expect 403
# 2. Project owner creates terminal → expect success

# Protocol test BE-TM-001:
# 1. Send binary PTY data through WS → verify arrives unchanged on socket
# 2. Send JSON-RPC message → verify decoded correctly

# Auth test TRM-BE-002:
# 1. No session cookie → expect WS close 4401
# 2. Expired session → expect WS close 4408
# 3. Session OK → expect WS open + keepalive
```
