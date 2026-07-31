# SOL-AG-004 — direct-websocket: Agent → Orca WS Server

**CR:** [CR-AG-004](../../../../../docs/crs/v2/agent/CR-AG-004-direct-websocket-mode.md)  
**TDD Refs:** TDD-11 §2 (Web Server Mode + HTTP Server), TDD-13 §3 (Dev Server)  
**Depends on:** SOL-AG-001, SOL-AG-002  
**Approach:** Test-Driven  
**Status:** ✅ IMPLEMENTED (2026-07-26)  
**Tasks:** [TASK-008](../tasks/TASK-008-agent-ws-server.md) ~ [TASK-012](../tasks/TASK-012-server-index-attach-agent-ws.md)  
**Files:** `agent-ws-server.ts`, `dev-server-relay-bridge.ts` (direct-ws), `server-bootstrap.ts`, `server/index.ts`  
**Tests:** 11/11 pass | **TypeScript:** 0 errors  

---

## 1. Phân tích từ Code Hiện tại

### 1.1 HTTP Server hiện tại (src/server/index.ts)

```typescript
// server đang chạy Express + express-ws trên port 6768
// Route: wss://:6768/ → OrcaRuntimeRpcServer (browser clients)
// Chưa có: wss://:6768/agent → AgentWebSocketServer
```

HTTP server object (`httpServer`) được tạo trong `startHttpServer()`. Cần expose nó để `AgentWebSocketServer` có thể attach.

### 1.2 ServerBootstrapResult

```typescript
// src/main/server-bootstrap.ts
export interface ServerBootstrapResult {
  devServerManager: DevServerManager
  dbMonitor: DatabaseHealthMonitor
  pushManager: WebPushManager
  authManager: AuthManager
  sessionManager: SessionManager
  shutdown(): Promise<void>
}
```

Cần return `agentWsServer` trong result để `src/server/index.ts` có thể call `agentWsServer.attach(httpServer)`.

### 1.3 DevServerManager — cần inject AgentWebSocketServer

`DevServerManager` tạo `DevServerRelayBridge` cho mỗi DevServer. Cần pass `AgentWebSocketServer` vào constructor.

### 1.4 Token lifecycle cho direct-websocket

- Orca generate `agentToken` khi user click Connect
- Token được store in-memory trong `AgentWebSocketServer.pendingSlots`
- Khi agent connect → slot consumed → callback fires → bridge.session set
- Khi agent disconnect → Orca phải re-register slot (cho reconnect)
- Timeout: nếu agent không connect trong 60s → slot removed → error

---

## 2. File Structure

```
src/main/dev-server/
├── agent-ws-server.ts              ← [NEW] AgentWebSocketServer
├── dev-server-relay-bridge.ts      ← [MODIFY] direct-websocket branch + agentWsServer inject
└── dev-server-manager.ts           ← [MODIFY] inject AgentWebSocketServer

src/main/
└── server-bootstrap.ts             ← [MODIFY] init + attach AgentWebSocketServer

src/main/dev-server/__tests__/
└── agent-ws-server.test.ts         ← [NEW] Unit tests
```

---

## 3. Implementation

### 3.1 `src/main/dev-server/agent-ws-server.ts`

