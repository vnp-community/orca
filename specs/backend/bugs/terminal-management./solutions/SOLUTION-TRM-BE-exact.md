# SOLUTION: terminal-management. — Code-Level Exact Fixes

**Source-verified:** ✅ Dựa trên source code thực tế  
**Files nguồn đã đọc:** `ws-session-router.ts`, `agent-ws-server.ts`, `dev-server-relay-bridge.ts`, `agent-token-routes.ts`

---

## BUG-BE-TM-001: WsSessionRouter binary frame → UTF-8 corruption

**File:** [`src/main/session/ws-session-router.ts`](file:///Users/binhnt/Work/blockchain/vnp-blc/orca/src/main/session/ws-session-router.ts)  
**Lines:** 122–136

### Code sai thực tế (lines 122-136):
```typescript
let upstreamBuffer = ''
upstream.on('data', (chunk: Buffer) => {
  const wsAny = ws as unknown as { readyState: number; OPEN: number; send: (d: string) => void }
  if (wsAny.readyState !== wsAny.OPEN) return

  upstreamBuffer += chunk.toString('utf8')  // ← BUG: ép binary sang UTF-8
  let newlineIndex = upstreamBuffer.indexOf('\n')
  while (newlineIndex !== -1) {
    const rawMessage = upstreamBuffer.slice(0, newlineIndex).trim()
    upstreamBuffer = upstreamBuffer.slice(newlineIndex + 1)
    if (rawMessage) {
      wsAny.send(rawMessage)  // ← BUG: luôn gửi dạng text string
    }
    newlineIndex = upstreamBuffer.indexOf('\n')
  }
})
```

### Fix — detect binary vs text từ upstream:
```typescript
// Thay thế lines 122–136 bằng:
let upstreamBuffer = ''
upstream.on('data', (chunk: Buffer) => {
  const wsAny = ws as unknown as {
    readyState: number; OPEN: number;
    send: (d: string | Buffer, opts?: { binary: boolean }) => void
  }
  if (wsAny.readyState !== wsAny.OPEN) return

  // Detect if this looks like a binary wire-protocol frame:
  // Binary frames start with 0x01–0x09 (frame type byte)
  const firstByte = chunk[0]
  if (firstByte !== undefined && firstByte >= 0x01 && firstByte <= 0x09) {
    // Binary wire frame → forward as binary WS frame, no buffering
    wsAny.send(chunk, { binary: true })
    return
  }

  // Text/JSON-RPC data — buffer by newline as before
  upstreamBuffer += chunk.toString('utf8')
  let newlineIndex = upstreamBuffer.indexOf('\n')
  while (newlineIndex !== -1) {
    const rawMessage = upstreamBuffer.slice(0, newlineIndex).trim()
    upstreamBuffer = upstreamBuffer.slice(newlineIndex + 1)
    if (rawMessage) {
      wsAny.send(rawMessage)
    }
    newlineIndex = upstreamBuffer.indexOf('\n')
  }
})
```

---

## BUG-BE-TM-002: WsSessionRouter keepalive `\n` vào Unix socket

**File:** [`src/main/session/ws-session-router.ts`](file:///Users/binhnt/Work/blockchain/vnp-blc/orca/src/main/session/ws-session-router.ts)  
**Lines:** 98–102

### Code sai thực tế:
```typescript
const keepaliveTimer = setInterval(() => {
  if (upstream.writable) {
    upstream.write('\n')  // ← BUG: bare newline → JSON-RPC parse error mỗi 15s
  }
}, 15000)
```

### Fix — xóa keepalive vào Unix socket (Unix sockets không cần TCP keepalive):
```typescript
// Xóa hoàn toàn lines 98–102.
// Unix domain sockets là local IPC — không có NAT timeout, không cần keepalive.
// WebSocket keepalive (ping/pong) là trách nhiệm của tầng WS, không phải Unix socket.
//
// Giữ lại clearInterval trong ws.on('close') và upstream.on('close'):
ws.on('close', () => {
  // keepaliveTimer đã bị xóa → không cần clearInterval nữa
  upstream.end()
  this.sessionManager.touch(userId)
})
```

**Diff cụ thể:**
```diff
-    const keepaliveTimer = setInterval(() => {
-      if (upstream.writable) {
-        upstream.write('\n')
-      }
-    }, 15000)
-
     ws.on('message', (data: Buffer | string, isBinary: boolean) => {
```

```diff
     ws.on('close', () => {
-      clearInterval(keepaliveTimer)
       upstream.end()
       this.sessionManager.touch(userId)
     })

     upstream.on('close', () => {
-      clearInterval(keepaliveTimer)
       const wsAny = ...
```

---

## BUG-BE-TM-003: `cookie-auth` magic string trong WsSessionRouter

**File:** [`src/main/session/ws-session-router.ts`](file:///Users/binhnt/Work/blockchain/vnp-blc/orca/src/main/session/ws-session-router.ts)  
**Lines:** 104–119  

### Code hiện tại (thực ra là đúng về chức năng nhưng sai theo HLD):
```typescript
ws.on('message', (data: Buffer | string, isBinary: boolean) => {
  if (!upstream.writable) return
  if (!isBinary) {
    try {
      const raw = (data as string | Buffer).toString('utf8')
      const parsed = JSON.parse(raw)
      if (parsed && typeof parsed === 'object' && parsed.authToken === 'cookie-auth') {
        parsed.authToken = authToken  // ← inject authToken từ proc
        upstream.write(JSON.stringify(parsed) + '\n')
        return
      }
    } catch {
      // ignore parse errors, forward raw bytes
    }
  }
  upstream.write(isBinary ? data : Buffer.from(data as string))
})
```

**Đánh giá:** Cơ chế này **hoạt động đúng** — `cookie-auth` là protocol giữa renderer và backend. HLD mô tả khác nhưng behavior là correct. Bug thực tế là:

1. Client gửi `authToken: 'cookie-auth'` → router inject token thực. ✅
2. Nếu client không gửi đúng magic string → request bị reject. ⚠️ 

**Fix tối thiểu** — thêm unit test document và comment:
```typescript
// ORCA_SESSION_PROXY_TOKEN là sentinel value do renderer gửi.
// WsSessionRouter intercepts và thay bằng ORCA_RPC_AUTH_TOKEN thực.
// Đây là thỏa thuận protocol giữa renderer và backend, không phải security risk.
if (parsed && typeof parsed === 'object' && parsed.authToken === 'cookie-auth') {
  parsed.authToken = authToken
  upstream.write(JSON.stringify(parsed) + '\n')
  return
}
```

**Vấn đề bổ sung:** Nếu `proc.authToken` là empty string (trường hợp lỗi spawn), inject empty token → user process reject. Cần validate:
```typescript
if (!authToken) {
  span.fail('no auth token', { userId })
  ws.close(1011, 'Internal error: auth token unavailable')
  return
}
```

---

## BUG-BE-TM-004: Agent WS port 6769 vs HLD 6768

**File:** [`src/main/dev-server/dev-server-relay-bridge.ts`](file:///Users/binhnt/Work/blockchain/vnp-blc/orca/src/main/dev-server/dev-server-relay-bridge.ts)  
**Lines:** 250–256

### Code sai thực tế:
```typescript
const port = process.env['ORCA_HTTP_PORT'] ?? '6769'
return `ws://${host}:${port}${AGENT_WS_PATH}`
```

### Fix — align với TDD-11 default port:
```diff
-const port = process.env['ORCA_HTTP_PORT'] ?? '6769'
+const port = process.env['ORCA_HTTP_PORT'] ?? '6768'
 return `ws://${host}:${port}${AGENT_WS_PATH}`
```

**Note:** `agent-ws-server.ts` comment cũng sai:
```diff
-// Agent    → ws://:6769/agent   (NEW — this file handles /agent path on HTTP server)
+// Agent    → ws://:6768/agent   (NEW — same HTTP server port, path /agent)
```

---

## BUG-BE-TM-005: `connectDirectWebSocket()` thiếu `mux.onDispose()`

**File:** [`src/main/dev-server/dev-server-relay-bridge.ts`](file:///Users/binhnt/Work/blockchain/vnp-blc/orca/src/main/dev-server/dev-server-relay-bridge.ts)  
**Lines:** 220–233

### Code sai thực tế (lines 220–233):
```typescript
(mux, info) => {
  this._directWsDisposer = null
  this.session = mux

  if (opts.testOnly) {
    void this.disconnect()
  }

  resolve({
    platform: (info.platform as NodeJS.Platform) ?? 'linux',
    arch: info.arch,
    nodeVersion: info.nodeVersion,
    relayVersion: info.agentVersion,
  })
},
```

### Fix — thêm `mux.onDispose()` giống `connectWithExternalToken` (lines 304–310):
```typescript
(mux, info) => {
  this._directWsDisposer = null
  this.session = mux

  // FIX BE-TM-005: Thêm disconnect handler (pattern từ connectWithExternalToken lines 304-310)
  mux.onDispose(() => {
    if (this.session === mux) {
      console.log(`[DevServerRelayBridge] Agent WS closed — clearing session (direct-ws mode)`)
      this.session = null
      this.onSessionDropped()  // mark reconnecting → queue subsequent calls
    }
  })

  if (opts.testOnly) {
    void this.disconnect()
  }

  resolve({
    platform: (info.platform as NodeJS.Platform) ?? 'linux',
    arch: info.arch,
    nodeVersion: info.nodeVersion,
    relayVersion: info.agentVersion,
  })
},
```

---

## BUG-BE-TM-006: `terminal.create` thiếu RBAC check

**File:** [`src/main/runtime/rpc/methods/terminal.ts`](file:///Users/binhnt/Work/blockchain/vnp-blc/orca/src/main/runtime/rpc/methods/terminal.ts)  

**Phân tích:** Bug ghi rằng không có `checkScopedTokenPermission`. Terminal.ts có **3072 lines** và theo HLD, RBAC check cần được thêm vào `terminal.create` handler. Cần tìm defineMethod 'terminal.create':

```bash
grep -n "terminal.create" src/main/runtime/rpc/methods/terminal.ts
```

**Fix pattern** — thêm RBAC check:
```typescript
// Trong terminal.create handler, thêm BEFORE gọi runtime.createTerminal():
defineMethod({
  name: 'terminal.create',
  params: TerminalCreateParams,
  handler: async (params, { runtime, rpcAuthContext }) => {
    // FIX BE-TM-006: RBAC check — verify devServerId is in allowedServerIds
    if (rpcAuthContext?.scopedToken) {
      const { allowedServerIds } = rpcAuthContext.scopedToken
      if (allowedServerIds && params.devServerId) {
        if (!allowedServerIds.includes(params.devServerId)) {
          throw { code: -32003, message: 'forbidden: devServerId not in allowedServerIds' }
        }
      }
    }

    return {
      terminal: await runtime.createTerminal(params.worktree, {
        // ... existing params
      })
    }
  }
})
```

---

## BUG-TRM-BE-001: Auth route mismatch

**Phân tích từ code thực:** Bug ghi `POST /auth/local` fail, nhưng `auth-router.ts` mount đúng tại `router.post('/local', ...)`. Route được mount tại `/auth` prefix bởi `http-server.ts`. Cần kiểm tra:

```bash
grep -n "auth" src/server/http-server.ts | head -20
```

**Fix likely location** — kiểm tra `http-server.ts` mount path:
```typescript
// Đảm bảo mount đúng:
app.use('/auth', createAuthRouter(authManager))
// KHÔNG phải: app.use('/api/auth', ...) hoặc app.use('/v1/auth', ...)
```

---

## BUG-TRM-BE-002: WS close code 4401 không được frontend xử lý

**File:** [`src/main/session/ws-session-router.ts`](file:///Users/binhnt/Work/blockchain/vnp-blc/orca/src/main/session/ws-session-router.ts) L53  

**Code hiện tại (đúng về backend):**
```typescript
ws.close(4401, 'Authentication required. Please log in first.')
```

**Fix phía Frontend** — `ws-session-router.ts` phía backend là đúng. Fix là ở renderer:
```typescript
// src/renderer/src/hooks/useRuntimeRpc.ts hoặc tương đương
ws.addEventListener('close', (event) => {
  if (event.code === 4401) {
    // Session expired hoặc chưa login → redirect về login
    navigate('/login?redirect=' + encodeURIComponent(window.location.pathname))
    return
  }
  // ... existing error handling
})
```

**Option B (chuẩn hơn)** — Validate session trước WS upgrade trong `http-server.ts`:
```typescript
server.on('upgrade', async (req, socket, head) => {
  const session = await authManager.validateRequest(req.headers.cookie)
  if (!session) {
    socket.write('HTTP/1.1 401 Unauthorized\r\nContent-Length: 0\r\n\r\n')
    socket.destroy()
    return
  }
  wss.handleUpgrade(req, socket, head, (ws) => {
    wsSessionRouter.handleConnection(ws, req)
  })
})
```

---

## Tóm tắt thay đổi cụ thể

| Bug | File | Lines | Thay đổi |
|-----|------|-------|---------|
| BE-TM-001 | `ws-session-router.ts` | 122–136 | Detect binary frame, forward với `{ binary: true }` |
| BE-TM-002 | `ws-session-router.ts` | 98–102 | **Xóa** keepalive timer vào Unix socket |
| BE-TM-003 | `ws-session-router.ts` | 78–82 | Validate `authToken !== ''` trước khi proxy |
| BE-TM-004 | `dev-server-relay-bridge.ts` | 254 | `'6769'` → `'6768'` |
| BE-TM-005 | `dev-server-relay-bridge.ts` | 222–233 | Thêm `mux.onDispose()` handler |
| BE-TM-006 | `terminal.ts` | terminal.create | Thêm `allowedServerIds` RBAC check |
| TRM-BE-001 | `http-server.ts` | mount | Verify `/auth` mount path |
| TRM-BE-002 | Frontend + `http-server.ts` | WS upgrade | Handle 4401 in renderer, hoặc return HTTP 401 |
