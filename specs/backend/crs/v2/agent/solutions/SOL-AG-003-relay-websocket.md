# SOL-AG-003 — relay-websocket: Orca → Agent WS Server

**CR:** [CR-AG-003](../../../../../docs/crs/v2/agent/CR-AG-003-relay-websocket-mode.md)  
**TDD Refs:** TDD-05 §4 (Relay), TDD-13 §3 (Dev Server Manager)  
**Depends on:** SOL-AG-001, SOL-AG-002  
**Approach:** Test-Driven  
**Status:** ✅ IMPLEMENTED (2026-07-26)  
**Tasks:** [TASK-006](../tasks/TASK-006-relay-bridge-relay-websocket.md), [TASK-007](../tasks/TASK-007-dev-server-manager-test-connection.md)  
**Files:** `dev-server-relay-bridge.ts` (relay-websocket branch), `dev-server-manager.ts` (WS fast path)  
**Tests:** 19/19 pass | **TypeScript:** 0 errors  

---

## 1. Phân tích từ Code Hiện tại

### 1.1 DevServerRelayBridge — Phase 2 gap

```typescript
// src/main/dev-server/dev-server-relay-bridge.ts:85-89 (HIỆN TẠI)
throw new Error(
  `Connection type '${this.config.connectionType}' is not yet implemented. ` +
  `Only 'relay-ssh' is supported in Phase 1.`
)
```

Cần thêm branch `relay-websocket` trước throw này.

### 1.2 `ws` package availability

Cần check `package.json` xem `ws` đã có:
```bash
grep '"ws"' package.json
```
Nếu chưa có: `pnpm add ws && pnpm add -D @types/ws`

### 1.3 `app.getVersion()` trong Server mode

`DevServerRelayBridge` hiện không nhận `orcaVersion`. Cần truyền vào qua constructor hoặc dùng `getPlatform().app.getVersion()`.

### 1.4 `PersistedDevServer.wsUrl` — đã có

```typescript
// src/shared/dev-server-types.ts — ĐÃ CÓ
wsUrl?: string  // ws://host:port/orca-relay?token=<secret>
```

---

## 2. File Structure

```
src/main/dev-server/
└── dev-server-relay-bridge.ts  ← [MODIFY] Thêm connectRelayWebSocket() + branch relay-websocket

src/main/dev-server/__tests__/
└── relay-websocket.test.ts     ← [NEW] Integration-style unit tests
```

---

## 3. Implementation

### 3.1 Sửa `DevServerRelayBridge`

**Thêm import và method `connectRelayWebSocket()`:**

```typescript
// src/main/dev-server/dev-server-relay-bridge.ts
// [ADD imports]
import WebSocket from 'ws'
import { SshChannelMultiplexer } from '../ssh/ssh-channel-multiplexer'
import { createWebSocketTransport } from './ws-transport'
import { runOrcaInitiatorHandshake } from './ws-handshake'
import { getPlatform } from '../../platform'

// [MODIFY] connect() — thêm branch relay-websocket TRƯỚC throw
async connect(opts: { testOnly?: boolean } = {}): Promise<RelayHandshakeInfo> {
  if (this.config.connectionType === 'relay-ssh') {
    // ... existing SSH logic (unchanged)
  }

  // ─── Phase 2: relay-websocket ────────────────────────────────────────────
  if (this.config.connectionType === 'relay-websocket') {
    const wsUrl = this.config.wsUrl
    if (!wsUrl) {
      throw new Error(`DevServer '${this.config.name}' has no wsUrl configured for relay-websocket mode`)
    }
    return this.connectRelayWebSocket(wsUrl, opts)
  }

  // ─── Phase 2: direct-websocket (SOL-AG-004) ──────────────────────────────
  if (this.config.connectionType === 'direct-websocket') {
    return this.connectDirectWebSocket(opts)
  }

  throw new Error(
    `Connection type '${this.config.connectionType}' is not supported.`
  )
}

// [ADD] Private method
private async connectRelayWebSocket(
  rawUrl: string,
  opts: { testOnly?: boolean }
): Promise<RelayHandshakeInfo> {
  // Parse token from URL query param: ws://host:port/path?token=<secret>
  // Why: URL is cleaner for UI input than separate fields,
  // token is stripped before creating WS to avoid it appearing in headers
  const url = new URL(rawUrl)
  const token = url.searchParams.get('token') ?? ''
  url.searchParams.delete('token')
  const cleanUrl = url.toString()

  const orcaVersion = getPlatform().app.getVersion()

  return new Promise<RelayHandshakeInfo>((resolve, reject) => {
    // Why: ws constructor does not throw — errors come via 'error' event
    const ws = new WebSocket(cleanUrl, {
      headers: token ? { Authorization: `Bearer ${token}` } : {},
    })

    // Ensure binary messages are delivered as Buffer (not string/ArrayBuffer)
    ;(ws as { binaryType?: string }).binaryType = 'nodebuffer'

    const connectionTimeout = setTimeout(() => {
      ws.close()
      reject(new Error(
        `relay-websocket: TCP connection timed out after 10s to ${cleanUrl}. ` +
        `Check agent is running and reachable.`
      ))
    }, 10_000)

    ws.on('error', (err) => {
      clearTimeout(connectionTimeout)
      reject(new Error(`relay-websocket: WebSocket error: ${err.message}`))
    })

    ws.on('open', () => {
      clearTimeout(connectionTimeout)

      // Run handshake then wire multiplexer
      runOrcaInitiatorHandshake(ws, orcaVersion)
        .then((info) => {
          // Handshake done — now wire multiplexer
          const transport = createWebSocketTransport(ws)
          this.session = new SshChannelMultiplexer(transport)

          if (opts.testOnly) {
            // Don't keep the connection — just verify connectivity
            void this.disconnect()
          }

          resolve({
            platform: (info.platform as NodeJS.Platform) ?? 'linux',
            arch: info.arch,
            nodeVersion: info.nodeVersion,
            relayVersion: info.agentVersion,
          })
        })
        .catch((err) => {
          ws.close()
          reject(err)
        })
    })
  })
}
```