```typescript
// src/main/dev-server/agent-ws-server.ts
// WebSocket server for accepting incoming agent connections (direct-websocket mode).
//
// Architecture:
//   Browser/Orca app → ws://:6768/         (existing OrcaRuntimeRpcServer)
//   Agent           → ws://:6768/agent      (NEW — this file)
//
// The HTTP server upgrade event is shared between both WS servers.
// AgentWebSocketServer intercepts only requests to path '/agent'.
//
// Does NOT import from 'electron' — works in Node.js server mode.

import { WebSocketServer, WebSocket } from 'ws'
import type { IncomingMessage } from 'node:http'
import type { Server as HttpServer } from 'node:http'
import { SshChannelMultiplexer } from '../ssh/ssh-channel-multiplexer'
import { createWebSocketTransport } from './ws-transport'
import { runOrcaReceiverHandshake } from './ws-handshake'
import type { WsHandshakeInfo } from './ws-handshake'
import { AGENT_WS_PATH, AGENT_CONNECT_TIMEOUT_MS } from '../../shared/agent-wire-protocol'

export type AgentConnectedInfo = WsHandshakeInfo

export type AgentConnectionCallback = (
  mux: SshChannelMultiplexer,
  info: AgentConnectedInfo
) => void

type PendingSlot = {
  callback: AgentConnectionCallback
  expireTimer: ReturnType<typeof setTimeout>
  onExpired: (reason: string) => void
}

export class AgentWebSocketServer {
  private wss: WebSocketServer | null = null
  // Map<agentToken, PendingSlot>
  private pendingSlots = new Map<string, PendingSlot>()
  private orcaVersion: string

  constructor(orcaVersion: string) {
    this.orcaVersion = orcaVersion
  }

  /**
   * Attach to an existing HTTP server, intercepting WS upgrades on AGENT_WS_PATH.
   * Call once during server startup (server-bootstrap.ts).
   */
  attach(httpServer: HttpServer): void {
    this.wss = new WebSocketServer({ noServer: true })

    httpServer.on('upgrade', (req: IncomingMessage, socket, head) => {
      const url = new URL(req.url ?? '/', `http://${req.headers.host ?? 'localhost'}`)
      if (url.pathname !== AGENT_WS_PATH) return  // not for us — let other handlers process

      this.wss!.handleUpgrade(req, socket, head, (ws: WebSocket) => {
        this.handleConnection(ws)
      })
    })
  }

  /**
   * Register a slot for a specific agent token.
   * When an agent connecting with this token passes handshake, callback fires.
   *
   * @returns disposer — call to remove slot (e.g. when DevServer is removed)
   */
  registerSlot(
    agentToken: string,
    onConnected: AgentConnectionCallback,
    onExpired: (reason: string) => void
  ): () => void {
    // Clear any existing slot for same token (e.g. re-register on reconnect)
    this.removeSlot(agentToken)

    const expireTimer = setTimeout(() => {
      this.removeSlot(agentToken)
      onExpired(
        `direct-websocket: Agent did not connect within ${AGENT_CONNECT_TIMEOUT_MS / 1000}s. ` +
        `Configure agent with ORCA_URL=ws://<orca-host>:6768${AGENT_WS_PATH} and AGENT_TOKEN=${agentToken}`
      )
    }, AGENT_CONNECT_TIMEOUT_MS)

    this.pendingSlots.set(agentToken, { callback: onConnected, expireTimer, onExpired })

    return () => this.removeSlot(agentToken)
  }

  private removeSlot(agentToken: string): void {
    const slot = this.pendingSlots.get(agentToken)
    if (slot) {
      clearTimeout(slot.expireTimer)
      this.pendingSlots.delete(agentToken)
    }
  }

  private handleConnection(ws: WebSocket): void {
    // Why: runOrcaReceiverHandshake validates agentToken embedded in handshake params.
    // We pass a validator that checks our slot map — this lets us reject unknown tokens early.
    runOrcaReceiverHandshake(
      ws,
      (token) => this.pendingSlots.has(token),
      this.orcaVersion
    )
      .then((info) => {
        const token = info.agentToken ?? ''
        const slot = this.pendingSlots.get(token)
        if (!slot) {
          // Race condition: slot expired between validate and consume
          ws.close(1008, 'Slot expired — agent token no longer valid')
          return
        }

        // Consume slot (agent is now connected)
        this.removeSlot(token)

        const transport = createWebSocketTransport(ws)
        const mux = new SshChannelMultiplexer(transport)
        slot.callback(mux, info)
      })
      .catch((err: Error) => {
        console.warn('[AgentWsServer] Handshake failed:', err.message)
        // ws already closed by runOrcaReceiverHandshake on failure
      })
  }

  stop(): void {
    // Clear all pending slots and timers
    for (const [token] of this.pendingSlots) {
      this.removeSlot(token)
    }
    this.wss?.close()
    this.wss = null
  }
}
```

### 3.2 Sửa `DevServerRelayBridge` — direct-websocket branch

```typescript
// src/main/dev-server/dev-server-relay-bridge.ts

import type { AgentWebSocketServer } from './agent-ws-server'
import { generateAgentToken } from '../../shared/agent-wire-protocol'
import EventEmitter from 'node:events'  // Thêm extends EventEmitter nếu chưa

// [MODIFY constructor]
constructor(
  private config: PersistedDevServer,
  private sshManager: SshConnectionManager,
  private agentWsServer: AgentWebSocketServer | null = null  // null trong relay-ssh mode
) {}

