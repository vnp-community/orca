# TASK-006: Sửa `dev-server-relay-bridge.ts` — thêm relay-websocket branch

> **Status:** ✅ DONE (2026-07-26)
> **TypeScript:** 0 errors

**Status:** ✅ DONE  
**Phase:** 3 — relay-websocket mode  
**Solution:** [SOL-AG-003](../solutions/SOL-AG-003-relay-websocket.md) §3.1  
**Depends on:** TASK-003, TASK-004  
**Blocks:** TASK-007  

---

## Mục tiêu

Sửa `src/main/dev-server/dev-server-relay-bridge.ts` để:
1. Thêm `import WebSocket from 'ws'`
2. Thêm private method `connectRelayWebSocket()`
3. Fill branch `relay-websocket` trong `connect()` — hiện đang throw error

---

## File cần sửa

**Path:** `src/main/dev-server/dev-server-relay-bridge.ts`

---

## Thay đổi cần thực hiện

### 1. Thêm imports (ở đầu file, sau các imports hiện có)

```typescript
import WebSocket from 'ws'
import { createWebSocketTransport } from './ws-transport'
import { runOrcaInitiatorHandshake } from './ws-handshake'
import { getPlatform } from '../../platform/context'
import { SshChannelMultiplexer } from '../ssh/ssh-channel-multiplexer'
```

### 2. Sửa `connect()` — thêm relay-websocket branch TRƯỚC throw

**Tìm đoạn code hiện tại:**
```typescript
    // relay-websocket / direct-websocket: Phase 2 — not yet implemented
    throw new Error(
      `Connection type '${this.config.connectionType}' is not yet implemented. ` +
        `Only 'relay-ssh' is supported in Phase 1.`
    )
```

**Thay bằng:**
```typescript
    // ─── Phase 2: relay-websocket ──────────────────────────────────────────────
    if (this.config.connectionType === 'relay-websocket') {
      const wsUrl = this.config.wsUrl
      if (!wsUrl) {
        throw new Error(
          `DevServer '${this.config.name}' has no wsUrl configured. ` +
          `Set wsUrl to ws://host:port/orca-relay?token=<secret> for relay-websocket mode.`
        )
      }
      return this.connectRelayWebSocket(wsUrl, opts)
    }

    // ─── Phase 2: direct-websocket ─────────────────────────────────────────────
    // (SOL-AG-004 / TASK-010 — placeholder until AgentWebSocketServer is wired)
    if (this.config.connectionType === 'direct-websocket') {
      throw new Error(
        `direct-websocket mode requires AgentWebSocketServer — not yet wired in this build (TASK-010)`
      )
    }

    throw new Error(
      `Connection type '${this.config.connectionType}' is not supported. ` +
      `Supported types: relay-ssh, relay-websocket, direct-websocket`
    )
```

### 3. Thêm private method `connectRelayWebSocket()` (sau `disconnect()`)

```typescript
  /**
   * relay-websocket mode: Orca acts as WebSocket CLIENT, connecting to agent's WS server.
   *
   * URL format: ws://host:port/path?token=<secret>
   *   Token is stripped from URL and sent as Authorization: Bearer <token> header.
   *
   * Flow:
   *   1. TCP connect to agent WS server
   *   2. Run agent.handshake (Orca initiator)
   *   3. Wire SshChannelMultiplexer on transport
   *   4. Return RelayHandshakeInfo
   */
  private connectRelayWebSocket(
    rawUrl: string,
    opts: { testOnly?: boolean }
  ): Promise<RelayHandshakeInfo> {
    // Parse token from URL: ws://host:port/path?token=<secret>
    // Why: UI stores token in URL query string for simplicity;
    // strip it before creating WS to send as Authorization header instead.
    const url = new URL(rawUrl)
    const token = url.searchParams.get('token') ?? ''
    url.searchParams.delete('token')
    const cleanUrl = url.toString()

    const orcaVersion = getPlatform().app.getVersion()

    return new Promise<RelayHandshakeInfo>((resolve, reject) => {
      const ws = new WebSocket(cleanUrl, {
        headers: token ? { Authorization: `Bearer ${token}` } : {},
      })

      // Ensure binary messages are delivered as Buffer (not string/ArrayBuffer)
      ;(ws as unknown as { binaryType: string }).binaryType = 'nodebuffer'

      // TCP connection timeout (before WS handshake)
      const connectionTimeout = setTimeout(() => {
        ws.close()
        reject(new Error(
          `relay-websocket: TCP connection timed out after 10s to ${cleanUrl}. ` +
          `Verify the agent is running and the address is reachable.`
        ))
      }, 10_000)

      ws.on('error', (err: Error) => {
        clearTimeout(connectionTimeout)
        reject(new Error(`relay-websocket: WebSocket error connecting to ${cleanUrl}: ${err.message}`))
      })

      ws.on('open', () => {
        clearTimeout(connectionTimeout)

        runOrcaInitiatorHandshake(ws, orcaVersion)
          .then((info) => {
            const transport = createWebSocketTransport(ws)
            this.session = new SshChannelMultiplexer(transport)

            if (opts.testOnly) {
              void this.disconnect()
            }

            resolve({
              platform: (info.platform as NodeJS.Platform) ?? 'linux',
              arch: info.arch,
              nodeVersion: info.nodeVersion,
              relayVersion: info.agentVersion,
            })
          })
          .catch((err: Error) => {
            ws.close()
            reject(err)
          })
      })
    })
  }
```

---

## Acceptance Criteria

- [x] `DevServerRelayBridge.connect()` không throw error cho `connectionType === 'relay-websocket'`
- [x] `wsUrl` không có → throw error với message hữu ích (không cần SSH)
- [x] Token được strip khỏi URL trước khi tạo WebSocket
- [x] Token được pass qua `Authorization: Bearer` header
- [x] TCP timeout 10s với error message hữu ích
- [x] Sau handshake thành công: `this.session` là `SshChannelMultiplexer` instance
- [x] `testOnly=true`: `disconnect()` được gọi sau handshake (session về null)
- [x] TypeScript compile không lỗi
- [x] `direct-websocket` không throw nữa — implemented by TASK-010 (`connectDirectWebSocket()`)
