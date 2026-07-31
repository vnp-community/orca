# SOL-DS-004 — Reconnect Status + relay-ws Auto-Reconnect

**Fixes:** [BUG-DS-004](../BUG-DS-004-inmemory-state-lost-on-restart.md), [BUG-DS-005](../BUG-DS-005-relay-ws-no-reconnect.md)  
**TDD Ref:** TDD-08 (DevServer lifecycle), TDD-13 §3 (DevServerRelay.connect)  
**Files:** `src/main/dev-server/dev-server-manager.ts`, `src/main/dev-server/dev-server-relay-bridge.ts`  
**Effort:** ~1.5 giờ  
**Status:** ✅ DONE — 2026-07-27 (TASK-DS-007, 008)  
**Implemented in:**
- `dev-server-manager.ts` dng 55-91 — constructor + `restoreConnections()`
- `dev-server-relay-bridge.ts` dng 42-46, 121-140, 290-385 — fields + disconnect + reconnect loop

---

## Phần 1: BUG-DS-004 — Reconnect Status Sau Orca Server Restart

### Phân Tích

Theo TDD-13 §3.1, `DevServerManager` quản lý status lifecycle. Khi server restart:
- `direct-websocket` servers: agent sẽ tự reconnect (systemd) → status nên là `'connecting'` thay vì `'disconnected'`
- `relay-ssh` và `relay-websocket`: không có auto-reconnect → status = `'disconnected'` (đúng)

### Thay Đổi: `src/main/dev-server/dev-server-manager.ts`

**Constructor** — sửa init state cho direct-websocket:

```typescript
// TRƯỚC:
constructor(persistStore: Store, private sshManager: SshConnectionManager,
            private agentWsServer: AgentWebSocketServer | null = null) {
  super()
  this.store = new DevServerStore(persistStore)
  for (const ds of this.store.list()) {
    this.initRuntimeState(ds.id)  // all = 'disconnected'
  }
}

// SAU:
constructor(persistStore: Store, private sshManager: SshConnectionManager,
            private agentWsServer: AgentWebSocketServer | null = null) {
  super()
  this.store = new DevServerStore(persistStore)
  for (const ds of this.store.list()) {
    this.initRuntimeState(ds.id)
    // direct-websocket: agent tự reconnect via systemd — show 'connecting' not 'disconnected'
    if (ds.connectionType === 'direct-websocket') {
      this.setRuntimeState(ds.id, { status: 'connecting', lastError: null })
    }
  }
}
```

**Thêm method `restoreConnections()`** — gọi sau startup để trigger daemon agent slots:

```typescript
/**
 * Restore direct-websocket connections after server restart.
 * For each persisted direct-websocket DevServer, emit 'statusChanged' → 'connecting'
 * so the UI shows correct state. The actual connection will be established when
 * the daemon agent restarts and calls POST /api/agent-token.
 *
 * relay-websocket / relay-ssh: cannot auto-restore, leave as 'disconnected'.
 */
restoreConnections(): void {
  for (const ds of this.store.list()) {
    if (ds.connectionType === 'direct-websocket') {
      // Broadcast status so any already-connected UI client gets updated state
      this.emit('devServer:statusChanged', ds.id, 'connecting')
      console.log(`[DevServerManager] Startup: ${ds.id} (direct-ws) → awaiting daemon reconnect`)
    }
  }
}
```

**Gọi trong server bootstrap** (`src/server/index.ts` hoặc `server-bootstrap.ts`):
```typescript
// Sau khi tạo devServerManager:
devServerManager.restoreConnections()
```

---

## Phần 2: BUG-DS-005 — relay-ws Auto-Reconnect

### Phân Tích

Theo TDD-13 §3.2, `DevServerRelay` interface không define reconnect nhưng production reliability yêu cầu nó. Pattern đơn giản nhất: monitor WebSocket `close` event, retry sau delay.

### Thay Đổi: `src/main/dev-server/dev-server-relay-bridge.ts`

**Thêm private field**:
```typescript
private _relayWsReconnectTimer: ReturnType<typeof setTimeout> | null = null
private _relayWsActive = false
```

**Sửa `connectRelayWebSocket()`**:

```typescript
private connectRelayWebSocket(
  rawUrl: string,
  opts: { testOnly?: boolean }
): Promise<RelayHandshakeInfo> {
  const url = new URL(rawUrl)
  const token = url.searchParams.get('token') ?? ''
  url.searchParams.delete('token')
  const cleanUrl = url.toString()
  const orcaVersion = getPlatform().app.getVersion()

  this._relayWsActive = !opts.testOnly  // don't reconnect for test-only probes

  return new Promise<RelayHandshakeInfo>((resolve, reject) => {
    let initialResolved = false

    const attempt = () => {
      if (!this._relayWsActive) return  // stopped (disconnect() called)

      const ws = new WebSocket(cleanUrl, {
        headers: token ? { Authorization: `Bearer ${token}` } : {},
      })
      ;(ws as unknown as { binaryType: string }).binaryType = 'nodebuffer'

      const connectionTimeout = setTimeout(() => {
        ws.terminate()
        if (!initialResolved) {
          reject(new Error(
            `relay-websocket: TCP connection timed out after 10s to ${cleanUrl}. ` +
            `Verify the agent is running and the address is reachable.`
          ))
        } else {
          // Post-resolve timeout: will retry via close handler
          console.warn(`[RelayBridge] relay-ws reconnect timeout to ${cleanUrl}`)
        }
      }, 10_000)

      ws.on('error', (err: Error) => {
        clearTimeout(connectionTimeout)
        if (!initialResolved) {
          reject(new Error(`relay-websocket: WebSocket error: ${err.message}`))
        } else {
          console.warn(`[RelayBridge] relay-ws error: ${err.message} — retry in 15s`)
          // close will fire next, retry handled there
        }
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

            // Monitor for disconnect → trigger reconnect
            if (!opts.testOnly) {
              ws.on('close', () => {
                if (this.session) {
                  console.log('[RelayBridge] relay-ws disconnected — reconnecting in 15s...')
                  this.session = null
                }
                if (this._relayWsActive) {
                  this._relayWsReconnectTimer = setTimeout(attempt, 15_000)
                }
              })
            }

            if (!initialResolved) {
              initialResolved = true
              resolve({
                platform: (info.platform as NodeJS.Platform) ?? 'linux',
                arch: info.arch,
                nodeVersion: info.nodeVersion,
                relayVersion: info.agentVersion,
              })
            }
          })
          .catch((err: Error) => {
            ws.close()
            if (!initialResolved) {
              reject(err)
            } else {
              console.warn(`[RelayBridge] relay-ws handshake failed — retry in 15s: ${err.message}`)
              this._relayWsReconnectTimer = setTimeout(attempt, 15_000)
            }
          })
      })
    }

    attempt()
  })
}
```

**Sửa `disconnect()`** để stop reconnect loop:

```typescript
async disconnect(): Promise<void> {
  this._relayWsActive = false  // ← thêm dòng này
  if (this._relayWsReconnectTimer) {
    clearTimeout(this._relayWsReconnectTimer)
    this._relayWsReconnectTimer = null
  }
  // ... rest of existing disconnect logic
}
```

---

## Verification

### BUG-DS-004 test:
```bash
# 1. Agent đang connected
# 2. docker restart orca-server
# 3. Quan sát UI: phải hiện "Connecting..." không phải "Disconnected"
# 4. Sau ~30s: agent reconnect → "Connected" ✅
```

### BUG-DS-005 test:
```bash
# 1. relay-ws agent running trên 172.20.2.31:6799
# 2. Orca connected (relay-ws mode)
# 3. docker restart orca-server
# 4. Quan sát server logs:
#    "[RelayBridge] relay-ws disconnected — reconnecting in 15s..."
# 5. Sau 15s: tự động reconnect ✅ — KHÔNG cần thao tác thủ công trong UI
```

---

## Files Liên Quan

| File | Thay đổi |
|------|---------|
| `src/main/dev-server/dev-server-manager.ts` | Constructor: direct-ws → 'connecting'; thêm `restoreConnections()` |
| `src/main/dev-server/dev-server-relay-bridge.ts` | `connectRelayWebSocket()`: thêm reconnect loop; `disconnect()`: stop loop |
| `src/server/index.ts` hoặc `server-bootstrap.ts` | Gọi `devServerManager.restoreConnections()` sau startup |