// [ADD event type — cho UI hiển thị token]
// 'agentTokenGenerated': { devServerId, agentToken, orcaUrl }

// [ADD private method]
private connectDirectWebSocket(opts: { testOnly?: boolean }): Promise<RelayHandshakeInfo> {
  if (!this.agentWsServer) {
    return Promise.reject(
      new Error('AgentWebSocketServer not available — cannot use direct-websocket mode in this context')
    )
  }

  const agentToken = generateAgentToken(this.config.id)

  return new Promise<RelayHandshakeInfo>((resolve, reject) => {
    const disposer = this.agentWsServer!.registerSlot(
      agentToken,
      // onConnected
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
      // onExpired
      (reason) => {
        reject(new Error(reason))
      }
    )

    // Emit token to UI so user can configure agent
    this.emit('agentTokenGenerated', {
      devServerId: this.config.id,
      agentToken,
      orcaUrl: `ws://<orca-host>:6768${AGENT_WS_PATH}`,
    })
  })
}

// [MODIFY connect() — thêm direct-websocket branch]
// (thêm SAU relay-websocket branch, TRƯỚC throw)
if (this.config.connectionType === 'direct-websocket') {
  return this.connectDirectWebSocket(opts)
}
```

### 3.3 Sửa `DevServerManager` — inject AgentWebSocketServer

```typescript
// src/main/dev-server/dev-server-manager.ts

import type { AgentWebSocketServer } from './agent-ws-server'

export class DevServerManager extends EventEmitter {
  constructor(
    persistStore: Store,
    private sshManager: SshConnectionManager,
    private agentWsServer: AgentWebSocketServer | null = null  // nullable — không break existing
  ) {
    // ...
  }

  // [MODIFY] createBridge() hoặc bất kỳ chỗ nào tạo DevServerRelayBridge
  private createBridge(ds: PersistedDevServer): DevServerRelayBridge {
    return new DevServerRelayBridge(ds, this.sshManager, this.agentWsServer)
  }
}
```

### 3.4 Sửa `server-bootstrap.ts` — init + attach AgentWebSocketServer

```typescript
// src/main/server-bootstrap.ts

import { AgentWebSocketServer } from './dev-server/agent-ws-server'

// Trong initializeOrcaServices() — thêm SAU RPC server init:
const platform = getPlatform()
const agentWsServer = new AgentWebSocketServer(platform.app.getVersion())

// Pass vào DevServerManager
const devServerManager = new DevServerManager(store, sshConnectionManager, agentWsServer)

// Return trong result để server/index.ts có thể attach
return {
  devServerManager,
  agentWsServer,          // ← NEW
  // ... existing fields
  async shutdown() {
    agentWsServer.stop()  // ← Cleanup on shutdown
    // ... existing shutdown logic
  }
}

// Trong src/server/index.ts — sau startHttpServer():
const { agentWsServer } = await initializeOrcaServices(...)
agentWsServer.attach(httpServer)
console.log(`[Orca Server] Agent WS: ws://0.0.0.0:${rpcPort}${AGENT_WS_PATH}`)
```

---

## 4. Token Display trong UI

Khi user chọn `direct-websocket` và click Connect, `DevServerRelayBridge` emit `agentTokenGenerated`.
`DevServerManager` forward event qua RPC push hoặc polling → UI display:

```
Connect your agent to this Orca instance:

  ORCA_URL=ws://b15.openledger.vn:6768/agent \
  AGENT_TOKEN=agt-ds-abc-1722033600 \
  node your-agent.js

Waiting for agent to connect... (expires in 60s)
```

UI component: `DevServerPane.tsx` — thêm state `agentToken: string | null` + countdown timer.

---

## 5. Test Specifications

```typescript
// src/main/dev-server/__tests__/agent-ws-server.test.ts
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { AgentWebSocketServer } from '../agent-ws-server'
import type { SshChannelMultiplexer } from '../../ssh/ssh-channel-multiplexer'

// Mock ws module
vi.mock('ws')
// Mock ws-handshake
vi.mock('../ws-handshake')
// Mock ws-transport
vi.mock('../ws-transport')

