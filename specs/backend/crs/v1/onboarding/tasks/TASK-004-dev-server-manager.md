# TASK-004: Tạo file `src/main/dev-server/dev-server-manager.ts`

**Phase:** 1 — Foundation  
**Solution:** [SOL-002](../solutions/SOL-002-dev-server-manager.md) §5  
**Depends on:** TASK-001, TASK-003, TASK-005  
**Blocks:** TASK-007, TASK-013

---

## Mục tiêu

Tạo class `DevServerManager` quản lý lifecycle của DevServer: CRUD, connect, disconnect, relay access.

---

## File cần tạo

**Path:** `src/main/dev-server/dev-server-manager.ts`

---

## Nội dung cần implement

```typescript
import EventEmitter from 'node:events'
import type { Store } from '../persistence'
import type { DevServer, DevServerInput, DevServerStatus, ConnectionTestResult, PersistedDevServer } from '../../shared/dev-server-types'
import { DevServerStore } from './dev-server-store'
import { DevServerRelayBridge } from './dev-server-relay-bridge'
import type { SshConnectionManager } from '../ssh/ssh-connection-manager'  // existing

export class DevServerManager extends EventEmitter {
  private store: DevServerStore
  private runtimeState = new Map<string, RuntimeDevServerState>()
  private relays = new Map<string, DevServerRelayBridge>()

  constructor(
    persistStore: Store,
    private sshManager: SshConnectionManager
  ) {
    super()
    this.store = new DevServerStore(persistStore)
    // Restore runtime state cho các persisted servers
    for (const ds of this.store.list()) {
      this.initRuntimeState(ds.id)
    }
  }

  // ── CRUD ──────────────────────────────────────────────

  list(): DevServer[] {
    return this.store.list().map(ds => this.mergeWithRuntime(ds))
  }

  async add(input: DevServerInput): Promise<DevServer> {
    const persisted = this.store.add(input)
    this.initRuntimeState(persisted.id)
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
    return this.mergeWithRuntime(persisted)
  }

  // ── Connection ────────────────────────────────────────

  async testConnection(input: DevServerInput): Promise<ConnectionTestResult> {
    const bridge = new DevServerRelayBridge(
      { id: 'test', ...input, workspaceDir: null, addedAt: 0 } as PersistedDevServer,
      this.sshManager
    )
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
        lastConnectedAt: Date.now(),
        lastError: null
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

  private initRuntimeState(id: string): void {
    this.runtimeState.set(id, {
      status: 'disconnected',
      platform: null,
      arch: null,
      nodeVersion: null,
      lastConnectedAt: null,
      lastError: null
    })
  }

  private getRuntimeState(id: string): RuntimeDevServerState {
    return this.runtimeState.get(id) ?? {
      status: 'disconnected',
      platform: null,
      arch: null,
      nodeVersion: null,
      lastConnectedAt: null,
      lastError: null
    }
  }

  private setRuntimeState(id: string, updates: Partial<RuntimeDevServerState>): void {
    this.runtimeState.set(id, { ...this.getRuntimeState(id), ...updates })
  }

  private mergeWithRuntime(persisted: PersistedDevServer): DevServer {
    return { ...persisted, ...this.getRuntimeState(persisted.id) }
  }
}

type RuntimeDevServerState = {
  status: DevServerStatus
  platform: NodeJS.Platform | null
  arch: string | null
  nodeVersion: string | null
  lastConnectedAt: number | null
  lastError: string | null
}
```

---

## Acceptance Criteria

- [x] File tồn tại tại `src/main/dev-server/dev-server-manager.ts`
- [x] `DevServerManager` extends `EventEmitter`
- [x] `add()` persists và set runtime `status: 'disconnected'`
- [x] `add()` emit `'devServer:added'`
- [x] `connect()` transitions: `connecting` → `connected` (hoặc `error`)
- [x] `connect()` emit `'devServer:statusChanged'` ở mỗi transition
- [x] `disconnect()` đóng relay, set status `disconnected`, emit event
- [x] `remove()` gọi `disconnect()` trước khi xóa
- [x] `getRelay()` trả về bridge khi connected, `null` khi không
- [x] `list()` merge persisted + runtime state cho từng server
- [x] Constructor restore runtime state với status = `disconnected` cho tất cả persisted servers
- [x] TypeScript compile thành công
