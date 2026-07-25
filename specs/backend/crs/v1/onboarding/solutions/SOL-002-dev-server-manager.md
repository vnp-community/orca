# SOL-002: DevServerManager — Backend Solution

**CR:** [CR-OB-002](../../../../../docs/crs/v1/onboarding/CR-OB-002-dev-server-registration.md)  
**TDD refs:** TDD-05 (SSH Relay), TDD-06 (Persistence), TDD-09 (IPC Handlers)  
**Status:** ✅ Implemented | **Phase:** 1 (Foundation)

---

## 1. New Files

```
src/main/dev-server/
├── dev-server-manager.ts          ← Lifecycle: add, connect, disconnect, remove
├── dev-server-store.ts            ← CRUD trên PersistedState.devServers
├── dev-server-relay-bridge.ts     ← Wrap SshRelaySession → DevServerRelay interface
└── dev-server-preflight.ts        ← Test connection, platform probe

src/shared/
└── dev-server-types.ts            ← DevServer, DevServerInput, ConnectionTestResult
```

---

## 2. Schema — `src/shared/dev-server-types.ts`

```typescript
export type DevServerConnectionType =
  | 'relay-ssh'         // Orca SSH → deploy relay → stdin/stdout
  | 'relay-websocket'   // Dev server connects WS → Orca (reverse)
  | 'direct-websocket'  // Orca connects WS → dev server relay

export type DevServerStatus =
  | 'connected'
  | 'disconnected'
  | 'connecting'
  | 'error'

export type DevServer = {
  id: string                          // 'ds-<uuid>'
  name: string                        // Human label: "MacBook Pro M3"
  connectionType: DevServerConnectionType
  // relay-ssh specific:
  sshTargetId?: string                // Links to existing SshTarget
  // relay-websocket / direct-websocket specific:
  wsUrl?: string                      // ws://devserver.local:6799
  // Runtime (không persist):
  status: DevServerStatus
  platform: NodeJS.Platform | null    // Populated after handshake
  arch: string | null                 // 'arm64' | 'x64'
  nodeVersion: string | null
  lastConnectedAt: number | null
  lastError: string | null
  workspaceDir: string | null         // Remote default workspace directory
  addedAt: number
}

export type DevServerInput = {
  name: string
  connectionType: DevServerConnectionType
  sshTargetId?: string
  wsUrl?: string
}

export type ConnectionTestResult =
  | { ok: true; platform: NodeJS.Platform; nodeVersion: string }
  | { ok: false; error: string; hint?: string }
```

---

## 3. Persistence — `src/shared/types.ts` (MODIFY)

```typescript
// Thêm vào PersistedState:
type PersistedState = {
  // ... existing fields ...
  devServers: PersistedDevServer[]        // NEW
}

// Không persist status/platform (runtime-only):
type PersistedDevServer = {
  id: string
  name: string
  connectionType: DevServerConnectionType
  sshTargetId?: string
  wsUrl?: string
  workspaceDir: string | null
  addedAt: number
}

// Thêm vào GlobalSettings:
type GlobalSettings = {
  // ... existing ...
  activeDevServerId?: string | null      // NEW
}
```

---

## 4. Store CRUD — `src/main/dev-server/dev-server-store.ts`

```typescript
import type { Store } from '../persistence'
import type { DevServer, DevServerInput } from '../../shared/dev-server-types'
import { randomUUID } from 'node:crypto'

export class DevServerStore {
  constructor(private store: Store) {}

  list(): PersistedDevServer[] {
    return this.store.getState().devServers ?? []
  }

  add(input: DevServerInput): PersistedDevServer {
    const record: PersistedDevServer = {
      id: `ds-${randomUUID()}`,
      name: input.name,
      connectionType: input.connectionType,
      sshTargetId: input.sshTargetId,
      wsUrl: input.wsUrl,
      workspaceDir: null,
      addedAt: Date.now()
    }
    this.store.mutate(state => {
      state.devServers = [...(state.devServers ?? []), record]
    })
    return record
  }

  update(id: string, updates: Partial<PersistedDevServer>): void {
    this.store.mutate(state => {
      const idx = state.devServers.findIndex(ds => ds.id === id)
      if (idx >= 0) {
        state.devServers[idx] = { ...state.devServers[idx], ...updates }
      }
    })
  }

  remove(id: string): void {
    this.store.mutate(state => {
      state.devServers = state.devServers.filter(ds => ds.id !== id)
    })
  }
}
```

---

## 5. Manager — `src/main/dev-server/dev-server-manager.ts`

