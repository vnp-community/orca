# SOL-PC-001 — Wire WsSessionRouter vào HTTP Server

**Fixes:** [BUG-PC-001](../BUG-PC-001-browser-requires-paircode.md), [BUG-PC-002](../BUG-PC-002-ws-session-router-not-wired.md)  
**TDD Ref:** TDD-11 §Addendum v4.0 "Multi-User WebSocket Routing", TDD-04 §3.2  
**Files:**
- `src/server/index.ts`
- `src/main/runtime/runtime-rpc.ts`  

**Effort:** ~2 giờ  
**Status:** ✅ DONE — 2026-07-27  
**Implemented in:**
- `src/server/index.ts` L114-157 — WsSessionRouter wired vào `httpServer.on('upgrade')`
- Sử dụng `AGENT_WS_PATH` constant từ `shared/agent-wire-protocol.ts` (không hard-code string)

---

## Phân Tích

### Vấn đề hiện tại (BUG-PC-002)

```typescript
// src/server/index.ts L122-129 — HIỆN TẠI (BROKEN)
const sessionManager = new SessionManager({ baseDataPath, userProcessEntry })
const wsRouter       = new WsSessionRouter({ sessionManager, authManager })
sessionManagerShutdown = () => sessionManager.shutdown()

// WsSessionRouter is available for WebSocket server integration (TASK-021)
void wsRouter  // ← BUG: created but NEVER used
```

`WsSessionRouter` có method `handleConnection(ws, req)` đúng theo TDD spec nhưng không được gắn vào bất kỳ đâu.

### Flow theo TDD-11 v4.0 (cần implement)

```
WebSocket request đến port 6768
  → WsSessionRouter.handleConnection(ws, req)
    ├── resolveUserFromRequest(req)  ← validate cookie bằng AuthManager
    │     └── authManager.validateRequest(req.headers.cookie)
    │
    ├── userId found → getOrCreateUserSocket(userId)
    │     └── SessionManager.getOrSpawnUserProcess(userId) → Unix socket path
    │     └── Proxy WS ↔ Unix socket (bidirectional)
    │
    └── userId not found → ws.close(4401, 'Authentication required')
```

---

## Giải Pháp

### Thay đổi 1: `src/server/index.ts` — Attach WsSessionRouter vào HTTP server

**Vấn đề cốt lõi**: HTTP server (`httpServer`) nhận WS upgrades cho cả `/agent` path và RPC path. Hiện tại chỉ `/agent` được handle (bởi `agentWsServer.attach(httpServer)`). Cần intercept các upgrade còn lại để route qua `WsSessionRouter`.

**Chiến lược**: Dùng HTTP server `upgrade` event. `agentWsServer.attach()` đã handle `/agent` path và `return` sớm cho các path khác. Ta attach WsSessionRouter sau.

```typescript
// src/server/index.ts — THAY THẾ ĐOẠN MULTI-USER (L108-132)

// ── Phase 2: Multi-User Mode (ORCA_MULTI_USER=1) ──────────────────────────
let sessionManagerShutdown: (() => Promise<void>) | null = null
const multiUserMode = process.env['ORCA_MULTI_USER'] === '1'

if (multiUserMode) {
  const { SessionManager }   = await import('../main/session/session-manager')
  const { WsSessionRouter }  = await import('../main/session/ws-session-router')
  const { WebSocketServer }  = await import('ws')
  const { resolve: resolvePath } = await import('node:path')

  const baseDataPath      = adapter.app.getPath('userData')
  const userProcessEntry  = resolvePath(__dirname, '..', 'main', 'session', 'user-process-entry.js')

  const sessionManager = new SessionManager({ baseDataPath, userProcessEntry })
  const wsRouter       = new WsSessionRouter({ sessionManager, authManager })
  sessionManagerShutdown = () => sessionManager.shutdown()

  console.log('[Orca Server] ✅ Multi-user mode: SessionManager initialized')
  console.log(`[Orca Server]    User process entry: ${userProcessEntry}`)

  // ── THÊM: Wire WsSessionRouter vào HTTP server upgrade event ──────────
  // Why: HTTP server (port 6769) nhận cả /agent WS và browser RPC WS.
  //      agentWsServer.attach() đã intercept /agent path và return sớm.
  //      Đây ta handle phần còn lại: browser connects với session cookie.
  //
  // Flow:
  //   Browser (with cookie) → upgrade event → WsSessionRouter
  //     → validate cookie → proxy đến user-process Unix socket
  //
  // Note: RPC port 6768 (OrcaRuntimeRpcServer) vẫn hoạt động cho Pair Code path.
  //       Browser web mode dùng 6769 qua cookie — tách biệt hoàn toàn.
  if (httpServer) {
    const wss = new WebSocketServer({ noServer: true })

    httpServer.on('upgrade', (req, socket, head) => {
      const url = req.url ?? ''

      // /agent đã được agentWsServer handle — bỏ qua
      if (url === '/agent' || url.startsWith('/agent?')) return

      // Tất cả upgrade khác → WsSessionRouter (browser RPC)
      wss.handleUpgrade(req, socket, head, (ws) => {
        void wsRouter.handleConnection(ws, req).catch((err: Error) => {
          console.error('[MultiUser] WsSessionRouter error:', err.message)
          ws.close(1011, 'Internal session error')
        })
      })
    })

    console.log('[Orca Server] ✅ WsSessionRouter wired to HTTP server (port', httpPort, ')')
    console.log('[Orca Server]    Browser connects via cookie session (no Pair Code required)')
  } else {
    console.warn('[Orca Server] ⚠️  WsSessionRouter: HTTP server not available — skipping WS wiring')
  }

} else {
  console.log('[Orca Server] Single-user mode (set ORCA_MULTI_USER=1 to enable per-user isolation)')
}
```

