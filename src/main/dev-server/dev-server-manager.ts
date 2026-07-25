// ─── DevServerManager ────────────────────────────────────────────────────────
// Manages the full lifecycle of remote dev servers:
// CRUD (via DevServerStore), connection (via DevServerRelayBridge), and
// runtime state (in-memory only — not persisted).

import EventEmitter from 'node:events'
import type { Store } from '../persistence'
import type {
  DevServer,
  DevServerInput,
  DevServerStatus,
  ConnectionTestResult,
  PersistedDevServer
} from '../../shared/dev-server-types'
import { DevServerStore } from './dev-server-store'
import { DevServerRelayBridge } from './dev-server-relay-bridge'
import type { SshConnectionManager } from '../ssh/ssh-connection-manager'

// ─── Runtime-only state (NOT persisted) ─────────────────────────────────────
type RuntimeDevServerState = {
  status: DevServerStatus
  platform: NodeJS.Platform | null
  arch: string | null
  nodeVersion: string | null
  lastConnectedAt: number | null
  lastError: string | null
}

const DEFAULT_RUNTIME_STATE: RuntimeDevServerState = {
  status: 'disconnected',
  platform: null,
  arch: null,
  nodeVersion: null,
  lastConnectedAt: null,
  lastError: null
}

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
    // Restore runtime state for all persisted servers (status = disconnected)
    for (const ds of this.store.list()) {
      this.initRuntimeState(ds.id)
    }
  }

  // ── CRUD ──────────────────────────────────────────────────────────────────

  list(): DevServer[] {
    return this.store.list().map((ds) => this.mergeWithRuntime(ds))
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
    const persisted = this.store.list().find((ds) => ds.id === id)
    if (!persisted) return null
    return this.mergeWithRuntime(persisted)
  }

  // ── Connection ────────────────────────────────────────────────────────────

  async testConnection(input: DevServerInput): Promise<ConnectionTestResult> {
    // Ephemeral bridge — test only, do not persist or store
    const ephemeralPersisted: PersistedDevServer = {
      id: 'test-ephemeral',
      name: input.name,
      connectionType: input.connectionType,
      sshTargetId: input.sshTargetId,
      wsUrl: input.wsUrl,
      workspaceDir: null,
      addedAt: Date.now()
    }
    const bridge = new DevServerRelayBridge(ephemeralPersisted, this.sshManager)
    try {
      const info = await bridge.connect({ testOnly: true })
      return { ok: true, platform: info.platform, nodeVersion: info.nodeVersion }
    } catch (err) {
      return {
        ok: false,
        error: err instanceof Error ? err.message : String(err)
      }
    } finally {
      await bridge.disconnect()
    }
  }

  async connect(id: string): Promise<void> {
    const persisted = this.store.list().find((ds) => ds.id === id)
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

  // ── Relay access (for IPC handlers) ──────────────────────────────────────

  getRelay(id: string): DevServerRelayBridge | null {
    return this.relays.get(id) ?? null
  }

  // ── Private helpers ───────────────────────────────────────────────────────

  private initRuntimeState(id: string): void {
    this.runtimeState.set(id, { ...DEFAULT_RUNTIME_STATE })
  }

  private getRuntimeState(id: string): RuntimeDevServerState {
    return this.runtimeState.get(id) ?? { ...DEFAULT_RUNTIME_STATE }
  }

  private setRuntimeState(id: string, updates: Partial<RuntimeDevServerState>): void {
    this.runtimeState.set(id, { ...this.getRuntimeState(id), ...updates })
  }

  private mergeWithRuntime(persisted: PersistedDevServer): DevServer {
    return { ...persisted, ...this.getRuntimeState(persisted.id) }
  }
}
