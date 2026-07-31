# TASK-010: Sửa `dev-server-relay-bridge.ts` — thêm direct-websocket branch

> **Status:** ✅ DONE (2026-07-26)
> **TypeScript:** 0 errors

**Status:** ✅ DONE  
**Phase:** 4 — direct-websocket mode  
**Solution:** [SOL-AG-004](../solutions/SOL-AG-004-direct-websocket.md) §3.2  
**Depends on:** TASK-006, TASK-008  
**Blocks:** TASK-011  

---

## Mục tiêu

Sửa `DevServerRelayBridge` để:
1. Nhận `AgentWebSocketServer` qua constructor (optional, null = relay-ssh mode)
2. Thêm private method `connectDirectWebSocket()`
3. Thay placeholder throw bằng real `connectDirectWebSocket()` call
4. Emit event `agentTokenGenerated` để UI display token

---

## File cần sửa

**Path:** `src/main/dev-server/dev-server-relay-bridge.ts`

---

## Thay đổi cần thực hiện

### 1. Thêm imports

```typescript
import EventEmitter from 'node:events'
import type { AgentWebSocketServer } from './agent-ws-server'
import { generateAgentToken, AGENT_WS_PATH } from '../../shared/agent-wire-protocol'
```

### 2. Sửa class declaration — extend EventEmitter

```typescript
// Trước:
export class DevServerRelayBridge {

// Sau:
export class DevServerRelayBridge extends EventEmitter {
```

### 3. Sửa constructor — thêm agentWsServer param

```typescript
// Trước:
  constructor(
    private config: PersistedDevServer,
    private sshManager: SshConnectionManager
  ) {}

// Sau:
  constructor(
    private config: PersistedDevServer,
    private sshManager: SshConnectionManager,
    private agentWsServer: AgentWebSocketServer | null = null
  ) {
    super()
  }
```

### 4. Sửa placeholder direct-websocket trong connect()

**Tìm và xóa:**
```typescript
    // ─── Phase 2: direct-websocket ─────────────────────────────────────────────
    // (SOL-AG-004 / TASK-010 — placeholder until AgentWebSocketServer is wired)
    if (this.config.connectionType === 'direct-websocket') {
      throw new Error(
        `direct-websocket mode requires AgentWebSocketServer — not yet wired in this build (TASK-010)`
      )
    }
```

**Thay bằng:**
```typescript
    // ─── Phase 2: direct-websocket ─────────────────────────────────────────────
    if (this.config.connectionType === 'direct-websocket') {
      return this.connectDirectWebSocket(opts)
    }
```

### 5. Thêm private method `connectDirectWebSocket()` (sau `connectRelayWebSocket()`)

```typescript
  /**
   * direct-websocket mode: Orca is WS SERVER, agent will connect inbound.
   *
   * Flow:
   *   1. Generate unique agentToken
   *   2. Register slot in AgentWebSocketServer
   *   3. Emit 'agentTokenGenerated' event → UI displays token + command
   *   4. Wait for agent to connect and handshake (max 60s)
   *   5. On success: session = mux, resolve RelayHandshakeInfo
   *   6. On timeout: reject with instructions
   */
  private connectDirectWebSocket(opts: { testOnly?: boolean }): Promise<RelayHandshakeInfo> {
    if (!this.agentWsServer) {
      return Promise.reject(
        new Error(
          'direct-websocket mode requires AgentWebSocketServer to be initialized. ' +
          'Ensure server-bootstrap.ts creates and passes AgentWebSocketServer to DevServerManager.'
        )
      )
    }

    const agentToken = generateAgentToken(this.config.id)

    return new Promise<RelayHandshakeInfo>((resolve, reject) => {
      const disposer = this.agentWsServer!.registerSlot(
        agentToken,
        // onConnected: agent successfully connected and handshaked
        (mux, info) => {
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
        // onExpired: agent did not connect within 60s
        (reason) => {
          reject(new Error(reason))
        }
      )

      // Emit token to UI for user to configure agent
      // UI listens via DevServerManager event forwarding or RPC push
      this.emit('agentTokenGenerated', {
        devServerId: this.config.id,
        agentToken,
        orcaUrl: `ws://<orca-host>:6768${AGENT_WS_PATH}`,
      })

      // Store disposer so disconnect() can clean up the slot
      this._directWsDisposer = disposer
    })
  }

  /** Disposer for active direct-websocket slot — cleaned up on disconnect */
  private _directWsDisposer: (() => void) | null = null
```

### 6. Sửa `disconnect()` — cleanup direct-websocket slot

```typescript
  async disconnect(): Promise<void> {
    // NEW: cancel direct-websocket slot if pending
    if (this._directWsDisposer) {
      this._directWsDisposer()
      this._directWsDisposer = null
    }

    // existing disconnect logic (giữ nguyên):
    if (this.session && typeof this.session.close === 'function') {
      await this.session.close()
    } else if (this.session && typeof this.session.destroy === 'function') {
      this.session.destroy()
    }
    this.session = null
  }
```

---

## Acceptance Criteria

- [x] `DevServerRelayBridge` extends `EventEmitter`
- [x] Constructor nhận `agentWsServer: AgentWebSocketServer | null = null` (backward compat)
- [x] `connect()` với `direct-websocket` không throw error
- [x] `agentWsServer === null` → reject với error message hữu ích
- [x] `emit('agentTokenGenerated', { devServerId, agentToken, orcaUrl })` được emit
- [x] `agentToken` format: `agt-<devServerId>-<timestamp>`
- [x] `disconnect()` gọi disposer (cancel slot nếu còn pending)
- [x] `this.session` được set khi agent connect thành công
- [x] TypeScript compile không lỗi