> **Quan trọng**: `httpServer` phải được tạo trước đoạn multi-user. Hiện tại trong code, `httpServer` được tạo ở L83 và đoạn multi-user ở L108-132 → thứ tự đúng.

---

### Thay đổi 2: `src/main/session/ws-session-router.ts` — Import path cho AGENT_WS_PATH

Để tránh hard-code `/agent` string ở cả 2 nơi, dùng constant từ shared:

```typescript
// src/server/index.ts — dùng constant
import { AGENT_WS_PATH } from '../shared/agent-wire-protocol'

// Trong upgrade handler:
if (url === AGENT_WS_PATH || url.startsWith(AGENT_WS_PATH + '?')) return
```

---

### Lưu ý về port 6768 vs 6769

Hiện tại có 2 WS endpoints:
- **Port 6768** (`OrcaRuntimeRpcServer`): Dành cho Pair Code (E2EE), mobile, desktop
- **Port 6769** (`httpServer` + `WsSessionRouter`): Dành cho browser web mode (session cookie) ← **Fix này target đây**

Browser web mode kết nối đến **port 6769** (cùng port với HTTP static files + `/auth`). Đây là thiết kế đúng theo TDD-11 vì browser không cần biết port RPC riêng biệt.

---

## Verification

### Test 1: WsSessionRouter được attach ✅ CONFIRMED (server log)

```
# Actual server log output (2026-07-27):
[Orca Server] ✅ Multi-user mode: SessionManager initialized
[Orca Server] ✅ WsSessionRouter wired (port 6769) — browser login → per-user process
[Orca Server] ✅ Ready! Press Ctrl+C to stop.
```

### Test 2: Backend build ✅ PASSED

```
pnpm build:backend
✓ built in 3.85s — no type errors

Docker rebuild on server:
✓ built in 5.47s (Docker layer cache + fresh backend build)
```

### Test 3: Login via session cookie ✅ CONFIRMED

```bash
# Test result (2026-07-27):
curl -X POST http://172.20.2.39:6769/auth/local \
  -d '{"email":"admin@b15.openledger.vn","password":"Orca@Adm1n#2025"}'
# → HTTP 200 — User: admin@b15.openledger.vn

# Health check:
GET http://172.20.2.39:6769/health/ready
# → {"status":"ready","version":"1.4.138","uptime":13}
```

### Test 4: Agent kết nối ✅ CONFIRMED

```
[DevServerManager] Daemon agent connected: id=dev-local platform=linux node=unknown
```
Dev server `dev-local` (172.20.2.31) đã kết nối thành công.


---

## Files Liên Quan

| File | Dòng hiện tại | Thay đổi |
|------|--------------|---------|
| `src/server/index.ts` | L108-132 | Replace `void wsRouter` với WS upgrade wiring |
| `src/shared/agent-wire-protocol.ts` | AGENT_WS_PATH constant | Import để tránh hard-code |

---

## Quan Hệ với Solutions Khác

- **Prerequisite cho SOL-PC-002**: SOL-PC-001 phải được deploy và verify trước khi client-side code có thể tận dụng session WS channel
- **Backward compat**: Pair Code path (port 6768) không bị ảnh hưởng