describe('AgentWebSocketServer', () => {
  let server: AgentWebSocketServer

  beforeEach(() => {
    server = new AgentWebSocketServer('1.4.0')
  })

  afterEach(() => {
    server.stop()
  })

  it('registerSlot() stores slot and returns disposer', () => {
    const onConnected = vi.fn()
    const onExpired = vi.fn()
    const disposer = server.registerSlot('test-token', onConnected, onExpired)
    expect(typeof disposer).toBe('function')
    // slot is registered — verify via handleConnection mock
  })

  it('disposer removes slot before timeout fires', () => {
    vi.useFakeTimers()
    const onExpired = vi.fn()
    const disposer = server.registerSlot('tok', vi.fn(), onExpired)
    disposer()
    vi.advanceTimersByTime(70_000)
    expect(onExpired).not.toHaveBeenCalled()
    vi.useRealTimers()
  })

  it('slot expires after AGENT_CONNECT_TIMEOUT_MS with descriptive error', () => {
    vi.useFakeTimers()
    const onExpired = vi.fn()
    server.registerSlot('expiring-token', vi.fn(), onExpired)
    vi.advanceTimersByTime(61_000)
    expect(onExpired).toHaveBeenCalledWith(expect.stringContaining('did not connect'))
    vi.useRealTimers()
  })

  it('handleConnection() calls callback with mux on valid handshake', async () => {
    // Mock runOrcaReceiverHandshake to resolve with valid info
    // Mock createWebSocketTransport
    // Mock SshChannelMultiplexer constructor
    // Verify callback is called with mux
  })

  it('handleConnection() closes ws on failed handshake', async () => {
    // Mock runOrcaReceiverHandshake to reject
    // Verify no callback called, no slot consumed
  })

  it('re-registering same token cancels previous slot', () => {
    vi.useFakeTimers()
    const expired1 = vi.fn()
    const expired2 = vi.fn()
    server.registerSlot('same-token', vi.fn(), expired1)
    server.registerSlot('same-token', vi.fn(), expired2)  // replaces first
    vi.advanceTimersByTime(70_000)
    expect(expired1).not.toHaveBeenCalled()  // first timer cancelled
    expect(expired2).toHaveBeenCalled()       // second timer fires
    vi.useRealTimers()
  })

  it('stop() clears all slots and closes wss', () => {
    const onExpired = vi.fn()
    server.registerSlot('tok1', vi.fn(), onExpired)
    server.registerSlot('tok2', vi.fn(), onExpired)
    server.stop()
    // verify no timers fire after stop
  })
})

describe('DevServerRelayBridge — direct-websocket', () => {
  it('rejects immediately if agentWsServer is null', async () => {
    // bridge with null agentWsServer
    // expect reject with 'not available'
  })

  it('generates token and registers slot on connect()', async () => {
    // mock agentWsServer.registerSlot
    // verify called with agt-<id>-<ts> format token
  })

  it('rejects when slot expires (onExpired called)', async () => {
    // mock registerSlot to call onExpired immediately
    // verify promise rejects with expiry message
  })

  it('resolves when agent connects (onConnected called)', async () => {
    // mock registerSlot to call onConnected immediately
    // verify resolve with platform/arch
  })

  it('emits agentTokenGenerated event with token and orcaUrl', async () => {
    // listen for event on bridge
    // trigger connect()
    // verify event emitted before resolution
  })
})
```

---

## 6. Acceptance Criteria

- [ ] `AgentWebSocketServer.attach()` intercepts upgrades trên path `/agent` không ảnh hưởng `/` (browser RPC)
- [ ] Token timeout 60s: `onExpired` callback fired với error message hữu ích
- [ ] `registerSlot()` disposer cancel timer trước khi expire
- [ ] Re-register cùng token: slot cũ bị replace, timer cũ bị cancel
- [ ] Handshake fail: ws closed, callback KHÔNG gọi
- [ ] Slot consumed sau lần connect đầu (agent phải re-handshake để reconnect)
- [ ] `server-bootstrap.ts`: `agentWsServer.stop()` được gọi trong `shutdown()`
- [ ] Log line: `[Orca Server] Agent WS: ws://0.0.0.0:<port>/agent`
- [ ] `DevServerRelayBridge` emit `agentTokenGenerated` event
- [ ] `DevServerRelayBridge.session` là `SshChannelMultiplexer` sau agent connect
- [ ] Unit tests pass
