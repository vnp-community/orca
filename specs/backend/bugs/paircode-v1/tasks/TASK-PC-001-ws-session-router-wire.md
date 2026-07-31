# TASK-PC-001 — Wire WsSessionRouter vào HTTP Server Upgrade Event

**Solution:** [SOL-PC-001](../solutions/SOL-PC-001-ws-session-router-wiring.md)  
**Bug:** [BUG-PC-002](../BUG-PC-002-ws-session-router-not-wired.md)  
**File:** `src/server/index.ts`  
**Phụ thuộc:** Không  
**Estimated:** 30 phút  
**Status:** ✅ DONE — 2026-07-27

---

## Mục Tiêu

Thay `void wsRouter` (dòng 129) bằng code thực sự attach `WsSessionRouter` vào HTTP server `upgrade` event. Khi browser connect WebSocket với session cookie, request sẽ được route đến per-user process thay vì bị reject bằng E2EE handshake.

---

## Context

Đọc trước:
- `src/server/index.ts` — L108-132 (đoạn multi-user hiện tại)
- `src/main/session/ws-session-router.ts` — method `handleConnection(ws, req)`
- `src/shared/agent-wire-protocol.ts` — L28: `AGENT_WS_PATH = '/agent'`

---

## Thay Đổi Cần Thực Hiện

### File: `src/server/index.ts`

**TÌM đoạn hiện tại** (L108-132):

```typescript
  // ── Phase 2: Multi-User Mode (ORCA_MULTI_USER=1) ──────────────────────────
  // When enabled: each authenticated user gets their own forked OrcaRuntime process.
  // WsSessionRouter intercepts WS connections and proxies to the per-user process.
  let sessionManagerShutdown: (() => Promise<void>) | null = null
  const multiUserMode = process.env['ORCA_MULTI_USER'] === '1'

  if (multiUserMode) {
    const { SessionManager }   = await import('../main/session/session-manager')
    const { WsSessionRouter }  = await import('../main/session/ws-session-router')
    const { resolve: resolvePath } = await import('node:path')

    const baseDataPath      = adapter.app.getPath('userData')
    const userProcessEntry  = resolvePath(__dirname, '..', 'main', 'session', 'user-process-entry.js')

    const sessionManager = new SessionManager({ baseDataPath, userProcessEntry })
    const wsRouter       = new WsSessionRouter({ sessionManager, authManager })
    sessionManagerShutdown = () => sessionManager.shutdown()

    console.log('[Orca Server] ✅ Multi-user mode: SessionManager initialized')
    console.log(`[Orca Server]    User process entry: ${userProcessEntry}`)
    // WsSessionRouter is available for WebSocket server integration (TASK-021)
    void wsRouter  // suppress unused-var warning — wired in future WS server task
  } else {
    console.log('[Orca Server] Single-user mode (set ORCA_MULTI_USER=1 to enable per-user isolation)')
  }
```

**THAY BẰNG:**

```typescript
  // ── Phase 2: Multi-User Mode (ORCA_MULTI_USER=1) ──────────────────────────
  // When enabled: each authenticated user gets their own forked OrcaRuntime process.
  // WsSessionRouter intercepts WS connections and proxies to the per-user process.
  let sessionManagerShutdown: (() => Promise<void>) | null = null
  const multiUserMode = process.env['ORCA_MULTI_USER'] === '1'

  if (multiUserMode) {
    const { SessionManager }   = await import('../main/session/session-manager')
    const { WsSessionRouter }  = await import('../main/session/ws-session-router')
    const { WebSocketServer }  = await import('ws')
    const { resolve: resolvePath } = await import('node:path')
    const { AGENT_WS_PATH }    = await import('../shared/agent-wire-protocol')

    const baseDataPath      = adapter.app.getPath('userData')
    const userProcessEntry  = resolvePath(__dirname, '..', 'main', 'session', 'user-process-entry.js')

    const sessionManager = new SessionManager({ baseDataPath, userProcessEntry })
    const wsRouter       = new WsSessionRouter({ sessionManager, authManager })
    sessionManagerShutdown = () => sessionManager.shutdown()

    console.log('[Orca Server] ✅ Multi-user mode: SessionManager initialized')
    console.log(`[Orca Server]    User process entry: ${userProcessEntry}`)

    // Wire WsSessionRouter into HTTP server WebSocket upgrade event.
    // Why: browsers connect to port httpPort (6769) using session cookie auth.
    //      agentWsServer.attach() already handles AGENT_WS_PATH (/agent) upgrades.
    //      All other WS upgrade requests → WsSessionRouter → per-user process.
    if (httpServer) {
      const wss = new WebSocketServer({ noServer: true })

      httpServer.on('upgrade', (req, socket, head) => {
        const reqUrl = req.url ?? ''
        // Skip /agent path — already handled by agentWsServer.attach()
        if (reqUrl === AGENT_WS_PATH || reqUrl.startsWith(AGENT_WS_PATH + '?')) return

        wss.handleUpgrade(req, socket, head, (ws) => {
          void wsRouter.handleConnection(ws, req).catch((err: Error) => {
            console.error('[MultiUser] WsSessionRouter error:', err.message)
            const wsAny = ws as unknown as { readyState: number; OPEN: number; close: (c: number, r: string) => void }
            if (wsAny.readyState === wsAny.OPEN) wsAny.close(1011, 'Internal session error')
          })
        })
      })

      console.log(`[Orca Server] ✅ WsSessionRouter wired (port ${httpPort}) — browser login → per-user process`)
    } else {
      console.warn('[Orca Server] ⚠️  WsSessionRouter: httpServer unavailable — WS routing skipped')
    }

  } else {
    console.log('[Orca Server] Single-user mode (set ORCA_MULTI_USER=1 to enable per-user isolation)')
  }
```

---

## Verify

```bash
# 1. Build backend
cd /Users/binhnt/Work/blockchain/vnp-blc/orca
pnpm build:backend 2>&1 | tail -20

# 2. Deploy lên server
cd deploy/dev && ./scripts/sync-and-deploy.sh

# 3. Kiểm tra log sau deploy
ssh ubuntu@172.20.2.39 "docker logs orca-server 2>&1 | grep -E 'WsSessionRouter|Multi-user'"
# Expected:
# [Orca Server] ✅ Multi-user mode: SessionManager initialized
# [Orca Server] ✅ WsSessionRouter wired (port 6769) — browser login → per-user process

# 4. Test: browser kết nối WS với cookie (không bị 4001)
ssh ubuntu@172.20.2.39 "
  COOKIE_FILE=/tmp/orca-test-cookie.jar
  docker exec orca-server sh -c \"
    curl -sc $COOKIE_FILE -X POST http://localhost:6769/auth/local \
      -H 'Content-Type: application/json' \
      -d '{\\\"email\\\":\\\"admin@b15.openledger.vn\\\",\\\"password\\\":\\\"Orca@Adm1n#2025\\\"}' \
      -o /dev/null && echo 'Login OK'
  \"
"
# Expected: Login OK (không 401)
```

---

## Definition of Done

- [x] `void wsRouter` đã được thay bằng `httpServer.on('upgrade', ...)` handler
- [x] `/agent` path được skip đúng (không conflict với `agentWsServer`)
- [x] Log `[Orca Server] ✅ WsSessionRouter wired` xuất hiện khi `ORCA_MULTI_USER=1`
- [x] TypeScript compile OK (no type errors)
- [x] `pnpm build:backend` thành công