```typescript
import EventEmitter from 'node:events'
import type { DevServer, DevServerInput, ConnectionTestResult } from '../../shared/dev-server-types'
import { DevServerStore } from './dev-server-store'
import { DevServerRelayBridge } from './dev-server-relay-bridge'

export class DevServerManager extends EventEmitter {
  private store: DevServerStore
  // Runtime state (không persist):
  private runtimeState = new Map<string, Omit<DevServer, keyof PersistedDevServer>>()
  private relays = new Map<string, DevServerRelayBridge>()

  constructor(persistStore: Store) {
    super()
    this.store = new DevServerStore(persistStore)
  }

  // ── CRUD ──────────────────────────────────────────────

  list(): DevServer[] {
    return this.store.list().map(ds => ({
      ...ds,
      ...this.getRuntimeState(ds.id)
    }))
  }

  async add(input: DevServerInput): Promise<DevServer> {
    const persisted = this.store.add(input)
    this.setRuntimeState(persisted.id, {
      status: 'disconnected',
      platform: null,
      arch: null,
      nodeVersion: null,
      lastConnectedAt: null,
      lastError: null
    })
    this.emit('devServer:added', persisted.id)
    return this.get(persisted.id)!
  }

  async remove(id: string): Promise<void> {
    await this.disconnect(id)
    this.store.remove(id)
    this.runtimeState.delete(id)
    this.emit('devServer:removed', id)
  }

  get(id: string): DevServer | null {
    const persisted = this.store.list().find(ds => ds.id === id)
    if (!persisted) return null
    return { ...persisted, ...this.getRuntimeState(id) }
  }

  // ── Connection ────────────────────────────────────────

  async testConnection(input: DevServerInput): Promise<ConnectionTestResult> {
    // Tạo ephemeral relay, test handshake, đóng ngay
    const bridge = new DevServerRelayBridge(input, this.sshManager)
    try {
      const info = await bridge.connect({ testOnly: true })
      return { ok: true, platform: info.platform, nodeVersion: info.nodeVersion }
    } catch (err) {
      return { ok: false, error: err instanceof Error ? err.message : String(err) }
    } finally {
      await bridge.disconnect()
    }
  }

  async connect(id: string): Promise<void> {
    const persisted = this.store.list().find(ds => ds.id === id)
    if (!persisted) throw new Error(`DevServer not found: ${id}`)

    this.setRuntimeState(id, { status: 'connecting', lastError: null })
    this.emit('devServer:statusChanged', id, 'connecting')

    try {
      const bridge = new DevServerRelayBridge(persisted, this.sshManager)
      const info = await bridge.connect()

      this.relays.set(id, bridge)
      this.setRuntimeState(id, {
        status: 'connected',
        platform: info.platform,
        arch: info.arch,
        nodeVersion: info.nodeVersion,
        lastConnectedAt: Date.now()
      })
      this.emit('devServer:statusChanged', id, 'connected')
    } catch (err) {
      const errorMsg = err instanceof Error ? err.message : String(err)
      this.setRuntimeState(id, { status: 'error', lastError: errorMsg })
      this.emit('devServer:statusChanged', id, 'error')
      throw err
    }
  }

  async disconnect(id: string): Promise<void> {
    const relay = this.relays.get(id)
    if (relay) {
      await relay.disconnect()
      this.relays.delete(id)
    }
    this.setRuntimeState(id, { status: 'disconnected' })
    this.emit('devServer:statusChanged', id, 'disconnected')
  }

  // ── Relay access (for IPC handlers) ──────────────────

  getRelay(id: string): DevServerRelayBridge | null {
    return this.relays.get(id) ?? null
  }

  // ── Private helpers ───────────────────────────────────

  private getRuntimeState(id: string) {
    return this.runtimeState.get(id) ?? {
      status: 'disconnected' as const,
      platform: null, arch: null, nodeVersion: null,
      lastConnectedAt: null, lastError: null
    }
  }

  private setRuntimeState(id: string, updates: Partial<ReturnType<typeof this.getRuntimeState>>): void {
    this.runtimeState.set(id, { ...this.getRuntimeState(id), ...updates })
  }
}
```

---

## 6. Relay Bridge — `src/main/dev-server/dev-server-relay-bridge.ts`

```typescript
// Wrap existing SSH relay infrastructure (ssh-relay-deploy.ts / ssh-relay-session.ts)
// Cung cấp interface thuần túy cho DevServerManager

export type RelayHandshakeInfo = {
  platform: NodeJS.Platform
  arch: string
  nodeVersion: string
  relayVersion: string
}

export class DevServerRelayBridge {
  private session: SshRelaySession | null = null

  constructor(
    private config: PersistedDevServer,
    private sshManager: SshConnectionManager  // existing
  ) {}

  async connect(opts: { testOnly?: boolean } = {}): Promise<RelayHandshakeInfo> {
    // relay-ssh: dùng existing SshConnection + deployRelay()
    if (this.config.connectionType === 'relay-ssh') {
      const conn = await this.sshManager.getConnection(this.config.sshTargetId!)
      const result = await deployRelay(conn, { testOnly: opts.testOnly })
      this.session = result.session
      // Handshake info (platform) từ relay-handshake.ts:
      return {
        platform: result.platform,
        arch: result.arch ?? process.arch,
        nodeVersion: result.nodeVersion ?? 'unknown',
        relayVersion: result.relayVersion
      }
    }
    // relay-websocket / direct-websocket: TODO Phase 2
    throw new Error(`Connection type ${this.config.connectionType} not yet implemented`)
  }

  async disconnect(): Promise<void> {
    await this.session?.close()
    this.session = null
  }

  // Forward preflight calls to relay:
  async detectAgents(commands: AgentDetectionCommand[]): Promise<string[]> {
    if (!this.session) throw new Error('Not connected')
    const result = await this.session.call('preflight.detectAgents', { commands })
    return result.agents as string[]
  }

  async detectWindowsCapabilities(): Promise<WindowsTerminalCapabilities> {
    if (!this.session) throw new Error('Not connected')
    return this.session.call('preflight.detectWindowsTerminalCapabilities', {})
  }

  async detectPreflightStatus(): Promise<RemotePreflightStatus> {
    if (!this.session) throw new Error('Not connected')
    return this.session.call('preflight.check', {})
  }
}
```