### 3.2 Sửa `DevServerManager.testConnection()` — relay-websocket path

`testConnection()` hiện chỉ handle relay-ssh (cần SSH connect trước). Với relay-websocket KHÔNG cần SSH:

```typescript
// src/main/dev-server/dev-server-manager.ts
async testConnection(input: DevServerInput): Promise<ConnectionTestResult> {
  const { connectionType } = input

  // relay-ssh: cần SSH connect trước (logic hiện tại)
  if (connectionType === 'relay-ssh') {
    // ... existing SSH connect logic (không đổi)
  }

  // relay-websocket: không cần SSH — connect trực tiếp
  // direct-websocket: không cần setup gì — bridge sẽ wait for agent
  // → Fall through to ephemeral bridge test

  const ephemeralPersisted: PersistedDevServer = {
    id: 'test-ephemeral',
    name: input.name,
    connectionType: input.connectionType,
    sshTargetId: input.sshTargetId,
    wsUrl: input.wsUrl,
    workspaceDir: null,
    addedAt: Date.now(),
  }
  const bridge = new DevServerRelayBridge(ephemeralPersisted, this.sshManager)
  try {
    const info = await bridge.connect({ testOnly: true })
    return { ok: true, platform: info.platform, nodeVersion: info.nodeVersion }
  } catch (err) {
    return { ok: false, error: err instanceof Error ? err.message : String(err) }
  } finally {
    await bridge.disconnect()
  }
}
```

---

## 4. wsUrl Format Spec

```
ws://host:port/orca-relay?token=<secret>
wss://host:port/orca-relay?token=<secret>   (TLS)
```

- **Host**: IP hoặc hostname của agent machine
- **Port**: default `6799`
- **Path**: `/orca-relay` (convention, agent can use any path)
- **Token**: Bearer token khớp với agent config

**Example:**
```
ws://172.20.2.31:6799/orca-relay?token=my-secret-token
```

---

## 5. Test Specifications

```typescript
// src/main/dev-server/__tests__/relay-websocket.test.ts
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { DevServerRelayBridge } from '../dev-server-relay-bridge'
import type { PersistedDevServer } from '../../../shared/dev-server-types'
import type { SshConnectionManager } from '../../ssh/ssh-connection-manager'

// Mock ws module
vi.mock('ws', () => {
  // ... mock WebSocket class
})

// Mock getPlatform
vi.mock('../../../platform', () => ({
  getPlatform: () => ({ app: { getVersion: () => '1.4.0' } }),
}))

const basePersisted: PersistedDevServer = {
  id: 'ds-test',
  name: 'Test WS Agent',
  connectionType: 'relay-websocket',
  wsUrl: 'ws://localhost:6799/orca-relay?token=test-token',
  sshTargetId: undefined,
  workspaceDir: null,
  addedAt: Date.now(),
}

const mockSshManager = {} as SshConnectionManager

describe('DevServerRelayBridge — relay-websocket', () => {
  it('throws if wsUrl is not set', async () => {
    const bridge = new DevServerRelayBridge(
      { ...basePersisted, wsUrl: undefined },
      mockSshManager
    )
    await expect(bridge.connect()).rejects.toThrow('no wsUrl configured')
  })

  it('strips token from URL before connecting', async () => {
    // Verify ws() is called with URL WITHOUT ?token=... in the URL
    // but WITH Authorization header set
    // (mock ws, verify call args)
  })

  it('resolves with platform/arch from agent handshake', async () => {
    // Mock successful WS open + handshake response
    // verify resolve({ platform: 'linux', arch: 'arm64', ... })
  })

  it('rejects on WS error before open', async () => {
    // Mock ws emitting 'error' before 'open'
  })

  it('rejects on connection timeout (10s)', async () => {
    // Mock ws never opening, advance timers by 11s
  })

  it('testOnly=true: disconnects after handshake', async () => {
    // verify session is null after testOnly connect
  })
})

describe('DevServerManager.testConnection — relay-websocket', () => {
  it('returns ok:true on successful relay-websocket test', async () => {
    // Mock bridge.connect resolving
  })

  it('does NOT call connectRegisteredSshTarget for relay-websocket', async () => {
    // verify SSH connect is never called
  })
})
```

---

## 6. Acceptance Criteria

- [ ] `DevServerRelayBridge.connect()` không còn throw error cho `relay-websocket`
- [ ] Token được strip khỏi URL trước khi tạo WebSocket
- [ ] Token được pass qua `Authorization: Bearer <token>` header
- [ ] TCP connection timeout sau 10s với error message hữu ích
- [ ] Handshake timeout sau 20s (từ `runOrcaInitiatorHandshake`)
- [ ] `testConnection()` với `relay-websocket` không call `connectRegisteredSshTarget`
- [ ] `DevServerRelayBridge.session` là `SshChannelMultiplexer` sau connect thành công
- [ ] `disconnect()` close ws gracefully
- [ ] Unit tests pass
