# TASK-DS-008 — relay-ws Auto-Reconnect Loop Trong `DevServerRelayBridge`

**Solution:** [SOL-DS-004 §2](../solutions/SOL-DS-004-reconnect-status.md)  
**Bug:** [BUG-DS-005](../BUG-DS-005-relay-ws-no-reconnect.md)  
**File:** `src/main/dev-server/dev-server-relay-bridge.ts`  
**Phụ thuộc:** TASK-DS-007 (context về reconnect pattern)  
**Estimated:** 45 phút  
**Status:** ✅ DONE — 2026-07-27

---

## Mục Tiêu

Thêm auto-reconnect loop vào `connectRelayWebSocket()`. Khi WebSocket bị đóng (Orca restart, network drop), tự động retry sau 15 giây. Không cần user thao tác thủ công trong UI.

---

## Context

Đọc trước:
- `src/main/dev-server/dev-server-relay-bridge.ts` — hàm `connectRelayWebSocket()` (dòng ~270-331) và `disconnect()` (dòng ~121-133)
- `src/main/dev-server/ws-handshake.ts` — `runOrcaInitiatorHandshake()` để hiểu handshake có thể fail

---

## Thay Đổi Cần Thực Hiện

**File:** `src/main/dev-server/dev-server-relay-bridge.ts`

### Bước 1: Thêm private fields (sau `_directWsDisposer`)

```typescript
  /** Cancel flag để dừng reconnect loop khi disconnect() được gọi */
  private _relayWsActive = false
  /** Timer handle cho reconnect delay */
  private _relayWsReconnectTimer: ReturnType<typeof setTimeout> | null = null
```

### Bước 2: Sửa `connectRelayWebSocket()` — thay toàn bộ method

**TÌM toàn bộ method** `private connectRelayWebSocket(...)` và **THAY BẰNG:**

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

    // testOnly probe: không cần reconnect loop
    this._relayWsActive = !opts.testOnly

    return new Promise<RelayHandshakeInfo>((resolve, reject) => {
      let initialResolved = false

      const attempt = () => {
        if (!this._relayWsActive) return  // disconnect() đã gọi → dừng

        const ws = new WebSocket(cleanUrl, {
          headers: token ? { Authorization: `Bearer ${token}` } : {},
        })
        ;(ws as unknown as { binaryType: string }).binaryType = 'nodebuffer'

        const connectionTimeout = setTimeout(() => {
          ws.terminate()
          const msg = `relay-websocket: TCP connection timed out after 10s to ${cleanUrl}.`
          if (!initialResolved) {
            reject(new Error(`${msg} Verify the agent is running and reachable.`))
          } else {
            console.warn(`[RelayBridge] ${msg} Retry in 15s.`)
          }
        }, 10_000)

        ws.on('error', (err: Error) => {
          clearTimeout(connectionTimeout)
          if (!initialResolved) {
            reject(new Error(`relay-websocket: WebSocket error: ${err.message}`))
          } else {
            console.warn(`[RelayBridge] relay-ws error: ${err.message}`)
            // 'close' event sẽ fire sau → retry handled ở đó
          }
        })

        ws.on('open', () => {
          clearTimeout(connectionTimeout)

          runOrcaInitiatorHandshake(ws, orcaVersion)
            .then((info) => {
              const transport = createWebSocketTransport(ws)
              this.session = new SshChannelMultiplexer(transport)

              // Monitor disconnect → trigger reconnect (skip for testOnly)
              if (!opts.testOnly) {
                ws.on('close', () => {
                  if (this.session) {
                    console.log('[RelayBridge] relay-ws closed — clearing session')
                    this.session = null
                  }
                  if (this._relayWsActive) {
                    console.log('[RelayBridge] relay-ws will reconnect in 15s...')
                    this._relayWsReconnectTimer = setTimeout(attempt, 15_000)
                  }
                })
              }

              if (opts.testOnly) {
                void this.disconnect()
              }

              if (!initialResolved) {
                initialResolved = true
                resolve({
                  platform: (info.platform as NodeJS.Platform) ?? 'linux',
                  arch: info.arch,
                  nodeVersion: info.nodeVersion,
                  relayVersion: info.agentVersion,
                })
              } else {
                console.log('[RelayBridge] relay-ws reconnected successfully')
              }
            })
            .catch((err: Error) => {
              ws.close()
              if (!initialResolved) {
                reject(err)
              } else {
                console.warn(`[RelayBridge] relay-ws handshake failed: ${err.message} — retry in 15s`)
                if (this._relayWsActive) {
                  this._relayWsReconnectTimer = setTimeout(attempt, 15_000)
                }
              }
            })
        })
      }

      attempt()
    })
  }
```

### Bước 3: Sửa `disconnect()` — thêm cleanup cho reconnect state

**TÌM** trong `disconnect()`:
```typescript
  async disconnect(): Promise<void> {
    // Cancel direct-websocket slot if still pending
    if (this._directWsDisposer) {
```

**THÊM 3 dòng ĐẦU** của method `disconnect()`:
```typescript
  async disconnect(): Promise<void> {
    // Stop relay-ws reconnect loop first
    this._relayWsActive = false
    if (this._relayWsReconnectTimer) {
      clearTimeout(this._relayWsReconnectTimer)
      this._relayWsReconnectTimer = null
    }
    // Cancel direct-websocket slot if still pending
    if (this._directWsDisposer) {
```

---

## Verify

```bash
# 1. Setup relay-ws agent trên dev server
# 2. Orca connect → "Connected" ✅
# 3. docker restart orca-server
# 4. Quan sát server logs sau restart:
grep "relay-ws\|RelayBridge" logs/server.log
# Expected trong 15s: "[RelayBridge] relay-ws will reconnect in 15s..."
#                     "[RelayBridge] relay-ws reconnected successfully"
# KHÔNG cần thao tác thủ công trong UI ✅
```

---

## Definition of Done

- [x] Private fields `_relayWsActive` và `_relayWsReconnectTimer` đã thêm
- [x] `connectRelayWebSocket()` có reconnect loop (`ws.on('close')` → setTimeout → attempt)
- [x] `disconnect()` dừng reconnect loop trước khi close session
- [x] testOnly mode không có reconnect loop
- [x] `initialResolved` flag để không reject() sau khi đã resolve() ban đầu
- [x] TypeScript compile OK (no errors)