---

## 7. Handshake Extension — `src/relay/relay-handshake.ts` (MODIFY)

```typescript
// Thêm platform info vào handshake frame:
export type DaemonHandshakeCallbacks = {
  onAccepted: (sock: Socket, leftover: Buffer, info: RelayHandshakeInfo) => void  // CHANGED
  launchVersion: string
}

// relay-handshake.ts — daemon gửi platform info:
const handshakeFrame = encodeHandshakeFrame({
  type: MessageType.Handshake,
  version: launchVersion,
  platform: process.platform,          // NEW
  arch: process.arch,                  // NEW
  nodeVersion: process.version         // NEW
})
```

---

## 8. IPC Handlers — `src/main/ipc/dev-server-ipc.ts` (NEW)

```typescript
// Pattern: theo TDD-09 IPC Handler convention
import type { DevServerManager } from '../dev-server/dev-server-manager'

export function registerDevServerIpcHandlers(
  ipc: IpcMain | WebIpcBridge,
  manager: DevServerManager
): void {
  // List
  ipc.handle('devServer.list', async () => manager.list())

  // Add
  ipc.handle('devServer.add', async (_, input: DevServerInput) => {
    return manager.add(input)
  })

  // Remove
  ipc.handle('devServer.remove', async (_, id: string) => {
    await manager.remove(id)
  })

  // Test connection
  ipc.handle('devServer.testConnection', async (_, input: DevServerInput) => {
    return manager.testConnection(input)
  })

  // Connect / Disconnect
  ipc.handle('devServer.connect', async (_, id: string) => {
    await manager.connect(id)
    return manager.get(id)
  })

  ipc.handle('devServer.disconnect', async (_, id: string) => {
    await manager.disconnect(id)
  })

  // Status push events:
  manager.on('devServer:statusChanged', (id: string, status: DevServerStatus) => {
    ipc.emit('devServer:statusChanged', { id, status })
  })
}
```

---

## 9. Persistence Migration

```typescript
// src/main/persistence.ts — thêm migration trong normalizeLoadedState():
function migrateDevServers(state: PersistedState): PersistedState {
  // v0 → v1: nếu chưa có devServers, không tạo gì (user phải add thủ công)
  if (!state.devServers) {
    state.devServers = []
  }
  return state
}
```

---

## 10. Tests

```typescript
// src/main/dev-server/__tests__/dev-server-manager.test.ts
describe('DevServerManager', () => {
  it('add() persists devServer với id và addedAt')
  it('add() set runtime status = disconnected')
  it('testConnection() relay-ssh success → return platform info')
  it('testConnection() relay-ssh failure → return { ok: false, error }')
  it('connect() relay-ssh → status: connecting → connected')
  it('connect() relay-ssh failure → status: error + lastError set')
  it('disconnect() → status: disconnected, relay closed')
  it('remove() → disconnect, remove from store')
  it('getRelay() trả về bridge khi connected, null khi không')
  it('list() merge persisted + runtime state')
  it('emit devServer:statusChanged khi connect/disconnect/error')
})

// src/main/dev-server/__tests__/dev-server-store.test.ts
describe('DevServerStore', () => {
  it('add() tạo id với prefix ds-')
  it('list() trả về rỗng khi chưa có')
  it('update() sửa field cụ thể')
  it('remove() xóa đúng record')
})
```

---

## 11. Checklist triển khai

- [x] Tạo `src/shared/dev-server-types.ts`
- [x] Thêm `devServers: PersistedDevServer[]` vào `PersistedState`
- [x] Thêm `activeDevServerId` vào `GlobalSettings`
- [x] Tạo `DevServerStore`
- [x] Tạo `DevServerManager`
- [x] Tạo `DevServerRelayBridge` (relay-ssh mode)
- [x] Extend relay handshake với `platform` + `arch` + `nodeVersion`
- [x] Tạo IPC handlers `devServer.*`
- [x] Đăng ký handlers trong `server-bootstrap.ts`
- [x] Persistence migration cho `devServers: []`
- [x] Unit tests: Manager, Store
