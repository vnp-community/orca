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
import type { AgentWebSocketServer } from './agent-ws-server'
import type { AgentTokenInfo } from '../../shared/dev-server-types'
import {
  connectRegisteredSshTarget,
  disconnectRegisteredSshTarget
} from '../ipc/ssh'
import { generateAgentToken } from '../../shared/agent-wire-protocol'
import { createTracer } from '../../shared/trace'

const mgr = createTracer('devServer:manager')


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
    private sshManager: SshConnectionManager,
    private agentWsServer: AgentWebSocketServer | null = null
  ) {
    super()
    this.store = new DevServerStore(persistStore)
    // Restore runtime state for all persisted servers.
    // direct-websocket: agent tự reconnect via systemd → show 'connecting' not 'disconnected'.
    // relay-ssh, relay-websocket: no auto-reconnect → leave as 'disconnected' (default).
    for (const ds of this.store.list()) {
      this.initRuntimeState(ds.id)
      if (ds.connectionType === 'direct-websocket') {
        this.setRuntimeState(ds.id, { status: 'connecting', lastError: null })
      }
    }
  }

  /**
   * Emit 'devServer:statusChanged' → 'connecting' for all persisted direct-websocket
   * servers after server startup. Call once from server bootstrap AFTER the HTTP
   * server is listening so WebSocket UI clients receive the event.
   *
   * Background: DevServerManager runtime state is in-memory and lost on restart.
   * direct-websocket agents reconnect via systemd (exit → start.sh → fresh token).
   * This signals the UI to show "Connecting..." instead of "Disconnected" while
   * the agent re-establishes its connection (TASK-DS-007 / BUG-DS-004).
   */
  restoreConnections(): void {
    for (const ds of this.store.list()) {
      if (ds.connectionType === 'direct-websocket') {
        this.emit('devServer:statusChanged', ds.id, 'connecting')
        console.log(
          `[DevServerManager] Startup restore: ${ds.id} (${ds.name}) → 'connecting' ` +
          `(daemon agent will reconnect via systemd)`
        )
      }
    }
  }

  // ── CRUD ──────────────────────────────────────────────────────────────────

  list(): DevServer[] {
    return this.store.list().map((ds) => this.mergeWithRuntime(ds))
  }

  async add(input: DevServerInput): Promise<DevServer> {
    const span = mgr.start({ op: 'add', name: input.name, type: input.connectionType })
    const persisted = this.store.add(input)
    this.initRuntimeState(persisted.id)
    this.emit('devServer:added', persisted.id)
    span.ok({ id: persisted.id })
    return this.get(persisted.id)!
  }

  async remove(id: string): Promise<void> {
    const span = mgr.start({ op: 'remove', devServerId: id })
    await this.disconnect(id)
    this.store.remove(id)
    this.runtimeState.delete(id)
    this.emit('devServer:removed', id)
    span.ok()
  }

  get(id: string): DevServer | null {
    const persisted = this.store.list().find((ds) => ds.id === id)
    if (!persisted) return null
    return this.mergeWithRuntime(persisted)
  }

  // ── Connection ────────────────────────────────────────────────────────────

  async testConnection(input: DevServerInput): Promise<ConnectionTestResult> {
    const { connectionType, sshTargetId } = input

    // Phase 2: relay-websocket and direct-websocket bypass SSH entirely.
    // Create ephemeral bridge and connect directly via WS.
    if (connectionType === 'relay-websocket' || connectionType === 'direct-websocket') {
      const ephemeral: PersistedDevServer = {
        id: 'test-ephemeral',
        name: input.name ?? 'test',
        connectionType: input.connectionType,
        sshTargetId: undefined,
        wsUrl: input.wsUrl,
        workspaceDir: null,
        addedAt: Date.now(),
      }
      const bridge = new DevServerRelayBridge(ephemeral, this.sshManager, this.agentWsServer)
      try {
        const info = await bridge.connect({ testOnly: true })
        return { ok: true, platform: info.platform, nodeVersion: info.nodeVersion }
      } catch (err) {
        return {
          ok: false,
          error: err instanceof Error ? err.message : String(err),
        }
      }
      // bridge auto-disconnects in testOnly mode, no finally needed
    }

    // Why: for relay-ssh mode the relay bridge requires an active SSH connection.
    // Establish it here (test-only), disconnecting on completion regardless of outcome.
    // This allows AddDevServerDialog to test without a pre-existing SSH session.
    let sshConnectedForTest = false
    if (connectionType === 'relay-ssh' && sshTargetId) {
      const existing = this.sshManager.getConnection(sshTargetId)
      if (!existing) {
        try {
          await connectRegisteredSshTarget(sshTargetId)
          sshConnectedForTest = true
        } catch (err) {
          return {
            ok: false,
            error: `SSH connection failed: ${err instanceof Error ? err.message : String(err)}`
          }
        }
      }
    } else if (connectionType === 'relay-ssh' && !sshTargetId) {
      return { ok: false, error: 'No SSH target selected. Choose an SSH target for relay-ssh mode.' }
    }

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
    const bridge = new DevServerRelayBridge(ephemeralPersisted, this.sshManager, this.agentWsServer)
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
      // Disconnect SSH only if we established it just for this test
      if (sshConnectedForTest && sshTargetId) {
        try {
          await disconnectRegisteredSshTarget(sshTargetId)
        } catch {
          // Best effort — non-fatal
        }
      }
    }
  }


  async connect(id: string): Promise<void> {
    const persisted = this.store.list().find((ds) => ds.id === id)
    if (!persisted) throw new Error(`DevServer not found: ${id}`)

    const span = mgr.start({ op: 'connect', devServerId: id, type: persisted.connectionType })
    this.setRuntimeState(id, { status: 'connecting', lastError: null })
    this.emit('devServer:statusChanged', id, 'connecting')

    try {
      const bridge = new DevServerRelayBridge(persisted, this.sshManager, this.agentWsServer)

      // Forward bridge's agentTokenGenerated event to manager level.
      // Why: IPC handlers listen on manager events, not individual bridge instances.
      // This allows dev-server-ipc.ts to broadcast the token to the renderer
      // without knowing about individual bridge instances.
      bridge.on('agentTokenGenerated', (info: AgentTokenInfo) => {
        this.emit('devServer:agentToken', info)
      })

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
      span.ok({ platform: String(info.platform), node: info.nodeVersion })
    } catch (err) {
      const errorMsg = err instanceof Error ? err.message : String(err)
      this.setRuntimeState(id, { status: 'error', lastError: errorMsg })
      this.emit('devServer:statusChanged', id, 'error')
      span.fail(err, { devServerId: id })
      throw err
    }
  }

  async disconnect(id: string): Promise<void> {
    const span = mgr.start({ op: 'disconnect', devServerId: id })
    const relay = this.relays.get(id)
    if (relay) {
      await relay.disconnect()
      this.relays.delete(id)
    }
    this.setRuntimeState(id, { status: 'disconnected' })
    this.emit('devServer:statusChanged', id, 'disconnected')
    span.ok()
  }

  /**
   * Daemon-initiated connection: find-or-create a DevServer record and wire an
   * already-registered agent token to a DevServerRelayBridge.
   *
   * Called by POST /api/agent-token when the agent was started as a daemon with
   * a pre-registered token (not via the UI "Add Dev Server → Connect" flow).
   * After this method completes (or when the agent connects), the server will
   * appear as "connected" in the UI just like a normal direct-websocket server.
   *
   * This is non-blocking: the returned Promise resolves immediately after the
   * record is created and the slot is wired — before the agent actually connects.
   * The 'devServer:statusChanged' → 'connected' event fires when the agent handshakes.
   */
  async connectDaemonAgent(opts: {
    devServerId: string
    name: string
    token: string
    ttlMs?: number
  }): Promise<{ devServerId: string; created: boolean }> {
    // 1. Find or create persisted DevServer record
    const existing = this.store.list().find((ds) => ds.id === opts.devServerId)
    const persisted = this.store.addOrUpdate(opts.devServerId, {
      name: opts.name,
      connectionType: 'direct-websocket',
    })
    const wasCreated = !existing

    // 2. If there is already an active relay for this ID, check if it's connected.
    //    - Already connected (session != null): keep it alive, just re-register a NEW
    //      slot so the agent can reconnect after a network drop without full re-init.
    //    - Pending/disconnected: disconnect and create a fresh bridge.
    const existingRelay = this.relays.get(opts.devServerId)
    if (existingRelay) {
      if (existingRelay.isAlive()) {
        // Bridge is already live or reconnecting — no need to reset.
        // IMPORTANT: do NOT await connectWithExternalToken here.
        // The API must return the token immediately so start.sh can exec agent.js.
        // Awaiting would deadlock: API waits for agent to connect, but agent
        // cannot start until it receives the token from the API response.
        console.log(
          `[DevServerManager] Daemon re-register: id=${opts.devServerId} already connected ` +
          `— registering new slot non-blocking (no disconnect).`
        )
        // Fire-and-forget: register slot for newToken, update status when agent connects
        existingRelay.connectWithExternalToken(opts.token).then((info) => {
          this.setRuntimeState(opts.devServerId, {
            status: 'connected',
            platform: info.platform,
            arch: info.arch,
            nodeVersion: info.nodeVersion,
            lastConnectedAt: Date.now(),
            lastError: null,
          })
          this.emit('devServer:statusChanged', opts.devServerId, 'connected')
        }).catch((err: Error) => {
          console.warn(`[DevServerManager] Daemon re-register slot expired: id=${opts.devServerId} — ${err.message}`)
        })
        return { devServerId: opts.devServerId, created: wasCreated }
      }

      // Not connected — safe to disconnect and reinitialise
      await existingRelay.disconnect()
      this.relays.delete(opts.devServerId)
    }

    // Ensure runtime state entry exists
    if (!this.runtimeState.has(opts.devServerId)) {
      this.initRuntimeState(opts.devServerId)
    }
    if (wasCreated) {
      this.emit('devServer:added', opts.devServerId)
    }

    // 3. Create bridge and wire external token
    const bridge = new DevServerRelayBridge(persisted, this.sshManager, this.agentWsServer)

    // Forward agentTokenGenerated events (e.g. UI reconnect later)
    bridge.on('agentTokenGenerated', (info: AgentTokenInfo) => {
      this.emit('devServer:agentToken', info)
    })

    this.relays.set(opts.devServerId, bridge)
    this.setRuntimeState(opts.devServerId, { status: 'connecting', lastError: null })
    this.emit('devServer:statusChanged', opts.devServerId, 'connecting')

    // 4. Wire token (non-blocking) — resolves when agent connects
    bridge.connectWithExternalToken(opts.token).then((info) => {
      this.setRuntimeState(opts.devServerId, {
        status: 'connected',
        platform: info.platform,
        arch: info.arch,
        nodeVersion: info.nodeVersion,
        lastConnectedAt: Date.now(),
        lastError: null,
      })
      this.emit('devServer:statusChanged', opts.devServerId, 'connected')
      console.log(
        `[DevServerManager] Daemon agent connected: id=${opts.devServerId} ` +
        `platform=${info.platform} node=${info.nodeVersion}`
      )
    }).catch((err: Error) => {
      this.relays.delete(opts.devServerId)
      this.setRuntimeState(opts.devServerId, { status: 'error', lastError: err.message })
      this.emit('devServer:statusChanged', opts.devServerId, 'error')
      console.warn(`[DevServerManager] Daemon agent token expired/failed: id=${opts.devServerId} — ${err.message}`)
    })

    return { devServerId: opts.devServerId, created: wasCreated }
  }

  /**
   * generateAgentToken — Generate a one-time agent token for a direct-websocket dev server.
   * Called by session-manager.ts IPC handler 'generateAgentToken'.
   *
   * Flow: UI calls generateAgentToken(id) → gets token → displays setup instructions
   * Then: agent calls agent.handshake with token → AgentWebSocketServer validates → bridge connects.
   *
   * Note: This only generates the token. The actual slot registration happens in
   * connectWithExternalToken() which is called by connectDaemonAgent(). For now,
   * this generates and returns the token for use in setup instructions.
   */
  async generateAgentToken(devServerId: string): Promise<AgentTokenInfo> {
    const token = generateAgentToken(devServerId)
    const persisted = this.store.list().find((ds) => ds.id === devServerId)
    if (!persisted) {
      throw new Error(`Dev server not found: ${devServerId}`)
    }
    // Register slot so agent can connect immediately after user configures token
    const relay = this.relays.get(devServerId)
    if (relay && this.agentWsServer) {
      // Pre-register the slot so the agent can connect when ready
      void relay.connectWithExternalToken(token).catch(() => {
        // Slot may expire if agent doesn't connect — that's fine
      })
    }
    const host   = process.env['ORCA_ADVERTISED_HOST'] ?? 'localhost'
    const port   = process.env['ORCA_HTTP_PORT']       ?? '6768'
    const orcaUrl = `ws://${host}:${port}/agent`
    return { devServerId, agentToken: token, orcaUrl }
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
